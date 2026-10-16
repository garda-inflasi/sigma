#!/usr/bin/env bash
# ==============================================================================
#  Garda Inflasi — scripts/deploy/remote-deploy.sh
#
#  ⚠️  THIS SCRIPT RUNS ON THE VM, NOT ON THE MAC.
#
#  It is uploaded and executed by deploy.sh via:
#      ssh "$VM_USER@$VM_HOST" "bash -s -- $GIT_SHA" < remote-deploy.sh
#
#  Contract:
#    * The VM NEVER builds anything — every `docker compose up` uses --no-build.
#    * Images arrive via `docker load` from dist/garda-images-<sha>.tar.gz.
#    * Exit code 0 = deploy healthy. Non-zero = deploy.sh triggers auto-rollback.
#
#  Usage (on the VM):
#      cd "$REMOTE_DIR" && bash scripts/deploy/remote-deploy.sh <git-sha>
# ==============================================================================

set -euo pipefail

# ------------------------------------------------------------------------------
#  Error trap — prints the failing line + command so the Mac-side log is useful.
#  (This script runs standalone on the VM, so it installs its own trap rather
#  than relying on common.sh's install_traps.)
# ------------------------------------------------------------------------------
__garda_remote_err() {
  local exit_code=$?
  printf '\n[ERROR] remote-deploy gagal (exit=%s) di %s:%s\n' \
    "$exit_code" "${BASH_SOURCE[0]:-remote-deploy.sh}" "${1:-?}" >&2
  printf '        Perintah: %s\n' "${2:-?}" >&2
  printf '        ↳ Solusi: periksa log di atas, lalu jalankan ulang dari Mac: make deploy\n' >&2
  exit "$exit_code"
}
trap '__garda_remote_err "$LINENO" "$BASH_COMMAND"' ERR

# ------------------------------------------------------------------------------
#  Locate ourselves + load shared helpers
# ------------------------------------------------------------------------------
REMOTE_DIR="${REMOTE_DIR:-$(pwd)}"
cd "$REMOTE_DIR"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
if [[ -f "${SCRIPT_DIR}/../lib/common.sh" ]]; then
  # shellcheck source=../lib/common.sh
  source "${SCRIPT_DIR}/../lib/common.sh"
elif [[ -f "${REMOTE_DIR}/scripts/lib/common.sh" ]]; then
  # shellcheck source=/dev/null
  source "${REMOTE_DIR}/scripts/lib/common.sh"
else
  # Minimal fallback so the script still works if helpers were not transferred.
  log_info()  { printf '[INFO ] %s\n' "$*" >&2; }
  log_warn()  { printf '[WARN ] %s\n' "$*" >&2; }
  log_error() { printf '[ERROR] %s\n' "$*" >&2; }
  log_success() { printf '[ OK  ] %s\n' "$*" >&2; }
  log_section() { printf '\n── %s ──\n' "$*" >&2; }
  log_debug() { :; }
  die() { log_error "${1:-Unknown error}"; [[ -n "${2:-}" ]] && printf '        ↳ Solusi: %s\n' "$2" >&2; exit 1; }
  human_size() { awk -v b="${1:-0}" 'BEGIN { split("B KB MB GB TB", u, " "); i=1; while (b>=1024 && i<5) { b/=1024; i++ } printf "%.2f %s", b, u[i] }'; }
fi

# ------------------------------------------------------------------------------
#  Arguments
# ------------------------------------------------------------------------------
GIT_SHA="${1:-}"
if [[ -z "$GIT_SHA" ]]; then
  die "GIT_SHA wajib diberikan sebagai argumen pertama." \
      "Jalankan dari Mac: bash scripts/deploy/deploy.sh (script ini dipanggil otomatis)."
fi

# ------------------------------------------------------------------------------
#  Configuration
# ------------------------------------------------------------------------------
COMPOSE_INFRA="compose/compose.infra.yaml"
COMPOSE_APP="compose/compose.yaml"
COMPOSE_PROD="compose/compose.prod.yaml"
TARBALL="dist/garda-images-${GIT_SHA}.tar.gz"
NETWORK_NAME="garda_inflasi_net"
HEALTH_TIMEOUT_SECONDS="${HEALTH_TIMEOUT_SECONDS:-60}"
HTTP_HEALTH_RETRIES="${HTTP_HEALTH_RETRIES:-10}"
HTTP_HEALTH_DELAY="${HTTP_HEALTH_DELAY:-3}"

