# 实例系统设置实施文档

更新日期：2026-06-19

本文定义“实例管理 -> 系统设置”的实施方案。当前入口只保留“重装系统”。“重置密码”功能已从产品中移除，后续不规划通过 Oracle Cloud Agent / Run Command 或离线救援盘自动重置密码，避免把不可通用、不可稳定验证的路径做成平台能力。

## 1. 目标

- 在实例管理中增加“系统设置”操作组。
- 展开后提供“重装系统”入口。
- 所有操作都创建可追踪 Job，由后端 OCI executor 执行。
- 所有敏感凭据只在后端生成、加密或敏感通知中短期展示，不写入普通日志、Job 输出或浏览器缓存。
- 所有真实 OCI 行为必须单独验证，未验证前在文档、UI 和任务结果中明确标记。

## 2. 非目标

- 不把 OCI Console 里的人工交互功能伪装成已经自动化。
- 不用 local executor、mock 数据或静态成功响应冒充真实重装。
- 不在 GitHub、安装脚本、前端 bundle、普通通知、审计明文中保存 root 密码、SMTP 密码、OCI PEM 私钥。
- 第一阶段不支持 Windows 密码重置自动化。Windows 初始密码和后续密码管理与 Linux 路径不同，需要单独设计。

## 3. 功能入口

```text
实例管理
├─ 实例列表
│  └─ 行操作
│     └─ 系统设置
│        └─ 重装系统
└─ 实例详情
   └─ 危险操作区
      └─ 系统设置
         └─ 重装系统
```

UI 形态建议：

- 列表行上显示一个“系统设置”按钮，点击后展开下拉菜单。
- “重装系统”打开专用向导。
- 弹窗提交成功后必须自动关闭，并跳转或提示查看任务中心。
- 弹窗内固定展示实例名、实例 OCID、Profile、Region、Compartment、当前 Shape、当前镜像、当前启动盘大小。

## 4. 权限与护栏

新增权限点：

| 权限 | 含义 |
| --- | --- |
| `instances.systemSettings.view` | 查看实例系统设置入口 |
| `instances.reinstall.execute` | 提交重装系统任务 |
| `instances.reinstall.preserveBootVolumeOverride` | 修改是否保留旧启动盘 |

安全护栏：

- 生产保护实例默认禁止重装，需要管理员解除保护或审批。
- 重装系统必须二次确认，确认文案需要输入实例名。
- 删除旧启动盘或不保留旧启动盘时必须单独确认。
- 重装系统必须写审计日志，包含 actor、profileId、region、instanceId、requestId、workRequestId、任务 ID、输入参数脱敏摘要。
- RBAC 范围校验必须复用现有 Profile、Region、Compartment 限制，禁止通过 path parameter 越权操作不在用户范围内的实例。
- 所有 ID 参数只接受 OCI OCID 或系统内合法 ID，拒绝 `../`、反斜杠、控制字符、URL 编码穿越等路径穿越输入。

## 5. 密码能力移除决策

OCI Compute 没有一个通用、稳定、跨镜像的“直接重置 Linux root 密码”API。平台不再提供“重置密码”入口，也不规划通过实例 Agent、远程命令、SSH 执行或离线救援盘自动重置密码。

移除原因：

- Agent / 远程命令依赖实例内组件、镜像、网络、IAM 和 sshd 配置，失败面过大。
- 离线救援盘流程涉及停机、启动盘挂载、文件系统修改和回滚，风险高，不适合做成通用平台按钮。
- 密码属于敏感凭据，任何未充分验证的自动化都可能制造假成功、锁死登录或泄露风险。
- 当前产品核心是 OCI 资源生命周期控制，不把系统内账号恢复包装成通用能力。

后续边界：

