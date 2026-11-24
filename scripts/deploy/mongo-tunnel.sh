#!/usr/bin/env bash
# ==============================================================================
#  Garda Inflasi — scripts/deploy/mongo-tunnel.sh
#  Membuka SSH port-forwarding dari Mac lokal ke MongoDB produksi di VM
#  (yang terikat aman di 127.0.0.1:27017 pada VM). Idempoten: jika tunnel
#  ke VM sudah aktif pada port lokal tersebut, skrip langsung keluar 0.
#
#  Pemakaian:
#      make mongo/tunnel
#      make mongo/tunnel ARGS=--background
#      make mongo/tunnel ARGS=--restart
#      make mongo/tunnel ARGS=--stop
#      LOCAL_MONGO_PORT=27019 bash scripts/deploy/mongo-tunnel.sh
# ==============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/ssh-wrapper.sh
source "${SCRIPT_DIR}/../lib/ssh-wrapper.sh"

LOCAL_PORT="${LOCAL_MONGO_PORT:-27018}"
REMOTE_PORT="${REMOTE_MONGO_PORT:-27017}"
MODE="foreground"

for arg in "$@"; do
  case "$arg" in
    --background|-b)
      MODE="background"
      ;;
    --stop)
      MODE="stop"
      ;;
    --restart|-r)
      MODE="restart"
      ;;
    --port=*)
      LOCAL_PORT="${arg#*=}"
      ;;
    -h|--help)
      echo "Pemakaian: $0 [--port=27018] [--background|--restart|--stop]"
      exit 0
      ;;
  esac
done

load_env_deploy
validate_ssh_key
require_cmd ssh "ssh diperlukan untuk membuka SSH tunnel."

existing_tunnel_pids() {
  local pids=() p cmd
  while IFS= read -r p; do
    [[ -n "$p" ]] || continue
    cmd="$(ps -p "$p" -o command= 2>/dev/null || true)"
    if [[ "$cmd" == *"ssh"* && "$cmd" == *"${LOCAL_PORT}:127.0.0.1:${REMOTE_PORT}"* ]]; then
      pids+=("$p")
    fi
  done < <(lsof -nP -t -iTCP:"$LOCAL_PORT" -sTCP:LISTEN 2>/dev/null || true)
  printf '%s\n' "${pids[@]}"
}

stop_existing_tunnels() {
  local pids=()
  while IFS= read -r p; do
    [[ -n "$p" ]] && pids+=("$p")
  done < <(existing_tunnel_pids)
  if [[ "${#pids[@]}" -eq 0 ]]; then
    return 1
  fi
  kill "${pids[@]}" 2>/dev/null || true
  sleep 0.5
  return 0
}

if [[ "$MODE" == "stop" ]]; then
  if stop_existing_tunnels; then
    log_success "SSH tunnel MongoDB pada 127.0.0.1:${LOCAL_PORT} ditutup."
  else
    log_info "Tidak ada SSH tunnel aktif pada 127.0.0.1:${LOCAL_PORT}."
  fi
  exit 0
fi

if [[ "$MODE" == "restart" ]]; then
  stop_existing_tunnels || true
  MODE="background"
fi

# Cek apakah port lokal sudah dipakai
if lsof -nP -iTCP:"$LOCAL_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
  ACTIVE_PIDS=()
  while IFS= read -r p; do
    [[ -n "$p" ]] && ACTIVE_PIDS+=("$p")
  done < <(existing_tunnel_pids)

  if [[ "${#ACTIVE_PIDS[@]}" -gt 0 ]]; then
    log_success "SSH tunnel MongoDB prod sudah aktif pada 127.0.0.1:${LOCAL_PORT} (PID: ${ACTIVE_PIDS[*]})."
    log_info "Konek ke mongodb://garda:<MONGO_PASSWORD>@127.0.0.1:${LOCAL_PORT}/?authSource=admin"
    log_info "Untuk memulai ulang tunnel: make mongo/tunnel ARGS=--restart | Untuk menutup: make mongo/tunnel ARGS=--stop"
    exit 0
  fi

  log_error "Port lokal 127.0.0.1:${LOCAL_PORT} dipakai oleh proses lain (bukan tunnel MongoDB prod):"
  lsof -nP -iTCP:"$LOCAL_PORT" -sTCP:LISTEN || true
  log_info "Gunakan port lain, mis.: LOCAL_MONGO_PORT=27019 make mongo/tunnel"
  exit 1
fi

opts=()
while IFS= read -r line; do opts+=("$line"); done < <(__garda_ssh_opts)

log_info "Membuka SSH tunnel MongoDB prod: 127.0.0.1:${LOCAL_PORT} -> ${VM_HOST} (127.0.0.1:${REMOTE_PORT})"

if [[ "$MODE" == "background" ]]; then
  if [[ "${VM_SSH_AUTH_MODE:-key}" == "password" ]]; then
    if command -v sshpass >/dev/null 2>&1; then
      SSHPASS="$SSH_PASSWORD" sshpass -e ssh -f -N -o ExitOnForwardFailure=yes \
        -L "127.0.0.1:${LOCAL_PORT}:127.0.0.1:${REMOTE_PORT}" \
        "${opts[@]}" "${VM_USER}@${VM_HOST}"
    else
      GARDA_EXPECT_PW="$SSH_PASSWORD" expect -c "
        log_user 0
        spawn -noecho ssh -f -N -o ExitOnForwardFailure=yes -L 127.0.0.1:${LOCAL_PORT}:127.0.0.1:${REMOTE_PORT} ${opts[*]} ${VM_USER}@${VM_HOST}
        expect {
          -re {(?i)Are you sure you want to continue connecting} { send -- \"yes\r\"; exp_continue }
          -re {(?i)'s password: ?|password: ?$} { send -- \"\$env(GARDA_EXPECT_PW)\r\"; exp_continue }
          eof
        }
      " >/dev/null 2>&1 || true
    fi
  else
    ssh -f -N -o ExitOnForwardFailure=yes \
      -L "127.0.0.1:${LOCAL_PORT}:127.0.0.1:${REMOTE_PORT}" \
      "${opts[@]}" "${VM_USER}@${VM_HOST}"
  fi

  if ! lsof -nP -iTCP:"$LOCAL_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
    die "Gagal membuka SSH tunnel pada 127.0.0.1:${LOCAL_PORT}."
  fi
  log_success "Tunnel aktif di latar pada 127.0.0.1:${LOCAL_PORT}."
  exit 0
fi

log_info "Tekan Ctrl+C untuk menutup tunnel."
if [[ "${VM_SSH_AUTH_MODE:-key}" == "password" ]]; then
  __garda_run_with_password ssh -N -o ExitOnForwardFailure=yes \
    -L "127.0.0.1:${LOCAL_PORT}:127.0.0.1:${REMOTE_PORT}" \
    "${opts[@]}" "${VM_USER}@${VM_HOST}"
else
  exec ssh -N -o ExitOnForwardFailure=yes \
    -L "127.0.0.1:${LOCAL_PORT}:127.0.0.1:${REMOTE_PORT}" \
    "${opts[@]}" "${VM_USER}@${VM_HOST}"
fi
