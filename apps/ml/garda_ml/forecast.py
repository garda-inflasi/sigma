"""Inti peramalan: fungsi murni tanpa jaringan, berkas, atau keadaan global.

Model yang tersedia sengaja sederhana dan dapat dijelaskan:

* ``naive``          : harga terakhir dipertahankan (baseline pembanding).
* ``seasonal_naive`` : mengulang pola satu pekan terakhir.
* ``holt``           : level + tren teredam (pemulusan eksponensial Holt).
* ``holt_winters``   : level + tren teredam + musiman mingguan aditif.

Pemilihan model dan metrik memakai backtest walk-forward. Sebagian lipatan
terakhir disisihkan sebagai lipatan uji yang tidak ikut memilih model, sehingga
MAPE/RMSE yang dilaporkan tidak terlalu optimistis. Bila riwayat terlalu pendek
untuk dipisah, metrik dilaporkan dengan penanda ``optimistis``.
"""

from __future__ import annotations

import math
from dataclasses import dataclass
from datetime import date, timedelta
from itertools import product

PERIODE_MUSIM = 7
MIN_LATIH = 28  # hari riwayat minimum sebelum titik asal backtest pertama
LANGKAH_LIPATAN = 7  # jarak hari antar titik asal backtest
MIN_LIPATAN = 2
MIN_COVERAGE = 0.7  # bagian hari yang harus punya data asli
Z_80 = 1.2816  # pengali simpangan untuk rentang keyakinan 80%

KANDIDAT = ("naive", "seasonal_naive", "holt", "holt_winters")
BASELINE = "naive"

LABEL_FAKTOR = {
    "level": ("Penyesuaian level harga", "Selisih level terpulus dengan harga terakhir"),
    "tren": ("Tren harga", "Kemiringan harga terkini yang diteruskan (teredam)"),
    "musiman": ("Pola musiman mingguan", "Pola harga per hari dalam sepekan"),
    "tetap": ("Harga terakhir dipertahankan", "Model naif: tidak ada perubahan struktural"),
}

_GRID_ALPHA = (0.2, 0.5, 0.8)
_GRID_BETA = (0.05, 0.2)
_GRID_GAMMA = (0.1, 0.3)
_GRID_PHI = (0.8, 0.95)


class DataTidakCukup(ValueError):
    """Riwayat terlalu pendek atau terlalu bolong untuk diproses."""


@dataclass(frozen=True)
class Jalur:
    """Hasil ramalan: ``nilai[k-1]`` untuk langkah k, beserta penguraiannya.

    Untuk setiap k, jumlah ``komponen[*][k-1]`` sama dengan
    ``nilai[k-1] - harga_terakhir``.
    """

    nilai: list[float]
    komponen: dict[str, list[float]]


# --------------------------------------------------------------------------- data


def regularkan(titik: list[dict]) -> tuple[date, list[float]]:
    """Mengubah titik (tanggal ISO, nilai) menjadi deret harian yang bersambung.

    Hari tanpa data diisi interpolasi linear. Menolak nilai tak positif, tanggal
    ganda, dan cakupan data asli di bawah ``MIN_COVERAGE``.
    """
    if not titik:
        raise DataTidakCukup("riwayat kosong")
    per_hari: dict[date, float] = {}
    for t in titik:
        try:
            hari = date.fromisoformat(str(t["tanggal"]))
            nilai = float(t["nilai"])
        except (KeyError, TypeError, ValueError) as err:
            raise ValueError(f"titik tidak valid: {t!r}") from err
        if not math.isfinite(nilai) or nilai <= 0:
            raise ValueError(f"nilai harus positif: {t!r}")
        if hari in per_hari:
            raise ValueError(f"tanggal ganda: {hari.isoformat()}")
        per_hari[hari] = nilai

    awal, akhir = min(per_hari), max(per_hari)
    panjang = (akhir - awal).days + 1
    if len(per_hari) / panjang < MIN_COVERAGE:
        raise DataTidakCukup(
            f"cakupan data {len(per_hari)}/{panjang} hari di bawah {int(MIN_COVERAGE * 100)}%"
        )

    deret: list[float] = []
    for i in range(panjang):
        hari = awal + timedelta(days=i)
        if hari in per_hari:
            deret.append(per_hari[hari])
            continue
        kanan = i + 1  # hari terakhir pasti punya data, jadi pencarian berhenti
        while awal + timedelta(days=kanan) not in per_hari:
            kanan += 1
        y_kiri, y_kanan = deret[i - 1], per_hari[awal + timedelta(days=kanan)]
        deret.append(y_kiri + (y_kanan - y_kiri) / (kanan - i + 1))
    return awal, deret


