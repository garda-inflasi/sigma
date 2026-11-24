"""Immutable audit log of crawl_runs and latest schedule slot state (crawl_jadwal)."""

from datetime import date, datetime
from typing import Any

from pymongo import DESCENDING
from pymongo.database import Database

from garda_crawler import __version__
from garda_crawler.transport.time_utils import to_midnight_utc

RUNS_COLLECTION = "crawl_runs"
SCHEDULE_COLLECTION = "crawl_jadwal"


class RunLedger:
    """Manages crawl execution logs (`crawl_runs`) and schedule watermarks (`crawl_jadwal`)."""

    def __init__(self, db: Database) -> None:
        self._db = db

    def ensure_indexes(self) -> None:
        """Create query indexes for run history lookup."""
        runs = self._db[RUNS_COLLECTION]
        runs.create_index([("sumber", 1), ("dataset", 1), ("selesai", DESCENDING)])
        runs.create_index([("crawl_date", 1)])

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
    ) -> None:
        """Insert or update a crawl execution audit record."""
        doc: dict[str, Any] = {
            "_id": run_id,
            "crawler": f"garda-crawler/{__version__}",
            "sumber": source_code,
            "dataset": dataset_name,
            "crawl_date": crawl_date,
            "mulai": started_at,
            "selesai": finished_at,
            "status": status,
            "jumlah_baru": new_count,
            "jumlah_diteruskan": carried_forward_count,
            "last_available_date": to_midnight_utc(last_available_date) if last_available_date else None,
            "pesan": message,
            "pemicu": trigger,
        }
        self._db[RUNS_COLLECTION].replace_one({"_id": run_id}, doc, upsert=True)

    def get_schedule_slot(self, slot_name: str) -> str | None:
        """Return the last completed slot key (e.g., YYYY-MM-DD or YYYY-MM-DDTHH)."""
        doc = self._db[SCHEDULE_COLLECTION].find_one({"_id": slot_name})
        return str(doc["terakhir"]) if doc and doc.get("terakhir") else None

    def mark_schedule_slot(self, slot_name: str, slot_key: str, finished_at: datetime) -> None:
        """Record completion watermark for a scheduled slot."""
        self._db[SCHEDULE_COLLECTION].replace_one(
            {"_id": slot_name},
            {"_id": slot_name, "terakhir": slot_key, "selesai_pada": finished_at},
            upsert=True,
        )
