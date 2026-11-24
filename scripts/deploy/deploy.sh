#!/usr/bin/env bash
# ==============================================================================
#  Garda Inflasi — scripts/deploy/deploy.sh
#  ORCHESTRATOR (Zero-Git, Server-Build & Clean Server):
#    sync clean source (Mac → VM .build-src) → build on VM → delete .build-src →
#    migrate/seed (if api) → compose up selected services → sync Caddy SSL →
#    verify https://PUBLIC_HOST/healthz → prune old tags.
#
#  Usage:
#      bash scripts/deploy/deploy.sh [options]
#
#  Options:
#      --init                   Bootstrap VM first (docker group, folders, .env)
#      --only=api,web,ml,worker Sync, build, and update only listed services
#      --tag=<tag>              Explicit release tag (default: vYYYYMMDD-HHMMSS)
#      --skip-sync              Skip source sync to VM
#      --skip-build             Skip remote docker build (reuse tag on VM)
#      --no-cache               Disable Docker build cache on VM
#      --keep-src               Keep .build-src on VM for debugging (default: delete)
#      --sync-caddy             Force sync Caddyfile on external gateway
#      --skip-caddy             Skip Caddyfile sync even on full deploy
#      --skip-migrate           Skip DB migrate/seed even when api/worker updated
#      --with-migrate           Force DB migrate/seed even on partial deploy
#      --backup                 pg_dump the VM database before deploying
#      --no-rollback            Do not auto-rollback when remote deploy fails
#      --dry-run                Print every command; NEVER touches the VM
#      -h, --help               Show this help
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"
# shellcheck source=../lib/ssh-wrapper.sh
source "${SCRIPT_DIR}/../lib/ssh-wrapper.sh"

install_traps

DO_INIT=0
SKIP_SYNC=0
SKIP_BUILD=0
NO_CACHE=0
KEEP_SRC=0
ONLY_SERVICES=""
TAG_OVERRIDE=""
SYNC_CADDY=-1   # -1 = auto (yes on full deploy, no on --only unless --sync-caddy)
SKIP_MIGRATE=0
WITH_MIGRATE=0
DRY_RUN="${DRY_RUN:-0}"
DO_BACKUP=0
AUTO_ROLLBACK=1

usage() {
  sed -n '2,25p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --init)          DO_INIT=1 ;;
    --skip-sync)     SKIP_SYNC=1 ;;
    --skip-build)    SKIP_BUILD=1 ;;
    --no-cache)      NO_CACHE=1 ;;
    --keep-src)      KEEP_SRC=1 ;;
    --only=*)        ONLY_SERVICES="${1#*=}" ;;
    --only)          shift; ONLY_SERVICES="${1:-}" ;;
    --tag=*)         TAG_OVERRIDE="${1#*=}" ;;
    --tag)           shift; TAG_OVERRIDE="${1:-}" ;;
    --sha=*)         TAG_OVERRIDE="${1#*=}" ;;
    --sha)           shift; TAG_OVERRIDE="${1:-}" ;;
    --sync-caddy)    SYNC_CADDY=1 ;;
    --skip-caddy)    SYNC_CADDY=0 ;;
    --skip-migrate)  SKIP_MIGRATE=1 ;;
    --with-migrate)  WITH_MIGRATE=1 ;;
    --allow-dirty)   ;; # no-op (zero-git)
    --dry-run)       DRY_RUN=1 ;;
    --backup)        DO_BACKUP=1 ;;
    --no-rollback)   AUTO_ROLLBACK=0 ;;
    -h|--help)       usage; exit 0 ;;
    *) die "Opsi tidak dikenal: $1" "Jalankan 'bash scripts/deploy/deploy.sh --help'." ;;
  esac
  shift
done

export DRY_RUN

