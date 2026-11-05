#!/bin/sh
# ==============================================================================
#  Garda Inflasi — verify-restore.sh
#  Bukti bahwa backup benar-benar bisa dipulihkan:
#    1. membuat backup baru (atau memakai --file <dump>);
#    2. memeriksa checksum;
#    3. me-restore ke database SEMENTARA bernama garda_verify_<waktu>_<pid>
#       (BUKAN database yang sedang berjalan);
#    4. membandingkan versi goose dan jumlah baris tiap tabel dengan manifest
#       yang dicatat backup.sh (rentang sebelum..sesudah dump, karena database
#       hidup bisa menerima tulisan selama dump berjalan);
#    5. menghapus database sementara (juga saat gagal; VERIFY_KEEP_DB=1 untuk
#       menyimpannya buat diselidiki).
#
#  Pakai:   scripts/backup/verify-restore.sh [--file <dump>]
#           make db/backup/verify
#  Keluar 0 hanya bila semuanya cocok. Waktu restore terukur dicetak sebagai
#  masukan RTO.
# ==============================================================================

set -eu
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lib.sh
. "${SCRIPT_DIR}/lib.sh"

DUMP=""
while [ $# -gt 0 ]; do
  case "$1" in
    --file) DUMP="${2:-}"; shift 2 ;;
    -h | --help) sed -n '2,19p' "$0" | sed 's/^# \{0,2\}//' >&2; exit 2 ;;
    *) die "Opsi tidak dikenal: $1" ;;
  esac
done

assert_safe_dbname "${PGDATABASE}"
TMPDB="garda_verify_$(date -u +%Y%m%d%H%M%S)_$$"
assert_safe_dbname "${TMPDB}"
WORK="$(mktemp -d)"

cleanup() {
  rc=$?
  rm -rf -- "${WORK}"
  # Hanya database berawalan garda_verify_ dan BUKAN database sumber yang boleh dihapus.
  case "${TMPDB}" in
    garda_verify_*)
      if [ "${TMPDB}" != "${PGDATABASE}" ] && [ "${VERIFY_KEEP_DB:-0}" != "1" ]; then
        if db_exists "${TMPDB}" 2>/dev/null; then
          if psql_db postgres "DROP DATABASE ${TMPDB} WITH (FORCE)" >/dev/null 2>&1; then
            info "Database sementara ${TMPDB} dihapus."
          else
            warn "Gagal menghapus ${TMPDB}; hapus manual: DROP DATABASE ${TMPDB};"
          fi
        fi
      elif [ "${VERIFY_KEEP_DB:-0}" = "1" ]; then
        warn "VERIFY_KEEP_DB=1: database ${TMPDB} dibiarkan."
      fi
      ;;
  esac
  [ "${rc}" -eq 0 ] || log ERROR "verify-restore GAGAL (kode ${rc})."
  exit "${rc}"
}
trap cleanup EXIT INT TERM

T0="$(date +%s)"
if [ -z "${DUMP}" ]; then
  info "Langkah 1/5: backup baru"
  DUMP="$(sh "${SCRIPT_DIR}/backup.sh")"
else
  info "Langkah 1/5: memakai dump yang ada: ${DUMP}"
fi
T1="$(date +%s)"

info "Langkah 2/5: checksum"
verify_checksum "${DUMP}"
[ -f "${DUMP}.manifest" ] || die "Manifest ${DUMP}.manifest tidak ada (backup dibuat dengan BACKUP_MANIFEST=0?); tidak ada pembanding."

info "Langkah 3/5: restore ke database sementara ${TMPDB}"
psql_db postgres "CREATE DATABASE ${TMPDB}" >/dev/null
pg_run pg_restore -U "${PGUSER}" -d "${TMPDB}" --no-owner --exit-on-error <"${DUMP}"
T2="$(date +%s)"

info "Langkah 4/5: membandingkan dengan manifest"
FAIL=0
WANT_GOOSE="$(sed -n 's/^goose_version=//p' "${DUMP}.manifest")"
GOT_GOOSE="$(goose_version "${TMPDB}")"
if [ -n "${WANT_GOOSE}" ] && [ "${WANT_GOOSE}" = "${GOT_GOOSE}" ]; then
  info "goose version: ${GOT_GOOSE} (cocok)"
else
  warn "goose version berbeda: manifest=${WANT_GOOSE:-?} hasil restore=${GOT_GOOSE}"
  FAIL=1
fi

row_counts "${TMPDB}" >"${WORK}/restored"
grep '^count|' "${DUMP}.manifest" >"${WORK}/manifest" || true
# manifest: count|tabel|sebelum|sesudah ; restored: tabel|jumlah
awk -F'|' '
  NR == FNR { lo[$2] = ($3 < $4 ? $3 : $4) + 0; hi[$2] = ($3 > $4 ? $3 : $4) + 0; seen[$2] = 1; next }
  { got[$1] = $2 + 0; if (!($1 in seen)) { printf "TIDAK ADA di manifest: %s\n", $1; bad++ } }
  END {
    for (t in seen) {
      if (!(t in got)) { printf "HILANG setelah restore: %s\n", t; bad++; continue }
      ok = (got[t] >= lo[t] && got[t] <= hi[t])
      if (!ok) bad++
      n++
      printf "%-34s manifest %d..%d  restore %d  %s\n", t, lo[t], hi[t], got[t], (ok ? "OK" : "BEDA")
    }
    printf "RINGKAS tabel=%d beda=%d\n", n, bad + 0
  }' "${WORK}/manifest" "${WORK}/restored" | LC_ALL=C sort >"${WORK}/report"
if ! grep -q '^RINGKAS tabel=[1-9]' "${WORK}/report"; then
  warn "Tidak ada tabel yang dibandingkan."
  FAIL=1
fi
cat "${WORK}/report" >&2
grep -q 'BEDA\|HILANG\|TIDAK ADA' "${WORK}/report" && FAIL=1

[ "${FAIL}" -eq 0 ] || die "Verifikasi restore GAGAL: lihat ketidakcocokan di atas."

T3="$(date +%s)"
info "Langkah 5/5: lulus. Database sementara dihapus oleh cleanup."
info "HASIL OK  backup=$((T1 - T0))s restore=$((T2 - T1))s (RTO terukur) bandingkan=$((T3 - T2))s dump=${DUMP}"
