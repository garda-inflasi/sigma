"""Tipe data bersama dan nilai enum kontrak."""

from collections import Counter
from dataclasses import dataclass, field
from datetime import date
from typing import Any

FRESH = "FRESH"
STALE = "STALE_CARRIED_FORWARD"
ERROR = "ERROR"

PEMICU_JADWAL = "JADWAL"
PEMICU_MANUAL = "MANUAL"
PEMICU_BACKFILL = "BACKFILL"

AWALAN_DILEWATI = "DILEWATI: "


def pesan_dilewati(alasan: str, tindakan: str) -> str:
    """Pesan untuk sumber yang tidak dijalankan karena konfigurasi atau aturan akses.

    Kegagalan jaringan dan respons sungguhan tidak memakai awalan ini.
    """
    return f"{AWALAN_DILEWATI}{alasan}; tindakan: {tindakan}"


KOLEKSI_RAW = (
    "raw_pangan_harian",
    "raw_iklim_bmkg",
    "raw_bps_makro",
    "raw_konsumsi_pendapatan",
    "raw_neraca_ekonomi",
    "raw_tenaga_kerja",
    "raw_sektor_primer",
)


@dataclass(frozen=True)
class DefinisiDataset:
    nama: str
    koleksi: str


@dataclass(frozen=True)
class Seri:
    """Identitas dan metadata satu deret."""

    sumber: str
    dataset: str
    koleksi: str
    kunci_seri: str
    indikator: str
    satuan: str
    frekuensi: str
    komoditas_kode: str | None = None
    kabupaten_kode: str | None = None
    url_sumber: str = ""


@dataclass(frozen=True)
class Titik:
    """Satu nilai asli dari sumber."""

    seri: Seri
    periode: date
    nilai: float
    url_sumber: str
    atribut: dict[str, Any] = field(default_factory=dict)


@dataclass
class HasilAmbil:
    titik: list[Titik] = field(default_factory=list)
    seri_gagal: list[Seri] = field(default_factory=list)
    galat: list[str] = field(default_factory=list)
    ditolak: Counter = field(default_factory=Counter)
    permintaan: int = 0
    permintaan_gagal: int = 0
    dilewati: str | None = None
