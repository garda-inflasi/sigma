"""Model 1: klasifikasi tekanan inflasi per wilayah.

**Definisi label dan horizon**

- ``RESMI_BPS``: baris fitur per akhir bulan ``T``. Label 1 (``INFLASI``) bila inflasi
  month-to-month resmi BPS pada bulan kalender *setelah* ``T`` lebih besar dari 0. Horizon
  sekitar satu bulan. Dipakai untuk wilayah yang punya minimal ``MIN_BULAN_RESMI`` label.
- ``INDEKS_PANGAN``: dipakai bila inflasi BPS tidak cukup. Baris fitur tiap 7 hari. Indeks
  adalah rantai rata-rata return log harian komoditas PIHPS berbobot sama. Label 1 bila
  rata-rata indeks 7 hari yang berakhir pada t+30 lebih tinggi dari indeks pada t
  (``MA7(indeks)[t+30] > indeks[t]``). Horizon 30 hari. Ini **bukan** inflasi resmi.
  Pembandingnya level pada t, bukan ``MA7[t]``: selisih ``indeks[t] - MA7[t]`` sudah diketahui
  pada t, sehingga label MA7-ke-MA7 membuat random walk pun tampak terprediksi (AUC rata-rata
  0,57 pada 16 random walk, 2 lolos kelayakan; dengan pembanding level 0,49 dan tidak ada).

**Validasi:** 20% tanggal terakhir menjadi uji out-of-time. Baris latih berjarak minimal
satu horizon (embargo) dari awal uji supaya label latih tidak tumpang tindih dengan uji.

**Kalibrasi:** ``CalibratedClassifierCV`` pada lipatan waktu ber-embargo.

**Kelayakan:** model hanya dipakai sebagai prediksi bila ROC-AUC holdout >= ``MIN_AUC_LAYAK`` dan
Brier holdout lebih rendah dari ``brier_baseline`` (tebakan konstan laju dasar data latih,
dinilai pada holdout yang sama). Bila tidak layak, ``kelas``, ``skor_tekanan``, dan ``siklus``
bernilai ``None``; ``probabilitas`` tetap dicatat untuk transparansi, dan tidak ada XAI maupun
sinyal EWS tekanan yang diterbitkan.

**Skor tekanan** (0-100): probabilitas dinormalkan terhadap laju dasar kelas positif, 50
berarti setara rata-rata historis. ``TEKANAN`` bila skor >= 60, ``SURPLUS`` bila skor <= 40
dan momentum indeks 30 hari tidak positif, selain itu ``NORMAL``.
"""

from __future__ import annotations

import logging
import re
import warnings
from dataclasses import dataclass, field
from typing import Any

import numpy as np
import pandas as pd
from sklearn.calibration import CalibratedClassifierCV
from sklearn.metrics import brier_score_loss, f1_score, roc_auc_score

from .backend import Backend
from .config import MAKS_UMUR_INFERENSI_HARI
from .ekstraksi import DataMentah
from .fitur import Panel

log = logging.getLogger("garda_ml.pipeline")

MIN_BULAN_RESMI = 24
MIN_AUC_LAYAK = 0.60
MIN_BARIS_INDEKS = 30
HORIZON_INDEKS_HARI = 30
EMBARGO_RESMI_HARI = 31
AMBANG_TEKANAN = 60.0
AMBANG_SURPLUS = 40.0
KOLOM_BOBOT = "_bobot_sampel"
_KOLOM_BUANG = ("komoditas_id", KOLOM_BOBOT)


@dataclass
class ModelKelas:
    target: str
    model: Any
    kolom: list[str]
    laju_dasar: float
    metode_kalibrasi: str
    metrik: dict


@dataclass
class HasilKlasifikasi:
    sinyal: list[dict] = field(default_factory=list)
    metrik: dict = field(default_factory=dict)
    model: dict[str, ModelKelas] = field(default_factory=dict)
    baris_inferensi: dict[str, tuple[str, np.ndarray]] = field(default_factory=dict)
    target_siklus: str = "INDEKS_PANGAN"
    catatan: list[str] = field(default_factory=list)


