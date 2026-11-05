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
INFRA_SERVICE_COUNT=2   # postgres, redis
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

# ------------------------------------------------------------------------------
#  Rahasia produksi: tolak deploy bila kosong atau masih nilai contoh/bawaan.
#  (API juga menolak start di production; pemeriksaan di sini menggagalkan lebih
#  awal, sebelum image dimuat dan migrasi berjalan.)
# ------------------------------------------------------------------------------
require_secret() {
  local name="$1" value="${!1:-}" min_len="${2:-16}"
  [[ -n "$value" ]] || die "${name} kosong di ${REMOTE_DIR}/.env." \
      "Isi dengan nilai acak kuat: openssl rand -base64 32"
  case "$value" in
    *GANTI_DENGAN*|*garda_secret*|*change-in-production*|*dev-secret*|0123456789abcdef*)
      die "${name} masih berisi nilai contoh/bawaan." "Ganti dengan nilai acak: openssl rand -base64 32" ;;
  esac
  [[ "${#value}" -ge "$min_len" ]] || die "${name} terlalu pendek (minimal ${min_len} karakter)." \
      "Gunakan: openssl rand -base64 32"
}
require_secret POSTGRES_PASSWORD 16
require_secret JWT_SECRET 32
require_secret AES_KEY 32
[[ "${#AES_KEY}" -eq 32 ]] || die "AES_KEY harus tepat 32 karakter." "Gunakan: openssl rand -hex 16"
[[ -n "${APP_ALLOWED_ORIGINS:-}" ]] || die "APP_ALLOWED_ORIGINS kosong di .env." \
    "Isi dengan origin publik aplikasi, mis. https://app.garda.id"

# Compose needs the same variables the app containers use.
export POSTGRES_DB="${POSTGRES_DB:-garda_inflasi}"
export POSTGRES_USER="${POSTGRES_USER:-garda}"
export DB_USER="${DB_USER:-$POSTGRES_USER}"
export DB_PASSWORD="${DB_PASSWORD:-$POSTGRES_PASSWORD}"
export DB_NAME="${DB_NAME:-$POSTGRES_DB}"
export APP_ENV=production

# ------------------------------------------------------------------------------
#  Compose helper — always --no-build (the VM must never build)
# ------------------------------------------------------------------------------
compose_infra() {
  docker compose --project-directory "$REMOTE_DIR" -f "$COMPOSE_INFRA" -f "$COMPOSE_PROD" "$@"
}
compose_app() {
  docker compose --project-directory "$REMOTE_DIR" -f "$COMPOSE_APP" -f "$COMPOSE_PROD" "$@"
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
for svc in api web caddy ml; do
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
log_section "4/8 — Up infra (postgres, redis)"
# compose.prod.yaml dipakai bersama oleh stack infra dan app, jadi file itu memuat stub
# service milik stack lain. SELALU sebut nama service agar tidak ikut menaikkan stub tersebut.
infra_services=(postgres redis)
case ",${COMPOSE_PROFILES:-}," in *,backup,*) infra_services+=(backup) ;; esac
compose_infra up -d --no-build --remove-orphans "${infra_services[@]}"

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

  if [[ "$healthy_count" -ge "$INFRA_SERVICE_COUNT" ]]; then
    break
  fi
  log_debug "Infra healthy: ${healthy_count}/${INFRA_SERVICE_COUNT} — tunggu 5s ..."
  sleep 5
done

if [[ "$healthy_count" -lt "$INFRA_SERVICE_COUNT" ]]; then
  log_error "Hanya ${healthy_count}/${INFRA_SERVICE_COUNT} service infra yang healthy setelah ${HEALTH_TIMEOUT_SECONDS}s."
  compose_infra ps || true
  compose_infra logs --tail=80 || true
  die "Healthcheck infra gagal." \
      "Periksa log di atas. Umumnya: port bentrok, volume korup, atau kredensial .env salah."
fi
log_success "Infra healthy (${healthy_count}/${INFRA_SERVICE_COUNT})."

# ------------------------------------------------------------------------------
#  6. Database migrations + reference seed (hard failure)
#
#  Migrasi dan seed berjalan dari image API yang sama dengan aplikasi (service
#  `migrate` dan `seed` di compose.yaml). Kegagalan menghentikan deploy
#  (set -e + ERR trap) sehingga aplikasi tidak pernah naik di atas skema rusak.
#  Produksi hanya mengisi lapis reference dan admin, tidak pernah demo
#  (lihat compose.prod.yaml).
# ------------------------------------------------------------------------------
log_section "6/8 — Migrasi database & seed referensi"
compose_app run --rm --no-deps -T migrate
log_success "Migrasi selesai (versi skema tercetak di atas)."
compose_app run --rm --no-deps -T seed
log_success "Seed referensi/admin selesai."

# ------------------------------------------------------------------------------
#  7. Bring up the application
# ------------------------------------------------------------------------------
log_section "7/8 — Up aplikasi (api, worker, ml, web, caddy)"
compose_app up -d --no-build --no-deps --remove-orphans api worker ml web caddy

# ------------------------------------------------------------------------------
#  8. HTTP healthcheck
# ------------------------------------------------------------------------------
log_section "8/8 — HTTP healthcheck"
http_ok=0
# Bila CADDY_SITE_ADDRESS berisi domain, Caddy hanya melayani host itu (HTTP dialihkan
# ke HTTPS), jadi probe lewat HTTPS dengan --resolve ke loopback. Sertifikat dari Let's
# Encrypt tidak valid untuk "localhost", maka host asli dipakai pada SNI/Host.
health_url="http://localhost/healthz"
health_curl=(curl -fsS --max-time 5)
case "${CADDY_SITE_ADDRESS:-}" in
  ""|localhost*) ;;
  *)
    health_host="${CADDY_SITE_ADDRESS%%[ ,]*}"
    health_url="https://${health_host}/healthz"
    health_curl+=(--resolve "${health_host}:443:127.0.0.1")
    ;;
esac
for i in $(seq 1 "$HTTP_HEALTH_RETRIES"); do
  if "${health_curl[@]}" "$health_url" >/dev/null 2>&1; then
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
  die "Aplikasi tidak merespons di ${health_url}." \
      "Periksa log di atas. Umumnya: env kurang, port 80 dipakai proses lain, atau image tidak cocok arsitektur."
fi
log_success "Healthcheck HTTP OK (${health_url})."

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