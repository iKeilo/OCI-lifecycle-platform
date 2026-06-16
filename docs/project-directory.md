# 项目目录索引

这份文档用于快速判断项目里每个目录的职责、部署入口、验证入口，以及哪些文件不应该进入仓库。项目定位为 **OCI 机器生命周期控制平台**，当前只面向 Oracle Cloud Infrastructure，不引入其它云厂商抽象。

## 根目录

| 路径 | 说明 |
| --- | --- |
| `backend/` | Go 后端服务，包含 API、认证、OCI executor、任务执行、通知、持久化和验证工具。 |
| `src/` | React + Vite 前端控制台，包含登录、账号、实例、创建实例、任务、通知、审计等页面。 |
| `docs/` | 产品设计、UI 设计、API、部署阶段、真实 OCI 验证、安装说明和本目录索引。 |
| `scripts/` | 本地/服务器部署、真实 OCI 验证、安装默认值检查、测试资源清理脚本。 |
| `docker/` | Docker 运行环境示例配置。真实密钥只允许放在服务器环境文件，不进入仓库。 |
| `references/` | UI 参考设计和截图素材，仅作为设计参考，不参与运行时。 |
| `Dockerfile` | Docker 版面板镜像构建入口。 |
| `docker-compose.yml` | Docker 版部署编排入口。 |
| `panel_install.sh` | 远程一键 Docker 安装脚本入口，支持安装、更新、改密码、端口配置、卸载等。 |
| `panel_linux_install.sh` | 远程一键原生 Linux/systemd 安装脚本入口。 |
| `.env.example` | 本地开发环境变量示例，不包含真实密钥。 |
| `README.md` | 项目总说明和快速开始入口。 |

## 后端目录

| 路径 | 说明 |
| --- | --- |
| `backend/cmd/server/` | Web/API 服务启动入口。 |
| `backend/cmd/panel-password/` | 面板密码 bcrypt hash 生成工具。 |
| `backend/cmd/oci-testnet/` | 真实 OCI 测试网络/实例验证工具。 |
| `backend/cmd/oci-ipv6-orch-smoke/` | IPv6 编排专项验证工具。 |
| `backend/cmd/oci-rootpwd-testnet/` | root 密码实例、邮件通知、SSH 登录专项验证工具。 |
| `backend/internal/api/` | HTTP API、路由、请求校验、响应模型、登录鉴权中间件。 |
| `backend/internal/auth/` | 面板登录密码校验和 session 相关逻辑。 |
| `backend/internal/config/` | 环境变量加载、运行模式、数据库、SMTP、Webhook、OCI fallback 配置。 |
| `backend/internal/db/` | PostgreSQL schema、迁移和持久化实现。 |
| `backend/internal/domain/` | Profile、Instance、Job、Notification、Audit 等领域模型。 |
| `backend/internal/fileprofile/` | 本地文件 profile sink，用于非数据库部署场景。 |
| `backend/internal/jobs/` | 任务 runner、local executor、真实 OCI executor。 |
| `backend/internal/notify/` | 邮件与 Webhook 通知发送器。 |
| `backend/internal/oci/` | OCI SDK 客户端、实例生命周期、启动选项、IP/IPv6 网络、readiness、验证逻辑。 |
| `backend/internal/profileconfig/` | OCI config 与 PEM 私钥解析、profile 解析。 |
| `backend/internal/store/` | 内存/文件/数据库存储边界与测试。 |

## 前端目录

| 路径 | 说明 |
| --- | --- |
| `src/app/` | 应用入口、路由和导航定义。 |
| `src/components/` | Shell、状态、指标卡、异步状态等通用组件。 |
| `src/pages/DashboardPage.tsx` | 总览页面。 |
| `src/pages/ProfilesPage.tsx` | OCI Profile 添加、测试连接、启用/禁用、删除。 |
| `src/pages/InstancesPage.tsx` | 实例列表、生命周期操作、IP/IPv6 操作入口。 |
| `src/pages/CreateInstancePage.tsx` | 创建实例向导、Launch Options、重试模式、标签等。 |
| `src/pages/TemplatesPage.tsx` | 模板管理页面。模板只保存创建实例表单预输入，不调用 OCI API。 |
| `src/pages/JobsPage.tsx` | 任务中心与任务状态。 |
| `src/pages/NotificationsPage.tsx` | 站内通知列表。 |
| `src/pages/EmailPage.tsx` | 通知通道配置，包含 SMTP 和 Webhook。 |
| `src/pages/AuditPage.tsx` | 审计日志查询、筛选、详情。 |
| `src/pages/BudgetManagementPage.tsx` | 预算管理入口，展示预算策略、预算熔断、自动降配/删机护栏；当前执行器待后端接入。 |
| `src/pages/AutomationsPage.tsx` | 自动化策略入口，目前仍需补齐 scheduler 与真实执行链路。 |
| `src/pages/SettingsPage.tsx` | 平台设置入口。 |
| `src/pages/UsersPage.tsx` | 用户/RBAC 入口，目前仍需补齐真实 RBAC 与审批。 |
| `src/services/api.ts` | 前端 API client 与类型定义。 |
| `src/styles.css` | 全局样式、布局、表格、表单、详情面板样式。 |

