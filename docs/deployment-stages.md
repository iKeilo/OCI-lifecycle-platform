# 阶段化部署与实施状态

更新日期：2026-06-10

目标是建设一套 **OCI 机器生命周期控制平台**。所有影响 OCI 资源的操作都必须走真实 Oracle SDK/API、任务队列、审计记录和可验证的执行结果。本地模式只用于开发与 UI 验证，不能当作真实 OCI 验收。

## Stage 1：本地可运行骨架

状态：已完成。

已落地：

- React/Vite Web 控制台。
- Go HTTP API。
- 本地任务 Runner。
- 实例列表、创建任务、生命周期任务、IP 管理入口、任务中心。
- Profile 页面：添加、详情、测试、启用、禁用、删除。
- 创建实例页面：Profile、Region、Compartment、AD、Image、Shape、VCN、Subnet、Reserved IP 真实选项。
- 实例管理页面：启动、停止、重启、终止、升降级、IP 管理。
- 面板登录密码：后端 bcrypt、HttpOnly Cookie、前端登录页。

本地验证：

```powershell
cd backend
go test ./...

cd ..
npm run build
```

## Stage 2：Profile Repository 与密钥存储

状态：已完成本地加密文件仓库；PostgreSQL 环境仍待独立验证。

已落地：

- PostgreSQL sink 与迁移代码：`profiles`、`instances`、`jobs`、`audit_logs`。
- `PROFILE_KEY_ENCRYPTION_KEY` 支持 32 字节明文或 base64 32 字节 key。
- 内联 PEM 使用 AES-GCM 加密存储。
- 本地加密 Profile 文件仓库：`PROFILE_STORE_FILE`。
- 无 PostgreSQL 的机器也可以保存 Profile 并驱动真实 OCI executor。
- API 不返回私钥。

待验证：

- PostgreSQL migration、DB reload、Job/Instance 持久恢复需要在真实数据库环境验收。

## Stage 3：OCI SDK 执行器

状态：核心能力已真实验证。

已落地：

- `LocalExecutor` 与 `OCIExecutor` 分离。
- OCI 模式不回退成本地假成功。
- Profile resolver 按 `job.profileId` 解析 Web/API 保存的 Profile。
- 真实 `LaunchInstance`。
- 真实 `InstanceAction`：START、STOP、REBOOT、TERMINATE。
- 真实 `UpdateInstance`：Shape/OCPU/Memory resize。
- 真实重装：`UpdateInstance SourceDetails`。
- Job 写入 OCI request id / work request id。
- 失败保留真实 OCI 错误。

真实验证摘要：

- E3 Flex 1C/1G/50G 创建成功。
- STOP、START、REBOOT 成功。
- 重装成功。
- 升级到 2C/2G 成功。
- 降级回 1C/1G 成功。
- 最终停止、终止并删除启动盘成功。

具体 OCID 与请求 ID 已脱敏，避免仓库包含账号元数据。

## Stage 4：资源发现与 Web 控制台

状态：已落地并真实验证。

已落地：

- `GET /api/launch-options` 真实发现：
  - Region subscriptions
  - Compartments
  - Availability Domains
  - Images
  - Shapes
  - VCNs
  - Subnets
  - Reserved Public IPs
- Subnet 选项返回 `ipv6Enabled`。
- 创建实例页面提交 `POST /api/instances`。
- 实例列表在 OCI 模式下同步真实实例和 VNIC IP。

真实验证摘要：

- `POST /api/profiles/{id}/test` 成功。
- `GET /api/launch-options` 成功。
- `POST /api/instances` 到真实 `LaunchInstance` 成功。
- `GET /api/instances` 能发现已创建测试实例，并最终确认状态为 `Terminated`。

## Stage 5：真实 IP 管理

状态：IPv6 分配、IPv4-only VCN/Subnet 后补 IPv6、NSG 规则追加、克隆路由表、预留公网 IP 创建前绑定均已真实验证。

已落地：

- OCI 模式 `/api/instances/{id}/ip-tasks` 不再返回 501。
- 支持 `enableIpv6=true`。
- Worker 执行：
  - `GetInstance`
  - `ListVnicAttachments`
  - `GetVnic`
  - `GetSubnet`
  - `CreateIpv6`
  - `GetIpv6`
- 如果子网未启用 IPv6，返回真实 `OCI_IPV6_SUBNET_NOT_ENABLED`。
- `backend/cmd/oci-testnet` 可创建/删除 `codex-` 前缀临时 IPv6 VCN/Subnet。
- IPv4-only VCN/Subnet 自动 IPv6 网络编排：
  - `AddIpv6VcnCidr`
  - `AddIpv6SubnetCidr`
  - 复用或创建 Internet Gateway
  - 合并追加 `::/0 -> IGW`，或克隆 Route Table 后切换 Subnet
  - 追加 IPv6 Security List / NSG 规则
  - 最后为 primary VNIC 调用 `CreateIpv6`
