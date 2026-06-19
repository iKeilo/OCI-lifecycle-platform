# Real OCI Validation Log

## 2026-06-19 FreeX86Test 系统设置真实验证

测试环境：`http://10.0.0.142:34033` Docker 部署，`OCI_EXECUTION_MODE=oci`，通过测试服务器内已保存的 `profile-default` 执行真实 OCI SDK 调用。

测试对象：用户指定的既有实例 `FreeX86Test`。本次只操作该实例，未触碰账号内其它实例。

已验证：

- 历史版本中 `POST /api/instances/{id}/system/password-reset` 曾按预期返回 `501 PASSWORD_RESET_PATH_UNVERIFIED`。该端点现已从产品和 API 中移除。
- `POST /api/instances/{id}/system/reinstall` 使用兼容 Ubuntu 镜像提交真实 `UpdateInstance SourceDetails` 重装任务，OCI 返回 Work Request，任务最终 `SUCCESS`。
- 重装链路补齐并验证了启动盘 attachment 等待、启动盘 hydration 等待，以及 Blockstorage `GetBootVolume` 瞬时网络错误重试。
- 重装后实例仍保持 `Running`，公网 IPv4 和公网 IPv6 均保留，实例 Shape 仍为 `VM.Standard.E2.1.Micro`。
- 重装后启动盘性能被 OCI 重置为 `10 VPUs/GB`，随后通过实例升降级接口提交只调整启动盘性能的真实任务，恢复到 `110 VPUs/GB`，任务最终 `SUCCESS`。

本次发现并修复：

- 固定规格实例只调整启动盘容量/性能时，执行器不应调用 `UpdateInstance`。旧逻辑会对 `VM.Standard.E2.1.Micro -> VM.Standard.E2.1.Micro` 发起无意义规格更新，OCI 返回 `OCI_UPDATE_INSTANCE_FAILED / InvalidParameter`。
- 修复后 `executeResize` 会先判断 Shape / ShapeConfig 是否真的变化；如果没有变化，则跳过 Compute `UpdateInstance`，直接执行 Block Volume `UpdateBootVolume`。

最终状态：

- `FreeX86Test`：`Running`
- Shape：`VM.Standard.E2.1.Micro`
- 启动盘：`50 GB`
- 启动盘性能：`110 VPUs/GB`
- IPv6：已启用并可在实例列表显示

产品边界：

- 已有实例重置密码功能已移除，后续不规划 Agent / Run Command 或离线救援盘自动重置密码。
- 重装时注入 root 密码、cloud-init 或 SSH key 不作为平台能力。当前 OCI SDK 路径不支持修改已启动实例的 `user_data` / `ssh_authorized_keys`，所以产品必须继续拒绝该请求。

## 2026-06-10 IPv6 网络编排专项补充验证

测试环境：`http://10.0.0.142:24320` Docker 部署，`OCI_EXECUTION_MODE=oci`，通过 Web 保存的 `profile-default-2` 执行真实 OCI SDK 调用。

测试工具：`backend/cmd/oci-ipv6-orch-smoke`。本次为工具新增了强制模式和专项开关：

```bash
go run ./cmd/oci-ipv6-orch-smoke \
  -profile profile-default-2 \
  -mode clone_route_table|additive|replace_public_path \
  -nsg=true \
  -reserved-public-ip=true \
  -cleanup=true
```

已完成真实 OCI 专项：

- `clone_route_table`：创建 IPv4-only VCN/Subnet/实例后，自动添加 VCN IPv6 `/56`、Subnet IPv6 `/64`，克隆 Route Table，追加 `::/0 -> IGW`，并将目标 Subnet 切换到克隆路由表。结果 `verified=true`，`routeTableChanged=true`，`createdRouteTableId` 非空，公网 IPv4 保持不变，清理全部成功。
- NSG 场景：测试命令真实创建 NSG、绑定到 primary VNIC，再执行 IPv6 编排。结果 `nsgBound=true`、`nsgsChanged=true`，`ENSURE_NSG_IPV6` 成功追加 IPv6 egress、ICMPv6 Packet Too Big 和 SSH 入站规则；测试 NSG 已删除。
- 预留公网 IPv4：测试命令真实创建 reserved public IP，并通过 `LaunchInstanceFromRequest` 在实例启动后绑定到 primary private IP。结果 `reservedPublicIpUsed=true`，初始/最终公网 IPv4 与 reserved public IP 一致；测试 reserved public IP 已删除。
- `replace_public_path`：强制危险公网路径模式并显式允许公网 IPv4 变化。当前 OCI 测试路径下仍复用已有 Internet Gateway 并合并路由，结果 `verified=true`、`publicIpv4Changed=false`，公网 IPv4 未变化。

