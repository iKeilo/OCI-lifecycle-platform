# GitHub Releases 与 Packages 发布说明

项目发布包含两部分：

- GitHub Packages / GHCR：Docker 镜像，用于默认 Docker 一键安装。
- GitHub Releases：安装脚本、说明文档、原生 Linux 预编译包和校验文件。

## 自动发布流程

Workflow 文件：

```text
.github/workflows/build-and-push-images.yml
```

触发方式：

- 推送版本 tag，例如 `v1.0.27`。
- 在 GitHub Actions 手动运行 `Build and Push Images`。

## Docker Packages

发布 tag 后会构建并推送多架构镜像：

```text
ghcr.io/ikeilo/oci-lifecycle-platform:latest
ghcr.io/ikeilo/oci-lifecycle-platform:v1.0.27
ghcr.io/ikeilo/oci-lifecycle-platform:sha-<commit>
```

支持平台：

- `linux/amd64`
- `linux/arm64`

Docker 一键安装默认直接拉取 GHCR 镜像，不在服务器本地编译。

## Release 附件

每个版本 Release 会附带：

```text
panel_install.sh
panel_linux_install.sh
docker-compose.yml
docker/.env.example
docs/one-click-install.md
docs/docker-install.md
oci-lifecycle-platform-linux-amd64.tar.gz
oci-lifecycle-platform-linux-arm64.tar.gz
oci-lifecycle-platform-linux-386.tar.gz
oci-lifecycle-platform-linux-armv7.tar.gz
oci-lifecycle-platform-linux-*.tar.gz.sha256
SHA256SUMS
```

原生 Linux tar 包内包含：

```text
bin/oci-lifecycle-platform
bin/panel-password
www/
scripts/install.sh
panel_linux_install.sh
README.md
LICENSE
docs/
```

## 创建新版本

```powershell
git fetch origin --tags
git status --short --branch
git tag -a v1.0.27 -m "Release v1.0.27"
git push origin main
git push origin v1.0.27
```

推送 tag 后，打开 GitHub Actions 等待发布流程完成。

## 安装器如何选择包

Docker 入口：

```bash
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh)
```

默认拉取：

```text
ghcr.io/ikeilo/oci-lifecycle-platform:latest
```

原生 Linux 入口：

```bash
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_linux_install.sh)
```

默认从 GitHub Release latest 下载当前架构的 tar 包。无法下载或架构不支持时，安装器会明确提示并回退源码安装。

## 手动验证

验证 Docker 镜像：

```bash
docker pull ghcr.io/ikeilo/oci-lifecycle-platform:latest
docker run --rm ghcr.io/ikeilo/oci-lifecycle-platform:latest /app/oci-lifecycle-platform --help
```

验证 Release 附件：

```bash
curl -fL https://github.com/iKeilo/OCI-lifecycle-platform/releases/latest/download/oci-lifecycle-platform-linux-amd64.tar.gz -o /tmp/oci-lifecycle-platform-linux-amd64.tar.gz
curl -fL https://github.com/iKeilo/OCI-lifecycle-platform/releases/latest/download/oci-lifecycle-platform-linux-amd64.tar.gz.sha256 -o /tmp/oci-lifecycle-platform-linux-amd64.tar.gz.sha256
cd /tmp && sha256sum -c oci-lifecycle-platform-linux-amd64.tar.gz.sha256
```

如果 GHCR Package 不是公开状态，新服务器需要先登录 GHCR。建议将 Packages 设置为公开，以保证一键安装无需凭据。
