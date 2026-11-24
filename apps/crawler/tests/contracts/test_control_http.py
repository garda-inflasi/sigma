"""Contract tests for internal HTTP control server (:8091), scheduling locks, and CLI."""

import json
import urllib.error
import urllib.request
from datetime import timedelta

import pytest

from garda_crawler.domain.enums import SourceStatus
from garda_crawler.entrypoints.cli import main as cli_main
from garda_crawler.orchestration.control_server import (
    CrawlerControlState,
    find_last_failed_sources,
    start_control_server_background,
)
from garda_crawler.orchestration.scheduler import is_heartbeat_healthy, run_if_due, write_heartbeat
from garda_crawler.storage.lock_manager import MongoLeaseLock
from tests.conftest import NOW_UTC


def test_control_http_status_run_and_double_run_lock(db):
    state = CrawlerControlState(db)
    triggered: list[tuple[list[str], bool, str]] = []

    def fake_executor(st, sources, failed_only, job_id):
        triggered.append((sources, failed_only, job_id))

    server = start_control_server_background(state, port=0, run_executor=fake_executor)
    port = server.server_address[1]
    base_url = f"http://127.0.0.1:{port}"
    try:
        with urllib.request.urlopen(f"{base_url}/health") as resp:
            assert resp.status == 200
            assert json.loads(resp.read().decode("utf-8")) == {"ok": True}

        with urllib.request.urlopen(f"{base_url}/status") as resp:
            status_body = json.loads(resp.read().decode("utf-8"))
            assert status_body["berjalan"] is False
            assert set(status_body["jadwal"]) == {"harian", "lapangan"}
            assert {"job_id", "sumber", "gagal_saja", "tahap", "progress_pct", "logs", "hasil", "slot_terakhir"} <= set(status_body)

        req = urllib.request.Request(
            f"{base_url}/run",
            data=json.dumps({"sumber": ["PIHPS", "BMKG"], "gagal_saja": False, "job_id": "j-1"}).encode("utf-8"),
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        with urllib.request.urlopen(req) as resp:
            assert resp.status == 202
            accepted = json.loads(resp.read().decode("utf-8"))
            assert accepted["accepted"] is True
            assert accepted["state"]["berjalan"] is True

        with pytest.raises(urllib.error.HTTPError) as err:
            urllib.request.urlopen(req)
        assert err.value.code == 409
        conflict = json.loads(err.value.read().decode("utf-8"))
        assert conflict["error"] == "SEDANG_BERJALAN"
        assert triggered == [(["PIHPS", "BMKG"], False, "j-1")]
    finally:
        server.shutdown()
        server.server_close()


def test_retry_failed_filters_out_skipped_sources(db):
    db["crawl_runs"].insert_many(
        [
            {"_id": "1", "sumber": "PIHPS", "dataset": "d1", "status": SourceStatus.ERROR, "pesan": "HTTP 503", "selesai": NOW_UTC},
            {"_id": "2", "sumber": "BPS", "dataset": "d2", "status": SourceStatus.ERROR, "pesan": "DILEWATI: missing key; tindakan: fill", "selesai": NOW_UTC},
            {"_id": "3", "sumber": "BMKG", "dataset": "d3", "status": SourceStatus.FRESH, "pesan": "baru 3", "selesai": NOW_UTC},
        ]
    )
    assert find_last_failed_sources(db) == ["PIHPS"]


def test_schedule_lock_and_heartbeat_cli(db, tmp_path, monkeypatch):
    executed: list[str] = []
    assert run_if_due(db, "harian", "2026-11-24", lambda: executed.append("first"), "owner-a")
    assert not run_if_due(db, "harian", "2026-11-24", lambda: executed.append("second"), "owner-b")
    assert executed == ["first"]

    lock_a = MongoLeaseLock(db, "k1", owner="a")
    lock_b = MongoLeaseLock(db, "k1", owner="b")
    assert lock_a.acquire()
    assert not lock_b.acquire()
    lock_a.release()
    assert lock_b.acquire()

    hb = tmp_path / "heartbeat"
    assert not is_heartbeat_healthy(hb)
    write_heartbeat(hb, NOW_UTC)
    assert is_heartbeat_healthy(hb, max_age_seconds=900, now_utc=NOW_UTC + timedelta(minutes=5))
    assert not is_heartbeat_healthy(hb, max_age_seconds=900, now_utc=NOW_UTC + timedelta(minutes=30))

    monkeypatch.setenv("MONGO_URI", "mongodb://localhost:27017")
    monkeypatch.setenv("CRAWLER_BERKAS_DETAK", str(hb))
    write_heartbeat(hb)
    assert cli_main(["health"]) == 0
