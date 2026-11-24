"""Pure parser, capability, registry, and adapter plugin unit tests."""

from datetime import date

import httpx
import pytest

from garda_crawler.adapters.bapanas import BapanasPanelAdapter, parse_bapanas_panel
from garda_crawler.adapters.base import BaseAdapter
from garda_crawler.adapters.bmkg import BmkgForecastAdapter, parse_bmkg_forecast
from garda_crawler.adapters.bps import (
    MISSING_KEY_MESSAGE,
    BpsWebApiAdapter,
    lookup_var_id,
    parse_bps_payload,
    strip_bps_key,
)
from garda_crawler.adapters.open_meteo import OpenMeteoArchiveAdapter, parse_open_meteo_archive
from garda_crawler.adapters.pihps import PihpsBiAdapter, parse_pihps_grid
from garda_crawler.adapters.registry import SourceRegistry, build_default_registry, create_adapter
from garda_crawler.adapters.sp2kp import Sp2kpKemendagAdapter
from garda_crawler.config.reference_catalog import PIHPS_COMMODITIES, PIHPS_IGNORED_ROWS, REGENCIES
from garda_crawler.config.settings import CrawlerSettings
from garda_crawler.config.source_specs import BMKG_WARNINGS_DATASET, BPS_DATASET_SPECS
from garda_crawler.domain.entities import DatasetDefinition, FetchOutcome
from garda_crawler.domain.enums import SourceStatus
from garda_crawler.domain.errors import HttpFetchError, SchemaValidationError
from garda_crawler.orchestration.runner import DatasetRunner
from garda_crawler.validation.normalizers import normalize_alias
from tests.conftest import NOW_UTC, load_fixture, make_http_client, without_robots

RUNNER = DatasetRunner()
DS_INFLATION = next(d for d in BPS_DATASET_SPECS if d.name == "bps_inflasi")
SPEC_YOY = BPS_DATASET_SPECS[DS_INFLATION][0]


# --- PIHPS ---


def test_pihps_parse_real_grid_fixture():
    payload = load_fixture("pihps_grid_banda_aceh.json")
    outcome = parse_pihps_grid(payload, "1171", 1, "https://contoh/grid")
    assert not outcome.rejected_counts

    by_key = {(p.series.series_key, p.period): p for p in outcome.points}
    row = next(r for r in payload["data"] if r["name"].strip() == "Beras Kualitas Medium I")
    assert by_key[("BRS-MED:1171", date(2026, 11, 16))].value == float(row["16/11/2026"].replace(",", ""))
    assert ("BRS-MED:1171", date(2026, 11, 17)) not in by_key
    assert {p.series.commodity_code for p in outcome.points} <= set(PIHPS_COMMODITIES.values())
    assert any(p.series.commodity_code == "CBM-KRT" for p in outcome.points)
    assert all(p.series.unit == ("Rp/liter" if p.series.commodity_code == "MYK" else "Rp/kg") for p in outcome.points)


def test_pihps_recognizes_every_real_commodity_name():
    for row in load_fixture("pihps_grid_banda_aceh.json")["data"]:
        alias = normalize_alias(row["name"])
        assert alias in PIHPS_COMMODITIES or alias in PIHPS_IGNORED_ROWS, alias


def test_pihps_rejects_invalid_prices_and_unknown_commodities():
    payload = {
        "data": [
            {"name": "Gula Pasir Lokal", "01/10/2026": "0", "02/10/2026": "999.999.999", "03/10/2026": "abc", "05/10/2026": "18,000"},
            {"name": "Ikan Segar Baru", "01/10/2026": "40,000"},
        ]
    }
    outcome = parse_pihps_grid(payload, "1171", 1, "u")
    assert [p.value for p in outcome.points] == [18000.0]
    assert outcome.rejected_counts["harga_non_positif"] == 1
    assert outcome.rejected_counts["harga_di_luar_batas"] == 1
    assert outcome.rejected_counts["harga_bukan_angka"] == 1
    assert outcome.rejected_counts["komoditas_tak_terpetakan"] == 1


def test_pihps_malformed_payload_raises_schema_error():
    with pytest.raises(SchemaValidationError):
        parse_pihps_grid({"pesan": "rusak"}, "1171", 1, "u")


