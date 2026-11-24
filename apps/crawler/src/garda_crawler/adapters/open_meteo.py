"""Open-Meteo historical climate reanalysis adapter."""

from collections import Counter
from collections.abc import Iterable, Iterator
from datetime import date, timedelta
from typing import Any
from urllib.parse import urlencode

from garda_crawler.adapters.base import BaseAdapter
from garda_crawler.config.reference_catalog import REGENCIES, RegencySpec
from garda_crawler.config.source_specs import OPEN_METEO_ARCHIVE_DATASET
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
from garda_crawler.validation.weather_rules import is_valid_weather_value, weather_reject_reason

OPEN_METEO_ARCHIVE_URL = "https://archive-api.open-meteo.com/v1/archive"
MAX_ARCHIVE_CHUNK_DAYS = 90

OPEN_METEO_INDICATORS: tuple[tuple[str, str, str, str], ...] = (
    ("precipitation_sum", "curah_hujan", "curah_hujan_harian", "mm"),
    ("temperature_2m_mean", "suhu", "suhu_rata", "C"),
    ("relative_humidity_2m_mean", "kelembapan", "kelembapan_rata", "%"),
)


def _build_series(key_prefix: str, indicator: str, unit: str, regency_code: str) -> TimeSeriesSpec:
    return TimeSeriesSpec(
        source_code="OPEN_METEO",
        dataset_name=OPEN_METEO_ARCHIVE_DATASET.name,
        collection=OPEN_METEO_ARCHIVE_DATASET.collection,
        series_key=f"{key_prefix}:{regency_code}",
        indicator=indicator,
        unit=unit,
        frequency=Frequency.DAILY,
        regency_code=regency_code,
        source_url=OPEN_METEO_ARCHIVE_URL,
    )


def parse_open_meteo_archive(payload: Any, regency: RegencySpec, source_url: str) -> FetchOutcome:
    """Pure parser extracting daily precipitation, temperature, and humidity arrays."""
    if not isinstance(payload, dict) or not isinstance(payload.get("daily"), dict):
        raise SchemaValidationError("respons Open-Meteo tidak memiliki objek `daily`")

    daily = payload["daily"]
    dates = daily.get("time")
    if not isinstance(dates, list):
        raise SchemaValidationError("respons Open-Meteo tidak memiliki daftar `daily.time`")

    points: list[DataPoint] = []
    rejected: Counter[str] = Counter()

    for api_field, prefix, indicator, unit in OPEN_METEO_INDICATORS:
        values = daily.get(api_field)
        if not isinstance(values, list):
            continue
        series = _build_series(prefix, indicator, unit, regency.code)
        for day_str, raw_value in zip(dates, values, strict=False):
            if raw_value is None:
                continue
            value = round(float(raw_value), 1)
            if not is_valid_weather_value(indicator, value):
                rejected[weather_reject_reason(indicator)] += 1
                continue
            points.append(
                DataPoint(
                    series=series,
                    period=date.fromisoformat(str(day_str)[:10]),
                    value=value,
                    source_url=source_url,
                    attributes={"sumber_sekunder": "ERA5_REANALISIS", "jenis_data": "HISTORIS"},
                )
            )

    return FetchOutcome(points=points, rejected_counts=rejected, request_count=1)


class OpenMeteoArchiveAdapter(BaseAdapter):
    """Historical weather adapter using Open-Meteo ERA5 archive endpoints."""

    source_code = "OPEN_METEO"

    def __init__(self, regions: Iterable[str] | None = None) -> None:
        super().__init__(regions)

    def list_datasets(self) -> list[DatasetDefinition]:
        return [OPEN_METEO_ARCHIVE_DATASET]

    def expected_series(self, dataset: DatasetDefinition) -> list[TimeSeriesSpec]:
        return [
            _build_series(prefix, indicator, unit, regency_code)
            for regency_code in self.regions
            for _, prefix, indicator, unit in OPEN_METEO_INDICATORS
        ]

    def _fetch_window(self, client: HttpTransport, start_date: date, end_date: date) -> FetchOutcome:
        outcome = FetchOutcome()
        daily_fields = ",".join(item[0] for item in OPEN_METEO_INDICATORS)
        for regency_code in self.regions:
            regency = REGENCIES[regency_code]
            params = {
                "latitude": regency.lat,
                "longitude": regency.lon,
                "start_date": start_date.isoformat(),
                "end_date": end_date.isoformat(),
                "daily": daily_fields,
                "timezone": "Asia/Jakarta",
            }
            full_url = f"{OPEN_METEO_ARCHIVE_URL}?{urlencode(params)}"
            outcome.request_count += 1
            try:
                payload = client.get_json(OPEN_METEO_ARCHIVE_URL, params=params)
                parsed = parse_open_meteo_archive(payload, regency, full_url)
            except RobotsDisallowedError:
                raise
            except CrawlerError as exc:
                outcome.failed_count += 1
                outcome.warnings.append(f"{regency_code}: {exc}")
                continue

            outcome.points.extend(parsed.points)
            outcome.rejected_counts.update(parsed.rejected_counts)
            outcome.raw_captures.append(
                RawBronzeCapture(
                    source_code="OPEN_METEO",
                    dataset_name=OPEN_METEO_ARCHIVE_DATASET.name,
                    endpoint=OPEN_METEO_ARCHIVE_URL,
                    params={
                        "kabupaten_kode": regency_code,
                        "start_date": start_date.isoformat(),
                        "end_date": end_date.isoformat(),
                    },
                    payload=payload,
                    capture_date=end_date,
                    partition_year=end_date.year,
                )
            )
        return outcome

    def fetch_daily(
        self,
        dataset: DatasetDefinition,
        client: HttpTransport,
        target_date: date,
    ) -> Iterator[FetchOutcome]:
        # ERA5 reanalysis trails real time by ~2 days; fetch recent 14-day trailing window.
        yield self._fetch_window(client, target_date - timedelta(days=14), target_date - timedelta(days=2))

    def fetch_backfill(
        self,
        dataset: DatasetDefinition,
        client: HttpTransport,
        start_date: date,
        end_date: date,
    ) -> Iterator[FetchOutcome]:
        for chunk_start, chunk_end in split_date_chunks(start_date, end_date, MAX_ARCHIVE_CHUNK_DAYS):
            yield self._fetch_window(client, chunk_start, chunk_end)
