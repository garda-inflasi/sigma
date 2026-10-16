#!/usr/bin/env bash
# ==============================================================================
#  Garda Inflasi — scripts/lib/ssh-wrapper.sh
#  Loads .env.deploy, validates SSH prerequisites, and exposes vm_ssh / vm_scp /
#  vm_rsync helpers with hardened, non-interactive SSH options.
#
#  This file is meant to be SOURCED, never executed directly.
# ==============================================================================

if [[ -n "${__GARDA_SSH_WRAPPER_SH_LOADED:-}" ]]; then
  return 0 2>/dev/null || true
fi
__GARDA_SSH_WRAPPER_SH_LOADED=1

# ------------------------------------------------------------------------------
#  Locate repo root + load common.sh
# ------------------------------------------------------------------------------
__GARDA_SSH_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./common.sh
source "${__GARDA_SSH_LIB_DIR}/common.sh"

GARDA_ROOT="${GARDA_ROOT:-$(garda_repo_root "$__GARDA_SSH_LIB_DIR")}"
ENV_DEPLOY_FILE="${ENV_DEPLOY_FILE:-${GARDA_ROOT}/.env.deploy}"

# ------------------------------------------------------------------------------
#  load_env_deploy — source .env.deploy and validate required fields
# ------------------------------------------------------------------------------
# Fields that MUST be present and non-empty.
GARDA_REQUIRED_ENV_FIELDS=(
  VM_HOST
  VM_USER
  SSH_KEY_PATH
  SSH_PORT
  REMOTE_DIR
)

load_env_deploy() {
  if [[ ! -f "$ENV_DEPLOY_FILE" ]]; then
    die "File konfigurasi deploy tidak ditemukan: $ENV_DEPLOY_FILE" \
        "Jalankan 'make deploy/init-env' lalu isi nilainya (lihat .env.deploy.example)."
  fi

  # Snapshot nilai yang sudah ada di environment SEBELUM source, supaya override
  # dari command line (mis. `TARGET_PLATFORM=linux/arm64 make deploy`) menang
  # atas nilai di .env.deploy.
  local __override_vars=(
    TARGET_PLATFORM IMAGE_TAG_PREFIX KEEP_LAST_N_IMAGES BUILDX_CACHE_DIR
    SSH_CONNECT_TIMEOUT SSH_ALIVE_INTERVAL SSH_ALIVE_COUNT_MAX PUBLIC_HOST
    NEXT_PUBLIC_API_BASE_URL NEXT_PUBLIC_APP_URL NEXT_PUBLIC_MAPBOX_TOKEN
    HEALTH_TIMEOUT_SECONDS HTTP_HEALTH_RETRIES HTTP_HEALTH_DELAY
    MIN_FREE_GB GARDA_LOCK_STALE_SECONDS LOG_LEVEL
  )
  local __saved_names=() __saved_values=() __v
  for __v in "${__override_vars[@]}"; do
    if [[ -n "${!__v+x}" ]]; then
      __saved_names+=("$__v")
      __saved_values+=("${!__v}")
    fi
  done

  # shellcheck disable=SC1090
  set -a
  # shellcheck source=/dev/null
  source "$ENV_DEPLOY_FILE"
  set +a

  # Re-apply override dari environment.
  local __i
  for __i in "${!__saved_names[@]}"; do
    printf -v "${__saved_names[$__i]}" '%s' "${__saved_values[$__i]}"
    export "${__saved_names[$__i]}"
  done

  local missing=()
  local field
  for field in "${GARDA_REQUIRED_ENV_FIELDS[@]}"; do
    if [[ -z "${!field:-}" ]]; then
      missing+=("$field")
    fi
  done

  if [[ "${#missing[@]}" -gt 0 ]]; then
    log_error "Field wajib berikut kosong/tidak ada di $ENV_DEPLOY_FILE:"
    local m
    for m in "${missing[@]}"; do
      printf '        - %s\n' "$m" >&2
    done
    die "Konfigurasi deploy tidak lengkap (${#missing[@]} field hilang)." \
        "Buka $ENV_DEPLOY_FILE dan isi field di atas. Template: .env.deploy.example"
  fi

  # Defaults for optional fields.
  : "${TARGET_PLATFORM:=linux/amd64}"
  : "${IMAGE_TAG_PREFIX:=garda}"
  : "${KEEP_LAST_N_IMAGES:=3}"
  : "${BUILDX_CACHE_DIR:=.buildx-cache}"
  : "${SSH_CONNECT_TIMEOUT:=10}"
  : "${SSH_ALIVE_INTERVAL:=30}"
  : "${SSH_ALIVE_COUNT_MAX:=3}"
  : "${PUBLIC_HOST:=}"

  export TARGET_PLATFORM IMAGE_TAG_PREFIX KEEP_LAST_N_IMAGES BUILDX_CACHE_DIR
  export SSH_CONNECT_TIMEOUT SSH_ALIVE_INTERVAL SSH_ALIVE_COUNT_MAX PUBLIC_HOST

  log_debug "Konfigurasi deploy dimuat dari $ENV_DEPLOY_FILE"
}

