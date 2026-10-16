#!/usr/bin/env bash
# ==============================================================================
#  Garda Inflasi — scripts/deploy/logs.sh
#  Tails container logs on the VM.
#
#  Usage:
#      bash scripts/deploy/logs.sh [service] [options]
#
#  Arguments:
#      service         api | web | caddy | postgres | mongodb | redis | all
#                      (default: api)
#
#  Options:
#      --tail=N        Number of lines to show (default: 200)
#      --since=<time>  Show logs since a timestamp (e.g. 10m, 2h, 2024-01-01)
#      --no-follow     Print and exit instead of following
#      --dry-run       Print commands without touching the VM
#      -h, --help      Show this help
#
#  Environment:
#      SERVICE=api TAIL=200 make logs
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
SERVICE="${SERVICE:-api}"
TAIL="${TAIL:-200}"
SINCE=""
FOLLOW=1
DRY_RUN="${DRY_RUN:-0}"

usage() {
  sed -n '2,22p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --tail=*)    TAIL="${1#*=}" ;;
    --tail)      shift; TAIL="${1:-200}" ;;
    --since=*)   SINCE="${1#*=}" ;;
    --since)     shift; SINCE="${1:-}" ;;
    --no-follow) FOLLOW=0 ;;
    --dry-run)   DRY_RUN=1 ;;
    -h|--help)   usage; exit 0 ;;
    -*)          die "Opsi tidak dikenal: $1" "Jalankan 'bash scripts/deploy/logs.sh --help'." ;;
    *)           SERVICE="$1" ;;
  esac
  shift
done

export DRY_RUN

load_env_deploy
validate_ssh_key

require_cmd ssh "ssh tersedia secara default di macOS/Linux."

cd "$GARDA_ROOT"

# ------------------------------------------------------------------------------
#  Validate the service name
# ------------------------------------------------------------------------------
INFRA_SERVICES=(postgres mongodb redis)
APP_SERVICES=(api web caddy)

if [[ "$SERVICE" != "all" ]]; then
  valid=0
  for s in "${INFRA_SERVICES[@]}" "${APP_SERVICES[@]}"; do
    [[ "$SERVICE" == "$s" ]] && valid=1
  done
  if [[ "$valid" -ne 1 ]]; then
    die "Service tidak dikenal: '$SERVICE'" \
        "Pilih salah satu: api, web, caddy, postgres, mongodb, redis, all"
  fi
fi

# ------------------------------------------------------------------------------
#  Build the compose command
# ------------------------------------------------------------------------------
compose_files="-f compose/compose.infra.yaml -f compose/compose.yaml -f compose/compose.prod.yaml"

log_args=("logs" "--tail=${TAIL}")
[[ "$FOLLOW" -eq 1 ]] && log_args+=("-f")
[[ -n "$SINCE" ]] && log_args+=("--since=${SINCE}")
[[ "$SERVICE" != "all" ]] && log_args+=("$SERVICE")

log_section "Logs — ${SERVICE} (tail=${TAIL}${SINCE:+, since=${SINCE}}${FOLLOW:+, follow})"
print_kv "VM" "${VM_USER}@${VM_HOST}"

remote_cmd="cd '${REMOTE_DIR}' && docker compose ${compose_files} ${log_args[*]}"

if [[ "$DRY_RUN" == "1" ]]; then
  echo_cmd "ssh ${VM_USER}@${VM_HOST} \"${remote_cmd}\""
  exit 0
fi

if [[ "$FOLLOW" -eq 1 ]]; then
  log_info "Tekan Ctrl-C untuk berhenti mengikuti log."
fi

# `exec` is intentionally avoided so the EXIT trap still runs.
vm_ssh "$remote_cmd"