# Go 后端 API 说明

更新日期：2026-06-09

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
  "vcnId": "",
  "subnetId": "",
  "sshKey": "",
  "cloudInit": "",
  "tags": {
    "managedBy": "oci-lifecycle-platform"
  },
  "maxRetries": 0,
  "requireApproval": false,
  "snapshotBefore": true
}
```

行为：
- `local` 模式只创建本地占位任务，不能作为 OCI 验收。
- `oci` 模式创建 Job，由 Worker 调用真实 `LaunchInstance`。
- `availabilityAd`、`imageId`、`subnetId` 为空时，后端会用真实 OCI API 自动发现第一个可用值。
- 成功后 Job 写入 OCI request id / work request id，并回写真实 instance OCID。

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

## IP 管理

```http
POST /api/instances/{id}/ip-tasks
```

```json
{
  "mode": "enable-ipv6",
  "vnicId": "primary",
  "enableIpv6": true,
  "snapshotBefore": true,
  "note": "assign IPv6 to primary VNIC"
}
```

当前真实支持：
- 为 primary VNIC 或指定 VNIC 分配 IPv6。
- 子网未启用 IPv6 时返回 `OCI_IPV6_SUBNET_NOT_ENABLED`。
- 已存在 IPv6 时返回成功并标记 `noop=true`。

真实调用：
- Compute `GetInstance`
- Compute `ListVnicAttachments`
- VirtualNetwork `GetVnic`
- VirtualNetwork `GetSubnet`
- VirtualNetwork `CreateIpv6`
- VirtualNetwork `GetIpv6`

尚未实现：
- 临时公网 IP 分配/释放。
- 保留公网 IP 绑定/解绑。
- DNS label 变更。

## 开发验证工具

创建/删除临时 IPv6 VCN/Subnet：
```powershell
cd backend
go run ./cmd/oci-testnet -action create -compartment $env:OCI_TENANCY_OCID
go run ./cmd/oci-testnet -action delete -vcn <vcn_ocid> -subnet <subnet_ocid>
```

该工具只用于真实测试，不会扫描或删除非指定资源。

## 当前限制

- PostgreSQL 真实环境迁移/恢复尚未在本机验证。
- Web 上传 PEM 建议改为 multipart。
- Work Request 精确恢复未实现。
- Template CRUD 未实现。
- Automation 调度器未实现。
- Audit 查询/导出未实现。
- RBAC、审批、预算、通知未实现。
- Instance Pool / Autoscaling 未实现。

