# 项目目录索引

本项目是 **OCI 机器生命周期控制平台**，当前只面向 Oracle Cloud Infrastructure。代码按“Web 控制台、Go 后端、OCI 执行器、持久化、部署发布、实施文档”分层组织。

## 根目录

| 路径 | 说明 |
| --- | --- |
| `backend/` | Go 后端服务，包含 API、认证、任务执行、OCI SDK 编排、通知、审计、数据库持久化。 |
| `src/` | React + Vite 前端控制台。 |
| `docs/` | 产品设计、实施文档、部署文档、真实 OCI 验证记录、目录索引。 |
| `scripts/` | 本地部署、真实 OCI 验证、清理和安装辅助脚本。 |
| `docker/` | Docker 环境变量示例和容器部署相关文件。 |
| `references/` | UI 参考素材，不参与运行。 |
| `.github/workflows/` | GitHub Actions，用于镜像构建和发布。 |
| `Dockerfile` | 多阶段镜像构建入口。 |
| `docker-compose.yml` | Docker 版部署编排，默认包含 app 与 PostgreSQL。 |
| `panel_install.sh` | Docker 一键安装入口。 |
| `panel_linux_install.sh` | 原生 Linux/systemd 一键安装入口。 |
| `README.md` | 项目说明和快速开始。 |
| `.env.example` | 根目录环境变量示例。 |

## 后端目录

| 路径 | 说明 |
| --- | --- |
| `backend/cmd/server/` | Web/API 服务启动入口，负责配置加载、存储初始化、任务 runner 启动、静态前端托管。 |
| `backend/cmd/panel-password/` | 面板密码 bcrypt hash 生成工具。 |
| `backend/cmd/oci-testnet/` | 真实 OCI 实例生命周期专项验证工具。 |
| `backend/cmd/oci-ipv6-orch-smoke/` | IPv6 网络编排专项验证工具。 |
| `backend/cmd/oci-rootpwd-testnet/` | root 密码实例、邮件通知、SSH 登录专项验证工具。 |
| `backend/cmd/oci-firewall-smoke/` | 真实 OCI 防火墙/安全列表端口规则专项验证工具。 |
| `backend/internal/api/` | HTTP API、路由、请求校验、响应、认证中间件。 |
| `backend/internal/auth/` | 面板登录、密码校验、session 管理。 |
| `backend/internal/config/` | 环境变量加载，覆盖 OCI、SMTP、Webhook、数据库、安全配置。 |
| `backend/internal/db/` | PostgreSQL schema、迁移和持久化实现。 |
| `backend/internal/domain/` | 领域模型：Profile、Instance、Job、Notification、Audit、Budget、Network、Firewall 等。 |
| `backend/internal/fileprofile/` | 文件存储 fallback 与旧 Profile 数据迁移来源。 |
| `backend/internal/jobs/` | 任务 runner、local executor、真实 OCI executor。 |
| `backend/internal/lifecyclenotify/` | 实例生命周期通知组装，例如重装系统通知。 |
| `backend/internal/notify/` | SMTP 邮件与 Webhook 发送器。 |
| `backend/internal/oci/` | OCI SDK 客户端、实例生命周期、启动选项缓存、网络/IP/IPv6、防火墙、安全验证。 |
| `backend/internal/profileconfig/` | OCI config 与 PEM 私钥解析。 |
| `backend/internal/store/` | 内存状态与持久化 sink 的协调层。 |

## 前端目录

