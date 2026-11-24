"""Bapanas Panel Harga Pangan adapter (auto-activates once BAPANAS_API_KEY is supplied)."""

from collections import Counter
from collections.abc import Iterable, Iterator
from datetime import date, datetime
from typing import Any

from garda_crawler.adapters.base import BaseAdapter
from garda_crawler.config.reference_catalog import COMMODITIES, REGENCIES
from garda_crawler.config.source_specs import BAPANAS_DATASET
from garda_crawler.domain.entities import (
    DataPoint,
    DatasetDefinition,
    FetchOutcome,
    RawBronzeCapture,
    TimeSeriesSpec,
)
from garda_crawler.domain.enums import Frequency
from garda_crawler.domain.errors import CrawlerError, RobotsDisallowedError, SchemaValidationError
from garda_crawler.domain.protocols import HttpTransport
from garda_crawler.validation.normalizers import normalize_alias, parse_decimal
from garda_crawler.validation.price_rules import REASON_NOT_NUMERIC, validate_price

# Canonical Bapanas panel commodity display names -> Garda master commodity codes.
BAPANAS_COMMODITY_ALIASES: dict[str, str] = {
    "beras premium": "BRS-PRE",
    "beras medium": "BRS-MED",
    "bawang merah": "BWM",
    "bawang putih bonggol": "BWP",
    "bawang putih": "BWP",
    "cabai merah keriting": "CBM-KRT",
    "cabai rawit merah": "CBR-MRH",
    "daging sapi murni": "DGG-SPI",
    "daging ayam ras": "DGG-AYM",
    "telur ayam ras": "TLR-AYM",
    "gula konsumsi": "GLA",
    "minyak goreng kemasan sederhana": "MYK",
    "minyak goreng kemasan": "MYK",
}

BAPANAS_DEFAULT_BASE_URL = "https://panelharga.badanpangan.go.id/api/front"
ACEH_PROVINCE_ID = 1


def _build_series(commodity_code: str, regency_code: str, source_url: str) -> TimeSeriesSpec:
    commodity = COMMODITIES[commodity_code]
    return TimeSeriesSpec(
        source_code="BAPANAS",
        dataset_name=BAPANAS_DATASET.name,
        collection=BAPANAS_DATASET.collection,
        series_key=f"{commodity_code}:{regency_code}",
        indicator="harga",
        unit=commodity.unit,
        frequency=Frequency.DAILY,
        commodity_code=commodity_code,
        regency_code=regency_code,
        source_url=source_url,
    )


def _resolve_regency_code(raw_row: dict[str, Any], default_regency_code: str) -> str | None:
    raw_code = str(raw_row.get("kabupaten_kode") or raw_row.get("city_code") or "").strip()
    if raw_code in REGENCIES:
        return raw_code
    raw_name = normalize_alias(str(raw_row.get("city_name") or raw_row.get("kabupaten") or ""))
    for regency in REGENCIES.values():
        if raw_name and normalize_alias(regency.name).endswith(raw_name):
            return regency.code
    return default_regency_code if default_regency_code in REGENCIES else None


def parse_bapanas_panel(
    payload: Any,
    fallback_date: date,
    source_url: str,
    default_regency_code: str = "1171",
) -> FetchOutcome:
    """Pure parser transforming Bapanas Panel Harga JSON payload into DataPoints."""
    if not isinstance(payload, dict) or not isinstance(payload.get("data"), list):
        raise SchemaValidationError("respons Bapanas bukan objek dengan daftar `data`")

    points: list[DataPoint] = []
    rejected: Counter[str] = Counter()

    for row in payload["data"]:
        if not isinstance(row, dict):
            continue
        alias = normalize_alias(str(row.get("name") or row.get("komoditas") or ""))
        commodity_code = BAPANAS_COMMODITY_ALIASES.get(alias)
        if commodity_code is None:
            rejected["komoditas_tak_terpetakan"] += 1
            continue

        regency_code = _resolve_regency_code(row, default_regency_code)
        if regency_code is None:
            rejected["wilayah_tak_terpetakan"] += 1
            continue

        raw_price = row.get("today") if row.get("today") is not None else row.get("price") or row.get("harga")
        if raw_price is None or str(raw_price).strip() in {"", "-", "0"}:
            continue

        value = parse_decimal(raw_price)
        if value is None:
            rejected[REASON_NOT_NUMERIC] += 1
            continue

        reason = validate_price(value, COMMODITIES[commodity_code].reference_price)
        if reason is not None:
            rejected[reason] += 1
            continue

        raw_date = str(row.get("date") or row.get("tanggal") or "").strip()
        period = fallback_date
        if raw_date:
            try:
                period = datetime.fromisoformat(raw_date[:10]).date()
            except ValueError:
                period = fallback_date

        series = _build_series(commodity_code, regency_code, source_url)
        points.append(
            DataPoint(
                series=series,
                period=period,
                value=value,
                source_url=source_url,
                attributes={"panel": "konsumen"},
            )
        )

    return FetchOutcome(points=points, rejected_counts=rejected, request_count=1)


class BapanasPanelAdapter(BaseAdapter):
    """Plugin adapter for Bapanas Panel Harga Pangan; stays guarded until BAPANAS_API_KEY is set."""

    source_code = "BAPANAS"

    def __init__(
        self,
        api_key: str = "",
        *,
        base_url: str = BAPANAS_DEFAULT_BASE_URL,
        regions: Iterable[str] | None = None,
    ) -> None:
        super().__init__(regions)
        self.api_key = api_key.strip()
        self.base_url = base_url.rstrip("/")

    def list_datasets(self) -> list[DatasetDefinition]:
        return [BAPANAS_DATASET]

    def fetch_daily(
        self,
        dataset: DatasetDefinition,
        client: HttpTransport,
        target_date: date,
    ) -> Iterator[FetchOutcome]:
        if not self.api_key:
            yield self.skipped_outcome(
                "API Panel Harga Bapanas memerlukan API key resmi atau sesi terdaftar",
                "ajukan akses resmi Bapanas dan isi BAPANAS_API_KEY",
            )
            return

        endpoint = f"{self.base_url}/harga-pangan-informasi"
        params = {
            "province_id": ACEH_PROVINCE_ID,
            "level_harga_id": 3,
            "key": self.api_key,
            "date": target_date.isoformat(),
        }
        masked_url = client.redact_secrets(f"{endpoint}?province_id={ACEH_PROVINCE_ID}&date={target_date.isoformat()}")
        try:
            payload = client.get_json(endpoint, params=params)
        except RobotsDisallowedError:
            raise
        except CrawlerError as exc:
            outcome = FetchOutcome(request_count=1, failed_count=1)
            outcome.warnings.append(client.redact_secrets(str(exc)))
            yield outcome
            return

        outcome = parse_bapanas_panel(payload, target_date, masked_url)
        outcome.raw_captures.append(
            RawBronzeCapture(
                source_code="BAPANAS",
                dataset_name=dataset.name,
                endpoint=endpoint,
                params={"province_id": ACEH_PROVINCE_ID, "date": target_date.isoformat()},
                payload=payload,
                capture_date=target_date,
                partition_year=target_date.year,
            )
        )
        yield outcome
