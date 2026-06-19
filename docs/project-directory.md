# 项目目录索引

本项目是 **OCI 机器生命周期控制平台**，当前只面向 Oracle Cloud Infrastructure。目录划分遵循“前端控制台、Go 后端、真实 OCI 执行器、PostgreSQL 持久化、部署脚本、实施文档”六个边界。

## 根目录

| 路径 | 说明 |
| --- | --- |
| `backend/` | Go 后端服务，包含 API、认证、OCI executor、任务、通知、审计、数据库持久化。 |
| `src/` | React + Vite 前端控制台。 |
| `docs/` | 产品方案、实施文档、部署文档、真实 OCI 验证记录、目录索引。 |
| `scripts/` | 安装、更新、测试、清理和发布辅助脚本。 |
| `docker/` | Docker 环境变量示例和容器部署说明。 |
| `references/` | UI 参考素材，不参与运行。 |
| `.github/` | GitHub Actions workflow。 |
| `Dockerfile` | 多阶段镜像构建入口。 |
| `docker-compose.yml` | Docker 版部署编排，默认包含 app + PostgreSQL。 |
| `panel_install.sh` | Docker 一键安装入口。 |
| `panel_linux_install.sh` | 原生 Linux/systemd 一键安装入口。 |
| `README.md` | 项目说明和快速开始。 |

## 后端

| 路径 | 说明 |
| --- | --- |
| `backend/cmd/server/` | Web/API 服务启动入口，负责配置加载、存储初始化、任务 runner 启动、静态前端服务。 |
| `backend/cmd/panel-password/` | 面板密码 bcrypt hash 生成工具。 |
| `backend/cmd/oci-testnet/` | 真实 OCI 测试网络/实例验证工具。 |
| `backend/cmd/oci-ipv6-orch-smoke/` | IPv6 编排专项验证工具。 |
| `backend/cmd/oci-rootpwd-testnet/` | root 密码实例、邮件通知、SSH 登录专项验证工具。 |
| `backend/internal/api/` | HTTP API、路由、请求校验、响应、认证中间件。 |
| `backend/internal/auth/` | 面板登录、密码校验、session 管理。 |
| `backend/internal/config/` | 环境变量加载，包含 OCI、SMTP、Webhook、数据库、安全配置。 |
| `backend/internal/db/` | PostgreSQL schema、迁移和持久化实现。当前负责 Profile、模板、实例、任务、审计日志、通知、平台设置。 |
| `backend/internal/domain/` | 领域模型：Profile、Instance、Job、Notification、Audit、Budget、Network 等。 |
| `backend/internal/fileprofile/` | 文件存储 fallback，以及旧数据迁移来源。Docker 默认已切换到 PostgreSQL。 |
| `backend/internal/jobs/` | 任务 runner、local executor、真实 OCI executor。 |
| `backend/internal/notify/` | SMTP 邮件与 Webhook 发送器。 |
| `backend/internal/oci/` | OCI SDK 客户端、实例生命周期、启动选项、网络/IP/IPv6、readiness 验证。 |
| `backend/internal/profileconfig/` | OCI config 和 PEM 私钥解析。 |
| `backend/internal/store/` | 内存状态与持久化 sink 的协调层。 |

## 前端

