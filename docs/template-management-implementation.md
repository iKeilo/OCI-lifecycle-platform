# 模板管理回归实施文档

更新日期：2026-06-14

本文定义“模板管理”重新加入 OCI 机器生命周期控制平台的实施边界。模板不是一个新的 OCI 执行器，也不是验证资源是否一定可创建的机制；模板只是一组“创建实例表单预输入值”，用于减少重复填写和人为填错。

## 1. 核心结论

- 模板只面向 Oracle Cloud Infrastructure，不引入其它云厂商抽象。
- 模板不调用 OCI API，不需要 OCI 密钥，也不做真实 OCI 验证。
- 模板 API 可以在没有任何 PEM 的情况下创建、编辑、复制、删除；Web 新建/编辑模板使用创建实例页的真实选项向导，正常使用时需要先有可用 Profile 以加载 Launch Options。
- 模板页的“检查”只做本地字段完整性检查，例如名称、Shape、OCPU、内存、启动盘、网络字段是否缺失。
- 真正调用 OCI API 的时刻只有用户在“创建实例”页面点击“创建实例任务”之后。
- 模板被应用后，后端解析模板保存的 JSON/YAML 配置并预填创建实例表单；JSON/YAML 是内部存储格式，不作为常规新建/编辑模板入口暴露给用户。
- 不允许内置演示模板。空列表就是空列表，所有模板都必须来自用户保存。

## 2. 产品定位

模板是可复用的 OCI 创建参数预设，适合以下场景：

- 常用实例名称、区域、Shape、镜像、网络、公网 IPv4、IPv6、启动盘大小和硬盘性能预填。
- 将某次创建页面的当前配置保存为可复用模板。
- 在创建实例时快速套用模板，再根据当前需求微调。
- 给后续自动化策略提供稳定的输入来源，但自动化执行仍应单独做真实创建前检查。

模板不保存以下内容：

- OCI API 私钥。
- SMTP 密码。
- 面板登录密码。
- Webhook secret。
- 明文 root 密码。
- 已生成的一次性敏感凭据。

## 3. 功能层级

```text
模板管理
├─ 模板列表
│  ├─ 按 Profile / Region / 状态 / 关键字筛选
│  ├─ 展示 Shape、OCPU、内存、启动盘、网络、IPv6 和字段检查状态
│  └─ 操作：使用、检查字段、编辑、复制、删除
├─ 新建模板
│  ├─ 进入 /templates/new
│  ├─ 在模板管理内调用真实 Launch Options，不跳转创建实例页
│  ├─ 使用与创建实例一致的上下文、镜像规格、网络访问、公网 IPv4/IPv6 选项
│  ├─ 支持设置默认实例名称，使用模板创建机器时自动预填
│  ├─ Profile、Region、Compartment、AD、Image、Shape、VCN、Subnet、Reserved IP 都来自真实下拉选项
│  ├─ OCPU、内存由 Shape 返回的 min/max 生成可选项；固定规格会冻结选择
│  ├─ SSH Key、cloud-init、标签和重试策略不作为模板新建页的自由输入项
│  └─ 最后一步保存模板，不提交 LaunchInstance 任务
├─ 编辑模板
│  ├─ 进入 /templates/{templateId}/edit
│  ├─ 先按模板解析结果预填模板选项页
│  └─ 最后一步把“创建实例任务”替换为“更新模板”
└─ 创建实例页使用模板
   ├─ 选择模板
   ├─ 应用到当前表单
   ├─ 可清空模板选择
   ├─ 可保存当前表单为模板
   └─ 提交时按当前表单创建真实 OCI 任务
```

## 4. 状态模型

业务状态：

| 状态 | 含义 | 是否显示 |
| --- | --- | --- |
| `ACTIVE` | 可在创建实例页选择 | 是 |
| `DISABLED` | 暂停使用 | 是 |
| `ARCHIVED` | 历史归档 | 是 |
| `DELETED` | 已删除 | 否 |

字段检查状态：

| 状态 | 含义 |
| --- | --- |
| `UNVERIFIED` | 尚未执行本地字段完整性检查 |
| `VALID` | 必填字段完整 |
| `INVALID` | 必填字段缺失或数值不合法 |

注意：这里的 `VALID` 只表示“本地字段完整”，不表示 OCI 资源兼容或容量可用。

## 5. 当前数据模型

第一阶段使用单表/单对象模型，保留 `version` 字段，为后续版本化预留空间。

```go
type InstanceTemplate struct {
    ID                    string
    Name                  string
    Description           string
    Version               string
    ProfileID             string
    Region                string
    Compartment           string
    CompartmentID         string
    AvailabilityAD        string
    ImageID               string
    ImageName             string
    Shape                 string
    OCPUs                 int
    MemoryGB              int
    BootVolumeGB          int
    BootVolumeVPUsPerGB   int
    VCNID                 string
    SubnetID              string
    AssignPublicIP        bool
    EnableIPv6            bool
    ReservedPublicIP      string
    SSHKey                string
    CloudInit             string
    CloudInitSet          bool
    Tags                  map[string]string
    ConfigFormat          string // json, yaml
    ConfigText            string // 创建实例预输入配置文本
    Status                string
    ValidationStatus      string
    ValidationErrorCode   string
    ValidationMessage     string
    LastValidatedAt       time.Time
    CreatedBy             string
    CreatedAt             time.Time
    UpdatedAt             time.Time
}
```

## 6. 后端实施

已实施接口：

```text
GET    /api/templates
POST   /api/templates
GET    /api/templates/{id}
PATCH  /api/templates/{id}
DELETE /api/templates/{id}
POST   /api/templates/{id}/validate
```

