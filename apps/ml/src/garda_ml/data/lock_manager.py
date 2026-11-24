"""Lease-based distributed lock protecting training cycles in MongoDB."""

import logging
import os
import socket
import threading
import uuid
from datetime import UTC, datetime, timedelta

from pymongo import ReturnDocument
from pymongo.database import Database
from pymongo.errors import DuplicateKeyError

from garda_ml.domain.errors import CycleLockError

logger = logging.getLogger("garda_ml.pipeline")

LOCK_COLLECTION = "ml_worker_locks"


class CycleLeaseLock:
    """Distributed TTL lease lock with heartbeat renewal and zombie write fencing."""

    LOCK_NAME = "pipeline"

    def __init__(self, db: Database, lease_seconds: int = 300) -> None:
        self._collection = db[LOCK_COLLECTION]
        self._lease = timedelta(seconds=lease_seconds)
        self.owner = f"{socket.gethostname()}:{os.getpid()}:{uuid.uuid4().hex[:8]}"
        self._stop_event = threading.Event()
        self._lost_event = threading.Event()
        self._thread: threading.Thread | None = None

    @staticmethod
    def _now() -> datetime:
        return datetime.now(UTC)

    def acquire(self) -> bool:
        """Acquire or reclaim an expired pipeline lease lock."""
        now = self._now()
        try:
            doc = self._collection.find_one_and_update(
                {
                    "_id": self.LOCK_NAME,
                    "$or": [{"kedaluwarsa": {"$lte": now}}, {"pemilik": self.owner}],
                },
                {
                    "$set": {
                        "pemilik": self.owner,
                        "kedaluwarsa": now + self._lease,
                        "diambil": now,
                    }
                },
                upsert=True,
                return_document=ReturnDocument.AFTER,
            )
        except DuplicateKeyError:
            return False

        if not doc or doc.get("pemilik") != self.owner:
            return False

        self._thread = threading.Thread(
            target=self._renew_loop,
            name="ml-cycle-lock-renew",
            daemon=True,
        )
        self._thread.start()
        return True

    def _renew_loop(self) -> None:
        interval = max(5.0, self._lease.total_seconds() / 3.0)
        while not self._stop_event.wait(interval):
            try:
                now = self._now()
                res = self._collection.update_one(
                    {"_id": self.LOCK_NAME, "pemilik": self.owner},
                    {"$set": {"kedaluwarsa": now + self._lease}},
                )
                if res.matched_count == 0:
                    self._lost_event.set()
                    logger.error("cycle lease lost to another worker")
                    return
            except Exception:
                logger.warning("failed renewing cycle lease", exc_info=True)

    def assert_held(self) -> None:
        """Fencing guard: abort writes immediately if lease ownership moved."""
        if self._lost_event.is_set():
            raise CycleLockError("lease siklus hilang")
        doc = self._collection.find_one({"_id": self.LOCK_NAME}, {"pemilik": 1})
        if not doc or doc.get("pemilik") != self.owner:
            self._lost_event.set()
            raise CycleLockError("lease siklus hilang")

    def release(self) -> None:
        """Stop heartbeat renewal and expire own lease."""
        self._stop_event.set()
        if self._thread is not None and self._thread.is_alive():
            self._thread.join(timeout=5)
        try:
            self._collection.update_one(
                {"_id": self.LOCK_NAME, "pemilik": self.owner},
                {"$set": {"kedaluwarsa": self._now()}},
            )
        except Exception:
            logger.warning("failed releasing cycle lease", exc_info=True)
