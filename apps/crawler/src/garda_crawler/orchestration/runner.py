"""Single-dataset execution runner: fetch -> persist -> carry-forward -> audit ledger."""

from collections import Counter
from datetime import date, datetime

from garda_crawler.adapters.base import format_skip_message
from garda_crawler.domain.entities import (
    DatasetDefinition,
    FetchOutcome,
    RunSummary,
    TimeSeriesSpec,
)
from garda_crawler.domain.enums import SourceStatus, TriggerType
from garda_crawler.domain.errors import CrawlerError, RobotsDisallowedError
from garda_crawler.domain.protocols import BackfillCapableAdapter, HttpTransport, RawStorageRepository, SourceAdapter
from garda_crawler.transport.time_utils import to_wib_date, utc_now


def _compose_run_message(
    *,
    skipped_reason: str | None,
    new_count: int,
    carried_forward_count: int,
    failed_count: int,
    rejected: Counter[str],
    warnings: list[str],
) -> str:
    if skipped_reason:
        return skipped_reason

    parts = [f"baru {new_count}"]
    if carried_forward_count:
        parts.append(f"diteruskan {carried_forward_count}")
    if failed_count:
        parts.append(f"gagal {failed_count}")
    if rejected:
        detail = ", ".join(f"{reason}={count}" for reason, count in sorted(rejected.items()))
        parts.append(f"ditolak ({detail})")
    if warnings:
        parts.append("; ".join(warnings[:3]))
    return "; ".join(parts)


class DatasetRunner:
    """Executes one dataset or source adapter safely with carry-forward and ledger persistence."""

    def run_dataset(
        self,
        adapter: SourceAdapter,
        dataset: DatasetDefinition,
        client: HttpTransport,
        repository: RawStorageRepository,
        now_utc: datetime,
        trigger: str,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> RunSummary:
        """Fetch chunks for a dataset, persist valid points, write carry-forward markers, and log crawl_runs."""
        crawl_day = to_wib_date(now_utc)
        crawl_date = crawl_day.isoformat()
        is_backfill = trigger == TriggerType.BACKFILL
        run_id = f"{crawl_date}:{adapter.source_code}:{dataset.name}:{trigger}"
        started_at = utc_now()

        covered_keys: set[str] = set()
        new_count = 0
        total_requests = 0
        total_failed = 0
        rejected: Counter[str] = Counter()
        warnings: list[str] = []
        failed_series: list[TimeSeriesSpec] = []
        skipped_reason: str | None = None
        crashed = False

        try:
            if is_backfill:
                if start_date is None or end_date is None:
                    raise ValueError("start_date dan end_date wajib diisi untuk backfill")
                if not isinstance(adapter, BackfillCapableAdapter):
                    chunks = iter(
                        [
                            FetchOutcome(
                                skipped_reason=format_skip_message(
                                    f"sumber {adapter.source_code} tidak mendukung backfill historis",
                                    "jalankan mode harian",
                                )
                            )
                        ]
                    )
                else:
                    chunks = iter(adapter.fetch_backfill(dataset, client, start_date, end_date))
            else:
                chunks = iter(adapter.fetch_daily(dataset, client, crawl_day))

            for outcome in chunks:
                if outcome.skipped_reason and skipped_reason is None:
                    skipped_reason = outcome.skipped_reason
                total_requests += outcome.request_count
                total_failed += outcome.failed_count
                rejected.update(outcome.rejected_counts)
                warnings.extend(client.redact_secrets(w) for w in outcome.warnings)
                failed_series.extend(outcome.failed_series)

                if outcome.points:
                    saved, keys = repository.save_points(
                        outcome.points,
                        now_utc,
                        crawl_date,
                        run_id,
                        is_backfill=is_backfill,
                    )
                    new_count += saved
                    covered_keys.update(keys)
        except RobotsDisallowedError as exc:
            crashed = True
            skipped_reason = format_skip_message(
                client.redact_secrets(str(exc)),
                "periksa robots.txt atau minta izin resmi ke penyelenggara sumber data",
            )
        except (CrawlerError, Exception) as exc:
            crashed = True
            total_failed += 1
            warnings.append(client.redact_secrets(f"{type(exc).__name__}: {exc}"))

        carried_forward = 0
        if not is_backfill:
            expected = list(adapter.expected_series(dataset))
            if failed_series:
                known_keys = {item.series_key for item in expected}
                for extra in failed_series:
                    if extra.series_key not in known_keys:
                        expected.append(extra)
                        known_keys.add(extra.series_key)

            carried_forward = repository.write_carried_forward(
                expected,
                covered_keys,
                now_utc,
                crawl_date,
                run_id,
            )

        all_requests_failed = total_requests > 0 and total_failed >= total_requests and not covered_keys
        if skipped_reason or crashed or all_requests_failed:
            status = SourceStatus.ERROR
        elif is_backfill:
            status = SourceStatus.FRESH if new_count > 0 else SourceStatus.STALE_CARRIED_FORWARD
        else:
            fresh_today = (
                new_count > 0
                or repository.count_today_points(dataset.collection, adapter.source_code, dataset.name, crawl_date) > 0
            )
            status = SourceStatus.FRESH if fresh_today else SourceStatus.STALE_CARRIED_FORWARD

        latest_date = repository.latest_available_date(dataset.collection, adapter.source_code, dataset.name)
        message = _compose_run_message(
            skipped_reason=skipped_reason,
            new_count=new_count,
            carried_forward_count=carried_forward,
            failed_count=total_failed,
            rejected=rejected,
            warnings=warnings,
        )
        finished_at = utc_now()

        repository.record_run(
            run_id=run_id,
            source_code=adapter.source_code,
            dataset_name=dataset.name,
            crawl_date=crawl_date,
            started_at=started_at,
            finished_at=finished_at,
            status=status,
            new_count=new_count,
            carried_forward_count=carried_forward,
            last_available_date=latest_date,
            message=message,
            trigger=trigger,
        )

        return RunSummary(
            run_id=run_id,
            source_code=adapter.source_code,
            dataset_name=dataset.name,
            crawl_date=crawl_date,
            status=status,
            new_count=new_count,
            carried_forward_count=carried_forward,
            last_available_date=latest_date,
            message=message,
        )

    def run_adapter(
        self,
        adapter: SourceAdapter,
        client: HttpTransport,
        repository: RawStorageRepository,
        trigger: str,
        now_utc: datetime,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> list[RunSummary]:
        """Execute every dataset belonging to an adapter cleanly without cross-dataset side effects."""
        return [
            self.run_dataset(
                adapter=adapter,
                dataset=dataset,
                client=client,
                repository=repository,
                now_utc=now_utc,
                trigger=trigger,
                start_date=start_date,
                end_date=end_date,
            )
            for dataset in adapter.list_datasets()
        ]
