#!/usr/bin/env bash
# ==============================================================================
#  Garda Inflasi — scripts/deploy/remote-deploy.sh
#
#  ⚠️  THIS SCRIPT RUNS ON THE VM, NOT ON THE MAC.
#  Server-Build + Clean-Server Workflow:
#    1. Builds only selected service image(s) directly on the VM from
#       `${REMOTE_DIR}/.build-src` using native BuildKit/buildx.
#    2. Immediately removes `${REMOTE_DIR}/.build-src` (including via EXIT trap)
#       so NO application source code lingers on the server.
#    3. Updates `.env.tags` selectively, runs DB migrations when `api`/`worker`
#       updated, restarts ONLY target services (`--no-build --no-deps`), and
#       verifies healthchecks.
#
#  Usage (on the VM):
#      bash scripts/deploy/remote-deploy.sh --tag=<tag> [--only=svc1,svc2] ...
# ==============================================================================

set -euo pipefail

REMOTE_DIR="${REMOTE_DIR:-$(pwd)}"
cd "$REMOTE_DIR"

BUILD_SRC_DIR="${REMOTE_DIR}/.build-src"
KEEP_SRC=0

cleanup_build_src() {
  if [[ "$KEEP_SRC" -ne 1 && -d "$BUILD_SRC_DIR" ]]; then
    rm -rf "$BUILD_SRC_DIR"
  fi
}

__garda_remote_err() {
  local exit_code=$?
  printf '\n[ERROR] remote-deploy gagal (exit=%s) di %s:%s\n' \
    "$exit_code" "${BASH_SOURCE[0]:-remote-deploy.sh}" "${1:-?}" >&2
  printf '        Perintah: %s\n' "${2:-?}" >&2
  cleanup_build_src
  exit "$exit_code"
}
trap '__garda_remote_err "$LINENO" "$BASH_COMMAND"' ERR
trap 'cleanup_build_src' EXIT

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
if [[ -f "${SCRIPT_DIR}/../lib/common.sh" ]]; then
  # shellcheck source=../lib/common.sh
  source "${SCRIPT_DIR}/../lib/common.sh"
elif [[ -f "${REMOTE_DIR}/scripts/lib/common.sh" ]]; then
  # shellcheck source=/dev/null
  source "${REMOTE_DIR}/scripts/lib/common.sh"
else
  log_info()    { printf '[INFO ] %s\n' "$*" >&2; }
  log_warn()    { printf '[WARN ] %s\n' "$*" >&2; }
  log_error()   { printf '[ERROR] %s\n' "$*" >&2; }
  log_success() { printf '[ OK  ] %s\n' "$*" >&2; }
  log_section() { printf '\n── %s ──\n' "$*" >&2; }
  log_debug()   { :; }
  die() { log_error "${1:-Unknown error}"; [[ -n "${2:-}" ]] && printf '        ↳ Solusi: %s\n' "$2" >&2; exit 1; }
  human_size() { awk -v b="${1:-0}" 'BEGIN { split("B KB MB GB TB", u, " "); i=1; while (b>=1024 && i<5) { b/=1024; i++ } printf "%.2f %s", b, u[i] }'; }
fi

