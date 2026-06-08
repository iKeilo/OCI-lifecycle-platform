#!/usr/bin/env bash
set -Eeuo pipefail

APP_NAME="oci-lifecycle-platform"
APP_TITLE="OCI Lifecycle Platform"
REPO_URL="${OCI_LIFECYCLE_REPO_URL:-https://github.com/iKeilo/OCI-lifecycle-platform.git}"
APP_DIR="${APP_DIR:-/opt/oci-lifecycle-platform-docker}"
ENV_DIR="${ENV_DIR:-/etc/oci-lifecycle-platform}"
ENV_FILE="${ENV_FILE:-$ENV_DIR/docker.env}"
KEY_DIR="${OCI_KEY_DIR:-$ENV_DIR/keys}"
SRC_DIR="$APP_DIR/src"
COMPOSE_FILE="$SRC_DIR/docker-compose.yml"
DEFAULT_IMAGE="oci-lifecycle-platform:local"
DEFAULT_WEB_PORT="18080"

ACTION="${1:-menu}"
ASSUME_YES="${ASSUME_YES:-0}"

if [ "$ACTION" = "-y" ]; then
  ACTION="${2:-menu}"
  ASSUME_YES=1
fi

if [ "${2:-}" = "-y" ]; then
  ASSUME_YES=1
fi

log() {
  printf '\033[1;34m[%s]\033[0m %s\n' "$APP_NAME" "$*"
}

warn() {
  printf '\033[1;33m[%s]\033[0m %s\n' "$APP_NAME" "$*" >&2
}

die() {
  printf '\033[1;31m[%s]\033[0m %s\n' "$APP_NAME" "$*" >&2
  exit 1
}

require_root() {
  if [ "$(id -u)" -ne 0 ]; then
    die "Please run as root, for example: sudo bash scripts/docker-install.sh"
  fi
}

confirm() {
  local prompt="$1"
  if [ "$ASSUME_YES" = "1" ]; then
    return 0
  fi
  read -r -p "$prompt [y/N] " answer
  case "$answer" in
    y|Y|yes|YES) return 0 ;;
    *) return 1 ;;
  esac
}

ensure_command() {
  command -v "$1" >/dev/null 2>&1
}

ensure_base_tools() {
  if ensure_command apt-get; then
    apt-get update
    apt-get install -y git curl ca-certificates openssl tar
  elif ensure_command dnf; then
    dnf install -y git curl ca-certificates openssl tar
  elif ensure_command yum; then
    yum install -y git curl ca-certificates openssl tar
  else
    ensure_command git || die "git is required"
    ensure_command curl || die "curl is required"
    ensure_command openssl || die "openssl is required"
    ensure_command tar || die "tar is required"
  fi
}

ensure_docker() {
  if ensure_command docker && docker compose version >/dev/null 2>&1; then
    return 0
  fi

  log "Docker or docker compose was not found. Installing Docker packages..."
  if ensure_command apt-get; then
    apt-get update
    apt-get install -y docker.io docker-compose-plugin
  elif ensure_command dnf; then
    dnf install -y docker docker-compose-plugin
  elif ensure_command yum; then
    yum install -y docker docker-compose-plugin
  else
    die "Cannot install Docker automatically on this OS. Install Docker Engine and docker compose plugin first."
  fi

  if ensure_command systemctl; then
    systemctl enable --now docker >/dev/null 2>&1 || true
  else
    service docker start >/dev/null 2>&1 || true
  fi

  ensure_command docker || die "Docker installation failed"
  docker compose version >/dev/null 2>&1 || die "docker compose plugin installation failed"
}

sync_source() {
  mkdir -p "$APP_DIR"

  if [ -d "$SRC_DIR/.git" ]; then
    log "Updating source at $SRC_DIR"
    git -C "$SRC_DIR" fetch --all --prune
    git -C "$SRC_DIR" checkout main
    git -C "$SRC_DIR" pull --ff-only
    return 0
  fi

  if [ -f "./package.json" ] && [ -d "./backend" ]; then
    log "Copying current source tree to $SRC_DIR"
    rm -rf "$SRC_DIR"
    mkdir -p "$SRC_DIR"
    tar \
      --exclude='.git' \
      --exclude='.codegraph' \
      --exclude='.runtime' \
      --exclude='node_modules' \
      --exclude='dist' \
      --exclude='*.pem' \
      --exclude='*.key' \
      -cf - . | tar -xf - -C "$SRC_DIR"
    return 0
  fi

  log "Cloning $REPO_URL to $SRC_DIR"
  git clone "$REPO_URL" "$SRC_DIR"
}

env_get() {
  local key="$1"
  if [ ! -f "$ENV_FILE" ]; then
    return 0
  fi
  grep -E "^${key}=" "$ENV_FILE" | tail -n 1 | cut -d= -f2-
}

escape_sed_value() {
  printf '%s' "$1" | sed -e 's/[\/&]/\\&/g'
}