# ------------------------------------------------------------------------------
#  validate_ssh_key — exists, is a regular file, and has mode 600
# ------------------------------------------------------------------------------
validate_ssh_key() {
  # Expand a leading ~ (env files are not shell-expanded by `source` when quoted).
  # shellcheck disable=SC2088  # literal '~/' prefix comparison, not expansion
  if [[ "${SSH_KEY_PATH:0:2}" == '~/' ]]; then
    SSH_KEY_PATH="${HOME}/${SSH_KEY_PATH#\~/}"
  fi

  if [[ ! -e "$SSH_KEY_PATH" ]]; then
    die "SSH private key tidak ditemukan: $(mask_secret "$SSH_KEY_PATH")" \
        "Periksa SSH_KEY_PATH di $ENV_DEPLOY_FILE, atau buat key: ssh-keygen -t ed25519 -f $SSH_KEY_PATH"
  fi

  if [[ ! -f "$SSH_KEY_PATH" ]]; then
    die "SSH_KEY_PATH bukan file biasa: $(mask_secret "$SSH_KEY_PATH")" \
        "Arahkan SSH_KEY_PATH ke file private key (bukan direktori)."
  fi

  local perms
  if is_macos; then
    perms="$(stat -f '%Lp' "$SSH_KEY_PATH" 2>/dev/null || echo '')"
  else
    perms="$(stat -c '%a' "$SSH_KEY_PATH" 2>/dev/null || echo '')"
  fi

  if [[ "$perms" != "600" ]]; then
    die "Permission SSH key salah: $(mask_secret "$SSH_KEY_PATH") mode=$perms (harus 600)." \
        "Perbaiki dengan: chmod 600 $(mask_secret "$SSH_KEY_PATH")"
  fi

  log_debug "SSH key OK (mode 600): $(mask_secret "$SSH_KEY_PATH")"
}

# ------------------------------------------------------------------------------
#  Common SSH option array
# ------------------------------------------------------------------------------
__garda_ssh_opts() {
  printf '%s\n' \
    -i "$SSH_KEY_PATH" \
    -p "$SSH_PORT" \
    -o StrictHostKeyChecking=accept-new \
    -o ConnectTimeout="$SSH_CONNECT_TIMEOUT" \
    -o ServerAliveInterval="$SSH_ALIVE_INTERVAL" \
    -o ServerAliveCountMax="$SSH_ALIVE_COUNT_MAX" \
    -o BatchMode=yes \
    -o LogLevel=ERROR
}

# ------------------------------------------------------------------------------
#  vm_ssh — run a command on the VM
#  Usage: vm_ssh "docker ps"
#         vm_ssh "bash -s -- arg1" < local-script.sh
# ------------------------------------------------------------------------------
vm_ssh() {
  local remote_cmd="$1"
  local opts=()
  while IFS= read -r line; do opts+=("$line"); done < <(__garda_ssh_opts)

  log_debug "vm_ssh → ${VM_USER}@${VM_HOST}:${SSH_PORT} :: $(mask_secret "$remote_cmd")"

  if [[ "${DRY_RUN:-0}" == "1" ]]; then
    echo_cmd "ssh ${VM_USER}@${VM_HOST}:${SSH_PORT} $(mask_secret "$remote_cmd")"
    return 0
  fi

  ssh "${opts[@]}" "${VM_USER}@${VM_HOST}" "$remote_cmd"
}

# ------------------------------------------------------------------------------
#  vm_scp — copy a local file to the VM
#  Usage: vm_scp ./dist/x.tar.gz "$REMOTE_DIR/dist/"
# ------------------------------------------------------------------------------
vm_scp() {
  local src="$1" dst="$2"
  local opts=()
  while IFS= read -r line; do opts+=("$line"); done < <(__garda_ssh_opts)

  log_debug "vm_scp → $src → ${VM_USER}@${VM_HOST}:$dst"

  if [[ "${DRY_RUN:-0}" == "1" ]]; then
    echo_cmd "scp $src ${VM_USER}@${VM_HOST}:$dst"
    return 0
  fi

  scp "${opts[@]}" "$src" "${VM_USER}@${VM_HOST}:${dst}"
}

