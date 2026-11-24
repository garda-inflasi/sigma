#!/usr/bin/env bash
# ==============================================================================
#  Garda Inflasi — scripts/deploy/sync-caddy.sh
#  Updates the external Caddy gateway Caddyfile on the VM safely:
#    1. Backs up existing Caddyfile with timestamp.
#    2. Preserves Cloudflare DNS ACME TLS config (CF_API_TOKEN + ACME_EMAIL)
#       and routes PUBLIC_HOST (app.gardainflasi.id) to garda-api:8080 &
#       garda-web:3000 per docker/Caddyfile routing contract.
#    3. Validates syntax via `caddy validate` (auto-restores backup on error).
#    4. Reloads Caddy zero-downtime via `caddy reload`.
#
#  Usage:
#      bash scripts/deploy/sync-caddy.sh [--dry-run]
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"
# shellcheck source=../lib/ssh-wrapper.sh
source "${SCRIPT_DIR}/../lib/ssh-wrapper.sh"

install_traps

DRY_RUN="${DRY_RUN:-0}"

usage() {
  sed -n '2,14p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=1 ;;
    -h|--help) usage; exit 0 ;;
    *) die "Opsi tidak dikenal: $1" "Jalankan 'bash scripts/deploy/sync-caddy.sh --help'." ;;
  esac
  shift
done

export DRY_RUN

load_env_deploy
validate_ssh_key

if [[ "$GATEWAY_MODE" != "external" ]]; then
  log_info "GATEWAY_MODE=${GATEWAY_MODE} — melewati sync-caddy eksternal."
  exit 0
fi

CADDYFILE_PATH="${GATEWAY_CADDY_DIR}/caddy/Caddyfile"
TIMESTAMP="$(date -u +%Y%m%d%H%M%S)"

log_section "Sinkronisasi Caddyfile Gateway (${PUBLIC_HOST})"
print_kv "VM"         "${VM_USER}@${VM_HOST}"
print_kv "Caddy dir"  "$GATEWAY_CADDY_DIR"
print_kv "Public"     "$PUBLIC_HOST"
print_kv "Apex"       "${APEX_HOST:-<none>}"

# Build Caddyfile content dynamically in memory.
APEX_BLOCK=""
if [[ -n "${APEX_HOST:-}" && "${APEX_HOST}" != "${PUBLIC_HOST}" ]]; then
  APEX_BLOCK="
${APEX_HOST} {
    redir https://${PUBLIC_HOST}{uri} permanent
}
"
fi

# Build optional pgAdmin /pgm Basic-Auth block from VM .env (if configured) or local env.
PGADMIN_HTTP_USER="${PGADMIN_HTTP_USER:-}"
PGADMIN_HTTP_HASH="${PGADMIN_HTTP_HASH:-}"
if [[ -z "$PGADMIN_HTTP_USER" || -z "$PGADMIN_HTTP_HASH" ]]; then
  PGM_ENV_OUT="$(vm_ssh "grep -E '^PGADMIN_HTTP_(USER|HASH)=' '${REMOTE_DIR}/.env' 2>/dev/null || true")"
  while IFS='=' read -r k v; do
    v="${v%\"}"; v="${v#\"}"
    v="${v%\'}"; v="${v#\'}"
    case "$k" in
      PGADMIN_HTTP_USER) [[ -z "$PGADMIN_HTTP_USER" ]] && PGADMIN_HTTP_USER="$v" ;;
      PGADMIN_HTTP_HASH) [[ -z "$PGADMIN_HTTP_HASH" ]] && PGADMIN_HTTP_HASH="$v" ;;
    esac
  done <<< "$PGM_ENV_OUT"
fi

PGADMIN_BLOCK=""
if [[ -n "$PGADMIN_HTTP_USER" && -n "$PGADMIN_HTTP_HASH" ]]; then
  PGADMIN_BLOCK="
    handle /pgm* {
        basic_auth {
            ${PGADMIN_HTTP_USER} ${PGADMIN_HTTP_HASH}
        }
        @pgm_root path /pgm
        redir @pgm_root /pgm/ permanent
        header X-Frame-Options \"SAMEORIGIN\"
        reverse_proxy garda-pgadmin:80 {
            header_up X-Script-Name /pgm
        }
    }
