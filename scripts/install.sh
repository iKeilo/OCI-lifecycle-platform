#!/usr/bin/env bash
set -Eeuo pipefail

APP_NAME="a-series-oracle"
APP_TITLE="A-Series Oracle"
APP_DIR="${APP_DIR:-/opt/a-series-oracle}"
SRC_DIR="$APP_DIR/src"
BIN_DIR="$APP_DIR/bin"
WWW_DIR="$APP_DIR/www"
ENV_DIR="${ENV_DIR:-/etc/a-series-oracle}"
ENV_FILE="$ENV_DIR/panel.env"
SERVICE_NAME="${SERVICE_NAME:-a-series-oracle}"
SYSTEMD_DIR="${SYSTEMD_DIR:-/etc/systemd/system}"
SERVICE_FILE="${SYSTEMD_DIR}/${SERVICE_NAME}.service"
REPO_URL="${A_SERIES_ORACLE_REPO_URL:-}"
BRANCH="${A_SERIES_ORACLE_BRANCH:-main}"
WEB_PORT="${WEB_PORT:-80}"
USE_NGINX="${USE_NGINX:-auto}"
GO_ROOT="${GO_ROOT:-$APP_DIR/.toolchain/go}"
BACKUP_DIR="${BACKUP_DIR:-/root}"
SOURCE_OVERRIDE=""
YES=0
ACTION=""
USE_NGINX_RESOLVED="false"

log() {
  printf '\033[1;32m[ok]\033[0m %s\n' "$*"
}

warn() {
  printf '\033[1;33m[warn]\033[0m %s\n' "$*" >&2
}

die() {
  printf '\033[1;31m[error]\033[0m %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'USAGE'
Usage:
  sudo bash scripts/install.sh
  sudo bash scripts/install.sh install --source /path/to/repo
  sudo bash scripts/install.sh update
  sudo bash scripts/install.sh change-password
  sudo bash scripts/install.sh uninstall

Environment:
  A_SERIES_ORACLE_REPO_URL   Git repository used when not installing from a local source tree.
  A_SERIES_ORACLE_BRANCH     Git branch, default main.
  PANEL_PASSWORD             Non-interactive install/change-password password input.
  WEB_PORT                   nginx listen port, default 80.
  USE_NGINX                  true, false, or auto. auto uses nginx only when already installed.
  GO_PROXY                   Optional Go module proxy, passed to GOPROXY.
  GO_ROOT                    Go toolchain directory, default APP_DIR/.toolchain/go.
  SYSTEMD_DIR                systemd unit directory, default /etc/systemd/system.
  BACKUP_DIR                 Backup output directory, default /root.
  APP_DIR                    Install directory, default /opt/a-series-oracle.
  ENV_DIR                    Config directory, default /etc/a-series-oracle.
USAGE
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      install|update|change-password|configure-oci|start|stop|restart|status|logs|backup|uninstall)
        ACTION="$1"
        shift
        ;;
      --source)
        SOURCE_OVERRIDE="${2:-}"
        [[ -n "$SOURCE_OVERRIDE" ]] || die "--source requires a path"
        shift 2
        ;;
      --repo)
        REPO_URL="${2:-}"
        [[ -n "$REPO_URL" ]] || die "--repo requires a URL"
        shift 2
        ;;
      --branch)
        BRANCH="${2:-}"
        [[ -n "$BRANCH" ]] || die "--branch requires a branch"
        shift 2
        ;;
      -y|--yes)
        YES=1
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die "unknown argument: $1"
        ;;
    esac
  done
}

require_root() {
  [[ "${EUID:-$(id -u)}" -eq 0 ]] || die "please run as root or with sudo"
}

confirm() {
  local prompt="$1"
  if [[ "$YES" -eq 1 ]]; then
    return 0
  fi
  read -r -p "$prompt [y/N] " answer
  [[ "$answer" == "y" || "$answer" == "Y" ]]
}

