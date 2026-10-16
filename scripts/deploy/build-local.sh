#!/usr/bin/env bash
# ==============================================================================
#  Garda Inflasi — scripts/deploy/build-local.sh
#  Builds the application images on the LOCAL machine (MacBook) with buildx.
#
#  Images produced:
#      garda-api:<git-sha>    garda-api:latest
#      garda-web:<git-sha>    garda-web:latest
#      garda-caddy:<git-sha>  garda-caddy:latest
#
#  Usage:
#      bash scripts/deploy/build-local.sh [options]
#
#  Options:
#      --only=api,web,caddy   Build only the listed services (default: all)
#      --no-cache             Disable buildx cache (slow, but clean)
#      --push-registry        Also push to a registry (off by default)
#      --allow-dirty          Allow building from a dirty working tree
#      --dry-run              Print commands without executing
#      -h, --help             Show this help
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
ALL_SERVICES=(api web caddy)
SERVICES=("${ALL_SERVICES[@]}")
NO_CACHE=0
PUSH_REGISTRY=0
ALLOW_DIRTY=0
DRY_RUN="${DRY_RUN:-0}"

usage() {
  sed -n '2,20p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --only=*)
      IFS=',' read -r -a SERVICES <<< "${1#*=}"
      ;;
    --only)
      shift
      IFS=',' read -r -a SERVICES <<< "${1:-}"
      ;;
    --no-cache)      NO_CACHE=1 ;;
    --push-registry) PUSH_REGISTRY=1 ;;
    --allow-dirty)   ALLOW_DIRTY=1 ;;
    --dry-run)       DRY_RUN=1 ;;
    -h|--help)       usage; exit 0 ;;
    *)
      die "Opsi tidak dikenal: $1" "Jalankan 'bash scripts/deploy/build-local.sh --help' untuk daftar opsi."
      ;;
  esac
  shift
done

export DRY_RUN

# Validate the requested service list.
for svc in "${SERVICES[@]}"; do
  case "$svc" in
    api|web|caddy) ;;
    *) die "Service tidak dikenal: '$svc'" "Pilih dari: api, web, caddy (contoh: --only=api,web)." ;;
  esac
done

# ------------------------------------------------------------------------------
#  Load config + prerequisites
# ------------------------------------------------------------------------------
load_env_deploy

require_cmd docker "Install Docker Desktop: brew install --cask docker"
require_cmd git    "Install git: brew install git"

if ! docker buildx version >/dev/null 2>&1; then
  die "docker buildx tidak tersedia." \
      "Docker Desktop modern sudah menyertakan buildx. Cek: docker buildx version"
fi

if ! docker info >/dev/null 2>&1; then
  die "Docker daemon tidak berjalan." "Jalankan Docker Desktop lalu ulangi."
fi

cd "$GARDA_ROOT"

# ------------------------------------------------------------------------------
#  Git SHA + dirty-tree guard
# ------------------------------------------------------------------------------
GIT_SHA="$(git rev-parse --short HEAD)"
GIT_BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo 'detached')"
export GIT_SHA

if [[ -n "$(git status --porcelain)" ]]; then
  if [[ "$ALLOW_DIRTY" -eq 1 ]]; then
    log_warn "Working tree kotor — build tetap dilanjutkan (--allow-dirty)."
  else
    log_error "Working tree kotor: ada perubahan yang belum di-commit."
    git status --short >&2
    die "Build dari working tree kotor akan menghasilkan image yang tidak reproducible." \
        "Commit perubahan dulu, atau jalankan ulang dengan --allow-dirty bila ini memang disengaja."
  fi
fi

# ------------------------------------------------------------------------------
#  Buildx cache directory
# ------------------------------------------------------------------------------
CACHE_DIR="${GARDA_ROOT}/${BUILDX_CACHE_DIR}"
mkdir -p "$CACHE_DIR"

# ------------------------------------------------------------------------------
#  Build args for the web image (browser-visible config)
# ------------------------------------------------------------------------------
WEB_BUILD_ARGS=()
if [[ -n "${NEXT_PUBLIC_API_BASE_URL:-}" ]]; then
  WEB_BUILD_ARGS+=(--build-arg "NEXT_PUBLIC_API_BASE_URL=${NEXT_PUBLIC_API_BASE_URL}")
