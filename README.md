# OCI Lifecycle Platform

OCI Lifecycle Platform 是一个面向 Oracle Cloud Infrastructure 的机器生命周期控制平台。目标不是把几个 OCI API 包一层按钮，而是提供一套可登录、可审计、可任务化、可自动化扩展的资源编排控制台。

项目目前只支持 OCI，不引入其它云厂商抽象，避免产品边界被污染。

## 核心能力

- 中文 Web 控制台：Dashboard、账号与密钥、实例管理、创建实例、任务中心、自动化入口、监控入口、审计入口、设置。
- 面板登录密码：后端 bcrypt 校验，浏览器只保存 HttpOnly Cookie。
- OCI Profile 管理：支持直接粘贴 OCI config，选择或粘贴 PEM 私钥。
- 密钥加密存储：`PROFILE_KEY_ENCRYPTION_KEY` + AES-GCM，本地文件仓库或 PostgreSQL sink。
- 真实 OCI executor：LaunchInstance、START、STOP、REBOOT、TERMINATE、Resize、Reinstall、Launch Options 发现、实例同步、IPv6 分配。
- 本地 executor：用于 UI 和流程开发，不冒充真实 OCI 验证。
- 一键安装脚本：支持安装、更新、改密码、配置 OCI env fallback、启停、重启、状态日志、备份、卸载。

## 快速安装

远程一键 Docker 安装：

```bash
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh)
```

安装时会提示输入 Web 端口；直接回车会随机分配一个可用端口。也可以手动指定：

```bash
WEB_PORT=18081 bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh)
```

安装时会提示输入面板密码；第一次输入直接回车会随机生成密码，并保存到服务器：

```text
/etc/oci-lifecycle-platform/panel-password.txt
```

Debian/Ubuntu 服务器：

```bash
git clone https://github.com/iKeilo/OCI-lifecycle-platform.git
cd OCI-lifecycle-platform
sudo bash scripts/install.sh
```

如果 80 端口已被占用，或服务器没有 nginx，可以直接由 Go 服务托管前端：

```bash
sudo WEB_PORT=18080 USE_NGINX=false bash scripts/install.sh install
```

私有仓库环境需要服务器具备 GitHub 拉取权限，或先把源码上传到服务器后使用：

```bash
sudo bash scripts/install.sh install --source /path/to/OCI-lifecycle-platform
```

完整说明见：[docs/one-click-install.md](docs/one-click-install.md)。

Docker 版推荐用于干净服务器或希望隔离运行时依赖的环境：

```bash
sudo bash scripts/install.sh install
```

最终访问地址以安装输出为准。完整说明见：[docs/docker-install.md](docs/docker-install.md)。

## 本地开发

启动后端：

```powershell
cd backend
go run ./cmd/server
```

启动前端：

```powershell
npm install
npm run dev
```

打开 [http://localhost:5173](http://localhost:5173)。

默认情况下，如果未配置 `PANEL_PASSWORD_HASH` 或 `PANEL_PASSWORD`，本地认证会关闭。生产部署必须设置面板密码。

## 测试

```powershell
cd backend
go test ./...

cd ..
npm run build
```

真实 OCI 验证必须配置有效 OCI Profile 后执行，不能用 local executor 或 mock 输出替代。

## 关键配置

复制 `.env.example` 后按需配置：

- `PANEL_PASSWORD_HASH`：面板登录密码的 bcrypt hash。
- `PANEL_SESSION_SECRET`：登录 Cookie 签名密钥。
- `PROFILE_KEY_ENCRYPTION_KEY`：32 字节或 base64 32 字节密钥，用于加密 OCI 私钥。
- `PROFILE_STORE_FILE`：无 PostgreSQL 时的本地加密 Profile 文件。
- `OCI_EXECUTION_MODE`：`local` 或 `oci`。
- `DATABASE_URL`：可选 PostgreSQL 持久化。
- `STATIC_DIR`：由 Go 服务直接托管前端时的静态目录。

## 安全说明

不要提交：

- OCI PEM 私钥。
- `.env`、`panel.env`。
- `.runtime/`。
- `profiles.json`。
- 真实 OCI request id / work request id / OCID 验证日志。

仓库 `.gitignore` 已覆盖常见敏感文件模式，但提交前仍应主动检查。

## 文档

- [一键安装说明](docs/one-click-install.md)
- [Docker 版部署说明](docs/docker-install.md)
- [阶段化实施状态](docs/deployment-stages.md)
- [Go 后端 API](docs/go-backend-api.md)
- [真实 OCI 验证记录](docs/real-oci-validation.md)
- [产品设计文档](docs/oci-machine-lifecycle-platform-product-design.md)
- [UI 设计文档](docs/oci-control-console-ui-design.md)

## License

本仓库沿用目标仓库已有许可证：GPL-3.0。详见 [LICENSE](LICENSE)。
