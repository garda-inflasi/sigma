"""Unit tests for the 70 BPS statistical table crawler, rowspan/colspan parser, and Aceh whitelist."""

from datetime import date
from pathlib import Path

import httpx

from garda_crawler.adapters.bps_html_table import (
    extract_bps_table_from_response,
    parse_bps_numeric,
    parse_html_table_records,
)
from garda_crawler.adapters.bps_tables import BpsStatTablesAdapter
from garda_crawler.adapters.registry import build_default_registry
from garda_crawler.config.bps_table_catalog import BPS_70_TABLES
from garda_crawler.config.reference_catalog import match_aceh_bps_region
from garda_crawler.config.settings import CrawlerSettings
from garda_crawler.domain.enums import SourceStatus
from garda_crawler.orchestration.runner import DatasetRunner
from garda_crawler.storage.bronze_store import BronzeStore
from garda_crawler.storage.crawl_log_repository import InMemoryCrawlLogRepository
from tests.conftest import NOW_UTC, make_http_client, without_robots


def test_catalog_contains_exact_70_unique_sources():
    assert len(BPS_70_TABLES) == 70
    assert [spec.index for spec in BPS_70_TABLES] == list(range(1, 71))
    assert len({spec.url for spec in BPS_70_TABLES}) == 70
    assert len({spec.dataset_slug for spec in BPS_70_TABLES}) == 70
    assert BPS_70_TABLES[68].table_type == 3 and "year=2025" in BPS_70_TABLES[68].url
    assert BPS_70_TABLES[69].table_type == 1 and BPS_70_TABLES[69].aceh_specific_rows is True


def test_aceh_whitelist_and_numeric_normalization():
    assert match_aceh_bps_region("PROV. ACEH") == ("11", "Provinsi Aceh")
    assert match_aceh_bps_region("Provinsi Aceh") == ("11", "Provinsi Aceh")
    assert match_aceh_bps_region("KAB ACEH TENGAH") == ("1106", "Kab. Aceh Tengah")
    assert match_aceh_bps_region("Kabupaten   Aceh Besar") == ("1108", "Kab. Aceh Besar")
    assert match_aceh_bps_region("KOTA BANDA ACEH") == ("1171", "Kota Banda Aceh")
    assert match_aceh_bps_region("MEULABOH") == ("1107", "Kab. Aceh Barat")
    assert match_aceh_bps_region("Simeulue", 1101000) == ("1101", "Kab. Simeulue")

    # Di luar Provinsi Aceh wajib diabaikan
    for non_aceh in ("INDONESIA", "SUMATERA UTARA", "KOTA MEDAN", "DKI JAKARTA", "KAB. BANDUNG"):
        assert match_aceh_bps_region(non_aceh) is None

    assert parse_bps_numeric("117,80") == 117.8
    assert parse_bps_numeric("1.234,56") == 1234.56
    assert parse_bps_numeric("-") is None
    assert parse_bps_numeric("...") is None
    assert parse_bps_numeric("") is None


def test_multilevel_html_table_rowspan_colspan_and_url70_aceh_exception():
    html_sample = """
    <html><body>
      <table>
        <thead>
          <tr>
            <th rowspan="3">Provinsi / Kabupaten / Kota</th>
            <th colspan="3">Indeks Harga Konsumen</th>
          </tr>
          <tr>
            <th colspan="2">2026</th>
            <th rowspan="2">2025</th>
          </tr>
          <tr>
            <th>Januari</th>
            <th>Februari</th>
          </tr>
        </thead>
        <tbody>
          <tr><td>KOTA MEDAN</td><td>110,10</td><td>111,20</td><td>109,00</td></tr>
          <tr><td>MEULABOH</td><td>117,80</td><td>-</td><td>114,15</td></tr>
          <tr><td>KOTA BANDA ACEH</td><td>115,25</td><td>...</td><td>112,40</td></tr>
        </tbody>
      </table>
    </body></html>
    """
    records, total_rows = parse_html_table_records(html_sample, BPS_70_TABLES[0], "2026-11-24T14:30:00+00:00")
    assert total_rows == 3
    assert {r.kode_bps for r in records} == {"1107", "1171"}  # Medan tersaring keluar
    assert len(records) == 6
    meulaboh_jan = next(r for r in records if r.kode_bps == "1107" and r.periode == "Januari")
    meulaboh_feb = next(r for r in records if r.kode_bps == "1107" and r.periode == "Februari")
    assert meulaboh_jan.tahun == 2026 and meulaboh_jan.nilai == 117.8
    assert meulaboh_feb.nilai is None

    # URL #70: baris komoditi khusus Aceh -> seluruh baris komoditi masuk dengan provinsi="Aceh"
    html_70 = """
    <table>
      <tbody>
        <tr><td rowspan="2">Komoditi Makanan</td><td colspan="2">2025</td></tr>
        <tr><td>Konsumsi</td><td>Pengeluaran</td></tr>
        <tr><td>Beras Lokal</td><td>1,45</td><td>18.500,50</td></tr>
      </tbody>
    </table>
    """
    recs_70, _ = parse_html_table_records(html_70, BPS_70_TABLES[69], "2026-11-24T14:30:00+00:00")
    assert len(recs_70) == 2
    assert all(r.wilayah == "Aceh" and r.kode_bps == "11" and r.komoditi == "Beras Lokal" for r in recs_70)
    assert recs_70[0].nilai == 1.45 and recs_70[1].nilai == 18500.5


