#!/usr/bin/env bash
# ==============================================================================
#  Garda Inflasi — scripts/deploy/save-images.sh
#  Exports only the selected/built images into a gzip-compressed tarball.
#  Zero-Git workflow.
#
#  Output: dist/garda-images-<release-tag>.tar.gz
#
#  Usage:
#      bash scripts/deploy/save-images.sh [options]
#
#  Options:
#      --tag=<tag>       Override release tag (default: .last-build-tag)
#      --only=svc1,svc2  Save only images for these services
#      --keep-tar        Keep the uncompressed .tar alongside the .tar.gz
#      --dry-run         Print commands without executing
#      -h, --help        Show this help
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"
# shellcheck source=../lib/ssh-wrapper.sh
source "${SCRIPT_DIR}/../lib/ssh-wrapper.sh"

install_traps

TAG_OVERRIDE=""
ONLY_RAW=""
KEEP_TAR=0
DRY_RUN="${DRY_RUN:-0}"
MIN_FREE_GB="${MIN_FREE_GB:-5}"

usage() {
  sed -n '2,18p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --tag=*)    TAG_OVERRIDE="${1#*=}" ;;
    --tag)      shift; TAG_OVERRIDE="${1:-}" ;;
    --sha=*)    TAG_OVERRIDE="${1#*=}" ;;
    --sha)      shift; TAG_OVERRIDE="${1:-}" ;;
    --only=*)   ONLY_RAW="${1#*=}" ;;
    --only)     shift; ONLY_RAW="${1:-}" ;;
    --keep-tar) KEEP_TAR=1 ;;
    --dry-run)  DRY_RUN=1 ;;
    -h|--help)  usage; exit 0 ;;
    *) die "Opsi tidak dikenal: $1" "Jalankan 'bash scripts/deploy/save-images.sh --help'." ;;
  esac
  shift
done

export DRY_RUN

load_env_deploy

require_cmd docker "Install Docker Desktop: brew install --cask docker"
require_cmd gzip   "gzip tersedia secara default di macOS/Linux."
require_cmd df     "df tersedia secara default di macOS/Linux."

cd "$GARDA_ROOT"

# ------------------------------------------------------------------------------
#  Resolve RELEASE_TAG
# ------------------------------------------------------------------------------
if [[ -n "$TAG_OVERRIDE" ]]; then
  RELEASE_TAG="$TAG_OVERRIDE"
elif [[ -f "${GARDA_ROOT}/.last-build-tag" ]]; then
  RELEASE_TAG="$(tr -d '[:space:]' < "${GARDA_ROOT}/.last-build-tag")"
elif [[ -f "${GARDA_ROOT}/.last-build-sha" ]]; then
  RELEASE_TAG="$(tr -d '[:space:]' < "${GARDA_ROOT}/.last-build-sha")"
else
  RELEASE_TAG=""
fi

if [[ -z "$RELEASE_TAG" ]]; then
  die "Tidak bisa menentukan RELEASE_TAG." \
      "Jalankan build dulu (make deploy/build), atau lewatkan eksplisit: --tag=<tag>"
fi
export RELEASE_TAG

# ------------------------------------------------------------------------------
#  Resolve services & deduplicated image list
# ------------------------------------------------------------------------------
TARGET_SERVICES=()
if [[ -n "$ONLY_RAW" ]]; then
  IFS=',' read -r -a TARGET_SERVICES <<< "$ONLY_RAW"
elif [[ -f "${GARDA_ROOT}/.last-build-services" ]]; then
  read -r -a TARGET_SERVICES < "${GARDA_ROOT}/.last-build-services"
else
  if [[ "${GATEWAY_MODE:-external}" == "builtin" ]]; then
    TARGET_SERVICES=(api worker ml web caddy)
  else
    TARGET_SERVICES=(api worker ml web)
  fi
fi

IMAGE_TARGETS=()
add_image_target() {
  local candidate="$1"
  local existing
  for existing in "${IMAGE_TARGETS[@]:-}"; do
    [[ "$existing" == "$candidate" ]] && return 0
  done
  IMAGE_TARGETS+=("$candidate")
}

for svc in "${TARGET_SERVICES[@]}"; do
  case "$svc" in
    api|worker) add_image_target "api" ;;
    web)        add_image_target "web" ;;
    ml)         add_image_target "ml" ;;
    ml-worker)  add_image_target "ml-pipeline" ;;
    crawler)    add_image_target "crawler" ;;
    caddy)      add_image_target "caddy" ;;
    *) die "Service tidak dikenal: '$svc'" "Pilih dari: api, worker, web, ml, ml-worker, crawler, caddy." ;;
  esac
