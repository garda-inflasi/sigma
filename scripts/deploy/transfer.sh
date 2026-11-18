#!/usr/bin/env bash
# ==============================================================================
#  Garda Inflasi — scripts/deploy/transfer.sh
#  Transfers the image tarball + compose files + DB assets to the VM over SSH.
#  Zero-Git workflow. NEVER transfers local .env.
#
#  Usage:
#      bash scripts/deploy/transfer.sh [options]
#
#  Options:
#      --tag=<tag>     Override release tag (default: .last-build-tag)
#      --dry-run       Print commands without touching the VM
#      -h, --help      Show this help
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"
# shellcheck source=../lib/ssh-wrapper.sh
source "${SCRIPT_DIR}/../lib/ssh-wrapper.sh"

install_traps

TAG_OVERRIDE=""
DRY_RUN="${DRY_RUN:-0}"

usage() {
  sed -n '2,16p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --tag=*)   TAG_OVERRIDE="${1#*=}" ;;
    --tag)     shift; TAG_OVERRIDE="${1:-}" ;;
    --sha=*)   TAG_OVERRIDE="${1#*=}" ;;
    --sha)     shift; TAG_OVERRIDE="${1:-}" ;;
    --dry-run) DRY_RUN=1 ;;
    -h|--help) usage; exit 0 ;;
    *) die "Opsi tidak dikenal: $1" "Jalankan 'bash scripts/deploy/transfer.sh --help'." ;;
  esac
  shift
done

export DRY_RUN

load_env_deploy
validate_ssh_key

require_cmd rsync "Install rsync: brew install rsync"
require_cmd ssh   "ssh tersedia secara default di macOS/Linux."

cd "$GARDA_ROOT"

# ------------------------------------------------------------------------------
#  Resolve RELEASE_TAG + tarball
# ------------------------------------------------------------------------------
if [[ -n "$TAG_OVERRIDE" ]]; then
  RELEASE_TAG="$TAG_OVERRIDE"
elif [[ -f "${GARDA_ROOT}/.last-build-tag" ]]; then
  RELEASE_TAG="$(tr -d '[:space:]' < "${GARDA_ROOT}/.last-build-tag")"
elif [[ -f "${GARDA_ROOT}/.last-build-sha" ]]; then
  RELEASE_TAG="$(tr -d '[:space:]' < "${GARDA_ROOT}/.last-build-sha")"
else
  RELEASE_TAG=""
fi

if [[ -z "$RELEASE_TAG" ]]; then
  die "Tidak bisa menentukan RELEASE_TAG untuk transfer." \
      "Jalankan save-images dulu (make deploy/save) atau lewatkan --tag=<tag>."
fi
export RELEASE_TAG

TARBALL="${GARDA_ROOT}/dist/garda-images-${RELEASE_TAG}.tar.gz"

if [[ "$DRY_RUN" != "1" ]]; then
  require_file "$TARBALL" "Jalankan dulu: make deploy/save (atau bash scripts/deploy/save-images.sh --tag=${RELEASE_TAG})."
fi

# ------------------------------------------------------------------------------
#  Pre-flight: SSH + Docker on the VM
# ------------------------------------------------------------------------------
vm_preflight

# ------------------------------------------------------------------------------
#  Pre-flight: VM free space must be >= 2x the tarball size
# ------------------------------------------------------------------------------
TARBALL_BYTES=0
if [[ "$DRY_RUN" != "1" ]]; then
  TARBALL_BYTES="$(file_size_bytes "$TARBALL")"
fi
REQUIRED_BYTES=$(( TARBALL_BYTES * 2 ))

check_vm_disk_space() {
  log_info "Pre-flight: cek free space di VM (${REMOTE_DIR}) ..."
  vm_ssh "mkdir -p '${REMOTE_DIR}'" >/dev/null

  local avail_gb
  avail_gb="$(vm_ssh "df -BG '${REMOTE_DIR}' | tail -1 | awk '{print \$4}' | tr -d 'G'" 2>/dev/null | tr -d '[:space:]' || echo '')"

  if [[ -z "$avail_gb" || ! "$avail_gb" =~ ^[0-9]+$ ]]; then
    log_warn "Tidak bisa membaca free space VM — pre-flight disk dilewati."
    return 0
  fi

  local avail_bytes=$(( avail_gb * 1024 * 1024 * 1024 ))
  log_info "Free space VM: ${avail_gb} GB — butuh ≥ $(human_size "$REQUIRED_BYTES") (2× tarball)"

  if [[ "$avail_bytes" -lt "$REQUIRED_BYTES" ]]; then
    die "Disk VM tidak cukup: ${avail_gb} GB tersisa, butuh ≥ $(human_size "$REQUIRED_BYTES")." \
        "Bersihkan di VM: docker system prune -af && rm -f ${REMOTE_DIR}/dist/*.tar.gz"
  fi

  log_success "Free space VM OK (${avail_gb} GB)."
}

if [[ "$DRY_RUN" == "1" ]]; then
  echo_cmd "ssh ${VM_USER}@${VM_HOST} \"df -BG ${REMOTE_DIR} | tail -1\"  # butuh >= $(human_size "$REQUIRED_BYTES")"
else
  check_vm_disk_space
fi