main() {
  cd "$GARDA_ROOT"

  # Auto-decide Caddyfile sync: enabled on full deploy when GATEWAY_MODE=external
  if [[ "$SYNC_CADDY" -eq -1 ]]; then
    if [[ -z "$ONLY_SERVICES" && "${GATEWAY_MODE:-external}" == "external" ]]; then
      SYNC_CADDY=1
    else
      SYNC_CADDY=0
    fi
  fi

  # ── Step 1: resolve RELEASE_TAG (no Git) ───────────────────────────────
  if [[ -n "$TAG_OVERRIDE" ]]; then
    RELEASE_TAG="$TAG_OVERRIDE"
  else
    RELEASE_TAG="v$(date -u +%Y%m%d-%H%M%S)"
  fi
  export RELEASE_TAG

  log_section "Deploy Garda Inflasi (Sync → Server Build → Clean)"
  print_kv "Mode"          "$([[ "$DRY_RUN" -eq 1 ]] && echo 'DRY-RUN (VM tidak disentuh)' || echo 'LIVE')"
  print_kv "Release tag"   "$RELEASE_TAG"
  print_kv "Gateway mode"  "${GATEWAY_MODE}"
  print_kv "Public host"   "${PUBLIC_HOST}"
  print_kv "Only services" "${ONLY_SERVICES:-<all app services>}"
  print_kv "Init VM"       "$([[ "$DO_INIT" -eq 1 ]] && echo yes || echo no)"
  print_kv "Sync source"   "$([[ "$SKIP_SYNC" -eq 1 ]] && echo no || echo yes)"
  print_kv "Build on VM"   "$([[ "$SKIP_BUILD" -eq 1 ]] && echo no || echo yes)"
  print_kv "Clean src"     "$([[ "$KEEP_SRC" -eq 1 ]] && echo no || echo yes)"
  print_kv "Sync Caddy"    "$([[ "$SYNC_CADDY" -eq 1 ]] && echo yes || echo no)"
  print_kv "Backup DB"     "$([[ "$DO_BACKUP" -eq 1 ]] && echo yes || echo no)"
  print_kv "Auto rollback" "$([[ "$AUTO_ROLLBACK" -eq 1 ]] && echo yes || echo no)"

  # ── Step 2: optional server bootstrap (--init) ─────────────────────────
  if [[ "$DO_INIT" -eq 1 ]]; then
    init_args=()
    [[ "$DRY_RUN" -eq 1 ]] && init_args+=("--dry-run")
    bash "${SCRIPT_DIR}/init-server.sh" ${init_args[@]+"${init_args[@]}"}
  fi

  # ── Step 3: pre-flight SSH + Docker on VM ──────────────────────────────
  if [[ "$DRY_RUN" != "1" ]]; then
    vm_preflight
  else
    echo_cmd "vm_preflight  # cek SSH + docker + compose v2 di VM"
  fi

  # ── Step 4: record previous deploy marker ──────────────────────────────
  log_section "Menyimpan marker deploy sebelumnya"
  vm_ssh "cd '${REMOTE_DIR}' && { [ -f .last-deploy ] && cp .last-deploy .previous-deploy || true; }"

  # ── Step 5: optional database backup ───────────────────────────────────
  if [[ "$DO_BACKUP" -eq 1 ]]; then
    log_section "Backup database di VM"
    vm_ssh "mkdir -p '${REMOTE_DIR}/scripts/backup' '${REMOTE_DIR}/backups'"
    vm_rsync "${GARDA_ROOT}/scripts/backup/" "${REMOTE_DIR}/scripts/backup/"
    backup_cmd="cd '${REMOTE_DIR}' && set -a && . ./.env && set +a && BACKUP_DIR=backups sh scripts/backup/backup.sh && ls -lh backups | tail -4"
    if [[ "$DRY_RUN" == "1" ]]; then
      echo_cmd "ssh ${VM_USER}@${VM_HOST} \"${backup_cmd}\""
    else
      if vm_ssh "$backup_cmd"; then
        log_success "Backup database selesai."
      else
        die "Backup database gagal."
      fi
    fi
  fi

  # ── Step 6: sync operational files + clean source code to VM ───────────
  if [[ "$SKIP_SYNC" -eq 1 ]]; then
    log_section "Sync source dilewati (--skip-sync)"
  else
    sync_args=()
    [[ -n "$ONLY_SERVICES" ]] && sync_args+=("--only=${ONLY_SERVICES}")
    [[ "$DRY_RUN" -eq 1 ]] && sync_args+=("--dry-run")

    if ! bash "${SCRIPT_DIR}/sync-source.sh" ${sync_args[@]+"${sync_args[@]}"}; then
      die "Sinkronisasi source ke VM gagal."
    fi
  fi

  # ── Step 7: run remote-deploy.sh (Build on VM → Clean .build-src → Up) ─
  log_section "Menjalankan build & deploy di VM"
  remote_args=(
    "--tag=${RELEASE_TAG}"
    "--gateway=${GATEWAY_MODE}"
    "--public-host=${PUBLIC_HOST}"
    "--next-public-api-base=${NEXT_PUBLIC_API_BASE_URL:-/api}"
    "--next-public-app-url=${NEXT_PUBLIC_APP_URL:-https://${PUBLIC_HOST}}"
  )
  [[ -n "$ONLY_SERVICES" ]] && remote_args+=("--only=${ONLY_SERVICES}")
  [[ "$NO_CACHE" -eq 1 ]] && remote_args+=("--no-cache")
  [[ "$SKIP_BUILD" -eq 1 ]] && remote_args+=("--skip-build")
  [[ "$KEEP_SRC" -eq 1 ]] && remote_args+=("--keep-src")
  [[ "$SKIP_MIGRATE" -eq 1 ]] && remote_args+=("--skip-migrate")
  [[ "$WITH_MIGRATE" -eq 1 ]] && remote_args+=("--with-migrate")

  remote_arg_str=""
  for a in "${remote_args[@]}"; do
    remote_arg_str+=" $(printf '%q' "$a")"
  done

  remote_rc=0
  if [[ "$DRY_RUN" == "1" ]]; then
    echo_cmd "ssh ${VM_USER}@${VM_HOST} \"cd ${REMOTE_DIR} && bash scripts/deploy/remote-deploy.sh${remote_arg_str}\""
  else
    set +e
    vm_ssh "cd '${REMOTE_DIR}' && bash scripts/deploy/remote-deploy.sh${remote_arg_str}"
    remote_rc=$?
    set -e
    if [[ "$remote_rc" -ne 0 ]]; then
      log_error "Remote build/deploy di VM gagal (exit ${remote_rc})."
    fi
  fi

  # ── Step 8: auto-rollback on failure ───────────────────────────────────
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
    die "Deploy gagal pada RELEASE_TAG=${RELEASE_TAG}."
  fi

  # ── Step 9: sync external Caddy gateway (Cloudflare SSL + routing) ─────
  if [[ "$SYNC_CADDY" -eq 1 ]]; then
    caddy_args=()
    [[ "$DRY_RUN" -eq 1 ]] && caddy_args+=("--dry-run")
    bash "${SCRIPT_DIR}/sync-caddy.sh" ${caddy_args[@]+"${caddy_args[@]}"}
  fi

  # ── Step 10: verify public HTTPS endpoint through Cloudflare & Caddy ───
  if [[ -n "${PUBLIC_HOST:-}" ]]; then
    log_section "Verifikasi publik HTTPS — https://${PUBLIC_HOST}/healthz"
    if [[ "$DRY_RUN" == "1" ]]; then
      echo_cmd "curl -fsS https://${PUBLIC_HOST}/healthz"
    else
      pub_ok=0
      for i in $(seq 1 20); do
        if curl -fsS --max-time 8 "https://${PUBLIC_HOST}/healthz" >/dev/null 2>&1; then
          pub_ok=1
          break
        fi
        log_info "Menunggu sertifikat TLS Cloudflare / kesiapan gateway (${i}/20) ..."
        sleep 3
      done
      if [[ "$pub_ok" -eq 1 ]]; then
        log_success "https://${PUBLIC_HOST}/healthz → OK (HTTP 200)"
      else
        log_warn "https://${PUBLIC_HOST}/healthz belum merespons 200 (cek log Caddy / DNS Cloudflare)."
      fi
    fi
  fi

  # ── Step 11: verify server is clean of build source & prune old images ─
  log_section "Verifikasi kebersihan server & prune image lama (keep=${KEEP_LAST_N_IMAGES})"
  if [[ "$DRY_RUN" == "1" ]]; then
    echo_cmd "vm_ssh 'test ! -d .build-src && KEEP_LAST_N_IMAGES=${KEEP_LAST_N_IMAGES} bash scripts/rollback/keep-last-n-images.sh --images-only'"
  else
    if [[ "$KEEP_SRC" -ne 1 ]]; then
      vm_ssh "rm -rf '${REMOTE_DIR}/.build-src' && test ! -d '${REMOTE_DIR}/.build-src'"
      log_success "Server bersih: ${REMOTE_DIR}/.build-src tidak ada."
    fi
    if vm_ssh "cd '${REMOTE_DIR}' && REMOTE_DIR='${REMOTE_DIR}' KEEP_LAST_N_IMAGES='${KEEP_LAST_N_IMAGES}' bash scripts/rollback/keep-last-n-images.sh --images-only"; then
      log_success "Prune image VM selesai."
    else
      log_warn "Prune image VM gagal — tidak fatal."
    fi
  fi

  # ── Step 12: summary ───────────────────────────────────────────────────
  log_section "Deploy selesai"
  print_kv "Release tag" "$RELEASE_TAG"
  print_kv "Timestamp"   "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  print_kv "VM"          "${VM_USER}@${VM_HOST}"
  print_kv "URL"         "https://${PUBLIC_HOST}"
  print_kv "Health"      "https://${PUBLIC_HOST}/healthz"
  log_success "Deploy ${RELEASE_TAG} selesai dengan sukses."
}

load_env_deploy
validate_ssh_key

require_cmd ssh   "ssh tersedia secara default di macOS/Linux."
require_cmd rsync "Install rsync: brew install rsync"

LOCK_PATH="${GARDA_ROOT}/.deploy.lock"
with_lock "$LOCK_PATH" main