# ------------------------------------------------------------------------------
#  Argument parsing
# ------------------------------------------------------------------------------
RELEASE_TAG=""
ONLY_RAW=""
GATEWAY_MODE="${GATEWAY_MODE:-external}"
GATEWAY_NETWORK="${GATEWAY_NETWORK:-caddy-gardainflasi_edge}"
PUBLIC_HOST="${PUBLIC_HOST:-app.gardainflasi.id}"
NEXT_PUBLIC_API_BASE_URL="${NEXT_PUBLIC_API_BASE_URL:-/api}"
NEXT_PUBLIC_APP_URL="${NEXT_PUBLIC_APP_URL:-https://${PUBLIC_HOST}}"
NO_CACHE=0
SKIP_BUILD=0
SKIP_MIGRATE=0
WITH_MIGRATE=0
WITH_PIPELINE="${ENABLE_PIPELINE:-1}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --tag=*)                     RELEASE_TAG="${1#*=}" ;;
    --tag)                       shift; RELEASE_TAG="${1:-}" ;;
    --only=*)                    ONLY_RAW="${1#*=}" ;;
    --only)                      shift; ONLY_RAW="${1:-}" ;;
    --with-pipeline)             WITH_PIPELINE=1 ;;
    --without-pipeline)          WITH_PIPELINE=0 ;;
    --gateway=*)                 GATEWAY_MODE="${1#*=}" ;;
    --gateway)                   shift; GATEWAY_MODE="${1:-}" ;;
    --public-host=*)             PUBLIC_HOST="${1#*=}" ;;
    --public-host)               shift; PUBLIC_HOST="${1:-}" ;;
    --next-public-api-base=*)    NEXT_PUBLIC_API_BASE_URL="${1#*=}" ;;
    --next-public-app-url=*)     NEXT_PUBLIC_APP_URL="${1#*=}" ;;
    --no-cache)                  NO_CACHE=1 ;;
    --skip-build)                SKIP_BUILD=1 ;;
    --keep-src)                  KEEP_SRC=1 ;;
    --skip-migrate)              SKIP_MIGRATE=1 ;;
    --with-migrate)              WITH_MIGRATE=1 ;;
    *)
      if [[ -z "$RELEASE_TAG" && "$1" != -* ]]; then
        RELEASE_TAG="$1"
      else
        die "Opsi tidak dikenal di remote-deploy: $1"
      fi
      ;;
  esac
  shift
done

if [[ -z "$RELEASE_TAG" ]]; then
  die "RELEASE_TAG wajib diberikan (--tag=<tag>)."
fi

# ------------------------------------------------------------------------------
#  Configuration & Service Selection
# ------------------------------------------------------------------------------
COMPOSE_INFRA="compose/compose.infra.yaml"
COMPOSE_APP="compose/compose.yaml"
COMPOSE_PROD="compose/compose.prod.yaml"
COMPOSE_GATEWAY="compose/compose.gateway.yaml"
NETWORK_NAME="garda_inflasi_net"
INFRA_SERVICE_COUNT=2
HEALTH_TIMEOUT_SECONDS="${HEALTH_TIMEOUT_SECONDS:-60}"
HTTP_HEALTH_RETRIES="${HTTP_HEALTH_RETRIES:-15}"
HTTP_HEALTH_DELAY="${HTTP_HEALTH_DELAY:-3}"

DEPLOY_SERVICES=()
if [[ -n "$ONLY_RAW" ]]; then
  IFS=',' read -r -a DEPLOY_SERVICES <<< "$ONLY_RAW"
else
  if [[ "$GATEWAY_MODE" == "builtin" ]]; then
    DEPLOY_SERVICES=(api worker ml web caddy)
  else
    DEPLOY_SERVICES=(api worker ml web)
  fi
  if [[ "$WITH_PIPELINE" -eq 1 ]]; then
    DEPLOY_SERVICES+=(crawler ml-worker)
  fi
fi

NEED_API_IMG=0
NEED_WEB_IMG=0
NEED_ML_IMG=0
NEED_ML_WORKER_IMG=0
NEED_CRAWLER_IMG=0
NEED_CADDY_IMG=0
NEED_PIPELINE=0
RUN_MIGRATE=0

for svc in "${DEPLOY_SERVICES[@]}"; do
  case "$svc" in
    api|worker) NEED_API_IMG=1; RUN_MIGRATE=1 ;;
    web)        NEED_WEB_IMG=1 ;;
    ml)         NEED_ML_IMG=1 ;;
    ml-worker)  NEED_ML_WORKER_IMG=1; NEED_PIPELINE=1 ;;
    crawler)    NEED_CRAWLER_IMG=1; NEED_PIPELINE=1 ;;
    caddy)
      if [[ "$GATEWAY_MODE" == "external" ]]; then
        die "Service 'caddy' tidak bisa dideploy saat GATEWAY_MODE=external."
      fi
      NEED_CADDY_IMG=1
      ;;
    *) die "Service tidak dikenal di remote-deploy: '$svc'" ;;
  esac
done

[[ "$SKIP_MIGRATE" -eq 1 ]] && RUN_MIGRATE=0
[[ "$WITH_MIGRATE" -eq 1 ]] && RUN_MIGRATE=1

