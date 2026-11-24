"""Deteksi anomali dan pembentukan sinyal EWS.

**Z-score musiman adaptif:** deviasi log harga dari median 8 hari-sepadan (t-7, t-14, ...)
dibandingkan dengan median dan IQR deviasi 180 hari *sebelum* t, sehingga ambang mengikuti
volatilitas tiap seri. **Isolation Forest** dilatih dari riwayat sebelum titik yang dinilai.

Hanya titik berbobot >= 0,8 (resmi, ``DIVALIDASI``, ``DIPUBLIKASI``) yang dinilai agar satu
input ``DIAJUKAN`` tidak memicu alarm. Penilaian hanya untuk titik asli yang masih segar.

``sinyal_kunci`` stabil lintas siklus untuk kondisi yang sama. Tidak ada cooldown di sini:
semua sinyal, termasuk ``KRITIS``, selalu ditulis pada setiap siklus selama kondisinya ada.
"""

from __future__ import annotations

import logging

import numpy as np
import pandas as pd
from sklearn.ensemble import IsolationForest

from .config import HORIZON
from .fitur import SeriHarga
from .regresi import HasilRamalan

log = logging.getLogger("garda_ml.pipeline")

SEVERITAS = ("KRITIS", "SIAGA", "WASPADA", "INFO")
MAKS_UMUR_ANOMALI_HARI = 3
MIN_BOBOT_ANOMALI = 0.8
JENDELA_ROBUST = 180
MIN_TITIK_ANOMALI = 45
KOLOM_IF = ["dev", "ret_1", "ret_7", "rasio_vol"]

SUMBER_HARIAN = {"PIHPS", "BMKG", "OPEN_METEO", "SP2KP", "BAPANAS", "LAPANGAN"}
SUMBER_HARGA = {"PIHPS", "SP2KP", "BAPANAS"}


def fitur_anomali(seri: SeriHarga) -> pd.DataFrame:
    """Deviasi musiman dan skor Z tiap tanggal; statistik acuan digeser satu hari."""
    log_h = pd.Series(seri.log_harga, index=seri.tanggal)
    acuan = pd.concat([log_h.shift(7 * k) for k in range(1, 9)], axis=1).median(axis=1, skipna=True)
    dev = log_h - acuan
    bergulir = dev.shift(1).rolling(JENDELA_ROBUST, min_periods=30)
    median = bergulir.median()
    iqr = bergulir.quantile(0.75) - bergulir.quantile(0.25)
    skala = (iqr / 1.349).clip(lower=0.005)  # lantai 0,5% agar seri datar tidak meledakkan Z
    ret = log_h.diff()
    vol_7 = ret.rolling(7, min_periods=3).std()
    vol_90 = ret.shift(1).rolling(90, min_periods=20).std().clip(lower=1e-4)
    return pd.DataFrame({
        "dev": dev, "z": (dev - median) / skala, "ret_1": ret, "ret_7": log_h - log_h.shift(7),
        "rasio_vol": vol_7 / vol_90,
    })


def tingkat_anomali(z: float, dev_pct: float, outlier_if: bool) -> str | None:
    """Ambang gabungan Z dan besar deviasi. 25% selaras ``AmbangAnomaliPct`` di API."""
    az, ad = abs(z), abs(dev_pct)
    if (az >= 3.5 and ad >= 25) or (az >= 6 and ad >= 15):
        return "KRITIS"
    if az >= 3.5 and ad >= 10:
        return "SIAGA"
    if (az >= 2.5 and ad >= 5) or (outlier_if and az >= 2 and ad >= 5):
        return "WASPADA"
    return None


