"""Rekayasa fitur tanpa kebocoran untuk panel harian komoditas x wilayah.

Setiap fitur pada tanggal ``t`` hanya memakai nilai pada ``t`` atau sebelumnya. Harga
diteruskan (ffill) hanya untuk masa lalu pada grid harian; umur titik asli ikut menjadi
fitur. Makro digabung as-of ``tanggal_rilis``. Kolom ``target_h*`` adalah satu-satunya
kolom yang melihat masa depan dan tidak pernah masuk ke ``kolom_fitur``.

Semua fitur harga berbentuk rasio log terhadap harga saat ini supaya satu model global
dapat dilatih lintas komoditas dengan level harga berbeda:

- ``lag_k`` = log(y_t / y_{t-k}) untuk k = 1, 3, 7, 14, 30
- ``ma_7``, ``ma_30`` = log y_t - rata-rata log y pada jendela
- ``vol_7``, ``vol_30`` = simpangan baku return log harian
- ``umur_data`` = hari sejak titik asli terakhir (memperhitungkan ``last_available_date``)
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field

import numpy as np
import pandas as pd

from .config import HORIZON, JEDA_CUACA_HARI, LAG_HARI, MAKS_UMUR_INFERENSI_HARI, MAKS_UMUR_LABEL_HARI
from .ekstraksi import DataMentah
from .kalender import fitur_kalender

MAKS_INDIKATOR_MAKRO = 20

_CUACA = {
    "hujan": re.compile(r"hujan|precip|rain"),
    "suhu": re.compile(r"suhu|temp"),
    "lembap": re.compile(r"lembab|lembap|humid"),
    "angin": re.compile(r"angin|wind"),
}
_RE_UPAH = re.compile(r"upah|ump|umk|gaji|biaya.?tenaga")
_RE_PASOKAN = re.compile(r"produksi|panen")
_RE_INFLASI_FITUR = "yoy|mtm|year_on_year|month_to_month"


@dataclass
class SeriHarga:
    """Seri harian satu komoditas x wilayah pada grid harian hingga cutoff."""

    komoditas_kode: str
    kabupaten_kode: str
    tanggal: pd.DatetimeIndex
    log_harga: np.ndarray  # ffill dari titik asli
    asli: np.ndarray  # True pada titik asli
    bobot: np.ndarray  # bobot titik asli (ffill)
    umur: np.ndarray  # hari sejak titik asli terakhir
    satuan: str = "Rp/kg"
    sumber: str = ""

    @property
    def n_asli(self) -> int:
        return int(self.asli.sum())

    @property
    def pos_terakhir(self) -> int:
        return int(np.flatnonzero(self.asli)[-1])

    @property
    def umur_terakhir(self) -> int:
        return int(len(self.asli) - 1 - self.pos_terakhir)

    @property
    def harga_terakhir(self) -> float:
        return float(np.exp(self.log_harga[self.pos_terakhir]))

    @property
    def tanggal_terakhir(self) -> pd.Timestamp:
        return self.tanggal[self.pos_terakhir]


@dataclass
class Panel:
    df: pd.DataFrame
    kolom_fitur: list[str]
    seri: dict[tuple[str, str], SeriHarga] = field(default_factory=dict)
    cutoff: pd.Timestamp | None = None

    @property
    def kosong(self) -> bool:
        return self.df.empty


def _slug(teks: str) -> str:
    return re.sub(r"[^a-z0-9]+", "_", str(teks).lower()).strip("_")[:40]


def _fitur_cuaca(cuaca: pd.DataFrame, indeks: pd.DatetimeIndex) -> dict[str, pd.DataFrame]:
    """Rata-rata bergulir 7 dan 30 hari per kabupaten, memakai data sampai t - jeda cuaca.

    Boleh kosong total: tanpa riwayat iklim tidak ada kolom cuaca, bukan kolom berisi pengganti.
    """
    if cuaca.empty:
        return {}
    d = cuaca.copy()
    nama = d["indikator"].astype(str).str.lower()
    d["jenis"] = None
    for jenis, pola in _CUACA.items():
        d.loc[nama.str.contains(pola) & d["jenis"].isna(), "jenis"] = jenis
    d = d.dropna(subset=["jenis"])
    hasil: dict[str, pd.DataFrame] = {}
    for wilayah, g in d.groupby("kabupaten_kode"):
        tabel = g.pivot_table(index="tanggal", columns="jenis", values="nilai", aggfunc="mean").reindex(indeks)
        kolom = {}
        for jenis in _CUACA:
            if jenis not in tabel:
                continue
            s = tabel[jenis].shift(JEDA_CUACA_HARI)
            kolom[f"cuaca_{jenis}_7"] = s.rolling(7, min_periods=2).mean()
            kolom[f"cuaca_{jenis}_30"] = s.rolling(30, min_periods=7).mean()
        hasil[str(wilayah)] = pd.DataFrame(kolom, index=indeks, dtype="float32")
    return hasil


def _pilih_indikator(makro: pd.DataFrame) -> list[tuple[str, str]]:
    if makro.empty:
        return []
    jumlah = makro.groupby(["domain", "indikator"]).size().reset_index(name="n")
    jumlah = jumlah[(jumlah["domain"] != "INFLASI") | jumlah["indikator"].str.contains(_RE_INFLASI_FITUR, case=False)]
    jumlah = jumlah.sort_values(["n", "domain", "indikator"], ascending=[False, True, True]).head(MAKS_INDIKATOR_MAKRO)
    return [(d, i) for d, i in zip(jumlah["domain"], jumlah["indikator"])]


def asof_rilis(tanggal: pd.DatetimeIndex, g: pd.DataFrame) -> tuple[np.ndarray, np.ndarray]:
    """Nilai terakhir yang sudah rilis pada tiap tanggal, dan perubahannya (persen).

    Titik yang dirilis terlambat tidak boleh menggantikan periode yang lebih baru.
    """
    d = g.dropna(subset=["tanggal_rilis", "nilai"]).sort_values(["tanggal_rilis", "periode"])
    if d.empty:
        kosong = np.full(len(tanggal), np.nan, dtype="float32")
        return kosong, kosong.copy()
    periode = d["periode"].to_numpy()
    baru = np.r_[True, periode[1:] > np.maximum.accumulate(periode)[:-1]]
    d = d.loc[baru]
    rilis = d["tanggal_rilis"].to_numpy("datetime64[ns]")
    nilai = d["nilai"].to_numpy(float)
    with np.errstate(divide="ignore", invalid="ignore"):
        ubah = np.r_[np.nan, (nilai[1:] / nilai[:-1] - 1) * 100]
    ubah[~np.isfinite(ubah)] = np.nan
    pos = np.searchsorted(rilis, tanggal.values, side="right") - 1
    sudah = pos >= 0
    n, c = np.full(len(tanggal), np.nan), np.full(len(tanggal), np.nan)
    n[sudah], c[sudah] = nilai[pos[sudah]], ubah[pos[sudah]]
    return n.astype("float32"), c.astype("float32")


def _fitur_makro(makro: pd.DataFrame, wilayah: list[str], indeks: pd.DatetimeIndex) -> dict[str, pd.DataFrame]:
    """Fitur makro as-of rilis per wilayah, dengan urutan fallback kab > provinsi > nasional."""
    pilihan = _pilih_indikator(makro)
    if not pilihan:
        return {}
    kelompok = {k: g for k, g in makro.groupby(["domain", "indikator", makro["wilayah_kode"].fillna("")], sort=False)}
    hasil: dict[str, pd.DataFrame] = {}
    cache: dict[tuple, tuple[np.ndarray, np.ndarray]] = {}
    for w in wilayah:
        kolom: dict[str, np.ndarray] = {}
        pasokan, neraca, upah = [], [], []
        for domain, indikator in pilihan:
            for kandidat in (w, w[:2], ""):
                kunci = (domain, indikator, kandidat)
                if kunci in kelompok:
                    if kunci not in cache:
                        cache[kunci] = asof_rilis(indeks, kelompok[kunci])
                    nilai, ubah = cache[kunci]
                    nama = f"makro_{domain}_{_slug(indikator)}"
                    kolom[nama], kolom[f"{nama}_chg"] = nilai, ubah
                    if domain in ("PERTANIAN", "PERIKANAN", "KEHUTANAN") and _RE_PASOKAN.search(indikator.lower()):
                        pasokan.append(ubah)
                    elif domain == "NERACA_EKONOMI":
                        neraca.append(ubah)
                    elif domain == "TENAGA_KERJA" and _RE_UPAH.search(indikator.lower()):
                        upah.append(ubah)
                    break
        for nama, daftar, tanda in (("tekanan_pasokan", pasokan, -1.0), ("tekanan_neraca", neraca, 1.0), ("biaya_tenaga_kerja", upah, 1.0)):
            if daftar:
                tumpuk = np.vstack(daftar)
                ada = np.isfinite(tumpuk).sum(axis=0)
                jumlah = np.nansum(tumpuk, axis=0)
                with np.errstate(invalid="ignore", divide="ignore"):
                    kolom[nama] = (tanda * jumlah / np.where(ada > 0, ada, np.nan)).astype("float32")
        hasil[w] = pd.DataFrame(kolom, index=indeks)
    return hasil


def _seri_dari_grup(k: str, w: str, g: pd.DataFrame, cutoff: pd.Timestamp, lad: pd.Timestamp | None) -> tuple[SeriHarga, pd.DataFrame]:
    idx = pd.date_range(g["tanggal"].min(), cutoff, freq="D")
    asli_df = g.set_index("tanggal")
    nilai = asli_df["nilai"].reindex(idx)
    maska = nilai.notna().to_numpy()
    log = np.log(nilai.astype(float)).ffill()
    posisi = np.arange(len(idx))
    terakhir = np.maximum.accumulate(np.where(maska, posisi, -1))
    umur = (posisi - terakhir).astype(float)
    if lad is not None and not pd.isna(lad):
        # Sumber menyatakan data asli terakhir; umur tidak boleh melewati pernyataan itu.
        hari_lad = (idx - lad.normalize()).days.to_numpy(dtype=float)
        umur = np.where(hari_lad >= 0, np.minimum(umur, hari_lad), umur)
    bobot = asli_df["bobot"].reindex(idx).ffill().to_numpy(float)

    kolom: dict[str, np.ndarray | pd.Series] = {}
    for k_lag in LAG_HARI:
        kolom[f"lag_{k_lag}"] = log - log.shift(k_lag)
    kolom["ma_7"] = log - log.rolling(7, min_periods=3).mean()
    kolom["ma_30"] = log - log.rolling(30, min_periods=7).mean()
    ret = log.diff()
    kolom["vol_7"] = ret.rolling(7, min_periods=3).std()
    kolom["vol_30"] = ret.rolling(30, min_periods=7).std()
    kolom["umur_data"] = umur
    lap = asli_df["nilai_lapangan"].reindex(idx).astype(float)
    bobot_lap = asli_df["bobot_lapangan"].reindex(idx).astype(float)
    kolom["lapangan_selisih_7"] = np.log(lap).rolling(7, min_periods=1).mean() - log
    kolom["lapangan_bobot_7"] = bobot_lap.rolling(7, min_periods=1).mean()

    frame = pd.DataFrame(kolom, index=idx).astype("float32")
    for h in HORIZON:
        depan = pd.Series(umur, index=idx).shift(-h)
        # Label sah hanya bila titik asli sumbernya terletak setelah t dan cukup dekat t+h.
        sah = (depan <= MAKS_UMUR_LABEL_HARI) & (depan < h)
        frame[f"target_h{h}"] = (log.shift(-h) - log).where(sah).astype("float32")
        b = pd.Series(bobot, index=idx)
        frame[f"bobot_h{h}"] = np.minimum(b, b.shift(-h)).astype("float32")

    seri = SeriHarga(
        komoditas_kode=k, kabupaten_kode=w, tanggal=idx, log_harga=log.to_numpy(float), asli=maska,
        bobot=bobot, umur=umur, satuan=str(g["satuan"].dropna().iloc[-1]) if g["satuan"].notna().any() else "Rp/kg",
        sumber=str(g["sumber"].iloc[-1]),
    )
    return seri, frame


def bangun_panel(data: DataMentah, cutoff: pd.Timestamp, min_titik: int = 2) -> Panel:
    """Bangun panel harian. Baris basi (umur besar) dibuang kecuali baris inferensi di cutoff."""
    cutoff = pd.Timestamp(cutoff).normalize()
    if data.harga.empty:
        return Panel(pd.DataFrame(), [], {}, cutoff)
    harga = data.harga[data.harga["tanggal"] <= cutoff]
    if harga.empty:
        return Panel(pd.DataFrame(), [], {}, cutoff)
    indeks = pd.date_range(harga["tanggal"].min(), cutoff, freq="D")
    wilayah = sorted(harga["kabupaten_kode"].astype(str).unique())
    cuaca = _fitur_cuaca(data.cuaca[data.cuaca["tanggal"] <= cutoff] if not data.cuaca.empty else data.cuaca, indeks)
    makro_valid = data.makro[data.makro["tanggal_rilis"] <= cutoff] if not data.makro.empty else data.makro
    makro = _fitur_makro(makro_valid, wilayah, indeks)
    kolom_cuaca = sorted({c for f in cuaca.values() for c in f.columns})
    kolom_makro = sorted({c for f in makro.values() for c in f.columns})
    kalender = fitur_kalender(indeks)

    kode_k = {k: i for i, k in enumerate(sorted(harga["komoditas_kode"].astype(str).unique()))}
    kode_w = {w: i for i, w in enumerate(wilayah)}
    semua_seri: dict[tuple[str, str], SeriHarga] = {}
    bagian: list[pd.DataFrame] = []
    for (k, w), g in harga.groupby(["komoditas_kode", "kabupaten_kode"], sort=True):
        k, w = str(k), str(w)
        if len(g) < min_titik:
            continue
        seri, frame = _seri_dari_grup(k, w, g, cutoff, data.tanggal_asli_terakhir.get((k, w, str(g["sumber"].iloc[-1]))))
        semua_seri[(k, w)] = seri
        eks_cuaca = cuaca.get(w)
        if eks_cuaca is None:
            eks_cuaca = cuaca.get(w[:2])
        if eks_cuaca is not None:
            frame = frame.join(eks_cuaca.reindex(frame.index))
        if w in makro:
            frame = frame.join(makro[w].reindex(frame.index))
        frame = frame.join(kalender.reindex(frame.index))
        simpan = (frame["umur_data"] <= MAKS_UMUR_INFERENSI_HARI).to_numpy()
        simpan[-1] = True
        frame = frame.loc[simpan].copy()
        frame.insert(0, "tanggal", frame.index)
        frame.insert(0, "kabupaten_kode", w)
        frame.insert(0, "komoditas_kode", k)
        frame["komoditas_id"] = np.int16(kode_k[k])
        frame["wilayah_id"] = np.int16(kode_w[w])
        bagian.append(frame.reset_index(drop=True))
    if not bagian:
        return Panel(pd.DataFrame(), [], {}, cutoff)
    df = pd.concat(bagian, ignore_index=True)
    for c in [*kolom_cuaca, *kolom_makro]:
        if c not in df:
            df[c] = np.nan
    harga_kolom = [f"lag_{k}" for k in LAG_HARI] + ["ma_7", "ma_30", "vol_7", "vol_30", "umur_data", "lapangan_selisih_7", "lapangan_bobot_7"]
    kolom_fitur = harga_kolom + kolom_cuaca + kolom_makro + list(kalender.columns) + ["komoditas_id", "wilayah_id"]
    return Panel(df=df, kolom_fitur=kolom_fitur, seri=semua_seri, cutoff=cutoff)
