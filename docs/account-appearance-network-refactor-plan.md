# 账号设置、外观背景与网络管理改造方案

本文档只做方案整理，不直接实施功能代码。确认方案后再进入开发。

## 目标范围

本轮改造围绕四件事：

1. 删除左侧导航和路由中的对象存储板块，继续保持项目只围绕 OCI 机器生命周期、网络、任务和治理能力展开。
2. 将顶部红框位置的设置图标改为“账号设置”，支持设置账号头像和修改面板密码。修改密码必须提交原密码。
3. 系统设置增加背景设置，完善白天/黑夜背景能力；顶部主题按钮要能立即切换白天/黑夜，并持久化。
4. 增加网络管理，用于管理预留公网 IP、临时公网 IP、私网 IP、IPv6、VNIC、VCN/Subnet 等网络资源；能力边界按 OCI 官方 SDK/API 设计。

## 当前项目整理

| 区域 | 当前状态 | 需要调整 |
| --- | --- | --- |
| 左侧导航 | `src/app/navigation.tsx` 中有 `对象存储`，路径为 `/object-storage`。 | 删除对象存储导航项；不要再保留对象存储入口。 |
| 路由 | `src/app/App.tsx` 中 `/object-storage` 使用 `PlaceholderPage`。 | 删除 `/object-storage` 路由；新增 `/account` 与 `/network`。 |
| 顶部设置图标 | `src/components/AppShell.tsx` 中 `Settings2` 只是按钮，没有功能。 | 改成账号设置入口，建议跳转 `/account`。 |
| 顶部头像 | 当前是静态 `A`，无点击行为。 | 头像显示账号设置中的头像；点击也可进入 `/account`。 |
| 顶部登出 | `LogOut` 按钮已调用 `onLogout`，后端已有 `/api/auth/logout`。 | 增加 loading/错误反馈；登出后明确回到登录页。 |
| 主题按钮 | `Moon` 只是按钮，没有主题状态。 | 增加 light/dark 状态、localStorage 快速生效、API 持久化。 |
| 系统设置页 | `src/pages/SettingsPage.tsx` 仍是四张说明卡。 | 增加外观/背景设置区，后续扩展安全、任务、自动化、通知配置。 |
| 用户与权限 | `src/pages/UsersPage.tsx` 是角色说明页。 | 保留为 RBAC/用户管理，不承担个人账号设置。 |
| 认证后端 | `backend/internal/auth` 支持登录、session、登出；密码来自 `PANEL_PASSWORD_HASH` 或 `PANEL_PASSWORD`。 | 增加改密码接口，原密码校验通过后更新 hash，并持久化。 |
| 设置持久化 | PostgreSQL 已有 `app_settings`，当前用于邮件和 Webhook。默认 Docker 文件模式主要持久化 profile。 | 新增通用平台设置持久化；文件模式也要支持账号/外观设置。 |
| IP 能力 | 实例页已有 IP 管理弹窗和一键 IPv6 任务入口。 | 拆出独立网络管理页，统一管理预留 IP 和网络资源；实例页只保留实例相关快捷入口。 |

## UI 层级设计

### 导航调整

建议左侧导航改为：

```text
资源运营
  概览
  实例管理
  创建实例
  模板管理
  资源池
  网络管理
  自动化规则

平台治理
  账号与密钥
  任务中心
  监控告警
  审计日志
  用户与权限

OCI 服务
  邮件服务

系统
  安全护栏
  通知设置
  系统设置
```

说明：

- `对象存储` 删除。
- `网络管理` 建议放在资源运营，因为它直接服务实例创建、IP 绑定、IPv6、VCN/Subnet。
- 顶部红框位置不放系统设置，而放个人账号设置。

### 顶部栏调整

顶部右侧建议从左到右：

```text
语言 | 主题切换 | 刷新 | 账号设置 | 通知 | 登出 | 头像
```

交互规则：

- `主题切换`：点击立即切换白天/黑夜；图标随状态变化。
- `账号设置`：点击进入 `/account`。
- `头像`：显示账号头像；点击也进入 `/account`。
- `登出`：点击后调用 `/api/auth/logout`，成功后返回登录页；失败也清理前端认证状态并提示。

## 账号设置设计

### 页面路径

```text
/account
```

页面名称：账号设置。

### 页面结构

```text
账号资料
  头像预览
  上传头像
  使用首字母头像
  显示名称
  保存资料

修改密码
  原密码
  新密码
  确认新密码
  修改密码

安全提示
  当前登录状态
  最近一次修改时间
  退出登录
```

### 头像规则

- 支持 `png/jpeg/webp`。
- 建议前端限制上传后压缩到 256 KB 以内。
- MVP 可以先存储为 base64 data URL；后续如头像变大，再拆为本地文件或对象路径。
- 如果没有头像，显示首字母头像，默认 `A`。

### 改密码规则

接口必须要求：

