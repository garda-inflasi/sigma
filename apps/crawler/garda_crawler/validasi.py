"""Validasi nilai sebelum disimpan. Nilai yang gagal ditolak dan dihitung, tidak pernah diperbaiki."""

import math
from typing import Any

HARGA_MAKS = 10_000_000
# Pita kewajaran terhadap harga acuan untuk menangkap satuan keliru (mis. telur per butir, bukan per kg).
RASIO_MIN = 0.2
RASIO_MAKS = 5.0

ALASAN_BUKAN_ANGKA = "harga_bukan_angka"
ALASAN_NON_POSITIF = "harga_non_positif"
ALASAN_DI_LUAR_BATAS = "harga_di_luar_batas"
ALASAN_SATUAN = "satuan_keliru"


def baca_angka(mentah: Any) -> float | None:
    """Membaca angka gaya Indonesia atau Inggris, setara `kit.Angka` di konektor Go.

    Pemisah terakhir adalah desimal, kecuali ia satu-satunya jenis pemisah dan diikuti
    tepat tiga digit atau muncul berulang (pemisah ribuan). PIHPS menulis "15,850" = 15850.
    """
    if mentah is None or isinstance(mentah, bool):
        return None
    if isinstance(mentah, (int, float)):
        nilai = float(mentah)
        return nilai if math.isfinite(nilai) else None
    if not isinstance(mentah, str):
        return None

    teks = mentah.strip()
    if teks.startswith("Rp"):
        teks = teks[2:]
    teks = teks.replace(" ", "")
    if teks in ("", "-"):
        return None

    titik, koma = teks.rfind("."), teks.rfind(",")
    desimal = ""
    if titik >= 0 and koma >= 0:
        desimal = "," if koma > titik else "."
    elif titik >= 0 or koma >= 0:
        pemisah = "," if koma >= 0 else "."
        if teks.count(pemisah) == 1 and len(teks) - teks.rfind(pemisah) - 1 != 3:
            desimal = pemisah

    if desimal == "":
        teks = teks.replace(".", "").replace(",", "")
    else:
        ribuan = "." if desimal == "," else ","
        teks = teks.replace(ribuan, "").replace(desimal, ".", 1)
    try:
        nilai = float(teks)
    except ValueError:
        return None
    return nilai if math.isfinite(nilai) else None


def periksa_harga(
    nilai: float | None,
    harga_acuan: float | None,
    satuan: str | None = None,
    satuan_master: str | None = None,
) -> str | None:
    """Mengembalikan alasan penolakan, atau None bila nilai sah."""
    if nilai is None:
        return ALASAN_BUKAN_ANGKA
    if nilai <= 0:
        return ALASAN_NON_POSITIF
    if nilai > HARGA_MAKS:
        return ALASAN_DI_LUAR_BATAS
    if satuan is not None and satuan_master is not None:
        if satuan.strip().lower() != satuan_master.strip().lower():
            return ALASAN_SATUAN
    if harga_acuan and not (harga_acuan * RASIO_MIN <= nilai <= harga_acuan * RASIO_MAKS):
        return ALASAN_SATUAN
    return None


def iklim_sah(indikator: str, nilai: float | None) -> bool:
    if nilai is None or not math.isfinite(nilai):
        return False
    if indikator == "curah_hujan":
        return 0 <= nilai <= 1000
    if indikator == "suhu":
        return -10 <= nilai <= 50
    if indikator == "kelembapan":
        return 0 <= nilai <= 100
    return False
