"""Domain enums and canonical contract constants."""

from enum import StrEnum


class SourceStatus(StrEnum):
    """Availability state of a crawled series or dataset run."""

    FRESH = "FRESH"
    STALE_CARRIED_FORWARD = "STALE_CARRIED_FORWARD"
    ERROR = "ERROR"
    SKIPPED = "SKIPPED"


class TriggerType(StrEnum):
    """Execution trigger recorded in crawl_runs."""

    SCHEDULED = "JADWAL"
    MANUAL = "MANUAL"
    BACKFILL = "BACKFILL"


class Frequency(StrEnum):
    """Time-series observation cadence stored in raw collections."""

    DAILY = "HARIAN"
    MONTHLY = "BULANAN"
    QUARTERLY = "TRIWULAN"
    ANNUAL = "TAHUNAN"


# Standardized prefix consumed by the API retry-failed filter and UI diagnostics.
SKIPPED_MESSAGE_PREFIX = "DILEWATI: "

# Canonical validation hierarchy for internal field entries (higher takes precedence in aggregates).
VALIDATION_STATUS_WEIGHT: dict[str, int] = {
    "DIAJUKAN": 1,
    "DIVALIDASI": 2,
    "DIPUBLIKASI": 3,
}

# Raw MongoDB collections monitored by dataunseen indexes and ML extraction.
RAW_COLLECTIONS: tuple[str, ...] = (
    "raw_pangan_harian",
    "raw_iklim_bmkg",
    "raw_bps_makro",
    "raw_konsumsi_pendapatan",
    "raw_neraca_ekonomi",
    "raw_tenaga_kerja",
    "raw_sektor_primer",
)