export IMAGE_TAG="$GIT_SHA"

# ------------------------------------------------------------------------------
#  Pre-flight
# ------------------------------------------------------------------------------
log_section "Remote deploy — Garda Inflasi (VM)"
log_info "GIT_SHA      : ${GIT_SHA}"
log_info "REMOTE_DIR   : ${REMOTE_DIR}"
log_info "IMAGE_TAG    : ${IMAGE_TAG}"

for f in "$COMPOSE_INFRA" "$COMPOSE_APP" "$COMPOSE_PROD" "$TARBALL"; do
  [[ -f "$f" ]] || die "File wajib tidak ditemukan di VM: ${REMOTE_DIR}/${f}" \
    "Jalankan transfer dari Mac: make deploy/transfer"
done

if [[ ! -f ./.env ]]; then
  die "File .env tidak ditemukan di ${REMOTE_DIR}." \
      "Buat .env production di VM (lihat docs/DEPLOYMENT.md § First-time VM setup)."
fi

# shellcheck disable=SC1091
set -a
source ./.env
set +a

# Compose needs the same variables the app containers use.
export POSTGRES_DB="${POSTGRES_DB:-garda_inflasi}"
export POSTGRES_USER="${POSTGRES_USER:-garda}"
export POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-garda_secret}"
export DB_USER="${DB_USER:-$POSTGRES_USER}"
export DB_PASSWORD="${DB_PASSWORD:-$POSTGRES_PASSWORD}"
export DB_NAME="${DB_NAME:-$POSTGRES_DB}"

# ------------------------------------------------------------------------------
#  Compose helper — always --no-build (the VM must never build)
# ------------------------------------------------------------------------------
compose_infra() {
  docker compose -f "$COMPOSE_INFRA" -f "$COMPOSE_PROD" "$@"
}
compose_app() {
  docker compose -f "$COMPOSE_APP" -f "$COMPOSE_PROD" "$@"
}

# ------------------------------------------------------------------------------
#  1. Load images
# ------------------------------------------------------------------------------
log_section "1/8 — Load image dari tarball"
log_info "Tarball: $(human_size "$(stat -c%s "$TARBALL" 2>/dev/null || stat -f%z "$TARBALL")")"

if ! gunzip -c "$TARBALL" | docker load; then
  die "Gagal memuat image dari ${TARBALL}." \
      "Tarball mungkin korup. Ulangi transfer dari Mac: make deploy/transfer"
fi

# ------------------------------------------------------------------------------
#  2. Verify images exist
# ------------------------------------------------------------------------------
log_section "2/8 — Verifikasi image"
for svc in api web caddy; do
  img="garda-${svc}:${GIT_SHA}"
  if ! docker image inspect "$img" >/dev/null 2>&1; then
    die "Image '${img}' tidak ada setelah docker load." \
        "Tarball tidak berisi image ini. Rebuild + retransfer dari Mac: make deploy"
  fi
  log_success "Image OK: ${img}"
done

# ------------------------------------------------------------------------------
#  3. Ensure the shared network exists
# ------------------------------------------------------------------------------
log_section "3/8 — Network ${NETWORK_NAME}"
if docker network inspect "$NETWORK_NAME" >/dev/null 2>&1; then
  log_success "Network '${NETWORK_NAME}' sudah ada."
else
  log_warn "Network '${NETWORK_NAME}' belum ada — membuat baru."
  docker network create --driver bridge --attachable "$NETWORK_NAME" >/dev/null
  log_success "Network '${NETWORK_NAME}' dibuat."
fi

# ------------------------------------------------------------------------------
#  4. Bring up infrastructure
# ------------------------------------------------------------------------------
log_section "4/8 — Up infra (postgres, mongodb, redis)"
compose_infra up -d --no-build --remove-orphans

# ------------------------------------------------------------------------------
#  5. Wait for infra healthchecks
# ------------------------------------------------------------------------------
log_section "5/8 — Menunggu healthcheck infra (maks ${HEALTH_TIMEOUT_SECONDS}s)"
deadline=$(( $(date +%s) + HEALTH_TIMEOUT_SECONDS ))
healthy_count=0

# `jq` tidak dijamin ada di VM. Kalau tidak ada, fallback ke parsing grep/awk
# atas output `docker compose ps` (kolom Status memuat "(healthy)").
if command -v jq >/dev/null 2>&1; then
  count_healthy() {
    compose_infra ps --format json 2>/dev/null \
      | jq -r 'select(.Health == "healthy") | .Service' 2>/dev/null \
      | grep -c . || true
  }