install_base_packages() {
  local required=(curl openssl tar)
  if [[ -z "$SOURCE_OVERRIDE" && -z "$(current_source_dir || true)" ]]; then
    required+=(git)
  fi
  if [[ "$USE_NGINX_RESOLVED" == "true" ]]; then
    required+=(nginx)
  fi
  local missing=()
  local cmd
  for cmd in "${required[@]}"; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
      missing+=("$cmd")
    fi
  done
  if [[ "${#missing[@]}" -eq 0 ]]; then
    log "required system commands are already installed"
    return
  fi
  command -v apt-get >/dev/null 2>&1 || die "missing commands (${missing[*]}) and apt-get is unavailable"
  export DEBIAN_FRONTEND=noninteractive
  apt-get update
  apt-get install -y ca-certificates "${missing[@]}"
}

resolve_web_stack() {
  case "$USE_NGINX" in
    true)
      USE_NGINX_RESOLVED="true"
      ;;
    false)
      USE_NGINX_RESOLVED="false"
      ;;
    auto)
      if command -v nginx >/dev/null 2>&1; then
        USE_NGINX_RESOLVED="true"
      else
        USE_NGINX_RESOLVED="false"
      fi
      ;;
    *)
      die "USE_NGINX must be true, false, or auto"
      ;;
  esac
}

version_ge() {
  [[ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" == "$2" ]]
}

required_go_version() {
  local source="$1"
  awk '/^go / {print $2; exit}' "$source/backend/go.mod"
}

go_version_ok() {
  local required="$1"
  if [[ -x "$GO_ROOT/bin/go" ]]; then
    export PATH="$GO_ROOT/bin:$PATH"
  fi
  command -v go >/dev/null 2>&1 || return 1
  local current
  current="$(go version | awk '{print $3}' | sed 's/^go//')"
  version_ge "$current" "$required"
}

install_go() {
  local required="$1"
  if go_version_ok "$required"; then
    log "Go $(go version | awk '{print $3}') is ready"
    return
  fi
  warn "installing Go from go.dev because the current version is missing or older than $required"
  local arch tarball version url tmp parent
  arch="$(go_download_arch)"
  tmp="$(mktemp -d)"
  curl -fsSL 'https://go.dev/dl/?mode=json' -o "$tmp/go-downloads.json"
  tarball="$(grep -m1 -o "go[0-9.]*\\.linux-${arch}\\.tar\\.gz" "$tmp/go-downloads.json")"
  [[ -n "$tarball" ]] || die "cannot resolve latest Go linux-${arch} tarball"
  version="${tarball#go}"
  version="${version%.linux-${arch}.tar.gz}"
  version_ge "$version" "$required" || die "latest Go $version is older than required $required"
  url="https://go.dev/dl/${tarball}"
  curl -fsSL "$url" -o "$tmp/$tarball"
  parent="$(dirname "$GO_ROOT")"
  mkdir -p "$parent"
  rm -rf "$GO_ROOT"
  tar -C "$parent" -xzf "$tmp/$tarball"
  if [[ "$parent/go" != "$GO_ROOT" ]]; then
    mv "$parent/go" "$GO_ROOT"
  fi
  export PATH="$GO_ROOT/bin:$PATH"
  rm -rf "$tmp"
  log "installed Go $version"
}

go_download_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'amd64\n' ;;
    aarch64|arm64) printf 'arm64\n' ;;
    *) die "unsupported CPU architecture for Go installer: $(uname -m)" ;;
  esac
}

node_version_ok() {
  command -v node >/dev/null 2>&1 || return 1
  local major
  major="$(node -v | sed 's/^v//' | cut -d. -f1)"
  [[ "$major" -ge 20 ]]
}

install_node() {
  if node_version_ok; then
    log "Node $(node -v) is ready"
    return
  fi
  warn "installing Node.js 22 from NodeSource"
  curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
  apt-get install -y nodejs
}

current_source_dir() {
  local here
  here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
  if [[ -f "$here/package.json" && -f "$here/backend/go.mod" ]]; then
    printf '%s\n' "$here"
  fi
}