清理与残留检查：

- 以上每个专项均创建独立 `codex-ipv6-orch-smoke-*` 测试实例和临时网络资源。
- 每次测试结束后均执行 `TERMINATE`，`preserveBootVolume=false`。
- 临时 Subnet、Route Table、Security List、Internet Gateway、VCN、NSG、reserved public IP 均已清理。
- 后置只读检查结果：`nonTerminatedCodexIPv6Smoke=0`。

本次结论：

- 在当前 OCI 区域和公网子网路径下，IPv4-only VCN/Subnet 可以在保持当前公网 IPv4 的情况下原地增加公网 IPv6。
- `clone_route_table`、NSG 规则追加、reserved public IP 创建前绑定、`replace_public_path` 强制模式均已通过真实 OCI 验证。
- `replace_public_path` 目前并不会主动销毁/替换已有 IGW；它表示允许公网路径变更的高风险确认模式。在本次验证环境中没有触发 IPv4 变更。

## 2026-06-10 IPv4-only 网络原地添加 IPv6 真实验证

测试环境：`http://10.0.0.142:24320` Docker 部署，`OCI_EXECUTION_MODE=oci`，通过 Web 保存的 `profile-default-2` 执行真实 OCI SDK 调用。

测试工具：`backend/cmd/oci-ipv6-orch-smoke`。

测试资源：
- 测试实例：`codex-ipv6-orch-smoke-20260609-173955`
- Shape：`VM.Standard.E3.Flex`
- 配置：`1 OCPU / 1 GB RAM / 50 GB boot volume`
- 测试网络：由 smoke 命令临时创建 IPv4-only VCN、公网 Subnet、Internet Gateway、Route Table、Security List。

验证结果：
- `additive` 原地双栈增配成功。
- VCN 从 IPv4-only 成功添加 Oracle GUA IPv6 `/56`。
- Subnet 成功添加 IPv6 `/64`。
- 复用已有测试 Internet Gateway，未创建额外 IGW。
- Route Table 成功合并 `::/0 -> Internet Gateway`。
- Security List 成功追加 IPv6 egress、ICMPv6 和 SSH IPv6 入站规则。
- Primary VNIC 成功分配公网 IPv6：`2603:c024:f:ef00:0:3a09:33eb:a264`。
- 初始公网 IPv4：`168.110.122.181`。
- 最终公网 IPv4：`168.110.122.181`。
- 结论：本次真实 OCI 测试证明在当前区域/公网子网/IGW 路径下，可以在保持原公网 IPv4 不变的情况下，为 IPv4-only VCN/Subnet/实例原地添加公网 IPv6。
- `clone_route_table` / `replace_public_path` 未触发，因为 additive 模式已经成功。

清理结果：
- 测试实例已执行 `TERMINATE`，`preserveBootVolume=false`。
- 测试 Subnet、Route Table、Security List、Internet Gateway、VCN 均已删除，smoke 清理步骤全部 `verified=true`。
- 后置 `GET /api/instances?profileId=profile-default-2` 实时查询返回该测试实例状态为 `Terminated`，没有运行中的测试实例残留。

## 2026-06-10 IPv6 网络编排实施记录

状态：已实现；`additive` 原地双栈增配、`clone_route_table`、NSG 专项、保留公网 IP 创建前绑定和 `replace_public_path` 强制模式均已完成真实 OCI 端到端验证。

本次落地：

- IPv4-only VCN 自动添加 Oracle GUA IPv6 CIDR：调用 `AddIpv6VcnCidr`，并等待 VCN 重新读取得到 IPv6 CIDR。
- IPv4-only Subnet 自动添加 IPv6 `/64`：从 VCN IPv6 前缀内选择未被现有 Subnet 占用的 `/64`，调用 `AddIpv6SubnetCidr`。
- Internet Gateway 编排：优先复用已启用 IGW；如无可用 IGW，则创建带 `managedBy=codex` 标签的新 IGW。
- Route Table 编排：
  - `additive` / `merge_existing`：读取现有规则后合并追加 `::/0 -> Internet Gateway`。
  - `clone_route_table`：克隆现有路由规则，追加 `::/0`，再将目标 Subnet 切换到新 Route Table。
