# 预算管理与预算熔断实施文档

本文档定义 OCI 机器生命周期控制平台的预算管理模块。目标不是简单显示一个金额，而是建立“预算采集、风险判定、自动降配、预算熔断、审计回滚”的闭环。

当前状态：

- 已新增前端入口：平台治理 -> 预算管理。
- 已新增预算策略页面：可查看策略结构、触发阈值、动作模式和护栏。
- 已新增前端机器范围选择：支持按标签、Compartment、资源池/Fleet、手工实例列表限定候选机器。
- 已新增配置保存接口：`GET /api/budget/settings`、`PUT /api/budget/settings`，可持久化开关状态、预算阈值、范围配置和动作护栏。
- 尚未接入真实 Budget Evaluator、Cost Usage 采集器和自动执行器。
- 所有自动删机/降配逻辑必须后端落地并进入任务中心，前端不能直接执行 OCI 操作。

## 1. 官方能力边界

OCI Billing and Cost Management 支持预算、预算告警、成本分析和成本报告。预算是软限制，可设置在 compartment 或 cost-tracking tag 上，用于跟踪目标范围内的花费。预算告警会周期性评估，Oracle 文档说明预算告警按周期评估，通常不是实时熔断系统。

因此本平台需要两层逻辑：

1. OCI Budgets：用于创建、读取、同步官方预算和告警。
2. 平台 Budget Guard：用于更高频地估算机器成本，并在超限时创建任务执行降配、停止或终止实例。

官方依据：

- OCI Budgets Overview: https://docs.oracle.com/en-us/iaas/Content/Billing/Concepts/budgetsoverview.htm
- Creating a Budget: https://docs.oracle.com/en-us/iaas/Content/Billing/Tasks/create-budget.htm
- Billing and Cost Management Overview: https://docs.oracle.com/en-us/iaas/Content/Billing/Concepts/billingoverview.htm
- Listing Budgets: https://docs.oracle.com/en-us/iaas/Content/Billing/Tasks/list-budget.htm

## 2. 页面层级

导航位置：

```text
平台治理
├─ 账号与密钥
├─ 任务中心
├─ 预算管理
├─ 监控告警
├─ 审计日志
└─ 用户与权限
```

预算管理页面分为五块：

1. 预算概览：月预算、实际用量、预测用量、触发状态。
2. 策略配置：机器范围、阈值、动作模式、降配预设、审批开关。
3. 动作模式：只通知、优先降配、超出预算即删机。
4. 执行流程：采集、判定、候选、执行、审计。
5. 安全护栏：保护实例、审批、执行上限、冷却时间、保留启动卷。

机器范围选择是预算管理的前置条件，不能默认选中全部机器。页面必须至少提供以下四种范围：

- 按标签：推荐默认模式，例如 `budget.autoAction=enabled`。
- 按 Compartment：限定 Profile、Region、Compartment。
- 按资源池/Fleet：只处理指定 Instance Pool、Fleet 或平台资源池成员。
- 手工选择实例：页面读取真实实例清单并展示表格，用户勾选机器后形成精确影响范围，适合一次性处理小范围机器。

## 3. 核心数据模型

建议后端新增以下模型。