# ------------------------------------------------------------------------------
#  vm_rsync — sync a local path to the VM (resumable, partial-aware)
#  Usage: vm_rsync ./dist/x.tar.gz "$REMOTE_DIR/dist/"
#         vm_rsync ./db/migrations/ "$REMOTE_DIR/db/migrations/"
# ------------------------------------------------------------------------------
vm_rsync() {
  local src="$1" dst="$2"
  shift 2
  local extra=("$@")

  local ssh_cmd="ssh -i ${SSH_KEY_PATH} -p ${SSH_PORT}"
  ssh_cmd+=" -o StrictHostKeyChecking=accept-new"
  ssh_cmd+=" -o ConnectTimeout=${SSH_CONNECT_TIMEOUT}"
  ssh_cmd+=" -o ServerAliveInterval=${SSH_ALIVE_INTERVAL}"
  ssh_cmd+=" -o ServerAliveCountMax=${SSH_ALIVE_COUNT_MAX}"
  ssh_cmd+=" -o BatchMode=yes -o LogLevel=ERROR"

  log_debug "vm_rsync → $src → ${VM_USER}@${VM_HOST}:$dst"

  if [[ "${DRY_RUN:-0}" == "1" ]]; then
    echo_cmd "rsync -avz --partial --progress -e '<ssh>' $src ${VM_USER}@${VM_HOST}:$dst"
    return 0
  fi

  rsync -avz --partial --progress \
    --timeout=300 \
    -e "$ssh_cmd" \
    "${extra[@]}" \
    "$src" "${VM_USER}@${VM_HOST}:${dst}"
}

# ------------------------------------------------------------------------------
#  vm_ssh_script — pipe a local script into `bash -s` on the VM
#  Usage: vm_ssh_script ./scripts/deploy/remote-deploy.sh "$GIT_SHA"
# ------------------------------------------------------------------------------
vm_ssh_script() {
  local script_path="$1"; shift
  local args=("$@")

  require_file "$script_path" "Script harus ada sebelum dikirim ke VM."

  local opts=()
  while IFS= read -r line; do opts+=("$line"); done < <(__garda_ssh_opts)

  local arg_str=""
  if [[ "${#args[@]}" -gt 0 ]]; then
    arg_str=" -- ${args[*]}"
  fi

  log_debug "vm_ssh_script → $(basename "$script_path")${arg_str}"

  if [[ "${DRY_RUN:-0}" == "1" ]]; then
    echo_cmd "ssh ${VM_USER}@${VM_HOST} 'bash -s${arg_str}' < $script_path"
    return 0
  fi

  ssh "${opts[@]}" "${VM_USER}@${VM_HOST}" "bash -s${arg_str}" < "$script_path"
}

# ------------------------------------------------------------------------------
#  vm_preflight — verify connectivity + docker availability on the VM
# ------------------------------------------------------------------------------
vm_preflight() {
  log_info "Pre-flight: cek koneksi SSH ke ${VM_USER}@${VM_HOST}:${SSH_PORT} ..."

  if [[ "${DRY_RUN:-0}" == "1" ]]; then
    echo_cmd "ssh ${VM_USER}@${VM_HOST} 'docker version && docker compose version'"
    return 0
  fi

  if ! vm_ssh "true" >/dev/null 2>&1; then
    die "Tidak bisa SSH ke ${VM_USER}@${VM_HOST}:${SSH_PORT}." \
        "Cek: (1) VM hidup & reachable, (2) public key sudah di ~/.ssh/authorized_keys VM, (3) SSH_KEY_PATH benar, (4) firewall mengizinkan port ${SSH_PORT}."
  fi

  if ! vm_ssh "command -v docker >/dev/null 2>&1"; then
    die "Docker tidak terpasang di VM (${VM_HOST})." \
        "Install Docker Engine di VM: curl -fsSL https://get.docker.com | sh"
  fi

  if ! vm_ssh "docker compose version >/dev/null 2>&1"; then
    die "Docker Compose v2 plugin tidak tersedia di VM (${VM_HOST})." \
        "Install plugin compose v2: sudo apt-get install -y docker-compose-plugin (JANGAN pakai docker-compose v1)."
  fi

  if ! vm_ssh "docker info >/dev/null 2>&1"; then
    die "User '${VM_USER}' tidak punya akses ke Docker daemon di VM." \
        "Tambahkan user ke grup docker: sudo usermod -aG docker ${VM_USER} && newgrp docker (lalu login ulang)."
  fi

  log_success "Pre-flight SSH + Docker OK (${VM_USER}@${VM_HOST})."
}