| 路径 | 说明 |
| --- | --- |
| `src/app/` | 应用入口、路由、导航定义。 |
| `src/components/` | 通用组件：AppShell、PageHeader、AsyncState、状态标签、指标卡等。 |
| `src/pages/DashboardPage.tsx` | 总览页面。 |
| `src/pages/ProfilesPage.tsx` | OCI Profile 添加、测试连接、启用/禁用、删除。 |
| `src/pages/InstancesPage.tsx` | 实例管理、生命周期操作、IP/IPv6 操作入口。 |
| `src/pages/CreateInstancePage.tsx` | 创建实例向导、Launch Options、预算估算、重试模式。 |
| `src/pages/TemplatesPage.tsx` | 模板管理。模板只保存创建实例表单预输入，不直接调用 OCI API。 |
| `src/pages/JobsPage.tsx` | 任务中心。 |
| `src/pages/NotificationsPage.tsx` | 站内通知列表，支持已读、邮件状态展示、删除。 |
| `src/pages/EmailPage.tsx` | SMTP 与 Webhook 通知通道配置。 |
| `src/pages/AuditPage.tsx` | 审计日志查询、筛选、详情、JSON/CSV 导出。 |
| `src/pages/NetworkPage.tsx` | 网络管理，包含预留公网 IP、IPv6 等入口。 |
| `src/pages/BudgetManagementPage.tsx` | 预算管理与预算熔断入口。 |
| `src/pages/AutomationsPage.tsx` | 自动化策略入口。 |
| `src/pages/SettingsPage.tsx` | 系统设置。 |
| `src/pages/AccountPage.tsx` | 账号头像、资料、密码。 |
| `src/services/api.ts` | 前端 API client 与类型定义。 |
| `src/styles.css` | 全局样式、布局、表格、响应式和移动端样式。 |

## 数据持久化

| 数据 | 存储位置 |
| --- | --- |
| OCI Profile 与 PEM 密钥 | PostgreSQL，私钥字段加密。 |
| SMTP/Webhook/账号/外观/预算设置 | PostgreSQL `app_settings`。 |
| 站内通知与邮件发送状态 | PostgreSQL `notifications`。 |
| 任务、实例缓存、审计日志 | PostgreSQL。 |
| Docker 数据卷 | `oci-lifecycle-platform-postgres-data`、`oci-lifecycle-platform-profile-data`。 |

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

真实 OCI 验证必须使用有效 OCI Profile 和 PEM 私钥，不能用 local executor 或 mock 输出冒充真实通过。验证记录维护在 `docs/real-oci-validation.md`。

## 禁止提交

以下内容不得进入 GitHub：

- `.env`、`.env.*` 中的真实环境变量。
- OCI PEM 私钥、API key、fingerprint 对应私钥文件。
- SMTP 密码、Webhook secret、面板明文密码。
- `profiles.json`、服务器 `/etc/oci-lifecycle-platform/*.env`。
- `node_modules/`、`dist/`、`.runtime/`、`.codegraph/`。
- `*.log`、临时构建产物、真实测试导出的敏感结果。

## 当前重点

| 模块 | 状态 |
| --- | --- |
| PostgreSQL 持久化 | Docker 默认启用，覆盖 Profile、设置、通知、任务、审计。 |
| 任务中心 | 任务已持久化到 PostgreSQL，支持启动恢复、重试/取消、清除终态任务。 |
| 通知系统 | 已支持数据库持久化、邮件状态记录、删除通知。 |
| 审计日志 | 已支持数据库查询、更多筛选、详情查看、JSON/CSV 导出。 |
| 用户与权限 | 已支持数据库持久化的本地 RBAC、用户密码 hash、角色权限、Profile/区域/Compartment 范围限制。 |
| 安全护栏 | 已支持区域、容量、启动盘、重试、批量公网 IP、删机删盘、IPv6 网络改造等 API 层拦截。 |
| SMTP 真实投递 | API 与配置已落地；真实投递需要页面保存 SMTP 后专项验证。 |
| Webhook 真实投递 | 配置入口存在，真实外部 URL 投递待专项验证。 |
| Instance Pool / Autoscaling | 当前 UI 入口已移除，后续需按 OCI 官方资源重新设计。 |
| RBAC / 审批 | 页面和部分模型存在，真实权限与审批流待落地。 |
| 自动化 scheduler | 策略页面存在，真实周期触发、冷却和执行链路待补齐。 |

## 新增实施文档

| 文档 | 说明 |
| --- | --- |
| `docs/instance-system-settings-implementation.md` | 实例管理“系统设置”实施方案，覆盖重装系统、权限护栏、任务、审计、通知和真实 OCI 验证计划；已有实例重置密码功能已移除。 |
