"""Model 2: regresi multi-horizon H+7, H+14, H+30 dengan seleksi walk-forward.

**Kandidat per komoditas x wilayah:** model kuantil global (LightGBM, atau HistGradient
Boosting bila LightGBM tidak dapat dimuat), SARIMAX, Holt-Winters, serta dua baseline:
``naive`` (persistensi, nilai terakhir) dan ``seasonal_naive``.

**Walk-forward:** ``N_LIPATAN_SELEKSI`` lipatan seleksi ditambah ``N_LIPATAN_HOLDOUT`` lipatan
holdout murni, berjarak ``LANGKAH_LIPATAN_HARI`` (akhir data dikurangi 150 sampai 30 hari). Tiap
lipatan memuat ``N_ASAL_PER_LIPATAN`` titik asal mingguan, dan target suatu lipatan tidak melewati
awal lipatan berikutnya, sehingga skor antarlipatan tidak berbagi hari target. Skor lipatan
adalah rata-rata skor per horizon. Pada tiap titik asal, model hanya dilatih dari baris yang labelnya sudah diketahui saat itu
(``tanggal + h <= asal``). Semua kandidat dinilai pada pasangan (asal, horizon) yang sama
terhadap titik harga asli.

**Pemenang:** baseline terbaik ditentukan dari rata-rata MAPE lipatan seleksi. Kandidat
non-baseline hanya boleh menang bila MAPE-nya lebih rendah dari baseline itu di *setiap*
lipatan seleksi dan rata-ratanya lebih rendah minimal ``MARGIN_BASELINE``. Riwayat pendek boleh
memakai lebih sedikit lipatan, tetapi di bawah ``MIN_LIPATAN_SELEKSI`` hanya baseline yang
dipublikasikan.

**Metrik yang dilaporkan:** pemenang dan baseline terbaik diukur pada lipatan terakhir yang
tidak ikut memilih (``evaluasi: holdout_lipatan_terakhir``), sehingga ``metrik`` selalu
menggambarkan model yang dipublikasikan tanpa bias seleksi. ``mengalahkan_baseline`` bernilai
benar hanya bila model non-baseline tetap unggul sebesar margin pada holdout, dan ``None``
bila tidak ada holdout.

**Strategi bertingkat (24 siklus/hari):**

- ``PENUH`` (tiap ``interval_seleksi_jam``): retune hiperparameter dan walk-forward semua kandidat.
- ``FIT_ULANG`` (jam lainnya): pemenang sebelumnya dipakai ulang; model global dan model
  statistik pemenang dilatih ulang dengan data terbaru. Seri baru tetap diseleksi penuh.
"""

from __future__ import annotations

import logging
from collections import Counter
from dataclasses import dataclass, field
from typing import Any

import numpy as np
import pandas as pd
from joblib import Parallel, delayed

from .backend import GRID_PARAM, PARAM_BAWAAN, Backend
from .config import (
    HORIZON,
    JARAK_ASAL_HARI,
    KUANTIL,
    LANGKAH_LIPATAN_HARI,
    MAKS_UMUR_BARIS_HARI,
    MAKS_UMUR_INFERENSI_HARI,
    MAKS_UMUR_LABEL_HARI,
    MAKS_UMUR_RAMALAN_HARI,
    MARGIN_BASELINE,
    MIN_LIPATAN_SELEKSI,
    MIN_TITIK_GLOBAL,
    MIN_TITIK_RAMALAN,
    N_ASAL_PER_LIPATAN,
    N_LIPATAN_HOLDOUT,
    N_LIPATAN_SELEKSI,
    Konfigurasi,
)
from .fitur import Panel, SeriHarga
from .statistik import RAMALAN, kandidat_statistik, ramal_lipatan, rapikan_kuantil

log = logging.getLogger("garda_ml.pipeline")

BASELINES = ("naive", "seasonal_naive")
BASELINE_CADANGAN = "naive"  # dipakai bila belum ada lipatan yang bisa dinilai
MIN_BARIS_GLOBAL = 500


