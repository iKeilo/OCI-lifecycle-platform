#!/usr/bin/env bash
set -Eeuo pipefail

APP_NAME="oci-lifecycle-platform"
APP_TITLE="OCI Lifecycle Platform"
WEB_PORT_INPUT="${WEB_PORT:-}"
DEPLOY_MODE="${DEPLOY_MODE:-docker}"
APP_DIR="${APP_DIR:-/opt/oci-lifecycle-platform}"
SRC_DIR="$APP_DIR/src"
BIN_DIR="$APP_DIR/bin"
WWW_DIR="$APP_DIR/www"
ENV_DIR="${ENV_DIR:-/etc/oci-lifecycle-platform}"
ENV_FILE="$ENV_DIR/panel.env"
DOCKER_APP_DIR="${DOCKER_APP_DIR:-/opt/oci-lifecycle-platform-docker}"
DOCKER_SRC_DIR="$DOCKER_APP_DIR/src"
DOCKER_COMPOSE_FILE="$DOCKER_SRC_DIR/docker-compose.yml"
DOCKER_ENV_FILE="${DOCKER_ENV_FILE:-$ENV_DIR/docker.env}"
DOCKER_KEY_DIR="${OCI_KEY_DIR:-$ENV_DIR/keys}"
DOCKER_PACKAGE_IMAGE="${OCI_LIFECYCLE_PACKAGE_IMAGE:-ghcr.io/ikeilo/oci-lifecycle-platform}"
DOCKER_PACKAGE_TAG="${OCI_LIFECYCLE_IMAGE_TAG:-latest}"
DOCKER_USE_PACKAGE="${OCI_LIFECYCLE_USE_PACKAGE:-true}"
if [[ "$DOCKER_USE_PACKAGE" == "true" && -z "${OCI_LIFECYCLE_IMAGE:-}" ]]; then
  DOCKER_IMAGE="${DOCKER_PACKAGE_IMAGE}:${DOCKER_PACKAGE_TAG}"
else
  DOCKER_IMAGE="${OCI_LIFECYCLE_IMAGE:-oci-lifecycle-platform:local}"
fi
DOCKER_WEB_PORT="${WEB_PORT_INPUT:-18080}"
DOCKER_WEB_PORT_EXPLICIT=0
[[ -n "$WEB_PORT_INPUT" ]] && DOCKER_WEB_PORT_EXPLICIT=1
SERVICE_NAME="${SERVICE_NAME:-oci-lifecycle-platform}"
SYSTEMD_DIR="${SYSTEMD_DIR:-/etc/systemd/system}"
SERVICE_FILE="${SYSTEMD_DIR}/${SERVICE_NAME}.service"
REPO_URL="${OCI_LIFECYCLE_REPO_URL:-${A_SERIES_ORACLE_REPO_URL:-https://github.com/iKeilo/OCI-lifecycle-platform.git}}"
BRANCH="${OCI_LIFECYCLE_BRANCH:-${A_SERIES_ORACLE_BRANCH:-main}}"
WEB_PORT="${WEB_PORT_INPUT:-80}"
USE_NGINX="${USE_NGINX:-auto}"
GO_ROOT="${GO_ROOT:-$APP_DIR/.toolchain/go}"
BACKUP_DIR="${BACKUP_DIR:-/root}"
SOURCE_OVERRIDE=""
PREBUILT_OVERRIDE=""
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
  sudo bash scripts/install.sh install
  sudo bash scripts/install.sh update
  sudo bash scripts/install.sh change-password
  sudo bash scripts/install.sh uninstall
  sudo bash scripts/install.sh --systemd install
  sudo DEPLOY_MODE=systemd bash scripts/install.sh install --source /path/to/repo
  sudo bash scripts/install.sh --systemd install --prebuilt /path/to/release-package