- 创建实例时选择 Reserved Public IP：
  - 实例创建时不分配 ephemeral public IP
  - 实例进入 Running/Stopped 后解析 primary VNIC 和 primary private IP
  - 调用 `UpdatePublicIp` 将 reserved public IP 绑定到 primary private IP
  - 如果 reserved public IP 已绑定到其它 private IP，后端拒绝抢占
- Web IP 管理弹窗支持：
  - 只添加 IPv6
  - 原地双栈增配
  - 克隆路由表
  - 危险公网路径替换确认

真实验证摘要：

- 账号既有子网未启用 IPv6 的错误路径已验证。
- 临时 IPv6 VCN/Subnet 创建成功。
- 测试实例 primary VNIC 分配 IPv6 成功。
- 测试结束后实例、Subnet、VCN 均已删除。
- IPv4-only VCN/Subnet 后补 IPv6 的 `additive` 端到端流程已验证，原公网 IPv4 保持不变。
- `clone_route_table` 模式已验证，目标 Subnet 成功切换到克隆路由表。
- NSG 规则追加场景已验证，测试 NSG 已清理。
- Reserved Public IP 创建、绑定到新实例、随测试资源清理已验证。
- `replace_public_path` 强制模式已验证；当前实现不会主动销毁已有 IGW，本次测试未触发 IPv4 变化。

## Stage 6：部署、治理、自动化与审计

状态：部署脚本、站内通知和 SMTP 邮件配置已落地；治理和自动化仍待继续实现。

已落地：

- `scripts/install.sh` 一键安装脚本，默认仓库为 `https://github.com/iKeilo/OCI-lifecycle-platform.git`。
- systemd API 服务。
- nginx 静态前端与 `/api` 反代，或 Go 后端直接托管 SPA。
- 安装、更新、改密码、配置 OCI 环境、启停、重启、状态日志、备份、卸载。
- 站内通知中心：未读数量、通知列表、单条/全部已读。
- Root tenancy 创建实例时可生成随机 root 密码，并通过站内通知保存敏感通知。
- SMTP 邮件配置页和测试发送接口；站内通知可按 `emailRequested` 推送邮件。
- 实例状态拆分 `Terminating` / `Terminated`，终止中不再全部混用 `Terminated`。

尚未落地：

- Template CRUD 和版本化。
- Automation repository 和真实调度器。
- 定时策略、指标策略、容量重试策略。
- Work Request 精确恢复。
- Audit 查询、筛选、导出 API。
- RBAC、审批流、预算护栏。
- 通知渠道：Webhook。
- Instance Configuration / Instance Pool / Autoscaling。
- 已有实例上的保留公网 IP 绑定、解绑、释放操作页面。

## 当前优先级

1. 接 PostgreSQL 环境，验证 Profile/Job/Instance 重启恢复。
2. 做 Audit 查询页面和导出。
3. 做 Template CRUD，再接自动化调度器。
4. 补齐已有实例上的保留公网 IP 绑定、解绑、释放 UI。
5. 做 Webhook 通知渠道。
6. 做 Instance Configuration / Instance Pool / Autoscaling。

## 2026-06-11 Stage 6 增量更新

已新增落地：

- 审计查询 API：`GET /api/audit-logs`，支持 actor、action、resourceType、resourceId、profileId、status、limit 筛选。
- Web 审计页面：读取真实审计 API，不再展示演示审计数据。
- Webhook 通知通道：`GET/PUT /api/webhook/settings`、`POST /api/webhook/test`。
- 站内通知 Webhook 投递：普通通知发送完整消息；敏感通知只发送脱敏提示，不把 root 密码等内容推送到外部 Webhook。
- PostgreSQL `app_settings` 表：用于持久化 Email/Webhook 控制台配置。
- `.env.example`、`docker/.env.example`、`docker-compose.yml` 已加入 `WEBHOOK_ENABLED`、`WEBHOOK_URL`、`WEBHOOK_SECRET`。

已真实验证：

- SMTP 真实投递。
- Root tenancy 随机 root 密码通知邮件投递。
- 临时 root 密码 VM 的 SSH 登录。
- 测试 VM 和临时网络资源清理。

仍需专项验证：

- PostgreSQL `app_settings`、profiles、instances、jobs、audit_logs 在真实数据库中的重启恢复。
- Webhook 对真实外部 URL 的投递。
- Instance Configuration / Instance Pool / Autoscaling。
- RBAC 多用户、审批流、预算护栏。
- Automation repository 与调度器。
