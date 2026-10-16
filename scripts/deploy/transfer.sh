#!/usr/bin/env bash
# ==============================================================================
#  Garda Inflasi — scripts/deploy/transfer.sh
#  Transfers the image tarball + compose files + DB assets to the VM over SSH.
#
#  NEVER transfers .env — the VM owns its own production .env.
#
#  Usage:
#      bash scripts/deploy/transfer.sh [options]
#
#  Options:
#      --sha=<sha>     Override the git SHA (default: .last-build-sha)
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

# ------------------------------------------------------------------------------
#  Defaults + argument parsing
# ------------------------------------------------------------------------------
SHA_OVERRIDE=""
DRY_RUN="${DRY_RUN:-0}"

usage() {
  sed -n '2,16p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --sha=*)   SHA_OVERRIDE="${1#*=}" ;;
    --sha)     shift; SHA_OVERRIDE="${1:-}" ;;
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
#  Resolve GIT_SHA + tarball
# ------------------------------------------------------------------------------
if [[ -n "$SHA_OVERRIDE" ]]; then
  GIT_SHA="$SHA_OVERRIDE"
elif [[ -f "${GARDA_ROOT}/.last-build-sha" ]]; then
  GIT_SHA="$(tr -d '[:space:]' < "${GARDA_ROOT}/.last-build-sha")"
else
  GIT_SHA="$(git rev-parse --short HEAD)"
fi
export GIT_SHA

TARBALL="${GARDA_ROOT}/dist/garda-images-${GIT_SHA}.tar.gz"

if [[ "$DRY_RUN" != "1" ]]; then
  require_file "$TARBALL" "Jalankan dulu: make deploy/save (atau bash scripts/deploy/save-images.sh)."
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

  # Ensure the remote dir exists before df-ing it.
  vm_ssh "mkdir -p '${REMOTE_DIR}'" >/dev/null

  local avail_gb
  avail_gb="$(vm_ssh "df -BG '${REMOTE_DIR}' | tail -1 | awk '{print \$4}' | tr -d 'G'" 2>/dev/null || echo '')"

  if [[ -z "$avail_gb" || ! "$avail_gb" =~ ^[0-9]+$ ]]; then
    log_warn "Tidak bisa membaca free space VM — pre-flight disk dilewati."
    return 0
  fi

  local avail_bytes=$(( avail_gb * 1024 * 1024 * 1024 ))
  log_info "Free space VM: ${avail_gb} GB — butuh ≥ $(human_size "$REQUIRED_BYTES") (2× tarball)"

  if [[ "$avail_bytes" -lt "$REQUIRED_BYTES" ]]; then
    die "Disk VM tidak cukup: ${avail_gb} GB tersisa, butuh ≥ $(human_size "$REQUIRED_BYTES")." \
        "Bersihkan di VM: ssh ${VM_USER}@${VM_HOST} 'docker system prune -af && rm -f ${REMOTE_DIR}/dist/*.tar.gz'"
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
)

log_section "Menyiapkan direktori di VM"
for d in "${REMOTE_DIRS[@]}"; do
  log_debug "mkdir -p $d"
done
vm_ssh "mkdir -p $(printf "'%s' " "${REMOTE_DIRS[@]}")"

# ------------------------------------------------------------------------------
#  Guard: the VM must already have its own .env (never transferred)
# ------------------------------------------------------------------------------
check_remote_env() {
  if ! vm_ssh "test -f '${REMOTE_DIR}/.env'"; then
    die "File '${REMOTE_DIR}/.env' tidak ada di VM." \
        "Buat dulu di VM (lihat docs/DEPLOYMENT.md § First-time VM setup), lalu ulangi transfer. Script ini SENGAJA tidak mengirim .env."
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
for f in compose.infra.yaml compose.yaml compose.prod.yaml; do
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

# 5. Deploy scripts needed on the VM (remote-deploy + keep-last-n)
log_info "[5/5] Script deploy untuk VM ..."
vm_ssh "mkdir -p '${REMOTE_DIR}/scripts/deploy' '${REMOTE_DIR}/scripts/rollback' '${REMOTE_DIR}/scripts/lib'"
vm_rsync "${GARDA_ROOT}/scripts/deploy/remote-deploy.sh" "${REMOTE_DIR}/scripts/deploy/"
vm_rsync "${GARDA_ROOT}/scripts/rollback/keep-last-n-images.sh" "${REMOTE_DIR}/scripts/rollback/"
vm_rsync "${GARDA_ROOT}/scripts/lib/common.sh" "${REMOTE_DIR}/scripts/lib/"

# ------------------------------------------------------------------------------
#  Verify the tarball landed intact (size comparison)
# ------------------------------------------------------------------------------
verify_transfer() {
  local remote_bytes
  remote_bytes="$(vm_ssh "stat -c%s '${REMOTE_DIR}/dist/garda-images-${GIT_SHA}.tar.gz' 2>/dev/null || stat -f%z '${REMOTE_DIR}/dist/garda-images-${GIT_SHA}.tar.gz'" 2>/dev/null || echo 0)"

  if [[ "$remote_bytes" != "$TARBALL_BYTES" ]]; then
    die "Ukuran tarball di VM tidak cocok (lokal=${TARBALL_BYTES}B, remote=${remote_bytes}B)." \
        "Transfer kemungkinan terputus. Ulangi: make deploy/transfer (rsync akan melanjutkan dari posisi terakhir)."
  fi
  log_success "Verifikasi tarball OK (${remote_bytes} bytes)."
}

if [[ "$DRY_RUN" == "1" ]]; then
  echo_cmd "ssh ${VM_USER}@${VM_HOST} \"stat -c%s ${REMOTE_DIR}/dist/garda-images-${GIT_SHA}.tar.gz\""
else
  verify_transfer
fi

# ------------------------------------------------------------------------------
#  Summary
# ------------------------------------------------------------------------------
log_section "Transfer selesai"
print_kv "VM"        "${VM_USER}@${VM_HOST}:${SSH_PORT}"
print_kv "Remote dir" "$REMOTE_DIR"
print_kv "GIT_SHA"   "$GIT_SHA"
print_kv "Tarball"   "$(human_size "$TARBALL_BYTES")"

log_info "Lanjut: bash scripts/deploy/deploy.sh --skip-build --skip-transfer"