# ------------------------------------------------------------------------- model


def _hw_state(y: list[float], alpha: float, beta: float, gamma: float, phi: float, musiman: bool):
    """Menjalankan filter Holt(-Winters) aditif.

    Mengembalikan (sse_satu_langkah, level, tren, musim); ``musim[i % PERIODE_MUSIM]``
    berlaku untuk hari ke-i. Bentuk koreksi-galat: l += phi*b + alpha*e,
    b = phi*b + alpha*beta*e, s += gamma*(1-alpha)*e.
    """
    m = PERIODE_MUSIM
    if musiman:
        level = sum(y[:m]) / m
        tren = (sum(y[m : 2 * m]) / m - level) / m
        musim = [y[i] - level for i in range(m)]
        mulai = 0
    else:
        lebar = min(m, len(y))
        level, tren, musim = y[0], (y[lebar - 1] - y[0]) / max(lebar - 1, 1), [0.0] * m
        mulai = 1

    sse = 0.0
    for i in range(mulai, len(y)):
        s = musim[i % m] if musiman else 0.0
        galat = y[i] - (level + phi * tren + s)
        sse += galat * galat
        level = level + phi * tren + alpha * galat
        tren = phi * tren + alpha * beta * galat
        if musiman:
            musim[i % m] = s + gamma * (1 - alpha) * galat
    return sse, level, tren, musim


def sesuaikan(nama: str, y: list[float]) -> dict:
    """Mencari parameter model pada ``y`` (grid kecil, minimum SSE satu langkah)."""
    if nama in ("naive", "seasonal_naive"):
        return {}
    musiman = nama == "holt_winters"
    if musiman:
        kombinasi = product(_GRID_ALPHA, _GRID_BETA, _GRID_GAMMA, _GRID_PHI)
    else:
        kombinasi = ((a, b, 0.0, p) for a, b, p in product(_GRID_ALPHA, _GRID_BETA, _GRID_PHI))
    calon = (
        (_hw_state(y, a, b, g, p, musiman)[0], {"alpha": a, "beta": b, "gamma": g, "phi": p})
        for a, b, g, p in kombinasi
    )
    return min(calon, key=lambda c: c[0])[1]


def ramal(nama: str, y: list[float], params: dict, langkah: int) -> Jalur:
    """Meramal ``langkah`` hari ke depan dari akhir ``y``."""
    if nama not in KANDIDAT:
        raise ValueError(f"algoritma tidak dikenal: {nama}")
    terakhir = y[-1]
    ks = range(1, langkah + 1)

    if nama == "naive":
        return Jalur([terakhir] * langkah, {"tetap": [0.0] * langkah})
    if nama == "seasonal_naive":
        if len(y) < PERIODE_MUSIM:
            raise DataTidakCukup("seasonal naive butuh minimal satu pekan riwayat")
        nilai = [y[len(y) - PERIODE_MUSIM + (k - 1) % PERIODE_MUSIM] for k in ks]
        return Jalur(nilai, {"musiman": [v - terakhir for v in nilai]})

    musiman = nama == "holt_winters"
    if musiman and len(y) < 2 * PERIODE_MUSIM:
        raise DataTidakCukup("Holt-Winters butuh minimal dua pekan riwayat")
    phi = params["phi"]
    _, level, tren, musim = _hw_state(y, params["alpha"], params["beta"], params["gamma"], phi, musiman)
    n = len(y)
    komp_tren, komp_musim = [], []
    akumulasi = 0.0
    for k in ks:
        akumulasi += phi**k
        komp_tren.append(tren * akumulasi)
        komp_musim.append(musim[(n + k - 1) % PERIODE_MUSIM] if musiman else 0.0)
    komp_level = [level - terakhir] * langkah
    nilai = [terakhir + komp_level[i] + komp_tren[i] + komp_musim[i] for i in range(langkah)]
    komponen = {"level": komp_level, "tren": komp_tren}
    if musiman:
        komponen["musiman"] = komp_musim
    return Jalur(nilai, komponen)


# ---------------------------------------------------------------------- metrik


def mape(aktual: list[float], ramalan: list[float]) -> float:
    """Mean absolute percentage error, dalam persen."""
    if not aktual or len(aktual) != len(ramalan):
        raise ValueError("deret aktual dan ramalan harus sama panjang dan tidak kosong")
    return 100 * sum(abs(a - f) / a for a, f in zip(aktual, ramalan)) / len(aktual)