"
fi

CADDYFILE_CONTENT=$(cat <<EOF
{
    email {\$ACME_EMAIL}
    acme_dns cloudflare {env.CF_API_TOKEN}

    # Secara eksplisit mengaktifkan HTTP/1.1, HTTP/2, dan HTTP/3
    servers {
        protocols h1 h2 h3
    }
}

${PUBLIC_HOST} {
    encode gzip zstd

    log {
        output stdout
        format json
    }

    header {
        X-Content-Type-Options "nosniff"
        Referrer-Policy "strict-origin-when-cross-origin"
        Permissions-Policy "camera=(self), geolocation=(self), microphone=()"
        Cross-Origin-Opener-Policy "same-origin"
        -Server
    }

    @not_pgm not path /pgm*
    header @not_pgm X-Frame-Options "DENY"

    @https protocol https
    header @https Strict-Transport-Security "max-age=63072000; includeSubDomains; preload"

    handle /healthz {
        reverse_proxy garda-api:8080
    }

    handle /swagger/* {
        reverse_proxy garda-api:8080
    }

    # flush_interval -1 wajib untuk SSE (/api/v1/ews/stream)
    handle_path /api/v1/* {
        reverse_proxy garda-api:8080 {
            flush_interval -1
        }
    }

    # Next.js BFF Route Handlers (/api/* mempertahankan prefix /api)
    handle /api/* {
        reverse_proxy garda-web:3000
    }
${PGADMIN_BLOCK}
    handle {
        reverse_proxy garda-web:3000
    }
}
${APEX_BLOCK}
EOF
)

CADDY_B64="$(printf '%s\n' "$CADDYFILE_CONTENT" | base64 | tr -d '\n')"

REMOTE_CMD=$(cat <<EOF
set -euo pipefail
CADDYFILE="${CADDYFILE_PATH}"
BACKUP="\${CADDYFILE}.bak.${TIMESTAMP}"

if [ -f "\$CADDYFILE" ]; then
  cp "\$CADDYFILE" "\$BACKUP"
fi

printf '%s' "${CADDY_B64}" | base64 -d > "\${CADDYFILE}.tmp"
chmod 644 "\${CADDYFILE}.tmp"
# Write in-place (>) instead of mv so Docker single-file bind mounts keep the same inode inside the container.
cat "\${CADDYFILE}.tmp" > "\$CADDYFILE"
rm -f "\${CADDYFILE}.tmp"

# Heal stale bind-mount inode if a previous tool replaced the file via mv.
if ! docker exec caddy cat /etc/caddy/Caddyfile 2>/dev/null | cmp -s - "\$CADDYFILE"; then
  echo "Bind-mount inode berbeda; me-restart caddy untuk mengaitkan Caddyfile baru..." >&2
  (cd "${GATEWAY_CADDY_DIR}" && docker compose restart caddy)
  sleep 2
fi

if ! docker exec caddy caddy validate --config /etc/caddy/Caddyfile >/dev/null 2>&1; then
  echo "VALIDASI CADDYFILE GAGAL! Mengembalikan backup..." >&2
  docker exec caddy caddy validate --config /etc/caddy/Caddyfile || true
  if [ -f "\$BACKUP" ]; then
    cp "\$BACKUP" "\$CADDYFILE"
  fi
  exit 1
fi

docker exec caddy caddy reload --config /etc/caddy/Caddyfile --force
echo "Caddy reloaded OK (backup: \$BACKUP)"
EOF
)

if [[ "$DRY_RUN" == "1" ]]; then
  echo_cmd "vm_ssh_sudo 'backup + write ${CADDYFILE_PATH} + caddy validate + caddy reload'"
  exit 0
fi

vm_ssh_sudo "$REMOTE_CMD"
log_success "Caddy gateway tersinkronisasi & reloaded untuk https://${PUBLIC_HOST}."
