"""Model statistik per seri, seluruhnya di ruang log sehingga harga selalu positif.

Setiap fungsi menerima ``y`` (log harga harian tanpa NaN, berakhir di titik asal) dan
daftar ``langkah``, lalu mengembalikan array ``(len(langkah), 3)`` berisi P10, P50, P90.
"""

from __future__ import annotations

import warnings

import numpy as np

from .config import MIN_TITIK_HOLT_WINTERS, MIN_TITIK_SARIMAX

Z_P90 = 1.2815515655446004
MAKS_TITIK_STATISTIK = 365
MUSIM = 7
SKALA_SARIMAX = 100.0


def _pita(p50: np.ndarray, skala: np.ndarray) -> np.ndarray:
    skala = np.maximum(np.nan_to_num(skala, nan=0.0), 1e-4)
    return np.column_stack([p50 - Z_P90 * skala, p50, p50 + Z_P90 * skala])


def ramal_naive(y: np.ndarray, langkah: list[int]) -> np.ndarray:
    """Baseline persistensi: nilai terakhir, ketidakpastian tumbuh seperti random walk."""
    sigma = float(np.std(np.diff(y))) if len(y) > 2 else 0.0
    return _pita(np.full(len(langkah), float(y[-1])), sigma * np.sqrt(np.asarray(langkah, dtype=float)))


def ramal_seasonal_naive(y: np.ndarray, langkah: list[int]) -> np.ndarray:
    """Baseline: nilai pada hari yang sama pekan terakhir (nilai terakhir bila riwayat < 2 musim)."""
    n = len(y)
    m = MUSIM if n >= 2 * MUSIM else 1
    p50 = np.array([y[n - m + ((h - 1) % m)] for h in langkah])
    galat = y[m:] - y[:-m] if n > m else np.array([0.0])
    sigma = float(np.std(galat)) if len(galat) >= 2 else 0.02
    skala = sigma * np.sqrt(np.ceil(np.asarray(langkah, dtype=float) / m))
    return _pita(p50, skala)


def ramal_holt_winters(y: np.ndarray, langkah: list[int]) -> np.ndarray:
    from statsmodels.tsa.holtwinters import ExponentialSmoothing

    y = y[-MAKS_TITIK_STATISTIK:]
    musiman = "add" if len(y) >= 3 * MUSIM else None
    with warnings.catch_warnings():
        warnings.simplefilter("ignore")
        hasil = ExponentialSmoothing(
            y, trend="add", damped_trend=True, seasonal=musiman,
            seasonal_periods=MUSIM if musiman else None, initialization_method="estimated",
        ).fit(optimized=True)
    maks = max(langkah)
    prakiraan = np.asarray(hasil.forecast(maks))
    idx = np.asarray(langkah) - 1
    sigma = float(np.std(np.asarray(hasil.resid)))
    return _pita(prakiraan[idx], sigma * np.sqrt(np.asarray(langkah, dtype=float)))


def _sarimax_fit(z: np.ndarray):
    from statsmodels.tsa.statespace.sarimax import SARIMAX

    with warnings.catch_warnings():
        warnings.simplefilter("ignore")
        return SARIMAX(
            z, order=(1, 1, 1), seasonal_order=(0, 1, 1, MUSIM),
            enforce_stationarity=False, enforce_invertibility=False,
        ).fit(disp=False, maxiter=50)


def _sarimax_ramal(hasil, dasar: float, y_akhir: float, langkah: list[int]) -> np.ndarray:
    with warnings.catch_warnings():
        warnings.simplefilter("ignore")
        ramalan = hasil.get_forecast(max(langkah))
        tengah = np.asarray(ramalan.predicted_mean) / SKALA_SARIMAX + dasar
        selang = np.asarray(ramalan.conf_int(alpha=0.2)) / SKALA_SARIMAX + dasar
    idx = np.asarray(langkah) - 1
    keluaran = np.column_stack([selang[idx, 0], tengah[idx], selang[idx, 1]])
    if not np.isfinite(keluaran).all():
        raise ValueError("SARIMAX menghasilkan nilai tidak hingga")
    # Model tidak stabil bisa meledak di ruang log; tolak agar jatuh ke kandidat lain.
    if np.abs(keluaran - y_akhir).max() > np.log(5.0):
        raise ValueError("SARIMAX tidak stabil")
    return keluaran


def ramal_sarimax(y: np.ndarray, langkah: list[int]) -> np.ndarray:
    # Dilatih dalam persen relatif titik terakhir: varians return harian sekitar 1e-5
    # membuat optimizer tidak stabil bila memakai log harga mentah.
    y = y[-MAKS_TITIK_STATISTIK:]
    dasar = float(y[-1])
    return _sarimax_ramal(_sarimax_fit((y - dasar) * SKALA_SARIMAX), dasar, dasar, langkah)


def _sarimax_lipatan(y: np.ndarray, posisi: list[int], langkah: list[int]) -> list[np.ndarray]:
    """Parameter diestimasi di asal pertama; asal berikutnya hanya difilter ulang."""
    keluaran: list[np.ndarray] = []
    dasar_fit, dasar = None, 0.0
    for p in posisi:
        yy = y[max(0, p + 1 - MAKS_TITIK_STATISTIK) : p + 1]
        if dasar_fit is None:
            dasar = float(yy[-1])
            dasar_fit = hasil = _sarimax_fit((yy - dasar) * SKALA_SARIMAX)
        else:
            with warnings.catch_warnings():
                warnings.simplefilter("ignore")
                hasil = dasar_fit.apply((yy - dasar) * SKALA_SARIMAX)
        keluaran.append(_sarimax_ramal(hasil, dasar, float(yy[-1]), langkah))
    return keluaran


RAMALAN = {
    "naive": ramal_naive,
    "seasonal_naive": ramal_seasonal_naive,
    "holt_winters": ramal_holt_winters,
    "sarimax": ramal_sarimax,
}


def ramal_lipatan(nama: str, y: np.ndarray, posisi: list[int], langkah: list[int]) -> list[np.ndarray]:
    """Ramalan dari beberapa titik asal; ramalan di posisi p hanya melihat ``y[: p + 1]``."""
    if nama == "sarimax":
        return _sarimax_lipatan(y, posisi, langkah)
    return [RAMALAN[nama](y[: p + 1], langkah) for p in posisi]


def kandidat_statistik(n_asli: int) -> list[str]:
    """Kandidat model statistik menurut panjang riwayat asli."""
    if n_asli < MIN_TITIK_HOLT_WINTERS:
        return ["naive", "seasonal_naive"]
    if n_asli < MIN_TITIK_SARIMAX:
        return ["naive", "seasonal_naive", "holt_winters"]
    return ["naive", "seasonal_naive", "holt_winters", "sarimax"]


def rapikan_kuantil(p: np.ndarray) -> np.ndarray:
    """Hilangkan persilangan kuantil per baris."""
    return np.sort(p, axis=-1)