def test_pihps_partial_region_failure_keeps_remaining_regions(repository):
    payload = load_fixture("pihps_grid_banda_aceh.json")

    def handler(req: httpx.Request) -> httpx.Response:
        if req.url.params["regency_id"] == "2":
            return httpx.Response(404)
        return httpx.Response(200, json=payload)

    adapter = PihpsBiAdapter({"1171", "1174"})
    with make_http_client(without_robots(handler)) as client:
        summary = RUNNER.run_adapter(adapter, client, repository, "MANUAL", NOW_UTC)[0]
    assert summary.status == SourceStatus.FRESH
    assert "gagal 1" in summary.message
    col = repository.db["raw_pangan_harian"]
    assert col.count_documents({"kabupaten_kode": "1171", "source_status": SourceStatus.FRESH}) > 0
    failed = list(col.find({"kabupaten_kode": "1174"}))
    assert failed and all(d["source_status"] == SourceStatus.ERROR and d["nilai"] is None for d in failed)


def test_pihps_backfill_chunks_30_days_and_defaults_to_verified_cities(repository):
    windows: list[tuple[str, str]] = []

    def handler(req: httpx.Request) -> httpx.Response:
        windows.append((req.url.params["start_date"], req.url.params["end_date"]))
        return httpx.Response(200, json={"data": []})

    with make_http_client(without_robots(handler)) as client:
        RUNNER.run_adapter(
            PihpsBiAdapter({"1171"}),
            client,
            repository,
            "BACKFILL",
            NOW_UTC,
            date(2026, 1, 1),
            date(2026, 3, 15),
        )
    assert windows[0] == ("2026-01-01", "2026-01-30")
    assert windows[-1][1] == "2026-03-15"
    assert len(windows) == 3

    requested_ids: list[str] = []

    def count_handler(req: httpx.Request) -> httpx.Response:
        requested_ids.append(req.url.params["regency_id"])
        return httpx.Response(200, json={"data": []})

    with make_http_client(without_robots(count_handler)) as client:
        RUNNER.run_adapter(PihpsBiAdapter(), client, repository, "BACKFILL", NOW_UTC, date(2026, 1, 1), date(2026, 1, 5))
        assert sorted(requested_ids) == ["1", "2", "3"]
        requested_ids.clear()
        RUNNER.run_adapter(PihpsBiAdapter(), client, repository, "JADWAL", NOW_UTC)
    assert len(requested_ids) == 23


# --- BMKG ---


def test_bmkg_parse_real_forecast_fixture():
    payload = load_fixture("bmkg_banda_aceh.json")
    outcome = parse_bmkg_forecast(payload, "11.71.01.2001", "1171", date(2026, 11, 24), "u")
    assert outcome.rejected_counts["hari_tidak_lengkap"] == 1
    assert len(outcome.points) == 6
    assert {p.period for p in outcome.points} == {date(2026, 11, 25), date(2026, 11, 26)}
    assert {p.series.series_key for p in outcome.points} == {"curah_hujan:1171", "suhu:1171", "kelembapan:1171"}
    assert all(p.attributes["jenis_data"] == "PRAKIRAAN" for p in outcome.points)
    assert {p.attributes["horizon_hari"] for p in outcome.points} == {1, 2}

    slots = [s for day in payload["data"][0]["cuaca"] for s in day if s["local_datetime"].startswith("2026-11-25")]
    expected_rain = round(sum(s["tp"] for s in slots), 1)
    assert next(p.value for p in outcome.points if p.series.indicator == "curah_hujan_harian" and p.period == date(2026, 11, 25)) == expected_rain


def test_bmkg_warnings_and_backfill_explicitly_skipped(repository):
    def no_network(_: httpx.Request) -> httpx.Response:
        raise AssertionError("network must not be called")

    adapter = BmkgForecastAdapter({"1171"})
    with make_http_client(no_network) as client:
        warning_run = RUNNER.run_dataset(adapter, BMKG_WARNINGS_DATASET, client, repository, NOW_UTC, "MANUAL")
        backfill_run = RUNNER.run_adapter(adapter, client, repository, "BACKFILL", NOW_UTC, date(2026, 1, 1), date(2026, 1, 2))[0]

    assert warning_run.status == SourceStatus.ERROR
    assert warning_run.message.startswith("DILEWATI: ") and "robots" in warning_run.message
    assert backfill_run.status == SourceStatus.ERROR
    assert backfill_run.message.startswith("DILEWATI: ")
    assert "tindakan: gunakan --sumber OPEN_METEO" in backfill_run.message


