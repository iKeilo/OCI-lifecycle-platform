#!/usr/bin/env bash
set -Eeuo pipefail

APP_NAME="oci-lifecycle-platform"
REPO_OWNER="${OCI_LIFECYCLE_REPO_OWNER:-iKeilo}"
REPO_NAME="${OCI_LIFECYCLE_REPO_NAME:-OCI-lifecycle-platform}"
BRANCH="${OCI_LIFECYCLE_BRANCH:-main}"
RAW_URL="${PANEL_INSTALL_RAW_URL:-https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/${BRANCH}/panel_install.sh}"
ARCHIVE_URL="${OCI_LIFECYCLE_ARCHIVE_URL:-https://github.com/${REPO_OWNER}/${REPO_NAME}/archive/refs/heads/${BRANCH}.tar.gz}"

usage() {
  cat <<USAGE
${APP_NAME} remote one-click installer

Default Docker install:
  bash <(curl -L https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/${BRANCH}/panel_install.sh)

Common commands:
  bash <(curl -L https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/${BRANCH}/panel_install.sh) install
  bash <(curl -L https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/${BRANCH}/panel_install.sh) update
  bash <(curl -L https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/${BRANCH}/panel_install.sh) change-password
  bash <(curl -L https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/${BRANCH}/panel_install.sh) uninstall

Traditional systemd mode:
  bash <(curl -L https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/${BRANCH}/panel_install.sh) --systemd install

Environment:
  OCI_LIFECYCLE_BRANCH       GitHub branch, default main.
  OCI_LIFECYCLE_REPO_OWNER   GitHub owner, default iKeilo.
  OCI_LIFECYCLE_REPO_NAME    GitHub repo, default OCI-lifecycle-platform.
  OCI_LIFECYCLE_USE_PACKAGE  Docker mode defaults to true and pulls ghcr.io/ikeilo/oci-lifecycle-platform.
                             Set false to build from source on the server.
  OCI_LIFECYCLE_IMAGE_TAG    GHCR image tag for package mode, default latest.
  PANEL_PASSWORD             Optional non-interactive panel password.
  PANEL_PASSWORD_FILE        File used when a random panel password is generated.
  WEB_PORT                   Docker default 18080, systemd default 80.
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
      OCI_LIFECYCLE_USE_PACKAGE="${OCI_LIFECYCLE_USE_PACKAGE:-}" \
      OCI_LIFECYCLE_IMAGE="${OCI_LIFECYCLE_IMAGE:-}" \
      OCI_LIFECYCLE_IMAGE_TAG="${OCI_LIFECYCLE_IMAGE_TAG:-}" \
      OCI_LIFECYCLE_PACKAGE_IMAGE="${OCI_LIFECYCLE_PACKAGE_IMAGE:-}" \
      DEPLOY_MODE="${DEPLOY_MODE:-}" \
      DOCKER_APP_DIR="${DOCKER_APP_DIR:-}" \
      DOCKER_ENV_FILE="${DOCKER_ENV_FILE:-}" \
      ENV_DIR="${ENV_DIR:-}" \
      OCI_KEY_DIR="${OCI_KEY_DIR:-}" \
      GO_PROXY="${GO_PROXY:-}" \
      PANEL_PASSWORD="${PANEL_PASSWORD:-}" \
      PANEL_PASSWORD_FILE="${PANEL_PASSWORD_FILE:-}" \
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

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

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

printf '[%s] running installer from %s\n' "$APP_NAME" "$project_dir"
exec bash "$project_dir/scripts/install.sh" "$@" --source "$project_dir"