def rmse(aktual: list[float], ramalan: list[float]) -> float:
    if not aktual or len(aktual) != len(ramalan):
        raise ValueError("deret aktual dan ramalan harus sama panjang dan tidak kosong")
    return math.sqrt(sum((a - f) ** 2 for a, f in zip(aktual, ramalan)) / len(aktual))


def titik_asal(n: int, horizon_maks: int) -> list[int]:
    """Panjang-latih tiap lipatan walk-forward: latih ``y[:o]``, uji ``y[o:o+H]``."""
    return list(range(MIN_LATIH, n - horizon_maks + 1, LANGKAH_LIPATAN))


Lipatan = tuple[list[float], list[float]]  # (aktual, ramalan) sepanjang horizon_maks


def _backtest(nama: str, y: list[float], asal: list[int], horizon_maks: int) -> list[Lipatan]:
    """Melatih ulang di tiap titik asal lalu meramal horizon_maks hari ke depan."""
    hasil = []
    for o in asal:
        latih = y[:o]
        hasil.append((y[o : o + horizon_maks], ramal(nama, latih, sesuaikan(nama, latih), horizon_maks).nilai))
    return hasil


def _skor(lipatan: list[Lipatan], horizon: list[int]) -> tuple[float, float, dict[str, float]]:
    """MAPE, RMSE gabungan, dan MAPE per horizon pada langkah yang diminta."""
    aktual = [a[h - 1] for a, _ in lipatan for h in horizon]
    taksir = [f[h - 1] for _, f in lipatan for h in horizon]
    per_horizon = {
        str(h): mape([a[h - 1] for a, _ in lipatan], [f[h - 1] for _, f in lipatan]) for h in horizon
    }
    return mape(aktual, taksir), rmse(aktual, taksir), per_horizon


# ------------------------------------------------------------------- penjelasan


def _persen_bulat(bagian: list[float], total: float) -> list[float]:
    """Membulatkan ke satu desimal; sisa pembulatan dibebankan ke bagian terbesar agar jumlahnya = total."""
    bulat = [round(b, 1) for b in bagian]
    sisa = round(round(total, 1) - sum(bulat), 1)
    besar = max(range(len(bulat)), key=lambda i: abs(bagian[i]))
    bulat[besar] = round(bulat[besar] + sisa, 1)
    return bulat


def faktor(jalur: Jalur, k: int, harga_dasar: float) -> list[dict]:
    """Kontribusi tiap komponen terhadap perubahan harga pada langkah k, dalam poin persen.

    Jumlahnya sama dengan perubahan proyeksi terhadap harga dasar (dibulatkan satu desimal).
    """
    nama = list(jalur.komponen)
    bagian = [jalur.komponen[n][k - 1] / harga_dasar * 100 for n in nama]
    total = (jalur.nilai[k - 1] - harga_dasar) / harga_dasar * 100
    return [
        {"nama": LABEL_FAKTOR[n][0], "kontribusi": b, "detail": LABEL_FAKTOR[n][1]}
        for n, b in zip(nama, _persen_bulat(bagian, total))
    ]


# ------------------------------------------------------------------- pelatihan


def _bentuk_ramalan(
    nama: str, y: list[float], params: dict, galat_rmse: dict[str, float], horizon: list[int], tanggal_dasar: date
) -> list[dict]:
    jalur = ramal(nama, y, params, horizon[-1])
    hasil = []
    for h in horizon:
        mentah = jalur.nilai[h - 1]
        nilai = max(1, round(mentah))
        sd = galat_rmse[str(h)]
        hasil.append(
            {
                "horizon_hari": h,
                "tanggal_target": (tanggal_dasar + timedelta(days=h)).isoformat(),
                "nilai": nilai,
                "bawah": max(1, min(nilai, math.floor(mentah - Z_80 * sd))),
                "atas": max(nilai, math.ceil(mentah + Z_80 * sd)),
                "faktor": faktor(jalur, h, y[-1]),
            }
        )
    return hasil


def _galat_rmse(lipatan: list[Lipatan], horizon: list[int]) -> dict[str, float]:
    """Simpangan galat backtest pada tiap horizon; dasar rentang keyakinan."""
    return {
        str(h): math.sqrt(sum((a[h - 1] - f[h - 1]) ** 2 for a, f in lipatan) / len(lipatan)) for h in horizon
    }


def _angka(v: object) -> bool:
    return isinstance(v, (int, float)) and not isinstance(v, bool) and math.isfinite(v)


def _horizon_valid(horizon: list[int]) -> list[int]:
    urut = sorted(set(horizon))
    if not urut or urut[0] < 1:
        raise ValueError("horizon harus berisi bilangan bulat positif")
    return urut