env_set() {
  local key="$1"
  local value="$2"
  mkdir -p "$(dirname "$ENV_FILE")"
  touch "$ENV_FILE"
  local escaped
  escaped="$(escape_sed_value "$value")"
  if grep -qE "^${key}=" "$ENV_FILE"; then
    sed -i "s/^${key}=.*/${key}=${escaped}/" "$ENV_FILE"
  else
    printf '%s=%s\n' "$key" "$value" >> "$ENV_FILE"
  fi
}

random_hex() {
  openssl rand -hex 32
}

random_b64() {
  openssl rand -base64 32 | tr -d '\n'
}

ensure_env_file() {
  mkdir -p "$ENV_DIR" "$KEY_DIR"
  chmod 700 "$ENV_DIR" "$KEY_DIR"

  if [ ! -f "$ENV_FILE" ]; then
    log "Creating Docker environment file at $ENV_FILE"
    cp "$SRC_DIR/docker/.env.example" "$ENV_FILE"
    chmod 600 "$ENV_FILE"
  fi

  [ -n "$(env_get COMPOSE_PROJECT_NAME)" ] || env_set COMPOSE_PROJECT_NAME "$APP_NAME"
  [ -n "$(env_get OCI_LIFECYCLE_IMAGE)" ] || env_set OCI_LIFECYCLE_IMAGE "$DEFAULT_IMAGE"
  [ -n "$(env_get WEB_PORT)" ] || env_set WEB_PORT "$DEFAULT_WEB_PORT"
  [ -n "$(env_get TZ)" ] || env_set TZ "Asia/Shanghai"
  [ -n "$(env_get OCI_KEY_DIR)" ] || env_set OCI_KEY_DIR "$KEY_DIR"
  [ -n "$(env_get PROFILE_DATA_VOLUME)" ] || env_set PROFILE_DATA_VOLUME "$APP_NAME-profile-data"
  [ -n "$(env_get POSTGRES_DATA_VOLUME)" ] || env_set POSTGRES_DATA_VOLUME "$APP_NAME-postgres-data"
  [ -n "$(env_get PANEL_SESSION_SECRET)" ] || env_set PANEL_SESSION_SECRET "$(random_hex)"
  [ -n "$(env_get PROFILE_KEY_ENCRYPTION_KEY)" ] || env_set PROFILE_KEY_ENCRYPTION_KEY "$(random_b64)"
  [ -n "$(env_get OCI_EXECUTION_MODE)" ] || env_set OCI_EXECUTION_MODE "local"
}

compose() {
  local project
  project="$(env_get COMPOSE_PROJECT_NAME)"
  docker compose --project-name "${project:-$APP_NAME}" \
    --env-file "$ENV_FILE" \
    -f "$COMPOSE_FILE" "$@"
}

current_image() {
  local image
  image="$(env_get OCI_LIFECYCLE_IMAGE)"
  printf '%s' "${image:-$DEFAULT_IMAGE}"
}

build_image() {
  log "Building Docker image $(current_image)"
  compose build app
}

hash_password() {
  local password="$1"
  printf '%s' "$password" | docker run --rm -i "$(current_image)" /app/panel-password hash
}

ensure_panel_password() {
  if [ -n "$(env_get PANEL_PASSWORD_HASH)" ]; then
    return 0
  fi

  local password password2 hash
  if [ -n "${PANEL_PASSWORD:-}" ]; then
    password="$PANEL_PASSWORD"
  else
    while true; do
      read -r -s -p "Set panel login password: " password
      printf '\n'
      read -r -s -p "Confirm panel login password: " password2
      printf '\n'
      [ "$password" = "$password2" ] || { warn "Passwords do not match"; continue; }
      [ "${#password}" -ge 8 ] || { warn "Password must be at least 8 characters"; continue; }
      break
    done
  fi

  hash="$(hash_password "$password")"
  env_set PANEL_PASSWORD_HASH "$hash"
  chmod 600 "$ENV_FILE"
}

health_check() {
  local port url
  port="$(env_get WEB_PORT)"
  url="http://127.0.0.1:${port:-$DEFAULT_WEB_PORT}/api/health"
  log "Waiting for $url"
  for _ in $(seq 1 40); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      log "$APP_TITLE is ready: http://$(hostname -I | awk '{print $1}'):${port:-$DEFAULT_WEB_PORT}/"
      return 0
    fi
    sleep 2
  done
  warn "Health check did not pass yet. Showing recent logs."
  compose logs --tail=80 app || true
  return 1
}

install_or_update() {
  require_root
  ensure_base_tools
  ensure_docker
  sync_source
  ensure_env_file
  build_image
  ensure_panel_password
  log "Starting containers"
  compose up -d app
  health_check
}

