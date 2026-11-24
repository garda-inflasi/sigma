"""Immutable domain entities decoupled from MongoDB and HTTP libraries."""

from collections import Counter
from dataclasses import dataclass, field
from datetime import date, datetime
from typing import Any


@dataclass(frozen=True)
class DatasetDefinition:
    """Declarative dataset metadata targeting a specific raw MongoDB collection."""

    name: str
    collection: str
    schedule_tier: str = "daily"
    stale_sla_days: int = 2
    default_backfill_days: int = 0


@dataclass(frozen=True)
class TimeSeriesSpec:
    """Logical time-series identifier used for grouping, carry-forward, and storage keys."""

    source_code: str
    dataset_name: str
    collection: str
    series_key: str
    indicator: str
    unit: str
    frequency: str
    commodity_code: str | None = None
    regency_code: str | None = None
    source_url: str = ""
    validation_status: str = "BELUM_DIVERIFIKASI"

    def document_id(self, period: date, crawl_date: str) -> str:
        """Build the immutable point-in-time vintage _id contract."""
        return f"{self.source_code}:{self.dataset_name}:{self.series_key}:{period.isoformat()}:{crawl_date}"


@dataclass(frozen=True)
class DataPoint:
    """Single validated domain observation ready for MongoDB persistence."""

    series: TimeSeriesSpec
    period: date
    value: float
    source_url: str
    attributes: dict[str, Any] = field(default_factory=dict)


@dataclass(frozen=True)
class RawBronzeCapture:
    """Original unaltered upstream JSON/XHR payload queued for immutable Bronze storage."""

    source_code: str
    dataset_name: str
    endpoint: str
    params: dict[str, Any]
    payload: Any
    partition_year: int | None = None
    capture_date: date | None = None


@dataclass
class FetchOutcome:
    """Result returned by pure parsers and adapter fetch methods."""

    points: list[DataPoint] = field(default_factory=list)
    raw_captures: list[RawBronzeCapture] = field(default_factory=list)
    rejected_counts: Counter[str] = field(default_factory=Counter)
    warnings: list[str] = field(default_factory=list)
    failed_series: list[TimeSeriesSpec] = field(default_factory=list)
    request_count: int = 0
    failed_count: int = 0
    skipped_reason: str | None = None


@dataclass(frozen=True)
class RunSummary:
    """Execution audit summary persisted to crawl_runs and returned by CLI/HTTP API."""

    run_id: str
    source_code: str
    dataset_name: str
    crawl_date: str
    status: str
    new_count: int
    carried_forward_count: int
    last_available_date: date | None
    message: str


@dataclass(frozen=True)
class FieldSyncOutcome:
    """Summary returned by the incremental Postgres-to-MongoDB field price sync."""

    run_id: str
    status: str
    written_count: int
    deleted_count: int
    rejected_count: int
    carried_forward_count: int
    message: str