# ------------------------------------------------------------------------------
#  Pre-flight
# ------------------------------------------------------------------------------
log_section "Remote deploy (Server-Build) — Garda Inflasi"
log_info "RELEASE_TAG  : ${RELEASE_TAG}"
log_info "REMOTE_DIR   : ${REMOTE_DIR}"
log_info "Services     : ${DEPLOY_SERVICES[*]}"
log_info "Gateway mode : ${GATEWAY_MODE}"

for f in "$COMPOSE_INFRA" "$COMPOSE_APP" "$COMPOSE_PROD"; do
  [[ -f "$f" ]] || die "File compose wajib tidak ditemukan: ${REMOTE_DIR}/${f}"
done

if [[ "$GATEWAY_MODE" == "external" && ! -f "$COMPOSE_GATEWAY" ]]; then
  die "File gateway tidak ditemukan: ${REMOTE_DIR}/${COMPOSE_GATEWAY}"
fi

if [[ ! -f ./.env ]]; then
  die "File .env tidak ditemukan di ${REMOTE_DIR}."
fi

set -a
# shellcheck source=/dev/null
source ./.env
set +a

require_secret() {
  local name="$1" value="${!1:-}" min_len="${2:-16}"
  [[ -n "$value" ]] || die "${name} kosong di ${REMOTE_DIR}/.env."
  case "$value" in
    *GANTI_DENGAN*|*garda_secret*|*change-in-production*|*dev-secret*|0123456789abcdef*)
      die "${name} masih berisi nilai contoh/bawaan." ;;
  esac
  [[ "${#value}" -ge "$min_len" ]] || die "${name} terlalu pendek (minimal ${min_len} karakter)."
}
require_secret POSTGRES_PASSWORD 16
require_secret JWT_SECRET 32
require_secret AES_KEY 32
[[ "${#AES_KEY}" -eq 32 ]] || die "AES_KEY harus tepat 32 karakter."
[[ -n "${APP_ALLOWED_ORIGINS:-}" ]] || die "APP_ALLOWED_ORIGINS kosong di .env."
if [[ -n "${DB_APP_USER:-}" ]]; then
  require_secret DB_APP_PASSWORD 16
  [[ "${DB_APP_USER}" != "${DB_USER:-${POSTGRES_USER:-garda}}" ]] || die "DB_APP_USER tidak boleh sama dengan pemilik skema."
fi

export POSTGRES_DB="${POSTGRES_DB:-garda_inflasi}"
export POSTGRES_USER="${POSTGRES_USER:-garda}"
export DB_USER="${DB_USER:-$POSTGRES_USER}"
export DB_PASSWORD="${DB_PASSWORD:-$POSTGRES_PASSWORD}"
export DB_NAME="${DB_NAME:-$POSTGRES_DB}"
export DB_APP_USER="${DB_APP_USER:-}"
export DB_APP_PASSWORD="${DB_APP_PASSWORD:-}"
export APP_ENV=production
export MONGO_VOLUME="${MONGO_VOLUME:-garda_pipeline_mongo_data}"
export MONGO_USER="${MONGO_USER:-garda}"
export MONGO_DB="${MONGO_DB:-garda_inflasi}"
export MONGO_DB_UNSEEN="${MONGO_DB_UNSEEN:-dataunseen}"
export MONGO_DB_SAFE="${MONGO_DB_SAFE:-datasafe}"
if [[ -n "${MONGO_PASSWORD:-}" && -z "${MONGO_URI:-}" ]]; then
  export MONGO_URI="mongodb://${MONGO_USER}:${MONGO_PASSWORD}@garda-mongodb:27017/?authSource=admin"
fi

