#!/usr/bin/env bash
set -Eeuo pipefail

APP_NAME="oci-lifecycle-platform"
REPO_OWNER="${OCI_LIFECYCLE_REPO_OWNER:-iKeilo}"
REPO_NAME="${OCI_LIFECYCLE_REPO_NAME:-OCI-lifecycle-platform}"
BRANCH="${OCI_LIFECYCLE_BRANCH:-main}"
RAW_URL="${PANEL_LINUX_INSTALL_RAW_URL:-https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/${BRANCH}/panel_linux_install.sh}"
ARCHIVE_URL="${OCI_LIFECYCLE_ARCHIVE_URL:-https://github.com/${REPO_OWNER}/${REPO_NAME}/archive/refs/heads/${BRANCH}.tar.gz}"
RELEASE_DOWNLOAD_BASE="${OCI_LIFECYCLE_RELEASE_DOWNLOAD_BASE:-https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/latest/download}"
USE_PREBUILT="${OCI_LIFECYCLE_USE_PREBUILT:-true}"

usage() {
  cat <<USAGE
${APP_NAME} native Linux one-click installer

Default native Linux/systemd install:
  bash <(curl -L https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/${BRANCH}/panel_linux_install.sh)

Common commands:
  bash <(curl -L https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/${BRANCH}/panel_linux_install.sh) install
  bash <(curl -L https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/${BRANCH}/panel_linux_install.sh) update
  bash <(curl -L https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/${BRANCH}/panel_linux_install.sh) change-password
  bash <(curl -L https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/${BRANCH}/panel_linux_install.sh) uninstall

Environment:
  OCI_LIFECYCLE_BRANCH       GitHub branch, default main.
  OCI_LIFECYCLE_REPO_OWNER   GitHub owner, default iKeilo.
  OCI_LIFECYCLE_REPO_NAME    GitHub repo, default OCI-lifecycle-platform.
  OCI_LIFECYCLE_USE_PREBUILT true by default. Pulls release binaries for fast install.
  OCI_LIFECYCLE_RELEASE_DOWNLOAD_BASE
                             Release asset base URL, default latest/download.
  PANEL_PASSWORD             Optional non-interactive panel password.
  PANEL_PASSWORD_FILE        File used when a random panel password is generated.
  WEB_PORT                   Web port, default 80 in systemd mode.
  USE_NGINX                  true, false, or auto. Default auto.
  APP_DIR                    Install directory, default /opt/oci-lifecycle-platform.
  ENV_DIR                    Config directory, default /etc/oci-lifecycle-platform.
USAGE
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ "$(id -u)" -ne 0 ]]; then
  if command -v sudo >/dev/null 2>&1; then
    exec sudo env \
      OCI_LIFECYCLE_BRANCH="$BRANCH" \
      OCI_LIFECYCLE_REPO_OWNER="$REPO_OWNER" \
      OCI_LIFECYCLE_REPO_NAME="$REPO_NAME" \
      OCI_LIFECYCLE_USE_PREBUILT="$USE_PREBUILT" \
      OCI_LIFECYCLE_RELEASE_DOWNLOAD_BASE="$RELEASE_DOWNLOAD_BASE" \
      APP_DIR="${APP_DIR:-}" \
      ENV_DIR="${ENV_DIR:-}" \
      GO_PROXY="${GO_PROXY:-}" \
      GO_ROOT="${GO_ROOT:-}" \
      PANEL_PASSWORD="${PANEL_PASSWORD:-}" \
      PANEL_PASSWORD_FILE="${PANEL_PASSWORD_FILE:-}" \
      USE_NGINX="${USE_NGINX:-}" \
      WEB_PORT="${WEB_PORT:-}" \
      bash -c 'curl -fsSL "$1" | bash -s -- "${@:2}"' bash "$RAW_URL" "$@"
  fi
  printf '[%s] please run as root or install sudo first\n' "$APP_NAME" >&2
  exit 1
fi

need_command() {
  local command_name="$1"
  if ! command -v "$command_name" >/dev/null 2>&1; then
    printf '[%s] missing required command: %s\n' "$APP_NAME" "$command_name" >&2
    exit 1
  fi
}

need_command curl
need_command tar
need_command mktemp

release_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'amd64\n' ;;
    aarch64|arm64) printf 'arm64\n' ;;
    i386|i686) printf '386\n' ;;
    armv7l|armv7*) printf 'armv7\n' ;;
    *) return 1 ;;
  esac
}

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

if [[ "$USE_PREBUILT" == "true" ]]; then
  if asset_arch="$(release_arch)"; then
    asset="oci-lifecycle-platform-linux-${asset_arch}.tar.gz"
    asset_url="${RELEASE_DOWNLOAD_BASE%/}/${asset}"
    printf '[%s] trying prebuilt release %s\n' "$APP_NAME" "$asset_url"
    if curl -fL --retry 3 --connect-timeout 20 "$asset_url" -o "$tmp_dir/prebuilt.tgz"; then
      mkdir -p "$tmp_dir/prebuilt"
      tar -xzf "$tmp_dir/prebuilt.tgz" -C "$tmp_dir/prebuilt"
      prebuilt_binary="$(find "$tmp_dir/prebuilt" -maxdepth 3 -type f -path '*/bin/oci-lifecycle-platform' -print -quit)"
      if [[ -n "$prebuilt_binary" ]]; then
        prebuilt_dir="$(dirname "$(dirname "$prebuilt_binary")")"
        if [[ -x "$prebuilt_dir/bin/oci-lifecycle-platform" && -x "$prebuilt_dir/bin/panel-password" && -d "$prebuilt_dir/www" ]]; then
          if [[ $# -eq 0 ]]; then
            set -- install
          fi
          printf '[%s] running native Linux installer from prebuilt %s\n' "$APP_NAME" "$asset"
          exec bash "$prebuilt_dir/scripts/install.sh" --systemd "$@" --prebuilt "$prebuilt_dir"
        fi
      fi
      printf '[%s] prebuilt package layout is invalid; falling back to source install\n' "$APP_NAME" >&2
    else
      printf '[%s] prebuilt release is unavailable for this architecture; falling back to source install\n' "$APP_NAME" >&2
    fi
  else
    printf '[%s] unsupported architecture for prebuilt install: %s; falling back to source install\n' "$APP_NAME" "$(uname -m)" >&2
  fi
fi

printf '[%s] downloading %s\n' "$APP_NAME" "$ARCHIVE_URL"
curl -fL --retry 3 --connect-timeout 20 "$ARCHIVE_URL" -o "$tmp_dir/source.tgz"
tar -xzf "$tmp_dir/source.tgz" -C "$tmp_dir"

package_json="$(find "$tmp_dir" -maxdepth 3 -type f -name package.json -print -quit)"
project_dir=""
if [[ -n "$package_json" ]]; then
  project_dir="$(dirname "$package_json")"
fi

if [[ -z "$project_dir" || ! -f "$project_dir/scripts/install.sh" ]]; then
  printf '[%s] downloaded archive does not contain scripts/install.sh\n' "$APP_NAME" >&2
  exit 1
fi

if [[ $# -eq 0 ]]; then
  set -- install
fi

printf '[%s] running native Linux installer from %s\n' "$APP_NAME" "$project_dir"
exec bash "$project_dir/scripts/install.sh" --systemd "$@" --source "$project_dir"
