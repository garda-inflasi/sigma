#!/usr/bin/env bash
# ==============================================================================
#  Garda Inflasi — scripts/deploy/status.sh
#  Shows the state of every service on the VM plus live resource usage.
#
#  Usage:
#      bash scripts/deploy/status.sh [options]
#
#  Options:
#      --no-stats      Skip `docker stats`
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

SHOW_STATS=1
DRY_RUN="${DRY_RUN:-0}"

usage() {
  sed -n '2,14p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --no-stats) SHOW_STATS=0 ;;
    --dry-run)  DRY_RUN=1 ;;
    -h|--help)  usage; exit 0 ;;
    *) die "Opsi tidak dikenal: $1" "Jalankan 'bash scripts/deploy/status.sh --help'." ;;
  esac
  shift
done

export DRY_RUN

load_env_deploy
validate_ssh_key

require_cmd ssh "ssh tersedia secara default di macOS/Linux."

cd "$GARDA_ROOT"

# ------------------------------------------------------------------------------
#  Header
# ------------------------------------------------------------------------------
log_section "Status Garda Inflasi — ${VM_USER}@${VM_HOST}"
print_kv "Remote dir" "$REMOTE_DIR"
print_kv "Timestamp"  "$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# ------------------------------------------------------------------------------
#  Deployed version
# ------------------------------------------------------------------------------
log_section "Versi ter-deploy"
if [[ "$DRY_RUN" == "1" ]]; then
  echo_cmd "ssh ${VM_USER}@${VM_HOST} \"cat ${REMOTE_DIR}/.last-deploy\""
else
  last_sha="$(vm_ssh "cat '${REMOTE_DIR}/.last-deploy' 2>/dev/null || echo '<belum pernah deploy>'" | tr -d '[:space:]')"
  last_time="$(vm_ssh "cat '${REMOTE_DIR}/.last-deploy-time' 2>/dev/null || echo '-'" | tr -d '[:space:]')"
  prev_sha="$(vm_ssh "cat '${REMOTE_DIR}/.previous-deploy' 2>/dev/null || echo '-'" | tr -d '[:space:]')"
  print_kv "Aktif"     "${last_sha:-<unknown>}"
  print_kv "Deployed at" "${last_time:-<unknown>}"
  print_kv "Sebelumnya" "${prev_sha:-none}"
fi

# ------------------------------------------------------------------------------
#  Compose ps — infra + app
# ------------------------------------------------------------------------------
log_section "Container — infra"
vm_ssh "cd '${REMOTE_DIR}' && docker compose -f compose/compose.infra.yaml -f compose/compose.prod.yaml ps" || \
  log_warn "Tidak bisa membaca status infra."

log_section "Container — aplikasi"
vm_ssh "cd '${REMOTE_DIR}' && docker compose -f compose/compose.yaml -f compose/compose.prod.yaml ps" || \
  log_warn "Tidak bisa membaca status aplikasi."

# ------------------------------------------------------------------------------
#  Resource usage
# ------------------------------------------------------------------------------
if [[ "$SHOW_STATS" -eq 1 ]]; then
  log_section "Resource usage"
  vm_ssh "docker stats --no-stream --format 'table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}'" || \
    log_warn "Tidak bisa membaca docker stats."
fi

# ------------------------------------------------------------------------------
#  Disk usage on the VM
# ------------------------------------------------------------------------------
log_section "Disk VM"
vm_ssh "df -h '${REMOTE_DIR}' | tail -1" || log_warn "Tidak bisa membaca disk VM."

# ------------------------------------------------------------------------------
#  Public healthcheck
# ------------------------------------------------------------------------------
if [[ -n "${PUBLIC_HOST:-}" ]]; then
  log_section "Healthcheck publik — https://${PUBLIC_HOST}/healthz"
  if [[ "$DRY_RUN" == "1" ]]; then
    echo_cmd "curl -fsS https://${PUBLIC_HOST}/healthz"
  else
    if curl -fsS --max-time 10 "https://${PUBLIC_HOST}/healthz" >/dev/null 2>&1; then
      log_success "https://${PUBLIC_HOST}/healthz → OK"
    else
      log_warn "https://${PUBLIC_HOST}/healthz tidak merespons (DNS/SSL/firewall?)."
    fi
  fi
else
  log_section "Healthcheck lokal VM"
  if [[ "$DRY_RUN" == "1" ]]; then
    echo_cmd "ssh ${VM_USER}@${VM_HOST} \"curl -fsS http://localhost/healthz\""
  else
    if vm_ssh "curl -fsS --max-time 5 http://localhost/healthz >/dev/null 2>&1"; then
      log_success "http://localhost/healthz (di VM) → OK"
    else
      log_warn "http://localhost/healthz (di VM) tidak merespons."
    fi
  fi
fi

log_success "Status selesai."