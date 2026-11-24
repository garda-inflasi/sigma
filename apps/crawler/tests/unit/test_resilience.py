"""Resilience tests: carried-forward lineage, idempotency, vintage updates, and isolation."""

from datetime import UTC, date, datetime, timedelta

import httpx

from garda_crawler.adapters.base import BaseAdapter
from garda_crawler.domain.entities import (
    DataPoint,
    DatasetDefinition,
    FetchOutcome,
    TimeSeriesSpec,
)
from garda_crawler.domain.enums import Frequency, SourceStatus
from garda_crawler.orchestration.runner import DatasetRunner
from garda_crawler.transport.time_utils import to_midnight_utc, to_wib_date
from tests.conftest import NOW_UTC, make_http_client

RUNNER = DatasetRunner()
DS = DatasetDefinition("uji", "raw_pangan_harian")
SERIES_A = TimeSeriesSpec("PIHPS", "uji", DS.collection, "CBM-KRT:1171", "harga", "Rp/kg", Frequency.DAILY, "CBM-KRT", "1171", "https://s/a")
SERIES_B = TimeSeriesSpec("PIHPS", "uji", DS.collection, "BWM:1171", "harga", "Rp/kg", Frequency.DAILY, "BWM", "1171", "https://s/b")
TOMORROW = NOW_UTC + timedelta(days=1)


class ScriptedAdapter(BaseAdapter):
    source_code = "PIHPS"

    def __init__(self, *results):
        super().__init__()
        self._results = results

    def list_datasets(self):
        return [DS]

    def expected_series(self, dataset):
        return [SERIES_A, SERIES_B]

    def fetch_daily(self, dataset, client, target_date):
        for item in self._results:
            if isinstance(item, Exception):
                raise item
            yield item

    def fetch_backfill(self, dataset, client, start_date, end_date):
        yield from self.fetch_daily(dataset, client, start_date)


def pt(series: TimeSeriesSpec, day: date, value: float) -> DataPoint:
    return DataPoint(series=series, period=day, value=value, source_url=series.source_url, attributes={})


def run_scripted(repo, *results, now_utc=NOW_UTC, trigger="MANUAL", **kwargs):
    def no_network(_):
        raise AssertionError("scripted adapter must not touch network")

    with make_http_client(no_network) as client:
        return RUNNER.run_adapter(ScriptedAdapter(*results), client, repo, trigger, now_utc, **kwargs)[0]


def raw_col(repo):
    return repo.db["raw_pangan_harian"]


def test_mongo_document_contract_fields(repository):
    run_scripted(repository, FetchOutcome(points=[pt(SERIES_A, date(2026, 11, 23), 50000)], request_count=1))
    doc = raw_col(repository).find_one({"kunci_seri": "CBM-KRT:1171"})
    assert doc["_id"] == "PIHPS:uji:CBM-KRT:1171:2026-11-23:2026-11-24"
    required = {
        "sumber", "dataset", "kunci_seri", "indikator", "komoditas_kode", "kabupaten_kode",
        "satuan", "frekuensi", "periode", "nilai", "is_carried_forward", "last_available_date",
        "crawled_at", "crawl_date", "source_status", "run_id", "url_sumber", "status_validasi", "atribut",
    }
    assert required <= set(doc)
    assert doc["periode"] == datetime(2026, 11, 23, tzinfo=UTC)
    assert doc["crawl_date"] == "2026-11-24"
    assert doc["source_status"] == SourceStatus.FRESH and doc["is_carried_forward"] is False
    assert doc["run_id"] in {r["_id"] for r in repository.db["crawl_runs"].find()}


def test_crawl_runs_ledger_contract(repository):
    run_scripted(repository, FetchOutcome(points=[pt(SERIES_A, date(2026, 11, 23), 50000)], request_count=1))
    run = repository.db["crawl_runs"].find_one()
    assert {
        "_id", "crawler", "sumber", "dataset", "crawl_date", "mulai", "selesai",
        "status", "jumlah_baru", "jumlah_diteruskan", "last_available_date", "pesan", "pemicu",
    } <= set(run)
    assert run["status"] == SourceStatus.FRESH and run["jumlah_baru"] == 1


def test_empty_source_writes_carried_forward_preserving_last_date(repository):
    run_scripted(repository, FetchOutcome(points=[pt(SERIES_A, date(2026, 11, 23), 50000), pt(SERIES_B, date(2026, 11, 23), 40000)], request_count=1))
    summary = run_scripted(repository, FetchOutcome(request_count=1), now_utc=TOMORROW)
    assert summary.status == SourceStatus.STALE_CARRIED_FORWARD
    assert summary.carried_forward_count == 2

    cf_docs = list(raw_col(repository).find({"is_carried_forward": True}))
    assert len(cf_docs) == 2
    for doc in cf_docs:
        assert doc["source_status"] == SourceStatus.STALE_CARRIED_FORWARD
        assert doc["periode"] == to_midnight_utc(to_wib_date(TOMORROW))
        assert doc["last_available_date"] == datetime(2026, 11, 23, tzinfo=UTC)
        assert doc["crawl_date"] == "2026-11-25"
        assert doc["crawled_at"] == TOMORROW
    assert {doc["nilai"] for doc in cf_docs} == {50000, 40000}


