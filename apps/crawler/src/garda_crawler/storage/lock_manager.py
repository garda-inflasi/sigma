"""MongoDB distributed lease lock manager stored in the `crawl_kunci` collection."""

import os
import socket
import threading
from datetime import timedelta
from types import TracebackType

from pymongo import ReturnDocument
from pymongo.database import Database
from pymongo.errors import DuplicateKeyError

from garda_crawler.transport.time_utils import utc_now

LOCK_COLLECTION = "crawl_kunci"
DEFAULT_LEASE_SECONDS = 600


def default_lock_owner() -> str:
    """Build a unique host:pid identifier for lock ownership."""
    return f"{socket.gethostname()}:{os.getpid()}"


class MongoLeaseLock:
    """Distributed TTL lease lock with automatic background heartbeat renewal."""

    def __init__(
        self,
        db: Database,
        lock_key: str,
        owner: str | None = None,
        lease_seconds: int = DEFAULT_LEASE_SECONDS,
    ) -> None:
        self._collection = db[LOCK_COLLECTION]
        self._key = lock_key
        self._owner = owner or default_lock_owner()
        self._lease_seconds = max(30, lease_seconds)
        self._stop_event = threading.Event()
        self._renew_thread: threading.Thread | None = None
        self.acquired = False

    def acquire(self) -> bool:
        """Attempt to acquire or reclaim an expired lease lock."""
        now = utc_now()
        expires_at = now + timedelta(seconds=self._lease_seconds)
        try:
            doc = self._collection.find_one_and_update(
                {
                    "_id": self._key,
                    "$or": [{"kedaluwarsa": {"$lte": now}}, {"pemilik": self._owner}],
                },
                {
                    "$set": {
                        "pemilik": self._owner,
                        "kedaluwarsa": expires_at,
                        "diambil_pada": now,
                    }
                },
                upsert=True,
                return_document=ReturnDocument.AFTER,
            )
            self.acquired = doc is not None and doc.get("pemilik") == self._owner
        except DuplicateKeyError:
            self.acquired = False
        return self.acquired

    def renew(self) -> None:
        """Extend the active lease TTL while this owner still holds the lock."""
        now = utc_now()
        self._collection.update_one(
            {"_id": self._key, "pemilik": self._owner},
            {"$set": {"kedaluwarsa": now + timedelta(seconds=self._lease_seconds)}},
        )

    def release(self) -> None:
        """Release the lease only if still owned by this instance."""
        self._stop_event.set()
        if self._renew_thread is not None and self._renew_thread.is_alive():
            self._renew_thread.join(timeout=2)
        self._collection.delete_one({"_id": self._key, "pemilik": self._owner})
        self.acquired = False

    def start_heartbeat(self) -> None:
        """Spawn daemon thread refreshing the lease at 1/3 of lease TTL."""
        interval = max(10.0, self._lease_seconds / 3.0)

        def _beat() -> None:
            while not self._stop_event.wait(interval):
                try:
                    self.renew()
                except Exception:
                    # Keep retrying until release or shutdown; expiry protects against hangs.
                    continue

        self._renew_thread = threading.Thread(
            target=_beat,
            name=f"lock-renew-{self._key}",
            daemon=True,
        )
        self._renew_thread.start()

    def __enter__(self) -> "MongoLeaseLock":
        if self.acquire():
            self.start_heartbeat()
        return self

    def __exit__(
        self,
        exc_type: type[BaseException] | None,
        exc_val: BaseException | None,
        exc_tb: TracebackType | None,
    ) -> None:
        if self.acquired:
            self.release()