sync_source() {
  mkdir -p "$APP_DIR"
  local source="${SOURCE_OVERRIDE:-}"
  if [[ -z "$source" ]]; then
    source="$(current_source_dir || true)"
  fi
  if [[ -n "$source" ]]; then
    [[ -f "$source/package.json" && -f "$source/backend/go.mod" ]] || die "source path is not a project tree: $source"
    rm -rf "$SRC_DIR"
    mkdir -p "$SRC_DIR"
    tar -C "$source" -cf - \
      --exclude '.git' \
      --exclude '.codegraph' \
      --exclude '.runtime' \
      --exclude 'node_modules' \
      --exclude 'dist' \
      --exclude '*.log' \
      . | tar -C "$SRC_DIR" -xf -
    log "source copied from $source"
    return
  fi

  [[ -n "$REPO_URL" ]] || die "A_SERIES_ORACLE_REPO_URL is required when installing outside a source tree"
  if [[ -d "$SRC_DIR/.git" ]]; then
    git -C "$SRC_DIR" fetch --all --prune
    git -C "$SRC_DIR" checkout "$BRANCH"
    git -C "$SRC_DIR" pull --ff-only origin "$BRANCH"
  else
    rm -rf "$SRC_DIR"
    git clone --branch "$BRANCH" "$REPO_URL" "$SRC_DIR"
  fi
  log "source synced from $REPO_URL#$BRANCH"
}

env_get() {
  local key="$1"
  [[ -f "$ENV_FILE" ]] || return 0
  grep -E "^${key}=" "$ENV_FILE" | tail -n1 | cut -d= -f2- || true
}

env_set() {
  local key="$1"
  local value="$2"
  mkdir -p "$ENV_DIR"
  touch "$ENV_FILE"
  chmod 600 "$ENV_FILE"
  local escaped
  escaped="$(printf '%s' "$value" | sed -e 's/[\/&]/\\&/g')"
  if grep -qE "^${key}=" "$ENV_FILE"; then
    sed -i "s/^${key}=.*/${key}=${escaped}/" "$ENV_FILE"
  else
    printf '%s=%s\n' "$key" "$value" >>"$ENV_FILE"
  fi
}

ensure_env_defaults() {
  mkdir -p "$ENV_DIR"
  chmod 700 "$ENV_DIR"
  touch "$ENV_FILE"
  chmod 600 "$ENV_FILE"
  [[ -n "$(env_get PORT)" ]] || env_set PORT "8080"
  [[ -n "$(env_get OCI_EXECUTION_MODE)" ]] || env_set OCI_EXECUTION_MODE "local"
  [[ -n "$(env_get PROFILE_STORE_FILE)" ]] || env_set PROFILE_STORE_FILE "$ENV_DIR/profiles.json"
  [[ -n "$(env_get PROFILE_KEY_ENCRYPTION_KEY)" ]] || env_set PROFILE_KEY_ENCRYPTION_KEY "$(openssl rand -base64 32)"
  [[ -n "$(env_get PANEL_SESSION_SECRET)" ]] || env_set PANEL_SESSION_SECRET "$(openssl rand -hex 32)"
  [[ -n "$(env_get PANEL_AUTH_DISABLED)" ]] || env_set PANEL_AUTH_DISABLED "false"
}

read_password() {
  if [[ -n "${PANEL_PASSWORD:-}" ]]; then
    printf '%s\n' "$PANEL_PASSWORD"
    return
  fi
  local first second
  read -r -s -p "Set panel login password: " first
  printf '\n'
  read -r -s -p "Repeat panel login password: " second
  printf '\n'
  [[ "$first" == "$second" ]] || die "passwords do not match"
  printf '%s\n' "$first"
}

hash_password_with_source() {
  local password="$1"
  printf '%s' "$password" | (cd "$SRC_DIR/backend" && go run ./cmd/panel-password hash)
}

hash_password_with_binary() {
  local password="$1"
  if [[ -x "$BIN_DIR/panel-password" ]]; then
    printf '%s' "$password" | "$BIN_DIR/panel-password" hash
  else
    hash_password_with_source "$password"
  fi
}

ensure_panel_password() {
  local force="${1:-no}"
  if [[ "$force" != "yes" && -n "$(env_get PANEL_PASSWORD_HASH)" ]]; then
    return
  fi
  local password hash
  password="$(read_password)"
  hash="$(hash_password_with_source "$password")"
  env_set PANEL_PASSWORD_HASH "$hash"
  env_set PANEL_PASSWORD ""
  log "panel password hash updated"
}

