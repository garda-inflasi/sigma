"""Read-only PostgreSQL reader for internal field price entries."""

from collections.abc import Iterable
from typing import Any

import psycopg
from psycopg.rows import dict_row

from garda_crawler.domain.enums import VALIDATION_STATUS_WEIGHT

# Demo entries use HRG-D* prefix and must remain excluded from production unseen collections.
DEMO_ENTRY_REGEX = "^HRG-D[0-9]"

_ACTIVE_STATUSES_SQL = "', '".join(VALIDATION_STATUS_WEIGHT.keys())

_FILTER_CLAUSE = f"""
    e.sumber = 'LAPANGAN'
    AND e.status IN ('{_ACTIVE_STATUSES_SQL}')
    AND (e.kode IS NULL OR e.kode !~ '{DEMO_ENTRY_REGEX}')
"""

_VERSION_JOIN = (
    "LEFT JOIN LATERAL ("
    "SELECT max(ev.versi) AS versi FROM entri_harga_versi ev WHERE ev.entri_id = e.id"
    ") v ON TRUE"
)

INDEX_SQL = f"SELECT e.id, COALESCE(v.versi, 0) AS versi, e.dicatat_pada FROM entri_harga e {_VERSION_JOIN} WHERE {_FILTER_CLAUSE}"

ROWS_SQL = f"""
SELECT
    e.id,
    e.komoditas_kode,
    e.pasar_id,
    p.kabupaten_kode,
    e.tipe::text AS tipe,
    e.harga::double precision AS harga,
    e.satuan,
    e.kualitas::text AS kualitas,
    e.status::text AS status,
    e.deviasi_pct::double precision AS deviasi_pct,
    e.skor_anomali::double precision AS skor_anomali,
    e.dicatat_pada,
    k.satuan AS satuan_master,
    COALESCE(k.harga_acuan::double precision, 0) AS harga_acuan,
    COALESCE(v.versi, 0) AS versi
FROM entri_harga e
JOIN pasar p ON p.kode = e.pasar_id
JOIN komoditas k ON k.kode = e.komoditas_kode
{_VERSION_JOIN}
WHERE e.id = ANY(%s) AND {_FILTER_CLAUSE}
"""

COUNT_DEMO_SQL = f"""
SELECT count(*) AS jumlah
FROM entri_harga
WHERE kode ~ '{DEMO_ENTRY_REGEX}'
"""


class PostgresFieldReader:
    """Strictly read-only PostgreSQL snapshot reader excluding PII and demo rows."""

    def __init__(self, dsn: str) -> None:
        self._dsn = dsn
        self._connection: psycopg.Connection[Any] | None = None

    def __enter__(self) -> "PostgresFieldReader":
        self._connection = psycopg.connect(
            self._dsn,
            connect_timeout=10,
            application_name="garda-crawler",
            row_factory=dict_row,
        )
        self._connection.read_only = True
        self._connection.isolation_level = psycopg.IsolationLevel.REPEATABLE_READ
        return self

    def __exit__(self, *args: object) -> None:
        if self._connection is not None:
            self._connection.rollback()
            self._connection.close()
            self._connection = None

    def _require_connection(self) -> psycopg.Connection[Any]:
        if self._connection is None:
            raise RuntimeError("PostgresFieldReader belum dibuka dengan context manager")
        return self._connection

    def fetch_index(self) -> list[dict[str, Any]]:
        """Return `(id, versi, dicatat_pada)` tuples for incremental diff detection."""
        return list(self._require_connection().execute(INDEX_SQL).fetchall())

    def fetch_rows_by_ids(self, entry_ids: Iterable[int]) -> list[dict[str, Any]]:
        """Fetch sanitized field price rows matching given integer primary keys."""
        ids = list(entry_ids)
        if not ids:
            return []
        return list(self._require_connection().execute(ROWS_SQL, (ids,)).fetchall())

    def count_demo_entries(self) -> int:
        """Count seeded demo entries ignored during production sync."""
        row = self._require_connection().execute(COUNT_DEMO_SQL).fetchone()
        return int(row["jumlah"]) if row else 0
