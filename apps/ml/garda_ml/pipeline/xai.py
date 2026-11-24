"""Penjelasan model: SHAP ``TreeExplainer`` atau dekomposisi untuk model statistik.

Satuan ``kontribusi`` dicatat di ``satuan_kontribusi`` setiap dokumen:

- ``poin_persen_harga`` (target ``forecast``): perubahan P50 terhadap harga terakhir, ruang log x 100.
- ``poin_persen_probabilitas`` (target ``inflasi``): kontribusi pada probabilitas terkalibrasi yang
  dipublikasikan, sehingga jumlah semua kontribusi = (probabilitas - probabilitas_dasar) x 100.
  Lihat :func:`kontribusi_probabilitas`.

Fitur identitas (komoditas, wilayah) bukan faktor kebijakan sehingga tidak dilaporkan.
"""

from __future__ import annotations

import logging
from typing import Any

import numpy as np

from .config import HORIZON
from .regresi import HasilRamalan

log = logging.getLogger("garda_ml.pipeline")

LABEL_KELOMPOK = {
    "CUACA": "Cuaca",
    "PASOKAN": "Pasokan",
    "KONSUMSI_PENDAPATAN": "Konsumsi dan pendapatan",
    "TENAGA_KERJA": "Tenaga kerja",
    "NERACA_EKONOMI": "Neraca ekonomi",
    "TREN_HARGA_LAPANGAN": "Tren harga dan lapangan",
    "KALENDER": "Kalender hari besar",
}
_IDENTITAS = {"komoditas_id", "wilayah_id"}
HORIZON_XAI = 30


def kelompok_fitur(nama: str) -> str | None:
    """Petakan nama fitur ke kelompok faktor kontrak. ``None`` untuk fitur identitas."""
    if nama in _IDENTITAS:
        return None
    if nama.startswith("cuaca_"):
        return "CUACA"
    if nama.startswith("kal_"):
        return "KALENDER"
    if nama == "tekanan_pasokan" or nama.startswith(("makro_PERTANIAN", "makro_PERIKANAN", "makro_KEHUTANAN")):
        return "PASOKAN"
    if nama == "tekanan_neraca" or nama.startswith("makro_NERACA_EKONOMI"):
        return "NERACA_EKONOMI"
    if nama == "biaya_tenaga_kerja" or nama.startswith("makro_TENAGA_KERJA"):
        return "TENAGA_KERJA"
    if nama.startswith("makro_KONSUMSI_PENDAPATAN"):
        return "KONSUMSI_PENDAPATAN"
    return "TREN_HARGA_LAPANGAN"  # lag, MA, volatilitas, umur data, lapangan, inflasi lalu


def faktor_dari_nilai(nama_fitur: list[str], nilai: np.ndarray, skala: float = 100.0, maks_fitur: int = 3, masukan: np.ndarray | None = None) -> list[dict]:
    """Agregasi kontribusi per kelompok, terurut menurut besar absolut.

    ``masukan`` adalah nilai fitur baris itu. Fitur yang kosong dibuang, sehingga kelompok
    tanpa data (mis. ``CUACA`` saat riwayat iklim tidak ada) tidak pernah muncul.
    """
    kelompok: dict[str, list[tuple[str, float]]] = {}
    for i, (nama, v) in enumerate(zip(nama_fitur, nilai)):
        grup = kelompok_fitur(nama)
        if grup is None or not np.isfinite(v) or (masukan is not None and not np.isfinite(masukan[i])):
            continue
        kelompok.setdefault(grup, []).append((nama, float(v) * skala))
    faktor = []
    for grup, isi in kelompok.items():
        total = sum(v for _, v in isi)
        if abs(total) < 1e-9 and all(abs(v) < 1e-9 for _, v in isi):
            continue
        teratas = sorted(isi, key=lambda x: abs(x[1]), reverse=True)[:maks_fitur]
        faktor.append({
            "kelompok": grup, "label": LABEL_KELOMPOK[grup], "kontribusi": round(total, 4),
            "arah": "NAIK" if total >= 0 else "TURUN",
            "fitur": [{"nama": n, "kontribusi": round(v, 4)} for n, v in teratas],
        })
    return sorted(faktor, key=lambda f: abs(f["kontribusi"]), reverse=True)


