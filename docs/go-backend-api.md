# Go 后端 API 说明

更新日期：2026-06-10

后端位于 `backend/`，使用 Go 标准库 HTTP server 和 Oracle 官方 Go SDK。真实 OCI 能力只在 `OCI_EXECUTION_MODE=oci` 下执行；`local` 模式仅用于工程联调。

## 启动

本地工程模式：
```powershell
cd backend
go run ./cmd/server
```

真实 OCI 模式：
```powershell
$env:OCI_EXECUTION_MODE="oci"
$env:OCI_TENANCY_OCID="ocid1.tenancy..."
$env:OCI_USER_OCID="ocid1.user..."
$env:OCI_FINGERPRINT="xx:xx:xx"
$env:OCI_PRIVATE_KEY_FILE="E:\path\to\key.pem"
$env:OCI_REGION="ap-chuncheon-1"
go run ./cmd/server
```

无 PostgreSQL 时的本地加密 Profile 仓库：
```powershell
$env:PROFILE_STORE_FILE=".runtime/profiles.json"
$env:PROFILE_KEY_ENCRYPTION_KEY="<32-byte-or-base64-32-byte-key>"
```

PostgreSQL 模式：
```powershell
$env:DATABASE_URL="postgres://oci_lifecycle:oci_lifecycle@localhost:5432/oci_lifecycle?sslmode=disable"
$env:PROFILE_KEY_ENCRYPTION_KEY="<32-byte-or-base64-32-byte-key>"
```

## 已实现接口

```text
GET    /api/health

GET    /api/oci/readiness
POST   /api/oci/validate-readonly
POST   /api/oci/smoke/e2-micro-create-delete
POST   /api/oci/smoke/e3-flex-lifecycle
POST   /api/oci/smoke/reinstall-instance
POST   /api/oci/smoke/cleanup

GET    /api/profiles
POST   /api/profiles
GET    /api/profiles/{id}
POST   /api/profiles/{id}/test
POST   /api/profiles/{id}/enable
POST   /api/profiles/{id}/disable
DELETE /api/profiles/{id}

GET    /api/templates
GET    /api/launch-options

GET    /api/instances
POST   /api/instances
GET    /api/instances/{id}
POST   /api/instances/{id}/actions
POST   /api/instances/{id}/reboot
POST   /api/instances/{id}/ip-tasks

GET    /api/jobs
GET    /api/jobs/{id}
POST   /api/jobs/{id}/cancel
POST   /api/jobs/{id}/retry

GET    /api/notifications
POST   /api/notifications/{id}/read
POST   /api/notifications/read-all

GET    /api/email/settings
PUT    /api/email/settings
POST   /api/email/test

GET    /api/automations
POST   /api/automations/tasks
```

## Profile

创建：
```http
POST /api/profiles
Content-Type: application/json
```

```json
{
  "name": "DEFAULT",
  "tenancyOcid": "ocid1.tenancy...",
  "userOcid": "ocid1.user...",
  "fingerprint": "01:a7:...",
  "defaultRegion": "ap-chuncheon-1",
  "privateKey": "",
  "privateKeyFile": "E:\\path\\to\\key.pem"
}
```

行为：
- API 响应不会返回 `privateKey`。
- `privateKey` 非空时会用 AES-GCM 加密存储。
- `privateKeyFile` 非空时只保存路径，运行环境必须能读取该路径。
- 保存后可由 OCI executor 按 `profileId` 解析并初始化 SDK client。

测试连接：
```http
POST /api/profiles/{id}/test
```

```json
{
  "region": "ap-chuncheon-1",
  "compartmentId": "ocid1.tenancy..."
}
```

真实调用：
- Identity `ListRegionSubscriptions`
- Compute `ListInstances`

## Launch Options

```http
GET /api/launch-options?profileId=profile-default&region=ap-chuncheon-1&compartmentId=ocid1...
```

支持查询参数：
```text
profileId
region
compartmentId
availabilityDomain
vcnId
shape
```

OCI 模式真实调用：
- Identity `ListRegionSubscriptions`
- Identity `ListCompartments`
- Identity `ListAvailabilityDomains`
- Compute `ListImages`
- Compute `ListShapes`
- VirtualNetwork `ListVcns`
- VirtualNetwork `ListSubnets`
- VirtualNetwork `ListPublicIps`

