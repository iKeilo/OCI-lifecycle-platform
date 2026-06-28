# 一键安装脚本说明

项目提供两个入口：

- Docker 默认入口：`panel_install.sh`
- 原生 Linux/systemd 入口：`panel_linux_install.sh`

Docker 模式默认拉取 GitHub Container Registry 已构建镜像；原生 Linux 模式默认拉取 GitHub Release 已构建二进制包。只有在包不可用或显式关闭时，才回退到源码构建。

## Docker 快速安装

```bash
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh)
```

运行后会出现菜单：

```text
1) Install / first setup
2) Update from GitHub latest
3) Uninstall
4) Reset panel login password
```

默认镜像：

```text
ghcr.io/ikeilo/oci-lifecycle-platform:latest
```

指定版本：

```bash
OCI_LIFECYCLE_IMAGE_TAG=1.0.27 \
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh)
```

强制源码构建：

```bash
OCI_LIFECYCLE_USE_PACKAGE=false \
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh)
```

## 原生 Linux 快速安装

```bash
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_linux_install.sh)
```

安装器会根据 CPU 架构自动下载 Release 附件：

- `oci-lifecycle-platform-linux-amd64.tar.gz`
- `oci-lifecycle-platform-linux-arm64.tar.gz`
- `oci-lifecycle-platform-linux-386.tar.gz`
- `oci-lifecycle-platform-linux-armv7.tar.gz`

下载后直接安装 `bin/oci-lifecycle-platform`、`bin/panel-password` 和前端 `www/`，不再在服务器上执行 `npm run build` 或 `go build`。

通过统一入口安装 systemd 版本也会走预编译包：

```bash
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh) --systemd install
```

关闭预编译包回退源码构建：

```bash
OCI_LIFECYCLE_USE_PREBUILT=false \
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_linux_install.sh)
```

## 密码和端口

- 第一次设置密码时直接回车，会随机生成密码并只在终端打印一次。
- Docker 安装时端口直接回车，会随机选择可用端口。
- 可以用 `PANEL_PASSWORD` 和 `WEB_PORT` 做非交互安装。

示例：

```bash
PANEL_PASSWORD='change-this-password' WEB_PORT=18080 \
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh) install
```

## 默认路径

Docker 模式：

```text
/opt/oci-lifecycle-platform-docker/
/etc/oci-lifecycle-platform/
```

systemd 模式：

```text
/opt/oci-lifecycle-platform/
  bin/oci-lifecycle-platform
  bin/panel-password
  www/

/etc/oci-lifecycle-platform/
  panel.env
  profiles.json
```

## 更新

Docker 更新：

```bash
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh) update
```

systemd 更新：

```bash
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_linux_install.sh) update
```

两种模式都会保留 `/etc/oci-lifecycle-platform` 下的配置、数据库连接和加密 Profile 数据。

## 卸载

```bash
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh) uninstall
```

卸载时会询问是否删除配置目录和 Docker 卷。配置目录可能包含面板密钥、加密 Profile 数据和数据库连接信息，删除前请确认已有备份。

## 常用环境变量

| 变量 | 用途 | 默认值 |
| --- | --- | --- |
| `OCI_LIFECYCLE_IMAGE_TAG` | Docker 镜像版本 | `latest` |
| `OCI_LIFECYCLE_USE_PACKAGE` | Docker 是否拉 GHCR 镜像 | `true` |
| `OCI_LIFECYCLE_USE_PREBUILT` | systemd 是否拉 Release 二进制 | `true` |
| `OCI_LIFECYCLE_RELEASE_DOWNLOAD_BASE` | Release 附件下载地址 | GitHub latest download |
| `PANEL_PASSWORD` | 非交互面板密码 | 空 |
| `WEB_PORT` | Web 端口 | Docker `18080`，systemd `80` |
| `APP_DIR` | systemd 应用目录 | `/opt/oci-lifecycle-platform` |
| `ENV_DIR` | 配置目录 | `/etc/oci-lifecycle-platform` |
