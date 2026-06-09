#!/usr/bin/env bash
set -Eeuo pipefail

fail=0

require_contains() {
  local file="$1"
  local pattern="$2"
  if ! grep -Eq "$pattern" "$file"; then
    printf 'missing expected pattern in %s: %s\n' "$file" "$pattern" >&2
    fail=1
  fi
}

reject_contains() {
  local file="$1"
  local pattern="$2"
  if grep -Eq "$pattern" "$file"; then
    printf 'unexpected pattern in %s: %s\n' "$file" "$pattern" >&2
    fail=1
  fi
}

require_contains Dockerfile '^ENV OCI_EXECUTION_MODE=oci$'
require_contains docker-compose.yml 'OCI_EXECUTION_MODE: \$\{OCI_EXECUTION_MODE:-oci\}'
require_contains docker/.env.example '^OCI_EXECUTION_MODE=oci$'
require_contains .env.example '^OCI_EXECUTION_MODE=oci$'
require_contains scripts/install.sh 'env_set OCI_EXECUTION_MODE "oci"'
require_contains scripts/install.sh 'docker_env_set OCI_EXECUTION_MODE "oci"'
reject_contains scripts/install.sh 'env_set OCI_EXECUTION_MODE "local"'
reject_contains scripts/install.sh 'docker_env_set OCI_EXECUTION_MODE "local"'

exit "$fail"