Environment:
  DEPLOY_MODE                 docker or systemd. Default docker.
  OCI_LIFECYCLE_REPO_URL     Git repository used when not installing from a local source tree.
  OCI_LIFECYCLE_BRANCH       Git branch, default main.
  A_SERIES_ORACLE_REPO_URL   Backward-compatible alias for OCI_LIFECYCLE_REPO_URL.
  A_SERIES_ORACLE_BRANCH     Backward-compatible alias for OCI_LIFECYCLE_BRANCH.
  PANEL_PASSWORD             Non-interactive install/change-password password input.
  WEB_PORT                   web listen port. Docker default 18080, systemd default 80.
  USE_NGINX                  true, false, or auto. auto uses nginx only when already installed.
  GO_PROXY                   Optional Go module proxy, passed to GOPROXY.
  OCI_LIFECYCLE_USE_PACKAGE  Docker mode only. Default true pulls GHCR image instead of building locally.
                             Set false to build from source on the server.
  OCI_LIFECYCLE_REQUIRE_PACKAGE
                             Docker mode only. Set true to fail instead of falling back to local build
                             when the GHCR package pull fails.
  OCI_LIFECYCLE_IMAGE        Docker image override. Default GHCR image in package mode.
  OCI_LIFECYCLE_IMAGE_TAG    GHCR image tag for package mode, default latest.
  GO_ROOT                    Go toolchain directory, default APP_DIR/.toolchain/go.
  SYSTEMD_DIR                systemd unit directory, default /etc/systemd/system.
  BACKUP_DIR                 Backup output directory, default /root.
  APP_DIR                    Install directory, default /opt/oci-lifecycle-platform.
  ENV_DIR                    Config directory, default /etc/oci-lifecycle-platform.
USAGE
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      install|update|change-password|configure-oci|start|stop|restart|status|logs|backup|uninstall)
        ACTION="$1"
        shift
        ;;
      docker|--docker)
        DEPLOY_MODE="docker"
        shift
        ;;
      systemd|--systemd)
        DEPLOY_MODE="systemd"
        shift
        ;;
      --source)
        SOURCE_OVERRIDE="${2:-}"
        [[ -n "$SOURCE_OVERRIDE" ]] || die "--source requires a path"
        shift 2
        ;;
      --prebuilt)
        PREBUILT_OVERRIDE="${2:-}"
        [[ -n "$PREBUILT_OVERRIDE" ]] || die "--prebuilt requires a path"
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
  if [[ -z "$PREBUILT_OVERRIDE" && -z "$SOURCE_OVERRIDE" && -z "$(current_source_dir || true)" ]]; then
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
  if [[ -n "$PREBUILT_OVERRIDE" ]]; then
    [[ -x "$PREBUILT_OVERRIDE/bin/oci-lifecycle-platform" && -x "$PREBUILT_OVERRIDE/bin/panel-password" && -d "$PREBUILT_OVERRIDE/www" ]] || die "prebuilt package is incomplete: $PREBUILT_OVERRIDE"
    log "using prebuilt package from $PREBUILT_OVERRIDE"
    return
  fi

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
  if [[ -n "${OCI_EXECUTION_MODE:-}" ]]; then
    env_set OCI_EXECUTION_MODE "$OCI_EXECUTION_MODE"
  elif [[ -z "$(env_get OCI_EXECUTION_MODE)" || "$(env_get OCI_EXECUTION_MODE)" == "local" ]]; then
    env_set OCI_EXECUTION_MODE "oci"
  fi
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
  if [[ ! -t 0 ]]; then
    first="$(openssl rand -hex 12)"
    print_generated_panel_password "$first"
    printf '%s\n' "$first"
    return
  fi

  read -r -s -p "Set panel login password, or press Enter to generate one: " first
  printf '\n'
  if [[ -z "$first" ]]; then
    first="$(openssl rand -hex 12)"
    print_generated_panel_password "$first"
    printf '%s\n' "$first"
    return
  fi

  read -r -s -p "Repeat panel login password: " second
  printf '\n'
  [[ "$first" == "$second" ]] || die "passwords do not match"
  printf '%s\n' "$first"
}

print_generated_panel_password() {
  local password="$1"
  printf '[ok] random panel password generated; it will be shown only once\n' >&2
  printf '[ok] panel login password: %s\n' "$password" >&2
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
  hash="$(hash_password_with_binary "$password")"
  env_set PANEL_PASSWORD_HASH "$hash"
  env_set PANEL_PASSWORD ""
  log "panel password hash updated"
}