```json
{
  "currentPassword": "old-password",
  "newPassword": "new-password"
}
```

校验规则：

- 原密码必须通过 `auth.Manager.VerifyPassword`。
- 新密码至少 8 位。
- 新密码不能等于原密码。
- 修改成功后建议清除当前 session，要求重新登录；也可以保留当前 session，但要写入审计日志。推荐清除 session。

### 后端接口建议

```text
GET  /api/account
PUT  /api/account/profile
POST /api/account/password
```

数据模型：

```json
{
  "displayName": "Admin",
  "avatarType": "initial | image",
  "avatarInitial": "A",
  "avatarDataUrl": "data:image/webp;base64,...",
  "updatedAt": "2026-06-11T00:00:00Z"
}
```

密码 hash 持久化建议：

- 新增 `PanelSecuritySettings`，保存 `panelPasswordHash` 和 `passwordUpdatedAt`。
- PostgreSQL 模式保存到 `app_settings`。
- 文件模式新增 `/data/app-settings.json` 或扩展当前文件 sink，让默认 Docker 部署也能持久化改密码。

## 系统设置与背景设计

### 页面路径

```text
/settings
```

### 新增外观设置区

```text
外观
  默认主题：跟随系统 / 白天 / 黑夜
  白天背景：默认渐变 / 自定义图片 / 自定义颜色
  黑夜背景：默认渐变 / 自定义图片 / 自定义颜色
  毛玻璃强度
  背景模糊
  恢复默认
```

### 顶部主题按钮

顶部按钮只做快速切换：

- 当前为白天时，点击切到黑夜。
- 当前为黑夜时，点击切到白天。
- 写入 localStorage，立即改变 CSS。
- 后端可异步保存到 `/api/settings/appearance`。

### 前端实现建议

CSS 使用全局属性：

```text
html[data-theme="light"]
html[data-theme="dark"]
```

背景使用 CSS 变量：

```text
--app-bg
--app-bg-image
--glass-opacity
--glass-blur
```

### 后端接口建议

```text
GET /api/settings/appearance
PUT /api/settings/appearance
```

模型：

```json
{
  "theme": "light | dark | system",
  "lightBackground": {
    "type": "default | color | image",
    "value": ""
  },
  "darkBackground": {
    "type": "default | color | image",
    "value": ""
  },
  "glassOpacity": 0.66,
  "backgroundBlur": 24
}
```

## 网络管理设计

### 页面路径

```text
/network
```

### 页面结构

```text
网络总览
  VCN 数量
  Subnet 数量
  预留公网 IP 数量
  已绑定公网 IP 数量
  IPv6 地址数量

预留公网 IP
  列表
  创建预留公网 IP
  绑定到实例主私网 IP
  解绑
  删除
  标签管理

公网 IP 绑定
  按实例/VNIC/Private IP 查看
  绑定预留 IP
  替换临时 IP
  释放临时 IP

私网 IP
  按 VNIC 查看 Private IP
  创建 secondary private IP
  删除 secondary private IP
  主私网 IP 只读显示

IPv6
  VCN IPv6 状态
  Subnet IPv6 CIDR 状态
  VNIC IPv6 列表
  分配 IPv6
  删除 IPv6
  一键补齐 VCN/Subnet/路由/安全规则

VCN / Subnet / 网关
  VCN 列表
  Subnet 列表
  Internet Gateway
  Route Table
  NSG / Security List
```

### OCI 官方能力映射

| 功能 | OCI 能力 | 说明 |
| --- | --- | --- |
| 预留公网 IP 列表 | `ListPublicIps` / CLI `network public-ip list` | 官方文档说明 reserved public IP 用 `scope=REGION` 与 `lifetime=RESERVED` 筛选。 |
| 临时公网 IP 列表 | `ListPublicIps` | 临时 IP 可按 `EPHEMERAL`、`REGION` 或 `AVAILABILITY_DOMAIN` 区分。 |
| 创建预留公网 IP | `CreatePublicIp` | 创建区域级 reserved public IP，可选择 public IP pool。 |
| 绑定/解绑公网 IP | `UpdatePublicIp` | 绑定到 private IP 或移除绑定；高风险操作要进入任务中心。 |
| 删除预留公网 IP | `DeletePublicIp` | 删除前必须确认未绑定。 |
| 私网 IP 管理 | `ListPrivateIps` / `CreatePrivateIp` / `UpdatePrivateIp` / `DeletePrivateIp` | secondary private IP 可管理；primary private IP 只读。 |
| IPv6 管理 | `CreateIpv6` / `ListIpv6s` / `UpdateIpv6` / `DeleteIpv6` | OCI 文档说明 IPv6 可分配给 IPv6-enabled VCN/Subnet 中的 VNIC。 |
| Subnet 增加 IPv6 CIDR | `AddIpv6SubnetCidr` | 官方 CLI 文档说明该操作用于给 subnet 增加 IPv6 prefix，并通过 Work Request 跟踪。 |
| 安全规则 | NSG / Security List API | 官方文档说明 NSG 与 Security List 支持 IPv4 和 IPv6 安全规则。 |

