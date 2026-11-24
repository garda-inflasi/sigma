"""Field entry PostgreSQL-to-MongoDB sync tests."""

from datetime import UTC, datetime, timedelta

import psycopg

from garda_crawler.adapters.field_entries import sync_field_entries
from garda_crawler.domain.enums import VALIDATION_STATUS_WEIGHT, SourceStatus
from garda_crawler.transport.postgres_reader import INDEX_SQL, ROWS_SQL
from tests.conftest import NOW_UTC

RECORDED_AT = datetime(2026, 11, 23, 2, 0, tzinfo=UTC)
PII_COLUMNS = {
    "petugas_id",
    "divalidasi_oleh",
    "dipublikasi_oleh",
    "ditolak_oleh",
    "lat",
    "lng",
    "foto_url",
    "catatan",
    "kode_pedagang",
    "idempotency_key",
}


def make_entry(entry_id: int, status: str = "DIAJUKAN", version: int = 1, price: float = 50000, recorded_at: datetime = RECORDED_AT, **extra):
    return {
        "id": entry_id,
        "komoditas_kode": "CBM-KRT",
        "pasar_id": "PSR-1171-01",
        "kabupaten_kode": "1171",
        "tipe": "KONSUMEN",
        "harga": price,
        "satuan": "Kg",
        "kualitas": "MEDIUM",
        "status": status,
        "deviasi_pct": 1.5,
        "skor_anomali": 0.1,
        "dicatat_pada": recorded_at,
        "satuan_master": "kg",
        "harga_acuan": 52000,
        "versi": version,
        **extra,
    }


class FakeFieldReader:
    """In-memory test double mimicking read-only PostgresFieldReader."""

    def __init__(self, rows=(), demo_count: int = 0, error: Exception | None = None):
        self.data = {row["id"]: row for row in rows}
        self.demo_count = demo_count
        self.error = error
        self.fetched_ids: list[int] = []

    def __enter__(self):
        if self.error:
            raise self.error
        return self

    def __exit__(self, *args):
        return None

    def _publishable(self):
        return [row for row in self.data.values() if row["status"] in VALIDATION_STATUS_WEIGHT]

    def fetch_index(self):
        return [{"id": row["id"], "versi": row["versi"], "dicatat_pada": row["dicatat_pada"]} for row in self._publishable()]

    def fetch_rows_by_ids(self, entry_ids):
        self.fetched_ids.extend(entry_ids)
        return [row for row in self._publishable() if row["id"] in entry_ids]

    def count_demo_entries(self):
        return self.demo_count


def _originals(repo):
    return list(repo.db["raw_pangan_harian"].find({"sumber": "LAPANGAN", "is_carried_forward": False}))


def test_sync_excludes_pii_and_follows_contract(repository):
    row = make_entry(1, petugas_id="uuid-petugas", lat=5.5, lng=95.3, catatan="nama pedagang", kode_pedagang="PDG-1")
    outcome = sync_field_entries(FakeFieldReader([row]), repository, NOW_UTC, "JADWAL")
    assert outcome.status == SourceStatus.FRESH
    doc = _originals(repository)[0]
    assert doc["sumber"] == "LAPANGAN"
    assert doc["status_validasi"] == "DIAJUKAN"
    assert doc["komoditas_kode"] == "CBM-KRT" and doc["kabupaten_kode"] == "1171"
    assert doc["satuan"] == "Rp/kg" and doc["nilai"] == 50000
    assert doc["periode"] == datetime(2026, 11, 23, tzinfo=UTC)
    assert doc["kunci_seri"] == "CBM-KRT:1171:KONSUMEN:E1"
    assert doc["_id"] == "LAPANGAN:entri_harga:CBM-KRT:1171:KONSUMEN:E1:2026-11-23:2026-11-24"
    assert not PII_COLUMNS & (set(doc) | set(doc["atribut"]))
    assert "uuid-petugas" not in str(doc) and "nama pedagang" not in str(doc)


