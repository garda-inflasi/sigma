"""Akses MongoDB ``datasafe``: indeks, kunci lease, dan penulisan append-only."""

from __future__ import annotations

import logging
import math
import os
import socket
import threading
import uuid
from datetime import date, datetime, timedelta, timezone
from typing import Any

import numpy as np
import pandas as pd
from pymongo import ASCENDING, DESCENDING, ReturnDocument
from pymongo.database import Database
from pymongo.errors import DuplicateKeyError

log = logging.getLogger("garda_ml.pipeline")

K_SIKLUS = "ml_training_cycles"
K_INFLASI = "datasafe_inflasi_signals"
K_FORECAST = "datasafe_forecasts"
K_XAI = "datasafe_xai_explanations"
K_EWS = "datasafe_ews_signals"
K_MAKRO = "datasafe_macro_indicators"
K_KUNCI = "ml_worker_locks"  # operasional, bukan data hasil; satu dokumen per pipeline

KOLEKSI_HASIL = (K_MAKRO, K_FORECAST, K_XAI, K_INFLASI, K_EWS)
STATUS_FINAL_TERBACA = ("BERHASIL", "SEBAGIAN")

_KUNCI_BISNIS: dict[str, list[tuple[str, int]]] = {
    K_INFLASI: [("wilayah_kode", ASCENDING)],
    K_FORECAST: [("komoditas_kode", ASCENDING), ("kabupaten_kode", ASCENDING)],
    K_XAI: [("target", ASCENDING), ("komoditas_kode", ASCENDING), ("wilayah_kode", ASCENDING)],
    K_EWS: [("sinyal_kunci", ASCENDING)],
    K_MAKRO: [("domain", ASCENDING), ("indikator", ASCENDING), ("wilayah_kode", ASCENDING)],
}


def pastikan_indeks(db: Database) -> None:
    for nama, kunci in _KUNCI_BISNIS.items():
        koleksi = db[nama]
        koleksi.create_index([("cycle_id", ASCENDING)])
        koleksi.create_index([("cycle_timestamp", DESCENDING)])
        koleksi.create_index([*kunci, ("cycle_timestamp", DESCENDING)])
    db[K_EWS].create_index([("severity", ASCENDING), ("cycle_timestamp", DESCENDING)])
    siklus = db[K_SIKLUS]
    siklus.create_index([("cycle_id", ASCENDING)])
    siklus.create_index([("cycle_timestamp", DESCENDING)])
    siklus.create_index([("status", ASCENDING), ("cycle_timestamp", DESCENDING)])


def bersihkan(nilai: Any) -> Any:
    """Ubah tipe numpy/pandas menjadi tipe BSON. NaN dan inf menjadi ``None``."""
    if isinstance(nilai, dict):
        return {k: bersihkan(v) for k, v in nilai.items()}
    if isinstance(nilai, (list, tuple)):
        return [bersihkan(v) for v in nilai]
    if isinstance(nilai, (np.bool_, bool)):
        return bool(nilai)
    if isinstance(nilai, (np.integer,)):
        return int(nilai)
    if isinstance(nilai, (float, np.floating)):
        angka = float(nilai)
        return None if math.isnan(angka) or math.isinf(angka) else angka
    if nilai is pd.NaT:
        return None
    if isinstance(nilai, pd.Timestamp):
        return nilai.to_pydatetime().replace(tzinfo=timezone.utc) if nilai.tzinfo is None else nilai.to_pydatetime()
    if isinstance(nilai, datetime):
        return nilai
    if isinstance(nilai, date):
        return datetime(nilai.year, nilai.month, nilai.day, tzinfo=timezone.utc)
    return nilai


def tulis_dokumen(db: Database, koleksi: str, dokumen: list[dict], ukuran_batch: int = 1000) -> int:
    """Sisip dokumen berkunci deterministik. Duplikat adalah galat, bukan ditimpa diam-diam."""
    for awal in range(0, len(dokumen), ukuran_batch):
        db[koleksi].insert_many([bersihkan(d) for d in dokumen[awal : awal + ukuran_batch]], ordered=True)
    return len(dokumen)


