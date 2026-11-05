#!/bin/sh
# ==============================================================================
#  Garda Inflasi — backup.sh
#  Membuat backup logis database: pg_dump format custom (-Fc), berkas checksum
#  SHA-256, validasi daftar isi dengan pg_restore --list, lalu menerapkan retensi.
#
#  Pakai:   scripts/backup/backup.sh            (atau: make db/backup)
#  Hasil:   $BACKUP_DIR/<db>-<UTC>.dump, <dump>.sha256, dan <dump>.manifest
#           (versi goose + jumlah baris tiap tabel sebelum/sesudah dump; dipakai
#           verify-restore.sh untuk membuktikan hasil restore lengkap).
#           Jalur dump dicetak ke stdout (log ke stderr).
#
#  Variabel: lihat lib.sh, ditambah
#    BACKUP_RETENTION_DAYS  hapus dump lebih tua dari N hari          [14]
#    BACKUP_KEEP_MIN        minimal N dump terbaru selalu disimpan    [3]
#    BACKUP_MANIFEST        0 = lewati manifest (tabel sangat besar)  [1]
#
#  Aman dijalankan pada database yang sedang dipakai (pg_dump memakai snapshot
#  konsisten). Skrip tidak pernah menulis ke database.
# ==============================================================================

set -eu
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lib.sh
. "${SCRIPT_DIR}/lib.sh"

BACKUP_RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"
BACKUP_KEEP_MIN="${BACKUP_KEEP_MIN:-3}"

case "${BACKUP_RETENTION_DAYS}${BACKUP_KEEP_MIN}" in
  *[!0-9]*) die "BACKUP_RETENTION_DAYS dan BACKUP_KEEP_MIN harus bilangan bulat." ;;
esac

# Hapus dump (dan checksum-nya) yang lebih tua dari retensi, kecuali
# BACKUP_KEEP_MIN dump terbaru. Hanya menyentuh berkas "<db>-*.dump".
apply_retention() {
  _total=0
  # shellcheck disable=SC2012  # nama berkas buatan skrip ini sendiri: aman untuk ls -t
  for _f in $(ls -1t "${BACKUP_DIR}/${PGDATABASE}"-*.dump 2>/dev/null || true); do
    _total=$((_total + 1))
    [ "${_total}" -le "${BACKUP_KEEP_MIN}" ] && continue
    if [ -n "$(find "${_f}" -mtime +"${BACKUP_RETENTION_DAYS}" 2>/dev/null)" ]; then
      info "Retensi: menghapus ${_f}"
      rm -f -- "${_f}" "${_f}.sha256" "${_f}.manifest"
    fi
  done
}

assert_safe_dbname "${PGDATABASE}"
mkdir -p "${BACKUP_DIR}"
umask 077

OUT="${BACKUP_DIR}/${PGDATABASE}-$(now_utc).dump"
TMP="${OUT}.partial"
PRE="${OUT}.pre.$$"
POST="${OUT}.post.$$"
trap 'rm -f -- "${TMP}" "${PRE}" "${POST}"' EXIT INT TERM

info "Backup ${PGDATABASE} -> ${OUT}"
START="$(date +%s)"
GOOSE=""
if [ "${BACKUP_MANIFEST:-1}" = "1" ]; then
  GOOSE="$(goose_version "${PGDATABASE}")"
  row_counts "${PGDATABASE}" >"${PRE}"
fi
pg_run pg_dump -U "${PGUSER}" -d "${PGDATABASE}" --format=custom --compress=6 --no-owner >"${TMP}"

[ -s "${TMP}" ] || die "Dump kosong; backup dibatalkan."
# Dump custom yang utuh bisa didaftar isinya; gagal bila terpotong/rusak.
ENTRIES="$(pg_run pg_restore --list <"${TMP}" | grep -vc '^;' || true)"
[ "${ENTRIES}" -gt 0 ] || die "pg_restore --list tidak menemukan entri; dump tidak valid."

if [ "${BACKUP_MANIFEST:-1}" = "1" ]; then
  row_counts "${PGDATABASE}" >"${POST}"
  {
    printf 'goose_version=%s\n' "${GOOSE}"
    awk -F'|' 'NR == FNR { pre[$1] = $2; next } { print "count|" $1 "|" pre[$1] + 0 "|" $2 }' "${PRE}" "${POST}"
  } >"${OUT}.manifest"
fi

mv -- "${TMP}" "${OUT}"
printf '%s  %s\n' "$(sha256_of "${OUT}")" "$(basename "${OUT}")" >"${OUT}.sha256"
verify_checksum "${OUT}"

SIZE="$(wc -c <"${OUT}" | tr -d ' ')"
info "Backup selesai: ${SIZE} byte, ${ENTRIES} entri, $(($(date +%s) - START)) detik."
apply_retention
printf '%s\n' "${OUT}"
