# GitHub Releases 与 Packages 发布说明

本项目使用 GitHub Actions 同时发布：

- GitHub Release：用于版本归档、安装脚本附件和发布说明。
- GitHub Packages / GHCR：用于发布 Docker 镜像 `ghcr.io/ikeilo/oci-lifecycle-platform`。

## 自动发布流程

Workflow 文件：

```text
.github/workflows/build-and-push-images.yml
```

触发条件：

- 推送版本 tag，例如 `1.0.0` 或 `v1.0.0`。
- 在 GitHub Actions 页面手动运行 `Build and Push Images`。

发布结果：

- Docker 镜像：
  - `ghcr.io/ikeilo/oci-lifecycle-platform:<tag>`
  - `ghcr.io/ikeilo/oci-lifecycle-platform:latest`
  - `ghcr.io/ikeilo/oci-lifecycle-platform:sha-<commit>`
- GitHub Release 附件：
  - `panel_install.sh`
  - `panel_linux_install.sh`
  - `docker-compose.yml`
  - `docker/.env.example`
  - `docs/one-click-install.md`
  - `docs/docker-install.md`

## 创建新版本

从干净的 `main` 分支创建并推送 tag：

```powershell
git fetch origin --tags
git status --short --branch
git tag -a 1.0.0 -m "Release 1.0.0"
git push origin 1.0.0
```

推送 tag 后，打开：

```text
https://github.com/iKeilo/OCI-lifecycle-platform/actions
```

等待 `Build and Push Images` 完成。

## 使用 GitHub Packages 镜像安装

默认 Docker 一键安装会直接拉取 GHCR 已发布镜像，不再在服务器本地构建：

```bash
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh)
```

指定版本：

```bash
OCI_LIFECYCLE_IMAGE_TAG=1.0.0 \
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh)
```

也可以显式覆盖镜像：

```bash
OCI_LIFECYCLE_USE_PACKAGE=true \
OCI_LIFECYCLE_IMAGE=ghcr.io/ikeilo/oci-lifecycle-platform:1.0.0 \
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh)
```

如果必须在服务器本地重新构建镜像，例如调试源码构建问题：

```bash
OCI_LIFECYCLE_USE_PACKAGE=false \
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh)
```

## 手动拉取验证

```bash
docker pull ghcr.io/ikeilo/oci-lifecycle-platform:latest
docker run --rm ghcr.io/ikeilo/oci-lifecycle-platform:latest /app/oci-lifecycle-platform --help
```

如果 Package 是私有的，需要先登录 GHCR：

```bash
echo "$GITHUB_TOKEN" | docker login ghcr.io -u <github-user> --password-stdin
```

建议将仓库 Packages 设置为公开，方便一键安装脚本在新服务器上无需凭据拉取镜像。