def narasi_forecast(nama: str, wilayah: str, perubahan_pct: float, faktor: list[dict], model: str) -> str:
    if model == "naive":
        return f"Harga {nama} di wilayah {wilayah} diproyeksikan tetap pada harga terakhir (baseline persistensi)."
    arah = "naik" if perubahan_pct > 0.05 else "turun" if perubahan_pct < -0.05 else "relatif stabil"
    besar = "" if arah == "relatif stabil" else f" {abs(perubahan_pct):.1f}%"
    teks = f"Harga {nama} di wilayah {wilayah} diproyeksikan {arah}{besar} dalam {HORIZON_XAI} hari (model {model})."
    if faktor:
        utama = ", ".join(f"{f['label'].lower()} ({'mendorong naik' if f['kontribusi'] >= 0 else 'menekan turun'})" for f in faktor[:2])
        teks += f" Faktor terbesar: {utama}."
    return teks


def narasi_inflasi(wilayah: str, sinyal: dict, faktor: list[dict], dasar: float) -> str:
    sebutan = "inflasi resmi BPS" if sinyal["target"] == "RESMI_BPS" else "indeks harga pangan (bukan inflasi resmi)"
    teks = (
        f"Probabilitas kenaikan {sebutan} di wilayah {wilayah} sebesar {sinyal['probabilitas'] * 100:.0f}% "
        f"(dasar {dasar * 100:.0f}%), siklus {sinyal['siklus']}."
    )
    if faktor:
        teks += " Faktor terbesar: " + ", ".join(f["label"].lower() for f in faktor[:2]) + "."
    return teks


def nilai_shap(model: Any, X: np.ndarray, penjelas: Any = None) -> np.ndarray:
    """Nilai SHAP per baris x fitur; untuk klasifikasi diambil kelas positif."""
    import shap

    keluaran = (penjelas or shap.TreeExplainer(model)).shap_values(X)
    if isinstance(keluaran, list):
        keluaran = keluaran[-1]
    arr = np.asarray(keluaran)
    if arr.ndim == 3:
        arr = arr[:, :, -1]
    return arr


def _sigmoid(z: np.ndarray) -> np.ndarray:
    return 1.0 / (1.0 + np.exp(-z))