@dataclass
class HasilRamalan:
    seri: SeriHarga
    nama_model: str
    fallback: bool
    alasan: str
    metrik: dict
    kuantil: np.ndarray  # (len(HORIZON), 3) dalam satuan harga
    n_kandidat: int | None = None


@dataclass
class HasilRegresi:
    hasil: list[HasilRamalan] = field(default_factory=list)
    gagal: list[dict] = field(default_factory=list)
    dilewati: list[dict] = field(default_factory=list)
    tingkat: str = "PENUH"
    params: dict = field(default_factory=lambda: dict(PARAM_BAWAAN))
    model_global: dict[int, dict[float, Any]] = field(default_factory=dict)
    kolom_fitur: list[str] = field(default_factory=list)
    n_diseleksi: int = 0

    def pemenang(self) -> dict[str, int]:
        return dict(Counter(h.nama_model for h in self.hasil))


def skor(pasangan: list[tuple[float, float]]) -> dict[str, float]:
    """RMSE dan MAPE (persen) pada skala harga dari pasangan (prediksi log, aktual log)."""
    p = np.exp(np.array([x[0] for x in pasangan], dtype=float))
    a = np.exp(np.array([x[1] for x in pasangan], dtype=float))
    return {"rmse": float(np.sqrt(np.mean((p - a) ** 2))), "mape": float(np.mean(np.abs(p - a) / a) * 100)}


def skor_seimbang(nilai: dict, kunci: list) -> dict[str, float]:
    """Rata-rata skor per horizon; horizon pendek yang punya lebih banyak asal tidak mendominasi."""
    per_h: dict[int, list] = {}
    for k in kunci:
        per_h.setdefault(k[-1], []).append(nilai[k])
    s = [skor(v) for _, v in sorted(per_h.items())]
    return {"rmse": float(np.mean([x["rmse"] for x in s])), "mape": float(np.mean([x["mape"] for x in s]))}


def _rata_lipatan(skor_model: dict[Any, dict[str, float]], lipatan: list, kolom: str = "mape") -> float:
    return float(np.mean([skor_model[a][kolom] for a in lipatan]))


def baseline_terbaik(skor_lipatan: dict[str, dict[Any, dict[str, float]]]) -> str | None:
    """Baseline dengan rata-rata MAPE lipatan terkecil (seri memakai RMSE)."""
    ada = [b for b in BASELINES if skor_lipatan.get(b)]
    if not ada:
        return None
    lipatan = sorted(set.intersection(*(set(skor_lipatan[b]) for b in ada)))
    if not lipatan:
        return ada[0]
    return min(ada, key=lambda n: (_rata_lipatan(skor_lipatan[n], lipatan), _rata_lipatan(skor_lipatan[n], lipatan, "rmse")))


def _rasio_unggul(kandidat: dict, acuan: dict, margin: float, min_lipatan: int) -> float | None:
    """Rasio MAPE kandidat/baseline bila lolos aturan seleksi, selain itu ``None``."""
    lipatan = sorted(set(kandidat) & set(acuan))
    if len(lipatan) < min_lipatan or any(kandidat[a]["mape"] >= acuan[a]["mape"] for a in lipatan):
        return None
    rasio = _rata_lipatan(kandidat, lipatan) / _rata_lipatan(acuan, lipatan)
    return rasio if rasio <= 1 - margin else None


def pilih_pemenang(
    skor_lipatan: dict[str, dict[Any, dict[str, float]]],
    margin: float = MARGIN_BASELINE,
    min_lipatan: int = MIN_LIPATAN_SELEKSI,
) -> str:
    """Pilih dari skor ``{model: {lipatan: {mape, rmse}}}`` menurut aturan seleksi modul."""
    acuan = baseline_terbaik(skor_lipatan)
    if acuan is None:
        raise ValueError("seleksi membutuhkan minimal satu baseline")
    lolos: dict[str, float] = {}
    for nama, nilai in skor_lipatan.items():
        if nama not in BASELINES:
            rasio = _rasio_unggul(nilai, skor_lipatan[acuan], margin, min_lipatan)
            if rasio is not None:
                lolos[nama] = rasio
    if not lolos:
        return acuan
    return min(lolos, key=lambda n: (lolos[n], _rata_lipatan(skor_lipatan[n], sorted(skor_lipatan[n]), "rmse")))


