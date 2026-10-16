#!/usr/bin/env bash
# ==============================================================================
#  Garda Inflasi — scripts/lib/common.sh
#  Shared primitives for every deploy script: logging, colours, guards,
#  secret masking, atomic locking, retry, OS/arch detection, cleanup traps.
#
#  Usage (from any script):
#      SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
#      # shellcheck source=../lib/common.sh
#      source "${SCRIPT_DIR}/../lib/common.sh"
#
#  This file is meant to be SOURCED, never executed directly.
# ==============================================================================

# Guard against double-sourcing (idempotent).
if [[ -n "${__GARDA_COMMON_SH_LOADED:-}" ]]; then
  return 0 2>/dev/null || true
fi
__GARDA_COMMON_SH_LOADED=1

set -euo pipefail

# ------------------------------------------------------------------------------
#  Colours (respect NO_COLOR — https://no-color.org/)
# ------------------------------------------------------------------------------
if [[ -t 1 && -z "${NO_COLOR:-}" && "${TERM:-dumb}" != "dumb" ]]; then
  C_RESET=$'\033[0m'
  C_RED=$'\033[0;31m'
  C_GREEN=$'\033[0;32m'
  C_YELLOW=$'\033[0;33m'
  C_BLUE=$'\033[0;34m'
  C_MAGENTA=$'\033[0;35m'
  C_CYAN=$'\033[0;36m'
  C_BOLD=$'\033[1m'
  C_DIM=$'\033[2m'
else
  C_RESET='' C_RED='' C_GREEN='' C_YELLOW='' C_BLUE=''
  C_MAGENTA='' C_CYAN='' C_BOLD='' C_DIM=''
fi

# ------------------------------------------------------------------------------
#  Logging
# ------------------------------------------------------------------------------
# Log level: debug | info | warn | error  (LOG_LEVEL env overrides)
GARDA_LOG_LEVEL="${LOG_LEVEL:-info}"

__garda_level_num() {
  case "$1" in
    debug) echo 10 ;;
    info)  echo 20 ;;
    warn)  echo 30 ;;
    error) echo 40 ;;
    *)     echo 20 ;;
  esac
}

__garda_should_log() {
  local want have
  want="$(__garda_level_num "$1")"
  have="$(__garda_level_num "$GARDA_LOG_LEVEL")"
  [[ "$want" -ge "$have" ]]
}

__garda_ts() { date -u +%Y-%m-%dT%H:%M:%SZ; }

log_debug() {
  __garda_should_log debug || return 0
  printf '%s%s [DEBUG]%s %s\n' "$C_DIM" "$(__garda_ts)" "$C_RESET" "$*" >&2
}

log_info() {
  __garda_should_log info || return 0
  printf '%s%s [INFO ]%s %s\n' "$C_BLUE" "$(__garda_ts)" "$C_RESET" "$*" >&2
}

log_warn() {
  __garda_should_log warn || return 0
  printf '%s%s [WARN ]%s %s\n' "$C_YELLOW" "$(__garda_ts)" "$C_RESET" "$*" >&2
}

log_error() {
  __garda_should_log error || return 0
  printf '%s%s [ERROR]%s %s\n' "$C_RED" "$(__garda_ts)" "$C_RESET" "$*" >&2
}

log_success() {
  __garda_should_log info || return 0
  printf '%s%s [ OK  ]%s %s\n' "$C_GREEN" "$(__garda_ts)" "$C_RESET" "$*" >&2
}

# Section banner — purely cosmetic, always printed at info level.
log_section() {
  __garda_should_log info || return 0
  printf '\n%s%s── %s ──%s\n' "$C_BOLD" "$C_CYAN" "$*" "$C_RESET" >&2
}

# ------------------------------------------------------------------------------
#  die — actionable fatal error
#  Usage: die "what happened" "how to fix it"
# ------------------------------------------------------------------------------
die() {
  local cause="${1:-Unknown error}"
  local remedy="${2:-}"
  log_error "$cause"
  if [[ -n "$remedy" ]]; then
    printf '%s        ↳ Solusi: %s%s\n' "$C_YELLOW" "$remedy" "$C_RESET" >&2
  fi
  exit 1
}

# ------------------------------------------------------------------------------
#  require_cmd — verify a binary exists, with an actionable install hint
#  Usage: require_cmd docker "brew install --cask docker"
# ------------------------------------------------------------------------------
require_cmd() {
  local cmd="$1"
  local hint="${2:-}"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    die "Perintah '$cmd' tidak ditemukan di PATH." \
        "${hint:-Install '$cmd' terlebih dahulu lalu jalankan ulang.}"
  fi
  log_debug "require_cmd: $cmd → $(command -v "$cmd")"
}

