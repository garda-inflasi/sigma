"""Domain layer exports for garda_crawler."""

from garda_crawler.domain.entities import (
    DataPoint,
    DatasetDefinition,
    FetchOutcome,
    FieldSyncOutcome,
    RunSummary,
    TimeSeriesSpec,
)
from garda_crawler.domain.enums import (
    RAW_COLLECTIONS,
    SKIPPED_MESSAGE_PREFIX,
    VALIDATION_STATUS_WEIGHT,
    Frequency,
    SourceStatus,
    TriggerType,
)
from garda_crawler.domain.errors import (
    ConfigError,
    CrawlerError,
    HttpFetchError,
    RobotsDisallowedError,
    SchemaValidationError,
    TransientFetchError,
)
from garda_crawler.domain.protocols import (
    BackfillCapableAdapter,
    FieldEntriesReader,
    RawStorageRepository,
    SourceAdapter,
)

__all__ = [
    "BackfillCapableAdapter",
    "ConfigError",
    "CrawlerError",
    "DataPoint",
    "DatasetDefinition",
    "FetchOutcome",
    "FieldEntriesReader",
    "FieldSyncOutcome",
    "Frequency",
    "HttpFetchError",
    "RAW_COLLECTIONS",
    "RawStorageRepository",
    "RobotsDisallowedError",
    "RunSummary",
    "SKIPPED_MESSAGE_PREFIX",
    "SchemaValidationError",
    "SourceAdapter",
    "SourceStatus",
    "TimeSeriesSpec",
    "TransientFetchError",
    "TriggerType",
    "VALIDATION_STATUS_WEIGHT",
]
