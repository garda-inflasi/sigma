"""Shared base utilities and declarative stubs for source adapter plugins."""

from abc import ABC, abstractmethod
from collections.abc import Iterable, Iterator
from datetime import date

from garda_crawler.config.reference_catalog import REGENCIES
from garda_crawler.domain.entities import DatasetDefinition, FetchOutcome, TimeSeriesSpec
from garda_crawler.domain.enums import SKIPPED_MESSAGE_PREFIX
from garda_crawler.domain.protocols import HttpTransport


def format_skip_message(reason: str, action: str) -> str:
    """Build the standardized DILEWATI diagnostic message parsed by API/UI."""
    return f"{SKIPPED_MESSAGE_PREFIX}{reason}; tindakan: {action}"


def resolve_target_regions(regions: Iterable[str] | None) -> tuple[str, ...]:
    """Validate optional regency filter against the canonical 23 Aceh regencies."""
    if not regions:
        return tuple(REGENCIES.keys())
    selected = tuple(dict.fromkeys(code.strip() for code in regions if code.strip()))
    unknown = sorted(set(selected) - set(REGENCIES))
    if unknown:
        raise ValueError(f"kode kabupaten tidak dikenal: {', '.join(unknown)}")
    return selected


class BaseAdapter(ABC):
    """Base helper implementing region selection and standardized skip outcomes."""

    source_code: str

    def __init__(self, regions: Iterable[str] | None = None) -> None:
        self.regions: tuple[str, ...] = resolve_target_regions(regions)

    @abstractmethod
    def list_datasets(self) -> list[DatasetDefinition]:
        """Return datasets owned by this adapter."""

    def expected_series(self, dataset: DatasetDefinition) -> list[TimeSeriesSpec]:
        """Return canonical expected series for carry-forward/error markers."""
        return []

    def skipped_outcome(self, reason: str, action: str) -> FetchOutcome:
        """Build an immediate skipped FetchOutcome without performing outbound I/O."""
        return FetchOutcome(skipped_reason=format_skip_message(reason, action))

    @abstractmethod
    def fetch_daily(
        self,
        dataset: DatasetDefinition,
        client: HttpTransport,
        target_date: date,
    ) -> Iterator[FetchOutcome]:
        """Fetch latest daily data points for the specified dataset."""


class SkippedAdapter(BaseAdapter):
    """Guarded plugin stub that logs explicit operator action without network calls."""

    def __init__(
        self,
        *,
        source_code: str,
        datasets: tuple[DatasetDefinition, ...],
        reason: str,
        action: str,
        regions: Iterable[str] | None = None,
    ) -> None:
        super().__init__(regions)
        self.source_code = source_code
        self._datasets = list(datasets)
        self._reason = reason
        self._action = action

    def list_datasets(self) -> list[DatasetDefinition]:
        return list(self._datasets)

    def fetch_daily(
        self,
        dataset: DatasetDefinition,
        client: HttpTransport,
        target_date: date,
    ) -> Iterator[FetchOutcome]:
        yield self.skipped_outcome(self._reason, self._action)

    def fetch_backfill(
        self,
        dataset: DatasetDefinition,
        client: HttpTransport,
        start_date: date,
        end_date: date,
    ) -> Iterator[FetchOutcome]:
        yield self.skipped_outcome(self._reason, self._action)