# ------------------------------------------------------------------------------
#  mask_secret — redact values whose KEY looks sensitive
#  Usage: mask_secret "DB_PASSWORD=hunter2"  →  "DB_PASSWORD=***"
#         echo "$line" | mask_secret
# ------------------------------------------------------------------------------
mask_secret() {
  local input
  if [[ $# -gt 0 ]]; then
    input="$*"
  else
    input="$(cat)"
  fi
  printf '%s' "$input" | sed -E \
    -e 's#(://[^:/@[:space:]]+:)[^@[:space:]]+@#\1***@#g' \
    -e 's#(([A-Za-z0-9_]*(PASSWORD|PASSWD|SECRET|TOKEN|KEY|URI|DSN|CREDENTIAL)[A-Za-z0-9_]*)[[:space:]]*[=:][[:space:]]*)[^[:space:]]+#\1***#gI'
}

# ------------------------------------------------------------------------------
#  OS / architecture detection
# ------------------------------------------------------------------------------
detect_os() {
  case "$(uname -s)" in
    Darwin) echo "macos" ;;
    Linux)  echo "linux" ;;
    *)      echo "unknown" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    arm64|aarch64) echo "arm64" ;;
    x86_64|amd64)  echo "amd64" ;;
    *)             echo "unknown" ;;
  esac
}

# Portable "is this macOS?" helper (BSD vs GNU tool differences).
is_macos() { [[ "$(detect_os)" == "macos" ]]; }

# Portable in-place sed: BSD sed requires a suffix argument.
# Usage: sed_inplace 's/foo/bar/' file
sed_inplace() {
  local expr="$1" file="$2"
  if is_macos; then
    sed -i.bak "$expr" "$file" && rm -f "${file}.bak"
  else
    sed -i "$expr" "$file"
  fi
}

# Portable file size in bytes (BSD stat vs GNU stat).
file_size_bytes() {
  local f="$1"
  [[ -f "$f" ]] || { echo 0; return 0; }
  if is_macos; then
    stat -f%z "$f"
  else
    stat -c%s "$f"
  fi
}

# Human-readable size from a byte count.
human_size() {
  local bytes="${1:-0}"
  awk -v b="$bytes" 'BEGIN {
    split("B KB MB GB TB", u, " ");
    i = 1;
    while (b >= 1024 && i < 5) { b /= 1024; i++ }
    printf "%.2f %s", b, u[i]
  }'
}

# ------------------------------------------------------------------------------
#  Tempfile registry + cleanup trap
# ------------------------------------------------------------------------------
GARDA_TMPFILES=()
GARDA_LOCK_DIR=""
GARDA_CLEANUP_DONE=0

# Register a tempfile for automatic removal on exit.
register_tmpfile() {
  GARDA_TMPFILES+=("$1")
}

# Create a tempfile and register it.
make_tmpfile() {
  local prefix="${1:-garda}"
  local f
  f="$(mktemp "${TMPDIR:-/tmp}/${prefix}.XXXXXXXXXX")"
  register_tmpfile "$f"
  printf '%s' "$f"
}

cleanup() {
  local rc=$?
  [[ "$GARDA_CLEANUP_DONE" -eq 1 ]] && return "$rc"
  GARDA_CLEANUP_DONE=1

  local f
  for f in "${GARDA_TMPFILES[@]:-}"; do
    [[ -n "$f" && -e "$f" ]] && rm -rf "$f" 2>/dev/null || true
  done

  if [[ -n "$GARDA_LOCK_DIR" && -d "$GARDA_LOCK_DIR" ]]; then
    rmdir "$GARDA_LOCK_DIR" 2>/dev/null || true
    log_debug "Lock released: $GARDA_LOCK_DIR"
  fi

  return "$rc"
}

# Install the standard traps. Call once, early, from the entry-point script.
install_traps() {
  trap cleanup EXIT
  trap 'log_error "Interrupted (SIGINT) at ${BASH_SOURCE[0]}:${LINENO}"; exit 130' INT
  trap 'log_error "Terminated (SIGTERM) at ${BASH_SOURCE[0]}:${LINENO}"; exit 143' TERM
  trap 'log_error "Error at ${BASH_SOURCE[0]}:${LINENO} — command: ${BASH_COMMAND}"' ERR
}