def skor_tekanan(p: float, laju_dasar: float) -> float:
    """Petakan probabilitas ke 0-100 dengan laju dasar sebagai titik tengah 50."""
    b = float(np.clip(laju_dasar, 0.01, 0.99))
    nilai = 50 + 50 * (p - b) / (1 - b) if p >= b else 50 * p / b
    return float(np.clip(nilai, 0, 100))


def nilai_kelayakan(metrik: dict) -> tuple[bool, str]:
    """``(layak, alasan)`` menurut aturan kelayakan kontrak."""
    auc, brier, acuan = metrik.get("roc_auc"), metrik.get("brier"), metrik.get("brier_baseline")
    if auc is None or brier is None or acuan is None:
        return False, "holdout tidak cukup untuk menilai model"
    alasan = []
    if auc < MIN_AUC_LAYAK:
        alasan.append(f"ROC-AUC holdout {auc:.2f} < {MIN_AUC_LAYAK:.2f}")
    if not brier < acuan:
        alasan.append(f"Brier {brier:.3f} tidak lebih rendah dari baseline laju dasar {acuan:.3f}")
    if alasan:
        return False, "; ".join(alasan)
    return True, f"ROC-AUC holdout {auc:.2f} >= {MIN_AUC_LAYAK:.2f} dan Brier {brier:.3f} < baseline {acuan:.3f}"


def tentukan_siklus(skor: float, momentum: float | None) -> str:
    if skor >= AMBANG_TEKANAN:
        return "TEKANAN"
    if skor <= AMBANG_SURPLUS and (momentum is None or np.isnan(momentum) or momentum <= 0):
        return "SURPLUS"
    return "NORMAL"


def panel_wilayah(panel: Panel) -> dict[str, pd.DataFrame]:
    """Agregasi panel ke wilayah x tanggal, termasuk provinsi dari kabupaten/kota anggotanya."""
    if panel.kosong:
        return {}
    numerik = [c for c in panel.kolom_fitur if c not in ("komoditas_id", "wilayah_id")]
    # Seri basi tidak ikut agregat: baris inferensinya hanya berisi nilai yang diteruskan.
    segar = panel.df["umur_data"] <= MAKS_UMUR_INFERENSI_HARI
    df = panel.df.loc[segar, ["kabupaten_kode", "tanggal", "bobot_h30", *numerik]]
    if df.empty:
        return {}
    hasil: dict[str, pd.DataFrame] = {}
    kode = sorted(df["kabupaten_kode"].unique())
    def agregasi(g: pd.DataFrame) -> pd.DataFrame:
        kelompok = g.groupby("tanggal", sort=True)
        rata = kelompok[numerik].mean()
        rata["luas_kenaikan_30"] = (g["lag_30"] > 0).groupby(g["tanggal"]).mean().astype("float32")
        rata[KOLOM_BOBOT] = kelompok["bobot_h30"].mean()
        idx = pd.date_range(rata.index.min(), rata.index.max(), freq="D")
        rata = rata.reindex(idx)
        rata["indeks_log"] = rata["lag_1"].fillna(0).cumsum().astype("float64")
        rata["indeks_ma7"] = rata["indeks_log"].rolling(7, min_periods=3).mean()
        rata["indeks_mom_30"] = rata["indeks_log"] - rata["indeks_log"].shift(30)
        return rata
    for w, g in df.groupby("kabupaten_kode", sort=True):
        hasil[str(w)] = agregasi(g)
    for prov in sorted({w[:2] for w in kode if len(w) > 2} - set(kode)):
        hasil[prov] = agregasi(df[df["kabupaten_kode"].str.startswith(prov)])
    return hasil