done

IMAGES=()
for tgt in "${IMAGE_TARGETS[@]}"; do
  img="${IMAGE_TAG_PREFIX}-${tgt}:${RELEASE_TAG}"
  if [[ "$DRY_RUN" != "1" ]] && ! docker image inspect "$img" >/dev/null 2>&1; then
    die "Image '$img' tidak ada di Docker lokal." \
        "Build dulu: bash scripts/deploy/build-local.sh --tag=${RELEASE_TAG} --only=$(IFS=','; echo "${TARGET_SERVICES[*]}")"
  fi
  IMAGES+=("$img")
done

# ------------------------------------------------------------------------------
#  Disk space pre-flight
# ------------------------------------------------------------------------------
DIST_DIR="${GARDA_ROOT}/dist"
mkdir -p "$DIST_DIR"

check_disk_space() {
  local avail_gb
  if is_macos; then
    avail_gb="$(df -k "$DIST_DIR" | awk 'NR==2 {printf "%d", $4/1024/1024}')"
  else
    avail_gb="$(df -BG "$DIST_DIR" | awk 'NR==2 {gsub(/G/,"",$4); print $4}')"
  fi

  if [[ -z "$avail_gb" ]]; then
    log_warn "Tidak bisa membaca free space — pre-flight disk dilewati."
    return 0
  fi

  log_info "Free space di $(dirname "$DIST_DIR"): ${avail_gb} GB (butuh ≥ ${MIN_FREE_GB} GB)"

  if [[ "$avail_gb" -lt "$MIN_FREE_GB" ]]; then
    die "Disk lokal hampir penuh: hanya ${avail_gb} GB tersisa (butuh ≥ ${MIN_FREE_GB} GB)." \
        "Bersihkan artefak lama: bash scripts/rollback/keep-last-n-images.sh"
  fi
}

if [[ "$DRY_RUN" == "1" ]]; then
  echo_cmd "df -k $DIST_DIR  # pre-flight: butuh >= ${MIN_FREE_GB} GB free"
else
  check_disk_space
fi

# ------------------------------------------------------------------------------
#  Save + compress
# ------------------------------------------------------------------------------
TARBALL="${DIST_DIR}/garda-images-${RELEASE_TAG}.tar.gz"
RAW_TAR="${DIST_DIR}/garda-images-${RELEASE_TAG}.tar"

log_section "Save images → ${TARBALL}"
print_kv "Release tag" "$RELEASE_TAG"
print_kv "Services"    "${TARGET_SERVICES[*]}"
print_kv "Images"      "${IMAGES[*]}"

if [[ "$DRY_RUN" == "1" ]]; then
  echo_cmd "docker save ${IMAGES[*]} | gzip -9 > ${TARBALL}"
  if [[ "$KEEP_TAR" -eq 1 ]]; then
    echo_cmd "docker save ${IMAGES[*]} -o ${RAW_TAR}"
  fi
  exit 0
fi

start_ts="$(date +%s)"

if ! docker save "${IMAGES[@]}" | gzip -9 > "$TARBALL"; then
  rm -f "$TARBALL"
  die "Gagal menyimpan image ke '$TARBALL'." \
      "Cek free space (df -h .) dan pastikan semua image ada."
fi

if [[ "$KEEP_TAR" -eq 1 ]]; then
  log_info "Menyimpan .tar mentah (--keep-tar) ..."
  docker save "${IMAGES[@]}" -o "$RAW_TAR"
fi

elapsed=$(( $(date +%s) - start_ts ))
size_bytes="$(file_size_bytes "$TARBALL")"

log_success "Tarball dibuat dalam ${elapsed}s: $(human_size "$size_bytes")"
print_kv "Path" "$TARBALL"

printf '%s\n' "$RELEASE_TAG" > "${GARDA_ROOT}/.last-build-tag"
printf '%s\n' "$TARBALL" > "${GARDA_ROOT}/.last-tarball"
printf '%s\n' "${TARGET_SERVICES[*]}" > "${GARDA_ROOT}/.last-save-services"

log_info "Lanjut: bash scripts/deploy/transfer.sh --tag=${RELEASE_TAG}"
