#!/usr/bin/env bash
# ==============================================================================
#  Garda Inflasi — scripts/deploy/sync-source.sh
#  Syncs compose/ops files + a clean temporary build context (`.build-src/`)
#  from the Mac to the Ubuntu VM over SSH.
#
#  Zero-Git & Clean Server:
#    * Excludes node_modules, .next, .git, local .env files, tests/reports.
#    * `.build-src/` is deleted immediately after `remote-deploy.sh` builds.
#
#  Usage:
#      bash scripts/deploy/sync-source.sh [options]
#
#  Options:
#      --only=api,web,ml,worker   Sync only source needed for those services
#      --dry-run                  Print commands without touching the VM
#      -h, --help                 Show this help
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"
# shellcheck source=../lib/ssh-wrapper.sh
source "${SCRIPT_DIR}/../lib/ssh-wrapper.sh"

install_traps

ONLY_RAW=""
DRY_RUN="${DRY_RUN:-0}"

usage() {
  sed -n '2,17p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --only=*)  ONLY_RAW="${1#*=}" ;;
    --only)    shift; ONLY_RAW="${1:-}" ;;
    --dry-run) DRY_RUN=1 ;;
    -h|--help) usage; exit 0 ;;
    *) die "Opsi tidak dikenal: $1" "Jalankan 'bash scripts/deploy/sync-source.sh --help'." ;;
  esac
  shift
done

export DRY_RUN

load_env_deploy
validate_ssh_key

require_cmd rsync "Install rsync: brew install rsync"
require_cmd ssh   "ssh tersedia secara default di macOS/Linux."

cd "$GARDA_ROOT"

# Resolve selected services
TARGET_SERVICES=()
if [[ -n "$ONLY_RAW" ]]; then
  IFS=',' read -r -a TARGET_SERVICES <<< "$ONLY_RAW"
else
  if [[ "${GATEWAY_MODE:-external}" == "builtin" ]]; then
    TARGET_SERVICES=(api worker ml web caddy)
  else
    TARGET_SERVICES=(api worker ml web)
  fi
fi

NEED_API=0
NEED_WEB=0
NEED_ML=0
NEED_CADDY=0
for svc in "${TARGET_SERVICES[@]}"; do
  case "$svc" in
    api|worker) NEED_API=1 ;;
    web)        NEED_WEB=1 ;;
    ml)         NEED_ML=1 ;;
    caddy)      NEED_CADDY=1 ;;
    *) die "Service tidak dikenal: '$svc'" "Pilih dari: api, worker, web, ml, caddy." ;;
  esac
done

vm_preflight

# Ensure remote layout exists
REMOTE_DIRS=(
  "${REMOTE_DIR}"
  "${REMOTE_DIR}/compose"
  "${REMOTE_DIR}/docker/postgres"
  "${REMOTE_DIR}/db/migrations"
  "${REMOTE_DIR}/backups"
  "${REMOTE_DIR}/data/uploads"
  "${REMOTE_DIR}/logs"
  "${REMOTE_DIR}/scripts/deploy"
  "${REMOTE_DIR}/scripts/rollback"
  "${REMOTE_DIR}/scripts/lib"
  "${REMOTE_DIR}/scripts/backup"
  "${REMOTE_DIR}/.build-src"
)

log_section "Menyiapkan direktori sinkronisasi di VM"
vm_ssh "mkdir -p $(printf "'%s' " "${REMOTE_DIRS[@]}")"

# Guard: remote .env must exist
if [[ "$DRY_RUN" == "1" ]]; then
  echo_cmd "vm_ssh \"test -f ${REMOTE_DIR}/.env\""
else
  if ! vm_ssh "test -f '${REMOTE_DIR}/.env'"; then
    die "File '${REMOTE_DIR}/.env' tidak ada di VM." \
        "Jalankan dulu: bash scripts/deploy/init-server.sh (atau deploy --init)."
  fi
fi

# ------------------------------------------------------------------------------
#  1. Sync runtime/ops files (compose, init.sql, migrations, scripts)
# ------------------------------------------------------------------------------
log_section "Sync file operasional ke ${REMOTE_DIR}"

log_info "[1/3] Compose files ..."
for f in compose.infra.yaml compose.yaml compose.prod.yaml compose.gateway.yaml; do
  require_file "${GARDA_ROOT}/${f}" "File compose '$f' wajib ada di root repo."
  vm_rsync "${GARDA_ROOT}/${f}" "${REMOTE_DIR}/compose/"
done

log_info "[2/3] Postgres init & migrasi ..."
if [[ -f "${GARDA_ROOT}/docker/postgres/init.sql" ]]; then
  vm_rsync "${GARDA_ROOT}/docker/postgres/init.sql" "${REMOTE_DIR}/docker/postgres/"
fi
if [[ -d "${GARDA_ROOT}/db/migrations" ]]; then
  vm_rsync "${GARDA_ROOT}/db/migrations/" "${REMOTE_DIR}/db/migrations/"
fi