build_app() {
  mkdir -p "$BIN_DIR" "$WWW_DIR"
  if [[ -n "$PREBUILT_OVERRIDE" ]]; then
    install -m 0755 "$PREBUILT_OVERRIDE/bin/oci-lifecycle-platform" "$BIN_DIR/oci-lifecycle-platform"
    install -m 0755 "$PREBUILT_OVERRIDE/bin/panel-password" "$BIN_DIR/panel-password"
    rm -rf "$WWW_DIR"
    mkdir -p "$WWW_DIR"
    cp -a "$PREBUILT_OVERRIDE/www/." "$WWW_DIR/"
    log "application installed from prebuilt package"
    return
  fi

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
  (cd "$SRC_DIR/backend" && go build -o "$BIN_DIR/oci-lifecycle-platform" ./cmd/server)
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
ExecStart=${BIN_DIR}/oci-lifecycle-platform
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

docker_env_get() {
  local key="$1"
  [[ -f "$DOCKER_ENV_FILE" ]] || return 0
  grep -E "^${key}=" "$DOCKER_ENV_FILE" | tail -n1 | cut -d= -f2- || true
}

docker_env_set() {
  local key="$1"
  local value="$2"
  mkdir -p "$(dirname "$DOCKER_ENV_FILE")"
  touch "$DOCKER_ENV_FILE"
  chmod 600 "$DOCKER_ENV_FILE"
  if [[ "$key" == "PANEL_PASSWORD_HASH" ]]; then
    value="$(printf '%s' "$value" | sed 's/\$/\$\$/g')"
  fi
  local escaped
  escaped="$(printf '%s' "$value" | sed -e 's/[\/&]/\\&/g')"
  if grep -qE "^${key}=" "$DOCKER_ENV_FILE"; then
    sed -i "s/^${key}=.*/${key}=${escaped}/" "$DOCKER_ENV_FILE"
  else
    printf '%s=%s\n' "$key" "$value" >>"$DOCKER_ENV_FILE"
  fi
}

docker_current_source_dir() {
  local here
  here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
  if [[ -f "$here/package.json" && -f "$here/backend/go.mod" ]]; then
    printf '%s\n' "$here"
  fi
}

docker_install_base_packages() {
  local commands=(git curl openssl tar)
  local missing=()
  local command_name
  for command_name in "${commands[@]}"; do
    if ! command -v "$command_name" >/dev/null 2>&1; then
      missing+=("$command_name")
    fi
  done
  if [[ "${#missing[@]}" -eq 0 ]]; then
    log "required Docker install commands are already installed"
    return
  fi

  if command -v apt-get >/dev/null 2>&1; then
    apt-get update
    apt-get install -y ca-certificates "${missing[@]}"
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y "${missing[@]}"
  elif command -v yum >/dev/null 2>&1; then
    yum install -y "${missing[@]}"
  else
    local cmd
    for cmd in "${missing[@]}"; do
      command -v "$cmd" >/dev/null 2>&1 || die "missing required command: $cmd"
    done
  fi
}

docker_ensure_engine() {
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    return
  fi

  warn "Docker or docker compose is missing; installing Docker packages"
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update
    apt-get install -y docker.io docker-compose-plugin
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y docker docker-compose-plugin
  elif command -v yum >/dev/null 2>&1; then
    yum install -y docker docker-compose-plugin
  else
    die "cannot install Docker automatically on this OS"
  fi

  if command -v systemctl >/dev/null 2>&1; then
    systemctl enable --now docker >/dev/null 2>&1 || true
  else
    service docker start >/dev/null 2>&1 || true
  fi

  command -v docker >/dev/null 2>&1 || die "docker installation failed"
  docker compose version >/dev/null 2>&1 || die "docker compose plugin installation failed"
}

docker_sync_source() {
  mkdir -p "$DOCKER_APP_DIR"
  local source="${SOURCE_OVERRIDE:-}"
  if [[ -z "$source" ]]; then
    source="$(docker_current_source_dir || true)"
  fi

  if [[ -n "$source" ]]; then
    [[ -f "$source/package.json" && -f "$source/backend/go.mod" ]] || die "source path is not a project tree: $source"
    rm -rf "$DOCKER_SRC_DIR"
    mkdir -p "$DOCKER_SRC_DIR"
    tar -C "$source" -cf - \
      --exclude '.git' \
      --exclude '.codegraph' \
      --exclude '.runtime' \
      --exclude 'node_modules' \
      --exclude 'dist' \
      --exclude '*.log' \
      --exclude '*.pem' \
      --exclude '*.key' \
      . | tar -C "$DOCKER_SRC_DIR" -xf -
    log "Docker source copied from $source"
    return
  fi

  if [[ -d "$DOCKER_SRC_DIR/.git" ]]; then
    git -C "$DOCKER_SRC_DIR" fetch --all --prune
    git -C "$DOCKER_SRC_DIR" checkout "$BRANCH"
    git -C "$DOCKER_SRC_DIR" pull --ff-only origin "$BRANCH"
  else
    rm -rf "$DOCKER_SRC_DIR"
    git clone --branch "$BRANCH" "$REPO_URL" "$DOCKER_SRC_DIR"
  fi
  log "Docker source synced from $REPO_URL#$BRANCH"
}

docker_ensure_env_defaults() {
  mkdir -p "$ENV_DIR" "$DOCKER_KEY_DIR"
  chmod 700 "$ENV_DIR" "$DOCKER_KEY_DIR"

  if [[ ! -f "$DOCKER_ENV_FILE" ]]; then
    [[ -f "$DOCKER_SRC_DIR/docker/.env.example" ]] || die "missing docker env template"
    cp "$DOCKER_SRC_DIR/docker/.env.example" "$DOCKER_ENV_FILE"
    chmod 600 "$DOCKER_ENV_FILE"
    log "created Docker env file at $DOCKER_ENV_FILE"
  fi

  [[ -n "$(docker_env_get COMPOSE_PROJECT_NAME)" ]] || docker_env_set COMPOSE_PROJECT_NAME "$APP_NAME"
  if [[ "$DOCKER_USE_PACKAGE" == "true" && -z "${OCI_LIFECYCLE_IMAGE:-}" ]]; then
    docker_env_set OCI_LIFECYCLE_IMAGE "$DOCKER_IMAGE"
  elif [[ -n "${OCI_LIFECYCLE_IMAGE:-}" ]]; then
    docker_env_set OCI_LIFECYCLE_IMAGE "$DOCKER_IMAGE"
  else
    [[ -n "$(docker_env_get OCI_LIFECYCLE_IMAGE)" ]] || docker_env_set OCI_LIFECYCLE_IMAGE "$DOCKER_IMAGE"
  fi
  if [[ -n "$WEB_PORT_INPUT" ]]; then
    docker_env_set WEB_PORT "$DOCKER_WEB_PORT"
  else
    [[ -n "$(docker_env_get WEB_PORT)" ]] || docker_env_set WEB_PORT "$DOCKER_WEB_PORT"
  fi
  [[ -n "$(docker_env_get TZ)" ]] || docker_env_set TZ "Asia/Shanghai"
  [[ -n "$(docker_env_get OCI_KEY_DIR)" ]] || docker_env_set OCI_KEY_DIR "$DOCKER_KEY_DIR"
  [[ -n "$(docker_env_get PROFILE_DATA_VOLUME)" ]] || docker_env_set PROFILE_DATA_VOLUME "$APP_NAME-profile-data"
  [[ -n "$(docker_env_get POSTGRES_DATA_VOLUME)" ]] || docker_env_set POSTGRES_DATA_VOLUME "$APP_NAME-postgres-data"
  [[ -n "$(docker_env_get POSTGRES_DB)" ]] || docker_env_set POSTGRES_DB "oci_lifecycle"
  [[ -n "$(docker_env_get POSTGRES_USER)" ]] || docker_env_set POSTGRES_USER "oci_lifecycle"
  [[ -n "$(docker_env_get POSTGRES_PASSWORD)" ]] || docker_env_set POSTGRES_PASSWORD "$(openssl rand -hex 24)"
  if [[ -z "$(docker_env_get DATABASE_URL)" ]]; then
    docker_env_set DATABASE_URL "postgres://$(docker_env_get POSTGRES_USER):$(docker_env_get POSTGRES_PASSWORD)@postgres:5432/$(docker_env_get POSTGRES_DB)?sslmode=disable"
  fi
  [[ -n "$(docker_env_get PANEL_SESSION_SECRET)" ]] || docker_env_set PANEL_SESSION_SECRET "$(openssl rand -hex 32)"
  [[ -n "$(docker_env_get PROFILE_KEY_ENCRYPTION_KEY)" ]] || docker_env_set PROFILE_KEY_ENCRYPTION_KEY "$(openssl rand -base64 32 | tr -d '\n')"
  if [[ -n "${OCI_EXECUTION_MODE:-}" ]]; then
    docker_env_set OCI_EXECUTION_MODE "$OCI_EXECUTION_MODE"
  elif [[ -z "$(docker_env_get OCI_EXECUTION_MODE)" || "$(docker_env_get OCI_EXECUTION_MODE)" == "local" ]]; then
    docker_env_set OCI_EXECUTION_MODE "oci"
  fi
}

docker_compose() {
  local project
  project="$(docker_env_get COMPOSE_PROJECT_NAME)"
  docker compose --project-name "${project:-$APP_NAME}" \
    --env-file "$DOCKER_ENV_FILE" \
    -f "$DOCKER_COMPOSE_FILE" "$@"
}

docker_project_name() {
  local project
  project="$(docker_env_get COMPOSE_PROJECT_NAME)"
  printf '%s\n' "${project:-$APP_NAME}"
}

docker_port_is_current_app() {
  local port="$1"
  local project
  project="$(docker_project_name)"
  docker ps --filter "name=^/${project}-app-1$" --format '{{.Ports}}' 2>/dev/null | grep -Eq "(0\.0\.0\.0|::|\[::\]):${port}->8080/tcp"
}

docker_port_in_use() {
  local port="$1"
  if command -v ss >/dev/null 2>&1 && ss -ltnH 2>/dev/null | awk '{print $4}' | grep -Eq "(^|:|\\])${port}$"; then
    return 0
  fi
  if docker ps --format '{{.Ports}}' 2>/dev/null | grep -Eq "(0\.0\.0\.0|::|\[::\]):${port}->"; then
    return 0
  fi
  return 1
}

docker_random_available_port() {
  local candidate
  for _ in $(seq 1 100); do
    candidate=$((20000 + RANDOM % 25000))
    if ! docker_port_in_use "$candidate"; then
      printf '%s\n' "$candidate"
      return
    fi
  done
  for candidate in $(seq 20000 49151); do
    if ! docker_port_in_use "$candidate"; then
      printf '%s\n' "$candidate"
      return
    fi
  done
  die "cannot find a free random web port"
}

docker_prompt_web_port() {
  if [[ -n "$WEB_PORT_INPUT" ]]; then
    docker_env_set WEB_PORT "$DOCKER_WEB_PORT"
    return
  fi

  [[ -t 0 ]] || return

  local current selected
  current="$(docker_env_get WEB_PORT)"
  current="${current:-$DOCKER_WEB_PORT}"
  read -r -p "Set web panel port, or press Enter for a random available port [current: ${current}]: " selected
  if [[ -z "$selected" ]]; then
    selected="$(docker_random_available_port)"
    warn "using random WEB_PORT=${selected}"
    DOCKER_WEB_PORT_EXPLICIT=0
  else
    [[ "$selected" =~ ^[0-9]+$ ]] || die "WEB_PORT must be a number"
    (( selected >= 1 && selected <= 65535 )) || die "WEB_PORT must be between 1 and 65535"
    DOCKER_WEB_PORT_EXPLICIT=1
  fi
  docker_env_set WEB_PORT "$selected"
}

docker_resolve_web_port() {
  local port candidate max
  port="$(docker_env_get WEB_PORT)"
  port="${port:-$DOCKER_WEB_PORT}"

  if docker_port_is_current_app "$port"; then
    docker_env_set WEB_PORT "$port"
    return
  fi

  if ! docker_port_in_use "$port"; then
    docker_env_set WEB_PORT "$port"
    return
  fi

  if [[ "$DOCKER_WEB_PORT_EXPLICIT" == "1" ]]; then
    die "WEB_PORT=${port} is already in use. Re-run with another port, for example: WEB_PORT=$((port + 1)) bash <(curl -L https://raw.githubusercontent.com/iKeilo/OCI-lifecycle-platform/main/panel_install.sh)"
  fi

  max=$((port + 100))
  for candidate in $(seq $((port + 1)) "$max") 19080 28080; do
    if ! docker_port_in_use "$candidate"; then
      warn "port ${port} is already in use; using WEB_PORT=${candidate} instead"
      docker_env_set WEB_PORT "$candidate"
      return
    fi
  done

  die "cannot find a free web port after ${port}; set WEB_PORT explicitly"
}

docker_current_image() {
  local image
  image="$(docker_env_get OCI_LIFECYCLE_IMAGE)"
  printf '%s\n' "${image:-$DOCKER_IMAGE}"
}

docker_build_image() {
  [[ -f "$DOCKER_COMPOSE_FILE" ]] || die "missing compose file: $DOCKER_COMPOSE_FILE"
  if [[ "$DOCKER_USE_PACKAGE" == "true" ]]; then
    log "pulling Docker package $(docker_current_image)"
    if docker pull "$(docker_current_image)"; then
      return
    fi
    if [[ "${OCI_LIFECYCLE_REQUIRE_PACKAGE:-false}" == "true" ]]; then
      die "failed to pull Docker package $(docker_current_image)"
    fi
    warn "failed to pull Docker package $(docker_current_image); falling back to local build from downloaded source"
  fi
  log "building Docker image $(docker_current_image)"
  docker_compose build app
}

docker_hash_password() {
  local password="$1"
  printf '%s' "$password" | docker run --rm -i "$(docker_current_image)" /app/panel-password hash
}

docker_ensure_panel_password() {
  local force="${1:-no}"
  if [[ "$force" != "yes" && -n "$(docker_env_get PANEL_PASSWORD_HASH)" ]]; then
    return
  fi
  local password hash
  password="$(read_password)"
  [[ "${#password}" -ge 8 ]] || die "password must be at least 8 characters"
  hash="$(docker_hash_password "$password")"
  docker_env_set PANEL_PASSWORD_HASH "$hash"
  log "Docker panel password hash updated"
}

docker_health_check() {
  local port url
  port="$(docker_env_get WEB_PORT)"
  url="http://127.0.0.1:${port:-18080}/api/health"
  log "waiting for $url"
  for _ in $(seq 1 40); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      log "panel is available at http://$(hostname -I | awk '{print $1}'):${port:-18080}/"
      return
    fi
    sleep 2
  done
  warn "Docker health check did not pass yet"
  docker_compose logs --tail=80 app || true
}

docker_install_or_update() {
  docker_install_base_packages
  docker_ensure_engine
  docker_sync_source
  docker_ensure_env_defaults
  docker_prompt_web_port
  docker_resolve_web_port
  docker_build_image
  docker_ensure_panel_password "no"
  docker_compose up -d app
  docker_health_check
}

docker_change_password() {
  [[ -f "$DOCKER_ENV_FILE" ]] || die "not installed: missing $DOCKER_ENV_FILE"
  docker_ensure_engine
  docker_resolve_web_port
  docker_build_image
  docker_ensure_panel_password "yes"
  docker_compose up -d app
}

docker_configure_oci_env() {
  [[ -f "$DOCKER_ENV_FILE" ]] || die "not installed: missing $DOCKER_ENV_FILE"
  local tenancy user fingerprint region compartment key_path
  read -r -p "OCI tenancy OCID: " tenancy
  read -r -p "OCI user OCID: " user
  read -r -p "OCI fingerprint: " fingerprint
  read -r -p "OCI region [ap-chuncheon-1]: " region
  read -r -p "OCI compartment OCID: " compartment
  read -r -p "Private key path inside container [/keys/oci.pem]: " key_path
  region="${region:-ap-chuncheon-1}"
  key_path="${key_path:-/keys/oci.pem}"
  docker_env_set OCI_EXECUTION_MODE "oci"
  docker_env_set OCI_TENANCY_OCID "$tenancy"
  docker_env_set OCI_USER_OCID "$user"
  docker_env_set OCI_FINGERPRINT "$fingerprint"
  docker_env_set OCI_REGION "$region"
  docker_env_set OCI_COMPARTMENT_OCID "$compartment"
  docker_env_set OCI_PRIVATE_KEY_PATH "$key_path"
  log "OCI Docker env fallback updated. Put PEM files under $DOCKER_KEY_DIR."
}

docker_start_app() {
  docker_ensure_engine
  [[ -f "$DOCKER_ENV_FILE" ]] || die "not installed: missing $DOCKER_ENV_FILE"
  docker_resolve_web_port
  docker_compose up -d app
  docker_health_check
}

docker_stop_app() {
  docker_ensure_engine
  [[ -f "$DOCKER_ENV_FILE" ]] || die "not installed: missing $DOCKER_ENV_FILE"
  docker_compose stop app
}

docker_status_logs() {
  docker_ensure_engine
  [[ -f "$DOCKER_ENV_FILE" ]] || die "not installed: missing $DOCKER_ENV_FILE"
  docker_compose ps
  docker_compose logs --tail=120 app
}

docker_backup_data() {
  docker_ensure_engine
  [[ -f "$DOCKER_ENV_FILE" ]] || die "not installed: missing $DOCKER_ENV_FILE"
  local stamp backup_dir volume image
  stamp="$(date +%Y%m%d-%H%M%S)"
  backup_dir="$DOCKER_APP_DIR/backups/$stamp"
  volume="$(docker_env_get PROFILE_DATA_VOLUME)"
  image="$(docker_current_image)"
  mkdir -p "$backup_dir"
  chmod 700 "$backup_dir"
  cp "$DOCKER_ENV_FILE" "$backup_dir/docker.env"
  docker run --rm \
    -v "${volume:-$APP_NAME-profile-data}:/data:ro" \
    -v "$backup_dir:/backup" \
    "$image" /bin/sh -c "cd /data && tar -czf /backup/profile-data.tgz ."
  log "Docker backup written to $backup_dir"
  warn "backup contains secrets; keep it private"
}

docker_uninstall_app() {
  docker_ensure_engine
  if [[ -f "$DOCKER_ENV_FILE" && -f "$DOCKER_COMPOSE_FILE" ]]; then
    docker_compose down
  fi
  if confirm "Remove Docker volumes with encrypted profile data?"; then
    if [[ -f "$DOCKER_ENV_FILE" && -f "$DOCKER_COMPOSE_FILE" ]]; then
      docker_compose down -v || true
    fi
  fi
  if confirm "Remove Docker source directory $DOCKER_APP_DIR?"; then
    rm -rf "$DOCKER_APP_DIR"
  fi
  if confirm "Remove config directory $ENV_DIR?"; then
    rm -rf "$ENV_DIR"
  fi
  log "Docker uninstall complete"
}

docker_menu() {
  cat <<MENU

${APP_TITLE} one-click installer - Docker mode
1) Install / first setup
2) Update from GitHub latest
3) Uninstall
4) Reset panel login password
MENU
  read -r -p "Choose an option [1-4]: " choice
  case "$choice" in
    1) ACTION="install" ;;
    2) ACTION="update" ;;
    3) ACTION="uninstall" ;;
    4) ACTION="change-password" ;;
    *) die "invalid option" ;;
  esac
}