if [[ "$NEED_PIPELINE" -eq 1 ]]; then
  if [[ -z "${MONGO_PASSWORD:-}" ]]; then
    log_warn "MONGO_PASSWORD belum ada di ${REMOTE_DIR}/.env — menambahkan kredensial MongoDB acak ..."
    MONGO_PASSWORD="$(openssl rand -hex 24)"
    {
      printf '\n# MongoDB & Pipeline Data/ML (ditambahkan otomatis oleh remote-deploy.sh)\n'
      printf 'MONGO_USER=%s\n' "$MONGO_USER"
      printf 'MONGO_PASSWORD=%s\n' "$MONGO_PASSWORD"
      printf 'MONGO_DB=%s\n' "$MONGO_DB"
      printf 'MONGO_PORT=%s\n' "${MONGO_PORT:-27017}"
      printf 'MONGO_VOLUME=%s\n' "$MONGO_VOLUME"
      printf 'MONGO_DB_UNSEEN=%s\n' "$MONGO_DB_UNSEEN"
      printf 'MONGO_DB_SAFE=%s\n' "$MONGO_DB_SAFE"
      printf 'ML_N_JOBS=%s\n' "${ML_N_JOBS:-4}"
      printf 'ML_INTERVAL_SELEKSI_JAM=%s\n' "${ML_INTERVAL_SELEKSI_JAM:-24}"
    } >> ./.env
  fi
  require_secret MONGO_PASSWORD 16
  export MONGO_PASSWORD
  export MONGO_URI="${MONGO_URI:-mongodb://${MONGO_USER}:${MONGO_PASSWORD}@garda-mongodb:27017/?authSource=admin}"
  export ML_N_JOBS="${ML_N_JOBS:-4}"
  export ML_INTERVAL_SELEKSI_JAM="${ML_INTERVAL_SELEKSI_JAM:-24}"
fi

if [[ -f ./.env.tags ]]; then
  cp ./.env.tags ./.env.tags.prev
  set -a
  # shellcheck source=/dev/null
  source ./.env.tags
  set +a
else
  cat > ./.env.tags <<EOF
IMAGE_TAG=latest
API_IMAGE_TAG=latest
WEB_IMAGE_TAG=latest
ML_IMAGE_TAG=latest
ML_PIPELINE_IMAGE_TAG=latest
CRAWLER_IMAGE_TAG=latest
CADDY_IMAGE_TAG=latest
EOF
  cp ./.env.tags ./.env.tags.prev
fi

export IMAGE_TAG="${RELEASE_TAG}"
export API_IMAGE_TAG="${API_IMAGE_TAG:-latest}"
export WEB_IMAGE_TAG="${WEB_IMAGE_TAG:-latest}"
export ML_IMAGE_TAG="${ML_IMAGE_TAG:-latest}"
export ML_PIPELINE_IMAGE_TAG="${ML_PIPELINE_IMAGE_TAG:-latest}"
export CRAWLER_IMAGE_TAG="${CRAWLER_IMAGE_TAG:-latest}"
export CADDY_IMAGE_TAG="${CADDY_IMAGE_TAG:-latest}"

compose_infra() {
  docker compose --project-directory "$REMOTE_DIR" -f "$COMPOSE_INFRA" -f "$COMPOSE_PROD" "$@"
}

compose_app() {
  if [[ "$GATEWAY_MODE" == "external" ]]; then
    docker compose --project-directory "$REMOTE_DIR" -f "$COMPOSE_APP" -f "$COMPOSE_PROD" -f "$COMPOSE_GATEWAY" "$@"
  else
    docker compose --project-directory "$REMOTE_DIR" -f "$COMPOSE_APP" -f "$COMPOSE_PROD" "$@"
  fi
}

# Pre-flight validasi konfigurasi compose (infra + app + profil pipeline) sebelum build & up.
log_info "Validasi konfigurasi Docker Compose produksi (infra + app --profile pipeline) ..."
compose_infra config --quiet || die "Validasi ${COMPOSE_INFRA} + ${COMPOSE_PROD} gagal."
compose_app --profile pipeline config --quiet || die "Validasi ${COMPOSE_APP} + ${COMPOSE_PROD} (--profile pipeline) gagal."
log_success "Konfigurasi Docker Compose valid."

