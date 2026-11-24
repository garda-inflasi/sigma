"""Unit tests for Medallion Bronze storage, SHA-256 idempotency, BPS IHK150 (2024-2026), DJPK, and async HTTP."""

import asyncio
import gzip
import json
from datetime import date
from pathlib import Path

import httpx
import pytest

from garda_crawler.adapters.bps import (
    BPS_IHK150_TABLE_TOKEN,
    BpsWebApiAdapter,
    decode_bps_table_token,
)
from garda_crawler.adapters.djpk import DjpkKemenkeuAdapter
from garda_crawler.adapters.registry import build_default_registry
from garda_crawler.config.settings import CrawlerSettings
from garda_crawler.config.source_specs import BPS_DATASET_SPECS, DJPK_APBD_DATASET
from garda_crawler.domain.entities import RawBronzeCapture
from garda_crawler.domain.enums import SourceStatus
from garda_crawler.orchestration.runner import DatasetRunner
from garda_crawler.storage.bronze_store import BronzeStore
from garda_crawler.storage.crawl_log_repository import InMemoryCrawlLogRepository
from garda_crawler.transport.polite_http import AsyncPoliteHttpClient
from tests.conftest import NOW_UTC, make_http_client, without_robots


class FakeS3Client:
    def __init__(self) -> None:
        self.buckets: set[str] = set()
        self.objects: dict[tuple[str, str], bytes] = {}
        self.put_calls = 0

    def head_bucket(self, Bucket: str) -> None:
        if Bucket not in self.buckets:
            raise FileNotFoundError(Bucket)

    def create_bucket(self, Bucket: str) -> None:
        self.buckets.add(Bucket)

    def head_object(self, Bucket: str, Key: str) -> None:
        if (Bucket, Key) not in self.objects:
            raise FileNotFoundError(Key)

    def put_object(self, Bucket: str, Key: str, Body: bytes, **_kwargs) -> None:
        self.put_calls += 1
        self.objects[(Bucket, Key)] = Body


def test_bronze_store_rejects_apps_crawler_directory():
    crawler_pkg = Path(__file__).resolve().parents[2] / "src"
    with pytest.raises(ValueError, match="apps/crawler"):
        BronzeStore(local_dir=crawler_pkg / "raw_tmp")


def test_bronze_store_sha256_idempotency_and_minio_s3(tmp_path: Path):
    logs = InMemoryCrawlLogRepository()
    s3 = FakeS3Client()
    store = BronzeStore(minio_bucket="data-raw", crawl_logs=logs, s3_client=s3)

    capture = RawBronzeCapture(
        source_code="DJPK",
        dataset_name="apbd_pemda",
        endpoint="https://djpk.kemenkeu.go.id/portal/data/apbd",
        params={"periode": 12, "provinsi": "01", "pemda": "12", "tahun": 2025},
        payload={"data": [{"uraian": "Belanja Daerah", "realisasi": "1.250.000,50"}]},
        partition_year=2025,
        capture_date=date(2026, 11, 24),
    )

    first = store.persist_capture(capture, date(2026, 11, 24))
    assert first.status == "WRITTEN"
    assert first.uri.startswith("s3://data-raw/djpk/apbd_pemda/tahun=2025/dt=2026-11-24/")
    assert s3.put_calls == 1

    # Payload identical -> skip S3 write & record SKIPPED_DUPLICATE
    second = store.persist_capture(capture, date(2026, 11, 24))
    assert second.status == "SKIPPED_DUPLICATE"
    assert second.payload_sha256 == first.payload_sha256
    assert s3.put_calls == 1
    assert [e.status for e in logs.entries] == ["WRITTEN", "SKIPPED_DUPLICATE"]

    # Local compressed .json.gz verification
    local_logs = InMemoryCrawlLogRepository()
    local_store = BronzeStore(local_dir=tmp_path / "raw", crawl_logs=local_logs)
    local_res = local_store.persist_capture(capture, date(2026, 11, 24))
    assert local_res.status == "WRITTEN"
    loaded = json.loads(gzip.decompress(Path(local_res.uri).read_bytes()).decode("utf-8"))
    assert loaded == capture.payload