| 路径 | 说明 |
| --- | --- |
| `src/main.tsx` | 前端入口。 |
| `src/app/App.tsx` | 应用路由与认证状态入口。 |
| `src/app/navigation.tsx` | 左侧导航分组与页面入口定义。 |
| `src/app/ociContext.ts` | 当前 OCI Profile 与区域上下文。 |
| `src/components/` | 通用组件：AppShell、PageHeader、AsyncState、MetricCard、StatusPill。 |
| `src/pages/DashboardPage.tsx` | 总览页面。 |
| `src/pages/ProfilesPage.tsx` | OCI Profile 添加、测试连接、启用/禁用、删除。 |
| `src/pages/InstancesPage.tsx` | 实例管理、生命周期操作、IP/IPv6、防火墙、系统重装入口。 |
| `src/pages/CreateInstancePage.tsx` | 创建实例向导、Launch Options、预算估算、重试模式。 |
| `src/pages/TemplatesPage.tsx` | 模板管理列表。模板只保存创建实例表单预输入，不直接调用 OCI API。 |
| `src/pages/TemplateEditorPage.tsx` | 模板创建/编辑页面，复用创建实例的真实选项上下文。 |
| `src/pages/NetworkPage.tsx` | 网络管理，包含预留公网 IP、IPv6 等入口。 |
| `src/pages/BudgetManagementPage.tsx` | 预算管理、影响范围选择、超预算动作。 |
| `src/pages/JobsPage.tsx` | 任务中心与终态任务清理。 |
| `src/pages/NotificationsPage.tsx` | 站内通知列表、邮件状态展示、删除通知。 |
| `src/pages/EmailPage.tsx` | SMTP 与 Webhook 通知通道配置。 |
| `src/pages/AuditPage.tsx` | 审计日志查询、筛选、详情、导出。 |
| `src/pages/UsersPage.tsx` | 用户与权限管理。 |
| `src/pages/GuardrailsPage.tsx` | 安全护栏配置。 |
| `src/pages/SettingsPage.tsx` | 系统设置、外观、语言、背景。 |
| `src/pages/AccountPage.tsx` | 账号头像、资料、密码修改。 |
| `src/services/api.ts` | 前端 API client 与类型定义。 |
| `src/styles.css` | 全局样式、布局、表格、弹窗、响应式与移动端样式。 |

## 数据持久化

| 数据 | 默认存储 |
| --- | --- |
| OCI Profile 与 PEM 私钥 | PostgreSQL，私钥字段加密。 |
| SMTP、Webhook、账号、外观、预算设置 | PostgreSQL `app_settings`。 |
| 站内通知与邮件发送状态 | PostgreSQL `notifications`。 |
| 任务、实例缓存、审计日志、模板 | PostgreSQL。 |
| Docker 数据卷 | `oci-lifecycle-platform-postgres-data` 与 `oci-lifecycle-platform-profile-data`。 |
| 无 PostgreSQL fallback | `PROFILE_STORE_FILE` 指定的本地加密文件，仅用于开发/迁移。 |

## 部署入口

Docker 一键安装：

```bash
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh)
```

原生 Linux/systemd 一键安装：

```bash
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_linux_install.sh)
```

本地开发：

```powershell
cd backend
go run ./cmd/server

cd ..
npm install
npm run dev
```

## 验证入口

```powershell
cd backend
go test ./...

cd ..
npm run build
```

真实 OCI 验证必须使用有效 OCI Profile 和 PEM 私钥。不能用 local executor 或 mock 输出冒充真实通过。验证记录维护在 `docs/real-oci-validation.md`。

## 不应提交到仓库

以下内容不得进入 GitHub：

- `.env`、`.env.*` 中的真实环境变量。
- OCI PEM 私钥、API key、fingerprint 对应私钥文件。
- SMTP 密码、Webhook secret、面板明文密码。
- `profiles.json`、服务器 `/etc/oci-lifecycle-platform/*.env`。
- `node_modules/`、`dist/`、`.runtime/`、`.codegraph/`。
- `*.log`、临时构建产物、真实测试导出的敏感结果。

## 当前功能状态

| 模块 | 状态 |
| --- | --- |
| PostgreSQL 持久化 | Docker 默认启用，覆盖 Profile、设置、通知、任务、审计、模板。 |
| 任务中心 | 已持久化，支持启动恢复、重试、取消、清除终态任务。 |
| 通知系统 | 已支持数据库持久化、邮件状态记录、删除通知。 |
| 审计日志 | 已支持数据库查询、筛选、详情、JSON/CSV 导出。 |
| 用户与权限 | 已支持本地 RBAC、用户密码 hash、角色权限、Profile/区域/Compartment 范围限制。 |
| 安全护栏 | 已支持区域、容量、启动盘、重试、批量公网 IP、删机删盘、IPv6 网络改造等 API 层拦截。 |
| 模板管理 | 已恢复，定位为“创建实例预输入”，模板本身不调用 OCI API。 |
| 防火墙管理 | 已加入实例卡片入口，支持规则编辑、宽规则强制删除、统一应用变更。 |
| SMTP 真实投递 | 配置与持久化已落地，真实投递需在目标环境用有效 SMTP 专项验证。 |
| Webhook 真实投递 | 配置入口存在，真实外部 URL 投递待专项验证。 |
| Instance Pool / Autoscaling | 当前 UI 入口已移除，后续需要按 OCI 官方资源重新设计。 |
| 自动化 scheduler | 策略页面存在，真实周期触发、冷却和执行链路仍需补齐。 |