def lipatan_asal(
    akhir: pd.Timestamp,
    n: int = N_LIPATAN_SELEKSI + N_LIPATAN_HOLDOUT,
    langkah: int = LANGKAH_LIPATAN_HARI,
    n_asal: int = N_ASAL_PER_LIPATAN,
    jarak: int = JARAK_ASAL_HARI,
) -> list[list[pd.Timestamp]]:
    """Titik asal per lipatan, dari yang tertua; lipatan terakhir dimulai ``langkah`` hari sebelum akhir."""
    awal = [akhir - pd.Timedelta(days=langkah * k) for k in range(n, 0, -1)]
    return [[a + pd.Timedelta(days=jarak * j) for j in range(n_asal)] for a in awal]


def _dengan_batas(lipatan: list[list[pd.Timestamp]]) -> list[tuple[int, list[pd.Timestamp], pd.Timestamp | None]]:
    """(indeks, asal, batas target): target lipatan tidak boleh melewati awal lipatan berikutnya."""
    return [(i, blok, lipatan[i + 1][0] if i + 1 < len(lipatan) else None) for i, blok in enumerate(lipatan)]


def _aktual(seri: SeriHarga, pos: int, h: int) -> float | None:
    """Log harga asli pada asal + h, atau ``None`` bila tidak ada titik asli yang dekat."""
    i = pos + h
    if i >= len(seri.log_harga):
        return None
    umur = seri.umur[i]
    if umur > MAKS_UMUR_LABEL_HARI or umur >= h:
        return None
    return float(seri.log_harga[i])


def _cv_statistik(seri: SeriHarga, lipatan: list[list[pd.Timestamp]]) -> tuple[tuple[str, str], dict[str, dict]]:
    """Walk-forward model statistik satu seri: {model: {(lipatan, asal, h): (prediksi, aktual)}}."""
    hasil: dict[str, dict] = {}
    gugur: set[str] = set()
    for i, blok, batas in _dengan_batas(lipatan):
        posisi = []
        for t in blok:
            if seri.tanggal[0] <= t <= seri.tanggal[-1]:
                pos = int((t - seri.tanggal[0]).days)
                if int(seri.asli[: pos + 1].sum()) >= MIN_TITIK_RAMALAN:
                    posisi.append((t, pos))
        if not posisi:
            continue
        for nama in kandidat_statistik(int(seri.asli[: posisi[0][1] + 1].sum())):
            if nama in gugur:
                continue
            try:
                ramalan = ramal_lipatan(nama, seri.log_harga, [p for _, p in posisi], list(HORIZON))
            except Exception:  # noqa: BLE001 - kandidat gagal gugur, seri tetap jalan
                gugur.add(nama)
                hasil.pop(nama, None)
                continue
            for (t, pos), pred in zip(posisi, ramalan):
                for j, h in enumerate(HORIZON):
                    if batas is not None and t + pd.Timedelta(days=h) > batas:
                        continue
                    aktual = _aktual(seri, pos, h)
                    if aktual is not None:
                        hasil.setdefault(nama, {})[(i, t, h)] = (float(pred[j, 1]), aktual)
    return (seri.komoditas_kode, seri.kabupaten_kode), hasil


def _indeks_latih(df: pd.DataFrame, h: int, batas: pd.Timestamp, maks: int, seed: int = 42) -> np.ndarray:
    """Baris yang labelnya sudah diketahui pada ``batas``; yang terbaru diutamakan."""
    tanggal = df["tanggal"].to_numpy()
    maska = df[f"target_h{h}"].notna().to_numpy() & (tanggal <= np.datetime64(batas - pd.Timedelta(days=h))) & (df["umur_data"].to_numpy() <= MAKS_UMUR_BARIS_HARI)
    idx = np.flatnonzero(maska)
    if len(idx) <= maks:
        return idx
    urut = idx[np.argsort(tanggal[idx], kind="stable")]
    separuh = maks // 2
    baru, lama = urut[-separuh:], urut[:-separuh]
    pilih = np.random.default_rng(seed).choice(lama, size=maks - separuh, replace=False)
    return np.sort(np.r_[pilih, baru])


