#!/bin/sh
# ==============================================================================
#  Garda Inflasi — restore.sh
#  Memulihkan dump (-Fc) buatan backup.sh ke sebuah database.
#
#  Pakai:
#    scripts/backup/restore.sh <dump> --target-db <nama> [--replace]
#    make db/restore FILE=backups/garda_inflasi-....dump TARGET=garda_pulih
#
#  Pengaman (restore salah sasaran adalah insiden tersendiri):
#    - Checksum SHA-256 diperiksa dulu; tidak cocok -> berhenti.
#    - Target BARU secara bawaan: database dibuat; bila sudah ada, berhenti
#      kecuali --replace (hanya untuk target yang BUKAN database sumber).
#    - Target = database yang sedang dipakai (PGDATABASE) hanya boleh dengan
#      GARDA_RESTORE_CONFIRM=<nama-database>; langkah ini MENIMPA data hidup.
#      Hentikan api/worker/web dulu dan ambil backup baru sebelum menimpa.
#    - Nama sistem (postgres, template0, template1) ditolak.
#
#  Butuh pengguna DB dengan hak CREATEDB (dan superuser untuk membuat ekstensi
#  uuid-ossp/pgcrypto yang ada di dump).
# ==============================================================================

set -eu
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lib.sh
. "${SCRIPT_DIR}/lib.sh"

usage() {
  sed -n '2,22p' "$0" | sed 's/^# \{0,2\}//' >&2
  exit 2
}

DUMP=""
TARGET=""
REPLACE=0
while [ $# -gt 0 ]; do
  case "$1" in
    --target-db) TARGET="${2:-}"; shift 2 ;;
    --replace) REPLACE=1; shift ;;
    -h | --help) usage ;;
    -*) die "Opsi tidak dikenal: $1" ;;
    *) [ -z "${DUMP}" ] || die "Hanya satu berkas dump yang diterima."; DUMP="$1"; shift ;;
  esac
done
[ -n "${DUMP}" ] || usage
[ -n "${TARGET}" ] || die "--target-db wajib diisi."

assert_safe_dbname "${TARGET}"
case "${TARGET}" in
  postgres | template0 | template1) die "Menolak target database sistem: ${TARGET}" ;;
esac

verify_checksum "${DUMP}"

if [ "${TARGET}" = "${PGDATABASE}" ]; then
  [ "${GARDA_RESTORE_CONFIRM:-}" = "${TARGET}" ] ||
    die "Target ${TARGET} adalah database yang sedang dipakai. Untuk menimpanya set GARDA_RESTORE_CONFIRM=${TARGET} (hentikan api/worker dulu)."
  warn "MENIMPA database hidup ${TARGET} dengan isi ${DUMP}."
  RESTORE_FLAGS="--clean --if-exists"
else
  if db_exists "${TARGET}"; then
    [ "${REPLACE}" = "1" ] || die "Database ${TARGET} sudah ada. Pakai nama lain atau --replace."
    warn "Menghapus database ${TARGET} yang ada (--replace)."
    psql_db postgres "DROP DATABASE ${TARGET} WITH (FORCE)" >/dev/null
  fi
  info "Membuat database ${TARGET}"
  psql_db postgres "CREATE DATABASE ${TARGET}" >/dev/null
  RESTORE_FLAGS=""
fi

info "Restore ${DUMP} -> ${TARGET}"
START="$(date +%s)"
# shellcheck disable=SC2086  # RESTORE_FLAGS sengaja dipecah menjadi beberapa argumen
pg_run pg_restore -U "${PGUSER}" -d "${TARGET}" --no-owner --exit-on-error ${RESTORE_FLAGS} <"${DUMP}"
info "Restore selesai dalam $(($(date +%s) - START)) detik. Versi goose: $(goose_version "${TARGET}")"