### 网络管理后端接口建议

```text
GET  /api/network/summary
GET  /api/network/public-ips
POST /api/network/public-ips
POST /api/network/public-ips/{id}/assign
POST /api/network/public-ips/{id}/unassign
DELETE /api/network/public-ips/{id}

GET  /api/network/private-ips
POST /api/network/private-ips
DELETE /api/network/private-ips/{id}

GET  /api/network/ipv6
POST /api/network/ipv6
DELETE /api/network/ipv6/{id}

GET  /api/network/vcns
GET  /api/network/subnets
GET  /api/network/vnics
POST /api/network/subnets/{id}/ipv6-cidr
```

所有写操作都建议创建 Job：

```text
PENDING -> RUNNING -> WAITING_OCI -> VERIFYING -> SUCCESS / FAILED / ROLLBACK_REQUIRED
```

这样可以复用现有任务中心、审计日志和通知系统。

### 网络管理安全护栏

- 删除预留 IP、释放公网 IP、替换绑定必须二次确认。
- 绑定公网 IP 时必须展示当前 private IP、VNIC、实例名、区域、compartment。
- IPv6 自动补齐 VCN/Subnet/Route/Security 时必须展示不可逆提示。
- 对路由表和安全规则的写操作必须记录变更前后 diff。
- 不允许跨 profile/region 混绑资源。
- 任务失败必须保留 OCI request id / work request id。

## 实施阶段

### 阶段 1：UI 结构清理

- 删除对象存储导航和路由。
- 新增 `/account` 页面壳。
- 新增 `/network` 页面壳。
- 顶部设置图标跳转账号设置。
- 头像点击跳转账号设置。
- 顶部登出按钮增加 loading 与明确回登录页。

### 阶段 2：账号设置落地

- 增加账号资料模型。
- 增加账号设置 API。
- 增加改密码 API，必须提交原密码。
- 文件模式和 PostgreSQL 模式都支持持久化。
- 增加后端测试：原密码错误、新密码太短、成功修改、修改后旧密码不可用。

### 阶段 3：外观与背景落地

- 增加 appearance settings。
- 顶部主题按钮可切换白天/黑夜。
- 系统设置页支持白天/黑夜背景配置。
- 背景配置 localStorage 立即生效，后端异步持久化。
- 增加前端构建验证和基础交互验证。

### 阶段 4：网络管理 MVP

- 先做只读发现：VCN、Subnet、VNIC、Private IP、Public IP、IPv6。
- 增加预留公网 IP 列表、创建、删除。
- 增加绑定/解绑 reserved public IP 到 private IP。
- 写操作进入任务中心。
- 更新审计日志。

### 阶段 5：网络管理高级能力

- secondary private IP 管理。
- IPv6 分配/删除。
- 一键补齐 IPv6 网络能力迁移到网络管理页。
- NSG/Security List 规则可视化和变更 diff。
- Route Table / Internet Gateway 检查与修复。

## 待确认问题

1. 账号设置页路径使用 `/account` 还是 `/account-settings`？推荐 `/account`。
2. 网络管理放在“资源运营”还是“OCI 服务”？推荐“资源运营”。
3. 头像是否允许上传图片，还是第一版只做首字母头像？推荐第一版支持图片但限制 256 KB。
4. 修改密码成功后是否强制重新登录？推荐强制重新登录。
5. 背景图片是否允许 base64 存储？推荐 MVP 支持 base64 小图，限制大小，后续再做文件存储。

## 官方依据

- OCI Public IP CLI 文档说明 `publicIp` 是 Public IP 的 API 表示，并区分 ephemeral 与 reserved；列表可按 `scope` 和 `lifetime` 筛选 reserved public IP。参考：[public-ip list](https://docs.oracle.com/iaas/tools/oci-cli/latest/oci_cli_docs/cmdref/network/public-ip/list.html)。
- OCI IPv6 文档说明 IPv6 可以分配给 IPv6-enabled VCN/Subnet 中的 VNIC。参考：[network ipv6](https://docs.oracle.com/iaas/tools/oci-cli/latest/oci_cli_docs/cmdref/network/ipv6.html)。
- OCI Subnet IPv6 CIDR 文档说明可给 subnet 增加 IPv6 prefix，并通过 Work Request 跟踪。参考：[subnet add-ipv6-subnet-cidr](https://docs.oracle.com/iaas/tools/oci-cli/latest/oci_cli_docs/cmdref/network/subnet/add-ipv6-subnet-cidr.html)。
- OCI Security Lists 文档说明 Security List 与 NSG 规则支持 IPv4 和 IPv6。参考：[Security Lists](https://docs.oracle.com/iaas/Content/Network/Concepts/securitylists.htm)。

