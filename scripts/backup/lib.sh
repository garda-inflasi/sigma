#!/bin/sh
# ==============================================================================
#  Garda Inflasi — scripts/backup/lib.sh
#  Fungsi bersama backup.sh, restore.sh, dan verify-restore.sh. Di-source, bukan
#  dijalankan langsung. POSIX sh agar jalan di host (bash/zsh/dash) maupun di
#  container postgres:16-alpine (busybox ash).
#
#  Dua mode koneksi, dipilih otomatis:
#    - Langsung : bila PGHOST diisi (mis. container backup di jaringan compose).
#                 pg_dump/pg_restore/psql dipanggil dari PATH; sandi lewat PGPASSWORD.
#    - Docker   : bila PGHOST kosong. Perintah dijalankan lewat
#                 `docker exec` ke container Postgres yang sudah berjalan
#                 (PG_CONTAINER, bawaan garda-postgres), jadi tidak perlu klien
#                 Postgres di host dan tidak perlu sandi (soket lokal container).
#
#  Variabel (semua opsional):
#    PG_CONTAINER   nama container Postgres (mode Docker)      [garda-postgres]
#    PGUSER         pengguna DB (butuh hak CREATEDB untuk verifikasi) [garda]
#    PGDATABASE     database sumber                             [garda_inflasi]
#    BACKUP_DIR     folder keluaran                             [./backups]
# ==============================================================================

set -eu

PG_CONTAINER="${PG_CONTAINER:-garda-postgres}"
PGUSER="${PGUSER:-${POSTGRES_USER:-garda}}"
PGDATABASE="${PGDATABASE:-${POSTGRES_DB:-garda_inflasi}}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
export PGUSER

log() { printf '%s [%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "${1}" "${2}" >&2; }
info() { log INFO "$*"; }
warn() { log WARN "$*"; }
die() { log ERROR "$*"; exit 1; }

# Jalankan klien Postgres (pg_dump, pg_restore, psql, ...) sesuai mode koneksi.
# stdin diteruskan (dipakai pg_restore membaca berkas dump).
pg_run() {
  if [ -n "${PGHOST:-}" ]; then
    "$@"
  else
    command -v docker >/dev/null 2>&1 || die "docker tidak ditemukan dan PGHOST kosong."
    docker exec -i "${PG_CONTAINER}" "$@"
  fi
}

# psql tanpa keluaran hias: psql_db <database> <sql>
psql_db() {
  _db="$1"
  shift
  pg_run psql -X -q -v ON_ERROR_STOP=1 -At -U "${PGUSER}" -d "${_db}" -c "$*"
}

# Hash SHA-256 sebuah berkas (hanya heksadesimalnya).
sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | cut -d' ' -f1
  else
    die "sha256sum/shasum tidak ditemukan."
  fi
}

# Periksa dump terhadap berkas .sha256 pendampingnya. Gagal bila tidak cocok.
verify_checksum() {
  _file="$1"
  [ -f "${_file}" ] || die "Berkas dump tidak ada: ${_file}"
  [ -f "${_file}.sha256" ] || die "Berkas checksum tidak ada: ${_file}.sha256"
  _want="$(cut -d' ' -f1 <"${_file}.sha256")"
  _have="$(sha256_of "${_file}")"
  [ "${_want}" = "${_have}" ] || die "Checksum TIDAK cocok untuk ${_file} (dump rusak atau diubah)."
  info "Checksum cocok: ${_have}"
}

# Nama database yang aman dipakai tanpa kutip: huruf kecil, angka, garis bawah.
assert_safe_dbname() {
  case "$1" in
    "" | *[!a-z0-9_]* | [0-9]*) die "Nama database tidak valid: '$1' (hanya a-z, 0-9, _; tidak diawali angka)." ;;
  esac
}

db_exists() {
  [ "$(psql_db postgres "SELECT 1 FROM pg_database WHERE datname = '$1'")" = "1" ]
}

# Versi migrasi goose terbaru yang berstatus applied pada sebuah database.
goose_version() {
  psql_db "$1" "SELECT COALESCE(max(version_id), 0) FROM goose_db_version WHERE is_applied"
}

# Hitung baris semua tabel/partisi-induk/matview di skema public (atau daftar
# VERIFY_TABLES, dipisah spasi). Keluaran: "nama|jumlah" per baris, terurut.
row_counts() {
  _db="$1"
  _filter=""
  if [ -n "${VERIFY_TABLES:-}" ]; then
    _filter="AND c.relname IN ($(printf '%s' "${VERIFY_TABLES}" | tr -s ' ' '\n' | sed "s/.*/'&'/" | paste -sd, -))"
  fi
  _query="$(psql_db "${_db}" "
    SELECT string_agg(format('SELECT %L AS t, count(*) AS n FROM public.%I', c.relname, c.relname),
                      ' UNION ALL ' ORDER BY c.relname)
    FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p', 'm') AND NOT c.relispartition ${_filter}")"
  [ -n "${_query}" ] || die "Tidak ada tabel yang dihitung di database ${_db}."
  psql_db "${_db}" "SELECT t || '|' || n FROM (${_query}) x ORDER BY t"
}

now_utc() { date -u +%Y%m%dT%H%M%SZ; }
