"""BMKG public weather forecast adapter (ADM4 daily forecast + robots-guarded warnings)."""

from collections import Counter, defaultdict
from collections.abc import Iterable, Iterator
from datetime import date
from typing import Any
from urllib.parse import urlencode

from garda_crawler.adapters.base import BaseAdapter
from garda_crawler.config.reference_catalog import BMKG_REGIONS
from garda_crawler.config.source_specs import BMKG_FORECAST_DATASET, BMKG_WARNINGS_DATASET
from garda_crawler.domain.entities import (
    DataPoint,
    DatasetDefinition,
    FetchOutcome,
    TimeSeriesSpec,
)
from garda_crawler.domain.enums import Frequency
from garda_crawler.domain.errors import CrawlerError, RobotsDisallowedError, SchemaValidationError
from garda_crawler.domain.protocols import HttpTransport
from garda_crawler.validation.weather_rules import (
    REASON_INCOMPLETE_DAY,
    is_valid_weather_value,
    weather_reject_reason,
)

BMKG_FORECAST_URL = "https://api.bmkg.go.id/publik/prakiraan-cuaca"
MIN_SLOTS_PER_FULL_DAY = 8

BMKG_INDICATORS: tuple[tuple[str, str, str], ...] = (
    ("curah_hujan", "curah_hujan_harian", "mm"),
    ("suhu", "suhu_rata", "C"),
    ("kelembapan", "kelembapan_rata", "%"),
)

REGENCY_TO_ADM4: dict[str, str] = {code: adm4 for adm4, code in BMKG_REGIONS.items()}


def _build_series(key_prefix: str, indicator: str, unit: str, regency_code: str, adm4_code: str) -> TimeSeriesSpec:
    return TimeSeriesSpec(
        source_code="BMKG",
        dataset_name=BMKG_FORECAST_DATASET.name,
        collection=BMKG_FORECAST_DATASET.collection,
        series_key=f"{key_prefix}:{regency_code}",
        indicator=indicator,
        unit=unit,
        frequency=Frequency.DAILY,
        regency_code=regency_code,
        source_url=f"{BMKG_FORECAST_URL}?adm4={adm4_code}",
    )


def _group_weather_slots(payload: Any) -> dict[date, list[dict[str, Any]]]:
    if not isinstance(payload, dict) or not isinstance(payload.get("data"), list) or not payload["data"]:
        raise SchemaValidationError("respons BMKG tidak memiliki `data` prakiraan")
    days_block = payload["data"][0].get("cuaca")
    if not isinstance(days_block, list):
        raise SchemaValidationError("respons BMKG tidak memiliki daftar `cuaca`")

    by_date: dict[date, list[dict[str, Any]]] = defaultdict(list)
    for slot_group in days_block:
        if not isinstance(slot_group, list):
            continue
        for slot in slot_group:
            if not isinstance(slot, dict) or not slot.get("local_datetime"):
                continue
            day = date.fromisoformat(str(slot["local_datetime"])[:10])
            by_date[day].append(slot)
    return by_date


def parse_bmkg_forecast(
    payload: Any,
    adm4_code: str,
    regency_code: str,
    crawl_day: date,
    source_url: str,
) -> FetchOutcome:
    """Pure parser aggregating 3-hourly BMKG slots into complete daily rainfall, temperature, and humidity."""
    grouped = _group_weather_slots(payload)
    points: list[DataPoint] = []
    rejected: Counter[str] = Counter()

    for forecast_day, slots in sorted(grouped.items()):
        if len(slots) < MIN_SLOTS_PER_FULL_DAY:
            rejected[REASON_INCOMPLETE_DAY] += 1
            continue

        rainfall = [float(s["tp"]) for s in slots if s.get("tp") is not None]
        temps = [float(s["t"]) for s in slots if s.get("t") is not None]
        humidities = [float(s["hu"]) for s in slots if s.get("hu") is not None]

        aggregates = (
            ("curah_hujan", "curah_hujan_harian", "mm", round(sum(rainfall), 1) if len(rainfall) == len(slots) else None),
            ("suhu", "suhu_rata", "C", round(sum(temps) / len(temps), 1) if temps else None),
            ("kelembapan", "kelembapan_rata", "%", round(sum(humidities) / len(humidities), 1) if humidities else None),
        )
        for prefix, indicator, unit, value in aggregates:
            if value is None:
                continue
            if not is_valid_weather_value(indicator, value):
                rejected[weather_reject_reason(indicator)] += 1
                continue
            points.append(
                DataPoint(
                    series=_build_series(prefix, indicator, unit, regency_code, adm4_code),
                    period=forecast_day,
                    value=value,
                    source_url=source_url,
                    attributes={
                        "adm4": adm4_code,
                        "jenis_data": "PRAKIRAAN",
                        "horizon_hari": (forecast_day - crawl_day).days,
                    },
                )
            )

    return FetchOutcome(points=points, rejected_counts=rejected, request_count=1)


class BmkgForecastAdapter(BaseAdapter):
    """Adapter for BMKG ADM4 weather forecasts (plus explicit skip guard for disallowed warning feeds)."""

    source_code = "BMKG"

    def __init__(self, regions: Iterable[str] | None = None) -> None:
        super().__init__(regions)

    def list_datasets(self) -> list[DatasetDefinition]:
        return [BMKG_FORECAST_DATASET, BMKG_WARNINGS_DATASET]

    def expected_series(self, dataset: DatasetDefinition) -> list[TimeSeriesSpec]:
        if dataset.name != BMKG_FORECAST_DATASET.name:
            return []
        return [
            _build_series(prefix, indicator, unit, regency_code, REGENCY_TO_ADM4[regency_code])
            for regency_code in self.regions
            for prefix, indicator, unit in BMKG_INDICATORS
        ]

    def fetch_daily(
        self,
        dataset: DatasetDefinition,
        client: HttpTransport,
        target_date: date,
    ) -> Iterator[FetchOutcome]:
        if dataset.name == BMKG_WARNINGS_DATASET.name:
            yield self.skipped_outcome(
                "robots.txt api.bmkg.go.id melarang jalur /alerts/nowcast/id",
                "gunakan prakiraan BMKG atau hubungi BMKG bila butuh feed peringatan",
            )
            return

        outcome = FetchOutcome()
        for regency_code in self.regions:
            adm4_code = REGENCY_TO_ADM4[regency_code]
            params = {"adm4": adm4_code}
            full_url = f"{BMKG_FORECAST_URL}?{urlencode(params)}"
            outcome.request_count += 1
            try:
                payload = client.get_json(BMKG_FORECAST_URL, params=params)
                parsed = parse_bmkg_forecast(payload, adm4_code, regency_code, target_date, full_url)
            except RobotsDisallowedError:
                raise
            except CrawlerError as exc:
                outcome.failed_count += 1
                outcome.warnings.append(f"{regency_code}: {exc}")
                continue

            outcome.points.extend(parsed.points)
            outcome.rejected_counts.update(parsed.rejected_counts)
        yield outcome

    def fetch_backfill(
        self,
        dataset: DatasetDefinition,
        client: HttpTransport,
        start_date: date,
        end_date: date,
    ) -> Iterator[FetchOutcome]:
        yield self.skipped_outcome(
            "BMKG hanya menyediakan prakiraan ke depan, bukan data historis",
            "gunakan --sumber OPEN_METEO untuk data iklim historis",
        )
