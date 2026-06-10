# Linux 原生一键部署说明

Linux 原生部署使用 systemd 运行 Go 后端，并在本机编译前端和后端。它不依赖 Docker，适合不能使用容器或希望直接运行 systemd service 的服务器。

## 远程一键安装

```bash
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_linux_install.sh)
```

等价于下载源码后执行：

```bash
sudo bash scripts/install.sh --systemd install
```

默认安装路径：

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

## 常用命令

```bash
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_linux_install.sh) update
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_linux_install.sh) change-password
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_linux_install.sh) status
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_linux_install.sh) uninstall
```

## 端口

systemd 模式默认使用 `WEB_PORT=80`。如需改端口：

```bash
WEB_PORT=18080 bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_linux_install.sh)
```

如果不想使用 nginx，让 Go 服务直接托管前端：

```bash
USE_NGINX=false WEB_PORT=18080 bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_linux_install.sh)
```

## 密码

安装时会提示输入面板密码。第一次直接回车会随机生成密码，随机密码只会在终端打印一次，不会保存到服务器文件。

非交互指定密码：

```bash
PANEL_PASSWORD='change-this-password' bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_linux_install.sh)
```

修改密码：

```bash
PANEL_PASSWORD='new-strong-password' bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_linux_install.sh) change-password
```

## 小根分区服务器

如果根分区空间有限，把应用和配置放到大盘：

```bash
APP_DIR=/mnt/Storage1/oci-lifecycle-platform \
ENV_DIR=/mnt/Storage1/oci-lifecycle-platform-config \
USE_NGINX=false \
WEB_PORT=18080 \
bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_linux_install.sh)
```

## 验证

```bash
systemctl status oci-lifecycle-platform --no-pager
curl -fsS http://127.0.0.1:18080/api/health
```

如果使用 80 端口：

```bash
curl -fsS http://127.0.0.1/api/health
```
