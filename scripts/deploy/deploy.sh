#!/usr/bin/env bash
# ==============================================================================
#  Garda Inflasi — scripts/deploy/deploy.sh
#  ORCHESTRATOR: build (Mac) → save → transfer (SSH) → load & up (VM).
#
#  Usage:
#      bash scripts/deploy/deploy.sh [options]
#
#  Options:
#      --skip-build           Reuse the images already built locally
#      --skip-transfer        Reuse the tarball already on the VM
#      --only=api,web,caddy   Build/deploy only the listed services
#      --dry-run              Print every command; NEVER touches the VM
#      --allow-dirty          Allow building from a dirty working tree
#      --backup               pg_dump the VM database before deploying
#      --no-rollback          Do not auto-rollback when the remote deploy fails
#      -h, --help             Show this help
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
SKIP_BUILD=0
SKIP_TRANSFER=0
ONLY_SERVICES=""
DRY_RUN="${DRY_RUN:-0}"
ALLOW_DIRTY=0
DO_BACKUP=0
AUTO_ROLLBACK=1

usage() {
  sed -n '2,20p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-build)    SKIP_BUILD=1 ;;
    --skip-transfer) SKIP_TRANSFER=1 ;;
    --only=*)        ONLY_SERVICES="${1#*=}" ;;
    --only)          shift; ONLY_SERVICES="${1:-}" ;;
    --dry-run)       DRY_RUN=1 ;;
    --allow-dirty)   ALLOW_DIRTY=1 ;;
    --backup)        DO_BACKUP=1 ;;
    --no-rollback)   AUTO_ROLLBACK=0 ;;
    -h|--help)       usage; exit 0 ;;
    *) die "Opsi tidak dikenal: $1" "Jalankan 'bash scripts/deploy/deploy.sh --help'." ;;
  esac
  shift
done

export DRY_RUN

