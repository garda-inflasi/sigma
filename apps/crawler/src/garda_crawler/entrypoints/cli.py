"""CLI entrypoint preserving all Makefile, Dockerfile, and operator flags."""

import argparse
import json
import logging
import sys
from collections.abc import Sequence
from datetime import date
from typing import Any

from pymongo import MongoClient

from garda_crawler.adapters.field_entries import sync_field_entries
from garda_crawler.adapters.registry import DEFAULT_ACTIVE_SOURCES
from garda_crawler.config.settings import CrawlerSettings
from garda_crawler.domain.entities import RunSummary
from garda_crawler.domain.enums import SourceStatus, TriggerType
from garda_crawler.orchestration.orchestrator import CrawlOrchestrator
from garda_crawler.orchestration.scheduler import CrawlerWorker, is_heartbeat_healthy
from garda_crawler.storage.bronze_store import BronzeOnlyRepository, BronzeStore
from garda_crawler.storage.crawl_log_repository import PostgresCrawlLogRepository
from garda_crawler.storage.run_ledger import SCHEDULE_COLLECTION
from garda_crawler.storage.unseen_repository import UnseenRepository
from garda_crawler.domain.entities import RawBronzeCapture
from garda_crawler.transport.postgres_reader import PostgresFieldReader
from garda_crawler.transport.time_utils import to_wib_date, utc_now


def _parse_csv(raw: str | None) -> list[str] | None:
    if not raw:
        return None
    items = [part.strip() for part in raw.split(",") if part.strip()]
    return items or None


def _open_repository(settings: CrawlerSettings) -> tuple[MongoClient[Any], UnseenRepository]:
    client: MongoClient[Any] = MongoClient(settings.mongo_uri, tz_aware=True)
    return client, UnseenRepository(client[settings.mongo_db_unseen])


def _print_summaries(summaries: list[RunSummary]) -> None:
    payload = [
        {
            "run_id": item.run_id,
            "sumber": item.source_code,
            "dataset": item.dataset_name,
            "crawl_date": item.crawl_date,
            "status": item.status,
            "jumlah_baru": item.new_count,
            "jumlah_diteruskan": item.carried_forward_count,
            "last_available_date": item.last_available_date.isoformat() if item.last_available_date else None,
            "pesan": item.message,
        }
        for item in summaries
    ]
    print(json.dumps(payload, ensure_ascii=False, indent=2))


def build_parser() -> argparse.ArgumentParser:
    """Create the `garda_crawler` CLI argument parser."""
    parser = argparse.ArgumentParser(prog="garda_crawler", description="Garda Inflasi Data Crawler")
    sub = parser.add_subparsers(dest="command", required=True)

    run_cmd = sub.add_parser("run", help="Run daily source adapters once")
    run_cmd.add_argument("--sumber", "--source", dest="sources", default=None)
    run_cmd.add_argument("--wilayah", "--regions", dest="regions", default=None)
    run_cmd.add_argument("--pemicu", "--trigger", dest="trigger", default=TriggerType.MANUAL)
    run_cmd.add_argument("--bronze-only", action="store_true", help="Write directly to Bronze + crawl_logs without MongoDB")

    bf_cmd = sub.add_parser("backfill", help="Run historical backfill over [start, end]")
    bf_cmd.add_argument("--sumber", "--source", dest="sources", required=True)
    bf_cmd.add_argument("--dari", "--from", dest="start_date", required=True)
    bf_cmd.add_argument("--sampai", "--to", dest="end_date", required=True)
    bf_cmd.add_argument("--wilayah", "--regions", dest="regions", default=None)
    bf_cmd.add_argument("--bronze-only", action="store_true", help="Write directly to Bronze + crawl_logs without MongoDB")

    for sync_name in ("sync-lapangan", "sync-field"):
        sync_cmd = sub.add_parser(sync_name, help="Sync internal field price entries from PostgreSQL")
        sync_cmd.add_argument("--pemicu", "--trigger", dest="trigger", default=TriggerType.MANUAL)

    sub.add_parser("status", help="Inspect last scheduled runs and crawl_runs summary")
    sub.add_parser("worker", help="Run 24/7 scheduler worker and HTTP control plane")

    health_cmd = sub.add_parser("health", help="Check worker heartbeat freshness")
    health_cmd.add_argument("--max-age", type=int, default=900)

    return parser