def test_error_carries_forward_history_and_marks_missing_null(repository):
    run_scripted(repository, FetchOutcome(points=[pt(SERIES_A, date(2026, 11, 23), 50000)], request_count=1))
    summary = run_scripted(repository, RuntimeError("connection reset"), now_utc=TOMORROW)
    assert summary.status == SourceStatus.ERROR
    assert "DILEWATI" not in summary.message
    a = raw_col(repository).find_one({"kunci_seri": "CBM-KRT:1171", "crawl_date": "2026-11-25"})
    assert a["is_carried_forward"] is True and a["nilai"] == 50000 and a["source_status"] == SourceStatus.STALE_CARRIED_FORWARD
    b = raw_col(repository).find_one({"kunci_seri": "BWM:1171", "crawl_date": "2026-11-25"})
    assert b["nilai"] is None and b["source_status"] == SourceStatus.ERROR and b["is_carried_forward"] is False


def test_skipped_remains_error_even_if_earlier_points_exist_today(repository):
    run_scripted(repository, FetchOutcome(points=[pt(SERIES_A, date(2026, 11, 23), 50000)], request_count=1))
    summary = run_scripted(repository, FetchOutcome(skipped_reason="DILEWATI: missing key; tindakan: fill key"))
    assert summary.status == SourceStatus.ERROR
    assert summary.message.startswith("DILEWATI: ")
    assert raw_col(repository).count_documents({"source_status": SourceStatus.FRESH}) == 1


def test_idempotent_reruns_and_vintage_updates(repository):
    make_outcome = lambda: FetchOutcome(points=[pt(SERIES_A, date(2026, 11, 23), 50000)], request_count=1)
    run_scripted(repository, make_outcome())
    count = raw_col(repository).count_documents({})
    rerun = run_scripted(repository, make_outcome())
    assert raw_col(repository).count_documents({}) == count
    assert repository.db["crawl_runs"].count_documents({}) == 1
    assert rerun.status == SourceStatus.FRESH

    # Unchanged old period on next day does not create a new vintage.
    same_old = run_scripted(repository, make_outcome(), now_utc=TOMORROW)
    assert raw_col(repository).count_documents({"is_carried_forward": False, "nilai": {"$ne": None}}) == 1
    assert same_old.status == SourceStatus.STALE_CARRIED_FORWARD

    # Revised value on next day writes a new vintage.
    revised = run_scripted(repository, FetchOutcome(points=[pt(SERIES_A, date(2026, 11, 23), 51000)], request_count=1), now_utc=TOMORROW)
    assert revised.status == SourceStatus.FRESH
    vintages = {
        d["crawl_date"]: d["nilai"]
        for d in raw_col(repository).find({"is_carried_forward": False, "nilai": {"$ne": None}})
    }
    assert vintages == {"2026-11-24": 50000, "2026-11-25": 51000}


def test_late_release_replaces_same_day_carried_forward(repository):
    run_scripted(repository, FetchOutcome(points=[pt(SERIES_A, date(2026, 11, 23), 50000)], request_count=1))
    run_scripted(repository, FetchOutcome(request_count=1), now_utc=TOMORROW)
    assert raw_col(repository).count_documents({"is_carried_forward": True}) == 1
    late = run_scripted(repository, FetchOutcome(points=[pt(SERIES_A, date(2026, 11, 24), 52000)], request_count=1), now_utc=TOMORROW)
    assert late.status == SourceStatus.FRESH
    assert raw_col(repository).count_documents({"is_carried_forward": True}) == 0


def test_backfill_skips_carried_forward_and_is_not_counted_as_daily_fresh(repository):
    bf = run_scripted(
        repository,
        FetchOutcome(points=[pt(SERIES_A, date(2026, 1, 5), 45000)], request_count=1),
        trigger="BACKFILL",
        start_date=date(2026, 1, 1),
        end_date=date(2026, 1, 31),
    )
    assert bf.status == SourceStatus.FRESH and bf.new_count == 1
    assert raw_col(repository).count_documents({"is_carried_forward": True}) == 0
    assert raw_col(repository).find_one()["atribut"]["backfill"] is True

    daily = run_scripted(repository, FetchOutcome(request_count=1))
    assert daily.status == SourceStatus.STALE_CARRIED_FORWARD
    cf = raw_col(repository).find_one({"is_carried_forward": True})
    assert cf["last_available_date"] == datetime(2026, 1, 5, tzinfo=UTC)


def test_adapter_failure_isolation(repository):
    class BrokenAdapter(ScriptedAdapter):
        source_code = "BMKG"

        def list_datasets(self):
            return [DatasetDefinition("broken", "raw_iklim_bmkg")]

        def expected_series(self, dataset):
            return []

    with make_http_client(lambda _: httpx.Response(500)) as client:
        failed_run = RUNNER.run_adapter(BrokenAdapter(RuntimeError("boom")), client, repository, "JADWAL", NOW_UTC)[0]
        ok_run = RUNNER.run_adapter(
            ScriptedAdapter(FetchOutcome(points=[pt(SERIES_A, date(2026, 11, 23), 50000)], request_count=1)),
            client,
            repository,
            "JADWAL",
            NOW_UTC,
        )[0]

    assert failed_run.status == SourceStatus.ERROR
    assert ok_run.status == SourceStatus.FRESH
    assert repository.db["crawl_runs"].count_documents({}) == 2