Subnet 选项会返回：
```json
{
  "id": "ocid1.subnet...",
  "label": "subnet-name",
  "public": false,
  "ipv6Enabled": true
}
```

## 创建实例

```http
POST /api/instances
Content-Type: application/json
```

```json
{
  "name": "oci-worker-01",
  "profileId": "profile-default",
  "region": "ap-chuncheon-1",
  "compartmentId": "ocid1.tenancy...",
  "availabilityAd": "",
  "imageId": "",
  "shape": "VM.Standard.E3.Flex",
  "ocpus": 1,
  "memoryGb": 1,
  "bootVolumeGb": 50,
  "assignPublicIp": false,
  "enableIpv6": false,
  "reservedPublicIp": "",
  "vcnId": "",
  "subnetId": "",
  "sshKey": "",
  "cloudInit": "",
  "tags": {
    "managedBy": "oci-lifecycle-platform"
  },
  "maxRetries": 0,
  "retryMode": "none",
  "retryMaxAttempts": 0,
  "retryDelayMinSeconds": 0,
  "retryDelayMaxSeconds": 0,
  "requireApproval": false,
  "snapshotBefore": true,
  "generateRootPassword": false,
  "notifyRootPassword": true
}
```

行为：
- `local` 模式只创建本地占位任务，不能作为 OCI 验收。
- `oci` 模式创建 Job，由 Worker 调用真实 `LaunchInstance`。
- `availabilityAd`、`imageId`、`subnetId` 为空时，后端会用真实 OCI API 自动发现第一个可用值。
- 成功后 Job 写入 OCI request id / work request id，并回写真实 instance OCID。
- `reservedPublicIp` 非空时，创建 VNIC 阶段不会分配 ephemeral public IP；实例进入 Running/Stopped 后解析 primary private IP，再调用 `UpdatePublicIp` 绑定 reserved public IP。
- 如果 reserved public IP 已绑定到其它 private IP，后端拒绝抢占。
- 当 `compartmentId` 等于 tenancy OCID 且 `generateRootPassword=true` 时，后端会生成随机 root 密码并合并到 cloud-init，同时创建敏感站内通知；`notifyRootPassword=true` 时尝试邮件推送。

## 实例生命周期

```http
POST /api/instances/{id}/actions
```

```json
{
  "action": "RESIZE",
  "graceful": true,
  "preserveBootVolume": true,
  "targetShape": "VM.Standard.E3.Flex",
  "targetOcpus": 2,
  "targetMemoryGb": 2,
  "targetBootVolumeGb": 60,
  "expandBootVolume": true,
  "snapshotBefore": true,
  "note": "resize from console"
}
```

支持动作：
```text
START
STOP
REBOOT
TERMINATE
RESIZE
```

OCI 模式下 `{id}` 必须是真实 OCI instance OCID，或已同步到 Store 的真实 instance id。

状态说明：
- `Terminating`：已发起终止或 OCI 返回 `TERMINATING`，UI 会禁用高风险操作。
- `Terminated`：OCI 返回 `TERMINATED`。

硬盘扩容说明：
- `expandBootVolume=true` 时，后端会解析启动盘 attachment，调用真实 `UpdateBootVolume`。
- 只允许扩容，不允许降盘。目标值小于当前启动盘大小会返回 `OCI_BOOT_VOLUME_CANNOT_SHRINK`。

## IP 管理

```http
POST /api/instances/{id}/ip-tasks
```

```json
{
  "mode": "enable-ipv6",
  "vnicId": "primary",
  "enableIpv6": true,
  "autoConfigureIpv6": true,
  "ipv6Strategy": "additive",
  "networkChangeMode": "additive",
  "routeTableMode": "merge_existing",
  "securityMode": "append",
  "allowIrreversibleVcnIpv6": true,
  "allowPublicIpv4Change": false,
  "openSshIpv6": true,
  "openHttpIpv6": false,
  "openHttpsIpv6": false,
  "snapshotBefore": true,
  "note": "enable IPv6 network and assign IPv6 to primary VNIC"
}
```