```go
type BudgetPolicy struct {
    ID                  string
    Name                string
    ProfileID           string
    RegionScope         []string
    TargetType          string // tenancy, compartment, tag, pool, manual
    TargetIDs           []string
    MonthlyBudgetUSD    float64
    ThresholdPercent    float64
    ForecastThreshold   float64
    ActionMode          string // notify, downgrade, delete
    DowngradePreset     string // free-first, min-flex, custom, stop-only
    CustomShape         string
    CustomOCPUs         int
    CustomMemoryGB      int
    PreserveBootVolume  bool
    DeleteBootVolume    bool
    RequireApproval     bool
    Enabled             bool
    CooldownMinutes     int
    MaxActionsPerRun    int
    MaxActionsPerDay    int
    CreatedAt           time.Time
    UpdatedAt           time.Time
}

type BudgetScope struct {
    Mode                 string // tag, compartment, pool, manual
    ProfileID            string
    Regions              []string
    CompartmentIDs       []string
    ResourcePoolIDs      []string
    TagKey               string
    TagValue             string
    ManualInstanceIDs    []string // 来自实例表格勾选结果，而不是默认全选
    RequireAutoActionTag bool
}

type BudgetEvaluation struct {
    ID                string
    PolicyID          string
    ActualSpendUSD    float64
    ForecastSpendUSD  float64
    BudgetUSD         float64
    ActualPercent     float64
    ForecastPercent   float64
    Triggered         bool
    TriggerReason     string
    CandidateCount    int
    PlannedAction     string
    Status            string // clear, notify, pending_approval, enqueued, failed
    EvaluatedAt       time.Time
}

type BudgetCandidate struct {
    InstanceID          string
    OCIInstanceID       string
    Name                string
    ProfileID           string
    Region              string
    CompartmentID       string
    Shape               string
    OCPUs               int
    MemoryGB            int
    BootVolumeGB        int
    EstimatedMonthlyUSD float64
    Protected           bool
    AutoActionAllowed   bool
    RecommendedAction   string // downgrade, stop, terminate, skip
    SkipReason          string
}
```

## 4. 预算采集逻辑

采集来源按优先级：

1. OCI Budgets API：同步官方 Budget、Alert Rule、Actual Spend、Forecast Spend。
2. OCI Cost and Usage Reports / Cost Analysis：用于更细的资源级成本校准。
3. 本地成本估算：用于新建机器前的即时预算预估，以及官方账单数据延迟时的近实时保护。
4. 平台实例库存：补充 shape、OCPU、内存、启动盘、公网 IP、标签和业务归属。

注意：

- OCI Budgets 是软限制，不应被理解为实时断电开关。
- 预算告警按周期评估，平台若要实时熔断，必须有自己的定时 evaluator。
- 官方预算权限需要 root tenancy 级别的 `usage-budgets` 权限。

## 4.1 创建页预算估算与价格表覆盖

当前创建实例页的预算估算仍是临时本地估算，存在已知缺口：

- `VM.Standard.E2.1.Micro`、`VM.Standard.A1.Flex` 和部分 `VM.Standard.Ex.Flex` 类 Shape 已能显示预算估算。
- 非 Flex 标准 Shape 已按已发现的 OCPU/内存做粗估，并显示“粗估”状态。
- DenseIO、GPU、HPC、BM 或特殊 Shape 若未接入价格表，会显示“价格未接入”，不能显示 0 或空白。
- 仍需后续接统一价格表服务，把前端临时估算迁移到后端。

必须补齐为统一价格表服务：

```go
type PriceCatalogItem struct {
    ShapePattern       string
    ShapeFamily        string
    BillingModel       string // flex_ocpu_memory, fixed_instance, gpu, bare_metal, unknown
    OCPUHourUSD        float64
    MemoryGBHourUSD    float64
    InstanceHourUSD    float64
    BootVolumeGBMonth  float64
    Region             string
    Currency           string
    Source             string // oci_rate_card, cost_report, manual
    Version            string
    UpdatedAt          time.Time
}
```

估算规则：

1. 优先使用后端价格表，不允许前端维护独立价格逻辑。
2. 如果 Shape 是 Flex，按 OCPU + Memory 计算。
3. 如果 Shape 是固定规格，按 shape 对应 SKU 或固定实例小时价计算。
4. 如果 Shape 价格未知，页面必须显示“价格未接入”，不能显示 0 或空白。
5. Always Free 只能作为抵扣规则，不能简单等同于永久免费；必须结合账号已用免费额度。
6. 启动盘价格必须结合真实 boot volume 已用量和 Always Free Block Volume 额度。
7. 预算卡必须显示价格来源、价格表版本和最后更新时间。

前端验收条件：

- 任意 Shape 选择后预算卡都有明确状态：可估算、免费额度内、超出免费额度、价格未接入。
- 非 Flex Shape 不再静默显示“待计算”而无解释。
- 价格未接入时提供“需补价格表”的提示，不允许让用户误以为该 Shape 免费。

后端验收条件：