def seri_inflasi_resmi(makro: pd.DataFrame) -> dict[str, pd.DataFrame]:
    """Per wilayah: tabel bulanan ``mtm``, ``yoy``, ``ytd`` dari indikator inflasi BPS."""
    if makro.empty:
        return {}
    d = makro[(makro["domain"] == "INFLASI") & makro["wilayah_kode"].notna()].copy()
    if d.empty:
        return {}
    def jenis(nama: str) -> str | None:
        n = nama.lower()
        if re.search(r"mtm|m-to-m|month_to_month|bulanan", n):
            return "mtm"
        if re.search(r"yoy|y-on-y|year_on_year|tahunan", n):
            return "yoy"
        if re.search(r"ytd|y-to-d|year_to_date|kalender", n):
            return "ytd"
        if re.search(r"\bihk\b|indeks.harga.konsumen", n):
            return "ihk"
        return None
    d["jenis"] = d["indikator"].astype(str).map(jenis)
    d = d.dropna(subset=["jenis"])
    hasil: dict[str, pd.DataFrame] = {}
    for w, g in d.groupby("wilayah_kode"):
        g = g.assign(bulan=g["periode"].dt.to_period("M").dt.to_timestamp())
        tabel = g.pivot_table(index="bulan", columns="jenis", values="nilai", aggfunc="last").sort_index()
        if "mtm" not in tabel and "ihk" in tabel:
            selisih_bulan = tabel.index.to_series().dt.to_period("M").astype("int64").diff()
            tabel["mtm"] = (tabel["ihk"] / tabel["ihk"].shift(1) - 1).where(selisih_bulan == 1) * 100
        if "mtm" not in tabel:
            continue
        rilis = g[g["jenis"].isin(["mtm", "ihk"])].groupby("bulan")["tanggal_rilis"].max()
        tabel["tanggal_rilis"] = rilis.reindex(tabel.index)
        for kolom in ("yoy", "ytd"):
            if kolom not in tabel:
                tabel[kolom] = np.nan
        hasil[str(w)] = tabel
    return hasil


