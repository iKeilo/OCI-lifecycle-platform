# 阶段化部署与实施状态

更新日期：2026-06-09

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

状态：IPv6 分配已落地并真实验证；保留公网 IP 绑定/释放仍待实现。

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

真实验证摘要：

- 账号既有子网未启用 IPv6 的错误路径已验证。
- 临时 IPv6 VCN/Subnet 创建成功。
- 测试实例 primary VNIC 分配 IPv6 成功。
- 测试结束后实例、Subnet、VCN 均已删除。

## Stage 6：部署、治理、自动化与审计

状态：部署脚本已落地；治理和自动化仍待继续实现。

已落地：

- `scripts/install.sh` 一键安装脚本，默认仓库为 `https://github.com/iKeilo/OCI-lifecycle-platform.git`。
- systemd API 服务。
- nginx 静态前端与 `/api` 反代，或 Go 后端直接托管 SPA。
- 安装、更新、改密码、配置 OCI 环境、启停、重启、状态日志、备份、卸载。

尚未落地：

- Template CRUD 和版本化。
- Automation repository 和真实调度器。
- 定时策略、指标策略、容量重试策略。
- Work Request 精确恢复。
- Audit 查询、筛选、导出 API。
- RBAC、审批流、预算护栏。
- 通知渠道：Webhook、邮件。
- Instance Configuration / Instance Pool / Autoscaling。
- 公网 IP、保留公网 IP 的绑定与释放。

## 当前优先级

1. 在测试服务器完成一键安装、登录、API 探活、服务重启验证。
2. 创建 GitHub 仓库并推送脱敏代码。
3. 接 PostgreSQL 环境，验证 Profile/Job/Instance 重启恢复。
4. 补齐真实公网 IP/保留公网 IP 管理。
5. 做 Audit 查询页面和导出。
6. 做 Template CRUD，再接自动化调度器。