`GET /api/templates` 查询参数：

```text
profileId
region
status
q
limit
```

`POST /api/templates/{id}/validate` 行为：

- 不调用 OCI。
- 不读取 OCI 私钥。
- 不要求 `OCI_EXECUTION_MODE=oci`。
- 只检查本地字段是否完整。
- 字段完整时返回 `verified=true`，并将模板字段检查状态记为 `VALID`。
- 字段缺失时返回 HTTP 200 + `verified=false`，并将状态记为 `INVALID`。

`POST /api/instances` 与模板关系：

- 请求可以带 `templateId`。
- 后端读取模板作为预输入来源，再用请求里的当前表单字段覆盖模板字段。
- 后端优先解析 `configText`，支持 JSON 与简单 YAML；解析结果会同步到结构化字段。
- Job input 会记录 `templateId`，用于审计追踪。
- 最终创建仍走既有实例创建任务；如果运行在 OCI 模式，真实 OCI 调用发生在该任务中。

## 7. 持久化

已实施：

- PostgreSQL `instance_templates` 表。
- 表中保存结构化字段，同时保存 `config_format` 和 `config_text`。
- 无 PostgreSQL 时写入本地 profile/file store。
- 服务启动时从 PostgreSQL 或 file store 加载模板。
- 删除模板时从持久化仓库删除或标记删除，列表不展示 `DELETED`。
- API 列表不返回 `cloudInit` 明文，只返回 `cloudInitSet`。

后续可增强：

- 模板版本表 `instance_template_versions`。
- 模板使用记录 `template_usage_events`。
- 从成功创建任务反向生成模板。
- 从已有实例反向生成模板。

## 8. 前端实施

已实施页面：

- 左侧导航恢复“模板管理”。
- `/templates` 模板列表、筛选、新建、编辑、复制、删除、字段检查。
- `/templates` 的“新建模板”进入 `/templates/new`，页面内部调用真实 Launch Options。
- `/templates` 的“编辑模板”进入 `/templates/{templateId}/edit`，不暴露 JSON/YAML 手写配置作为主流程。
- `/templates` 的 Profile / Region 筛选改为下拉，避免用户手写 profileId 或 region。
- `/create` 创建实例页增加“模板预输入”区。
- `/templates/new` 和 `/templates/{templateId}/edit` 的 OCI 资源字段只显示真实选项下拉或禁用占位，不提供 OCID 手写 fallback。
- `/templates/new` 和 `/templates/{templateId}/edit` 支持设置默认实例名称，该值写入内部 JSON 配置。
- `/templates/new` 和 `/templates/{templateId}/edit` 的 OCPU、内存、启动盘大小使用可选项；固定规格 Shape 会冻结 OCPU/内存。
- `/templates/new` 和 `/templates/{templateId}/edit` 不展示标签与重试策略，不要求用户填写 SSH Key 或 cloud-init 文本。
- `/create?templateId=...` 可从模板页跳转并自动预填。
- 创建实例页支持“保存当前配置为模板”。
- 模板提示文案明确说明不会调用 OCI API。

移动端要求：

- 模板预输入工具栏在窄屏下改为单列。
- 模板列表表格沿用现有横向滚动容器，避免挤压字段。

## 9. 阶段计划

第一阶段，已落地：

- 移除内置演示模板。
- 模板 CRUD。
- 本地字段完整性检查。
- PostgreSQL/file store 持久化。
- 创建实例页应用模板与保存模板。
- 新建模板在模板管理内调用真实选项，保存为内部 JSON 配置文本。
- 模板模式移除 OCI 资源手写输入 fallback，常规用户只通过真实选项保存模板。
- 后端支持 JSON/YAML 模板配置解析。
- 后端创建任务记录 `templateId`。

第二阶段，建议继续：

- 模板详情页。
- 模板版本化。
- 模板使用历史。
- 从现有实例生成模板。
- 从成功 Job 生成模板。
- 自动化策略引用模板。

第三阶段，执行前真实检查：

- 不在模板页做 OCI 验证。
- 在创建实例任务提交后，由现有 OCI executor 做真实 Launch Options、Shape/Image 兼容、网络、配额、容量和权限检查。
- 所有真实 OCI 结果记录到 Job、Audit 和通知中心。

## 10. 验收标准

- 没有 OCI 密钥时，模板 API 仍可以创建、编辑、复制、删除结构化模板；Web 真实选项向导会提示先配置可用 Profile。
- 没有模板时，模板列表显示空状态，不显示演示数据。
- 点击“检查字段”不会访问 OCI，缺字段时返回可读提示。
- 从模板页点击“使用”后进入创建实例页并预填表单。
- 从模板页点击“新建模板”后进入 `/templates/new`，最后一步保存模板而非创建机器。
- 从模板页点击“编辑模板”后进入 `/templates/{templateId}/edit`，字段来自真实选项，不要求用户手写 JSON/YAML。
- 模板模式中的 OCI 资源字段没有自由输入框；除模板名称外，资源参数通过下拉、开关或受约束的数值选项选择。
- 使用带默认实例名称的模板创建机器时，创建实例页会自动填入实例名称；如果请求没有传 name，后端也会从模板配置兜底补齐。
- 创建实例页清空模板后不再显示已应用模板。
- 创建实例页保存当前配置后，模板列表可以看到新模板。
- `POST /api/instances` 带 `templateId` 时创建任务成功入队，并在 Job input 中记录来源模板。
- 前后端构建和测试通过。