- Security 编排：追加 IPv6 egress、ICMPv6 Packet Too Big、可选 SSH/HTTP/HTTPS 入站规则；支持 Security List 和 VNIC 绑定的 NSG。
- VNIC IPv6 分配：网络编排完成后继续调用 `CreateIpv6`，并等待 IPv6 进入 `AVAILABLE`。
- Web UI：IP 管理弹窗新增“只添加 IPv6 / 原地双栈增配 / 克隆路由表 / 危险公网路径替换”。
- 安全确认：VCN IPv6 CIDR 按不可逆风险处理；危险公网路径模式必须显式允许 IPv4 公网 IP 变化。

本地验证：

```powershell
cd backend
go test ./...

cd ..
npm run build
```

真实 OCI 验证状态：

- 已验证：在专用 `codex-` 测试 VCN/Subnet 上执行 `additive` 模式，VCN `/56`、Subnet `/64`、IGW 复用、`::/0` 路由、安全规则和 VNIC IPv6 全部成功。
- 已验证：原临时 IPv4 公网 IP 在 `additive` 模式下保持不变。
- 已验证：测试实例、测试 Subnet、测试 Route Table、测试 Security List、测试 IGW、测试 VCN 已清理。
- 已验证：执行 `clone_route_table` 模式，目标 Subnet 成功切换到克隆路由表。
- 已验证：NSG 场景真实追加 IPv6 规则成功。

## 2026-06-09 追加验证记录

测试环境：`http://10.0.0.142:24320` Docker 部署，`OCI_EXECUTION_MODE=oci`，通过 Web 保存的 OCI Profile 执行真实 SDK 调用。

已验证：

- E3 Flex 测试实例创建：`VM.Standard.E3.Flex`，`1 OCPU / 1 GB / 50 GB boot volume`，测试实例名称使用 `codex-` 前缀。
- 启动盘扩容：通过 `POST /api/instances/{ocid}/actions` 提交 `RESIZE` 任务，`targetBootVolumeGb=60`，真实调用 `UpdateInstance`、`ListBootVolumeAttachments`、`GetBootVolume`、`UpdateBootVolume`，任务成功，结果包含 `bootVolumeExpanded=true`、`currentBootVolumeGb=50`、`targetBootVolumeGb=60`。
- 降盘边界：再次提交 `targetBootVolumeGb=50`，任务按预期失败，错误为 `OCI_BOOT_VOLUME_CANNOT_SHRINK`，确认“一旦扩容无法降盘”的限制已由后端执行器拦截。
- IPv4-only 后补 IPv6 历史错误路径：创建 `codex-ipv6-postadd-*` 测试实例后提交 `enableIpv6=true`、`autoConfigureIpv6=true`、`ipv6Strategy=replace_gateway` 的 IP 管理任务。当时自动 VCN/Subnet IPv6 CIDR 与网关编排尚未实现，因此按真实 OCI 状态失败，错误为 `OCI_IPV6_SUBNET_NOT_ENABLED`，且未改动当前 IPv4 公网 IP。
- 清理：本次创建的 E3/IPv6 测试实例均已 `TERMINATED`，终止任务使用 `preserveBootVolume=false`。

历史未验证/未实现记录：

- 自动为 IPv4-only VCN/Subnet 添加 IPv6 CIDR、复用 Internet Gateway、调整 route table/security list 并分配 VNIC IPv6 的 `additive` 路径已在 2026-06-10 完成真实 OCI 端到端验证。
- NSG 专项、`clone_route_table` 路径、`replace_public_path` 强制模式和创建实例时绑定 reserved public IP 已在 2026-06-10 完成真实 OCI 验证。
- 已有实例上的保留公网 IP 绑定、解绑、释放 UI 仍待实现。
- E3 全量 smoke 中 `SOFTSTOP` 和 reinstall 后 OCI 存在较长后台修改窗口，已将 smoke 等待放宽并避免重复 stop，但完整 smoke 仍需要后续长跑复验。

更新日期：2026-06-09

本文记录真实 OCI API 验证结果。为了避免仓库包含账号元数据，所有真实 OCID、请求 ID、指纹和 IP 均已脱敏。

## 验证原则

