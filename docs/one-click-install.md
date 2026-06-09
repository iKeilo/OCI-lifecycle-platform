# 一键安装脚本说明

`scripts/install.sh` 是项目唯一的一键安装入口。默认使用 Docker 模式，也保留 systemd 模式。

Docker 版说明见：[Docker 版部署说明](docker-install.md)。

默认 Docker 安装：

```bash
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh)
```

交互安装时：

- Web 端口直接回车会随机分配可用端口。
- 第一次输入面板密码时直接回车会随机生成密码，并保存到 `/etc/oci-lifecycle-platform/panel-password.txt`。

如果已经克隆仓库，也可以执行：

```bash
sudo bash scripts/install.sh install
```

传统 systemd 安装：

```bash
sudo bash scripts/install.sh --systemd install
```

## 默认安装路径

```text
/opt/oci-lifecycle-platform/
  bin/oci-lifecycle-platform
  bin/panel-password
  src/
  www/

/etc/oci-lifecycle-platform/
  panel.env
  profiles.json

/etc/systemd/system/oci-lifecycle-platform.service
```

如果 `USE_NGINX=true`，或 `USE_NGINX=auto` 且检测到 nginx，脚本还会创建：

```text
/etc/nginx/sites-available/oci-lifecycle-platform.conf
```

## 支持系统

当前脚本面向 Debian/Ubuntu/Armbian：

- `systemd`
- `apt-get`
- `curl`
- `tar`
- Node.js 20+
- Go 版本满足 `backend/go.mod`

如果系统没有合适 Go 版本，脚本会从 `go.dev` 下载对应 CPU 架构的 Go 工具链到 `APP_DIR/.toolchain/go`，不会写入 `/usr/local/go`。

## 菜单选项

直接运行：

```bash
sudo bash scripts/install.sh
```

菜单包含 10 个选项：

1. Install / first setup
2. Update application
3. Change panel login password
4. Configure OCI env fallback
5. Start services
6. Stop services
7. Restart services
8. Status and logs
9. Backup local config
10. Uninstall

## 从 GitHub 安装

公开仓库或服务器已具备 GitHub 拉取权限时：

```bash
git clone https://github.com/iKeilo/OCI-lifecycle-platform.git
cd OCI-lifecycle-platform
sudo bash scripts/install.sh install
```

脚本默认仓库地址已经是：

```text
https://github.com/iKeilo/OCI-lifecycle-platform.git
```

因此也可以指定一个空目录运行：

```bash
sudo OCI_LIFECYCLE_REPO_URL=https://github.com/iKeilo/OCI-lifecycle-platform.git \
  bash scripts/install.sh install
```

私有仓库需要先配置 GitHub 凭据、deploy key，或使用本地源码安装。

## 从本地源码安装

适合测试服务器、内网服务器或私有仓库无 Git 凭据的场景：

```bash
sudo bash scripts/install.sh install --source /path/to/OCI-lifecycle-platform
```

非交互设置密码：

```bash
sudo PANEL_PASSWORD='change-this-password' \
  bash scripts/install.sh install --source /path/to/OCI-lifecycle-platform
```

脚本会把明文密码转换为 bcrypt，写入 `PANEL_PASSWORD_HASH`，并清空 `PANEL_PASSWORD`。

## 无 nginx / 端口被占用

如果 80 端口已经被占用，设置 `WEB_PORT`：

```bash
sudo WEB_PORT=18080 bash scripts/install.sh install
```

如果不想安装或使用 nginx，让 Go 服务直接托管前端：

```bash
sudo WEB_PORT=18080 USE_NGINX=false bash scripts/install.sh install
```

此时 `/api/*` 和前端页面都由同一个 Go 进程提供。

## 小根分区服务器

如果根分区很小，建议把应用、配置、缓存和备份放到大盘：

```bash
sudo APP_DIR=/mnt/Storage1/oci-lifecycle-platform \
  ENV_DIR=/mnt/Storage1/oci-lifecycle-platform-config \
  BACKUP_DIR=/mnt/Storage1/oci-lifecycle-platform-backups \
  WEB_PORT=18080 \
  USE_NGINX=false \
  GO_PROXY=https://goproxy.cn,direct \
  bash scripts/install.sh install --source /tmp/OCI-lifecycle-platform
```

临时测试可使用：

```bash
SYSTEMD_DIR=/run/systemd/system
```

注意：`/run/systemd/system` 不是持久目录，服务器重启后服务单元会丢失。生产环境应使用默认 `/etc/systemd/system`。

## 更新

```bash
sudo bash scripts/install.sh update
```

更新会：

- 同步源码。
- 重新执行 `npm ci && npm run build`。
- 重新构建 Go 后端。
- 保留 `panel.env` 和 `profiles.json`。
- 重启服务。

## 更改面板密码

```bash
sudo bash scripts/install.sh change-password
```

非交互方式：

```bash
sudo PANEL_PASSWORD='new-strong-password' bash scripts/install.sh change-password
```

## 配置 OCI env fallback

推荐在 Web 控制台的 Profile 页面粘贴 OCI config 和 PEM 私钥。脚本中的 `configure-oci` 只作为 env fallback：

```bash
sudo bash scripts/install.sh configure-oci
```

会写入：

- `OCI_EXECUTION_MODE=oci`
- `OCI_TENANCY_OCID`
- `OCI_USER_OCID`
- `OCI_FINGERPRINT`
- `OCI_REGION`
- `OCI_PRIVATE_KEY_FILE`

不要把 PEM 文件放进 Git 仓库。

## 验证

```bash
systemctl status oci-lifecycle-platform --no-pager
curl -i http://127.0.0.1/api/health
```

使用非 80 端口：

```bash
curl -i http://127.0.0.1:18080/api/health
```

开启面板密码后，未登录访问受保护 API 应返回：

```json
{"error":{"code":"AUTH_REQUIRED","message":"panel login required"}}
```

## 备份

```bash
sudo BACKUP_DIR=/root bash scripts/install.sh backup
```

备份包含 `panel.env` 和加密 Profile store，仍然属于敏感文件，请妥善保存。

## 卸载

```bash
sudo bash scripts/install.sh uninstall
```

卸载会移除应用目录、systemd unit 和 nginx site。脚本会询问是否删除配置目录，因为其中可能包含面板密钥和加密 Profile 数据。

## 环境变量速查

| 变量 | 用途 | 默认值 |
| --- | --- | --- |
| `OCI_LIFECYCLE_REPO_URL` | Git 仓库地址 | `https://github.com/iKeilo/OCI-lifecycle-platform.git` |
| `OCI_LIFECYCLE_BRANCH` | Git 分支 | `main` |
| `PANEL_PASSWORD` | 非交互安装/改密码输入 | 空 |
| `PANEL_PASSWORD_FILE` | 随机生成面板密码时的保存文件 | `$ENV_DIR/panel-password.txt` |
| `WEB_PORT` | Web 监听端口 | `80` |
| `USE_NGINX` | `true` / `false` / `auto` | `auto` |
| `GO_PROXY` | Go module proxy | 空 |
| `GO_ROOT` | Go 工具链安装路径 | `$APP_DIR/.toolchain/go` |
| `APP_DIR` | 应用目录 | `/opt/oci-lifecycle-platform` |
| `ENV_DIR` | 配置目录 | `/etc/oci-lifecycle-platform` |
| `BACKUP_DIR` | 备份输出目录 | `/root` |
| `SYSTEMD_DIR` | systemd unit 目录 | `/etc/systemd/system` |
