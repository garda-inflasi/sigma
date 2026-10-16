#!/usr/bin/env bash
# ==============================================================================
#  Garda Inflasi — scripts/deploy/rollback.sh
#  Rolls the VM back to the previously deployed image tag.
#
#  Usage:
#      bash scripts/deploy/rollback.sh [options]
#
#  Options:
#      --auto          Non-interactive mode (used by deploy.sh auto-rollback)
#      --to=<sha>      Roll back to a specific SHA instead of .previous-deploy
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
AUTO=0
TARGET_SHA=""
DRY_RUN="${DRY_RUN:-0}"

usage() {
  sed -n '2,15p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --auto)    AUTO=1 ;;
    --to=*)    TARGET_SHA="${1#*=}" ;;
    --to)      shift; TARGET_SHA="${1:-}" ;;
    --dry-run) DRY_RUN=1 ;;
    -h|--help) usage; exit 0 ;;
    *) die "Opsi tidak dikenal: $1" "Jalankan 'bash scripts/deploy/rollback.sh --help'." ;;
  esac
  shift
done

export DRY_RUN

load_env_deploy
validate_ssh_key

require_cmd ssh "ssh tersedia secara default di macOS/Linux."

cd "$GARDA_ROOT"

# ------------------------------------------------------------------------------
#  Resolve the target SHA
# ------------------------------------------------------------------------------
CURRENT_SHA=""
PREVIOUS_SHA=""

if [[ "$DRY_RUN" != "1" ]]; then
  CURRENT_SHA="$(vm_ssh "cat '${REMOTE_DIR}/.last-deploy' 2>/dev/null || echo ''" | tr -d '[:space:]')"
  PREVIOUS_SHA="$(vm_ssh "cat '${REMOTE_DIR}/.previous-deploy' 2>/dev/null || echo ''" | tr -d '[:space:]')"
else
  echo_cmd "ssh ${VM_USER}@${VM_HOST} \"cat ${REMOTE_DIR}/.last-deploy\""
  echo_cmd "ssh ${VM_USER}@${VM_HOST} \"cat ${REMOTE_DIR}/.previous-deploy\""
fi

if [[ -n "$TARGET_SHA" ]]; then
  ROLLBACK_SHA="$TARGET_SHA"
  log_info "Target rollback eksplisit: ${ROLLBACK_SHA}"
else
  ROLLBACK_SHA="$PREVIOUS_SHA"
fi

if [[ -z "$ROLLBACK_SHA" ]]; then
  die "Tidak ada versi sebelumnya untuk di-rollback." \
      "Marker .previous-deploy kosong di VM. Tentukan target manual: make rollback ARGS=--to=<sha>"
fi

if [[ "$ROLLBACK_SHA" == "$CURRENT_SHA" ]]; then
  log_warn "Target rollback sama dengan versi aktif (${ROLLBACK_SHA}) — tidak ada yang berubah."
  exit 0
fi

log_section "Rollback Garda Inflasi"
print_kv "Versi aktif"  "${CURRENT_SHA:-<unknown>}"
print_kv "Target"       "$ROLLBACK_SHA"

# ------------------------------------------------------------------------------
#  Confirm (unless --auto)
# ------------------------------------------------------------------------------
if [[ "$AUTO" -ne 1 && "$DRY_RUN" != "1" && -t 0 ]]; then
  printf '%sLanjutkan rollback ke %s? [y/N] %s' "$C_YELLOW" "$ROLLBACK_SHA" "$C_RESET" >&2
  read -r reply
  case "$reply" in
    [yY]|[yY][eE][sS]) ;;
    *) log_info "Rollback dibatalkan."; exit 0 ;;
  esac
fi

# ------------------------------------------------------------------------------
#  Verify the target images exist on the VM
# ------------------------------------------------------------------------------
verify_images() {
  local missing=()
  local svc
  for svc in api web caddy; do
    if ! vm_ssh "docker image inspect 'garda-${svc}:${ROLLBACK_SHA}' >/dev/null 2>&1"; then
      missing+=("garda-${svc}:${ROLLBACK_SHA}")
    fi
  done

  if [[ "${#missing[@]}" -gt 0 ]]; then
    log_error "Image target tidak ada di VM:"
    local m
    for m in "${missing[@]}"; do printf '        - %s\n' "$m" >&2; done
    die "Rollback tidak mungkin tanpa image target." \
        "Image lama mungkin sudah di-prune. Deploy ulang SHA tersebut dari Mac, atau pilih SHA lain: make rollback ARGS=--to=<sha>"
  fi
  log_success "Semua image target tersedia di VM."
}