else
  log_warn "jq tidak ditemukan di VM — memakai fallback grep untuk healthcheck."
  count_healthy() {
    compose_infra ps 2>/dev/null | grep -c '(healthy)' || true
  }
fi

while [[ "$(date +%s)" -lt "$deadline" ]]; do
  healthy_count="$(count_healthy)"
  [[ -z "$healthy_count" ]] && healthy_count=0

  if [[ "$healthy_count" -ge 3 ]]; then
    break
  fi
  log_debug "Infra healthy: ${healthy_count}/3 — tunggu 5s ..."
  sleep 5
done

if [[ "$healthy_count" -lt 3 ]]; then
  log_error "Hanya ${healthy_count}/3 service infra yang healthy setelah ${HEALTH_TIMEOUT_SECONDS}s."
  compose_infra ps || true
  compose_infra logs --tail=80 || true
  die "Healthcheck infra gagal." \
      "Periksa log di atas. Umumnya: port bentrok, volume korup, atau kredensial .env salah."
fi
log_success "Infra healthy (${healthy_count}/3)."

# ------------------------------------------------------------------------------
#  6. Database migrations (optional, one-shot)
# ------------------------------------------------------------------------------
log_section "6/8 — Migrasi database"
if [[ -d "db/migrations" ]] && [[ -n "$(ls -A db/migrations 2>/dev/null || true)" ]]; then
  if docker run --rm \
      --network "$NETWORK_NAME" \
      -e DATABASE_URL="postgres://${DB_USER}:${DB_PASSWORD}@postgres:5432/${DB_NAME}?sslmode=disable" \
      --entrypoint sh \
      "garda-api:${GIT_SHA}" \
      -c 'command -v goose >/dev/null 2>&1 && goose -dir /migrations postgres "$DATABASE_URL" up || echo "goose tidak tersedia di image — migrasi dilewati"'; then
    log_success "Migrasi selesai (atau dilewati dengan aman)."
  else
    log_warn "Migrasi gagal — deploy dilanjutkan. Jalankan manual bila perlu."
  fi
else
  log_info "Tidak ada db/migrations — dilewati."
fi

# ------------------------------------------------------------------------------
#  7. Bring up the application
# ------------------------------------------------------------------------------
log_section "7/8 — Up aplikasi (api, web, caddy)"
compose_app up -d --no-build --remove-orphans

# ------------------------------------------------------------------------------
#  8. HTTP healthcheck
# ------------------------------------------------------------------------------
log_section "8/8 — HTTP healthcheck"
http_ok=0
for i in $(seq 1 "$HTTP_HEALTH_RETRIES"); do
  if curl -fsS --max-time 5 http://localhost/healthz >/dev/null 2>&1; then
    http_ok=1
    break
  fi
  log_debug "Healthcheck HTTP ${i}/${HTTP_HEALTH_RETRIES} gagal — ulang dalam ${HTTP_HEALTH_DELAY}s"
  sleep "$HTTP_HEALTH_DELAY"
done

if [[ "$http_ok" -ne 1 ]]; then
  log_error "Healthcheck HTTP gagal setelah $(( HTTP_HEALTH_RETRIES * HTTP_HEALTH_DELAY ))s."
  compose_app ps || true
  compose_app logs --tail=100 || true
  die "Aplikasi tidak merespons di http://localhost/healthz." \
      "Periksa log di atas. Umumnya: env kurang, port 80 dipakai proses lain, atau image tidak cocok arsitektur."
fi
log_success "Healthcheck HTTP OK (http://localhost/healthz)."

# ------------------------------------------------------------------------------
#  Record deploy markers
# ------------------------------------------------------------------------------
if [[ -f .last-deploy ]]; then
  cp .last-deploy .previous-deploy
  log_debug "Marker .previous-deploy diperbarui."
fi
printf '%s\n' "$GIT_SHA" > .last-deploy
printf '%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > .last-deploy-time

# ------------------------------------------------------------------------------
#  Summary
# ------------------------------------------------------------------------------
log_section "Deploy selesai — ringkasan"
log_info "Infra:"
compose_infra ps || true
log_info "App:"
compose_app ps || true

log_success "Deploy ${GIT_SHA} berhasil dan sehat."
exit 0