def deteksi_anomali(semua_seri: dict[tuple[str, str], SeriHarga], nama: dict[str, str], maks_baris_if: int = 50_000) -> list[dict]:
    kandidat: list[tuple[SeriHarga, pd.Series]] = []
    latih: list[pd.DataFrame] = []
    for seri in semua_seri.values():
        if seri.n_asli < MIN_TITIK_ANOMALI or seri.umur_terakhir > MAKS_UMUR_ANOMALI_HARI:
            continue
        if seri.bobot[seri.pos_terakhir] < MIN_BOBOT_ANOMALI:
            continue
        f = fitur_anomali(seri)
        titik = f.iloc[seri.pos_terakhir]
        if not np.isfinite(titik["z"]) or not np.isfinite(titik["dev"]):
            continue
        kandidat.append((seri, titik))
        riwayat = f.iloc[: seri.pos_terakhir][KOLOM_IF].replace([np.inf, -np.inf], np.nan).dropna()
        latih.append(riwayat.iloc[-JENDELA_ROBUST:])
    if not kandidat:
        return []

    outlier = np.zeros(len(kandidat), dtype=bool)
    gabung = pd.concat(latih) if latih else pd.DataFrame()
    if len(gabung) >= 200:
        if len(gabung) > maks_baris_if:
            gabung = gabung.sample(maks_baris_if, random_state=42)
        hutan = IsolationForest(n_estimators=100, contamination=0.01, random_state=42, n_jobs=1).fit(gabung.to_numpy())
        uji = pd.DataFrame([t[KOLOM_IF] for _, t in kandidat]).replace([np.inf, -np.inf], np.nan).fillna(0.0).to_numpy()
        outlier = hutan.predict(uji) == -1

    sinyal = []
    for (seri, t), is_out in zip(kandidat, outlier):
        dev_pct = float(np.expm1(t["dev"]) * 100)
        tingkat = tingkat_anomali(float(t["z"]), dev_pct, bool(is_out))
        if tingkat is None:
            continue
        arah = "NAIK" if dev_pct > 0 else "TURUN"
        k, w = seri.komoditas_kode, seri.kabupaten_kode
        metode = "zscore_musiman+isolation_forest" if is_out else "zscore_musiman"
        sinyal.append({
            "sinyal_kunci": f"ANOMALI_HARGA:{k}:{w}:{arah}", "severity": tingkat, "jenis": "ANOMALI_HARGA",
            "komoditas_kode": k, "kabupaten_kode": w, "nilai": round(dev_pct, 3), "ambang": 25.0 if tingkat == "KRITIS" else 10.0 if tingkat == "SIAGA" else 5.0,
            "metode": metode, "judul": f"Anomali harga {nama.get(k, k)} ({arah.lower()})",
            "pesan": f"Harga {nama.get(k, k)} di wilayah {w} pada {seri.tanggal_terakhir.date()} menyimpang {dev_pct:+.1f}% dari pola musimannya (z={float(t['z']):.1f}).",
            "aktif": True,
        })
    return sinyal


def sinyal_proyeksi(hasil: list[HasilRamalan], nama: dict[str, str]) -> list[dict]:
    """Lonjakan proyeksi: KRITIS hanya bila P10 pun berada di atas harga terakhir."""
    sinyal = []
    for r in hasil:
        harga = r.seri.harga_terakhir
        ubah = (r.kuantil[:, 1] / harga - 1) * 100
        i = int(np.argmax(ubah))
        naik = float(ubah[i])
        p10_atas = bool(r.kuantil[i, 0] > harga)
        if naik >= 20 and p10_atas:
            tingkat, ambang = "KRITIS", 20.0
        elif naik >= 10:
            tingkat, ambang = "SIAGA", 10.0
        elif naik >= 5:
            tingkat, ambang = "WASPADA", 5.0
        else:
            continue
        k, w = r.seri.komoditas_kode, r.seri.kabupaten_kode
        h = HORIZON[i]
        sinyal.append({
            "sinyal_kunci": f"PROYEKSI_LONJAKAN:{k}:{w}", "severity": tingkat, "jenis": "PROYEKSI_LONJAKAN",
            "komoditas_kode": k, "kabupaten_kode": w, "nilai": round(naik, 3), "ambang": ambang, "metode": r.nama_model,
            "judul": f"Proyeksi lonjakan {nama.get(k, k)}",
            "pesan": f"Harga {nama.get(k, k)} di wilayah {w} diproyeksikan naik {naik:.1f}% pada H+{h} (P50).",
            "aktif": True,
        })
    return sinyal


def sinyal_tekanan(sinyal_inflasi: list[dict]) -> list[dict]:
    keluaran = []
    for s in sinyal_inflasi:
        skor = s.get("skor_tekanan")
        if skor is None or not s.get("layak", True):
            continue  # model belum layak: probabilitasnya bukan prediksi
        if skor >= 90:
            tingkat, ambang = "KRITIS", 90.0
        elif skor >= 75:
            tingkat, ambang = "SIAGA", 75.0
        elif skor >= 60:
            tingkat, ambang = "WASPADA", 60.0
        else:
            continue
        sebutan = "inflasi resmi BPS" if s["target"] == "RESMI_BPS" else "indeks harga pangan"
        keluaran.append({
            "sinyal_kunci": f"TEKANAN_INFLASI:{s['wilayah_kode']}", "severity": tingkat, "jenis": "TEKANAN_INFLASI",
            "komoditas_kode": None, "kabupaten_kode": s["wilayah_kode"], "nilai": skor, "ambang": ambang, "metode": "klasifikasi_terkalibrasi",
            "judul": f"Tekanan {sebutan}",
            "pesan": f"Skor tekanan {sebutan} wilayah {s['wilayah_kode']} sebesar {skor:.0f} (probabilitas {s['probabilitas'] * 100:.0f}%).",
            "aktif": True,
        })
    return keluaran