def kontribusi_probabilitas(model: Any, X: np.ndarray) -> tuple[np.ndarray, np.ndarray, np.ndarray]:
    """Atribusi SHAP pada probabilitas terkalibrasi: ``(kontribusi, probabilitas, dasar)``.

    ``CalibratedClassifierCV`` (``ensemble=True``) mempublikasikan rata-rata ``kalibrator_i(skor_i)``
    dari model tiap lipatan. Untuk tiap model lipatan ``i``:

    1. SHAP ``TreeExplainer`` memberi kontribusi log-odds ``phi_i`` dengan nilai dasar ``E_i``,
       sehingga ``skor mentah = E_i + sum(phi_i)``.
    2. ``p_i = kalibrator_i(skor)`` dan ``p0_i = kalibrator_i(E_i)``.
    3. ``phi_i`` diskalakan proporsional ke ``p_i - p0_i``. Kalibrator monoton, sehingga tanda
       kontribusi tidak berbalik.

    Hasilnya dirata-rata antar lipatan, persis seperti probabilitasnya. Maka
    ``sum(kontribusi) = probabilitas - dasar``, dengan ``dasar = rata-rata p0_i``. Model tanpa
    kalibrasi memakai sigmoid sebagai kalibratornya.
    """
    import shap

    if hasattr(model, "calibrated_classifiers_"):
        pasangan = [(c.estimator, c.calibrators[0]) for c in model.calibrated_classifiers_]
    else:
        pasangan = [(model, None)]
    total_c = np.zeros(X.shape, dtype=float)
    total_p = np.zeros(len(X))
    total_p0 = np.zeros(len(X))
    for estimator, kalibrator in pasangan:
        penjelas = shap.TreeExplainer(estimator)
        phi = nilai_shap(estimator, X, penjelas)
        dasar = float(np.ravel(penjelas.expected_value)[-1])
        skor_mentah = dasar + phi.sum(axis=1)
        if kalibrator is None:
            p, p0 = _sigmoid(skor_mentah), np.full(len(X), _sigmoid(np.array(dasar)))
        else:
            # Kalibrator sklearn menerima decision_function bila ada, selain itu probabilitas.
            ubah = (lambda z: z) if hasattr(estimator, "decision_function") else _sigmoid
            p = np.clip(kalibrator.predict(ubah(skor_mentah)), 0.0, 1.0)
            p0 = np.full(len(X), float(np.clip(kalibrator.predict(ubah(np.array([dasar]))), 0.0, 1.0)[0]))
        jumlah = phi.sum(axis=1)
        faktor = np.divide(p - p0, jumlah, out=np.zeros(len(X)), where=np.abs(jumlah) > 1e-12)
        total_c += phi * faktor[:, None]
        total_p += p
        total_p0 += p0
    n = len(pasangan)
    return total_c / n, total_p / n, total_p0 / n


def dekomposisi_seri(y: np.ndarray, perubahan_log: float) -> tuple[list[str], np.ndarray]:
    """Uraikan proyeksi model statistik menjadi momentum, pola mingguan, dan tren.

    Momentum dan pola mingguan dihitung dari riwayat. Tren adalah sisanya, sehingga jumlah
    kontribusi sama dengan perubahan yang diproyeksikan.
    """
    n = len(y)
    momentum = float(y[-1] - y[-8]) if n >= 8 else 0.0
    musiman = 0.0
    if n >= 21:
        ratarata = np.convolve(y, np.ones(7) / 7, mode="valid")
        sisa = y[6:] - ratarata
        hari_target = (len(sisa) - 1 + HORIZON_XAI) % 7
        efek = [float(np.mean(sisa[i::7])) for i in range(7)]
        musiman = efek[hari_target] - efek[(len(sisa) - 1) % 7]
    sisa_tren = perubahan_log - momentum - musiman
    return ["tren_jangka_menengah", "momentum_7_hari", "kal_pola_mingguan"], np.array([sisa_tren, momentum, musiman])


