"""Kalender hari besar Indonesia sebagai fitur (diketahui sebelumnya, tanpa kebocoran).

Tanggal hari besar Islam memakai tabel eksplisit penetapan Indonesia (SKB/sidang isbat)
untuk 2018-2026. Tahun 2027-2030 adalah perkiraan kalender dan bisa bergeser 1 hari saat
isbat. Di luar tabel dipakai kalender Hijriah tabular (akurasi sekitar 1-2 hari).
"""

from __future__ import annotations

import math
from datetime import date

import numpy as np
import pandas as pd

IDULFITRI = tuple(date.fromisoformat(t) for t in (
    "2018-06-15", "2019-06-05", "2020-05-24", "2021-05-13", "2022-05-02", "2023-04-22", "2024-04-10",
    "2025-03-31", "2026-03-21", "2027-03-10", "2028-02-27", "2029-02-15", "2030-02-05",
))
IDULADHA = tuple(date.fromisoformat(t) for t in (
    "2018-08-22", "2019-08-11", "2020-07-31", "2021-07-20", "2022-07-10", "2023-06-29", "2024-06-17",
    "2025-06-06", "2026-05-27", "2027-05-17", "2028-05-05", "2029-04-24", "2030-04-13",
))
AWAL_RAMADAN = tuple(date.fromisoformat(t) for t in (
    "2018-05-17", "2019-05-06", "2020-04-24", "2021-04-13", "2022-04-03", "2023-03-23", "2024-03-12",
    "2025-03-01", "2026-02-19", "2027-02-08", "2028-01-28", "2029-01-16", "2030-01-06", "2030-12-26",
))

_EPOCH_HIJRIAH = 227015  # Rata Die 1 Muharam 1 H (epoch sipil)
_TAHUN_H = range(1430, 1491)  # sekitar 2009-2068
BATAS_PERKIRAAN = 3  # hari: entri tabular sedekat ini diganti nilai tabel


def hijriah_ke_masehi(tahun: int, bulan: int, hari: int) -> date:
    """Konversi kalender Hijriah tabular (sipil) ke Masehi."""
    rd = hari + math.ceil(29.5 * (bulan - 1)) + (tahun - 1) * 354 + (3 + 11 * tahun) // 30 + _EPOCH_HIJRIAH - 1
    return date.fromordinal(rd)


def _peristiwa(bulan: int, hari: int, tabel: tuple[date, ...]) -> np.ndarray:
    hasil = []
    for th in _TAHUN_H:
        tgl = hijriah_ke_masehi(th, bulan, hari)
        dekat = [t for t in tabel if abs((t - tgl).days) <= BATAS_PERKIRAAN]
        hasil.append(dekat[0] if dekat else tgl)
    return np.array(sorted(set(hasil)), dtype="datetime64[D]")


_IDULFITRI = _peristiwa(10, 1, IDULFITRI)
_IDULADHA = _peristiwa(12, 10, IDULADHA)
_RAMADAN = _peristiwa(9, 1, AWAL_RAMADAN)


def _hari_ke_berikut(tanggal: np.ndarray, peristiwa: np.ndarray) -> np.ndarray:
    pos = np.searchsorted(peristiwa, tanggal, side="left")
    pos = np.clip(pos, 0, len(peristiwa) - 1)
    return (peristiwa[pos] - tanggal).astype("timedelta64[D]").astype(float)


def _hari_sejak(tanggal: np.ndarray, peristiwa: np.ndarray) -> np.ndarray:
    pos = np.searchsorted(peristiwa, tanggal, side="right") - 1
    pos = np.clip(pos, 0, len(peristiwa) - 1)
    return (tanggal - peristiwa[pos]).astype("timedelta64[D]").astype(float)


def _tahunan(tanggal: pd.DatetimeIndex, bulan: int, hari: int) -> np.ndarray:
    tahun = range(tanggal.year.min() - 1, tanggal.year.max() + 2)
    return np.array([date(t, bulan, hari) for t in tahun], dtype="datetime64[D]")


def fitur_kalender(tanggal: pd.DatetimeIndex) -> pd.DataFrame:
    """Fitur kalender per tanggal; hanya bergantung pada tanggal itu sendiri."""
    hari = tanggal.values.astype("datetime64[D]")
    ramadan_sejak = _hari_sejak(hari, _RAMADAN)
    ke_fitri = _hari_ke_berikut(hari, _IDULFITRI)
    kolom = {
        "kal_hari_ke_idulfitri": np.clip(ke_fitri, 0, 90),
        "kal_hari_sejak_idulfitri": np.clip(_hari_sejak(hari, _IDULFITRI), 0, 45),
        "kal_hari_ke_iduladha": np.clip(_hari_ke_berikut(hari, _IDULADHA), 0, 60),
        "kal_is_ramadan": ((ramadan_sejak >= 0) & (ramadan_sejak < 31) & (ke_fitri <= 30) & (ke_fitri > 0)).astype(float),
        "kal_hari_ke_natal": np.clip(_hari_ke_berikut(hari, _tahunan(tanggal, 12, 25)), 0, 45),
        "kal_hari_ke_tahun_baru": np.clip(_hari_ke_berikut(hari, _tahunan(tanggal, 1, 1)), 0, 45),
        "kal_hari_minggu": tanggal.dayofweek.to_numpy(dtype=float),
        "kal_bulan": tanggal.month.to_numpy(dtype=float),
    }
    return pd.DataFrame(kolom, index=tanggal, dtype="float32")


def tanggal_idulfitri(tahun: int) -> date:
    """Idulfitri pertama pada tahun Masehi tersebut."""
    awal = np.datetime64(date(tahun, 1, 1))
    return _IDULFITRI[np.searchsorted(_IDULFITRI, awal)].astype(date)

