#!/usr/bin/env bash
# ==============================================================================
#  Garda Inflasi — scripts/deploy/save-images.sh
#  Exports the built images into a single gzip-compressed tarball for transfer.
#
#  Output: dist/garda-images-<git-sha>.tar.gz
#
#  Usage:
#      bash scripts/deploy/save-images.sh [options]
#
#  Options:
#      --sha=<sha>     Override the git SHA (default: .last-build-sha or HEAD)
#      --keep-tar      Keep the uncompressed .tar alongside the .tar.gz
#      --dry-run       Print commands without executing
#      -h, --help      Show this help
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
SHA_OVERRIDE=""
KEEP_TAR=0
DRY_RUN="${DRY_RUN:-0}"
MIN_FREE_GB="${MIN_FREE_GB:-5}"

usage() {
  sed -n '2,17p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --sha=*)    SHA_OVERRIDE="${1#*=}" ;;
    --sha)      shift; SHA_OVERRIDE="${1:-}" ;;
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
#  Resolve GIT_SHA
# ------------------------------------------------------------------------------
if [[ -n "$SHA_OVERRIDE" ]]; then
  GIT_SHA="$SHA_OVERRIDE"
elif [[ -f "${GARDA_ROOT}/.last-build-sha" ]]; then
  GIT_SHA="$(tr -d '[:space:]' < "${GARDA_ROOT}/.last-build-sha")"
else
  GIT_SHA="$(git rev-parse --short HEAD)"
fi

if [[ -z "$GIT_SHA" ]]; then
  die "Tidak bisa menentukan GIT_SHA." \
      "Jalankan build dulu (make deploy/build), atau lewatkan eksplisit: --sha=<sha>"
fi

export GIT_SHA

# ------------------------------------------------------------------------------
#  Verify all images exist locally
# ------------------------------------------------------------------------------
IMAGES=()
for svc in api web caddy; do
  img="${IMAGE_TAG_PREFIX}-${svc}:${GIT_SHA}"
  if [[ "$DRY_RUN" != "1" ]] && ! docker image inspect "$img" >/dev/null 2>&1; then
    die "Image '$img' tidak ada di Docker lokal." \
        "Build dulu: make deploy/build (atau bash scripts/deploy/build-local.sh)."
  fi
  IMAGES+=("$img")
done

# ------------------------------------------------------------------------------
#  Disk space pre-flight (need >= MIN_FREE_GB free on the output filesystem)
# ------------------------------------------------------------------------------
DIST_DIR="${GARDA_ROOT}/dist"
mkdir -p "$DIST_DIR"

check_disk_space() {
  local avail_gb
  if is_macos; then
    # `df -g` prints 1K-blocks on macOS; use -k and convert.
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
        "Bersihkan artefak lama: make clean, atau hapus image/tarball lama: bash scripts/rollback/keep-last-n-images.sh"
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
TARBALL="${DIST_DIR}/garda-images-${GIT_SHA}.tar.gz"
RAW_TAR="${DIST_DIR}/garda-images-${GIT_SHA}.tar"

log_section "Save images → ${TARBALL}"
print_kv "GIT_SHA" "$GIT_SHA"
print_kv "Images"  "${IMAGES[*]}"

if [[ "$DRY_RUN" == "1" ]]; then
  echo_cmd "docker save ${IMAGES[*]} | gzip -9 > ${TARBALL}"
  if [[ "$KEEP_TAR" -eq 1 ]]; then
    echo_cmd "docker save ${IMAGES[*]} -o ${RAW_TAR}"
  fi
  exit 0
fi

start_ts="$(date +%s)"

# `docker save` writes to stdout; pipe straight into gzip to avoid a huge
# intermediate file on disk. `set -o pipefail` (from common.sh) makes a
# failure in either stage abort the script.
if ! docker save "${IMAGES[@]}" | gzip -9 > "$TARBALL"; then
  rm -f "$TARBALL"
  die "Gagal menyimpan image ke '$TARBALL'." \
      "Cek free space (df -h .) dan pastikan semua image ada: docker images | grep ${IMAGE_TAG_PREFIX}"
fi

if [[ "$KEEP_TAR" -eq 1 ]]; then
  log_info "Menyimpan .tar mentah (--keep-tar) ..."
  docker save "${IMAGES[@]}" -o "$RAW_TAR"
fi

elapsed=$(( $(date +%s) - start_ts ))
size_bytes="$(file_size_bytes "$TARBALL")"

log_success "Tarball dibuat dalam ${elapsed}s: $(human_size "$size_bytes")"
print_kv "Path" "$TARBALL"

# Persist for the transfer step.
printf '%s\n' "$GIT_SHA" > "${GARDA_ROOT}/.last-build-sha"
printf '%s\n' "$TARBALL" > "${GARDA_ROOT}/.last-tarball"

log_info "Lanjut: bash scripts/deploy/transfer.sh"