## 文档导航

| 文档 | 用途 |
| --- | --- |
| `docs/oci-machine-lifecycle-platform-product-design.md` | 产品方案、功能层级、核心流程。 |
| `docs/oci-control-console-ui-design.md` | UI 层级、页面组织、交互设计。 |
| `docs/go-backend-api.md` | Go 后端 API、接口状态、已验证/未验证说明。 |
| `docs/deployment-stages.md` | 阶段化部署、落地顺序、验证校准。 |
| `docs/real-oci-validation.md` | 真实 OCI 专项验证记录和限制说明。 |
| `docs/one-click-install.md` | 一键安装脚本说明。 |
| `docs/docker-install.md` | Docker 版部署说明。 |
| `docs/linux-install.md` | 原生 Linux/systemd 部署说明。 |
| `docs/github-release-packages.md` | GitHub Releases 与 GHCR Packages 发布说明。 |
| `docs/budget-management-implementation.md` | 预算管理、预算熔断、超预算降配/删机的实施方案。 |
| `docs/template-management-implementation.md` | 模板管理回归实施文档，定义模板为纯预输入功能。 |
| `docs/launch-options-cache-implementation.md` | Shape/Image 选项目录缓存方案，定义用真实 OCI API 预存 Shape 与绑定 Image，前端优先读缓存。 |

## 部署入口

Docker 一键安装，默认拉取 GitHub Packages / GHCR 镜像：

```bash
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh)
```

指定 GitHub Packages 镜像版本安装：

```bash
OCI_LIFECYCLE_IMAGE_TAG=1.0.15 bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh)
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

本地基础验证：

```powershell
cd backend
go test ./...

cd ..
npm run build
```

真实 OCI 验证必须使用有效 OCI Profile 和 PEM 私钥，不能用 local executor 或 mock 输出冒充真实通过。真实验证记录统一维护在 `docs/real-oci-validation.md`。

## 不应提交到仓库

以下内容只允许保留在本机或服务器安全位置，不能提交到 GitHub：

- `.env`、`.env.*` 中的真实环境变量。
- OCI PEM 私钥、API key、指纹对应的私钥文件。
- SMTP 密码、Webhook secret、面板明文密码。
- `profiles.json`、`panel.env`、服务器 `/etc/oci-lifecycle-platform/*.env`。
- `node_modules/`、`dist/`、`.runtime/`、`.codegraph/`。
- `*.log`、临时构建产物、真实测试导出的敏感结果。

## 当前待补齐/待专项验证

| 项目 | 状态 |
| --- | --- |
| PostgreSQL 重启恢复 | 已有持久化基础，仍需做完整重启恢复专项验证。 |
| Webhook 真实投递 | API 与配置已落地，真实外部 URL 投递待专项验证。 |
| Instance Pool / Autoscaling | 当前 UI 入口已移除，后续需要按 OCI Instance Configuration / Instance Pool / Autoscaling 重新设计并落地。 |
| RBAC / 审批 | 页面入口和部分模型存在，真实权限与审批流待落地。 |
| 预算管理 / 预算熔断 | 已新增前端入口和实施文档；BudgetPolicy CRUD、OCI Budgets 同步、Cost Usage 采集、自动降配/删机执行器待落地。 |
| Template CRUD | 已恢复模板管理页面、CRUD、本地字段检查、创建实例页预输入和 `templateId` 任务记录；模板版本化与使用历史待补齐。 |
| 自动化 scheduler | 策略页面存在，真实周期触发、冷却和执行链路待补齐。 |
| 审计导出 | 查询已落地，导出能力待补齐。 |
