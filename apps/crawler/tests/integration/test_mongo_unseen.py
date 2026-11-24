"""Real MongoDB integration tests (enabled when GARDA_UJI_MONGO_URI is set)."""

import os
import uuid
from datetime import date, timedelta

import pytest
from pymongo import MongoClient

from garda_crawler.adapters.field_entries import sync_field_entries
from garda_crawler.domain.entities import FetchOutcome
from garda_crawler.domain.enums import RAW_COLLECTIONS, SourceStatus
from garda_crawler.orchestration.scheduler import run_if_due
from garda_crawler.storage.lock_manager import MongoLeaseLock
from garda_crawler.storage.unseen_repository import UnseenRepository
from tests.conftest import NOW_UTC
from tests.integration.test_field_sync import FakeFieldReader, make_entry
from tests.unit.test_resilience import SERIES_A, SERIES_B, pt, run_scripted

URI = os.environ.get("GARDA_UJI_MONGO_URI")
pytestmark = [pytest.mark.integrasi, pytest.mark.skipif(not URI, reason="GARDA_UJI_MONGO_URI kosong")]


@pytest.fixture
def live_repository():
    client = MongoClient(URI, tz_aware=True, serverSelectionTimeoutMS=5000)
    db_name = f"uji_{uuid.uuid4().hex[:10]}"
    repo = UnseenRepository(client[db_name])
    repo.ensure_indexes()
    yield repo
    client.drop_database(db_name)
    client.close()


def test_contract_indexes_created(live_repository):
    for col_name in RAW_COLLECTIONS:
        keys = [tuple(k for k, _ in idx["key"].items()) for idx in live_repository.db[col_name].list_indexes()]
        assert ("kunci_seri", "periode") in keys
        assert ("sumber", "dataset", "crawl_date") in keys
        assert ("komoditas_kode", "kabupaten_kode", "periode") in keys
        assert ("source_status", "crawl_date") in keys


def test_carried_forward_idempotency_on_real_mongo(live_repository):
    col = live_repository.db["raw_pangan_harian"]
    initial = FetchOutcome(points=[pt(SERIES_A, date(2026, 11, 23), 50000), pt(SERIES_B, date(2026, 11, 23), 40000)], request_count=1)
    assert run_scripted(live_repository, initial).status == SourceStatus.FRESH
    tomorrow = NOW_UTC + timedelta(days=1)
    for _ in range(2):
        assert run_scripted(live_repository, FetchOutcome(request_count=1), now_utc=tomorrow).status == SourceStatus.STALE_CARRIED_FORWARD
    assert col.count_documents({"is_carried_forward": True}) == 2
    assert col.count_documents({}) == 4


def test_field_sync_status_on_real_mongo(live_repository):
    reader = FakeFieldReader([make_entry(1, "DIAJUKAN"), make_entry(2, "DIVALIDASI")])
    sync_field_entries(reader, live_repository, NOW_UTC, "JADWAL")
    reader.data[1] = make_entry(1, "DITOLAK", version=2)
    reader.data[2] = make_entry(2, "DIPUBLIKASI", version=2)
    sync_field_entries(reader, live_repository, NOW_UTC, "JADWAL")
    docs = list(live_repository.db["raw_pangan_harian"].find({"is_carried_forward": False}))
    assert [(d["atribut"]["entri_id"], d["status_validasi"]) for d in docs] == [(2, "DIPUBLIKASI")]


def test_lease_lock_mutual_exclusion(live_repository):
    db = live_repository.db
    a, b = MongoLeaseLock(db, "uji", "a"), MongoLeaseLock(db, "uji", "b")
    assert a.acquire()
    assert not b.acquire()
    a.release()
    assert b.acquire()


def test_schedule_slot_idempotent(live_repository):
    ran: list[str] = []
    assert run_if_due(live_repository.db, "harian", "2026-11-24", lambda: ran.append("a"), "a")
    assert not run_if_due(live_repository.db, "harian", "2026-11-24", lambda: ran.append("b"), "b")
    assert ran == ["a"]
