"""PIHPS Bank Indonesia traditional market price adapter (daily + 30-day chunked backfill)."""

from collections import Counter
from collections.abc import Iterable, Iterator
from datetime import date, datetime, timedelta
from typing import Any
from urllib.parse import urlencode

from garda_crawler.adapters.base import BaseAdapter
from garda_crawler.config.reference_catalog import (
    COMMODITIES,
    PIHPS_COMMODITIES,
    PIHPS_IGNORED_ROWS,
    PIHPS_REGENCIES_WITH_DATA,
    PIHPS_REGIONS,
)
from garda_crawler.config.source_specs import PIHPS_DAILY_DATASET
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
from garda_crawler.transport.time_utils import split_date_chunks
from garda_crawler.validation.normalizers import normalize_alias, parse_decimal
from garda_crawler.validation.price_rules import REASON_NOT_NUMERIC, validate_price

PIHPS_GRID_URL = "https://www.bi.go.id/hargapangan/WebSite/TabelHarga/GetGridDataDaerah"
PIHPS_PROVINCE_ID = 1  # Aceh
PIHPS_TRADITIONAL_MARKET_TYPE = 1
MAX_BACKFILL_CHUNK_DAYS = 30
EMPTY_VALUES = {"", "-", "--", "N/A", "null"}

# Inverse lookup from canonical regency code -> PIHPS regency_id.
REGENCY_TO_PIHPS_ID: dict[str, int] = {code: pid for pid, code in PIHPS_REGIONS.items()}
PIHPS_COMMODITY_CODES: tuple[str, ...] = tuple(dict.fromkeys(PIHPS_COMMODITIES.values()))


def _build_series(commodity_code: str, regency_code: str, pihps_regency_id: int) -> TimeSeriesSpec:
    commodity = COMMODITIES[commodity_code]
    return TimeSeriesSpec(
        source_code="PIHPS",
        dataset_name=PIHPS_DAILY_DATASET.name,
        collection=PIHPS_DAILY_DATASET.collection,
        series_key=f"{commodity_code}:{regency_code}",
        indicator="harga",
        unit=commodity.unit,
        frequency=Frequency.DAILY,
        commodity_code=commodity_code,
        regency_code=regency_code,
        source_url=f"{PIHPS_GRID_URL}?province_id={PIHPS_PROVINCE_ID}&regency_id={pihps_regency_id}",
    )


def _parse_ddmmyyyy(header_key: str) -> date | None:
    if len(header_key) != 10 or header_key[2] != "/" or header_key[5] != "/":
        return None
    try:
        return datetime.strptime(header_key, "%d/%m/%Y").date()
    except ValueError:
        return None


def parse_pihps_grid(
    payload: Any,
    regency_code: str,
    pihps_regency_id: int,
    source_url: str,
) -> FetchOutcome:
    """Pure parser converting raw PIHPS GetGridDataDaerah JSON into validated DataPoints."""
    if not isinstance(payload, dict) or not isinstance(payload.get("data"), list):
        raise SchemaValidationError("bentuk respons PIHPS bukan objek dengan daftar `data`")

    points: list[DataPoint] = []
    rejected: Counter[str] = Counter()

    for row in payload["data"]:
        if not isinstance(row, dict):
            continue
        raw_name = str(row.get("name", ""))
        alias = normalize_alias(raw_name)
        if not alias or alias in PIHPS_IGNORED_ROWS:
            continue

        commodity_code = PIHPS_COMMODITIES.get(alias)
        if commodity_code is None:
            rejected["komoditas_tak_terpetakan"] += 1
            continue

        series = _build_series(commodity_code, regency_code, pihps_regency_id)
        for key, cell in row.items():
            period = _parse_ddmmyyyy(key)
            if period is None:
                continue
            if cell is None or (isinstance(cell, str) and cell.strip() in EMPTY_VALUES):
                continue

            value = parse_decimal(cell)
            if value is None:
                rejected[REASON_NOT_NUMERIC] += 1
                continue

            reason = validate_price(value, 0)
            if reason is not None:
                rejected[reason] += 1
                continue

            points.append(
                DataPoint(
                    series=series,
                    period=period,
                    value=value,
                    source_url=source_url,
                    attributes={"pasar_tipe": "pasar_tradisional"},
                )
            )

    return FetchOutcome(points=points, rejected_counts=rejected, request_count=1)


class PihpsBiAdapter(BaseAdapter):
    """Adapter for Bank Indonesia PIHPS Aceh daily market food prices."""

    source_code = "PIHPS"

    def __init__(self, regions: Iterable[str] | None = None) -> None:
        self._explicit_regions = regions is not None
        super().__init__(regions)

    def list_datasets(self) -> list[DatasetDefinition]:
        return [PIHPS_DAILY_DATASET]

    def expected_series(self, dataset: DatasetDefinition) -> list[TimeSeriesSpec]:
        return [
            _build_series(commodity_code, regency_code, REGENCY_TO_PIHPS_ID[regency_code])
            for regency_code in self.regions
            for commodity_code in PIHPS_COMMODITY_CODES
        ]

    def _fetch_window(
        self,
        client: HttpTransport,
        start_date: date,
        end_date: date,
        regency_codes: tuple[str, ...],
    ) -> FetchOutcome:
        outcome = FetchOutcome()
        for regency_code in regency_codes:
            pihps_id = REGENCY_TO_PIHPS_ID[regency_code]
            params = {
                "price_type_id": PIHPS_TRADITIONAL_MARKET_TYPE,
                "comcat_id": "",
                "province_id": PIHPS_PROVINCE_ID,
                "regency_id": pihps_id,
                "show_all": "true",
                "start_date": start_date.isoformat(),
                "end_date": end_date.isoformat(),
            }
            full_url = f"{PIHPS_GRID_URL}?{urlencode(params)}"
            outcome.request_count += 1
            try:
                payload = client.get_json(PIHPS_GRID_URL, params=params)
                parsed = parse_pihps_grid(payload, regency_code, pihps_id, full_url)
            except RobotsDisallowedError:
                raise
            except CrawlerError as exc:
                outcome.failed_count += 1
                outcome.warnings.append(f"{regency_code}: {exc}")
                continue

            outcome.raw_captures.append(
                RawBronzeCapture(
                    source_code=self.source_code,
                    dataset_name=PIHPS_DAILY_DATASET.name,
                    endpoint=PIHPS_GRID_URL,
                    params=params,
                    payload=payload,
                    partition_year=end_date.year,
                    capture_date=end_date,
                )
            )
            outcome.points.extend(parsed.points)
            outcome.rejected_counts.update(parsed.rejected_counts)
        return outcome

    def fetch_daily(
        self,
        dataset: DatasetDefinition,
        client: HttpTransport,
        target_date: date,
    ) -> Iterator[FetchOutcome]:
        window_start = target_date - timedelta(days=7)
        yield self._fetch_window(client, window_start, target_date, self.regions)

    def fetch_backfill(
        self,
        dataset: DatasetDefinition,
        client: HttpTransport,
        start_date: date,
        end_date: date,
    ) -> Iterator[FetchOutcome]:
        target_regions = (
            self.regions
            if self._explicit_regions
            else tuple(code for code in self.regions if code in PIHPS_REGENCIES_WITH_DATA)
        )
        for chunk_start, chunk_end in split_date_chunks(start_date, end_date, MAX_BACKFILL_CHUNK_DAYS):
            yield self._fetch_window(client, chunk_start, chunk_end, target_regions)
