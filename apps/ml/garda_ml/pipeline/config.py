"""Konfigurasi pipeline dari variabel lingkungan dan konstanta kontrak."""

from __future__ import annotations

import os
from collections.abc import Mapping
from dataclasses import dataclass

HORIZON: tuple[int, ...] = (7, 14, 30)
KUANTIL: tuple[float, ...] = (0.1, 0.5, 0.9)
LAG_HARI: tuple[int, ...] = (1, 3, 7, 14, 30)

# Bobot sampel input lapangan menurut kontrak. Status lain (mis. ditolak) tidak dipakai.
BOBOT_LAPANGAN: dict[str, float] = {"DIPUBLIKASI": 1.0, "DIVALIDASI": 0.8, "DIAJUKAN": 0.3}

# Urutan sumber harga resmi. Satu seri memakai satu sumber utama agar level harga tidak melompat.
SUMBER_HARGA_RESMI: tuple[str, ...] = ("PIHPS", "SP2KP", "BAPANAS")

# Hanya harga konsumen yang sebanding dengan PIHPS (pasar tradisional, konsumen).
TIPE_HARGA_LAPANGAN = "KONSUMEN"

# Arsip reanalisis cuaca baru tersedia beberapa hari kemudian. Fitur cuaca pada t hanya
# memakai data sampai t-2 agar latih dan inferensi melihat ketersediaan yang sama.
JEDA_CUACA_HARI = 2

# Asumsi konservatif tanggal rilis (hari setelah akhir periode) bila tidak diketahui.
# Nilai bulanan ditetapkan kontrak; triwulanan dan tahunan mengikuti kelambatan rilis BPS.
JEDA_RILIS_HARI: dict[str, int] = {"HARIAN": 1, "BULANAN": 35, "TRIWULANAN": 60, "TAHUNAN": 120}

# Batas panjang riwayat (jumlah titik asli) untuk tiap keluarga model.
MIN_TITIK_RAMALAN = 8
MIN_TITIK_HOLT_WINTERS = 30
MIN_TITIK_SARIMAX = 90
MIN_TITIK_GLOBAL = 90

# Baris latih: umur titik asli maksimum; label harus berasal dari titik asli sedekat ini.
MAKS_UMUR_BARIS_HARI = 7
MAKS_UMUR_LABEL_HARI = 3
# Di atas umur ini model global tidak dipakai untuk inferensi.
MAKS_UMUR_INFERENSI_HARI = 14

# Di atas umur ini seri tidak diramal: horizon efektifnya terlalu jauh dari data asli.
MAKS_UMUR_RAMALAN_HARI = 45

# Aturan seleksi model. Kandidat non-baseline hanya dipublikasikan bila mengalahkan baseline
# terbaik di SETIAP lipatan seleksi dan rata-rata MAPE-nya lebih rendah minimal MARGIN_BASELINE.
MARGIN_BASELINE = 0.05
N_LIPATAN_SELEKSI = 4
N_LIPATAN_HOLDOUT = 1
# Di bawah jumlah lipatan seleksi ini hanya baseline yang boleh dipublikasikan.
MIN_LIPATAN_SELEKSI = 2
LANGKAH_LIPATAN_HARI = 30
# Tiap lipatan memuat beberapa titik asal mingguan agar skornya tidak bertumpu pada satu ramalan.
N_ASAL_PER_LIPATAN = 4
JARAK_ASAL_HARI = 7


def _bulat(env: Mapping[str, str], nama: str, bawaan: int, minimum: int = 1) -> int:
    mentah = env.get(nama, "").strip()
    if not mentah:
        return bawaan
    try:
        nilai = int(mentah)
    except ValueError as err:
        raise ValueError(f"{nama} harus bilangan bulat, didapat {mentah!r}") from err
    if nilai < minimum:
        raise ValueError(f"{nama} minimal {minimum}, didapat {nilai}")
    return nilai


@dataclass(frozen=True)
class Konfigurasi:
    mongo_uri: str
    db_unseen: str = "dataunseen"
    db_safe: str = "datasafe"
    n_jobs: int = 2
    maks_baris_latih: int = 150_000
    hari_riwayat: int = 1100
    panjang_riwayat_keluaran: int = 60
    interval_seleksi_jam: int = 24
    lease_detik: int = 300
    offset_menit: int = 5
    versi_kode: str = ""

    @classmethod
    def dari_env(cls, env: Mapping[str, str] | None = None) -> "Konfigurasi":
        env = os.environ if env is None else env
        uri = env.get("MONGO_URI", "").strip() or env.get("MONGO_URL", "").strip()
        if not uri:
            raise ValueError("MONGO_URI (atau MONGO_URL) wajib diisi")
        return cls(
            mongo_uri=uri,
            db_unseen=env.get("MONGO_DB_UNSEEN", "").strip() or "dataunseen",
            db_safe=env.get("MONGO_DB_SAFE", "").strip() or "datasafe",
            n_jobs=_bulat(env, "ML_N_JOBS", min(4, os.cpu_count() or 1)),
            maks_baris_latih=_bulat(env, "ML_MAKS_BARIS_LATIH", 150_000, 1000),
            hari_riwayat=_bulat(env, "ML_HARI_RIWAYAT", 1100, 60),
            panjang_riwayat_keluaran=_bulat(env, "ML_PANJANG_RIWAYAT", 60),
            interval_seleksi_jam=_bulat(env, "ML_INTERVAL_SELEKSI_JAM", 24),
            lease_detik=_bulat(env, "ML_LEASE_DETIK", 300, 30),
            offset_menit=_bulat(env, "ML_OFFSET_MENIT", 5, 0),
            versi_kode=env.get("GIT_SHA", "").strip(),
        )