def test_sql_excludes_demo_rejected_draft_and_pii():
    for sql in (INDEX_SQL, ROWS_SQL):
        assert "'DIAJUKAN', 'DIVALIDASI', 'DIPUBLIKASI'" in sql
        assert "DITOLAK" not in sql and "DRAF" not in sql
        assert "!~ '^HRG-D[0-9]'" in sql
        for col in PII_COLUMNS:
            assert col not in sql


def test_period_uses_wib_business_date(repository):
    sync_field_entries(
        FakeFieldReader([make_entry(1, recorded_at=datetime(2026, 11, 22, 18, 0, tzinfo=UTC))]),
        repository,
        NOW_UTC,
        "JADWAL",
    )
    assert _originals(repository)[0]["periode"] == datetime(2026, 11, 23, tzinfo=UTC)


def test_incremental_sync_is_idempotent(repository):
    reader = FakeFieldReader([make_entry(1), make_entry(2)])
    sync_field_entries(reader, repository, NOW_UTC, "JADWAL")
    assert sorted(reader.fetched_ids) == [1, 2]
    count = repository.db["raw_pangan_harian"].count_documents({})
    reader.fetched_ids.clear()
    second = sync_field_entries(reader, repository, NOW_UTC, "JADWAL")
    assert reader.fetched_ids == []
    assert second.written_count == 0
    assert repository.db["raw_pangan_harian"].count_documents({}) == count


def test_status_updates_rejected_draft_and_deletions(repository):
    reader = FakeFieldReader([make_entry(1, "DIAJUKAN", version=1)])
    sync_field_entries(reader, repository, NOW_UTC, "JADWAL")
    doc_id = _originals(repository)[0]["_id"]

    for status, version in (("DIVALIDASI", 2), ("DIPUBLIKASI", 3)):
        reader.data[1] = make_entry(1, status, version=version)
        sync_field_entries(reader, repository, NOW_UTC + timedelta(days=1), "JADWAL")
        docs = _originals(repository)
        assert len(docs) == 1 and docs[0]["_id"] == doc_id
        assert docs[0]["status_validasi"] == status

    reader.data[1] = make_entry(1, "DITOLAK", version=4)
    rm = sync_field_entries(reader, repository, NOW_UTC + timedelta(days=1, hours=1), "JADWAL")
    assert rm.deleted_count == 1
    assert repository.db["raw_pangan_harian"].count_documents({"sumber": "LAPANGAN"}) == 0


def test_carried_forward_picks_highest_validated_entry_and_handles_pg_failure(repository):
    rows = [
        make_entry(1, "DIPUBLIKASI", price=50000, recorded_at=RECORDED_AT - timedelta(days=1)),
        make_entry(2, "DIAJUKAN", price=51000),
        make_entry(3, "DIPUBLIKASI", price=52000),
    ]
    sync_field_entries(FakeFieldReader(rows), repository, NOW_UTC, "JADWAL")
    cf = list(repository.db["raw_pangan_harian"].find({"is_carried_forward": True}))
    assert len(cf) == 1 and cf[0]["nilai"] == 52000 and cf[0]["status_validasi"] == "DIPUBLIKASI"

    tomorrow = NOW_UTC + timedelta(days=1)
    failed = sync_field_entries(
        FakeFieldReader(error=psycopg.OperationalError("secret host")),
        repository,
        tomorrow,
        "JADWAL",
    )
    assert failed.status == SourceStatus.ERROR
    assert "DILEWATI" not in failed.message
    assert "secret host" not in failed.message
    assert repository.db["raw_pangan_harian"].count_documents({"is_carried_forward": True, "crawl_date": "2026-11-25"}) == 1


def test_missing_postgres_config_and_demo_seed_only(repository):
    skipped = sync_field_entries(None, repository, NOW_UTC, "JADWAL")
    assert skipped.status == SourceStatus.ERROR
    assert skipped.message.startswith("DILEWATI: ")
    assert "CRAWLER_DATABASE_URL" in skipped.message

    demo = sync_field_entries(FakeFieldReader(demo_count=32289), repository, NOW_UTC, "JADWAL")
    assert demo.status == SourceStatus.STALE_CARRIED_FORWARD
    assert "32289 entri seed demo diabaikan" in demo.message
