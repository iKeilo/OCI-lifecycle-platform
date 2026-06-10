# Docker 版部署说明

Docker 版适合把前端、Go 后端和运行时依赖打成一个镜像部署。默认使用容器内 Go 服务托管前端静态文件，不依赖 nginx。安装后默认启用真实 OCI SDK 模式，Profile 测试和资源发现会走真实 OCI API。

## 文件结构

```text
Dockerfile
docker-compose.yml
docker/.env.example
scripts/install.sh
```

容器运行时使用两个关键位置：

- `/app/www`：前端构建产物。
- `/data/profiles.json`：加密后的 OCI Profile 文件仓库，由 Docker volume 持久化。

OCI PEM 私钥不要放入仓库。Docker 部署时建议放在宿主机 `/etc/oci-lifecycle-platform/keys/`，并只读挂载到容器 `/keys`。

## 一键安装

远程一键安装：

```bash
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh)
```

在 Debian、Ubuntu、Armbian 或常见 RHEL 系服务器上执行：

```bash
git clone https://github.com/iKeilo/OCI-lifecycle-platform.git
cd OCI-lifecycle-platform
sudo bash scripts/install.sh install
```

脚本会自动完成：

- 安装基础工具和 Docker Compose 插件。
- 复制或拉取源码。
- 创建 `/etc/oci-lifecycle-platform/docker.env`。
- 生成 `PANEL_SESSION_SECRET` 和 `PROFILE_KEY_ENCRYPTION_KEY`。
- 设置 `OCI_EXECUTION_MODE=oci`，避免装完后 Profile 测试停留在本地模式。
- 构建 Docker 镜像。
- 初始化面板登录密码的 bcrypt hash。
- 启动服务并检查 `/api/health`。

安装时会提示输入 Web 端口：

```text
Set web panel port, or press Enter for a random available port [current: 18080]:
```

直接回车会随机分配一个可用端口，并在安装输出中显示最终地址。如果手动输入的端口已被占用，脚本会要求更换端口。如果要非交互指定端口：

```bash
WEB_PORT=8088 bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh)
```

安装时也会提示输入面板密码：

```text
Set panel login password, or press Enter to generate one:
```

第一次输入直接回车会随机生成密码。安装器会在终端直接打印一次随机密码，便于马上登录面板。随机密码不会保存到服务器文件；后续如果忘记密码，请使用 `change-password` 重新设置。

## 菜单模式

直接运行脚本会显示菜单：

```bash
sudo bash scripts/install.sh
```

菜单包含 4 个选项：

1. Install / first setup
2. Update from GitHub latest
3. Uninstall
4. Reset panel login password

## 非交互安装

可通过环境变量传入初始密码：

```bash
sudo PANEL_PASSWORD='change-this-password' \
  WEB_PORT=18080 \
  bash scripts/install.sh install
```

远程非交互方式：

```bash
PANEL_PASSWORD='change-this-password' \
  WEB_PORT=18080 \
  bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh)
```

脚本会在镜像构建后调用容器内 `/app/panel-password hash` 生成 bcrypt hash，只把 hash 写入 `/etc/oci-lifecycle-platform/docker.env`。

## OCI Profile

推荐方式是在 Web 控制台里添加 OCI Profile：粘贴 OCI config，再粘贴 PEM 私钥。后端会使用 `PROFILE_KEY_ENCRYPTION_KEY` 加密保存。

如果需要运行时 env fallback：

```bash
sudo bash scripts/install.sh configure-oci
```

然后把 PEM 放到：

```text
/etc/oci-lifecycle-platform/keys/
```

在脚本里填写容器内路径，例如：

```text
/keys/oci.pem
```

## 手动 Docker Compose

复制环境模板：

```bash
sudo mkdir -p /etc/oci-lifecycle-platform/keys
sudo cp docker/.env.example /etc/oci-lifecycle-platform/docker.env
sudo chmod 600 /etc/oci-lifecycle-platform/docker.env
```

构建并启动：

```bash
docker compose --env-file /etc/oci-lifecycle-platform/docker.env build app
docker compose --env-file /etc/oci-lifecycle-platform/docker.env up -d app
```

查看状态：

```bash
docker compose --env-file /etc/oci-lifecycle-platform/docker.env ps
docker compose --env-file /etc/oci-lifecycle-platform/docker.env logs -f app
```

## 可选 PostgreSQL

默认 Docker 版使用加密文件仓库，数据在 Docker volume `oci-lifecycle-platform-profile-data` 中。

如需 PostgreSQL：

1. 在 `/etc/oci-lifecycle-platform/docker.env` 设置 `DATABASE_URL`。
2. 设置强密码 `POSTGRES_PASSWORD`。
3. 启动 postgres profile。

```bash
docker compose --env-file /etc/oci-lifecycle-platform/docker.env --profile postgres up -d
```

## 更新

```bash
sudo bash scripts/install.sh update
```

更新会拉取最新源码、重新构建镜像并重启 `app` 容器。`docker.env`、PEM 目录和 Profile 数据卷会保留。

## 备份

```bash
sudo bash scripts/install.sh backup
```

备份内容包含：

- `/etc/oci-lifecycle-platform/docker.env`
- `oci-lifecycle-platform-profile-data` 数据卷中的加密 Profile 数据

备份文件仍然属于敏感数据，需要按密钥材料标准保存。

## 卸载

```bash
sudo bash scripts/install.sh uninstall
```

脚本会停止容器，并询问是否删除 Docker volume、源码目录和环境目录。删除 volume 会移除加密 Profile 数据。

## 验证

```bash
curl -fsS http://127.0.0.1:18080/api/health
```

未登录访问受保护 API 应返回：

```json
{"error":{"code":"AUTH_REQUIRED","message":"panel login required"}}
```

## GitHub Packages 镜像

默认 Docker 一键安装会直接使用 GitHub Packages / GHCR 已发布镜像，而不是在服务器本地构建：

```bash
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh)
```

指定版本镜像：

```bash
OCI_LIFECYCLE_IMAGE_TAG=1.0.0 \
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh)
```

如果需要回退为服务器本地构建：

```bash
OCI_LIFECYCLE_USE_PACKAGE=false \
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh)
```

默认镜像地址：

```text
ghcr.io/ikeilo/oci-lifecycle-platform
```

## 安全边界

不要提交以下内容：

- `/etc/oci-lifecycle-platform/docker.env`
- `/etc/oci-lifecycle-platform/keys/*.pem`
- Docker volume 备份包
- 真实 OCID、request id、work request id 验证日志
