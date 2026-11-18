#!/usr/bin/env bash
# ==============================================================================
#  Garda Inflasi — scripts/lib/ssh-wrapper.sh
#  Loads .env.deploy, supports SSH password auth (expect/sshpass) OR SSH key
#  auth, and exposes vm_ssh / vm_ssh_sudo / vm_scp / vm_rsync / vm_preflight.
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
GARDA_REQUIRED_ENV_FIELDS=(
  VM_HOST
  VM_USER
  SSH_PORT
  REMOTE_DIR
)

load_env_deploy() {
  if [[ ! -f "$ENV_DEPLOY_FILE" ]]; then
    die "File konfigurasi deploy tidak ditemukan: $ENV_DEPLOY_FILE" \
        "Jalankan 'make deploy/init-env' lalu isi nilainya (lihat .env.deploy.example)."
  fi

  local __override_vars=(
    TARGET_PLATFORM IMAGE_TAG_PREFIX KEEP_LAST_N_IMAGES BUILDX_CACHE_DIR
    SSH_CONNECT_TIMEOUT SSH_ALIVE_INTERVAL SSH_ALIVE_COUNT_MAX
    PUBLIC_HOST APEX_HOST GATEWAY_MODE GATEWAY_CADDY_DIR GATEWAY_NETWORK
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

  local __i
  if [[ "${#__saved_names[@]}" -gt 0 ]]; then
    for __i in "${!__saved_names[@]}"; do
      printf -v "${__saved_names[$__i]}" '%s' "${__saved_values[$__i]}"
      export "${__saved_names[$__i]}"
    done
  fi

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
  : "${SSH_CONNECT_TIMEOUT:=15}"
  : "${SSH_ALIVE_INTERVAL:=30}"
  : "${SSH_ALIVE_COUNT_MAX:=20}"
  : "${PUBLIC_HOST:=app.gardainflasi.id}"
  : "${APEX_HOST:=gardainflasi.id}"
  : "${GATEWAY_MODE:=external}"
  : "${GATEWAY_CADDY_DIR:=/home/ubuntu/caddy-gardainflasi}"
  : "${GATEWAY_NETWORK:=caddy-gardainflasi_edge}"
  : "${SSH_PASSWORD:=}"
  : "${SSH_KEY_PATH:=}"

  if [[ -n "$SSH_PASSWORD" ]]; then
    VM_SSH_AUTH_MODE="password"
  elif [[ -n "$SSH_KEY_PATH" ]]; then
    VM_SSH_AUTH_MODE="key"
  else
    die "Kredensial SSH tidak lengkap di $ENV_DEPLOY_FILE." \
        "Isi SSH_PASSWORD atau SSH_KEY_PATH."
  fi

  export TARGET_PLATFORM IMAGE_TAG_PREFIX KEEP_LAST_N_IMAGES BUILDX_CACHE_DIR
  export SSH_CONNECT_TIMEOUT SSH_ALIVE_INTERVAL SSH_ALIVE_COUNT_MAX
  export PUBLIC_HOST APEX_HOST GATEWAY_MODE GATEWAY_CADDY_DIR GATEWAY_NETWORK
  export VM_SSH_AUTH_MODE

  log_debug "Konfigurasi deploy dimuat dari $ENV_DEPLOY_FILE (auth=${VM_SSH_AUTH_MODE}, gateway=${GATEWAY_MODE})"
}

# ------------------------------------------------------------------------------
#  validate_ssh_key — validates key (key mode) OR expect/sshpass (password mode)
# ------------------------------------------------------------------------------
validate_ssh_key() {
  if [[ "${VM_SSH_AUTH_MODE:-key}" == "password" ]]; then
    if ! command -v sshpass >/dev/null 2>&1 && ! command -v expect >/dev/null 2>&1; then
      die "Autentikasi password SSH butuh 'expect' atau 'sshpass'." \
          "Di macOS 'expect' tersedia di /usr/bin/expect; atau install: brew install hudochenkov/sshpass/sshpass"
    fi
    log_debug "SSH auth mode: password ($(command -v sshpass >/dev/null 2>&1 && echo sshpass || echo expect))"
    return 0
  fi

  # shellcheck disable=SC2088
  if [[ "${SSH_KEY_PATH:0:2}" == '~/' ]]; then
    SSH_KEY_PATH="${HOME}/${SSH_KEY_PATH#\~/}"
  fi

  if [[ ! -e "$SSH_KEY_PATH" ]]; then
    die "SSH private key tidak ditemukan: $(mask_secret "$SSH_KEY_PATH")" \
        "Periksa SSH_KEY_PATH di $ENV_DEPLOY_FILE, atau isi SSH_PASSWORD."
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
#  Internal Expect / SSHPass runner (keeps password out of CLI args / ps)
# ------------------------------------------------------------------------------
__garda_run_with_password() {
  if command -v sshpass >/dev/null 2>&1; then
    SSHPASS="$SSH_PASSWORD" sshpass -e "$@"
    return $?
  fi

  GARDA_EXPECT_PW="$SSH_PASSWORD" expect -f - -- "$@" <<'EXPECT_EOF'
set timeout -1
set stty_init "-opost"
log_user 0
spawn -noecho {*}$argv
set authed 0
expect {
  -re {(?i)Are you sure you want to continue connecting} {
    send -- "yes\r"
    exp_continue
  }
  -re {(?i)(\[sudo\] password[^\r\n]*:|'s password: ?|password: ?$)} {
    send -- "$env(GARDA_EXPECT_PW)\r"
    if {!$authed} {
      expect -re {\r?\n}
      set authed 1
      log_user 1
    }
    exp_continue
  }
  eof
}
catch wait result
set rc [lindex $result 3]
if {$rc eq ""} { set rc 1 }
exit $rc
EXPECT_EOF
}

# ------------------------------------------------------------------------------
#  Common SSH option array
# ------------------------------------------------------------------------------
__garda_ssh_opts() {
  local opts=(
    -p "$SSH_PORT"
    -o StrictHostKeyChecking=accept-new
    -o ConnectTimeout="$SSH_CONNECT_TIMEOUT"
    -o ServerAliveInterval="$SSH_ALIVE_INTERVAL"
    -o ServerAliveCountMax="$SSH_ALIVE_COUNT_MAX"
    -o LogLevel=ERROR
  )
  if [[ "${VM_SSH_AUTH_MODE:-key}" == "password" ]]; then
    opts+=(
      -o PubkeyAuthentication=no
      -o NumberOfPasswordPrompts=1
    )
  else
    opts+=(
      -i "$SSH_KEY_PATH"
      -o BatchMode=yes
    )
  fi
  printf '%s\n' "${opts[@]}"
}

# ------------------------------------------------------------------------------
#  vm_ssh — run a command on the VM (returns clean stdout without PTY CRs)
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

  if [[ "${VM_SSH_AUTH_MODE:-key}" == "password" ]]; then
    __garda_run_with_password ssh -T "${opts[@]}" "${VM_USER}@${VM_HOST}" "$remote_cmd" | tr -d '\r'
    return "${PIPESTATUS[0]}"
  else
    ssh -T "${opts[@]}" "${VM_USER}@${VM_HOST}" "$remote_cmd"
  fi
}

# ------------------------------------------------------------------------------
#  vm_ssh_sudo — run a privileged command on the VM via sudo (uses SSH_PASSWORD)
# ------------------------------------------------------------------------------
vm_ssh_sudo() {
  local remote_cmd="$1"
  local opts=()
  while IFS= read -r line; do opts+=("$line"); done < <(__garda_ssh_opts)

  log_debug "vm_ssh_sudo → ${VM_USER}@${VM_HOST}:${SSH_PORT} :: $(mask_secret "$remote_cmd")"

  if [[ "${DRY_RUN:-0}" == "1" ]]; then
    echo_cmd "ssh -tt ${VM_USER}@${VM_HOST}:${SSH_PORT} sudo bash -c $(mask_secret "$remote_cmd")"
    return 0
  fi

  local encoded
  encoded="$(printf '%s' "$remote_cmd" | base64 | tr -d '\n')"
  local wrapped="echo '${encoded}' | base64 -d | sudo bash"

  if [[ "${VM_SSH_AUTH_MODE:-key}" == "password" ]]; then
    # Need PTY (-tt) when sudo prompts for password via expect, or sudo -S with sshpass
    if command -v sshpass >/dev/null 2>&1; then
      local wrapped_s="printf '%s\n' \"\$GARDA_SUDO_PW\" | sudo -S -p '' bash -c \"\$(echo '${encoded}' | base64 -d)\""
      SSHPASS="$SSH_PASSWORD" sshpass -e ssh -T "${opts[@]}" "${VM_USER}@${VM_HOST}" \
        "GARDA_SUDO_PW=$(printf '%q' "$SSH_PASSWORD") bash -c $(printf '%q' "$wrapped_s")"
    else
      __garda_run_with_password ssh -tt "${opts[@]}" "${VM_USER}@${VM_HOST}" "$wrapped" | tr -d '\r'
      return "${PIPESTATUS[0]}"
    fi
  else
    ssh -T "${opts[@]}" "${VM_USER}@${VM_HOST}" "echo '${encoded}' | base64 -d | sudo -n bash"
  fi
}

# ------------------------------------------------------------------------------
#  vm_scp — copy a local file to the VM
# ------------------------------------------------------------------------------
vm_scp() {
  local src="$1" dst="$2"
  local opts=()
  while IFS= read -r line; do opts+=("$line"); done < <(__garda_ssh_opts)
  # scp uses -P for port, ssh uses -p
  local scp_opts=()
  local skip_next=0
  for o in "${opts[@]}"; do
    if [[ "$skip_next" -eq 1 ]]; then
      scp_opts+=(-P "$o")
      skip_next=0
    elif [[ "$o" == "-p" ]]; then
      skip_next=1
    else
      scp_opts+=("$o")
    fi
  done

  log_debug "vm_scp → $src → ${VM_USER}@${VM_HOST}:$dst"

  if [[ "${DRY_RUN:-0}" == "1" ]]; then
    echo_cmd "scp $src ${VM_USER}@${VM_HOST}:$dst"
    return 0
  fi

  if [[ "${VM_SSH_AUTH_MODE:-key}" == "password" ]]; then
    __garda_run_with_password scp "${scp_opts[@]}" "$src" "${VM_USER}@${VM_HOST}:${dst}" | tr -d '\r'
    return "${PIPESTATUS[0]}"
  else
    scp "${scp_opts[@]}" "$src" "${VM_USER}@${VM_HOST}:${dst}"
  fi
}

# ------------------------------------------------------------------------------
#  vm_rsync — sync a local path to the VM (resumable, partial-aware)
# ------------------------------------------------------------------------------
vm_rsync() {
  local src="$1" dst="$2"
  shift 2
  local extra=("$@")

  local ssh_cmd="ssh -p ${SSH_PORT} -o StrictHostKeyChecking=accept-new -o ConnectTimeout=${SSH_CONNECT_TIMEOUT} -o ServerAliveInterval=${SSH_ALIVE_INTERVAL} -o ServerAliveCountMax=${SSH_ALIVE_COUNT_MAX} -o LogLevel=ERROR"
  if [[ "${VM_SSH_AUTH_MODE:-key}" == "password" ]]; then
    ssh_cmd+=" -o PubkeyAuthentication=no -o NumberOfPasswordPrompts=1"
  else
    ssh_cmd+=" -i ${SSH_KEY_PATH} -o BatchMode=yes"
  fi

  log_debug "vm_rsync → $src → ${VM_USER}@${VM_HOST}:$dst"

  if [[ "${DRY_RUN:-0}" == "1" ]]; then
    echo_cmd "rsync -avz --partial --progress -e '<ssh>' $src ${VM_USER}@${VM_HOST}:$dst"
    return 0
  fi

  if [[ "${VM_SSH_AUTH_MODE:-key}" == "password" ]]; then
    __garda_run_with_password rsync -avz --partial --progress \
      --timeout=300 \
      -e "$ssh_cmd" \
      ${extra[@]+"${extra[@]}"} \
      "$src" "${VM_USER}@${VM_HOST}:${dst}" | tr -d '\r'
    return "${PIPESTATUS[0]}"
  else
    rsync -avz --partial --progress \
      --timeout=300 \
      -e "$ssh_cmd" \
      ${extra[@]+"${extra[@]}"} \
      "$src" "${VM_USER}@${VM_HOST}:${dst}"
  fi
}

# ------------------------------------------------------------------------------
#  vm_ssh_script — execute a local script on the VM cleanly via base64/bash
#  (Avoids PTY stdin buffering issues under expect)
# ------------------------------------------------------------------------------
vm_ssh_script() {
  local script_path="$1"; shift
  local args=("$@")

  require_file "$script_path" "Script harus ada sebelum dikirim ke VM."

  local arg_quoted=""
  if [[ "${#args[@]}" -gt 0 ]]; then
    local a
    for a in "${args[@]}"; do
      arg_quoted+=" $(printf '%q' "$a")"
    done
  fi

  log_debug "vm_ssh_script → $(basename "$script_path")${arg_quoted}"

  if [[ "${DRY_RUN:-0}" == "1" ]]; then
    echo_cmd "ssh ${VM_USER}@${VM_HOST} 'bash -s --${arg_quoted}' < $script_path"
    return 0
  fi

  if [[ "${VM_SSH_AUTH_MODE:-key}" == "password" ]]; then
    local b64
    b64="$(base64 < "$script_path" | tr -d '\n')"
    vm_ssh "tmp=\$(mktemp /tmp/garda-script.XXXXXX.sh) && printf '%s' '${b64}' | base64 -d > \"\$tmp\" && chmod +x \"\$tmp\" && bash \"\$tmp\"${arg_quoted}; rc=\$?; rm -f \"\$tmp\"; exit \$rc"
  else
    local opts=()
    while IFS= read -r line; do opts+=("$line"); done < <(__garda_ssh_opts)
    ssh -T "${opts[@]}" "${VM_USER}@${VM_HOST}" "bash -s --${arg_quoted}" < "$script_path"
  fi
}

# ------------------------------------------------------------------------------
#  vm_preflight — verify connectivity + docker availability on the VM
# ------------------------------------------------------------------------------
vm_preflight() {
  log_info "Pre-flight: cek koneksi SSH ke ${VM_USER}@${VM_HOST}:${SSH_PORT} (${VM_SSH_AUTH_MODE}) ..."

  if [[ "${DRY_RUN:-0}" == "1" ]]; then
    echo_cmd "ssh ${VM_USER}@${VM_HOST} 'docker version && docker compose version'"
    return 0
  fi

  if ! vm_ssh "true" >/dev/null 2>&1; then
    die "Tidak bisa SSH ke ${VM_USER}@${VM_HOST}:${SSH_PORT}." \
        "Cek: (1) VM hidup & reachable, (2) kredensial di .env.deploy benar, (3) firewall mengizinkan port ${SSH_PORT}."
  fi

  if ! vm_ssh "command -v docker >/dev/null 2>&1"; then
    die "Docker tidak terpasang di VM (${VM_HOST})." \
        "Install Docker Engine di VM: curl -fsSL https://get.docker.com | sh"
  fi

  if ! vm_ssh "docker compose version >/dev/null 2>&1"; then
    die "Docker Compose v2 plugin tidak tersedia di VM (${VM_HOST})." \
        "Install plugin compose v2: sudo apt-get install -y docker-compose-plugin"
  fi

  if ! vm_ssh "docker info >/dev/null 2>&1"; then
    die "User '${VM_USER}' tidak punya akses ke Docker daemon di VM." \
        "Jalankan bootstrap server: bash scripts/deploy/init-server.sh (atau deploy dengan --init)."
  fi

  log_success "Pre-flight SSH + Docker OK (${VM_USER}@${VM_HOST})."
}