- 新增 `GET /api/pricing/estimate` 或在 `GET /api/launch-options` 中返回 `pricing` 字段。
- 创建实例页和预算管理页共用同一个价格估算服务。
- 价格表可以从 OCI Rate Card / Cost Report / 手动维护文件逐步接入。

## 4.2 Shape 变更与 Image 兼容刷新

当前已知问题：

- Web 创建实例页已实现：切换 Shape 后自动刷新兼容 Image。
- OCI 官方创建实例流程会根据 Shape 过滤可用镜像；有些镜像在某些 Shape 或架构上不可用。
- 如果前端保留旧 Image，用户可能提交一个与目标 Shape 不兼容的组合，最终在 `LaunchInstance` 阶段失败。

必须补齐联动逻辑：

1. 用户变更 Shape 后，前端立即调用：

```http
GET /api/launch-options?profileId=...&region=...&compartmentId=...&availabilityDomain=...&vcnId=...&shape=...
```

2. 后端继续使用 Compute `ListImages` 的 `shape` 过滤能力返回兼容镜像。
3. 前端进入 Image 刷新中状态，禁用提交按钮或显示“正在刷新兼容镜像”。
4. 如果当前 `imageId` 不在新的 Image 列表中，自动清空并选择第一个兼容镜像，或要求用户重新选择。
5. 如果用户手动输入 Image OCID，必须显示“未验证兼容性”提示，并在提交前执行兼容性预检查。
6. Shape、AD、Compartment 任一变化都应触发 Image 刷新，因为这些条件都会影响镜像和容量可用性。
7. 刷新失败时显示真实 OCI 错误，不回退到假数据。

验收条件：

- 切换 `VM.Standard.E3.Flex`、`VM.Standard.A1.Flex`、`VM.Standard.E2.1.Micro` 时 Image 列表会重新加载。当前前端逻辑已实现，仍需真实 OCI 专项验证。
- ARM Shape 不显示仅 x86 可用镜像。
- x86 Shape 不默认选中 ARM-only 镜像。
- 旧 Image 不兼容时，页面会清空或替换，而不是继续提交。
- 真实 API 返回错误时，页面展示错误并阻止创建。

## 5. 风险判定逻辑

每个策略定时评估：

```text
actualPercent = actualSpend / monthlyBudget * 100
forecastPercent = forecastSpend / monthlyBudget * 100

triggered =
  actualPercent >= thresholdPercent
  OR forecastPercent >= forecastThreshold
  OR localEstimatedMonthEnd > monthlyBudget
```

触发后进入动作分级：

1. Level 0：未触发，只记录评估结果。
2. Level 1：接近阈值，发送通知，冻结非必要创建。
3. Level 2：超过阈值，执行优先降配。
4. Level 3：超过熔断阈值，执行停止或终止实例。
5. Level 4：删除启动卷，只允许在显式开启并审批后执行。

## 6. 候选机器筛选

预算动作只能处理选中范围内的机器，不能扫描全账号后随便删。任何策略如果没有明确范围，后端必须拒绝保存或拒绝执行。

范围来源：

- Profile
- Region
- Compartment
- Instance Pool / Fleet
- 标签，例如 `budget.autoAction=enabled`
- 手工勾选的实例列表

范围规则：

1. 默认范围是按标签，不是全账号。
2. `manual` 模式下必须展示实例表格供用户勾选；如果勾选列表为空，候选数量必须为 0。
3. `compartment` 模式必须至少有 Profile、Region、Compartment 约束。
4. `pool` 模式必须能解析到平台资源池、Instance Pool 或 Fleet 成员，否则候选数量为 0。
5. `tag` 模式必须提供 tag key；tag value 允许为空但必须在 dry-run 中清晰展示匹配规则。
6. 删除和自动降配建议仍要求 `budget.autoAction=enabled` 或策略显式开启 `RequireAutoActionTag=false`，并写入审计。

执行前必须先 dry-run：

```text
BudgetPolicy
-> ResolveScope
-> ListInstancesByScope
-> ApplyProtectionRules
-> BuildCandidates
-> ShowCandidateList
-> Approval / Enqueue Jobs
```