# ------------------------------------------------------------------------------
#  with_lock — atomic mutual exclusion via `mkdir`
#  Usage: with_lock .deploy.lock my_function arg1 arg2
#  The lock directory is removed by the EXIT trap.
# ------------------------------------------------------------------------------
with_lock() {
  local lock_path="$1"; shift
  local stale_after="${GARDA_LOCK_STALE_SECONDS:-7200}"

  if [[ -d "$lock_path" ]]; then
    local age=0
    if is_macos; then
      age=$(( $(date +%s) - $(stat -f%m "$lock_path" 2>/dev/null || echo 0) ))
    else
      age=$(( $(date +%s) - $(stat -c%Y "$lock_path" 2>/dev/null || echo 0) ))
    fi
    if [[ "$age" -gt "$stale_after" ]]; then
      log_warn "Lock '$lock_path' berumur ${age}s (> ${stale_after}s) — dianggap stale, dihapus."
      rm -rf "$lock_path"
    else
      die "Deploy lain sedang berjalan (lock: $lock_path, umur ${age}s)." \
          "Tunggu sampai selesai, atau hapus manual bila yakin tidak ada proses aktif: rm -rf '$lock_path'"
    fi
  fi

  if ! mkdir "$lock_path" 2>/dev/null; then
    die "Gagal mengambil lock '$lock_path' (race condition)." \
        "Coba lagi sebentar lagi; bila persisten hapus: rm -rf '$lock_path'"
  fi
  GARDA_LOCK_DIR="$lock_path"
  log_debug "Lock acquired: $lock_path"

  "$@"
}

# ------------------------------------------------------------------------------
#  retry — run a command up to N times with a delay between attempts
#  Usage: retry 5 3 curl -fsS http://localhost/healthz
# ------------------------------------------------------------------------------
retry() {
  local attempts="$1" delay="$2"; shift 2
  local i=1 rc=0
  while [[ "$i" -le "$attempts" ]]; do
    if "$@"; then
      return 0
    fi
    rc=$?
    if [[ "$i" -lt "$attempts" ]]; then
      log_debug "retry $i/$attempts gagal (rc=$rc) — ulang dalam ${delay}s: $*"
      sleep "$delay"
    fi
    i=$(( i + 1 ))
  done
  log_error "retry: '$*' gagal setelah $attempts percobaan (rc=$rc)."
  return "$rc"
}

# ------------------------------------------------------------------------------
#  Misc helpers
# ------------------------------------------------------------------------------
# Run a command, or just print it when DRY_RUN=1.
run_or_echo() {
  if [[ "${DRY_RUN:-0}" == "1" ]]; then
    printf '%s[DRY-RUN]%s %s\n' "$C_MAGENTA" "$C_RESET" "$*" >&2
    return 0
  fi
  "$@"
}

# Print a command without executing it (dry-run helper for shell strings).
echo_cmd() {
  printf '%s[DRY-RUN]%s %s\n' "$C_MAGENTA" "$C_RESET" "$*" >&2
}

# Assert a required environment variable is set and non-empty.
require_env() {
  local name="$1"
  local hint="${2:-}"
  if [[ -z "${!name:-}" ]]; then
    die "Environment variable '$name' kosong/tidak diset." \
        "${hint:-Isi '$name' di .env.deploy (lihat .env.deploy.example).}"
  fi
}

# Assert a required file exists.
require_file() {
  local path="$1"
  local hint="${2:-}"
  if [[ ! -f "$path" ]]; then
    die "File wajib tidak ditemukan: $path" "${hint:-Pastikan file tersebut ada di repo.}"
  fi
}

# Assert a required directory exists.
require_dir() {
  local path="$1"
  local hint="${2:-}"
  if [[ ! -d "$path" ]]; then
    die "Direktori wajib tidak ditemukan: $path" "${hint:-Buat direktori tersebut terlebih dahulu.}"
  fi
}

# Resolve the repository root from any script location.
garda_repo_root() {
  local start="${1:-$(pwd)}"
  local dir="$start"
  while [[ "$dir" != "/" ]]; do
    if [[ -f "$dir/compose.yaml" && -d "$dir/scripts" ]]; then
      printf '%s' "$dir"
      return 0
    fi
    dir="$(dirname "$dir")"
  done
  die "Tidak bisa menemukan root repo (cari compose.yaml + scripts/)." \
      "Jalankan script dari dalam direktori repo garda-inflasi."
}

# Print a key/value summary block.
print_kv() {
  local key="$1" value="$2"
  printf '  %s%-22s%s %s\n' "$C_DIM" "$key" "$C_RESET" "$value" >&2
}