fi
if [[ -n "${NEXT_PUBLIC_APP_URL:-}" ]]; then
  WEB_BUILD_ARGS+=(--build-arg "NEXT_PUBLIC_APP_URL=${NEXT_PUBLIC_APP_URL}")
fi
if [[ -n "${NEXT_PUBLIC_MAPBOX_TOKEN:-}" ]]; then
  WEB_BUILD_ARGS+=(--build-arg "NEXT_PUBLIC_MAPBOX_TOKEN=${NEXT_PUBLIC_MAPBOX_TOKEN}")
fi

# ------------------------------------------------------------------------------
#  Build one service
# ------------------------------------------------------------------------------
build_service() {
  local svc="$1"
  local dockerfile="docker/${svc}.Dockerfile"
  local image="${IMAGE_TAG_PREFIX}-${svc}"

  require_file "$dockerfile" "Dockerfile untuk service '$svc' tidak ditemukan di repo."

  local args=(
    buildx build
    --platform "${TARGET_PLATFORM}"
    --load
    -t "${image}:${GIT_SHA}"
    -t "${image}:latest"
    -f "$dockerfile"
  )

  if [[ "$NO_CACHE" -eq 1 ]]; then
    args+=(--no-cache)
  else
    args+=(
      --cache-from "type=local,src=${CACHE_DIR}"
      --cache-to "type=local,dest=${CACHE_DIR},mode=max"
    )
  fi

  if [[ "$svc" == "web" && "${#WEB_BUILD_ARGS[@]}" -gt 0 ]]; then
    args+=("${WEB_BUILD_ARGS[@]}")
  fi

  if [[ "$PUSH_REGISTRY" -eq 1 ]]; then
    args+=(--push)
  fi

  args+=(.)

  log_section "Build ${image}:${GIT_SHA} (${TARGET_PLATFORM})"
  log_debug "docker ${args[*]}"

  if [[ "$DRY_RUN" == "1" ]]; then
    echo_cmd "docker ${args[*]}"
    return 0
  fi

  local start_ts
  start_ts="$(date +%s)"

  if ! docker "${args[@]}"; then
    die "Build image '${image}' gagal." \
        "Periksa output di atas. Untuk build bersih coba: --no-cache. Untuk masalah cache: rm -rf ${BUILDX_CACHE_DIR}"
  fi

  local elapsed=$(( $(date +%s) - start_ts ))
  local size
  size="$(docker image inspect "${image}:${GIT_SHA}" --format '{{.Size}}' 2>/dev/null || echo 0)"
  log_success "${image}:${GIT_SHA} selesai dalam ${elapsed}s — ukuran $(human_size "$size")"
}

# ------------------------------------------------------------------------------
#  Main
# ------------------------------------------------------------------------------
log_section "Build lokal — Garda Inflasi"
print_kv "Git SHA"        "$GIT_SHA"
print_kv "Branch"         "$GIT_BRANCH"
print_kv "Platform"       "$TARGET_PLATFORM"
print_kv "Services"       "${SERVICES[*]}"
print_kv "Cache dir"      "$BUILDX_CACHE_DIR"
print_kv "Push registry"  "$([[ "$PUSH_REGISTRY" -eq 1 ]] && echo yes || echo no)"
print_kv "Dry run"        "$([[ "$DRY_RUN" -eq 1 ]] && echo yes || echo no)"

for svc in "${SERVICES[@]}"; do
  build_service "$svc"
done

log_section "Ringkasan image lokal"
if [[ "$DRY_RUN" == "1" ]]; then
  echo_cmd "docker images --filter reference='${IMAGE_TAG_PREFIX}-*'"
else
  docker images --filter "reference=${IMAGE_TAG_PREFIX}-*" \
    --format 'table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedSince}}' || true
fi

log_success "Build lokal selesai. GIT_SHA=${GIT_SHA}"
printf '%s\n' "$GIT_SHA" > "${GARDA_ROOT}/.last-build-sha"
log_debug "GIT_SHA disimpan ke .last-build-sha"