def main(argv: Sequence[str] | None = None) -> int:
    """Execute CLI command and return POSIX exit code."""
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s [%(name)s] %(message)s",
    )
    parser = build_parser()
    args = parser.parse_args(list(argv) if argv is not None else None)
    settings = CrawlerSettings.from_env(require_mongo=False)
    use_bronze_only = bool(getattr(args, "bronze_only", False)) or settings.bronze_only

    if args.command == "health":
        healthy = is_heartbeat_healthy(settings.heartbeat_path, max_age_seconds=args.max_age)
        print("OK" if healthy else "STALE")
        return 0 if healthy else 1

    if args.command == "worker":
        CrawlerWorker(settings).run_forever()
        return 0

    if use_bronze_only or not settings.mongo_uri:
        client, repository = None, BronzeOnlyRepository()
    else:
        client, repository = _open_repository(settings)
    try:
        if args.command == "run":
            orchestrator = CrawlOrchestrator(settings, repository)
            sources = _parse_csv(args.sources) or list(DEFAULT_ACTIVE_SOURCES)
            summaries = orchestrator.run_sources(
                sources,
                trigger=args.trigger,
                regions=_parse_csv(args.regions),
            )
            _print_summaries(summaries)
            return 1 if any(s.status == SourceStatus.ERROR for s in summaries) else 0

        if args.command == "backfill":
            start = date.fromisoformat(args.start_date)
            end = date.fromisoformat(args.end_date)
            if start > end:
                raise ValueError("--dari tidak boleh sesudah --sampai")
            orchestrator = CrawlOrchestrator(settings, repository)
            summaries = orchestrator.run_sources(
                _parse_csv(args.sources),
                trigger=TriggerType.BACKFILL,
                regions=_parse_csv(args.regions),
                start_date=start,
                end_date=end,
            )
            _print_summaries(summaries)
            return 1 if any(s.status == SourceStatus.ERROR for s in summaries) else 0

        if args.command in ("sync-lapangan", "sync-field"):
            repository.ensure_indexes()
            reader = PostgresFieldReader(settings.postgres_url) if settings.postgres_url else None
            if isinstance(repository, BronzeOnlyRepository):
                if reader is None:
                    payload_out = {"status": SourceStatus.ERROR, "ditulis": 0, "pesan": "DATABASE_URL kosong"}
                    print(json.dumps(payload_out, ensure_ascii=False, indent=2))
                    return 1
                with reader:
                    entries = reader.fetch_all()
                now = utc_now()
                logs = PostgresCrawlLogRepository(settings.postgres_url) if settings.postgres_url else None
                store = BronzeStore(
                    local_dir=settings.bronze_dir,
                    minio_endpoint=settings.minio_endpoint,
                    minio_access_key=settings.minio_access_key,
                    minio_secret_key=settings.minio_secret_key,
                    minio_bucket=settings.minio_bucket_raw,
                    minio_secure=settings.minio_secure,
                    crawl_logs=logs,
                )
                capture = RawBronzeCapture(
                    source_code="LAPANGAN",
                    dataset_name="entri_harga",
                    endpoint="postgres://entri_harga",
                    params={"sumber": "LAPANGAN"},
                    payload={"records": entries},
                    capture_date=to_wib_date(now),
                )
                res = store.persist_capture(capture, to_wib_date(now))
                print(
                    json.dumps(
                        {
                            "status": SourceStatus.FRESH,
                            "ditulis": len(entries),
                            "bronze_uri": res.uri,
                            "bronze_status": res.status,
                        },
                        ensure_ascii=False,
                        indent=2,
                    )
                )
                return 0

            outcome = sync_field_entries(reader, repository, utc_now(), args.trigger)
            print(
                json.dumps(
                    {
                        "run_id": outcome.run_id,
                        "status": outcome.status,
                        "ditulis": outcome.written_count,
                        "dihapus": outcome.deleted_count,
                        "ditolak": outcome.rejected_count,
                        "diteruskan": outcome.carried_forward_count,
                        "pesan": outcome.message,
                    },
                    ensure_ascii=False,
                    indent=2,
                )
            )
            return 1 if outcome.status == SourceStatus.ERROR else 0

        if args.command == "status":
            if isinstance(repository, BronzeOnlyRepository):
                summary: dict[str, Any] = {"mode": "bronze", "bronze_dir": settings.bronze_dir, "datasets": []}
                if settings.postgres_url:
                    import psycopg
                    with psycopg.connect(settings.postgres_url, connect_timeout=5) as conn:
                        with conn.cursor() as cur:
                            cur.execute(
                                """
                                SELECT DISTINCT ON (source_code, dataset_code)
                                    source_code, dataset_code, status, bronze_uri, crawled_at
                                FROM crawl_logs
                                ORDER BY source_code, dataset_code, crawled_at DESC, id DESC
                                """
                            )
                            summary["datasets"] = [
                                {
                                    "sumber": r[0],
                                    "dataset": r[1],
                                    "status": r[2],
                                    "bronze_uri": r[3],
                                    "crawled_at": r[4].isoformat() if r[4] else None,
                                }
                                for r in cur.fetchall()
                            ]
                print(json.dumps(summary, default=str, ensure_ascii=False, indent=2))
                return 0
            slots = list(repository.db[SCHEDULE_COLLECTION].find())
            print(json.dumps({"jadwal": slots}, default=str, ensure_ascii=False, indent=2))
            return 0
    finally:
        if client is not None:
            client.close()

    return 2
