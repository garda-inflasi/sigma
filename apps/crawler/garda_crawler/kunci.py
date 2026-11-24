"""Kunci antar-instance di MongoDB dengan masa sewa yang diperpanjang selama pekerjaan berjalan."""

import logging
import os
import socket
import threading
import uuid
from datetime import datetime, timedelta
from types import TracebackType

from pymongo import ReturnDocument
from pymongo.database import Database
from pymongo.errors import DuplicateKeyError, PyMongoError

from garda_crawler.waktu import sekarang_utc

log = logging.getLogger(__name__)

SEWA_DETIK = 600


def pemilik_proses() -> str:
    return f"{socket.gethostname()}:{os.getpid()}:{uuid.uuid4().hex[:8]}"


class KunciMongo:
    def __init__(self, db: Database, nama: str, pemilik: str | None = None, sewa_detik: int = SEWA_DETIK) -> None:
        self.kol = db["crawl_kunci"]
        self.nama = nama
        self.pemilik = pemilik or pemilik_proses()
        self.sewa = timedelta(seconds=sewa_detik)
        self._henti = threading.Event()
        self._utas: threading.Thread | None = None

    def ambil(self, sekarang: datetime | None = None) -> bool:
        waktu = sekarang or sekarang_utc()
        try:
            doc = self.kol.find_one_and_update(
                {"_id": self.nama, "$or": [{"kedaluwarsa": {"$lte": waktu}}, {"pemilik": self.pemilik}]},
                {"$set": {"pemilik": self.pemilik, "kedaluwarsa": waktu + self.sewa, "diambil_pada": waktu}},
                upsert=True,
                return_document=ReturnDocument.AFTER,
            )
        except DuplicateKeyError:
            return False  # Baris ada dan masih disewa pemilik lain.
        return bool(doc) and doc.get("pemilik") == self.pemilik

    def perpanjang(self) -> bool:
        waktu = sekarang_utc()
        hasil = self.kol.update_one(
            {"_id": self.nama, "pemilik": self.pemilik}, {"$set": {"kedaluwarsa": waktu + self.sewa}}
        )
        return hasil.matched_count == 1

    def lepas(self) -> None:
        self.kol.delete_one({"_id": self.nama, "pemilik": self.pemilik})

    def _jaga(self) -> None:
        while not self._henti.wait(self.sewa.total_seconds() / 3):
            try:
                if not self.perpanjang():
                    log.error("kunci %s hilang saat pekerjaan berjalan", self.nama)
                    return
            except PyMongoError as exc:
                log.warning("gagal memperpanjang kunci %s: %s", self.nama, type(exc).__name__)

    def __enter__(self) -> "KunciMongo":
        self._utas = threading.Thread(target=self._jaga, name=f"kunci-{self.nama}", daemon=True)
        self._utas.start()
        return self

    def __exit__(
        self, _tipe: type[BaseException] | None, _galat: BaseException | None, _jejak: TracebackType | None
    ) -> None:
        self._henti.set()
        if self._utas:
            self._utas.join(timeout=5)
        try:
            self.lepas()
        except PyMongoError as exc:
            log.warning("gagal melepas kunci %s: %s", self.nama, type(exc).__name__)