if [[ "$DRY_RUN" == "1" ]]; then
  echo_cmd "ssh ${VM_USER}@${VM_HOST} \"docker image inspect garda-api:${ROLLBACK_SHA}\""
else
  verify_images
fi

# ------------------------------------------------------------------------------
#  Retag target → latest, then bring the stack up with IMAGE_TAG=<target>
# ------------------------------------------------------------------------------
log_section "Retag image target sebagai :latest"
for svc in api web caddy; do
  vm_ssh "docker tag 'garda-${svc}:${ROLLBACK_SHA}' 'garda-${svc}:latest'"
  log_success "garda-${svc}:${ROLLBACK_SHA} → garda-${svc}:latest"
done

log_section "Up stack dengan IMAGE_TAG=${ROLLBACK_SHA}"
rollback_cmd="cd '${REMOTE_DIR}' && \
  export IMAGE_TAG='${ROLLBACK_SHA}' && \
  docker compose -f compose/compose.infra.yaml -f compose/compose.prod.yaml up -d --no-build --remove-orphans && \
  docker compose -f compose/compose.yaml -f compose/compose.prod.yaml up -d --no-build --remove-orphans"

if [[ "$DRY_RUN" == "1" ]]; then
  echo_cmd "ssh ${VM_USER}@${VM_HOST} \"${rollback_cmd}\""
else
  if ! vm_ssh "$rollback_cmd"; then
    die "Rollback gagal saat menjalankan docker compose up." \
        "Periksa log: make logs SERVICE=api — lalu coba lagi atau deploy ulang dari Mac."
  fi
fi

# ------------------------------------------------------------------------------
#  HTTP healthcheck
# ------------------------------------------------------------------------------
log_section "Healthcheck setelah rollback"
health_ok=0
if [[ "$DRY_RUN" == "1" ]]; then
  echo_cmd "ssh ${VM_USER}@${VM_HOST} \"curl -fsS http://localhost/healthz\""
  health_ok=1
else
  for i in $(seq 1 10); do
    if vm_ssh "curl -fsS --max-time 5 http://localhost/healthz >/dev/null 2>&1"; then
      health_ok=1
      break
    fi
    log_debug "Healthcheck ${i}/10 gagal — ulang dalam 3s"
    sleep 3
  done
fi

if [[ "$health_ok" -ne 1 ]]; then
  log_error "Healthcheck gagal setelah rollback."
  vm_ssh "cd '${REMOTE_DIR}' && docker compose -f compose/compose.yaml -f compose/compose.prod.yaml logs --tail=80" || true
  die "Rollback selesai tetapi layanan tidak sehat." \
      "Periksa log di atas. Kemungkinan image target juga bermasalah — pertimbangkan rollback ke SHA lain."
fi
log_success "Healthcheck OK setelah rollback."

# ------------------------------------------------------------------------------
#  Swap the deploy markers
# ------------------------------------------------------------------------------
log_section "Menukar marker deploy"
swap_cmd="cd '${REMOTE_DIR}' && \
  printf '%s\n' '${ROLLBACK_SHA}' > .last-deploy && \
  printf '%s\n' '${CURRENT_SHA:-none}' > .previous-deploy && \
  printf '%s\n' \"\$(date -u +%Y-%m-%dT%H:%M:%SZ)\" > .last-deploy-time"

if [[ "$DRY_RUN" == "1" ]]; then
  echo_cmd "ssh ${VM_USER}@${VM_HOST} \"${swap_cmd}\""
else
  vm_ssh "$swap_cmd"
  log_success "Marker ditukar: .last-deploy=${ROLLBACK_SHA}, .previous-deploy=${CURRENT_SHA:-none}"
fi

# ------------------------------------------------------------------------------
#  Summary
# ------------------------------------------------------------------------------
log_section "Rollback selesai"
print_kv "Aktif sekarang" "$ROLLBACK_SHA"
print_kv "Sebelumnya"     "${CURRENT_SHA:-none}"
print_kv "Timestamp"      "$(date -u +%Y-%m-%dT%H:%M:%SZ)"

log_info "Status container:"
vm_ssh "cd '${REMOTE_DIR}' && docker compose -f compose/compose.yaml -f compose/compose.prod.yaml ps" || true

log_success "Rollback ke ${ROLLBACK_SHA} berhasil."