def sinyal_sumber(kesegaran: list[dict], ada_harga: bool) -> list[dict]:
    """Sinyal kesehatan sumber. Keterlambatan wajar sumber bulanan tidak menjadi alarm."""
    keluaran = []
    for s in kesegaran:
        sumber, dataset, umur = s["sumber"], s["dataset"], s.get("umur_hari")
        harian = sumber in SUMBER_HARIAN
        batas_waspada, batas_siaga = (7, 30) if harian else (75, 120)
        tingkat, ambang = None, None
        dilewati = str(s.get("pesan") or "").startswith("DILEWATI:")
        if dilewati:
            # Adapter sengaja tidak dijalankan (robots atau butuh kunci): dicatat, bukan alarm.
            tingkat, ambang = "INFO", 0.0
        elif s.get("status") == "ERROR":
            tingkat, ambang = "SIAGA", 0.0
        elif umur is not None and umur > batas_siaga:
            tingkat, ambang = "SIAGA", float(batas_siaga)
        elif umur is not None and umur > batas_waspada:
            tingkat, ambang = "WASPADA", float(batas_waspada)
        if tingkat is None:
            continue
        lad = s.get("last_available_date")
        teks_lad = lad.date().isoformat() if lad is not None else "tidak diketahui"
        keluaran.append({
            "sinyal_kunci": f"SUMBER_DATA:{sumber}:{dataset}", "severity": tingkat, "jenis": "SUMBER_DATA",
            "komoditas_kode": None, "kabupaten_kode": None, "nilai": None if umur is None else float(umur), "ambang": ambang,
            "metode": "kesegaran_sumber", "judul": f"Sumber {sumber} {'dilewati' if dilewati else 'gagal' if s.get('status') == 'ERROR' else 'basi'}: {dataset}",
            "pesan": f"Status {s.get('status')}, data asli terakhir {teks_lad}. {s.get('pesan') or ''}".strip(),
            "aktif": True,
        })
    harga_segar = [
        s for s in kesegaran
        if s["sumber"] in SUMBER_HARGA and s.get("status") != "ERROR" and not str(s.get("pesan") or "").startswith("DILEWATI:")
        and s.get("umur_hari") is not None and s["umur_hari"] <= 7
    ]
    if not ada_harga or not harga_segar:
        pesan = "Tidak ada data harga yang dapat dilatih." if not ada_harga else "Tidak ada sumber harga yang segar dalam 7 hari."
        keluaran.append({
            "sinyal_kunci": "SUMBER_DATA:HARGA:SEMUA", "severity": "KRITIS", "jenis": "SUMBER_DATA", "komoditas_kode": None,
            "kabupaten_kode": None, "nilai": None, "ambang": 7.0, "metode": "kesegaran_sumber",
            "judul": "Tidak ada sumber harga yang segar", "pesan": pesan, "aktif": True,
        })
    return keluaran


def tandai_pulih(sekarang: list[dict], sebelumnya: list[dict]) -> list[dict]:
    """Sinyal aktif siklus lalu yang kondisinya hilang dicatat sekali sebagai ``aktif: false``."""
    kunci_kini = {s["sinyal_kunci"] for s in sekarang}
    pulih = []
    for s in sebelumnya:
        if not s.get("aktif") or s["sinyal_kunci"] in kunci_kini:
            continue
        pulih.append({
            "sinyal_kunci": s["sinyal_kunci"], "severity": "INFO", "jenis": s["jenis"],
            "komoditas_kode": s.get("komoditas_kode"), "kabupaten_kode": s.get("kabupaten_kode"), "nilai": None, "ambang": s.get("ambang"),
            "metode": s.get("metode"), "judul": f"Pulih: {s.get('judul', '')}",
            "pesan": f"Kondisi {s['sinyal_kunci']} (sebelumnya {s.get('severity')}) tidak lagi terdeteksi.", "aktif": False,
        })
    return pulih
