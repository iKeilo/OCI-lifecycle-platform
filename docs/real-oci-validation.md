# Real OCI Validation Log

更新日期：2026-06-09

本文记录真实 OCI API 验证结果。为了避免仓库包含账号元数据，所有真实 OCID、请求 ID、指纹和 IP 均已脱敏。

## 验证原则

- 不使用假 API 结果作为验收。
- 无法真实验证的能力单独标记为“未验证”。
- 测试资源只使用 `codex-` 前缀或明确的测试名称。
- 测试结束后删除由本项目创建的临时实例和临时网络。
- 不触碰账号内其它实例。

## 已验证能力

### OCI Profile

结果：已验证。

- 使用用户提供的 OCI API signing key 和 PEM 私钥初始化 SDK。
- `POST /api/profiles` 可保存 Profile。
- API 返回值不包含 PEM 私钥明文。
- `POST /api/profiles/{id}/test` 可通过真实只读验证。

### Launch Options

结果：已验证。

- Region subscriptions
- Compartments
- Availability Domains
- Images
- Shapes
- VCNs
- Subnets
- Reserved Public IPs

### 创建实例

结果：已验证。

测试配置：

- Shape：`VM.Standard.E3.Flex`
- OCPU：1
- Memory：1 GB
- Boot Volume：50 GB
- Region：已脱敏
- Image：Oracle Linux 兼容镜像，由真实 Launch Options 发现

验证：

- `POST /api/instances` 生成 OCI launch job。
- Worker 调用真实 `LaunchInstance`。
- Job 记录真实 request id / work request id。
- `GET /api/instances` 可同步到该实例。

### 生命周期操作

结果：已验证。

- STOP
- START
- REBOOT
- REINSTALL
- RESIZE up：1C/1G -> 2C/2G
- RESIZE down：2C/2G -> 1C/1G
- Final STOP
- TERMINATE，并删除启动盘

### IPv6 网络

结果：已验证。

- 既有子网未启用 IPv6 时返回真实错误：`OCI_IPV6_SUBNET_NOT_ENABLED`。
- 创建临时 IPv6 VCN/Subnet。
- 为测试实例 primary VNIC 分配 IPv6。
- 删除测试实例、临时 Subnet、临时 VCN。

## 已清理资源

以下测试资源已删除或终止：

- 测试 E3 Flex 实例：已终止，启动盘已删除。
- 临时 IPv6 Subnet：已删除。
- 临时 IPv6 VCN：已删除。

## 未验证能力

- PostgreSQL migration 与持久恢复。
- Instance Pool / Autoscaling。
- 保留公网 IP 的绑定、解绑、释放。
- Automation repository 与调度器。
- Audit 查询、筛选和导出。
- RBAC、审批流、预算护栏。
- 通知渠道：Webhook、邮件。

## 本地验证命令

```powershell
cd backend
go test ./...

cd ..
npm run build
```

真实 OCI 验证必须在配置 OCI Profile 后手动执行，不应使用 mock 输出替代。