def latih_seri(titik: list[dict], horizon: list[int]) -> dict:
    """Melatih satu seri harga: memilih model lewat backtest, lalu meramal dari seluruh riwayat.

    Mengangkat ``DataTidakCukup`` bila riwayat tak memadai.
    """
    horizon = _horizon_valid(horizon)
    awal, y = regularkan(titik)
    tanggal_dasar = awal + timedelta(days=len(y) - 1)

    asal = titik_asal(len(y), horizon[-1])
    if len(asal) < MIN_LIPATAN:
        raise DataTidakCukup(
            f"riwayat {len(y)} hari kurang untuk backtest horizon {horizon[-1]} hari "
            f"(butuh minimal {MIN_LATIH + LANGKAH_LIPATAN * (MIN_LIPATAN - 1) + horizon[-1]})"
        )
    n_uji = len(asal) // 3
    optimistis = n_uji == 0
    idx_seleksi = list(range(len(asal) - n_uji))
    idx_uji = idx_seleksi if optimistis else list(range(len(asal) - n_uji, len(asal)))

    kandidat = {nama: _backtest(nama, y, asal, horizon[-1]) for nama in KANDIDAT}
    skor_seleksi = {nama: _skor([lip[i] for i in idx_seleksi], horizon)[0] for nama, lip in kandidat.items()}
    terpilih = min(skor_seleksi, key=lambda n: (skor_seleksi[n], KANDIDAT.index(n)))

    mape_uji, rmse_uji, per_horizon = _skor([kandidat[terpilih][i] for i in idx_uji], horizon)
    mape_base, rmse_base, _ = _skor([kandidat[BASELINE][i] for i in idx_uji], horizon)

    params = sesuaikan(terpilih, y)
    galat = _galat_rmse(kandidat[terpilih], horizon)
    return {
        "algoritma": terpilih,
        "params": params,
        "galat_rmse": galat,
        "metrik": {
            "mape": mape_uji,
            "rmse": rmse_uji,
            "baseline_mape": mape_base,
            "baseline_rmse": rmse_base,
            "mape_per_horizon": per_horizon,
            "n_lipatan_seleksi": len(idx_seleksi),
            "n_lipatan_uji": len(idx_uji),
            "optimistis": optimistis,
            "mape_seleksi": skor_seleksi,
        },
        "harga_dasar": round(y[-1]),
        "tanggal_dasar": tanggal_dasar.isoformat(),
        "n_hari": len(y),
        "ramalan": _bentuk_ramalan(terpilih, y, params, galat, horizon, tanggal_dasar),
    }


def prediksi_seri(titik: list[dict], algoritma: str, params: dict, galat_rmse: dict[str, float], horizon: list[int]) -> dict:
    """Meramal dari riwayat terbaru dengan algoritma dan parameter yang sudah dilatih (tanpa melatih ulang)."""
    horizon = _horizon_valid(horizon)
    if algoritma not in KANDIDAT:
        raise ValueError(f"algoritma tidak dikenal: {algoritma}")
    hilang = [h for h in horizon if not _angka(galat_rmse.get(str(h)))]
    if hilang:
        raise ValueError(f"galat_rmse harus berisi angka untuk horizon {hilang}")
    if algoritma in ("holt", "holt_winters") and not all(_angka(params.get(k)) for k in ("alpha", "beta", "gamma", "phi")):
        raise ValueError("params harus berisi angka alpha, beta, gamma, phi")
    awal, y = regularkan(titik)
    tanggal_dasar = awal + timedelta(days=len(y) - 1)
    return {
        "algoritma": algoritma,
        "harga_dasar": round(y[-1]),
        "tanggal_dasar": tanggal_dasar.isoformat(),
        "ramalan": _bentuk_ramalan(algoritma, y, params, galat_rmse, horizon, tanggal_dasar),
    }


def ringkas(hasil: list[dict]) -> dict:
    """Merata-ratakan metrik seri yang berhasil dilatih menjadi satu ringkasan per model."""
    ok = [h for h in hasil if h.get("status") == "OK"]
    ringkasan: dict = {"n_seri_ok": len(ok), "n_seri_dilewati": len(hasil) - len(ok)}
    if not ok:
        return ringkasan
    for kunci in ("mape", "rmse", "baseline_mape", "baseline_rmse"):
        ringkasan[kunci] = sum(h["metrik"][kunci] for h in ok) / len(ok)
    ringkasan["optimistis"] = any(h["metrik"]["optimistis"] for h in ok)
    ringkasan["algoritma"] = {
        nama: sum(1 for h in ok if h["algoritma"] == nama) for nama in KANDIDAT if any(h["algoritma"] == nama for h in ok)
    }
    return ringkasan