def _latih_satu(df: pd.DataFrame, kolom: list[str], h: int, q: float, batas: pd.Timestamp, backend: Backend, params: dict, maks: int) -> Any | None:
    idx = _indeks_latih(df, h, batas, maks)
    if len(idx) < MIN_BARIS_GLOBAL:
        return None
    X = df[kolom].iloc[idx].to_numpy(dtype=np.float32)
    y = df[f"target_h{h}"].iloc[idx].to_numpy(dtype=float)
    w = df[f"bobot_h{h}"].iloc[idx].fillna(1.0).to_numpy(dtype=float)
    return backend.regresor_kuantil(q, params).fit(X, y, sample_weight=w)


def _prediksi_model(model: Any, X: np.ndarray) -> np.ndarray:
    return np.asarray(model.predict(X), dtype=float)


def _tala_hiperparameter(panel: Panel, asal: pd.Timestamp, backend: Backend, cfg: Konfigurasi) -> dict:
    """Pilih hiperparameter pada titik ``asal``, H+14, P50, menurut MAE log out-of-time."""
    df, kolom = panel.df, panel.kolom_fitur
    h = 14
    uji = df[(df["tanggal"] == asal) & df[f"target_h{h}"].notna()]
    if uji.empty:
        return dict(PARAM_BAWAAN)
    terbaik, nilai_terbaik = dict(PARAM_BAWAAN), np.inf
    X_uji = uji[kolom].to_numpy(dtype=np.float32)
    y_uji = uji[f"target_h{h}"].to_numpy(dtype=float)
    for params in GRID_PARAM:
        model = _latih_satu(df, kolom, h, 0.5, asal, backend, params, cfg.maks_baris_latih)
        if model is None:
            return dict(PARAM_BAWAAN)
        mae = float(np.mean(np.abs(_prediksi_model(model, X_uji) - y_uji)))
        if mae < nilai_terbaik:
            terbaik, nilai_terbaik = dict(params), mae
    return terbaik


def _cv_global(panel: Panel, lipatan: list[list[pd.Timestamp]], backend: Backend, params: dict, cfg: Konfigurasi) -> dict[tuple[str, str], dict]:
    """Walk-forward model global P50: {(k, w): {(lipatan, asal, h): (prediksi, aktual)}}.

    Satu model per lipatan x horizon, dilatih dari label yang sudah diketahui di awal lipatan,
    lalu dipakai untuk semua titik asal lipatan itu.
    """
    df, kolom = panel.df, panel.kolom_fitur
    hasil: dict[tuple[str, str], dict] = {}
    for i, blok, batas in _dengan_batas(lipatan):
        baris = df[df["tanggal"].isin(blok)]
        if baris.empty:
            continue
        X = baris[kolom].to_numpy(dtype=np.float32)
        for h in HORIZON:
            model = _latih_satu(df, kolom, h, 0.5, blok[0], backend, params, cfg.maks_baris_latih)
            if model is None:
                continue
            pred = _prediksi_model(model, X)
            for k, w, t, p in zip(baris["komoditas_kode"], baris["kabupaten_kode"], baris["tanggal"], pred):
                if batas is not None and t + pd.Timedelta(days=h) > batas:
                    continue
                seri = panel.seri.get((k, w))
                if seri is None:
                    continue
                pos = int((t - seri.tanggal[0]).days)
                if int(seri.asli[: pos + 1].sum()) < MIN_TITIK_GLOBAL:
                    continue
                aktual = _aktual(seri, pos, h)
                if aktual is not None:
                    hasil.setdefault((k, w), {})[(i, t, h)] = (float(seri.log_harga[pos]) + float(p), aktual)
    return hasil