# ------------------------------------------------------------------------------
#  1. Build selected image(s) directly on the VM (sequentially for 3.6GB RAM safety)
# ------------------------------------------------------------------------------
build_vm_image() {
  local target="$1"
  local dockerfile_name="$target"
  local image_suffix="$target"
  local docker_target=""

  if [[ "$target" == "ml-worker" ]]; then
    dockerfile_name="ml"
    image_suffix="ml-pipeline"
    docker_target="pipeline"
  fi

  local dockerfile="${BUILD_SRC_DIR}/docker/${dockerfile_name}.Dockerfile"
  local image="garda-${image_suffix}"

  [[ -f "$dockerfile" ]] || die "Dockerfile tidak ditemukan di ${dockerfile}."

  local args=(
    buildx build
    --load
    -t "${image}:${RELEASE_TAG}"
    -t "${image}:latest"
    -f "$dockerfile"
  )
  [[ -n "$docker_target" ]] && args+=(--target "$docker_target")
  [[ "$NO_CACHE" -eq 1 ]] && args+=(--no-cache)

  if [[ "$target" == "web" ]]; then
    if [[ "${PUBLIC_HOST:-}" != "localhost"* ]]; then
      case "${NEXT_PUBLIC_API_BASE_URL:-}" in
        *localhost*|*127.0.0.1*|http://*)
          die "NEXT_PUBLIC_API_BASE_URL tidak valid untuk produksi: '${NEXT_PUBLIC_API_BASE_URL}'." \
              "Harus relatif '/api' di .env.deploy agar browser memanggil origin HTTPS yang sama." ;;
      esac
      case "${NEXT_PUBLIC_APP_URL:-}" in
        *localhost*|*127.0.0.1*)
          die "NEXT_PUBLIC_APP_URL tidak valid untuk produksi: '${NEXT_PUBLIC_APP_URL}'." \
              "Isi origin publik HTTPS di .env.deploy, mis. https://${PUBLIC_HOST}." ;;
      esac
    fi
    args+=(
      --build-arg "NEXT_PUBLIC_API_BASE_URL=${NEXT_PUBLIC_API_BASE_URL}"
      --build-arg "NEXT_PUBLIC_APP_URL=${NEXT_PUBLIC_APP_URL}"
      --build-arg "NODE_OPTIONS=--max-old-space-size=2048"
    )
  fi

  args+=("$BUILD_SRC_DIR")

  log_info "Building ${image}:${RELEASE_TAG} langsung di server ..."
  local start_ts
  start_ts="$(date +%s)"
  DOCKER_BUILDKIT=1 docker "${args[@]}"
  local elapsed=$(( $(date +%s) - start_ts ))
  log_success "Image ${image}:${RELEASE_TAG} selesai dibangun dalam ${elapsed}s."
}

if [[ "$SKIP_BUILD" -eq 1 ]]; then
  log_section "1/8 — Build dilewati (--skip-build)"
else
  log_section "1/8 — Build image langsung di server (${BUILD_SRC_DIR})"
  [[ -d "$BUILD_SRC_DIR" ]] || die "Direktori staging source tidak ditemukan: ${BUILD_SRC_DIR}" \
      "Jalankan sync-source.sh terlebih dahulu."

  [[ "$NEED_API_IMG" -eq 1 ]]       && build_vm_image "api"
  [[ "$NEED_ML_IMG" -eq 1 ]]        && build_vm_image "ml"
  [[ "$NEED_ML_WORKER_IMG" -eq 1 ]] && build_vm_image "ml-worker"
  [[ "$NEED_CRAWLER_IMG" -eq 1 ]]   && build_vm_image "crawler"
  [[ "$NEED_WEB_IMG" -eq 1 ]]       && build_vm_image "web"
  [[ "$NEED_CADDY_IMG" -eq 1 ]]     && build_vm_image "caddy"
fi

# ------------------------------------------------------------------------------
#  2. Remove staging source code immediately so the server remains clean
# ------------------------------------------------------------------------------
log_section "2/8 — Bersihkan source code di server"
if [[ "$KEEP_SRC" -eq 1 ]]; then
  log_warn "--keep-src aktif: ${BUILD_SRC_DIR} dipertahankan."
else
  rm -rf "$BUILD_SRC_DIR"
  docker image prune -f >/dev/null 2>&1 || true
  log_success "Source code build (${BUILD_SRC_DIR}) dan dangling layers telah dihapus bersih."
fi

# ------------------------------------------------------------------------------
#  3. Verify built images & update .env.tags selectively
# ------------------------------------------------------------------------------
log_section "3/8 — Verifikasi image target"
check_image() {
  local svc="$1"
  local img="garda-${svc}:${RELEASE_TAG}"
  if ! docker image inspect "$img" >/dev/null 2>&1; then
    die "Image '${img}' tidak ditemukan di server."
  fi
  log_success "Image OK: ${img}"
}

if [[ "$NEED_API_IMG" -eq 1 ]]; then
  check_image api
  export API_IMAGE_TAG="$RELEASE_TAG"
fi
if [[ "$NEED_WEB_IMG" -eq 1 ]]; then
  check_image web
  export WEB_IMAGE_TAG="$RELEASE_TAG"