run_docker_action() {
  case "$ACTION" in
    install|update) docker_install_or_update ;;
    change-password) docker_change_password ;;
    configure-oci) docker_configure_oci_env ;;
    start) docker_start_app ;;
    stop) docker_stop_app ;;
    restart) docker_stop_app || true; docker_start_app ;;
    status|logs) docker_status_logs ;;
    backup) docker_backup_data ;;
    uninstall) docker_uninstall_app ;;
    *) usage; exit 1 ;;
  esac
}

menu() {
  if [[ "$DEPLOY_MODE" == "docker" ]]; then
    docker_menu
    return
  fi

  cat <<MENU

${APP_TITLE} one-click installer - systemd mode
1) Install / first setup
2) Update from GitHub latest
3) Uninstall
4) Reset panel login password
MENU
  read -r -p "Choose an option [1-4]: " choice
  case "$choice" in
    1) ACTION="install" ;;
    2) ACTION="update" ;;
    3) ACTION="uninstall" ;;
    4) ACTION="change-password" ;;
    *) die "invalid option" ;;
  esac
}

run_action() {
  case "$DEPLOY_MODE" in
    docker)
      run_docker_action
      return
      ;;
    systemd) ;;
    *) die "DEPLOY_MODE must be docker or systemd" ;;
  esac

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