- 创建新实例时仍可通过 cloud-init 生成初始 root 密码，这是创建流程能力，不属于已有实例密码重置。
- 重装系统不会生成新 root 密码，也不会注入 SSH 公钥或 cloud-init。
- 通知中的 SSH 密码字段必须明确显示“未生成 / 未变更”。
- 若用户需要重置已有系统账号密码，应通过 OCI Console、串行控制台或人工运维流程处理，不进入本平台自动化。

### 5.1 重装时设置密码

实现时已确认 OCI Go SDK 的 `UpdateInstanceDetails` 文档限制：实例启动后 `metadata.user_data` 和 `metadata.ssh_authorized_keys` 不能被修改；`UpdateInstanceSourceViaImageDetails` 也只接受 `imageId`、是否保留旧启动盘、启动盘大小和 KMS key。因此当前版本不把“重装时设置 root 密码”标记为已支持能力。

当前限制：

- 当前产品 UI 不提供“重装时生成 root 密码”开关。
- 后端收到 `generateRootPassword=true`、`cloudInit` 或 `sshAuthorizedKey` 时返回明确错误。
- 重装通知中的 SSH 密码字段固定为“未生成 / 未变更”。

## 6. 重装系统设计

重装系统使用 OCI Compute `UpdateInstance`，将 `SourceDetails` 设置为新镜像来源。当前项目在 smoke 工具中已有类似路径，后续需要产品化到正式 executor。

### 6.1 UI 字段

```text
重装系统
├─ 目标镜像
│  ├─ Image 下拉
│  └─ 只展示当前 Shape 兼容镜像
├─ 启动盘
│  ├─ 当前启动盘大小
│  ├─ 目标启动盘大小
│  └─ 目标 VPUs/GB
├─ 数据保护
│  ├─ 保留旧启动盘
│  ├─ 重装前创建启动盘备份或快照
│  └─ 不保留旧启动盘时二次确认
├─ 登录方式
│  ├─ SSH 密码状态：未生成 / 未变更
│  ├─ 服务器连接信息：公网 IPv4、公网 IPv6、私网 IP
│  ├─ 推送站内通知
│  └─ 推送邮件通知
├─ 执行策略
│  ├─ 允许停机
│  ├─ 失败重试模式
│  └─ 超时时间
└─ 最终确认
   └─ 输入实例名确认
```

当前重装系统不会生成 root 密码，也不会注入 SSH 公钥。OCI `UpdateInstance SourceDetails` 路径不能修改已启动实例的 `user_data` 或 `ssh_authorized_keys`，所以通知必须明确写出“SSH 密码：未生成 / 未变更”，不能生成假密码或假成功。

重装通知是强制行为：
- 创建任务成功后发送“重装系统任务已创建”站内通知，并尝试邮件推送。
- OCI 执行成功后发送“重装系统成功”站内通知，并尝试邮件推送。
- OCI 执行失败后发送“重装系统失败”站内通知，并尝试邮件推送。
- 通知内容包含操作、Job ID、操作人、实例名称、实例 OCID、Profile、Region、Compartment、镜像、启动盘大小、启动盘性能、是否保留旧启动盘、公网 IPv4、公网 IPv6、私网 IP、OCI request id / work request id 和失败错误。

### 6.2 后端请求模型

```go
type InstanceSystemReinstallRequest struct {
    ProfileID                 string `json:"profileId"`
    Region                    string `json:"region"`
    CompartmentID             string `json:"compartmentId"`
    ImageID                   string `json:"imageId"`
    ImageName                 string `json:"imageName"`
    BootVolumeSizeGB          int64  `json:"bootVolumeSizeGb"`
    BootVolumeVPUsPerGB       int64  `json:"bootVolumeVpusPerGb"`
    PreserveOldBootVolume     bool   `json:"preserveOldBootVolume"`
    CreateBootVolumeBackup    bool   `json:"createBootVolumeBackup"`
    GenerateRootPassword      bool   `json:"generateRootPassword"`
    NotifyPasswordInApp       bool   `json:"notifyPasswordInApp"`
    NotifyPasswordByEmail     bool   `json:"notifyPasswordByEmail"`
    SSHAuthorizedKey          string `json:"sshAuthorizedKey,omitempty"`
    CloudInit                 string `json:"cloudInit,omitempty"`
    ConfirmationName          string `json:"confirmationName"`
}
```

