#!/usr/bin/env bash
# ==============================================================================
#  Garda Inflasi — scripts/deploy/build-local.sh
#  Builds application images on the LOCAL machine (MacBook) with docker buildx.
#  Zero-Git workflow: uses timestamped RELEASE_TAG (or --tag=<tag>).
#
#  Images produced:
#      garda-api:<tag>          (shared by api, worker, migrate, seed)
#      garda-web:<tag>
#      garda-ml:<tag>
#      garda-ml-pipeline:<tag>  (target=pipeline dari ml.Dockerfile; venv di container)
#      garda-crawler:<tag>      (venv di container dari crawler.Dockerfile)
#      garda-caddy:<tag>        (only if GATEWAY_MODE=builtin or --only=caddy)
#
#  Usage:
#      bash scripts/deploy/build-local.sh [options]
#
#  Options:
#      --only=api,web,ml,ml-worker,crawler,worker   Build only the listed services
#      --with-pipeline                              Include crawler & ml-worker (default)
#      --without-pipeline                           Exclude crawler & ml-worker
#      --tag=<tag>                                  Explicit release tag (default: vYYYYMMDD-HHMMSS)
#      --no-cache                 Disable buildx cache (slow, but clean)
#      --push-registry            Also push to a registry (off by default)
#      --dry-run                  Print commands without executing
#      -h, --help                 Show this help
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
ONLY_RAW=""
WITH_PIPELINE=""
TAG_OVERRIDE=""
NO_CACHE=0
PUSH_REGISTRY=0
DRY_RUN="${DRY_RUN:-0}"

usage() {
  sed -n '2,25p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --only=*)        ONLY_RAW="${1#*=}" ;;
    --only)          shift; ONLY_RAW="${1:-}" ;;
    --with-pipeline)    WITH_PIPELINE=1 ;;
    --without-pipeline) WITH_PIPELINE=0 ;;
    --tag=*)         TAG_OVERRIDE="${1#*=}" ;;
    --tag)           shift; TAG_OVERRIDE="${1:-}" ;;
    --sha=*)         TAG_OVERRIDE="${1#*=}" ;;
    --sha)           shift; TAG_OVERRIDE="${1:-}" ;;
    --no-cache)      NO_CACHE=1 ;;
    --push-registry) PUSH_REGISTRY=1 ;;
    --allow-dirty)   ;; # kept as no-op for backward CLI compatibility
    --dry-run)       DRY_RUN=1 ;;
    -h|--help)       usage; exit 0 ;;
    *)
      die "Opsi tidak dikenal: $1" "Jalankan 'bash scripts/deploy/build-local.sh --help' untuk daftar opsi."
      ;;
  esac
  shift
done

export DRY_RUN

load_env_deploy
WITH_PIPELINE="${WITH_PIPELINE:-${ENABLE_PIPELINE:-1}}"

require_cmd docker "Install Docker Desktop: brew install --cask docker"

if [[ "$DRY_RUN" != "1" ]]; then
  if ! docker buildx version >/dev/null 2>&1; then
    die "docker buildx tidak tersedia." \
        "Docker Desktop modern sudah menyertakan buildx. Cek: docker buildx version"
  fi

  if ! docker info >/dev/null 2>&1; then
    die "Docker daemon tidak berjalan." "Jalankan Docker Desktop lalu ulangi."
  fi
fi

cd "$GARDA_ROOT"

# ------------------------------------------------------------------------------
#  Resolve target services and deduplicated image build targets
# ------------------------------------------------------------------------------
REQUESTED_SERVICES=()
if [[ -n "$ONLY_RAW" ]]; then
  IFS=',' read -r -a REQUESTED_SERVICES <<< "$ONLY_RAW"
else
  if [[ "${GATEWAY_MODE:-external}" == "builtin" ]]; then
    REQUESTED_SERVICES=(api worker ml web caddy)
  else
    REQUESTED_SERVICES=(api worker ml web)
  fi
  if [[ "$WITH_PIPELINE" -eq 1 ]]; then
    REQUESTED_SERVICES+=(crawler ml-worker)
  fi
fi

# Map compose services -> dockerfile image names (api & worker share garda-api)
BUILD_IMAGES=()
add_build_image() {
  local candidate="$1"
  local existing
  for existing in "${BUILD_IMAGES[@]:-}"; do
    [[ "$existing" == "$candidate" ]] && return 0
  done
  BUILD_IMAGES+=("$candidate")
}

for svc in "${REQUESTED_SERVICES[@]}"; do
  case "$svc" in
    api|worker) add_build_image "api" ;;
    web)        add_build_image "web" ;;
    ml)         add_build_image "ml" ;;
    ml-worker)  add_build_image "ml-pipeline" ;;
    crawler)    add_build_image "crawler" ;;
    caddy)      add_build_image "caddy" ;;
    *)
      die "Service tidak dikenal: '$svc'" "Pilih dari: api, worker, web, ml, ml-worker, crawler, caddy (contoh: --only=web)."
      ;;
  esac
done