def test_bmkg_empty_data_raises_schema_error():
    with pytest.raises(SchemaValidationError):
        parse_bmkg_forecast({"data": []}, "a", "1171", date(2026, 11, 24), "u")


# --- Open-Meteo ---


def test_open_meteo_parse_fixture():
    outcome = parse_open_meteo_archive(load_fixture("open_meteo_arsip_format_dokumentasi.json"), REGENCIES["1171"], "u")
    assert outcome.rejected_counts["curah_hujan_tidak_wajar"] == 1
    rainfall = {p.period: p.value for p in outcome.points if p.series.series_key == "curah_hujan:1171"}
    assert rainfall == {date(2026, 10, 1): 12.4, date(2026, 10, 2): 0.0}
    assert all(p.series.source_code == "OPEN_METEO" and p.attributes["sumber_sekunder"] for p in outcome.points)


def test_open_meteo_obeys_robots(repository):
    paths: list[str] = []

    def handler(req: httpx.Request) -> httpx.Response:
        paths.append(req.url.path)
        if req.url.path == "/robots.txt":
            return httpx.Response(200, text="User-agent: *\nDisallow: /", headers={"content-type": "text/plain"})
        raise AssertionError("archive endpoint must not be called")

    adapter = OpenMeteoArchiveAdapter({"1171", "1174"})
    with make_http_client(handler) as client:
        summary = RUNNER.run_adapter(adapter, client, repository, "JADWAL", NOW_UTC)[0]
    assert paths == ["/robots.txt"]
    assert summary.status == SourceStatus.ERROR
    assert summary.message.startswith("DILEWATI: ")
    docs = list(repository.db["raw_iklim_bmkg"].find({"sumber": "OPEN_METEO"}))
    assert len(docs) == 6
    assert all(d["nilai"] is None and d["source_status"] == SourceStatus.ERROR for d in docs)


# --- BPS ---


def test_bps_without_key_skips_cleanly_without_network(repository):
    def no_network(_: httpx.Request) -> httpx.Response:
        raise AssertionError("no HTTP requests allowed without API key")

    adapter = BpsWebApiAdapter("")
    with make_http_client(no_network) as client:
        summaries = RUNNER.run_adapter(adapter, client, repository, "JADWAL", NOW_UTC)

    assert len(summaries) == len(BPS_DATASET_SPECS)
    assert all(s.status == SourceStatus.ERROR and s.message == MISSING_KEY_MESSAGE for s in summaries)
    assert repository.db["raw_bps_makro"].count_documents({"nilai": None, "source_status": SourceStatus.ERROR}) > 0


def test_bps_parse_fixture():
    outcome = parse_bps_payload(load_fixture("bps_inflasi_yoy_2025.json"), DS_INFLATION, SPEC_YOY, "https://webapi.test/data")
    values = {(p.series.series_key, p.period): p.value for p in outcome.points}
    assert values[("inflasi_yoy:11", date(2025, 8, 1))] == 2.3
    assert values[("inflasi_yoy:11", date(2025, 9, 1))] == 2.84
    assert values[("inflasi_yoy:1171", date(2025, 9, 1))] == 2.5
    assert outcome.rejected_counts["wilayah_tak_terpetakan"] >= 1


def test_bps_not_available_and_error_status():
    empty = parse_bps_payload({"status": "OK", "data-availability": "list-not-available"}, DS_INFLATION, SPEC_YOY, "u")
    assert not empty.points
    with pytest.raises(HttpFetchError):
        parse_bps_payload({"status": "Error", "message": "invalid key"}, DS_INFLATION, SPEC_YOY, "u")


def test_bps_never_leaks_api_key_in_mongo(repository):
    secret = "KUNCIBPSRAHASIA999"
    payload = load_fixture("bps_inflasi_yoy_2025.json")

    def handler(req: httpx.Request) -> httpx.Response:
        if "/var/12/" in req.url.path and "/th/125/" in req.url.path:
            return httpx.Response(200, json=payload)
        return httpx.Response(500)

    variables = {spec.key: "12" for spec in BPS_DATASET_SPECS[DS_INFLATION]}
    adapter = BpsWebApiAdapter(secret, variables)
    with make_http_client(without_robots(handler), secrets=[secret]) as client:
        RUNNER.run_adapter(adapter, client, repository, "MANUAL", NOW_UTC)

    assert repository.db["raw_bps_makro"].count_documents({"source_status": SourceStatus.FRESH}) > 0
    for col_name in repository.db.list_collection_names():
        for doc in repository.db[col_name].find():
            assert secret not in str(doc), col_name
    assert strip_bps_key(f"https://x/th/1/key/{secret}") == "https://x/th/1"