# ------------------------------------------------------------------------------
#  Entry point — everything runs under the deploy lock
# ------------------------------------------------------------------------------
main() {
  cd "$GARDA_ROOT"

  log_section "Deploy Garda Inflasi"
  print_kv "Mode"          "$([[ "$DRY_RUN" -eq 1 ]] && echo 'DRY-RUN (VM tidak disentuh)' || echo 'LIVE')"
  print_kv "Skip build"    "$([[ "$SKIP_BUILD" -eq 1 ]] && echo yes || echo no)"
  print_kv "Skip transfer" "$([[ "$SKIP_TRANSFER" -eq 1 ]] && echo yes || echo no)"
  print_kv "Only"          "${ONLY_SERVICES:-<all>}"
  print_kv "Backup DB"     "$([[ "$DO_BACKUP" -eq 1 ]] && echo yes || echo no)"
  print_kv "Auto rollback" "$([[ "$AUTO_ROLLBACK" -eq 1 ]] && echo yes || echo no)"

  # ── Step 1: resolve GIT_SHA ────────────────────────────────────────────
  GIT_SHA="$(git rev-parse --short HEAD)"
  export GIT_SHA
  log_info "GIT_SHA: ${GIT_SHA}"

  # ── Step 2: pre-flight (skipped entirely in dry-run) ───────────────────
  if [[ "$DRY_RUN" != "1" ]]; then
    vm_preflight
  else
    echo_cmd "vm_preflight  # cek SSH + docker + compose v2 di VM"
  fi

  # ── Step 3: record the previous deploy marker on the VM ────────────────
  log_section "Menyimpan marker deploy sebelumnya"
  vm_ssh "cd '${REMOTE_DIR}' && { [ -f .last-deploy ] && cp .last-deploy .previous-deploy || true; }"

  # ── Step 4: optional database backup ───────────────────────────────────
  if [[ "$DO_BACKUP" -eq 1 ]]; then
    log_section "Backup database di VM"
    backup_cmd="cd '${REMOTE_DIR}' && mkdir -p backups && docker exec garda-postgres pg_dump -U \"\${POSTGRES_USER:-garda}\" \"\${POSTGRES_DB:-garda_inflasi}\" | gzip > backups/\$(date +%F-%H%M).sql.gz && ls -lh backups | tail -3"
    if [[ "$DRY_RUN" == "1" ]]; then
      echo_cmd "ssh ${VM_USER}@${VM_HOST} \"${backup_cmd}\""
    else
      if vm_ssh "$backup_cmd"; then
        log_success "Backup database selesai."
      else
        die "Backup database gagal." \
            "Pastikan container 'garda-postgres' berjalan di VM: ssh ${VM_USER}@${VM_HOST} 'docker ps | grep postgres'"
      fi
    fi
  fi

  # ── Step 5: build locally ──────────────────────────────────────────────
  if [[ "$SKIP_BUILD" -eq 1 ]]; then
    log_section "Build dilewati (--skip-build)"
    if [[ -f "${GARDA_ROOT}/.last-build-sha" ]]; then
      GIT_SHA="$(tr -d '[:space:]' < "${GARDA_ROOT}/.last-build-sha")"
      export GIT_SHA
      log_info "Memakai GIT_SHA dari build terakhir: ${GIT_SHA}"
    fi
  else
    log_section "Build image lokal"
    build_args=()
    [[ -n "$ONLY_SERVICES" ]] && build_args+=("--only=${ONLY_SERVICES}")
    [[ "$ALLOW_DIRTY" -eq 1 ]] && build_args+=("--allow-dirty")
    [[ "$DRY_RUN" -eq 1 ]] && build_args+=("--dry-run")

    if ! bash "${SCRIPT_DIR}/build-local.sh" "${build_args[@]}"; then
      die "Build lokal gagal." "Perbaiki error di atas lalu ulangi: make deploy"
    fi

    if [[ "$DRY_RUN" != "1" && -f "${GARDA_ROOT}/.last-build-sha" ]]; then
      GIT_SHA="$(tr -d '[:space:]' < "${GARDA_ROOT}/.last-build-sha")"
      export GIT_SHA
    fi
  fi

  # ── Step 6: save images to a tarball ───────────────────────────────────
  if [[ "$SKIP_TRANSFER" -eq 1 ]]; then
    log_section "Save dilewati (--skip-transfer)"
  else
    log_section "Save image ke tarball"
    save_args=("--sha=${GIT_SHA}")
    [[ "$DRY_RUN" -eq 1 ]] && save_args+=("--dry-run")

    if ! bash "${SCRIPT_DIR}/save-images.sh" "${save_args[@]}"; then
      die "Save image gagal." "Cek free space lokal (df -h .) lalu ulangi: make deploy/save"
    fi
  fi

  # ── Step 7: transfer to the VM ─────────────────────────────────────────
  if [[ "$SKIP_TRANSFER" -eq 1 ]]; then
    log_section "Transfer dilewati (--skip-transfer)"
  else
    log_section "Transfer ke VM"
    transfer_args=("--sha=${GIT_SHA}")
    [[ "$DRY_RUN" -eq 1 ]] && transfer_args+=("--dry-run")

    if ! bash "${SCRIPT_DIR}/transfer.sh" "${transfer_args[@]}"; then
      die "Transfer ke VM gagal." \
          "Cek koneksi SSH dan free space VM. rsync aman diulang: make deploy/transfer"
    fi
  fi

  # ── Step 8: run the remote deploy ──────────────────────────────────────
  log_section "Menjalankan deploy di VM"
  remote_script="${SCRIPT_DIR}/remote-deploy.sh"

  if [[ "$DRY_RUN" == "1" ]]; then
    echo_cmd "ssh ${VM_USER}@${VM_HOST} 'bash -s -- ${GIT_SHA}' < ${remote_script}"
    log_section "DRY-RUN selesai — tidak ada perubahan di VM"
    return 0
  fi

  remote_rc=0
  if ! vm_ssh_script "$remote_script" "$GIT_SHA"; then
    remote_rc=$?
    log_error "Deploy di VM gagal (exit ${remote_rc})."
  fi

  # ── Step 9: auto-rollback on failure ───────────────────────────────────
  if [[ "$remote_rc" -ne 0 ]]; then
    if [[ "$AUTO_ROLLBACK" -eq 1 ]]; then
      log_warn "Menjalankan auto-rollback ..."
      if bash "${SCRIPT_DIR}/rollback.sh" --auto; then
        log_success "Auto-rollback berhasil — layanan kembali ke versi sebelumnya."
      else
        log_error "Auto-rollback GAGAL. Intervensi manual diperlukan."
      fi
    else
      log_warn "Auto-rollback dinonaktifkan (--no-rollback)."
    fi
    die "Deploy gagal pada GIT_SHA=${GIT_SHA}." \
        "Lihat log di atas. Untuk rollback manual: make rollback"
  fi

  # ── Step 10: prune old images ──────────────────────────────────────────
  log_section "Membersihkan image lama di VM"
  if vm_ssh "cd '${REMOTE_DIR}' && KEEP_LAST_N_IMAGES='${KEEP_LAST_N_IMAGES}' bash scripts/rollback/keep-last-n-images.sh"; then
    log_success "Prune selesai."
  else
    log_warn "Prune image lama gagal — tidak fatal, deploy tetap sukses."
  fi

  # ── Step 11: summary ───────────────────────────────────────────────────
  log_section "Deploy selesai"
  print_kv "GIT_SHA"   "$GIT_SHA"
  print_kv "Timestamp" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  print_kv "VM"        "${VM_USER}@${VM_HOST}"
  if [[ -n "${PUBLIC_HOST:-}" ]]; then
    print_kv "URL"     "https://${PUBLIC_HOST}"
    print_kv "Health"  "https://${PUBLIC_HOST}/healthz"
  else
    print_kv "URL"     "http://${VM_HOST}"
    print_kv "Health"  "http://${VM_HOST}/healthz"
  fi

  log_info "Status container:"
  vm_ssh "cd '${REMOTE_DIR}' && docker compose -f compose/compose.yaml -f compose/compose.prod.yaml ps" || true

  log_success "Deploy ${GIT_SHA} selesai dengan sukses."
}

# ------------------------------------------------------------------------------
#  Bootstrap: load config, then run main under the lock
# ------------------------------------------------------------------------------
load_env_deploy
validate_ssh_key

require_cmd docker "Install Docker Desktop: brew install --cask docker"
require_cmd git    "Install git: brew install git"
require_cmd ssh    "ssh tersedia secara default di macOS/Linux."
require_cmd rsync  "Install rsync: brew install rsync"

LOCK_PATH="${GARDA_ROOT}/.deploy.lock"
with_lock "$LOCK_PATH" main