当前真实支持：
- 为 primary VNIC 或指定 VNIC 分配 IPv6。
- `assign_only`：只在已启用 IPv6 的 Subnet 上调用 `CreateIpv6`；子网未启用 IPv6 时返回 `OCI_IPV6_SUBNET_NOT_ENABLED`。
- `additive`：为 IPv4-only VCN/Subnet 添加 IPv6 CIDR，复用或创建 Internet Gateway，合并追加 `::/0 -> IGW`，追加 IPv6 Security List / NSG 规则，然后调用 `CreateIpv6`。
- `clone_route_table`：克隆当前 Route Table，追加 `::/0 -> IGW`，再将目标 Subnet 切换到新 Route Table。
- `replace_public_path`：高风险模式，必须设置 `allowPublicIpv4Change=true`；当前不默认执行会导致 IPv4 丢失的动作。
- 已存在 IPv6 时返回成功并标记 `noop=true`。

真实验证：
- `additive`、`clone_route_table`、`replace_public_path` 已通过真实 OCI 测试。
- Security List 和 NSG IPv6 规则追加已通过真实 OCI 测试。
- 当前测试环境中三种模式均保持公网 IPv4 不变。

真实调用：
- Compute `GetInstance`
- Compute `ListVnicAttachments`
- VirtualNetwork `GetVnic`
- VirtualNetwork `GetSubnet`
- VirtualNetwork `GetVcn`
- VirtualNetwork `AddIpv6VcnCidr`
- VirtualNetwork `AddIpv6SubnetCidr`
- VirtualNetwork `ListInternetGateways`
- VirtualNetwork `CreateInternetGateway` / `UpdateInternetGateway`
- VirtualNetwork `GetRouteTable`
- VirtualNetwork `UpdateRouteTable` / `CreateRouteTable` / `UpdateSubnet`
- VirtualNetwork `GetSecurityList` / `UpdateSecurityList`
- VirtualNetwork `ListNetworkSecurityGroupSecurityRules` / `AddNetworkSecurityGroupSecurityRules`
- VirtualNetwork `CreateIpv6`
- VirtualNetwork `GetIpv6`

尚未实现：
- 临时公网 IP 分配/释放。
- 已有实例上的保留公网 IP 绑定/解绑。
- DNS label 变更。

## 通知中心

```http
GET /api/notifications
GET /api/notifications?unread=true
POST /api/notifications/{id}/read
POST /api/notifications/read-all
```

通知字段：
```json
{
  "id": "notice-1",
  "title": "Root password generated: instance-01",
  "message": "...",
  "severity": "warning",
  "category": "credential",
  "read": false,
  "sensitive": true,
  "emailRequested": true,
  "emailSent": false,
  "emailError": "email delivery is disabled",
  "createdBy": "admin",
  "createdAt": "2026-06-10T00:00:00Z"
}
```

Root tenancy 随机 root 密码通知：
- 只在 `compartmentId == tenancyOcid` 且 `generateRootPassword=true` 时生成。
- 通知标记为 `sensitive=true`。
- Job 输出会隐藏敏感 cloud-init，不在 API 响应里返回 root 密码脚本。

## 邮件服务

```http
GET /api/email/settings
PUT /api/email/settings
POST /api/email/test
```

配置字段：
```json
{
  "enabled": true,
  "host": "smtp.example.com",
  "port": 587,
  "username": "user@example.com",
  "password": "",
  "from": "OCI Lifecycle <user@example.com>",
  "to": ["ops@example.com"],
  "useTls": false,
  "startTls": true,
  "passwordSet": true
}
```

行为：
- `GET /api/email/settings` 不返回 SMTP 密码，只返回 `passwordSet`。
- `PUT /api/email/settings` 中 `password` 为空时保留旧密码。
- 支持普通 SMTP、STARTTLS 和直接 TLS。
- 站内通知只有在 `emailRequested=true` 且 `enabled=true` 时发送邮件。

## 开发验证工具

创建/删除临时 IPv6 VCN/Subnet：
```powershell
cd backend
go run ./cmd/oci-testnet -action create -compartment $env:OCI_TENANCY_OCID
go run ./cmd/oci-testnet -action delete -vcn <vcn_ocid> -subnet <subnet_ocid>
```

该工具只用于真实测试，不会扫描或删除非指定资源。