def test_bps_ambiguous_var_lookup_fails_explicitly():
    catalog = {
        "status": "OK",
        "data": [
            {"page": 1, "pages": 1},
            [
                {"var_id": 1, "title": "Inflasi Year on Year Kota A"},
                {"var_id": 2, "title": "Inflasi Year on Year Kota B"},
            ],
        ],
    }
    with make_http_client(without_robots(lambda _: httpx.Response(200, json=catalog)), secrets=["K"]) as client:
        with pytest.raises(HttpFetchError, match="ambigu"):
            lookup_var_id(client, "K", "1100", SPEC_YOY, {})


# --- Bapanas & SP2KP & Extensibility ---


@pytest.mark.parametrize(
    ("adapter_cls", "keyword"),
    [(Sp2kpKemendagAdapter, "reCAPTCHA"), (BapanasPanelAdapter, "API key")],
)
def test_guarded_adapters_skip_without_network(repository, adapter_cls, keyword):
    def no_network(_: httpx.Request) -> httpx.Response:
        raise AssertionError("network call forbidden when unconfigured")

    adapter = adapter_cls()
    with make_http_client(no_network) as client:
        summary = RUNNER.run_adapter(adapter, client, repository, "JADWAL", NOW_UTC)[0]
    assert summary.status == SourceStatus.ERROR
    assert summary.message.startswith("DILEWATI: ") and keyword in summary.message
    assert repository.db["raw_pangan_harian"].count_documents({"sumber": adapter.source_code}) == 0


def test_bapanas_auto_activates_once_api_key_supplied(repository):
    payload = {
        "data": [
            {"name": "Beras Premium", "city_code": "1171", "today": 16000, "date": "2026-11-24"},
            {"name": "Cabai Merah Keriting", "city_code": "1171", "today": 52000, "date": "2026-11-24"},
        ]
    }
    seen_keys: list[str] = []

    def handler(req: httpx.Request) -> httpx.Response:
        seen_keys.append(req.url.params.get("key", ""))
        return httpx.Response(200, json=payload)

    settings = CrawlerSettings(mongo_uri="mongodb://x", bapanas_api_key="BAPANAS_LIVE_SECRET")
    adapter = create_adapter("BAPANAS", settings)
    with make_http_client(without_robots(handler), secrets=settings.secrets) as client:
        summary = RUNNER.run_adapter(adapter, client, repository, "MANUAL", NOW_UTC)[0]

    assert summary.status == SourceStatus.FRESH
    assert summary.new_count == 2
    assert seen_keys == ["BAPANAS_LIVE_SECRET"]
    docs = list(repository.db["raw_pangan_harian"].find({"sumber": "BAPANAS"}))
    assert len(docs) == 2
    assert all("BAPANAS_LIVE_SECRET" not in str(doc) for doc in docs)


def test_bapanas_parser_rejects_bad_rows():
    outcome = parse_bapanas_panel(
        {"data": [{"name": "Komoditas Asing", "today": 10000}, {"name": "Beras Medium", "today": "0"}]},
        date(2026, 11, 24),
        "https://bapanas.test",
    )
    assert not outcome.points
    assert outcome.rejected_counts["komoditas_tak_terpetakan"] == 1


def test_source_registry_supports_custom_plugin_extension():
    class CustomPlugin(BaseAdapter):
        source_code = "PERINDAG"

        def list_datasets(self) -> list[DatasetDefinition]:
            return [DatasetDefinition("perindag_harian", "raw_pangan_harian")]

        def fetch_daily(self, dataset, client, target_date):
            yield FetchOutcome()

    registry = build_default_registry()
    for code in ("PIHPS", "BMKG", "OPEN_METEO", "BPS", "SP2KP", "BAPANAS"):
        assert registry.create(code, CrawlerSettings(mongo_uri="mongodb://x")).source_code == code

    registry.register("PERINDAG", lambda _cfg, regions: CustomPlugin(regions), supports_backfill=False)
    assert "PERINDAG" in registry.all_source_codes()
    assert not registry.supports_backfill("PERINDAG")
    assert registry.create("PERINDAG", CrawlerSettings(mongo_uri="mongodb://x")).source_code == "PERINDAG"
    assert isinstance(SourceRegistry(), SourceRegistry)