# ------------------------------------------------------------------------------
#  Create the remote directory layout
# ------------------------------------------------------------------------------
REMOTE_DIRS=(
  "${REMOTE_DIR}"
  "${REMOTE_DIR}/dist"
  "${REMOTE_DIR}/compose"
  "${REMOTE_DIR}/docker/postgres"
  "${REMOTE_DIR}/db/migrations"
  "${REMOTE_DIR}/backups"
  "${REMOTE_DIR}/data/uploads"
  "${REMOTE_DIR}/logs"
)

log_section "Menyiapkan direktori di VM"
vm_ssh "mkdir -p $(printf "'%s' " "${REMOTE_DIRS[@]}")"

# ------------------------------------------------------------------------------
#  Guard: the VM must already have its own .env (never transferred)
# ------------------------------------------------------------------------------
check_remote_env() {
  if ! vm_ssh "test -f '${REMOTE_DIR}/.env'"; then
    die "File '${REMOTE_DIR}/.env' tidak ada di VM." \
        "Jalankan dulu: bash scripts/deploy/init-server.sh (atau make deploy ARGS=--init)."
  fi
  log_success "Remote .env ditemukan (tidak ditransfer, sesuai kebijakan)."
}

if [[ "$DRY_RUN" == "1" ]]; then
  echo_cmd "ssh ${VM_USER}@${VM_HOST} \"test -f ${REMOTE_DIR}/.env\"  # wajib ada"
else
  check_remote_env
fi

# ------------------------------------------------------------------------------
#  Transfer artifacts
# ------------------------------------------------------------------------------
log_section "Transfer artefak ke VM"

# 1. Image tarball
log_info "[1/5] Image tarball ($(human_size "$TARBALL_BYTES")) ..."
vm_rsync "$TARBALL" "${REMOTE_DIR}/dist/"

# 2. Compose files
log_info "[2/5] Compose files ..."
for f in compose.infra.yaml compose.yaml compose.prod.yaml compose.gateway.yaml; do
  require_file "${GARDA_ROOT}/${f}" "File compose '$f' wajib ada di root repo."
  vm_rsync "${GARDA_ROOT}/${f}" "${REMOTE_DIR}/compose/"
done

# 3. Postgres init SQL
log_info "[3/5] docker/postgres/init.sql ..."
if [[ -f "${GARDA_ROOT}/docker/postgres/init.sql" ]]; then
  vm_rsync "${GARDA_ROOT}/docker/postgres/init.sql" "${REMOTE_DIR}/docker/postgres/"
else
  log_warn "docker/postgres/init.sql tidak ada — dilewati."
fi

# 4. DB migrations
log_info "[4/5] db/migrations/ ..."
if [[ -d "${GARDA_ROOT}/db/migrations" ]]; then
  vm_rsync "${GARDA_ROOT}/db/migrations/" "${REMOTE_DIR}/db/migrations/"
else
  log_warn "db/migrations/ tidak ada — dilewati."
fi

# 5. Deploy scripts needed on the VM
log_info "[5/5] Script deploy untuk VM ..."
vm_ssh "mkdir -p '${REMOTE_DIR}/scripts/deploy' '${REMOTE_DIR}/scripts/rollback' '${REMOTE_DIR}/scripts/lib' '${REMOTE_DIR}/scripts/backup'"
vm_rsync "${GARDA_ROOT}/scripts/backup/" "${REMOTE_DIR}/scripts/backup/"
vm_rsync "${GARDA_ROOT}/scripts/deploy/remote-deploy.sh" "${REMOTE_DIR}/scripts/deploy/"
vm_rsync "${GARDA_ROOT}/scripts/rollback/keep-last-n-images.sh" "${REMOTE_DIR}/scripts/rollback/"
vm_rsync "${GARDA_ROOT}/scripts/lib/common.sh" "${REMOTE_DIR}/scripts/lib/"

# ------------------------------------------------------------------------------
#  Verify the tarball landed intact
# ------------------------------------------------------------------------------
verify_transfer() {
  local remote_bytes
  remote_bytes="$(vm_ssh "stat -c%s '${REMOTE_DIR}/dist/garda-images-${RELEASE_TAG}.tar.gz' 2>/dev/null || stat -f%z '${REMOTE_DIR}/dist/garda-images-${RELEASE_TAG}.tar.gz'" 2>/dev/null | tr -d '[:space:]' || echo 0)"

  if [[ "$remote_bytes" != "$TARBALL_BYTES" ]]; then
    die "Ukuran tarball di VM tidak cocok (lokal=${TARBALL_BYTES}B, remote=${remote_bytes}B)." \
        "Transfer kemungkinan terputus. Ulangi transfer."
  fi
  log_success "Verifikasi tarball OK (${remote_bytes} bytes)."
}

if [[ "$DRY_RUN" == "1" ]]; then
  echo_cmd "ssh ${VM_USER}@${VM_HOST} \"stat -c%s ${REMOTE_DIR}/dist/garda-images-${RELEASE_TAG}.tar.gz\""
else
  verify_transfer
fi

# ------------------------------------------------------------------------------
#  Summary
# ------------------------------------------------------------------------------
log_section "Transfer selesai"
print_kv "VM"          "${VM_USER}@${VM_HOST}:${SSH_PORT}"
print_kv "Remote dir"  "$REMOTE_DIR"
print_kv "Release tag" "$RELEASE_TAG"
print_kv "Tarball"     "$(human_size "$TARBALL_BYTES")"
