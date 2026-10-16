#!/usr/bin/env bash
# ==============================================================================
#  Garda Inflasi — scripts/rollback/keep-last-n-images.sh
#  Prunes old image tags and old tarballs, keeping only the N most recent.
#
#  Runs BOTH on the Mac (prunes dist/*.tar.gz) and on the VM (prunes images).
#  Detection is automatic: if `docker` is available it prunes images too.
#
#  Usage:
#      bash scripts/rollback/keep-last-n-images.sh [options]
#
#  Options:
#      --keep=N        Number of versions to keep (default: KEEP_LAST_N_IMAGES or 3)
#      --images-only   Only prune docker images
#      --tarballs-only Only prune dist/*.tar.gz
#      --dry-run       Print what would be removed without removing anything
#      -h, --help      Show this help
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

install_traps

# ------------------------------------------------------------------------------
#  Defaults + argument parsing
# ------------------------------------------------------------------------------
KEEP="${KEEP_LAST_N_IMAGES:-3}"
PRUNE_IMAGES=1
PRUNE_TARBALLS=1
DRY_RUN="${DRY_RUN:-0}"

usage() {
  sed -n '2,18p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --keep=*)        KEEP="${1#*=}" ;;
    --keep)          shift; KEEP="${1:-3}" ;;
    --images-only)   PRUNE_TARBALLS=0 ;;
    --tarballs-only) PRUNE_IMAGES=0 ;;
    --dry-run)       DRY_RUN=1 ;;
    -h|--help)       usage; exit 0 ;;
    *) die "Opsi tidak dikenal: $1" "Jalankan 'bash scripts/rollback/keep-last-n-images.sh --help'." ;;
  esac
  shift
done

export DRY_RUN

if ! [[ "$KEEP" =~ ^[0-9]+$ ]] || [[ "$KEEP" -lt 1 ]]; then
  die "Nilai --keep tidak valid: '$KEEP'" "Gunakan bilangan bulat ≥ 1 (contoh: --keep=3)."
fi

# Resolve repo root without requiring .env.deploy (this script runs on the VM too).
if [[ -n "${REMOTE_DIR:-}" && -d "${REMOTE_DIR}" ]]; then
  GARDA_ROOT="${REMOTE_DIR}"
else
  GARDA_ROOT="$(garda_repo_root "$SCRIPT_DIR")"
fi
cd "$GARDA_ROOT"

IMAGE_PREFIX="${IMAGE_TAG_PREFIX:-garda}"
SERVICES=(api web caddy)

log_section "Prune — keep last ${KEEP} version(s)"
print_kv "Root"   "$GARDA_ROOT"
print_kv "Prefix" "$IMAGE_PREFIX"
print_kv "Dry run" "$([[ "$DRY_RUN" -eq 1 ]] && echo yes || echo no)"

# ------------------------------------------------------------------------------
#  Prune docker images
# ------------------------------------------------------------------------------
prune_images() {
  if ! command -v docker >/dev/null 2>&1; then
    log_debug "docker tidak tersedia — prune image dilewati."
    return 0
  fi

  if ! docker info >/dev/null 2>&1; then
    log_warn "Docker daemon tidak berjalan — prune image dilewati."
    return 0
  fi

  local svc
  for svc in "${SERVICES[@]}"; do
    local repo="${IMAGE_PREFIX}-${svc}"

    # Collect tags that look like a git SHA (hex, 7-40 chars) — never `latest`.
    local tags
    tags="$(docker image ls "$repo" --format '{{.Tag}}' 2>/dev/null \
      | grep -E '^[0-9a-f]{7,40}$' || true)"

    if [[ -z "$tags" ]]; then
      log_debug "${repo}: tidak ada tag SHA — dilewati."
      continue
    fi

    # Sort by image creation time, newest first.
    local sorted
    sorted="$(
      while IFS= read -r tag; do
        [[ -z "$tag" ]] && continue
        local created
        created="$(docker image inspect "${repo}:${tag}" --format '{{.Created}}' 2>/dev/null || echo '1970-01-01T00:00:00Z')"
        printf '%s %s\n' "$created" "$tag"
      done <<< "$tags" | sort -r | awk '{print $2}'
    )"

    local total
    total="$(printf '%s\n' "$sorted" | grep -c . || true)"
    [[ -z "$total" ]] && total=0

    if [[ "$total" -le "$KEEP" ]]; then
      log_info "${repo}: ${total} tag SHA (≤ ${KEEP}) — tidak ada yang dihapus."
      continue
    fi

    local to_remove
    to_remove="$(printf '%s\n' "$sorted" | tail -n +$(( KEEP + 1 )))"

    log_info "${repo}: ${total} tag SHA — menghapus $(( total - KEEP ))."
    local tag
    while IFS= read -r tag; do
      [[ -z "$tag" ]] && continue
      if [[ "$DRY_RUN" == "1" ]]; then
        echo_cmd "docker rmi ${repo}:${tag}"
      else
        if docker rmi "${repo}:${tag}" >/dev/null 2>&1; then
          log_success "Dihapus: ${repo}:${tag}"
        else
          log_warn "Gagal menghapus ${repo}:${tag} (mungkin masih dipakai container)."
        fi
      fi
    done <<< "$to_remove"
  done
}

# ------------------------------------------------------------------------------
#  Prune local tarballs
# ------------------------------------------------------------------------------
prune_tarballs() {
  local dist_dir="${GARDA_ROOT}/dist"
  if [[ ! -d "$dist_dir" ]]; then
    log_debug "dist/ tidak ada — prune tarball dilewati."
    return 0
  fi

  local files
  files="$(ls -1t "${dist_dir}"/garda-images-*.tar.gz 2>/dev/null || true)"

  if [[ -z "$files" ]]; then
    log_debug "Tidak ada tarball di dist/ — dilewati."
    return 0
  fi

  local total
  total="$(printf '%s\n' "$files" | grep -c . || true)"
  [[ -z "$total" ]] && total=0

  if [[ "$total" -le "$KEEP" ]]; then
    log_info "dist/: ${total} tarball (≤ ${KEEP}) — tidak ada yang dihapus."
    return 0
  fi

  log_info "dist/: ${total} tarball — menghapus $(( total - KEEP ))."
  local f
  while IFS= read -r f; do
    [[ -z "$f" ]] && continue
    if [[ "$DRY_RUN" == "1" ]]; then
      echo_cmd "rm -f $f"
    else
      rm -f "$f" && log_success "Dihapus: $(basename "$f")"
    fi
  done < <(printf '%s\n' "$files" | tail -n +$(( KEEP + 1 )))
}

# ------------------------------------------------------------------------------
#  Main
# ------------------------------------------------------------------------------
[[ "$PRUNE_IMAGES" -eq 1 ]] && prune_images
[[ "$PRUNE_TARBALLS" -eq 1 ]] && prune_tarballs

log_success "Prune selesai (keep=${KEEP})."