def _nilai_seri(hasil_cv: dict[str, dict]) -> dict | None:
    """Pilih pemenang pada lipatan seleksi dan ukur pada lipatan holdout.

    Pasangan (asal, horizon) ditentukan oleh baseline. Kandidat dinilai pada lipatan seleksi
    yang pasangannya lengkap (riwayat pendek bisa punya lebih sedikit) dan wajib punya holdout.
    """
    acuan = [b for b in BASELINES if hasil_cv.get(b)]
    if not acuan:
        return None
    kunci = sorted(set.intersection(*(set(hasil_cv[b]) for b in acuan)), key=lambda x: (x[0], x[1]))
    if not kunci:
        return None
    per_asal: dict[Any, list] = {}
    for k in kunci:
        per_asal.setdefault(k[0], []).append(k)
    asal = sorted(per_asal)
    ada_holdout = len(asal) > N_LIPATAN_HOLDOUT
    lipatan_holdout = asal[-N_LIPATAN_HOLDOUT:] if ada_holdout else asal
    lipatan_seleksi = (asal[:-N_LIPATAN_HOLDOUT] if ada_holdout else asal)[-N_LIPATAN_SELEKSI:]
    kunci_holdout = [k for a in lipatan_holdout for k in per_asal[a]]

    peserta = {n: v for n, v in hasil_cv.items() if n in BASELINES or all(k in v for k in kunci_holdout)}
    skor_lipatan = {
        n: {a: skor_seimbang(v, per_asal[a]) for a in lipatan_seleksi if all(k in v for k in per_asal[a])}
        for n, v in peserta.items()
    }
    skor_lipatan = {n: s for n, s in skor_lipatan.items() if s}
    nama = pilih_pemenang(skor_lipatan)
    baseline = baseline_terbaik(skor_lipatan)
    uji_model = skor_seimbang(peserta[nama], kunci_holdout)
    uji_baseline = skor_seimbang(peserta[baseline], kunci_holdout)
    mengalahkan = None
    if ada_holdout:
        mengalahkan = nama not in BASELINES and uji_model["mape"] < uji_baseline["mape"] * (1 - MARGIN_BASELINE)
    return {
        "nama": nama, "n_kandidat": len(skor_lipatan),
        "metrik": {
            "rmse": uji_model["rmse"], "mape": uji_model["mape"],
            "baseline": baseline, "mape_baseline": uji_baseline["mape"],
            "mengalahkan_baseline": mengalahkan,
            "evaluasi": "holdout_lipatan_terakhir" if ada_holdout else "tanpa_holdout",
            "n_lipatan": len(asal), "n_lipatan_seleksi": len(lipatan_seleksi), "horizon": list(HORIZON),
        },
    }


def _ramal_statistik(seri: SeriHarga, nama: str) -> np.ndarray:
    """Ramal dari titik asli terakhir; horizon digeser sebesar umur data."""
    y = seri.log_harga[: seri.pos_terakhir + 1]
    langkah = [h + seri.umur_terakhir for h in HORIZON]
    return np.exp(rapikan_kuantil(RAMALAN[nama](y, langkah)))


def _final_statistik(seri: SeriHarga, nama: str) -> tuple[tuple[str, str], str, np.ndarray | None, str | None]:
    """Fit final; bila pemenang gagal, jatuh ke baseline dan catat alasannya."""
    kunci = (seri.komoditas_kode, seri.kabupaten_kode)
    try:
        return kunci, nama, _ramal_statistik(seri, nama), None
    except Exception as err:  # noqa: BLE001
        if nama == BASELINE_CADANGAN:
            return kunci, nama, None, f"baseline gagal: {err}"
        try:
            return kunci, BASELINE_CADANGAN, _ramal_statistik(seri, BASELINE_CADANGAN), f"{nama} gagal saat fit final: {str(err)[:100]}"
        except Exception as err2:  # noqa: BLE001
            return kunci, nama, None, f"baseline gagal: {err2}"


def _latih_global_final(panel: Panel, backend: Backend, params: dict, cfg: Konfigurasi) -> dict[int, dict[float, Any]]:
    keluaran: dict[int, dict[float, Any]] = {}
    for h in HORIZON:
        model_h: dict[float, Any] = {}
        for q in KUANTIL:
            m = _latih_satu(panel.df, panel.kolom_fitur, h, q, panel.cutoff, backend, params, cfg.maks_baris_latih)
            if m is None:
                return {}
            model_h[q] = m
        keluaran[h] = model_h
    return keluaran