def test_type2_type3_adapters_and_registry(repository, tmp_path: Path):
    spec1 = BPS_70_TABLES[0]   # Type 2
    spec69 = BPS_70_TABLES[68]  # Type 3 (?year=2025)
    adapter = BpsStatTablesAdapter(specs=(spec1, spec69))

    type2_mock_html = (
        '<script>self.__next_f.push([1,"{\\"status\\":\\"OK\\",\\"var\\":[{\\"val\\":2212,\\"label\\":\\"IHK 01\\"}],'
        '\\"turvar\\":[{\\"val\\":\\"1\\",\\"label\\":\\"Makanan\\"}],'
        '\\"vervar\\":[{\\"val\\":1,\\"label\\":\\"KAB ACEH TENGAH\\"},{\\"val\\":2,\\"label\\":\\"SURABAYA\\"}],'
        '\\"tahun\\":[{\\"val\\":126,\\"label\\":\\"2026\\"}],'
        '\\"turtahun\\":[{\\"val\\":1,\\"label\\":\\"Januari\\"}],'
        '\\"datacontent\\":{\\"1221211261\\":\\"117,80\\",\\"2221211261\\":\\"120,00\\"}}"])</script>'
    )
    type3_mock_html = (
        '<script>self.__next_f.push([1,"{\\"columns\\":{\\"v1\\":{\\"nama_variabel\\":\\"TPT Agustus\\"}},'
        '\\"data\\":[{\\"label\\":\\"Simeulue\\",\\"kode_wilayah\\":1101000,\\"variables\\":{\\"v1\\":{\\"value\\":\\"5,31\\"}}},'
        '{\\"label\\":\\"Medan\\",\\"kode_wilayah\\":1275000,\\"variables\\":{\\"v1\\":{\\"value\\":\\"8,00\\"}}}],\\"year\\":\\"2025\\"}"])</script>'
    )

    seen_urls: list[str] = []

    def handler(req: httpx.Request) -> httpx.Response:
        seen_urls.append(str(req.url))
        if "statistics-table/3/" in str(req.url):
            assert req.url.params.get("year") == "2025"
            return httpx.Response(200, text=type3_mock_html)
        return httpx.Response(200, text=type2_mock_html)

    logs = InMemoryCrawlLogRepository()
    bronze = BronzeStore(local_dir=tmp_path / "raw", crawl_logs=logs)
    with make_http_client(without_robots(handler)) as client:
        summaries = [
            DatasetRunner().run_dataset(adapter, ds, client, repository, NOW_UTC, "MANUAL", bronze_store=bronze)
            for ds in adapter.list_datasets()
        ]

    assert len(seen_urls) == 2
    assert all(s.status == SourceStatus.FRESH and s.new_count == 1 for s in summaries)
    assert len(logs.entries) == 2

    reg = build_default_registry()
    assert "BPS_TABLES" in reg.all_source_codes()
    full_adapter = reg.create("BPS_TABLES", CrawlerSettings(mongo_uri="mongodb://contoh:27017"))
    assert len(full_adapter.list_datasets()) == 70


def test_cli_bronze_only_skips_mongo_open(monkeypatch, capsys):
    from garda_crawler.entrypoints import cli

    monkeypatch.setattr(cli, "_open_repository", lambda _settings: (_ for _ in ()).throw(AssertionError("should not open Mongo")))

    class _StubOrchestrator:
        def __init__(self, settings, repository):
            assert type(repository).__name__ == "BronzeOnlyRepository"

        def run_sources(self, sources, trigger, regions):
            return []

    monkeypatch.setattr(cli, "CrawlOrchestrator", _StubOrchestrator)
    rc = cli.main(["run", "--sumber", "BPS_TABLES", "--bronze-only"])
    assert rc == 0
    assert capsys.readouterr().out.strip() == "[]"