dry-run 结果必须返回：

- 命中的实例数量。
- 被跳过的实例数量和原因。
- 每台候选机器的当前成本、预计节省、推荐动作。
- 本次策略会影响的 Profile、Region、Compartment、Pool、标签或手工实例列表。
- 是否需要审批。

必须跳过：

- `protected=true`
- 标签 `budget.protected=true`
- 标签 `role=control-plane`
- 最近 N 分钟内刚创建的实例
- 正在执行其它 Job 的实例
- 备份/快照失败且策略要求先备份的实例
- 不属于当前预算策略 target 的实例

排序建议：

1. 预估月成本最高的实例优先。
2. 非生产标签优先，例如 `env=dev/test`。
3. 空闲或停止中的实例优先。
4. 最近使用率低的实例优先。
5. 未绑定保留公网 IP 或关键标签的实例优先。

## 7. 自动降配逻辑

“降低配置”不是简单改 shape。必须先做兼容性与容量检查。

降配步骤：

1. 读取实例当前 shape、OCPU、内存、镜像、架构、AD、启动卷。
2. 判断目标预设：
   - `free-first`：优先尝试免费范围。
   - `min-flex`：当前 Flex shape 的最小 OCPU/内存。
   - `custom`：用户指定 shape/OCPU/内存。
   - `stop-only`：不改 shape，只停止实例。
3. 检查目标 shape 是否在 AD 可用。
4. 检查镜像与目标 shape 是否兼容。
5. 检查服务限制和容量。
6. 如果运行中改配会重启，任务必须标注影响。
7. 可选创建快照。
8. 创建 `RESIZE` Job，执行 OCI `UpdateInstance`。
9. 验证实例状态、shape、OCPU、内存。
10. 写入审计日志和通知。

免费范围判断建议：

- `VM.Standard.E2.1.Micro`：Always Free 常见免费 Shape。
- `VM.Standard.A1.Flex`：只有在账号免费额度内才视为免费；需要结合 OCPU、内存和账号已用量判断。
- 从 x86 降到 ARM 不是普通降配，只有镜像、系统和业务明确兼容时才允许。

失败处理：

- 降配失败但策略允许停止：创建 `STOP` Job。
- 降配失败且策略不允许停止：通知并标记 `ROLLBACK_REQUIRED` 或 `MANUAL_REQUIRED`。
- 不允许静默跳过，必须记录原因。

## 8. 超出预算即删机逻辑

这是危险功能，必须是显式开启的预算熔断模式。

建议产品命名：

- 页面显示：超出预算即删机
- 后端动作：Budget Breaker / Termination Guard

执行条件：

1. 策略 `enabled=true`。
2. `actionMode=delete`。
3. 当前评估已超过熔断阈值。
4. 实例在策略范围内。
5. 实例没有保护标记。
6. 实例带有自动动作允许标记，或策略明确选择了该实例。
7. 未超过每日执行上限。
8. 冷却时间已过。
9. 若 `requireApproval=true`，必须审批通过。

终止策略：

- 默认 `preserveBootVolume=false`。
- 默认不释放保留公网 IP。
- 默认删除启动卷，避免终止实例后继续产生孤儿磁盘费用。
- 保留启动卷必须显式开关；删除启动卷仍需在审批中显示不可恢复风险。

任务流：

```text
BUDGET_TRIGGERED
-> BUILD_CANDIDATES
-> PLAN_ACTIONS
-> APPROVAL_REQUIRED 或 ENQUEUE_JOBS
-> TERMINATE_INSTANCE
-> VERIFY_TERMINATED
-> AUDIT_AND_NOTIFY
```

## 9. API 设计

建议新增接口：

```http
GET    /api/budgets/policies
POST   /api/budgets/policies
GET    /api/budgets/policies/{id}
PUT    /api/budgets/policies/{id}
DELETE /api/budgets/policies/{id}

POST   /api/budgets/policies/{id}/evaluate
POST   /api/budgets/policies/{id}/dry-run
POST   /api/budgets/evaluations/{id}/approve
POST   /api/budgets/evaluations/{id}/reject
GET    /api/budgets/evaluations
GET    /api/budgets/candidates?policyId=...
```