def _prediksi_global(panel: Panel, model: dict[int, dict[float, Any]]) -> dict[tuple[str, str], np.ndarray]:
    baris = panel.df[panel.df["tanggal"] == panel.cutoff]
    if baris.empty or not model:
        return {}
    X = baris[panel.kolom_fitur].to_numpy(dtype=np.float32)
    mat = np.empty((len(baris), len(HORIZON), len(KUANTIL)))
    for i, h in enumerate(HORIZON):
        for j, q in enumerate(KUANTIL):
            mat[:, i, j] = _prediksi_model(model[h][q], X)
    mat = rapikan_kuantil(mat)
    hasil = {}
    for b, (k, w) in enumerate(zip(baris["komoditas_kode"], baris["kabupaten_kode"])):
        seri = panel.seri.get((k, w))
        if seri is None or seri.umur_terakhir > MAKS_UMUR_INFERENSI_HARI:
            continue
        hasil[(k, w)] = np.exp(float(seri.log_harga[-1]) + mat[b])
    return hasil


def _paralel(cfg: Konfigurasi, tugas: list) -> list:
    if cfg.n_jobs <= 1 or len(tugas) < 8:
        return [fn(*a) for fn, a, _ in tugas]
    return Parallel(n_jobs=cfg.n_jobs, backend="loky", batch_size="auto")(delayed(fn)(*a) for fn, a, _ in tugas)