def hapus_parsial(db: Database, cycle_id: str) -> None:
    """Hapus sisa tulisan siklus yang belum final (tidak pernah terlihat pembaca)."""
    status = (db[K_SIKLUS].find_one({"_id": cycle_id}, {"status": 1}) or {}).get("status")
    if status in STATUS_FINAL_TERBACA:
        raise RuntimeError(f"menolak menghapus siklus final {cycle_id}")
    for nama in KOLEKSI_HASIL:
        db[nama].delete_many({"cycle_id": cycle_id})


def hapus_siklus(db: Database, cycle_id: str) -> None:
    """Hapus satu siklus beserta dokumen hasilnya (khusus pemicu manual --paksa)."""
    for nama in KOLEKSI_HASIL:
        db[nama].delete_many({"cycle_id": cycle_id})
    db[K_SIKLUS].delete_one({"_id": cycle_id})


class KunciHilang(RuntimeError):
    """Lease diambil pekerja lain; siklus ini tidak boleh difinalkan."""


class KunciSiklus:
    """Kunci lease di Mongo: satu siklus berjalan, aman bila proses mati.

    Lease diperpanjang utas latar. Proses yang mati melepaskan kunci secara alami saat
    lease kedaluwarsa, sehingga restart tidak tersangkut.
    """

    NAMA = "pipeline"

    def __init__(self, db: Database, lease_detik: int = 300) -> None:
        self.koleksi = db[K_KUNCI]
        self.lease = timedelta(seconds=lease_detik)
        self.pemilik = f"{socket.gethostname()}:{os.getpid()}:{uuid.uuid4().hex[:8]}"
        self._henti = threading.Event()
        self._utas: threading.Thread | None = None
        self._hilang = threading.Event()

    def _sekarang(self) -> datetime:
        return datetime.now(timezone.utc)

    def ambil(self) -> bool:
        kini = self._sekarang()
        try:
            hasil = self.koleksi.find_one_and_update(
                {"_id": self.NAMA, "$or": [{"kedaluwarsa": {"$lte": kini}}, {"pemilik": self.pemilik}]},
                {"$set": {"pemilik": self.pemilik, "kedaluwarsa": kini + self.lease, "diambil": kini}},
                upsert=True,
                return_document=ReturnDocument.AFTER,
            )
        except DuplicateKeyError:  # dokumen ada dan masih dipegang pekerja lain
            return False
        if not hasil or hasil.get("pemilik") != self.pemilik:
            return False
        self._utas = threading.Thread(target=self._detak, name="kunci-siklus", daemon=True)
        self._utas.start()
        return True

    def _detak(self) -> None:
        jeda = max(5.0, self.lease.total_seconds() / 3)
        while not self._henti.wait(jeda):
            try:
                kini = self._sekarang()
                hasil = self.koleksi.update_one(
                    {"_id": self.NAMA, "pemilik": self.pemilik},
                    {"$set": {"kedaluwarsa": kini + self.lease}},
                )
                if hasil.matched_count == 0:
                    self._hilang.set()
                    log.error("kunci siklus diambil pekerja lain")
                    return
            except Exception:  # noqa: BLE001 - gangguan sesaat tidak boleh mematikan utas detak
                log.warning("gagal memperpanjang kunci siklus", exc_info=True)

    def pastikan(self) -> None:
        """Pagar sebelum menulis: batalkan bila lease sudah berpindah pemilik."""
        if self._hilang.is_set():
            raise KunciHilang("lease siklus hilang")
        dok = self.koleksi.find_one({"_id": self.NAMA}, {"pemilik": 1})
        if not dok or dok.get("pemilik") != self.pemilik:
            self._hilang.set()
            raise KunciHilang("lease siklus hilang")

    def lepas(self) -> None:
        self._henti.set()
        if self._utas is not None:
            self._utas.join(timeout=5)
        try:
            self.koleksi.update_one(
                {"_id": self.NAMA, "pemilik": self.pemilik},
                {"$set": {"kedaluwarsa": self._sekarang()}},
            )
        except Exception:  # noqa: BLE001 - lease tetap kedaluwarsa sendiri
            log.warning("gagal melepas kunci siklus", exc_info=True)