def jelaskan_forecast(hasil: list[HasilRamalan], panel_df: Any, kolom: list[str], model_global: dict, nama_komoditas: dict[str, str], cutoff: Any) -> tuple[list[dict], list[dict]]:
    """Kembalikan (dokumen XAI tanpa metadata siklus, daftar kegagalan)."""
    gagal: list[dict] = []
    keluaran: list[dict] = []
    idx_h = list(HORIZON).index(HORIZON_XAI)
    pakai_shap = [h for h in hasil if h.nama_model.endswith("_quantile")]
    shap_map: dict[tuple[str, str], tuple[np.ndarray, np.ndarray]] = {}
    if pakai_shap and model_global:
        baris = panel_df[panel_df["tanggal"] == cutoff]
        kunci = list(zip(baris["komoditas_kode"], baris["kabupaten_kode"]))
        posisi = {k: i for i, k in enumerate(kunci)}
        pilih = [posisi[(h.seri.komoditas_kode, h.seri.kabupaten_kode)] for h in pakai_shap if (h.seri.komoditas_kode, h.seri.kabupaten_kode) in posisi]
        if pilih:
            X = baris.iloc[pilih][kolom].to_numpy(dtype=np.float32)
            try:
                nilai = nilai_shap(model_global[HORIZON_XAI][0.5], X)
                for b, i in enumerate(pilih):
                    shap_map[kunci[i]] = (nilai[b], X[b])
            except Exception as err:  # noqa: BLE001 - kegagalan SHAP tidak boleh menjatuhkan siklus
                log.exception("SHAP gagal")
                gagal.append({"tahap": "shap_forecast", "alasan": str(err)[:200]})
    for h in hasil:
        k, w = h.seri.komoditas_kode, h.seri.kabupaten_kode
        try:
            p50 = float(h.kuantil[idx_h, 1])
            perubahan_log = float(np.log(p50 / h.seri.harga_terakhir))
            if (k, w) in shap_map:
                metode = "shap"
                faktor = faktor_dari_nilai(kolom, shap_map[(k, w)][0], masukan=shap_map[(k, w)][1])
            elif h.nama_model == "naive":
                # Persistensi tidak memakai faktor apa pun, jadi tidak ada yang bisa diatribusikan.
                metode, faktor = "dekomposisi", []
            else:
                metode = "dekomposisi"
                nama, nilai = dekomposisi_seri(h.seri.log_harga[: h.seri.pos_terakhir + 1], perubahan_log)
                faktor = faktor_dari_nilai(nama, nilai)
            nama_k = nama_komoditas.get(k, k)
            keluaran.append({
                "target": "forecast", "komoditas_kode": k, "wilayah_kode": w, "metode": metode,
                "satuan_kontribusi": "poin_persen_harga", "faktor": faktor,
                "narasi": narasi_forecast(nama_k, w, float(np.expm1(perubahan_log) * 100), faktor, h.nama_model),
            })
        except Exception as err:  # noqa: BLE001
            gagal.append({"tahap": "xai_forecast", "komoditas_kode": k, "kabupaten_kode": w, "alasan": str(err)[:200]})
    return keluaran, gagal


def jelaskan_inflasi(klasifikasi: Any) -> tuple[list[dict], list[dict]]:
    keluaran: list[dict] = []
    gagal: list[dict] = []
    per_wilayah = {s["wilayah_kode"]: s for s in klasifikasi.sinyal}
    for target, mk in klasifikasi.model.items():
        if not mk.metrik.get("layak"):
            continue  # kontrak: model tidak layak tidak diberi penjelasan
        wilayah = [w for w, (t, _) in klasifikasi.baris_inferensi.items() if t == target]
        if not wilayah:
            continue
        X = np.vstack([klasifikasi.baris_inferensi[w][1] for w in wilayah])
        try:
            kontribusi, p, p0 = kontribusi_probabilitas(mk.model, X)
            dipublikasikan = mk.model.predict_proba(X)[:, 1]
            if not np.allclose(p, dipublikasikan, atol=1e-6):
                raise ValueError(f"atribusi tidak cocok dengan probabilitas terpublikasi (selisih {np.abs(p - dipublikasikan).max():.2e})")
        except Exception as err:  # noqa: BLE001
            log.exception("SHAP klasifikasi gagal")
            gagal.append({"tahap": "shap_inflasi", "target": target, "alasan": str(err)[:200]})
            continue
        for b, w in enumerate(wilayah):
            # Semua fitur ikut agar jumlahnya tepat sama dengan selisih probabilitas.
            faktor = faktor_dari_nilai(mk.kolom, kontribusi[b])
            keluaran.append({
                "target": "inflasi", "komoditas_kode": None, "wilayah_kode": w, "metode": "shap",
                "satuan_kontribusi": "poin_persen_probabilitas",
                "probabilitas": round(float(p[b]), 6), "probabilitas_dasar": round(float(p0[b]), 6), "faktor": faktor,
                "narasi": narasi_inflasi(w, per_wilayah[w], faktor, float(p0[b])),
            })
    return keluaran, gagal
