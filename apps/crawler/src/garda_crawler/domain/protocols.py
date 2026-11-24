"""Capability-based structural protocols for adapters, readers, and storage."""

from collections.abc import Iterable, Iterator
from datetime import date, datetime
from typing import Any, Protocol, runtime_checkable

from garda_crawler.domain.entities import (
    DataPoint,
    DatasetDefinition,
    FetchOutcome,
    RawBronzeCapture,
    TimeSeriesSpec,
)


@runtime_checkable
class HttpTransport(Protocol):
    """Minimal outbound HTTP interface required by online source adapters."""

    def get_json(self, url: str, params: dict[str, Any] | None = None) -> Any:
        """Execute a throttled, robots-checked GET returning parsed JSON."""

    def get_text(self, url: str, params: dict[str, Any] | None = None) -> str:
        """Execute a throttled, robots-checked GET returning raw response text."""

    def redact_secrets(self, text: str) -> str:
        """Mask registered secrets before logging or raising messages."""


@runtime_checkable
class SourceAdapter(Protocol):
    """Required capability contract for every daily source plugin."""

    source_code: str

    def list_datasets(self) -> list[DatasetDefinition]:
        """Return datasets owned by this source adapter."""

    def expected_series(self, dataset: DatasetDefinition) -> list[TimeSeriesSpec]:
        """Return canonical series expected to exist even on partial/total failures."""

    def fetch_daily(
        self,
        dataset: DatasetDefinition,
        client: HttpTransport,
        target_date: date,
    ) -> Iterator[FetchOutcome]:
        """Fetch daily observations for one dataset in isolated chunks."""


@runtime_checkable
class BackfillCapableAdapter(SourceAdapter, Protocol):
    """Optional capability contract for historical range backfills."""

    def fetch_backfill(
        self,
        dataset: DatasetDefinition,
        client: HttpTransport,
        start_date: date,
        end_date: date,
    ) -> Iterator[FetchOutcome]:
        """Fetch historical observations in bounded chunks over [start_date, end_date]."""


@runtime_checkable
class FieldEntriesReader(Protocol):
    """Read-only database reader contract for internal field price entries."""

    def __enter__(self) -> "FieldEntriesReader": ...

    def __exit__(self, *args: object) -> None: ...

    def fetch_index(self) -> list[dict[str, Any]]:
        """Return minimal (id, version, recorded_at) index of publishable entries."""

    def fetch_rows_by_ids(self, entry_ids: Iterable[int]) -> list[dict[str, Any]]:
        """Return sanitized rows for changed or new entry IDs."""

    def count_demo_entries(self) -> int:
        """Count seeded demo rows ignored during production sync."""


@runtime_checkable
class RawStorageRepository(Protocol):
    """Storage persistence contract implemented over MongoDB dataunseen."""

    def ensure_indexes(self) -> None: ...

    def save_points(
        self,
        points: list[DataPoint],
        now_utc: datetime,
        crawl_date: str,
        run_id: str,
        *,
        is_backfill: bool = False,
    ) -> tuple[int, set[str]]: ...

    def write_carried_forward(
        self,
        expected: list[TimeSeriesSpec],
        covered_keys: set[str],
        now_utc: datetime,
        crawl_date: str,
        run_id: str,
    ) -> int: ...

    def count_today_points(self, collection: str, source_code: str, dataset_name: str, crawl_date: str) -> int: ...

    def latest_available_date(self, collection: str, source_code: str, dataset_name: str) -> date | None: ...

    def record_run(
        self,
        *,
        run_id: str,
        source_code: str,
        dataset_name: str,
        crawl_date: str,
        started_at: datetime,
        finished_at: datetime,
        status: str,
        new_count: int,
        carried_forward_count: int,
        last_available_date: date | None,
        message: str,
        trigger: str,
    ) -> None: ...