当前已落地的轻量配置接口：

```http
GET /api/budget/settings
PUT /api/budget/settings
```

该接口只保存预算管理开关、阈值、范围和动作配置，不会执行 OCI 操作。真正执行仍必须进入 dry-run、审批和任务中心。

所有危险动作都必须生成 Job：

- `BUDGET_EVALUATE`
- `BUDGET_FREEZE_CREATE`
- `BUDGET_DOWNGRADE_INSTANCE`
- `BUDGET_STOP_INSTANCE`
- `BUDGET_TERMINATE_INSTANCE`

## 10. UI 行为

预算管理页必须坚持以下规则：

- 未接后端执行器时，按钮只能保存草稿或展示策略，不允许显示“已生效”。
- 页面必须展示“机器范围选择”，用户不能通过一个全局开关默认选择全部机器。
- 页面必须展示当前候选范围摘要；手工选择实例模式下需要显示实例表格、搜索框、已选数量、不可选原因。
- 手工选择实例表格必须来自真实 `/api/instances` 或后端实例库存；受保护、终止中、已终止实例默认不可勾选。
- 删除类动作必须显示候选实例列表和预计节省金额。
- 每个候选实例需要显示跳过原因或推荐动作。
- “超出预算即删机”必须有醒目的危险提示。
- 用户必须能看到该策略会影响哪些 Profile、Region、Compartment、标签和实例。
- 自动执行历史必须跳转任务中心。
- 审计日志必须能按 `budgetPolicyId` 和 `evaluationId` 过滤。

## 11. 阶段化落地

### Phase 1：页面与策略模型

- 新增预算管理导航和页面。
- 新增 `BudgetPolicy`、`BudgetEvaluation`、`BudgetCandidate` 模型。
- 前端支持按标签、Compartment、资源池/Fleet、手工实例列表选择预算动作范围。
- 支持策略 CRUD，但不执行实例动作。
- 支持 dry-run 生成候选列表。

### Phase 2：采集器

- 接 OCI Budgets API。
- 接 Cost Usage / Cost Reports。
- 将创建实例页预算估算与预算策略共用价格表。
- 增加价格表版本和最后校准时间。

### Phase 3：评估器

- 后端 Scheduler 定时评估预算策略。
- 实现阈值、预测、冷却时间、每日执行上限。
- 输出 `BudgetEvaluation` 和候选机器列表。

### Phase 4：降配执行器

- 支持 `free-first`、`min-flex`、`custom`、`stop-only`。
- 与现有 `RESIZE` Job 复用。
- 增加兼容性检查、容量检查、快照选项和验证步骤。

### Phase 5：预算熔断删机

- 接审批流。
- 支持按候选列表终止实例。
- 默认删除启动卷。
- 增加失败恢复、人工介入状态、审计导出。

## 12. 测试计划

单元测试：

- 阈值计算。
- 候选筛选。
- 保护实例跳过。
- 降配目标选择。
- 删机动作必须 requireApproval 或 explicit breaker mode。

集成测试：

- Dry-run 不创建 OCI Job。
- 超预算只通知模式只生成通知。
- 降配模式生成 `RESIZE` Job。
- 删机模式生成 `TERMINATE` Job，默认删除 boot volume；需要保留时必须显式设置 preserveBootVolume。
- 审计日志记录完整 request/result payload。

真实 OCI 验证：

- 读取 OCI Budget 列表。
- 创建测试预算和 alert rule。
- 同步 Actual/Forecast 字段。
- 创建测试实例，触发 dry-run 候选。
- 在人工确认后验证降配。
- 删机测试只允许测试实例，且默认删除启动卷。

## 13. 当前未落地清单

- OCI Budgets SDK 客户端。
- Cost Usage / Cost Report 采集。
- BudgetPolicy 后端 CRUD。
- Budget Evaluator 调度器。
- 候选实例 dry-run API。
- 机器范围后端解析器：tag / compartment / pool / manual。
- 自动降配策略执行器。
- 预算熔断审批与删机执行器。
- 预算审计过滤和导出。