### IPv6 原地编排 Smoke

用于验证“IPv4-only VCN/Subnet/实例是否能在保持原公网 IPv4 的情况下添加公网 IPv6”的真实 OCI 测试命令：

```bash
go run ./cmd/oci-ipv6-orch-smoke \
  -profile profile-default-2 \
  -mode auto \
  -nsg=false \
  -reserved-public-ip=false \
  -cleanup=true \
  -fallback=true \
  -timeout-minutes=75
```

行为：
- 创建临时 IPv4-only VCN、公网 Subnet、Internet Gateway、Route Table、Security List。
- 创建 `VM.Standard.E3.Flex` 最小测试实例：`1 OCPU / 1 GB / 50 GB boot volume`。
- 记录实例初始公网 IPv4。
- 调用真实 `ExecuteIPManagement`，优先执行 `additive` 原地双栈增配。
- 如果 `additive` 失败且 `-fallback=true`，再尝试 `clone_route_table` 和 `replace_public_path`。
- `-mode` 可强制执行 `additive`、`clone_route_table` 或 `replace_public_path`。
- `-nsg=true` 会创建测试 NSG、挂到 primary VNIC，并验证 IPv6 NSG 规则追加。
- `-reserved-public-ip=true` 会创建测试 reserved public IP，并验证创建实例时绑定该 reserved public IP。
- 记录最终公网 IPv4、IPv6 地址、VCN/Subnet IPv6 CIDR、路由和安全规则步骤。
- 默认清理测试实例、启动盘、Subnet、Route Table、Security List、Internet Gateway、VCN、测试 NSG、测试 reserved public IP。

2026-06-10 真实验证结论：
- `additive` 成功。
- `clone_route_table` 成功。
- `replace_public_path` 强制模式成功，本次未触发 IPv4 变化。
- NSG 规则追加成功。
- 创建实例时使用 reserved public IP 成功。
- 初始公网 IPv4 和最终公网 IPv4 一致，证明该路径可以保持原 IPv4。
- 测试资源清理完成。

## 当前限制

- PostgreSQL 真实环境迁移/恢复尚未在本机验证。
- Web 上传 PEM 建议改为 multipart。
- Work Request 精确恢复未实现。
- Template CRUD 未实现。
- Automation 调度器未实现。
- Audit 查询已实现；导出未实现。
- RBAC、审批、预算未实现。
- Webhook 通知已实现；真实外部 URL 投递待验证。
- Instance Pool / Autoscaling 未实现。

## 审计日志查询

```http
GET /api/audit-logs
```

可选查询参数：

- `actor`
- `action`
- `resourceType`
- `resourceId`
- `profileId`
- `status=success|failed`
- `limit`，默认 100，最大 500

返回：

```json
{
  "items": [
    {
      "id": 1,
      "actor": "admin",
      "action": "job.SUCCESS",
      "resourceType": "instance",
      "resourceId": "ocid1.instance...",
      "profileId": "profile-default",
      "region": "ap-chuncheon-1",
      "ociRequestId": "...",
      "ociWorkRequestId": "...",
      "requestPayload": {},
      "resultPayload": {},
      "createdAt": "2026-06-11T00:00:00Z"
    }
  ]
}
```

## Webhook 通知

```http
GET /api/webhook/settings
PUT /api/webhook/settings
POST /api/webhook/test
```

配置字段：

```json
{
  "enabled": true,
  "url": "https://example.com/webhook",
  "secret": "",
  "secretSet": true,
  "headers": {
    "X-Token": "example"
  }
}
```

行为：

- `GET /api/webhook/settings` 不返回 `secret` 明文，只返回 `secretSet`。
- `PUT /api/webhook/settings` 中 `secret` 为空时保留旧 secret。
- Webhook 使用 JSON `POST`。
- 配置 `secret` 时，请求头会包含 `X-OCI-Lifecycle-Signature: sha256=<hmac>`。
- 敏感站内通知不会把原文消息推送到 Webhook，只发送脱敏提示。

## 设置持久化

PostgreSQL 模式下，控制台级设置保存到 `app_settings`：

- `email`
- `webhook`

该表用于重启后恢复 Email/Webhook 设置。真实数据库恢复仍需在部署环境中专项验证。