### 6.3 后端执行流程

```text
校验权限和管理范围
-> 读取实例实时状态
-> 校验 confirmationName
-> 校验 Shape/Image 兼容
-> 校验启动盘只能等于或大于当前大小
-> 校验 VPUs/GB 合法范围
-> 校验通知和 root 密码护栏
-> 可选创建启动盘备份
-> 创建 Job
-> 调用 UpdateInstance SourceDetails
-> 记录 opc-request-id 和 opc-work-request-id
-> 等待实例进入 RUNNING 或 STOPPED
-> 同步实例和启动盘状态
-> 创建通知和审计日志
```

失败处理：

- 如果 OCI 在提交前失败，Job 标记 `FAILED`。
- 如果 OCI 已接受请求但后续等待失败，Job 标记 `VERIFYING_FAILED` 或 `ROLLBACK_REQUIRED`。
- 如果创建了备份但重装失败，不自动删除备份。
- 重装成功不代表 SSH 密码可用；平台不做密码登录验证。

## 7. API 设计

建议新增专用 API，而不是继续把高风险操作塞进通用 `/actions`：

```text
GET  /api/instances/{id}/system/options
POST /api/instances/{id}/system/reinstall
```

`GET /system/options` 返回：

- 实例当前状态。
- 当前镜像和 Shape。
- 当前启动盘大小和性能。
- 可选镜像列表。
- 当前用户是否有对应权限。
- 护栏阻止原因。

`POST /system/reinstall` 返回：

```json
{
  "jobId": "job_xxx",
  "status": "PENDING",
  "message": "重装系统任务已创建"
}
```

## 8. 任务类型

新增 Job 类型：

| Job type | 含义 |
| --- | --- |
| `INSTANCE_REINSTALL` | 重装系统 |
| `INSTANCE_SYSTEM_PRECHECK` | 系统设置预检查 |

新增 Job 阶段：

```text
PENDING
RUNNING
PRECHECKING
WAITING_OCI
WAITING_AGENT
VERIFYING
SUCCESS
FAILED
CANCELLED
ROLLBACK_REQUIRED
```

Job input 脱敏规则：

- 不保存明文密码。
- 不保存 PEM、SMTP 密码、Webhook secret。
- 重装系统不接受 cloud-init 或 SSH 公钥注入。

## 9. 审计日志

必须记录：

- `instance.system.reinstall.requested`
- `instance.system.reinstall.succeeded`
- `instance.system.reinstall.failed`

审计 detail 示例：

```json
{
  "instanceId": "ocid1.instance...",
  "profileId": "profile_xxx",
  "region": "ap-chuncheon-1",
  "mode": "reinstall",
  "imageId": "ocid1.image...",
  "preserveOldBootVolume": true,
  "sshPasswordChanged": false,
  "requestId": "opc-request-id",
  "workRequestId": "opc-work-request-id"
}
```

## 10. 前端实施拆分

第一阶段：

- `InstancesPage.tsx` 增加“系统设置”操作组。
- 新增 `InstanceSystemSettingsModal`。
- 增加重装系统表单。
- 重装系统只支持真实 OCI `UpdateInstance SourceDetails` 可表达的字段。
- 不显示“重置密码”或“重装时生成 root 密码”入口。
- 提交成功后关闭弹窗并刷新任务中心提示。

第二阶段：

- 增加 `/api/instances/{id}/system/options` 读取真实预检查。
- 重装系统镜像下拉复用 Launch Options 缓存，按 Shape 过滤 Image。
- 启动盘大小读取真实 boot volume。
- 预算提示展示重装后启动盘大小和性能变化。

第三阶段：

- 暂无密码重置相关规划；后续重点放在重装前备份、预算提示和任务可观测性。