fi
if [[ "$NEED_ML_IMG" -eq 1 ]]; then
  check_image ml
  export ML_IMAGE_TAG="$RELEASE_TAG"
fi
if [[ "$NEED_ML_WORKER_IMG" -eq 1 ]]; then
  check_image ml-pipeline
  export ML_PIPELINE_IMAGE_TAG="$RELEASE_TAG"
fi
if [[ "$NEED_CRAWLER_IMG" -eq 1 ]]; then
  check_image crawler
  export CRAWLER_IMAGE_TAG="$RELEASE_TAG"
fi
if [[ "$NEED_CADDY_IMG" -eq 1 ]]; then
  check_image caddy
  export CADDY_IMAGE_TAG="$RELEASE_TAG"
fi

cat > ./.env.tags <<EOF
IMAGE_TAG=${RELEASE_TAG}
API_IMAGE_TAG=${API_IMAGE_TAG}
WEB_IMAGE_TAG=${WEB_IMAGE_TAG}
ML_IMAGE_TAG=${ML_IMAGE_TAG}
ML_PIPELINE_IMAGE_TAG=${ML_PIPELINE_IMAGE_TAG}
CRAWLER_IMAGE_TAG=${CRAWLER_IMAGE_TAG}
CADDY_IMAGE_TAG=${CADDY_IMAGE_TAG}
EOF
log_info "Tag aktif: API=${API_IMAGE_TAG}, WEB=${WEB_IMAGE_TAG}, ML=${ML_IMAGE_TAG}, ML_PIPELINE=${ML_PIPELINE_IMAGE_TAG}, CRAWLER=${CRAWLER_IMAGE_TAG}"
compose_app --profile pipeline config --quiet || die "Validasi konfigurasi compose dengan .env.tags baru gagal."

# ------------------------------------------------------------------------------
#  4. Ensure networks exist
# ------------------------------------------------------------------------------
log_section "4/8 — Verifikasi Docker networks"
if docker network inspect "$NETWORK_NAME" >/dev/null 2>&1; then
  log_success "Network internal '${NETWORK_NAME}' siap."
else
  docker network create --driver bridge --attachable "$NETWORK_NAME" >/dev/null
  log_success "Network internal '${NETWORK_NAME}' dibuat."
fi

if [[ "$GATEWAY_MODE" == "external" ]]; then
  if ! docker network inspect "$GATEWAY_NETWORK" >/dev/null 2>&1; then
    die "Network gateway eksternal '${GATEWAY_NETWORK}' tidak ditemukan."
  fi
  log_success "Network gateway eksternal '${GATEWAY_NETWORK}' siap."
fi

# ------------------------------------------------------------------------------
#  5. Ensure infrastructure (postgres, redis, dan mongodb bila pipeline aktif)
# ------------------------------------------------------------------------------
infra_services=(postgres redis)
[[ "$NEED_PIPELINE" -eq 1 ]] && { infra_services+=(mongodb); INFRA_SERVICE_COUNT=$((INFRA_SERVICE_COUNT + 1)); }
case ",${COMPOSE_PROFILES:-}," in *,backup,*) infra_services+=(backup) ;; esac
log_section "5/8 — Pastikan infra sehat (${infra_services[*]})"
compose_infra up -d --no-build "${infra_services[@]}"

deadline=$(( $(date +%s) + HEALTH_TIMEOUT_SECONDS ))
healthy_count=0
count_healthy() {
  compose_infra ps 2>/dev/null | grep -c '(healthy)' || true
}

while [[ "$(date +%s)" -lt "$deadline" ]]; do
  healthy_count="$(count_healthy)"
  [[ -z "$healthy_count" ]] && healthy_count=0
  [[ "$healthy_count" -ge "$INFRA_SERVICE_COUNT" ]] && break
  sleep 3
done

if [[ "$healthy_count" -lt "$INFRA_SERVICE_COUNT" ]]; then
  compose_infra ps || true
  compose_infra logs --tail=60 || true
  die "Infra belum sehat (${healthy_count}/${INFRA_SERVICE_COUNT}) setelah ${HEALTH_TIMEOUT_SECONDS}s."
fi
log_success "Infra healthy (${healthy_count}/${INFRA_SERVICE_COUNT})."