build_app() {
  mkdir -p "$BIN_DIR" "$WWW_DIR"
  local required_go
  required_go="$(required_go_version "$SRC_DIR")"
  install_go "$required_go"
  install_node
  mkdir -p "$APP_DIR/.npm-cache" "$APP_DIR/.gocache" "$APP_DIR/.gomodcache"
  (cd "$SRC_DIR" && npm ci --cache "$APP_DIR/.npm-cache" && npm run build)
  export GOCACHE="$APP_DIR/.gocache"
  export GOMODCACHE="$APP_DIR/.gomodcache"
  if [[ -n "${GO_PROXY:-}" ]]; then
    export GOPROXY="$GO_PROXY"
  fi
  (cd "$SRC_DIR/backend" && go build -o "$BIN_DIR/a-series-oracle" ./cmd/server)
  (cd "$SRC_DIR/backend" && go build -o "$BIN_DIR/panel-password" ./cmd/panel-password)
  rm -rf "$WWW_DIR"
  mkdir -p "$WWW_DIR"
  cp -a "$SRC_DIR/dist/." "$WWW_DIR/"
  log "application built"
}

write_systemd() {
  mkdir -p "$SYSTEMD_DIR"
  cat >"$SERVICE_FILE" <<EOF
[Unit]
Description=${APP_TITLE} OCI lifecycle API
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=${ENV_FILE}
WorkingDirectory=${APP_DIR}
ExecStart=${BIN_DIR}/a-series-oracle
Restart=on-failure
RestartSec=3
User=root

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  if [[ "$SYSTEMD_DIR" == "/etc/systemd/system" ]]; then
    systemctl enable "$SERVICE_NAME"
  else
    warn "service is installed in $SYSTEMD_DIR and will not be enabled persistently"
  fi
}

nginx_target_file() {
  if [[ -d /etc/nginx/sites-available ]]; then
    printf '/etc/nginx/sites-available/%s.conf\n' "$APP_NAME"
  else
    printf '/etc/nginx/conf.d/%s.conf\n' "$APP_NAME"
  fi
}

write_nginx() {
  [[ "$USE_NGINX_RESOLVED" == "true" ]] || return 0
  local target
  target="$(nginx_target_file)"
  local api_port
  api_port="$(env_get PORT)"
  cat >"$target" <<EOF
server {
    listen ${WEB_PORT};
    server_name _;

    root ${WWW_DIR};
    index index.html;

    location /api/ {
        proxy_pass http://127.0.0.1:${api_port};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location / {
        try_files \$uri \$uri/ /index.html;
    }
}
EOF
  if [[ -d /etc/nginx/sites-enabled ]]; then
    ln -sf "$target" "/etc/nginx/sites-enabled/${APP_NAME}.conf"
  fi
  nginx -t
  systemctl enable nginx
}

restart_services() {
  systemctl restart "$SERVICE_NAME"
  if [[ "$USE_NGINX_RESOLVED" == "true" ]]; then
    systemctl restart nginx
  fi
  log "services restarted"
}

install_or_update() {
  resolve_web_stack
  install_base_packages
  sync_source
  ensure_env_defaults
  env_set STATIC_DIR "$WWW_DIR"
  if [[ "$USE_NGINX_RESOLVED" == "true" ]]; then
    env_set PORT "8080"
  else
    env_set PORT "$WEB_PORT"
  fi
  build_app
  ensure_panel_password "no"
  write_systemd
  write_nginx
  restart_services
  log "panel is available at http://$(hostname -I | awk '{print $1}'):${WEB_PORT}/"
}

change_password() {
  [[ -f "$ENV_FILE" ]] || die "not installed: missing $ENV_FILE"
  [[ -d "$SRC_DIR" || -x "$BIN_DIR/panel-password" ]] || die "not installed: missing password helper"
  local password hash
  password="$(read_password)"
  hash="$(hash_password_with_binary "$password")"
  env_set PANEL_PASSWORD_HASH "$hash"
  env_set PANEL_PASSWORD ""
  systemctl restart "$SERVICE_NAME"
  log "panel password changed"
}