- 不使用假 API 结果作为验收。
- 无法真实验证的能力单独标记为“未验证”。
- 测试资源只使用 `codex-` 前缀或明确的测试名称。
- 测试结束后删除由本项目创建的临时实例和临时网络。
- 不触碰账号内其它实例。

## 已验证能力

### 启动盘容量与性能

结果：已验证接口与库存同步；真实性能调整未在既有实例上执行。

- 创建实例页面已将启动盘拆分为“容量 GB”和“性能 VPUs/GB”两个输入区域。
- 实例升降级页面已支持填写目标启动盘容量和目标启动盘性能，并展示调整后的每小时、每天、每月预算变化。
- 后端任务链已接入 `bootVolumeVpusPerGb` / `targetBootVolumeVpusPerGb`，创建实例时传入 OCI `BootVolumeVpusPerGB`，升降级时通过 `UpdateBootVolume.VpusPerGB` 调整性能。
- 真实 OCI 库存同步已验证能返回 `bootVolumeGb` 和 `bootVolumeVpusPerGb`；测试服务器当前运行实例返回 50 GB / 10 VPUs/GB 与 100 GB / 10 VPUs/GB。
- 价格估算采用 OCI 官方价格表口径：Block Volume Storage $0.0255/GB-month，Block Volume Performance Units $0.0017/VPU/GB-month。最终账单仍以 OCI Cost Analysis / Usage 报表为准。

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
- 已有实例上的保留公网 IP 绑定、解绑、释放 UI。
- Automation repository 与调度器。
- Audit 查询、筛选和导出。
- RBAC、审批流、预算护栏后端执行链。
- 预算管理 Web 入口和实施文档已新增；OCI Budgets/Cost Usage/自动降配/删机尚未做真实验证。
- 创建实例页切换 Shape 后自动刷新兼容 Image 已实现，尚未做真实 OCI 专项验证。
- 非 Flex / 非 E2 Micro Shape 的预算估算状态展示已实现；统一后端价格表覆盖尚未专项验证。
- 通知渠道：Webhook。
- SMTP 邮件真实投递：接口和配置页已落地，仍需有效 SMTP 凭证后专项验证。

## 本地验证命令

```powershell
cd backend
go test ./...

cd ..
npm run build
```

真实 OCI 验证必须在配置 OCI Profile 后手动执行，不应使用 mock 输出替代。

## 2026-06-11 校准记录

本次新增真实验证：

- SMTP 真实投递已验证：测试服务器 Docker 环境配置 SMTP 后，`POST /api/email/test` 返回 `verified=true`，测试邮件发送成功。仓库不保存 SMTP 密码。
- Root tenancy 随机 root 密码 VM 已验证：通过 Web/API 创建临时 E3 Flex 公网测试实例，系统生成 root 密码并创建敏感站内通知，通知记录显示 `emailRequested=true`、`emailSent=true`。
- SSH root 登录已验证：使用通知中的临时 root 密码登录测试实例成功。
- 清理已完成：测试 VM 已终止并删除启动盘，临时 VCN/Subnet/IGW/Route Table/Security List 已删除；残留检查未发现 active `codex-rootpwd-*` 实例。

本次新增本地验证：

```powershell
cd backend
go test ./...

cd ..
npm run build
```

本次新增已落地但仍需专项真实环境验证：

- `GET /api/audit-logs` 审计查询 API 与 Web 审计页面已落地；PostgreSQL 环境下需通过真实 DB 重启恢复测试校验查询结果。
- Webhook 设置、测试发送 API 和 Web 配置页已落地；需要用户提供真实 Webhook URL 后做投递验证。
- `app_settings` PostgreSQL 持久化表已落地，用于保存 Email/Webhook 控制台配置；仍需在真实 PostgreSQL 部署中验证重启恢复。

仍未完成或未专项验证：

- Instance Configuration / Instance Pool / Autoscaling 的真实 OCI executor。
- RBAC 多用户、审批流、预算护栏后端执行链。
- OCI Budgets 同步、Cost Usage 采集、预算候选 dry-run、自动降配和预算熔断删机真实验证。
- Shape -> Image 兼容刷新：至少验证 E3.Flex、A1.Flex、E2.1.Micro 和一个非 Flex Shape。
- 预算估算覆盖：验证 Flex、固定 Shape、Always Free、价格未接入四类展示状态。
- Template CRUD 与模板版本化。
- Automation repository、调度器、定时/指标策略真实执行。
- 审计导出。