def test_bps_ihk150_multi_year_2024_2025_2026_and_bronze_capture(repository, tmp_path: Path):
    assert decode_bps_table_token(BPS_IHK150_TABLE_TOKEN) == ("2245", "2")
    ds_ihk150 = next(d for d in BPS_DATASET_SPECS if d.name == "ihk_150_kabkota")
    seen_years: list[str] = []

    def handler(req: httpx.Request) -> httpx.Response:
        parts = req.url.path.strip("/").split("/")
        th_val = int(parts[parts.index("th") + 1])
        year_label = str(1900 + th_val)
        seen_years.append(year_label)
        return httpx.Response(
            200,
            json={
                "status": "OK",
                "var": [{"val": 2245, "label": "Indeks Harga Konsumen 150 Kabupaten/Kota (2022=100)"}],
                "turvar": [{"val": "0", "label": "Tidak ada"}],
                "vervar": [
                    {"val": 1, "label": "KAB ACEH TENGAH"},
                    {"val": 2, "label": "MEULABOH"},
                    {"val": 3, "label": "KAB ACEH TAMIANG"},
                    {"val": 4, "label": "KOTA BANDA ACEH"},
                    {"val": 5, "label": "KOTA LHOKSEUMAWE"},
                    {"val": 151, "label": "INDONESIA"},
                ],
                "tahun": [{"val": th_val, "label": year_label}],
                "turtahun": [
                    {"val": 1, "label": "Januari"},
                    {"val": 9, "label": "September"},
                ],
                "datacontent": {
                    f"122450{th_val}1": 105.5 + (th_val - 124),
                    f"222450{th_val}1": 106.1,
                    f"322450{th_val}9": 107.2,
                    f"422450{th_val}9": 108.4,
                    f"522450{th_val}9": 106.9,
                    f"15122450{th_val}9": 107.8,
                },
            },
        )

    logs = InMemoryCrawlLogRepository()
    bronze = BronzeStore(local_dir=tmp_path / "raw", crawl_logs=logs)
    adapter = BpsWebApiAdapter(api_key="rahasia_bps", regions=("0000", "1106", "1107", "1114", "1171", "1174"))
    runner = DatasetRunner()

    with make_http_client(without_robots(handler)) as client:
        summary = runner.run_dataset(
            adapter,
            ds_ihk150,
            client,
            repository,
            NOW_UTC,
            "BACKFILL",
            start_date=date(2024, 1, 1),
            end_date=date(2026, 9, 30),
            bronze_store=bronze,
        )

    assert seen_years == ["2024", "2025", "2026"]
    assert summary.status == SourceStatus.FRESH
    assert summary.new_count == 18  # 6 regions x 3 years
    assert len(logs.entries) == 3
    assert all(e.status == "WRITTEN" and "rahasia_bps" not in e.endpoint for e in logs.entries)
    assert any("tahun=2024" in e.bronze_uri for e in logs.entries)
    assert any("tahun=2026" in e.bronze_uri for e in logs.entries)


def test_djpk_adapter_and_registry(repository, tmp_path: Path):
    settings = CrawlerSettings(mongo_uri="mongodb://contoh:27017", bronze_dir=str(tmp_path / "raw"))
    registry = build_default_registry()
    assert "DJPK" in registry.all_source_codes()
    adapter = registry.create("DJPK", settings, regions=("1101",))
    assert isinstance(adapter, DjpkKemenkeuAdapter)

    def handler(req: httpx.Request) -> httpx.Response:
        assert req.url.params["provinsi"] == "01"
        assert req.url.params["pemda"] == "12"
        return httpx.Response(200, json={"data": [{"uraian": "Belanja Pegawai", "realisasi": "450.250.000,75"}]})

    logs = InMemoryCrawlLogRepository()
    bronze = BronzeStore(local_dir=tmp_path / "raw", crawl_logs=logs)
    with make_http_client(without_robots(handler)) as client:
        summary = DatasetRunner().run_dataset(
            adapter,
            DJPK_APBD_DATASET,
            client,
            repository,
            NOW_UTC,
            "MANUAL",
            bronze_store=bronze,
        )
    assert summary.status == SourceStatus.FRESH
    assert summary.new_count == 2  # year-1 & year
    assert len(logs.entries) == 2


def test_async_polite_http_client():
    def handler(req: httpx.Request) -> httpx.Response:
        if req.url.path == "/robots.txt":
            return httpx.Response(404)
        return httpx.Response(200, json={"ok": True, "tahun": 2026})

    async def _run() -> dict:
        async with AsyncPoliteHttpClient("garda-test", delay_ms=0, transport=httpx.MockTransport(handler)) as client:
            return await client.get_json("https://contoh.go.id/api/data")

    assert asyncio.run(_run()) == {"ok": True, "tahun": 2026}
