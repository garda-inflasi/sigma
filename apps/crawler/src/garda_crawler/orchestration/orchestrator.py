"""Multi-source fault-isolated crawl orchestrator."""

from collections.abc import Callable, Iterable
from datetime import date, datetime

from garda_crawler.adapters.registry import DEFAULT_ACTIVE_SOURCES, DEFAULT_REGISTRY, SourceRegistry
from garda_crawler.config.settings import CrawlerSettings
from garda_crawler.domain.entities import DatasetDefinition, RunSummary
from garda_crawler.domain.enums import SourceStatus, TriggerType
from garda_crawler.domain.protocols import HttpTransport, RawStorageRepository
from garda_crawler.orchestration.runner import DatasetRunner
from garda_crawler.storage.bronze_store import BronzeStore
from garda_crawler.storage.crawl_log_repository import PostgresCrawlLogRepository
from garda_crawler.transport.polite_http import PoliteHttpClient
from garda_crawler.transport.time_utils import utc_now

ProgressCallback = Callable[[str, str, int, int, RunSummary | None], None]


class CrawlOrchestrator:
    """Coordinates isolated execution across selected registered source plugins."""

    def __init__(
        self,
        settings: CrawlerSettings,
        repository: RawStorageRepository,
        *,
        registry: SourceRegistry = DEFAULT_REGISTRY,
        runner: DatasetRunner | None = None,
        client_factory: Callable[[CrawlerSettings], HttpTransport] | None = None,
        bronze_store: BronzeStore | None = None,
    ) -> None:
        self._settings = settings
        self._repository = repository
        self._registry = registry
        self._runner = runner or DatasetRunner()
        self._client_factory = client_factory or self._default_client_factory
        if bronze_store is not None:
            self._bronze_store: BronzeStore | None = bronze_store
        elif settings.minio_endpoint or settings.bronze_dir:
            logs = PostgresCrawlLogRepository(settings.postgres_url) if settings.postgres_url else None
            self._bronze_store = BronzeStore(
                local_dir=settings.bronze_dir,
                minio_endpoint=settings.minio_endpoint,
                minio_access_key=settings.minio_access_key,
                minio_secret_key=settings.minio_secret_key,
                minio_bucket=settings.minio_bucket_raw,
                minio_secure=settings.minio_secure,
                crawl_logs=logs,
            )
        else:
            self._bronze_store = None

    @staticmethod
    def _default_client_factory(settings: CrawlerSettings) -> PoliteHttpClient:
        return PoliteHttpClient(
            user_agent=settings.user_agent,
            delay_ms=settings.request_delay_ms,
            secrets=settings.secrets,
        )

    def resolve_adapters(
        self,
        source_codes: Iterable[str] | None = None,
        regions: Iterable[str] | None = None,
    ) -> list[tuple[str, DatasetDefinition, object]]:
        """Build ordered `(source_code, dataset, adapter)` work plan."""
        selected = tuple(dict.fromkeys(code.strip().upper() for code in (source_codes or DEFAULT_ACTIVE_SOURCES) if code.strip()))
        plan: list[tuple[str, DatasetDefinition, object]] = []
        for code in selected:
            adapter = self._registry.create(code, self._settings, regions)
            for dataset in adapter.list_datasets():
                plan.append((code, dataset, adapter))
        return plan

    def run_sources(
        self,
        source_codes: Iterable[str] | None = None,
        *,
        trigger: str = TriggerType.MANUAL,
        regions: Iterable[str] | None = None,
        now_utc: datetime | None = None,
        start_date: date | None = None,
        end_date: date | None = None,
        on_progress: ProgressCallback | None = None,
    ) -> list[RunSummary]:
        """Run selected sources sequentially with full exception isolation per source."""
        moment = now_utc or utc_now()
        self._repository.ensure_indexes()
        plan = self.resolve_adapters(source_codes, regions)
        total = len(plan)
        summaries: list[RunSummary] = []

        client = self._client_factory(self._settings)
        close_fn = getattr(client, "close", None)
        try:
            for index, (code, dataset, adapter) in enumerate(plan):
                if on_progress:
                    on_progress(code, dataset.name, index, total, None)
                try:
                    summary = self._runner.run_dataset(
                        adapter=adapter,  # type: ignore[arg-type]
                        dataset=dataset,
                        client=client,
                        repository=self._repository,
                        now_utc=moment,
                        trigger=trigger,
                        start_date=start_date,
                        end_date=end_date,
                        bronze_store=self._bronze_store,
                    )
                except Exception as exc:
                    summary = RunSummary(
                        run_id=f"{moment.date().isoformat()}:{code}:{dataset.name}:{trigger}",
                        source_code=code,
                        dataset_name=dataset.name,
                        crawl_date=moment.date().isoformat(),
                        status=SourceStatus.ERROR,
                        new_count=0,
                        carried_forward_count=0,
                        last_available_date=None,
                        message=client.redact_secrets(f"orchestrator exception: {type(exc).__name__}: {exc}"),
                    )
                summaries.append(summary)
                if on_progress:
                    on_progress(code, dataset.name, index + 1, total, summary)
        finally:
            if callable(close_fn):
                close_fn()

        return summaries