change_password() {
  require_root
  ensure_docker
  [ -f "$ENV_FILE" ] || die "Environment file not found: $ENV_FILE"
  [ -f "$COMPOSE_FILE" ] || die "Compose file not found: $COMPOSE_FILE"
  build_image

  local password password2 hash
  read -r -s -p "New panel password: " password
  printf '\n'
  read -r -s -p "Confirm new panel password: " password2
  printf '\n'
  [ "$password" = "$password2" ] || die "Passwords do not match"
  [ "${#password}" -ge 8 ] || die "Password must be at least 8 characters"
  hash="$(hash_password "$password")"
  env_set PANEL_PASSWORD_HASH "$hash"
  compose up -d app
  log "Panel password updated"
}

configure_oci_env() {
  require_root
  [ -f "$ENV_FILE" ] || die "Environment file not found: $ENV_FILE"
  log "This config is only a runtime fallback. The preferred path is adding encrypted OCI Profiles in the Web UI."

  local tenancy user fingerprint region compartment key_path
  read -r -p "tenancy OCID: " tenancy
  read -r -p "user OCID: " user
  read -r -p "fingerprint: " fingerprint
  read -r -p "region, for example ap-chuncheon-1: " region
  read -r -p "compartment OCID: " compartment
  read -r -p "private key path inside container, for example /keys/oci.pem: " key_path

  env_set OCI_TENANCY_OCID "$tenancy"
  env_set OCI_USER_OCID "$user"
  env_set OCI_FINGERPRINT "$fingerprint"
  env_set OCI_REGION "$region"
  env_set OCI_COMPARTMENT_OCID "$compartment"
  env_set OCI_PRIVATE_KEY_PATH "$key_path"
  chmod 600 "$ENV_FILE"
  log "OCI fallback environment updated. Put the PEM under $KEY_DIR and restart the app."
}

start_app() {
  require_root
  ensure_docker
  [ -f "$ENV_FILE" ] || die "Environment file not found: $ENV_FILE"
  compose up -d app
  health_check
}

stop_app() {
  require_root
  ensure_docker
  [ -f "$ENV_FILE" ] || die "Environment file not found: $ENV_FILE"
  compose stop app
}

restart_app() {
  require_root
  stop_app || true
  start_app
}

status_logs() {
  require_root
  ensure_docker
  [ -f "$ENV_FILE" ] || die "Environment file not found: $ENV_FILE"
  compose ps
  compose logs --tail=120 app
}

backup_data() {
  require_root
  ensure_docker
  [ -f "$ENV_FILE" ] || die "Environment file not found: $ENV_FILE"

  local stamp backup_dir volume image
  stamp="$(date +%Y%m%d-%H%M%S)"
  backup_dir="$APP_DIR/backups/$stamp"
  volume="$(env_get PROFILE_DATA_VOLUME)"
  image="$(current_image)"
  mkdir -p "$backup_dir"
  chmod 700 "$backup_dir"
  cp "$ENV_FILE" "$backup_dir/docker.env"
  docker run --rm \
    -v "${volume:-$APP_NAME-profile-data}:/data:ro" \
    -v "$backup_dir:/backup" \
    "$image" /bin/sh -c "cd /data && tar -czf /backup/profile-data.tgz ."
  log "Backup created at $backup_dir"
}

uninstall_app() {
  require_root
  ensure_docker
  if [ -f "$ENV_FILE" ] && [ -f "$COMPOSE_FILE" ]; then
    compose down
  fi

  if confirm "Remove Docker volumes with profile data"; then
    if [ -f "$ENV_FILE" ] && [ -f "$COMPOSE_FILE" ]; then
      compose down -v || true
    fi
  fi

  if confirm "Remove source directory $APP_DIR"; then
    rm -rf "$APP_DIR"
  fi

  if confirm "Remove environment directory $ENV_DIR"; then
    rm -rf "$ENV_DIR"
  fi

  log "Uninstall finished"
}

show_menu() {
  cat <<MENU
$APP_TITLE Docker installer

1) Install / first setup
2) Update / rebuild
3) Change panel password
4) Configure OCI env fallback
5) Start
6) Stop
7) Restart
8) Status and logs
9) Backup env and profile data
10) Uninstall
q) Quit
MENU
  read -r -p "Select: " choice
  case "$choice" in
    1) install_or_update ;;
    2) install_or_update ;;
    3) change_password ;;
    4) configure_oci_env ;;
    5) start_app ;;
    6) stop_app ;;
    7) restart_app ;;
    8) status_logs ;;
    9) backup_data ;;
    10) uninstall_app ;;
    q|Q) exit 0 ;;
    *) die "Unknown option: $choice" ;;
  esac
}

case "$ACTION" in
  menu|"") show_menu ;;
  install) install_or_update ;;
  update) install_or_update ;;
  password|change-password) change_password ;;
  configure-oci|oci) configure_oci_env ;;
  start) start_app ;;
  stop) stop_app ;;
  restart) restart_app ;;
  status|logs) status_logs ;;
  backup) backup_data ;;
  uninstall) uninstall_app ;;
  *) die "Unknown action: $ACTION" ;;
esac