def lipatan_waktu(tanggal: np.ndarray, n_lipatan: int, embargo_hari: int, y: np.ndarray | None = None) -> list[tuple[np.ndarray, np.ndarray]]:
    """Lipatan maju: latih berakhir minimal ``embargo_hari`` sebelum awal uji."""
    unik = np.unique(tanggal)
    ukuran = max(1, len(unik) // (n_lipatan + 1))
    lipatan = []
    embargo = np.timedelta64(embargo_hari, "D")
    for i in range(n_lipatan):
        awal_pos = len(unik) - (n_lipatan - i) * ukuran
        if awal_pos <= 0:
            continue
        awal, akhir = unik[awal_pos], unik[min(awal_pos + ukuran - 1, len(unik) - 1)]
        latih = np.flatnonzero(tanggal < awal - embargo)
        uji = np.flatnonzero((tanggal >= awal) & (tanggal <= akhir))
        if len(latih) == 0 or len(uji) == 0:
            continue
        if y is not None and (len(np.unique(y[latih])) < 2 or len(np.unique(y[uji])) < 2):
            continue
        lipatan.append((latih, uji))
    return lipatan


def _latih_terkalibrasi(backend: Backend, X: np.ndarray, y: np.ndarray, w: np.ndarray, tanggal: np.ndarray, embargo: int) -> tuple[Any, str]:
    lipatan = lipatan_waktu(tanggal, 3, embargo, y)
    if len(lipatan) >= 2:
        metode = "isotonic" if len(y) >= 1000 else "sigmoid"
        with warnings.catch_warnings():
            warnings.simplefilter("ignore")
            model = CalibratedClassifierCV(backend.klasifikator(), method=metode, cv=lipatan, ensemble=True)
            model.fit(X, y, sample_weight=w)
        return model, metode
    with warnings.catch_warnings():
        warnings.simplefilter("ignore")
        model = backend.klasifikator().fit(X, y, sample_weight=w)
    return model, "tanpa_kalibrasi"


def _evaluasi_oot(backend: Backend, X: np.ndarray, y: np.ndarray, w: np.ndarray, tanggal: np.ndarray, embargo: int) -> dict:
    unik = np.unique(tanggal)
    if len(unik) < 10:
        return {"f1": None, "roc_auc": None, "brier": None, "brier_baseline": None, "n_uji": 0}
    batas = unik[int(len(unik) * 0.8)]
    latih = tanggal < batas - np.timedelta64(embargo, "D")
    uji = tanggal >= batas
    if latih.sum() < 10 or uji.sum() == 0 or len(np.unique(y[latih])) < 2:
        return {"f1": None, "roc_auc": None, "brier": None, "brier_baseline": None, "n_uji": int(uji.sum())}
    model, _ = _latih_terkalibrasi(backend, X[latih], y[latih], w[latih], tanggal[latih], embargo)
    p = model.predict_proba(X[uji])[:, 1]
    kelas = (p >= 0.5).astype(int)
    auc = float(roc_auc_score(y[uji], p)) if len(np.unique(y[uji])) == 2 else None
    laju_latih = float(y[latih].mean())  # yang diketahui saat memprediksi
    return {
        "f1": float(f1_score(y[uji], kelas, zero_division=0)), "roc_auc": auc,
        "brier": float(brier_score_loss(y[uji], p)),
        "brier_baseline": float(np.mean((y[uji] - laju_latih) ** 2)), "n_uji": int(uji.sum()),
    }


def _baris_indeks(wilayah: str, f: pd.DataFrame, kolom: list[str]) -> pd.DataFrame:
    akhir = f.index.max()
    pilih = ((akhir - f.index).days % 7) == 0
    depan = f["indeks_ma7"].shift(-HORIZON_INDEKS_HARI)
    label = (depan > f["indeks_log"]).astype(float).where(depan.notna() & f["indeks_log"].notna())
    keluaran = f.loc[pilih, kolom].copy()
    keluaran["label"] = label[pilih]
    keluaran[KOLOM_BOBOT] = f.loc[pilih, KOLOM_BOBOT].fillna(1.0)
    keluaran["wilayah_kode"] = wilayah
    keluaran["tanggal"] = keluaran.index
    return keluaran.reset_index(drop=True)


def _baris_resmi(wilayah: str, f: pd.DataFrame, resmi: pd.DataFrame, kolom: list[str], cutoff: pd.Timestamp) -> pd.DataFrame:
    baris = []
    for bulan, r in resmi.iterrows():
        rilis = r["tanggal_rilis"]
        if pd.isna(r["mtm"]) or pd.isna(rilis) or rilis > cutoff:
            continue
        t = bulan - pd.Timedelta(days=1)  # akhir bulan sebelum bulan label
        if t not in f.index:
            continue
        b = f.loc[t, kolom].to_dict()
        b.update({"label": float(r["mtm"] > 0), "wilayah_kode": wilayah, "tanggal": t, KOLOM_BOBOT: 1.0})
        baris.append(b)
    return pd.DataFrame(baris)


def _kolom_kelas(wilayah_df: dict[str, pd.DataFrame]) -> list[str]:
    contoh = next(iter(wilayah_df.values()))
    buang = {"indeks_log", "indeks_ma7", *_KOLOM_BUANG}
    return [c for c in contoh.columns if c not in buang]


def _latih_target(target: str, baris: pd.DataFrame, kolom: list[str], backend: Backend, embargo: int) -> ModelKelas | None:
    latih = baris[baris["label"].notna()].sort_values("tanggal")
    if len(latih) < 12 or latih["label"].nunique() < 2:
        return None
    X = latih[kolom].to_numpy(dtype=np.float32)
    y = latih["label"].to_numpy(dtype=int)
    w = latih[KOLOM_BOBOT].fillna(1.0).to_numpy(dtype=float)
    tanggal = latih["tanggal"].to_numpy("datetime64[ns]")
    metrik = _evaluasi_oot(backend, X, y, w, tanggal, embargo)
    metrik["layak"], metrik["alasan"] = nilai_kelayakan(metrik)
    model, metode = _latih_terkalibrasi(backend, X, y, w, tanggal, embargo)
    return ModelKelas(target, model, kolom, float(y.mean()), metode, metrik)


def _penyumbang(panel: Panel, wilayah: str, nama: dict[str, str], n: int = 5) -> list[dict]:
    """Andil komoditas pada perubahan indeks 30 hari (berbobot sama, poin persen)."""
    baris = panel.df[panel.df["tanggal"] == panel.cutoff]
    baris = baris[baris["kabupaten_kode"] == wilayah] if len(wilayah) > 2 else baris[baris["kabupaten_kode"].str.startswith(wilayah)]
    baris = baris[baris["umur_data"] <= MAKS_UMUR_INFERENSI_HARI].dropna(subset=["lag_30"])
    if baris.empty:
        return []
    per_komoditas = (np.expm1(baris["lag_30"].astype(float)) * 100).groupby(baris["komoditas_kode"]).mean()
    andil = per_komoditas / len(per_komoditas)
    urut = andil.reindex(andil.abs().sort_values(ascending=False).index).head(n)
    return [{"komoditas_kode": k, "nama": nama.get(k, k), "andil": round(float(v), 4)} for k, v in urut.items()]


def _aktual_indeks(f: pd.DataFrame) -> dict:
    """Perubahan indeks pangan (persen) pada tanggal terakhir, bukan inflasi resmi."""
    s = f["indeks_log"].dropna()
    if s.empty:
        return {"yoy": None, "mtm": None, "ytd": None, "periode": None}
    akhir = s.index[-1]
    def ubah(t: pd.Timestamp) -> float | None:
        if t < s.index[0]:
            return None
        return round(float(np.expm1(s.iloc[-1] - s.asof(t)) * 100), 4)
    return {
        "yoy": ubah(akhir - pd.DateOffset(years=1)), "mtm": ubah(akhir - pd.Timedelta(days=30)),
        "ytd": ubah(pd.Timestamp(akhir.year - 1, 12, 31)), "periode": akhir,
    }


def _aktual_resmi(resmi: pd.DataFrame, cutoff: pd.Timestamp) -> dict:
    terbit = resmi[resmi["tanggal_rilis"] <= cutoff].dropna(subset=["mtm"])
    if terbit.empty:
        return {"yoy": None, "mtm": None, "ytd": None, "periode": None}
    r = terbit.iloc[-1]
    return {"yoy": _opsi(r.get("yoy")), "mtm": _opsi(r.get("mtm")), "ytd": _opsi(r.get("ytd")), "periode": terbit.index[-1]}


def _opsi(v: Any) -> float | None:
    return None if v is None or pd.isna(v) else float(v)


def jalankan_klasifikasi(panel: Panel, data: DataMentah, backend: Backend, proyeksi_indeks: dict[str, float] | None = None) -> HasilKlasifikasi:
    """Latih model per target lalu hasilkan satu sinyal per wilayah."""
    hasil = HasilKlasifikasi()
    wilayah = panel_wilayah(panel)
    if not wilayah:
        hasil.metrik = {"model": None, "f1": None, "roc_auc": None, "brier": None, "n_uji": 0, "alasan": "panel harga kosong"}
        hasil.catatan.append("klasifikasi dilewati: tidak ada data harga")
        return hasil
    kolom = _kolom_kelas(wilayah)
    cutoff = panel.cutoff
    resmi = seri_inflasi_resmi(data.makro)
    wilayah_resmi = {}
    for w, t in resmi.items():
        if w in wilayah and int(t.loc[t["tanggal_rilis"] <= cutoff, "mtm"].notna().sum()) >= MIN_BULAN_RESMI:
            wilayah_resmi[w] = t

    bagian_resmi = [_baris_resmi(w, wilayah[w], t, kolom, cutoff) for w, t in wilayah_resmi.items()]
    bagian_resmi = [b for b in bagian_resmi if not b.empty]
    model_resmi = None
    if bagian_resmi:
        model_resmi = _latih_target("RESMI_BPS", pd.concat(bagian_resmi, ignore_index=True), kolom, backend, EMBARGO_RESMI_HARI)
        if model_resmi is None:
            hasil.catatan.append("inflasi BPS ada tetapi tidak cukup untuk melatih model")
    if model_resmi is None:
        wilayah_resmi = {}

    # Agregat provinsi dari sebagian kab/kota hanya dipakai bila punya label resmi provinsi.
    # Tanpa itu, sinyal hanya untuk wilayah yang benar-benar punya data harga.
    langsung = set(panel.df["kabupaten_kode"].unique())
    sisa = [w for w in wilayah if w not in wilayah_resmi and w in langsung]
    model_indeks = None
    if sisa:
        baris = pd.concat([_baris_indeks(w, wilayah[w], kolom) for w in sisa], ignore_index=True)
        if int(baris["label"].notna().sum()) >= MIN_BARIS_INDEKS:
            model_indeks = _latih_target("INDEKS_PANGAN", baris, kolom, backend, HORIZON_INDEKS_HARI)
        if model_indeks is None:
            hasil.catatan.append("indeks pangan belum cukup untuk melatih model klasifikasi")

    if model_resmi:
        hasil.model["RESMI_BPS"] = model_resmi
    if model_indeks:
        hasil.model["INDEKS_PANGAN"] = model_indeks

    for w, f in wilayah.items():
        if w not in wilayah_resmi and w not in langsung:
            continue
        target = "RESMI_BPS" if w in wilayah_resmi else "INDEKS_PANGAN"
        mk = hasil.model.get(target)
        if mk is None or cutoff not in f.index:
            continue
        x = f.loc[[cutoff], kolom].to_numpy(dtype=np.float32)
        p = float(mk.model.predict_proba(x)[0, 1])
        skor = skor_tekanan(p, mk.laju_dasar)
        momentum = f["indeks_mom_30"].iloc[-1]
        periode_target = (cutoff + pd.offsets.MonthBegin(1)) if target == "RESMI_BPS" else cutoff + pd.Timedelta(days=HORIZON_INDEKS_HARI)
        aktual = _aktual_resmi(wilayah_resmi[w], cutoff) if target == "RESMI_BPS" else _aktual_indeks(f)
        proyeksi = {"yoy": None, "mtm": None, "ytd": None}
        if target == "INDEKS_PANGAN" and proyeksi_indeks and w in proyeksi_indeks:
            proyeksi["mtm"] = round(float(proyeksi_indeks[w]), 4)
        layak = bool(mk.metrik.get("layak"))
        hasil.sinyal.append({
            "wilayah_kode": w, "periode": periode_target,
            "kelas": ("INFLASI" if p >= 0.5 else "TIDAK_INFLASI") if layak else None,
            "probabilitas": round(p, 6), "skor_tekanan": round(skor, 2) if layak else None,
            "siklus": tentukan_siklus(skor, momentum) if layak else None,
            "layak": layak, "alasan": mk.metrik.get("alasan"),
            "target": target, "proyeksi": proyeksi, "aktual": aktual,
            "penyumbang": _penyumbang(panel, w, data.nama_komoditas),
            "kalibrasi": {"metode": mk.metode_kalibrasi, "brier": mk.metrik.get("brier")},
        })
        hasil.baris_inferensi[w] = (target, x)

    target_dipakai = {s["target"] for s in hasil.sinyal}
    # Label siklus RESMI_BPS hanya bila semua sinyal benar-benar resmi.
    hasil.target_siklus = "RESMI_BPS" if target_dipakai == {"RESMI_BPS"} else "INDEKS_PANGAN"
    if target_dipakai == {"RESMI_BPS", "INDEKS_PANGAN"}:
        hasil.catatan.append("target campuran: sebagian wilayah memakai indeks pangan, lihat field target per dokumen")
    for target, mk in hasil.model.items():
        if not mk.metrik.get("layak"):
            hasil.catatan.append(f"klasifikasi {target} belum layak: {mk.metrik.get('alasan')}")
    utama = hasil.model.get(hasil.target_siklus) or next(iter(hasil.model.values()), None)
    if utama is None:
        hasil.metrik = {
            "model": None, "f1": None, "roc_auc": None, "brier": None, "brier_baseline": None, "n_uji": 0,
            "layak": False, "alasan": "; ".join(hasil.catatan) or "data tidak cukup",
        }
    else:
        hasil.metrik = {"model": backend.nama, **utama.metrik, "kalibrasi": utama.metode_kalibrasi}
        if backend.fallback:
            hasil.metrik["alasan_fallback"] = backend.alasan_fallback
    return hasil