# ------------------------------------------------------------------------------
#  Resolve RELEASE_TAG (no Git!)
# ------------------------------------------------------------------------------
if [[ -n "$TAG_OVERRIDE" ]]; then
  RELEASE_TAG="$TAG_OVERRIDE"
elif [[ -n "${RELEASE_TAG:-}" ]]; then
  RELEASE_TAG="${RELEASE_TAG}"
else
  RELEASE_TAG="v$(date -u +%Y%m%d-%H%M%S)"
fi
export RELEASE_TAG

# ------------------------------------------------------------------------------
#  Buildx cache directory + web build args
# ------------------------------------------------------------------------------
CACHE_DIR="${GARDA_ROOT}/${BUILDX_CACHE_DIR}"
mkdir -p "$CACHE_DIR"

WEB_BUILD_ARGS=()
if [[ -n "${NEXT_PUBLIC_API_BASE_URL:-}" ]]; then
  WEB_BUILD_ARGS+=(--build-arg "NEXT_PUBLIC_API_BASE_URL=${NEXT_PUBLIC_API_BASE_URL}")
fi
if [[ -n "${NEXT_PUBLIC_APP_URL:-}" ]]; then
  WEB_BUILD_ARGS+=(--build-arg "NEXT_PUBLIC_APP_URL=${NEXT_PUBLIC_APP_URL}")
fi

# ------------------------------------------------------------------------------
#  Build one image target
# ------------------------------------------------------------------------------
build_image_target() {
  local target="$1"
  local dockerfile_name="$target"
  local docker_target=""
  if [[ "$target" == "ml-pipeline" ]]; then
    dockerfile_name="ml"
    docker_target="pipeline"
  fi
  local dockerfile="docker/${dockerfile_name}.Dockerfile"
  local image="${IMAGE_TAG_PREFIX}-${target}"

  require_file "$dockerfile" "Dockerfile untuk image '$target' tidak ditemukan."

  local args=(
    buildx build
    --platform "${TARGET_PLATFORM}"
    --load
    -t "${image}:${RELEASE_TAG}"
    -t "${image}:latest"
    -f "$dockerfile"
  )
  [[ -n "$docker_target" ]] && args+=(--target "$docker_target")

  if [[ "$NO_CACHE" -eq 1 ]]; then
    args+=(--no-cache)
  else
    args+=(
      --cache-from "type=local,src=${CACHE_DIR}"
      --cache-to "type=local,dest=${CACHE_DIR},mode=max"
    )
  fi

  if [[ "$target" == "web" && "${#WEB_BUILD_ARGS[@]}" -gt 0 ]]; then
    args+=("${WEB_BUILD_ARGS[@]}")
  fi

  if [[ "$PUSH_REGISTRY" -eq 1 ]]; then
    args+=(--push)
  fi

  args+=(.)

  log_section "Build ${image}:${RELEASE_TAG} (${TARGET_PLATFORM})"
  log_debug "docker ${args[*]}"

  if [[ "$DRY_RUN" == "1" ]]; then
    echo_cmd "docker ${args[*]}"
    return 0
  fi

  local start_ts
  start_ts="$(date +%s)"

  if ! docker "${args[@]}"; then
    die "Build image '${image}' gagal." \
        "Periksa output di atas. Untuk build bersih coba: --no-cache."
  fi

  local elapsed=$(( $(date +%s) - start_ts ))
  local size
  size="$(docker image inspect "${image}:${RELEASE_TAG}" --format '{{.Size}}' 2>/dev/null || echo 0)"
  log_success "${image}:${RELEASE_TAG} selesai dalam ${elapsed}s — ukuran $(human_size "$size")"
}

# ------------------------------------------------------------------------------
#  Main
# ------------------------------------------------------------------------------
log_section "Build lokal — Garda Inflasi (Zero-Git)"
print_kv "Release tag"   "$RELEASE_TAG"
print_kv "Platform"      "$TARGET_PLATFORM"
print_kv "Services"      "${REQUESTED_SERVICES[*]}"
print_kv "Image targets" "${BUILD_IMAGES[*]}"
print_kv "Cache dir"     "$BUILDX_CACHE_DIR"
print_kv "Dry run"       "$([[ "$DRY_RUN" -eq 1 ]] && echo yes || echo no)"

for img_target in "${BUILD_IMAGES[@]}"; do
  build_image_target "$img_target"
done

log_section "Ringkasan image lokal"
if [[ "$DRY_RUN" == "1" ]]; then
  echo_cmd "docker images --filter reference='${IMAGE_TAG_PREFIX}-*'"
else
  docker images --filter "reference=${IMAGE_TAG_PREFIX}-*" \
    --format 'table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedSince}}' || true
fi

if [[ "$DRY_RUN" != "1" ]]; then
  printf '%s\n' "$RELEASE_TAG" > "${GARDA_ROOT}/.last-build-tag"
  printf '%s\n' "${REQUESTED_SERVICES[*]}" > "${GARDA_ROOT}/.last-build-services"
fi

log_success "Build lokal selesai. RELEASE_TAG=${RELEASE_TAG}"
