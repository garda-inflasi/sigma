#!/usr/bin/env bash
# ==============================================================================
#  Garda Inflasi — scripts/deploy/rollback.sh
#  Rolls the VM back to the previous per-service tags (.env.tags.prev) or a
#  specific release tag (--to=<tag>). Gateway-mode aware (external/builtin).
#
#  Usage:
#      bash scripts/deploy/rollback.sh [options]
#
#  Options:
#      --auto          Non-interactive mode (used by deploy.sh auto-rollback)
#      --to=<tag>      Roll back all services to a specific release tag
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

AUTO=0
TARGET_TAG=""
DRY_RUN="${DRY_RUN:-0}"

usage() {
  sed -n '2,15p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --auto)    AUTO=1 ;;
    --to=*)    TARGET_TAG="${1#*=}" ;;
    --to)      shift; TARGET_TAG="${1:-}" ;;
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

compose_app_files="-f compose/compose.yaml -f compose/compose.prod.yaml"
app_services="api worker ml web"
if [[ "${GATEWAY_MODE:-external}" == "external" ]]; then
  compose_app_files+=" -f compose/compose.gateway.yaml"
else
  app_services+=" caddy"
fi

if [[ "$DRY_RUN" == "1" ]]; then
  echo_cmd "vm_ssh 'restore .env.tags.prev or tag=${TARGET_TAG:-prev} & docker compose ${compose_app_files} up -d --no-build --no-deps ${app_services}'"
  exit 0
fi

# Confirmation unless --auto
if [[ "$AUTO" -ne 1 && -t 0 ]]; then
  target_label="${TARGET_TAG:-.env.tags.prev}"
  printf '%sLanjutkan rollback ke %s? [y/N] %s' "$C_YELLOW" "$target_label" "$C_RESET" >&2
  read -r reply
  case "$reply" in
    [yY]|[yY][eE][sS]) ;;
    *) log_info "Rollback dibatalkan."; exit 0 ;;
  esac
fi

log_section "Rollback Garda Inflasi (${VM_USER}@${VM_HOST})"

remote_rollback=$(cat <<EOF
set -euo pipefail
cd '${REMOTE_DIR}'
set -a
. ./.env
set +a
export POSTGRES_DB="\${POSTGRES_DB:-garda_inflasi}"
export POSTGRES_USER="\${POSTGRES_USER:-garda}"
export DB_USER="\${DB_USER:-\$POSTGRES_USER}"
export DB_PASSWORD="\${DB_PASSWORD:-\$POSTGRES_PASSWORD}"
export DB_NAME="\${DB_NAME:-\$POSTGRES_DB}"
export APP_ENV=production

if [ -n "${TARGET_TAG}" ]; then
  export IMAGE_TAG="${TARGET_TAG}"
  export API_IMAGE_TAG="${TARGET_TAG}"
  export WEB_IMAGE_TAG="${TARGET_TAG}"
  export ML_IMAGE_TAG="${TARGET_TAG}"
  export CADDY_IMAGE_TAG="${TARGET_TAG}"
  cat > ./.env.tags <<EOT
IMAGE_TAG=${TARGET_TAG}
API_IMAGE_TAG=${TARGET_TAG}
WEB_IMAGE_TAG=${TARGET_TAG}
ML_IMAGE_TAG=${TARGET_TAG}
CADDY_IMAGE_TAG=${TARGET_TAG}
EOT
elif [ -f ./.env.tags.prev ]; then
  cp ./.env.tags ./.env.tags.failed || true
  cp ./.env.tags.prev ./.env.tags
  set -a
  . ./.env.tags
  set +a
elif [ -f ./.previous-deploy ]; then
  prev="\$(tr -d '[:space:]' < ./.previous-deploy)"
  [ -n "\$prev" ] || { echo "Tidak ada target rollback." >&2; exit 1; }
  export IMAGE_TAG="\$prev"
  export API_IMAGE_TAG="\$prev"
  export WEB_IMAGE_TAG="\$prev"
  export ML_IMAGE_TAG="\$prev"
  export CADDY_IMAGE_TAG="\$prev"
else
  echo "Tidak ditemukan .env.tags.prev maupun .previous-deploy di VM." >&2
  exit 1
fi

for pair in "api:\${API_IMAGE_TAG}" "web:\${WEB_IMAGE_TAG}" "ml:\${ML_IMAGE_TAG}"; do
  svc="\${pair%%:*}"
  tag="\${pair#*:}"
  docker image inspect "garda-\${svc}:\${tag}" >/dev/null 2>&1 || {
    echo "Image garda-\${svc}:\${tag} tidak ditemukan di VM." >&2
    exit 1
  }
done

docker compose --project-directory '${REMOTE_DIR}' ${compose_app_files} up -d --no-build --no-deps ${app_services}

for i in \$(seq 1 10); do
  if docker exec garda-api wget -qO- http://127.0.0.1:8080/healthz >/dev/null 2>&1 \
     && docker exec garda-web wget -qO- http://127.0.0.1:3000/ >/dev/null 2>&1; then
    echo "Rollback healthcheck OK (API=\${API_IMAGE_TAG}, WEB=\${WEB_IMAGE_TAG}, ML=\${ML_IMAGE_TAG})"
    exit 0
  fi
  sleep 3
done
echo "Healthcheck gagal setelah rollback." >&2
exit 1
EOF
)

vm_ssh "$remote_rollback"
log_success "Rollback berhasil dan layanan sehat."