## 11. 后端实施拆分

第一阶段：

- 在 domain 增加请求和结果模型。
- 在 API 层增加路由、权限校验、护栏校验。
- 在 store 增加 Job 创建、审计、通知持久化字段兼容。
- 在 OCI executor 增加正式 `ReinstallInstance` 方法，复用 smoke 中已验证过的 `UpdateInstance SourceDetails` 思路。

第二阶段：

- 增加 system options resolver：
  - `GetInstance`
  - `ListBootVolumeAttachments`
  - `GetBootVolume`
  - `ListImages` 或 Launch Options Shape/Image 缓存
- 增加启动盘备份或保留旧启动盘开关。
- 强化重装三阶段通知和邮件投递。

第三阶段：

- 暂无密码重置执行器规划。

## 12. 真实 OCI 验证计划

验证原则：

- 只操作平台创建的 `codex-` 前缀测试实例。
- 不动账号内其它实例。
- 每个验证项记录到 `docs/real-oci-validation.md`。
- 无法验证的项列入“未验证”，不能写成已完成。

建议验证实例：

```text
Shape: VM.Standard.E3.Flex
OCPU: 1
Memory: 1 GB
Boot volume: 50 GB
Image: Oracle Linux 或 Ubuntu 官方兼容镜像
Network: 平台创建的临时 VCN/Subnet
```

验证矩阵：

| 项目 | 期望结果 |
| --- | --- |
| 重装同镜像 | UpdateInstance 成功，实例回到 RUNNING 或 STOPPED |
| 重装新镜像 | 镜像兼容 Shape，任务成功 |
| 保留旧启动盘 | 旧启动盘仍存在且可识别 |
| 不保留旧启动盘 | 明确确认后旧系统盘不作为可恢复对象保留 |
| 邮件通知 | 邮件发送状态记录到通知 |
| SSH 密码状态 | 通知明确显示未生成 / 未变更 |

## 13. 官方依据

- [OCI CLI compute instance update](https://docs.oracle.com/iaas/tools/oci-cli/latest/oci_cli_docs/cmdref/compute/instance/update.html)：实例更新支持停机约束，`ALLOW_DOWNTIME` 可能在需要时重启实例，实例 OCID 保持不变。
- [Creating an Instance](https://docs.oracle.com/iaas/Content/Compute/Tasks/launchinginstance.htm)：创建实例需要镜像、Shape、网络、SSH 等参数，IPv6 需要 VCN 和 Subnet 支持。
- [Overview of the Compute Service](https://docs.oracle.com/iaas/Content/Compute/Concepts/computeoverview.htm)：Compute 实例、镜像、Shape、启动访问方式的基础约束。

## 14. 验收标准

- 实例列表和详情页出现“系统设置”入口。
- 点击后打开“重装系统”表单。
- 无权限用户看不到入口或提交时被 API 拦截。
- 重装系统必须创建 Job，不直接同步等待 OCI。
- 重装系统提交后弹窗关闭，任务中心出现新任务。
- 重装系统任务记录 request ID、work request ID、审计日志和通知。
- 启动盘大小不能小于当前大小。
- root 密码不会出现在普通 API 响应、Job 输出、审计明文、浏览器 localStorage。
- 当前版本不支持重装时注入 root 密码，UI 必须禁用，API 必须拒绝。
- 当前版本不提供“重置密码”入口或 API。
- 所有真实 OCI 验证结果更新到 `docs/real-oci-validation.md`。

## 15. 当前结论

当前实际实施“重装系统”：通过真实 OCI `UpdateInstance SourceDetails` 替换镜像，并支持启动盘大小和 VPUs/GB 后续调整。`UpdateInstance` 不能修改已启动实例的 `user_data` 或 `ssh_authorized_keys`，所以“重装时设置 root 密码”不作为平台能力。已有实例“重置密码”功能已移除，后续也不规划通过 Agent 或离线救援盘实现。