log_info "[3/3] Remote deploy scripts ..."
vm_rsync "${GARDA_ROOT}/scripts/backup/" "${REMOTE_DIR}/scripts/backup/"
vm_rsync "${GARDA_ROOT}/scripts/deploy/remote-deploy.sh" "${REMOTE_DIR}/scripts/deploy/"
vm_rsync "${GARDA_ROOT}/scripts/rollback/keep-last-n-images.sh" "${REMOTE_DIR}/scripts/rollback/"
vm_rsync "${GARDA_ROOT}/scripts/lib/common.sh" "${REMOTE_DIR}/scripts/lib/"

# ------------------------------------------------------------------------------
#  2. Sync clean temporary build context → ${REMOTE_DIR}/.build-src
# ------------------------------------------------------------------------------
log_section "Sync source code ke ${REMOTE_DIR}/.build-src (${TARGET_SERVICES[*]})"

RSYNC_BUILD_ARGS=(
  --delete
  --include=env.example
  --include=env.local.example
  --exclude=.git/
  --exclude=node_modules/
  --exclude=.pnpm-store/
  --exclude=.next/
  --exclude=test-results/
  --exclude=playwright-report/
  --exclude=/bin/
  --exclude=/tmp/
  --exclude=__pycache__/
  --exclude=*.pyc
  --exclude=.env
  --exclude=.env.*
  --exclude=.DS_Store
  --exclude=*.test
  --exclude=*.out
  --exclude=*.log
  --exclude=.turbo/
  --exclude=coverage/
)

BUILD_SRC_REMOTE="${REMOTE_DIR}/.build-src"

# Common files at build root
vm_ssh "mkdir -p '${BUILD_SRC_REMOTE}/docker' '${BUILD_SRC_REMOTE}/apps' '${BUILD_SRC_REMOTE}/db'"
vm_rsync "${GARDA_ROOT}/.dockerignore" "${BUILD_SRC_REMOTE}/"

if [[ "$NEED_API" -eq 1 ]]; then
  log_info "Sync context: api/worker (Go + migrations + seeds) ..."
  vm_rsync "${GARDA_ROOT}/docker/api.Dockerfile" "${BUILD_SRC_REMOTE}/docker/"
  vm_rsync "${GARDA_ROOT}/go.work" "${BUILD_SRC_REMOTE}/"
  vm_rsync "${GARDA_ROOT}/go.work.sum" "${BUILD_SRC_REMOTE}/"
  vm_rsync "${GARDA_ROOT}/apps/api/" "${BUILD_SRC_REMOTE}/apps/api/" "${RSYNC_BUILD_ARGS[@]}"
  vm_rsync "${GARDA_ROOT}/db/" "${BUILD_SRC_REMOTE}/db/" "${RSYNC_BUILD_ARGS[@]}"
fi

if [[ "$NEED_WEB" -eq 1 ]]; then
  log_info "Sync context: web (Next.js + pnpm workspace) ..."
  vm_rsync "${GARDA_ROOT}/docker/web.Dockerfile" "${BUILD_SRC_REMOTE}/docker/"
  vm_rsync "${GARDA_ROOT}/pnpm-workspace.yaml" "${BUILD_SRC_REMOTE}/"
  vm_rsync "${GARDA_ROOT}/pnpm-lock.yaml" "${BUILD_SRC_REMOTE}/"
  vm_rsync "${GARDA_ROOT}/turbo.json" "${BUILD_SRC_REMOTE}/"
  vm_ssh "mkdir -p '${BUILD_SRC_REMOTE}/packages' '${BUILD_SRC_REMOTE}/apps/web'"
  vm_rsync "${GARDA_ROOT}/packages/" "${BUILD_SRC_REMOTE}/packages/" "${RSYNC_BUILD_ARGS[@]}"
  vm_rsync "${GARDA_ROOT}/apps/web/" "${BUILD_SRC_REMOTE}/apps/web/" "${RSYNC_BUILD_ARGS[@]}"
fi

if [[ "$NEED_ML" -eq 1 ]]; then
  log_info "Sync context: ml (Python) ..."
  vm_rsync "${GARDA_ROOT}/docker/ml.Dockerfile" "${BUILD_SRC_REMOTE}/docker/"
  vm_ssh "mkdir -p '${BUILD_SRC_REMOTE}/apps/ml'"
  vm_rsync "${GARDA_ROOT}/apps/ml/" "${BUILD_SRC_REMOTE}/apps/ml/" "${RSYNC_BUILD_ARGS[@]}"
fi

if [[ "$NEED_CADDY" -eq 1 ]]; then
  log_info "Sync context: caddy ..."
  vm_rsync "${GARDA_ROOT}/docker/caddy.Dockerfile" "${BUILD_SRC_REMOTE}/docker/"
  vm_rsync "${GARDA_ROOT}/docker/Caddyfile" "${BUILD_SRC_REMOTE}/docker/"
fi

log_success "Sync source ke ${BUILD_SRC_REMOTE} selesai."
