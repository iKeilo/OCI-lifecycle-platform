# One-Click Install Guide

This guide installs A-Series Oracle as:

- Go API service: `a-series-oracle`
- Static web console: nginx or the Go server on `WEB_PORT` (default `80`)
- Local config and encrypted profile store: `/etc/a-series-oracle`
- Application files: `/opt/a-series-oracle`

## Supported Server

The installer currently supports Debian/Ubuntu systems with `apt`, `systemd`, and `nginx`.

Run as root:

```bash
sudo bash scripts/install.sh
```

## Menu Options

The script exposes 10 options:

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

## Non-Interactive Install

From a local source tree:

```bash
sudo PANEL_PASSWORD='change-this-password' bash scripts/install.sh install --source /tmp/a-series-oracle
```

From GitHub after the repository exists:

```bash
sudo A_SERIES_ORACLE_REPO_URL='https://github.com/YOUR_USER/a-series-oracle.git' \
  WEB_PORT=8088 \
  USE_NGINX=false \
  APP_DIR=/mnt/Storage1/a-series-oracle \
  ENV_DIR=/mnt/Storage1/a-series-oracle-config \
  PANEL_PASSWORD='change-this-password' \
  bash scripts/install.sh install
```

The password is converted to a bcrypt hash and written as `PANEL_PASSWORD_HASH`. The plain password is not stored by the installer.

If port `80` is already occupied, set `WEB_PORT`, for example:

```bash
sudo WEB_PORT=18080 bash scripts/install.sh install
```

Set `USE_NGINX=false` to serve the frontend directly from the Go service. This is useful for small servers or systems where nginx is not installed:

```bash
sudo WEB_PORT=18080 USE_NGINX=false bash scripts/install.sh install
```

For very small test systems with a full root partition, you can keep app files, config, and build caches on a larger mounted disk:

```bash
sudo APP_DIR=/mnt/Storage1/a-series-oracle \
  ENV_DIR=/mnt/Storage1/a-series-oracle-config \
  SYSTEMD_DIR=/run/systemd/system \
  WEB_PORT=18080 \
  USE_NGINX=false \
  GO_PROXY=https://goproxy.cn,direct \
  BACKUP_DIR=/mnt/Storage1/a-series-oracle-backups \
  bash scripts/install.sh install
```

`SYSTEMD_DIR=/run/systemd/system` is suitable for temporary tests only. Production should use the default `/etc/systemd/system`.

If the server cannot reach `proxy.golang.org`, set `GO_PROXY` to a reachable module proxy.
If `/root` is small, set `BACKUP_DIR` when using the backup option.

## Update

```bash
sudo bash scripts/install.sh update
```

Update rebuilds the frontend, rebuilds the Go backend, refreshes nginx config, and restarts services. It preserves `/etc/a-series-oracle/panel.env` and the encrypted profile store.

## Change Panel Password

```bash
sudo bash scripts/install.sh change-password
```

The script prompts for the new password, generates a bcrypt hash through `backend/cmd/panel-password`, writes it to `PANEL_PASSWORD_HASH`, clears `PANEL_PASSWORD`, and restarts the API service.

## OCI Configuration

Preferred production flow:

1. Log in to the web console.
2. Open Profile management.
3. Paste the OCI profile block.
4. Paste or reference the PEM private key.
5. Test connection from the UI.

The installer also has an OCI env fallback option for simple deployments. It writes:

- `OCI_EXECUTION_MODE=oci`
- `OCI_TENANCY_OCID`
- `OCI_USER_OCID`
- `OCI_FINGERPRINT`
- `OCI_REGION`
- `OCI_PRIVATE_KEY_FILE`

Do not place PEM files inside the Git repository.

## Files Created

```text
/opt/a-series-oracle/
  bin/a-series-oracle
  bin/panel-password
  src/
  www/

/etc/a-series-oracle/
  panel.env
  profiles.json

/etc/systemd/system/a-series-oracle.service
/etc/nginx/sites-available/a-series-oracle.conf
```

The nginx file is only created when `USE_NGINX=true` or when `USE_NGINX=auto` detects nginx already installed.

## Verify

```bash
systemctl status a-series-oracle --no-pager
systemctl status nginx --no-pager
curl -i http://127.0.0.1/api/health
```

When panel auth is enabled, protected API routes return `401` until you log in through the web console.

## Uninstall

```bash
sudo bash scripts/install.sh uninstall
```

The uninstall flow removes the app directory, service file, and nginx site. It asks before removing `/etc/a-series-oracle` because that directory contains encrypted profile data and panel secrets.