configure_oci_env() {
  ensure_env_defaults
  local tenancy user fingerprint region key_file
  read -r -p "OCI tenancy OCID: " tenancy
  read -r -p "OCI user OCID: " user
  read -r -p "OCI fingerprint: " fingerprint
  read -r -p "OCI region [ap-chuncheon-1]: " region
  read -r -p "OCI private key file path on this server: " key_file
  region="${region:-ap-chuncheon-1}"
  [[ -f "$key_file" ]] || warn "private key file does not exist yet: $key_file"
  env_set OCI_EXECUTION_MODE "oci"
  env_set OCI_TENANCY_OCID "$tenancy"
  env_set OCI_USER_OCID "$user"
  env_set OCI_FINGERPRINT "$fingerprint"
  env_set OCI_REGION "$region"
  env_set OCI_PRIVATE_KEY_FILE "$key_file"
  systemctl restart "$SERVICE_NAME" || true
  log "OCI env fallback updated. Web Profile repository is still the preferred way to store OCI keys."
}

service_status() {
  systemctl --no-pager status "$SERVICE_NAME" || true
  if [[ "$USE_NGINX_RESOLVED" == "true" ]]; then
    systemctl --no-pager status nginx || true
  fi
  journalctl -u "$SERVICE_NAME" -n 80 --no-pager || true
}

backup_config() {
  [[ -d "$ENV_DIR" ]] || die "missing config directory: $ENV_DIR"
  mkdir -p "$BACKUP_DIR"
  local backup="$BACKUP_DIR/${APP_NAME}-config-$(date +%Y%m%d-%H%M%S).tar.gz"
  tar -C "$(dirname "$ENV_DIR")" -czf "$backup" "$(basename "$ENV_DIR")"
  chmod 600 "$backup"
  log "config backup written to $backup"
  warn "backup contains secrets; keep it private"
}

uninstall_app() {
  confirm "Uninstall ${APP_TITLE} from this server?" || return
  systemctl stop "$SERVICE_NAME" 2>/dev/null || true
  systemctl disable "$SERVICE_NAME" 2>/dev/null || true
  rm -f "$SERVICE_FILE"
  systemctl daemon-reload
  local target
  target="$(nginx_target_file)"
  rm -f "$target" "/etc/nginx/sites-enabled/${APP_NAME}.conf"
  if command -v nginx >/dev/null 2>&1; then
    systemctl reload nginx 2>/dev/null || true
  fi
  rm -rf "$APP_DIR"
  if confirm "Remove config and encrypted profile store in $ENV_DIR?"; then
    rm -rf "$ENV_DIR"
  else
    warn "config preserved at $ENV_DIR"
  fi
  log "uninstall complete"
}

menu() {
  cat <<MENU

${APP_TITLE} one-click installer
1) Install / first setup
2) Update application
3) Change panel login password
4) Configure OCI env fallback
5) Start services
6) Stop services
7) Restart services
8) Status and logs
9) Backup local config
10) Uninstall
MENU
  read -r -p "Choose an option [1-10]: " choice
  case "$choice" in
    1) ACTION="install" ;;
    2) ACTION="update" ;;
    3) ACTION="change-password" ;;
    4) ACTION="configure-oci" ;;
    5) ACTION="start" ;;
    6) ACTION="stop" ;;
    7) ACTION="restart" ;;
    8) ACTION="status" ;;
    9) ACTION="backup" ;;
    10) ACTION="uninstall" ;;
    *) die "invalid option" ;;
  esac
}

run_action() {
  resolve_web_stack
  case "$ACTION" in
    install|update) install_or_update ;;
    change-password) change_password ;;
    configure-oci) configure_oci_env ;;
    start)
      systemctl start "$SERVICE_NAME"
      if [[ "$USE_NGINX_RESOLVED" == "true" ]]; then
        systemctl start nginx
      fi
      log "services started"
      ;;
    stop) systemctl stop "$SERVICE_NAME"; log "application stopped" ;;
    restart) restart_services ;;
    status|logs) service_status ;;
    backup) backup_config ;;
    uninstall) uninstall_app ;;
    *) usage; exit 1 ;;
  esac
}

parse_args "$@"
require_root
if [[ -z "$ACTION" ]]; then
  menu
fi
run_action