# ------------------------------------------------------------------------------
#  6. Database migrations + reference/admin seed (when api/worker updated)
# ------------------------------------------------------------------------------
if [[ "$RUN_MIGRATE" -eq 1 ]]; then
  log_section "6/8 — Migrasi database & seed referensi/admin"
  compose_app run --rm --no-deps -T migrate
  log_success "Migrasi selesai."
  compose_app run --rm --no-deps -T seed
  log_success "Seed referensi/admin selesai."
else
  log_section "6/8 — Migrasi dilewati (service non-API: ${DEPLOY_SERVICES[*]})"
fi

# ------------------------------------------------------------------------------
#  7. Bring up only target services (--no-deps preserves unaffected containers)
# ------------------------------------------------------------------------------
log_section "7/8 — Up layanan: ${DEPLOY_SERVICES[*]}"
if [[ "$NEED_PIPELINE" -eq 1 ]]; then
  compose_app --profile pipeline up -d --no-build --no-deps "${DEPLOY_SERVICES[@]}"
else
  compose_app up -d --no-build --no-deps "${DEPLOY_SERVICES[@]}"
fi

# ------------------------------------------------------------------------------
#  8. Targeted service healthchecks
# ------------------------------------------------------------------------------
log_section "8/8 — Healthcheck layanan ter-deploy"

check_container_cmd() {
  local container="$1" desc="$2" cmd="$3"
  local i
  for i in $(seq 1 "$HTTP_HEALTH_RETRIES"); do
    if docker exec "$container" sh -c "$cmd" >/dev/null 2>&1; then
      log_success "${container}: ${desc} OK"
      return 0
    fi
    sleep "$HTTP_HEALTH_DELAY"
  done
  log_error "${container}: ${desc} gagal setelah $((HTTP_HEALTH_RETRIES * HTTP_HEALTH_DELAY))s."
  docker logs --tail=80 "$container" || true
  return 1
}

for svc in "${DEPLOY_SERVICES[@]}"; do
  case "$svc" in
    api)
      check_container_cmd "garda-api" "/healthz" "wget -qO- http://127.0.0.1:8080/healthz"
      ;;
    worker)
      check_container_cmd "garda-worker" ":8081/healthz" "wget -qO- http://127.0.0.1:8081/healthz"
      ;;
    ml)
      check_container_cmd "garda-ml" ":8000/health" "python -c \"import urllib.request; urllib.request.urlopen('http://127.0.0.1:8000/health', timeout=3)\""
      ;;
    ml-worker)
      check_container_cmd "garda-ml-worker" "venv pipeline ML (lightgbm, pymongo)" "python -c \"import garda_ml.pipeline, lightgbm, pymongo\""
      ;;
    crawler)
      check_container_cmd "garda-crawler" "detak crawler & venv Python" "python -m garda_crawler health"
      ;;
    web)
      check_container_cmd "garda-web" ":3000/" "wget -qO- http://127.0.0.1:3000/"
      ;;
    caddy)
      local_url="http://localhost/healthz"
      for i in $(seq 1 "$HTTP_HEALTH_RETRIES"); do
        if curl -fsS --max-time 5 "$local_url" >/dev/null 2>&1; then
          log_success "garda-caddy: ${local_url} OK"
          break
        fi
        [[ "$i" -eq "$HTTP_HEALTH_RETRIES" ]] && die "garda-caddy gagal merespons."
        sleep "$HTTP_HEALTH_DELAY"
      done
      ;;
  esac
done

# ------------------------------------------------------------------------------
#  Record deploy markers
# ------------------------------------------------------------------------------
if [[ -f .last-deploy ]]; then
  cp .last-deploy .previous-deploy
fi
printf '%s\n' "$RELEASE_TAG" > .last-deploy
printf '%s\n' "${DEPLOY_SERVICES[*]}" > .last-deploy-services
printf '%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > .last-deploy-time

log_section "Ringkasan container VM"
compose_infra ps || true
if [[ "$NEED_PIPELINE" -eq 1 ]] || docker ps -a --format '{{.Names}}' | grep -Eq '^(garda-crawler|garda-ml-worker)$'; then
  compose_app --profile pipeline ps || true
else
  compose_app ps || true
fi
log_success "Remote deploy ${RELEASE_TAG} (${DEPLOY_SERVICES[*]}) berhasil dan bersih."
