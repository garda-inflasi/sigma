"""Penjadwal worker: crawl harian 06.00 WIB dan sinkron lapangan tiap jam.

Aman untuk banyak instance: tiap pekerjaan dijaga KunciMongo, dan penanda `crawl_jadwal` dicek
ulang setelah kunci didapat sehingga satu jadwal tidak dijalankan dua kali. Instance yang baru hidup
setelah 06.00 mengejar crawl hari itu.
"""

import logging
import signal
import threading
from collections.abc import Callable
from datetime import datetime
from pathlib import Path

from pymongo.database import Database
from pymongo.errors import PyMongoError

from garda_crawler.kunci import KunciMongo, pemilik_proses
from garda_crawler.waktu import WIB, sekarang_utc

log = logging.getLogger(__name__)

JAM_HARIAN = 6
INTERVAL_DETIK = 30
PEKERJAAN_HARIAN = "harian"
PEKERJAAN_LAPANGAN = "lapangan"


def slot_harian(waktu: datetime) -> str | None:
    """Slot crawl harian yang sudah jatuh tempo, atau None sebelum 06.00 WIB."""
    lokal = waktu.astimezone(WIB)
    return lokal.date().isoformat() if lokal.hour >= JAM_HARIAN else None


def slot_jam(waktu: datetime) -> str:
    return waktu.astimezone(WIB).strftime("%Y-%m-%dT%H")


def tulis_detak(berkas: Path, waktu: datetime | None = None) -> None:
    berkas.write_text((waktu or sekarang_utc()).isoformat(), encoding="utf-8")


def detak_sehat(berkas: Path, batas_detik: int = 180, sekarang: datetime | None = None) -> bool:
    try:
        terakhir = datetime.fromisoformat(berkas.read_text(encoding="utf-8").strip())
    except (OSError, ValueError):
        return False
    return ((sekarang or sekarang_utc()) - terakhir).total_seconds() <= batas_detik


def jalankan_bila_jatuh_tempo(
    db: Database, pekerjaan: str, slot: str, tugas: Callable[[], None], pemilik: str | None = None
) -> bool:
    """Menjalankan tugas sekali per slot lintas instance. True bila instance ini yang menjalankan."""
    jadwal = db["crawl_jadwal"]
    penanda = jadwal.find_one({"_id": pekerjaan})
    if penanda and penanda.get("terakhir", "") >= slot:
        return False
    kunci = KunciMongo(db, f"jadwal:{pekerjaan}", pemilik)
    if not kunci.ambil():
        return False
    with kunci:
        penanda = jadwal.find_one({"_id": pekerjaan})
        if penanda and penanda.get("terakhir", "") >= slot:
            return False
        log.info("mulai %s slot %s", pekerjaan, slot)
        tugas()
        jadwal.update_one(
            {"_id": pekerjaan}, {"$set": {"terakhir": slot, "selesai_pada": sekarang_utc()}}, upsert=True
        )
        log.info("selesai %s slot %s", pekerjaan, slot)
    return True


class Worker:
    def __init__(
        self,
        db: Database,
        tugas_harian: Callable[[], None],
        tugas_lapangan: Callable[[], None],
        berkas_detak: Path,
    ) -> None:
        self.db = db
        self.tugas = {PEKERJAAN_HARIAN: tugas_harian, PEKERJAAN_LAPANGAN: tugas_lapangan}
        self.berkas_detak = berkas_detak
        self.pemilik = pemilik_proses()
        self._henti = threading.Event()
        self._utas: dict[str, threading.Thread] = {}

    def henti(self, *_: object) -> None:
        self._henti.set()

    def _picu(self, pekerjaan: str, slot: str) -> None:
        utas = self._utas.get(pekerjaan)
        if utas and utas.is_alive():
            return  # Tanpa tumpang tindih: putaran sebelumnya masih berjalan.

        def badan() -> None:
            try:
                jalankan_bila_jatuh_tempo(self.db, pekerjaan, slot, self.tugas[pekerjaan], self.pemilik)
            except Exception as exc:  # noqa: BLE001 - worker harus tetap hidup
                log.error("pekerjaan %s gagal: %s", pekerjaan, type(exc).__name__)

        baru = threading.Thread(target=badan, name=f"pekerjaan-{pekerjaan}", daemon=True)
        self._utas[pekerjaan] = baru
        baru.start()

    def langkah(self, sekarang: datetime | None = None) -> None:
        waktu = sekarang or sekarang_utc()
        tulis_detak(self.berkas_detak, waktu)
        try:
            if slot := slot_harian(waktu):
                self._picu(PEKERJAAN_HARIAN, slot)
            self._picu(PEKERJAAN_LAPANGAN, slot_jam(waktu))
        except PyMongoError as exc:
            log.error("penjadwal: MongoDB bermasalah: %s", type(exc).__name__)

    def jalan(self) -> None:
        signal.signal(signal.SIGTERM, self.henti)
        signal.signal(signal.SIGINT, self.henti)
        log.info("worker aktif")
        while not self._henti.is_set():
            self.langkah()
            self._henti.wait(INTERVAL_DETIK)
        for utas in self._utas.values():
            utas.join(timeout=30)
        log.info("worker berhenti")
