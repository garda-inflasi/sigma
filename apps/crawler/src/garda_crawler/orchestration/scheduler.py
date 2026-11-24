"""Daily (06:00 WIB) & hourly field-sync worker scheduler + healthcheck heartbeat."""

import logging
import threading
from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from pymongo import MongoClient
from pymongo.database import Database

from garda_crawler.adapters.field_entries import sync_field_entries
from garda_crawler.adapters.registry import DEFAULT_ACTIVE_SOURCES
from garda_crawler.config.settings import CrawlerSettings
from garda_crawler.domain.enums import TriggerType
from garda_crawler.orchestration.control_server import (
    CrawlerControlState,
    start_control_server_background,
)
from garda_crawler.orchestration.orchestrator import CrawlOrchestrator
from garda_crawler.storage.lock_manager import MongoLeaseLock, default_lock_owner
from garda_crawler.storage.run_ledger import SCHEDULE_COLLECTION, RunLedger
from garda_crawler.storage.unseen_repository import UnseenRepository
from garda_crawler.transport.postgres_reader import PostgresFieldReader
from garda_crawler.transport.time_utils import WIB_TZ, utc_now

logger = logging.getLogger(__name__)

DAILY_RUN_HOUR_WIB = 6
HEARTBEAT_INTERVAL_SECONDS = 30
HEARTBEAT_MAX_AGE_SECONDS = 900


def write_heartbeat(path: str | Path, moment: datetime | None = None) -> None:
    """Persist ISO UTC timestamp for container HEALTHCHECK probing."""
    target = Path(path)
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text((moment or utc_now()).isoformat(), encoding="utf-8")


def is_heartbeat_healthy(
    path: str | Path,
    max_age_seconds: int = HEARTBEAT_MAX_AGE_SECONDS,
    now_utc: datetime | None = None,
) -> bool:
    """Return True if the worker heartbeat file exists and is within the freshness window."""
    target = Path(path)
    if not target.exists():
        return False
    try:
        raw = target.read_text(encoding="utf-8").strip()
        recorded = datetime.fromisoformat(raw)
        if recorded.tzinfo is None:
            recorded = recorded.replace(tzinfo=UTC)
        age = ((now_utc or utc_now()) - recorded.astimezone(UTC)).total_seconds()
        return 0 <= age <= max_age_seconds or age < 0
    except ValueError:
        return False


def run_if_due(
    db: Database,
    slot_name: str,
    slot_key: str,
    task_fn: Callable[[], Any],
    owner_id: str | None = None,
) -> bool:
    """Execute `task_fn` atomically under distributed lock if `slot_key` has not yet completed."""
    ledger = RunLedger(db)
    if ledger.get_schedule_slot(slot_name) == slot_key:
        return False

    owner = owner_id or default_lock_owner()
    with MongoLeaseLock(db, f"jadwal:{slot_name}", owner=owner) as lock:
        if not lock.acquired:
            return False
        if ledger.get_schedule_slot(slot_name) == slot_key:
            return False
        task_fn()
        ledger.mark_schedule_slot(slot_name, slot_key, utc_now())
        return True


def _run_daily_cycle(settings: CrawlerSettings, db: Database) -> None:
    repo = UnseenRepository(db)
    orchestrator = CrawlOrchestrator(settings, repo)
    orchestrator.run_sources(DEFAULT_ACTIVE_SOURCES, trigger=TriggerType.SCHEDULED, now_utc=utc_now())


def _run_field_sync_cycle(settings: CrawlerSettings, db: Database) -> None:
    repo = UnseenRepository(db)
    repo.ensure_indexes()
    reader = PostgresFieldReader(settings.postgres_url) if settings.postgres_url else None
    sync_field_entries(reader, repo, utc_now(), TriggerType.SCHEDULED)


class CrawlerWorker:
    """Long-running background scheduler with embedded HTTP control plane."""

    def __init__(
        self,
        settings: CrawlerSettings,
        *,
        db: Database | None = None,
    ) -> None:
        self._settings = settings
        self._mongo_client: MongoClient[Any] | None = None
        if db is None:
            self._mongo_client = MongoClient(settings.mongo_uri, tz_aware=True)
            self._db = self._mongo_client[settings.mongo_db_unseen]
        else:
            self._db = db
        self._stop_event = threading.Event()
        self.control_state = CrawlerControlState(self._db)

    def tick(self, now_utc: datetime | None = None) -> None:
        """Evaluate scheduled daily/hourly slots once and update heartbeat."""
        moment = now_utc or utc_now()
        local_wib = moment.astimezone(WIB_TZ)
        daily_slot = local_wib.date().isoformat()
        hourly_slot = local_wib.strftime("%Y-%m-%dT%H")

        try:
            run_if_due(
                self._db,
                "lapangan",
                hourly_slot,
                lambda: _run_field_sync_cycle(self._settings, self._db),
            )
        except Exception as exc:
            logger.exception("hourly field sync error: %s", exc)

        if local_wib.hour >= DAILY_RUN_HOUR_WIB:
            try:
                run_if_due(
                    self._db,
                    "harian",
                    daily_slot,
                    lambda: _run_daily_cycle(self._settings, self._db),
                )
            except Exception as exc:
                logger.exception("daily scheduled crawl error: %s", exc)

        write_heartbeat(self._settings.heartbeat_path, moment)

    def run_forever(self) -> None:
        """Start internal HTTP control server and block until stopped."""
        UnseenRepository(self._db).ensure_indexes()
        server = start_control_server_background(
            self.control_state,
            settings=self._settings,
            port=self._settings.control_port,
        )
        logger.info(
            "garda-crawler worker started (control HTTP on :%d, schedule collection=%s)",
            self._settings.control_port,
            SCHEDULE_COLLECTION,
        )
        try:
            while not self._stop_event.is_set():
                self.tick()
                self._stop_event.wait(HEARTBEAT_INTERVAL_SECONDS)
        finally:
            server.shutdown()
            server.server_close()
            if self._mongo_client is not None:
                self._mongo_client.close()

    def stop(self) -> None:
        """Signal worker loop to exit gracefully."""
        self._stop_event.set()