def jalankan_regresi(
    panel: Panel,
    backend: Backend,
    cfg: Konfigurasi,
    juara: dict[tuple[str, str], dict] | None = None,
    seleksi_penuh: bool = True,
    params_lama: dict | None = None,
) -> HasilRegresi:
    """Seleksi (bila perlu), fit final, dan ramalan untuk semua seri panel."""
    juara = juara or {}
    keluaran = HasilRegresi(tingkat="PENUH" if seleksi_penuh else "FIT_ULANG", params=dict(params_lama or PARAM_BAWAAN))
    if panel.kosong:
        return keluaran
    keluaran.kolom_fitur = list(panel.kolom_fitur)

    layak: dict[tuple[str, str], SeriHarga] = {}
    for kunci, seri in panel.seri.items():
        if seri.n_asli < MIN_TITIK_RAMALAN:
            keluaran.dilewati.append({"komoditas_kode": kunci[0], "kabupaten_kode": kunci[1], "alasan": f"riwayat {seri.n_asli} titik (< {MIN_TITIK_RAMALAN})"})
        elif seri.umur_terakhir > MAKS_UMUR_RAMALAN_HARI:
            keluaran.dilewati.append({"komoditas_kode": kunci[0], "kabupaten_kode": kunci[1], "alasan": f"data asli terakhir {seri.umur_terakhir} hari lalu (> {MAKS_UMUR_RAMALAN_HARI})"})
        else:
            layak[kunci] = seri
    if not layak:
        return keluaran

    perlu = list(layak) if seleksi_penuh else [k for k in layak if k not in juara]
    keluaran.n_diseleksi = len(perlu)
    akhir = max(s.tanggal_terakhir for s in layak.values())
    lipatan = lipatan_asal(akhir)
    hasil_cv: dict[tuple[str, str], dict[str, dict]] = {}
    if perlu:
        tugas = [(_cv_statistik, (layak[k], lipatan), k) for k in perlu]
        for kunci, h_stat in _paralel(cfg, tugas):
            hasil_cv[kunci] = h_stat
        if seleksi_penuh:
            # Ditala sebelum lipatan pertama agar skor walk-forward tidak bias oleh penalaan.
            keluaran.params = _tala_hiperparameter(panel, lipatan[0][0] - pd.Timedelta(days=LANGKAH_LIPATAN_HARI), backend, cfg)
        for kunci, nilai in _cv_global(panel, lipatan, backend, keluaran.params, cfg).items():
            if kunci in hasil_cv:
                hasil_cv[kunci][backend.nama_kuantil] = nilai

    pemilihan: dict[tuple[str, str], dict] = {}
    for kunci, seri in layak.items():
        if kunci in hasil_cv:
            pemilihan[kunci] = _nilai_seri(hasil_cv[kunci]) or {
                "nama": BASELINE_CADANGAN, "n_kandidat": 1,
                "metrik": {
                    "rmse": None, "mape": None, "baseline": None, "mape_baseline": None, "mengalahkan_baseline": None,
                    "evaluasi": "tanpa_evaluasi", "n_lipatan": 0, "n_lipatan_seleksi": 0, "horizon": list(HORIZON),
                },
            }
        else:
            lama = juara[kunci]
            pemilihan[kunci] = {"nama": lama["nama"], "n_kandidat": lama.get("n_kandidat"), "metrik": lama["metrik"]}

    butuh_global = any(p["nama"] == backend.nama_kuantil for p in pemilihan.values())
    # Model global juga dipakai SHAP, jadi tetap dilatih bila seri cukup panjang.
    if butuh_global or any(s.n_asli >= MIN_TITIK_GLOBAL for s in layak.values()):
        keluaran.model_global = _latih_global_final(panel, backend, keluaran.params, cfg)
    prediksi_global = _prediksi_global(panel, keluaran.model_global)

    tugas_stat = []
    for kunci, pilih in pemilihan.items():
        nama = pilih["nama"]
        if nama == backend.nama_kuantil and kunci in prediksi_global:
            continue
        if nama not in RAMALAN:  # model global tak tersedia atau backend berganti
            nama = pilih["metrik"].get("baseline") or BASELINE_CADANGAN
        tugas_stat.append((_final_statistik, (layak[kunci], nama), kunci))
    final_stat = {r[0]: r for r in _paralel(cfg, tugas_stat)} if tugas_stat else {}

    for kunci, pilih in pemilihan.items():
        seri = layak[kunci]
        catatan: list[str] = []
        metrik = pilih["metrik"]
        if seri.n_asli < MIN_TITIK_GLOBAL:
            catatan.append(f"riwayat {seri.n_asli} titik: kandidat dibatasi")
        fallback = seri.n_asli < MIN_TITIK_GLOBAL
        if pilih["nama"] == backend.nama_kuantil and kunci in prediksi_global:
            nama, kuantil = pilih["nama"], prediksi_global[kunci]
            if backend.fallback:
                fallback = True
                catatan.append(backend.alasan_fallback or "")
        else:
            _, nama, kuantil, galat = final_stat[kunci]
            if kuantil is None:
                keluaran.gagal.append({"komoditas_kode": kunci[0], "kabupaten_kode": kunci[1], "alasan": galat})
                continue
            if galat:
                fallback = True
                catatan.append(galat)
            if pilih["nama"] != nama:
                fallback = True
                if not galat:
                    catatan.append(f"{pilih['nama']} tidak tersedia pada siklus ini")
                # Skor pemenang asli tidak berlaku untuk model pengganti.
                metrik = {
                    **metrik, "rmse": None, "mape": metrik.get("mape_baseline") if nama == metrik.get("baseline") else None,
                    "mengalahkan_baseline": False, "evaluasi": "model_pengganti",
                }
        if nama in BASELINES and (metrik.get("n_lipatan_seleksi") or 0) < MIN_LIPATAN_SELEKSI:
            catatan.append(f"lipatan seleksi kurang dari {MIN_LIPATAN_SELEKSI}: baseline dipakai")
        if nama in BASELINES and not catatan:
            catatan.append("tidak ada kandidat yang mengalahkan baseline" if (pilih.get("n_kandidat") or 1) > len(BASELINES) else "baseline")
        if seri.umur_terakhir > 0:
            catatan.append(f"data asli terakhir {seri.umur_terakhir} hari lalu")
        if not np.isfinite(kuantil).all() or (kuantil <= 0).any():
            keluaran.gagal.append({"komoditas_kode": kunci[0], "kabupaten_kode": kunci[1], "alasan": "ramalan tidak valid"})
            continue
        keluaran.hasil.append(HasilRamalan(seri, nama, fallback, "; ".join(c for c in catatan if c), metrik, kuantil, pilih.get("n_kandidat")))
    return keluaran
