"""Tes unit pipeline tanpa Mongo: kebocoran, penggabungan, kalender, seleksi, anomali."""

from __future__ import annotations

from datetime import date, datetime, timezone

import numpy as np
import pandas as pd
import pytest

from garda_ml.pipeline import kalender
from garda_ml.pipeline.anomali import (
    deteksi_anomali,
    sinyal_proyeksi,
    sinyal_sumber,
    sinyal_tekanan,
    tandai_pulih,
    tingkat_anomali,
)
from garda_ml.pipeline.backend import Backend, pilih_backend
from garda_ml.pipeline.config import (
    BOBOT_LAPANGAN,
    HORIZON,
    JARAK_ASAL_HARI,
    LANGKAH_LIPATAN_HARI,
    MARGIN_BASELINE,
    MIN_LIPATAN_SELEKSI,
    N_ASAL_PER_LIPATAN,
    N_LIPATAN_HOLDOUT,
    N_LIPATAN_SELEKSI,
    Konfigurasi,
)
from garda_ml.pipeline.ekstraksi import DataMentah, domain_makro, gabung_harga, tanggal_rilis_konservatif
from garda_ml.pipeline.fitur import asof_rilis, bangun_panel
from garda_ml.pipeline.klasifikasi import MIN_AUC_LAYAK, jalankan_klasifikasi, lipatan_waktu, nilai_kelayakan, seri_inflasi_resmi, skor_tekanan, tentukan_siklus
from garda_ml.pipeline.penyimpanan import bersihkan
from garda_ml.pipeline.regresi import BASELINES, _aktual, _indeks_latih, _nilai_seri, baseline_terbaik, jalankan_regresi, lipatan_asal, pilih_pemenang, skor_seimbang
from garda_ml.pipeline.statistik import RAMALAN, kandidat_statistik, rapikan_kuantil
from garda_ml.pipeline.waktu import detik_ke_siklus_berikut, id_siklus, tanggal_wib
from garda_ml.pipeline.xai import dekomposisi_seri, faktor_dari_nilai, jelaskan_inflasi, kelompok_fitur, kontribusi_probabilitas

AKHIR = pd.Timestamp("2026-06-30")
HGB = Backend("hist_gradient_boosting", "hgb_quantile", "uji", 1)
CFG = Konfigurasi(mongo_uri="mongodb://tidak-dipakai", n_jobs=1, maks_baris_latih=20_000)
# LightGBM ikut diuji bila bisa dimuat (jalur Docker), HGB selalu.
BACKEND_UJI = [HGB] + ([pilih_backend(1)] if not pilih_backend(1).fallback else [])
WILAYAH_6 = ("1171", "1174", "1107", "1173", "1105", "1106")


def _harga_acak(hari: int = 1100, wilayah=WILAYAH_6, komoditas=("CBM-KRT", "BRS-MED"), seed: int = 31) -> pd.DataFrame:
    """Random walk murni: label indeks pangan tidak dapat diprediksi."""
    rng = np.random.default_rng(seed)
    t = pd.date_range(AKHIR - pd.Timedelta(days=hari), AKHIR, freq="D")
    bagian = []
    for k in komoditas:
        for w in wilayah:
            d = pd.DataFrame({"komoditas_kode": k, "kabupaten_kode": w, "tanggal": t, "nilai": np.exp(np.log(20000) + np.cumsum(rng.normal(0, 0.005, len(t)))), "bobot": 1.0, "nilai_lapangan": np.nan, "bobot_lapangan": np.nan, "sumber": "PIHPS", "satuan": "Rp/kg"})
            bagian.append(d[d["tanggal"].dt.dayofweek < 5])
    return pd.concat(bagian, ignore_index=True)


def _harga_musiman(hari: int = 1100, amplitudo: float = 0.05, derau: float = 0.002, komoditas=("CBM-KRT", "BRS-MED", "DAG-AYM"), wilayah=("1171", "1174", "1107"), seed: int = 3) -> pd.DataFrame:
    """Musiman tahunan bersama: label indeks pangan dapat diprediksi dari kalender."""
    rng = np.random.default_rng(seed)
    t = pd.date_range(AKHIR - pd.Timedelta(days=hari), AKHIR, freq="D")
    bagian = []
    for k in komoditas:
        for w in wilayah:
            log_h = np.log(20000) + np.cumsum(rng.normal(0, derau, len(t))) + amplitudo * np.sin(2 * np.pi * t.dayofyear / 365.25)
            d = pd.DataFrame({"komoditas_kode": k, "kabupaten_kode": w, "tanggal": t, "nilai": np.exp(log_h), "bobot": 1.0, "nilai_lapangan": np.nan, "bobot_lapangan": np.nan, "sumber": "PIHPS", "satuan": "Rp/kg"})
            bagian.append(d[d["tanggal"].dt.dayofweek < 5])
    return pd.concat(bagian, ignore_index=True)


def _harga(hari: int = 400, komoditas=("CBM-KRT", "BRS-MED"), wilayah=("1171",), seed: int = 1, akhir: pd.Timestamp = AKHIR) -> pd.DataFrame:
    rng = np.random.default_rng(seed)
    bagian = []
    for k in komoditas:
        for w in wilayah:
            t = pd.date_range(akhir - pd.Timedelta(days=hari), akhir, freq="D")
            log_h = np.log(20000) + np.cumsum(rng.normal(0.0002, 0.005, len(t))) + 0.01 * np.sin(2 * np.pi * t.dayofweek / 7)
            d = pd.DataFrame({"komoditas_kode": k, "kabupaten_kode": w, "tanggal": t, "nilai": np.exp(log_h), "bobot": 1.0, "nilai_lapangan": np.nan, "bobot_lapangan": np.nan, "sumber": "PIHPS", "satuan": "Rp/kg"})
            bagian.append(d[d["tanggal"].dt.dayofweek < 5])
    return pd.concat(bagian, ignore_index=True)


def _data(harga: pd.DataFrame, makro: pd.DataFrame | None = None, cuaca: pd.DataFrame | None = None) -> DataMentah:
    kolom_makro = ["domain", "indikator", "wilayah_kode", "periode", "tanggal_rilis", "nilai", "satuan", "sumber", "frekuensi", "is_carried_forward", "last_available_date"]
    kosong = pd.DataFrame(columns=kolom_makro)
    return DataMentah(harga=harga, cuaca=cuaca if cuaca is not None else pd.DataFrame(columns=["kabupaten_kode", "tanggal", "indikator", "nilai"]), makro=makro if makro is not None else kosong, makro_terkini=kosong, kesegaran=[])


def _makro_bulanan(nilai_awal=100.0, n=30, domain="TENAGA_KERJA", indikator="upah_buruh", wilayah="11") -> pd.DataFrame:
    periode = pd.date_range(AKHIR - pd.DateOffset(months=n), periods=n, freq="MS")
    frek = pd.Series(["BULANAN"] * n)
    rilis = tanggal_rilis_konservatif(pd.Series(periode), frek)
    return pd.DataFrame({"domain": domain, "indikator": indikator, "wilayah_kode": wilayah, "periode": periode, "tanggal_rilis": rilis.values, "nilai": nilai_awal + np.arange(n), "satuan": "Rp", "sumber": "BPS", "frekuensi": "BULANAN", "is_carried_forward": False, "last_available_date": periode})


class TestKebocoran:
    def test_fitur_tidak_berubah_saat_masa_depan_diubah(self):
        harga = _harga()
        titik = pd.Timestamp("2026-03-16")
        ubah = harga.copy()
        ubah.loc[ubah["tanggal"] > titik, "nilai"] *= 3.0
        a, b = bangun_panel(_data(harga), AKHIR), bangun_panel(_data(ubah), AKHIR)
        kolom = a.kolom_fitur
        x = a.df[a.df["tanggal"] <= titik].sort_values(["komoditas_kode", "tanggal"])[kolom].to_numpy(float)
        y = b.df[b.df["tanggal"] <= titik].sort_values(["komoditas_kode", "tanggal"])[kolom].to_numpy(float)
        np.testing.assert_array_equal(x, y)

    def test_fitur_sama_dengan_panel_yang_dipotong(self):
        harga, makro = _harga(), _makro_bulanan()
        titik = pd.Timestamp("2026-03-16")
        penuh = bangun_panel(_data(harga, makro), AKHIR)
        potong = bangun_panel(_data(harga[harga["tanggal"] <= titik], makro[makro["tanggal_rilis"] <= titik]), titik)
        a = penuh.df[penuh.df["tanggal"] == titik].sort_values("komoditas_kode")
        b = potong.df[potong.df["tanggal"] == titik].sort_values("komoditas_kode")
        kolom = [c for c in potong.kolom_fitur if c in penuh.kolom_fitur]
        assert len(kolom) > 20
        np.testing.assert_allclose(a[kolom].to_numpy(float), b[kolom].to_numpy(float), rtol=1e-5, atol=1e-5)

    def test_cuaca_dijeda_ketersediaan(self):
        harga = _harga(hari=60, komoditas=("CBM-KRT",))
        t = pd.date_range(AKHIR - pd.Timedelta(days=60), AKHIR, freq="D")
        cuaca = pd.DataFrame({"kabupaten_kode": "1171", "tanggal": t, "indikator": "curah_hujan_harian", "nilai": 1.0})
        ubah = cuaca.copy()
        ubah.loc[ubah["tanggal"] >= AKHIR - pd.Timedelta(days=1), "nilai"] = 500.0  # dua hari terakhir belum tersedia
        a = bangun_panel(_data(harga, cuaca=cuaca), AKHIR).df.iloc[-1]["cuaca_hujan_7"]
        b = bangun_panel(_data(harga, cuaca=ubah), AKHIR).df.iloc[-1]["cuaca_hujan_7"]
        assert a == b == pytest.approx(1.0)

    def test_target_bukan_fitur(self):
        panel = bangun_panel(_data(_harga()), AKHIR)
        assert not [c for c in panel.kolom_fitur if c.startswith(("target_", "bobot_h"))]

    def test_makro_tidak_terlihat_sebelum_rilis(self):
        g = pd.DataFrame({"periode": [pd.Timestamp("2026-01-01"), pd.Timestamp("2026-02-01")], "tanggal_rilis": [pd.Timestamp("2026-03-07"), pd.Timestamp("2026-04-04")], "nilai": [10.0, 20.0]})
        t = pd.DatetimeIndex(["2026-03-06", "2026-03-07", "2026-04-03", "2026-04-04"])
        nilai, _ = asof_rilis(t, g)
        assert np.isnan(nilai[0]) and nilai[1] == 10 and nilai[2] == 10 and nilai[3] == 20

    def test_rilis_terlambat_tidak_menggantikan_periode_baru(self):
        g = pd.DataFrame({"periode": [pd.Timestamp("2026-02-01"), pd.Timestamp("2026-01-01")], "tanggal_rilis": [pd.Timestamp("2026-03-10"), pd.Timestamp("2026-03-20")], "nilai": [20.0, 10.0]})
        nilai, _ = asof_rilis(pd.DatetimeIndex(["2026-03-25"]), g)
        assert nilai[0] == 20

    def test_asumsi_rilis_bulanan_35_hari(self):
        r = tanggal_rilis_konservatif(pd.Series([pd.Timestamp("2026-01-01")]), pd.Series(["BULANAN"]))
        assert r.iloc[0] == pd.Timestamp("2026-01-31") + pd.Timedelta(days=35)

    def test_baris_latih_hanya_label_yang_sudah_diketahui(self):
        panel = bangun_panel(_data(_harga()), AKHIR)
        asal = pd.Timestamp("2026-04-01")
        for h in HORIZON:
            idx = _indeks_latih(panel.df, h, asal, 100_000)
            assert len(idx) and panel.df.iloc[idx]["tanggal"].max() + pd.Timedelta(days=h) <= asal

    def test_label_tidak_memakai_titik_sebelum_t(self):
        """Akhir pekan: label H+1 dari Jumat tidak boleh memakai harga Jumat itu sendiri."""
        harga = _harga(hari=60)
        panel = bangun_panel(_data(harga), AKHIR)
        seri = panel.seri[("BRS-MED", "1171")]
        pos = int(np.flatnonzero(seri.tanggal.dayofweek == 4)[3])
        assert _aktual(seri, pos, 1) is None  # Sabtu: titik terakhir tetap Jumat (t itu sendiri)

    def test_lipatan_berembargo(self):
        t = np.array(pd.date_range("2025-01-01", periods=120, freq="7D").values)
        y = np.arange(120) % 2
        lipatan = lipatan_waktu(t, 3, 30, y)
        assert len(lipatan) == 3
        for latih, uji in lipatan:
            assert t[latih].max() < t[uji].min() - np.timedelta64(30, "D")

    def test_walk_forward_asal_maju(self):
        lipatan = lipatan_asal(AKHIR)
        semua = [t for blok in lipatan for t in blok]
        assert semua == sorted(semua) and len(lipatan) == N_LIPATAN_SELEKSI + N_LIPATAN_HOLDOUT
        assert lipatan[0][0] == AKHIR - pd.Timedelta(days=150) and lipatan[-1][0] == AKHIR - pd.Timedelta(days=30)
        assert all(len(b) == N_ASAL_PER_LIPATAN for b in lipatan) and lipatan[0][1] - lipatan[0][0] == pd.Timedelta(days=JARAK_ASAL_HARI)

    def test_target_lipatan_tidak_melewati_lipatan_berikutnya(self):
        harga = _harga(hari=420, komoditas=("CBM-KRT",))
        from garda_ml.pipeline.regresi import _cv_statistik
        panel = bangun_panel(_data(harga), AKHIR)
        lipatan = lipatan_asal(AKHIR - pd.Timedelta(days=1))
        _, hasil = _cv_statistik(panel.seri[("CBM-KRT", "1171")], lipatan)
        for (i, t, h) in hasil["naive"]:
            assert t in lipatan[i]
            if i + 1 < len(lipatan):
                assert t + pd.Timedelta(days=h) <= lipatan[i + 1][0]
        assert {h for (_, _, h) in hasil["naive"]} == {7, 14, 30}

    def test_sarimax_lipatan_tanpa_melihat_masa_depan(self):
        from garda_ml.pipeline.statistik import ramal_lipatan
        y = np.log(15000) + np.cumsum(np.random.default_rng(4).normal(0, 0.005, 400))
        a = ramal_lipatan("sarimax", y, [300, 307], [7, 14, 30])
        z = y.copy()
        z[308:] += 1.0  # masa depan diubah setelah asal terakhir
        b = ramal_lipatan("sarimax", z, [300, 307], [7, 14, 30])
        np.testing.assert_allclose(a[0], b[0])
        np.testing.assert_allclose(a[1], b[1])

    def test_skor_seimbang_antar_horizon(self):
        a = np.log(100.0)
        nilai = {("x", 7): (np.log(110.0), a), ("y", 7): (np.log(110.0), a), ("x", 30): (np.log(130.0), a)}
        assert skor_seimbang(nilai, list(nilai))["mape"] == pytest.approx(20.0)


class TestPenggabungan:
    @staticmethod
    def _baris(sumber, nilai, t="2026-05-04", status=None, k="CBM-KRT", w="1171", crawl="2026-05-05", kunci=None):
        return {"sumber": sumber, "dataset": "d", "kunci_seri": kunci or f"{k}:{w}:{sumber}:{status}", "komoditas_kode": k, "kabupaten_kode": w, "tanggal": pd.Timestamp(t), "nilai": nilai, "satuan": "Rp/kg", "status_validasi": status, "crawled_at": pd.Timestamp(crawl)}

    def test_bobot_kontrak(self):
        assert BOBOT_LAPANGAN == {"DIPUBLIKASI": 1.0, "DIVALIDASI": 0.8, "DIAJUKAN": 0.3}

    def test_resmi_tetap_target_lapangan_jadi_fitur_berbobot(self):
        d = pd.DataFrame([self._baris("PIHPS", 100.0), self._baris("LAPANGAN", 200.0, status="DIPUBLIKASI"), self._baris("LAPANGAN", 100.0, status="DIAJUKAN")])
        h = gabung_harga(d).iloc[0]
        assert h["nilai"] == 100 and h["bobot"] == 1.0
        assert h["nilai_lapangan"] == pytest.approx((200 * 1.0 + 100 * 0.3) / 1.3)

    @pytest.mark.parametrize("status,bobot", [("DIPUBLIKASI", 1.0), ("DIVALIDASI", 0.8), ("DIAJUKAN", 0.3)])
    def test_seri_hanya_lapangan_memakai_bobot_status(self, status, bobot):
        h = gabung_harga(pd.DataFrame([self._baris("LAPANGAN", 150.0, status=status)])).iloc[0]
        assert h["nilai"] == 150 and h["bobot"] == bobot and h["sumber"] == "LAPANGAN"

    @pytest.mark.parametrize("status", ["DITOLAK", None, "REVISI"])
    def test_status_lain_tidak_dipakai(self, status):
        assert gabung_harga(pd.DataFrame([self._baris("LAPANGAN", 150.0, status=status)])).empty

    def test_pihps_diutamakan_tanpa_mencampur_sumber(self):
        d = pd.DataFrame([self._baris("SP2KP", 90.0), self._baris("PIHPS", 100.0), self._baris("SP2KP", 91.0, t="2026-05-05")])
        h = gabung_harga(d)
        assert list(h["nilai"]) == [100.0] and set(h["sumber"]) == {"PIHPS"}

    def test_crawl_ulang_nilai_terbaru_menang(self):
        d = pd.DataFrame([self._baris("PIHPS", 100.0, crawl="2026-05-05"), self._baris("PIHPS", 105.0, crawl="2026-05-06")])
        assert list(gabung_harga(d)["nilai"]) == [105.0]

    def test_nilai_tidak_positif_dibuang(self):
        assert gabung_harga(pd.DataFrame([self._baris("PIHPS", 0.0), self._baris("PIHPS", -5.0, t="2026-05-05")])).empty

    def test_lapangan_selain_konsumen_tidak_dicampur(self):
        konsumen = {**self._baris("LAPANGAN", 150.0, status="DIPUBLIKASI", kunci="a:E1"), "tipe": "KONSUMEN"}
        produsen = {**self._baris("LAPANGAN", 90.0, status="DIPUBLIKASI", kunci="a:E2"), "tipe": "PRODUSEN"}
        assert list(gabung_harga(pd.DataFrame([konsumen, produsen]))["nilai"]) == [150.0]

    def test_banyak_entri_lapangan_per_hari_dirata_berbobot(self):
        d = pd.DataFrame([self._baris("LAPANGAN", 100.0, status="DIPUBLIKASI", kunci="a:E1"), self._baris("LAPANGAN", 200.0, status="DIAJUKAN", kunci="a:E2")])
        h = gabung_harga(d).iloc[0]
        assert h["nilai"] == pytest.approx((100 + 200 * 0.3) / 1.3) and h["bobot"] == 1.0

    def test_domain_makro(self):
        assert domain_makro("raw_bps_makro", "inflasi_yoy") == "INFLASI"
        assert domain_makro("raw_sektor_primer", "produksi_ikan_tangkap") == "PERIKANAN"
        assert domain_makro("raw_sektor_primer", "produksi kayu bulat") == "KEHUTANAN"
        assert domain_makro("raw_tenaga_kerja", "apa saja") == "TENAGA_KERJA"


class TestKalender:
    @pytest.mark.parametrize("tabel,bulan,hari", [(kalender.IDULFITRI, 10, 1), (kalender.IDULADHA, 12, 10), (kalender.AWAL_RAMADAN, 9, 1)])
    def test_tabel_konsisten_dengan_hijriah_tabular(self, tabel, bulan, hari):
        tabular = [kalender.hijriah_ke_masehi(th, bulan, hari) for th in range(1430, 1460)]
        for t in tabel:
            assert min(abs((t - x).days) for x in tabular) <= 2, t

    def test_tanggal_dikenal(self):
        assert kalender.tanggal_idulfitri(2024) == date(2024, 4, 10)
        assert kalender.tanggal_idulfitri(2025) == date(2025, 3, 31)

    def test_ramadan_dan_jarak(self):
        f = kalender.fitur_kalender(pd.DatetimeIndex(["2024-03-11", "2024-03-12", "2024-04-09", "2024-04-10", "2024-12-20"]))
        assert list(f["kal_is_ramadan"]) == [0, 1, 1, 0, 0]
        assert f["kal_hari_ke_idulfitri"].iloc[2] == 1 and f["kal_hari_ke_idulfitri"].iloc[3] == 0
        assert f["kal_hari_ke_natal"].iloc[4] == 5
        assert f["kal_hari_ke_tahun_baru"].iloc[4] == 12


class TestRegresi:
    @staticmethod
    def _lipatan(*mape: float) -> dict:
        return {i: {"mape": float(m), "rmse": float(m)} for i, m in enumerate(mape)}

    def test_konstanta_aturan_seleksi(self):
        assert (MARGIN_BASELINE, N_LIPATAN_SELEKSI, N_LIPATAN_HOLDOUT, MIN_LIPATAN_SELEKSI, LANGKAH_LIPATAN_HARI) == (0.05, 4, 1, 2, 30)
        assert (N_ASAL_PER_LIPATAN, JARAK_ASAL_HARI) == (4, 7)

    def test_harus_unggul_minimal_margin(self):
        dasar = self._lipatan(5, 5, 5, 5)
        assert pilih_pemenang({"seasonal_naive": dasar, "sarimax": self._lipatan(4.8, 4.8, 4.8, 4.8)}) == "seasonal_naive"  # unggul 4%
        assert pilih_pemenang({"seasonal_naive": dasar, "sarimax": self._lipatan(4.7, 4.7, 4.7, 4.7)}) == "sarimax"  # unggul 6%

    def test_harus_unggul_di_setiap_lipatan(self):
        dasar = self._lipatan(5, 5, 5, 5)
        # Rata-ratanya jauh lebih baik, tetapi kalah pada satu lipatan.
        assert pilih_pemenang({"naive": dasar, "sarimax": self._lipatan(1, 1, 1, 5.1)}) == "naive"
        assert pilih_pemenang({"naive": dasar, "sarimax": self._lipatan(1, 1, 1, 5.0)}) == "naive"  # seri bukan unggul
        assert pilih_pemenang({"naive": dasar, "sarimax": self._lipatan(1, 1, 1, 4.9)}) == "sarimax"

    def test_kurang_dari_minimum_lipatan_memakai_baseline(self):
        assert pilih_pemenang({"naive": self._lipatan(5), "sarimax": self._lipatan(1)}) == "naive"
        assert pilih_pemenang({"naive": self._lipatan(5, 5), "sarimax": self._lipatan(1, 1)}) == "sarimax"

    def test_kandidat_terbaik_di_antara_yang_lolos(self):
        s = {"naive": self._lipatan(5, 5, 5, 5), "holt_winters": self._lipatan(4, 4, 4, 4), "sarimax": self._lipatan(3, 3, 3, 3), "lightgbm_quantile": self._lipatan(1, 1, 1, 6)}
        assert pilih_pemenang(s) == "sarimax"

    @pytest.mark.parametrize("n,harap", [(10, ["naive", "seasonal_naive"]), (40, ["naive", "seasonal_naive", "holt_winters"]), (120, ["naive", "seasonal_naive", "holt_winters", "sarimax"])])
    def test_kandidat_menurut_panjang(self, n, harap):
        assert kandidat_statistik(n) == harap

    @pytest.mark.parametrize("nama", list(RAMALAN))
    def test_kuantil_terurut_dan_positif(self, nama):
        y = np.log(15000) + np.cumsum(np.random.default_rng(2).normal(0, 0.005, 150))
        p = rapikan_kuantil(RAMALAN[nama](y, [7, 14, 30]))
        assert (np.diff(p, axis=1) >= 0).all() and (np.exp(p) > 0).all()

    def test_rapikan_persilangan(self):
        assert rapikan_kuantil(np.array([[3.0, 1.0, 2.0]])).tolist() == [[1.0, 2.0, 3.0]]

    def test_seri_pendek_fallback_dan_terlalu_pendek_dilewati(self):
        harga = pd.concat([_harga(hari=300, komoditas=("CBM-KRT",)), _harga(hari=16, komoditas=("BRS-MED",), seed=3), _harga(hari=4, komoditas=("DAG-AYM",), seed=4)], ignore_index=True)
        hasil = jalankan_regresi(bangun_panel(_data(harga), AKHIR), HGB, CFG)
        per = {h.seri.komoditas_kode: h for h in hasil.hasil}
        assert "DAG-AYM" not in per and [d["komoditas_kode"] for d in hasil.dilewati] == ["DAG-AYM"]
        pendek = per["BRS-MED"]
        assert pendek.nama_model == "naive" and pendek.fallback and "riwayat" in pendek.alasan
        for h in hasil.hasil:
            assert h.kuantil.shape == (3, 3) and (h.kuantil > 0).all() and (np.diff(h.kuantil, axis=1) >= 0).all()

    def test_kompetisi_mencatat_baseline(self):
        hasil = jalankan_regresi(bangun_panel(_data(_harga(hari=420)), AKHIR), HGB, CFG)
        assert len(hasil.hasil) == 2
        for h in hasil.hasil:
            assert h.metrik["mape_baseline"] is not None and h.metrik["n_lipatan"] == 5 and h.metrik["n_lipatan_seleksi"] == 4 and h.metrik["horizon"] == [7, 14, 30]
            assert h.metrik["baseline"] in BASELINES and h.metrik["evaluasi"] == "holdout_lipatan_terakhir"
            assert h.nama_model not in BASELINES or h.metrik["mengalahkan_baseline"] is False

    def test_data_basi_digeser_ke_cutoff(self):
        harga = _harga(hari=200, komoditas=("CBM-KRT",))
        panel = bangun_panel(_data(harga[harga["tanggal"] <= AKHIR - pd.Timedelta(days=40)]), AKHIR)
        h = jalankan_regresi(panel, HGB, CFG).hasil[0]
        assert h.nama_model in RAMALAN and "40 hari" in h.alasan  # terlalu basi untuk model global

    def test_metrik_dinilai_pada_lipatan_yang_tidak_ikut_memilih(self):
        t = [pd.Timestamp("2026-03-01"), pd.Timestamp("2026-04-01"), pd.Timestamp("2026-05-01")]
        a = np.log(100.0)
        cv = {
            "seasonal_naive": {(x, 7): (np.log(105.0), a) for x in t},
            "sarimax": {(t[0], 7): (np.log(101.0), a), (t[1], 7): (np.log(101.0), a), (t[2], 7): (np.log(120.0), a)},
        }
        h = _nilai_seri(cv)
        m = h["metrik"]
        assert h["nama"] == "sarimax" and m["n_lipatan"] == 3 and m["n_lipatan_seleksi"] == 2 and m["evaluasi"] == "holdout_lipatan_terakhir"
        assert m["mape"] == pytest.approx(20.0) and m["mape_baseline"] == pytest.approx(5.0)
        assert m["baseline"] == "seasonal_naive" and m["mengalahkan_baseline"] is False

    def test_baseline_terbaik_antara_naive_dan_musiman(self):
        s = {"naive": self._lipatan(2, 2, 2, 2), "seasonal_naive": self._lipatan(3, 3, 3, 3), "sarimax": self._lipatan(2.5, 2.5, 2.5, 2.5)}
        assert baseline_terbaik(s) == "naive"
        # SARIMAX mengalahkan seasonal_naive tetapi tidak naive: baseline yang dipublikasikan.
        assert pilih_pemenang(s) == "naive"
        s["sarimax"] = self._lipatan(1.5, 1.5, 1.5, 1.5)
        assert pilih_pemenang(s) == "sarimax"

    @staticmethod
    def _cv(nilai: dict[str, tuple[float, float, float]]) -> dict:
        t = [pd.Timestamp("2026-03-01"), pd.Timestamp("2026-04-01"), pd.Timestamp("2026-05-01")]
        a = np.log(100.0)
        return {n: {(x, 7): (np.log(100.0 + e), a) for x, e in zip(t, v)} for n, v in nilai.items()}

    @staticmethod
    def _cv_n(n: int, kandidat_e: float = 1.0, holdout_e: float = 1.0) -> dict:
        t = [pd.Timestamp("2026-01-01") + pd.Timedelta(days=30 * i) for i in range(n)]
        a = np.log(100.0)
        return {
            "naive": {(x, 7): (np.log(104.0), a) for x in t},
            "seasonal_naive": {(x, 7): (np.log(105.0), a) for x in t},
            "sarimax": {(x, 7): (np.log(100.0 + (holdout_e if i == n - 1 else kandidat_e)), a) for i, x in enumerate(t)},
        }

    def test_lima_lipatan_empat_seleksi_satu_holdout(self):
        h = _nilai_seri(self._cv_n(5))
        assert h["nama"] == "sarimax" and h["metrik"]["n_lipatan"] == 5 and h["metrik"]["n_lipatan_seleksi"] == 4

    def test_holdout_tidak_memengaruhi_pemilihan(self):
        h = _nilai_seri(self._cv_n(5, holdout_e=50.0))
        assert h["nama"] == "sarimax" and h["metrik"]["mengalahkan_baseline"] is False
        assert h["metrik"]["mape"] == pytest.approx(50.0) and h["metrik"]["mape_baseline"] == pytest.approx(4.0)

    def test_lipatan_berlebih_hanya_empat_terbaru_untuk_seleksi(self):
        cv = self._cv_n(7)
        awal = pd.Timestamp("2026-01-01")
        cv["sarimax"][(awal, 7)] = (np.log(190.0), np.log(100.0))  # lipatan tertua di luar empat terbaru
        h = _nilai_seri(cv)
        assert h["nama"] == "sarimax" and h["metrik"]["n_lipatan_seleksi"] == 4

    @pytest.mark.parametrize("n,harap,seleksi", [(1, "naive", 1), (2, "naive", 1), (3, "sarimax", 2), (4, "sarimax", 3)])
    def test_riwayat_pendek_dan_batas_minimum(self, n, harap, seleksi):
        h = _nilai_seri(self._cv_n(n))
        assert h["nama"] == harap and h["metrik"]["n_lipatan_seleksi"] == seleksi

    def test_kandidat_riwayat_pendek_dinilai_pada_lipatannya(self):
        cv = self._cv_n(5)
        for k in sorted(cv["sarimax"])[:2]:
            del cv["sarimax"][k]  # hanya punya 2 lipatan seleksi + holdout
        assert _nilai_seri(cv)["nama"] == "sarimax"
        del cv["sarimax"][sorted(cv["sarimax"])[0]]  # tinggal 1 lipatan seleksi
        assert _nilai_seri(cv)["nama"] == "naive"

    def test_tanpa_kandidat_unggul_baseline_dipublikasikan(self):
        h = _nilai_seri(self._cv({"naive": (2, 2, 2), "seasonal_naive": (4, 4, 4), "holt_winters": (3, 3, 1)}))
        assert h["nama"] == "naive" and h["metrik"]["baseline"] == "naive"
        assert h["metrik"]["mape"] == h["metrik"]["mape_baseline"] and h["metrik"]["mengalahkan_baseline"] is False

    def test_kandidat_unggul_pada_holdout_dihitung(self):
        h = _nilai_seri(self._cv({"naive": (4, 4, 4), "seasonal_naive": (5, 5, 5), "sarimax": (1, 1, 2)}))
        assert h["nama"] == "sarimax" and h["metrik"]["baseline"] == "naive" and h["metrik"]["mengalahkan_baseline"] is True

    def test_kandidat_tanpa_semua_lipatan_tidak_bersaing(self):
        cv = self._cv({"naive": (4, 4, 4), "seasonal_naive": (5, 5, 5), "sarimax": (1, 1, 1)})
        del cv["sarimax"][next(iter(cv["sarimax"]))]
        assert _nilai_seri(cv)["nama"] == "naive"

    def test_satu_lipatan_tidak_mengklaim_unggul(self):
        cv = {n: dict(list(v.items())[:1]) for n, v in self._cv({"naive": (4, 4, 4), "seasonal_naive": (5, 5, 5), "sarimax": (1, 1, 1)}).items()}
        m = _nilai_seri(cv)["metrik"]
        assert m["evaluasi"] == "tanpa_holdout" and m["mengalahkan_baseline"] is None

    def test_seri_terlalu_basi_tidak_diramal(self):
        harga = pd.concat([_harga(hari=300, komoditas=("CBM-KRT",)), _harga(hari=300, komoditas=("CBR-RWT",), seed=5, akhir=AKHIR - pd.Timedelta(days=60))], ignore_index=True)
        hasil = jalankan_regresi(bangun_panel(_data(harga), AKHIR), HGB, CFG)
        assert [h.seri.komoditas_kode for h in hasil.hasil] == ["CBM-KRT"]
        assert hasil.dilewati[0]["komoditas_kode"] == "CBR-RWT" and "hari lalu" in hasil.dilewati[0]["alasan"]

    def test_nama_backend_jujur(self, monkeypatch):
        monkeypatch.setenv("ML_PAKSA_HGB", "1")
        b = pilih_backend()
        assert b.nama == "hist_gradient_boosting" and b.nama_kuantil == "hgb_quantile" and b.fallback


class TestKlasifikasi:
    def test_skor_titik_tengah(self):
        assert skor_tekanan(0.7, 0.7) == pytest.approx(50)
        assert skor_tekanan(1.0, 0.7) == 100 and skor_tekanan(0.0, 0.7) == 0

    def test_siklus(self):
        assert tentukan_siklus(70, 0.1) == "TEKANAN"
        assert tentukan_siklus(30, -0.01) == "SURPLUS"
        assert tentukan_siklus(30, 0.01) == "NORMAL"
        assert tentukan_siklus(50, None) == "NORMAL"

    def test_tanpa_bps_memakai_indeks_pangan(self):
        harga = _harga(hari=500, wilayah=("1171", "1174"))
        hasil = jalankan_klasifikasi(bangun_panel(_data(harga), AKHIR), _data(harga), HGB)
        assert hasil.target_siklus == "INDEKS_PANGAN" and hasil.sinyal
        assert {s["target"] for s in hasil.sinyal} == {"INDEKS_PANGAN"}
        # Provinsi tidak diekstrapolasi dari sebagian kab/kota bila tanpa label resmi.
        assert {s["wilayah_kode"] for s in hasil.sinyal} == {"1171", "1174"}
        for s in hasil.sinyal:
            assert 0 <= s["probabilitas"] <= 1 and s["penyumbang"]
            assert s["skor_tekanan"] is None if not s["layak"] else 0 <= s["skor_tekanan"] <= 100

    @pytest.mark.parametrize("indikator,jenis", [("inflasi_month_to_month", "mtm"), ("inflasi_year_on_year", "yoy"), ("inflasi_year_to_date", "ytd"), ("indeks_harga_konsumen", "ihk")])
    def test_nama_indikator_crawler_dikenali(self, indikator, jenis):
        periode = pd.date_range("2025-01-01", periods=3, freq="MS")
        m = pd.DataFrame({"domain": "INFLASI", "indikator": indikator, "wilayah_kode": "11", "periode": periode, "tanggal_rilis": periode + pd.Timedelta(days=40), "nilai": [100.0, 101.0, 102.0]})
        hasil = seri_inflasi_resmi(m)
        if jenis in ("mtm", "ihk"):
            assert hasil["11"]["mtm"].notna().any()
        else:
            assert not hasil  # tanpa mtm atau IHK tidak ada label

    def test_bps_cukup_memakai_resmi(self):
        harga = _harga(hari=900, wilayah=("1171", "1174"))
        periode = pd.date_range("2024-02-01", "2026-04-01", freq="MS")
        rng = np.random.default_rng(0)
        makro = pd.DataFrame({"domain": "INFLASI", "indikator": "inflasi_mtm", "wilayah_kode": "1171", "periode": periode, "nilai": rng.normal(0.1, 0.4, len(periode)), "satuan": "persen", "sumber": "BPS", "frekuensi": "BULANAN", "is_carried_forward": False, "last_available_date": periode})
        makro["tanggal_rilis"] = tanggal_rilis_konservatif(makro["periode"], makro["frekuensi"])
        data = _data(harga, makro)
        hasil = jalankan_klasifikasi(bangun_panel(data, AKHIR), data, HGB)
        per = {s["wilayah_kode"]: s for s in hasil.sinyal}
        assert per["1171"]["target"] == "RESMI_BPS" and per["1171"]["aktual"]["mtm"] is not None
        assert per["1174"]["target"] == "INDEKS_PANGAN" and "11" not in per  # tanpa seri resmi tidak boleh mengaku resmi
        assert hasil.target_siklus == "INDEKS_PANGAN"  # campuran tidak boleh diklaim resmi


class TestKelayakan:
    @pytest.mark.parametrize("auc,brier,acuan,harap", [
        (0.65, 0.20, 0.25, True), (MIN_AUC_LAYAK, 0.20, 0.25, True), (0.59, 0.20, 0.25, False),
        (0.80, 0.25, 0.25, False), (0.80, 0.26, 0.25, False), (None, 0.2, 0.25, False), (0.8, 0.2, None, False),
    ])
    def test_aturan(self, auc, brier, acuan, harap):
        layak, alasan = nilai_kelayakan({"roc_auc": auc, "brier": brier, "brier_baseline": acuan})
        assert layak is harap and alasan

    @pytest.mark.parametrize("backend", BACKEND_UJI, ids=lambda b: b.nama)
    def test_tanpa_sinyal_tidak_layak_dan_kelas_null(self, backend):
        harga = _harga_acak()
        data = _data(harga)
        hasil = jalankan_klasifikasi(bangun_panel(data, AKHIR), data, backend)
        m = hasil.metrik
        assert m["layak"] is False and m["alasan"] and m["brier_baseline"] is not None and m["n_uji"] > 100
        assert hasil.sinyal and all(s["kelas"] is None and s["layak"] is False and s["alasan"] for s in hasil.sinyal)
        assert all(s["skor_tekanan"] is None and s["siklus"] is None and 0 <= s["probabilitas"] <= 1 for s in hasil.sinyal)
        assert jelaskan_inflasi(hasil) == ([], [])
        assert sinyal_tekanan(hasil.sinyal) == []

    @pytest.mark.parametrize("backend", BACKEND_UJI, ids=lambda b: b.nama)
    def test_sinyal_kuat_layak_dan_xai_menjumlah_ke_selisih_probabilitas(self, backend):
        harga = _harga_musiman()
        data = _data(harga)
        hasil = jalankan_klasifikasi(bangun_panel(data, AKHIR), data, backend)
        assert hasil.metrik["layak"] is True and hasil.metrik["roc_auc"] >= MIN_AUC_LAYAK
        assert hasil.metrik["brier"] < hasil.metrik["brier_baseline"]
        assert all(s["layak"] and s["kelas"] in ("INFLASI", "TIDAK_INFLASI") for s in hasil.sinyal)
        dok, gagal = jelaskan_inflasi(hasil)
        assert not gagal and len(dok) == len(hasil.sinyal)
        per = {s["wilayah_kode"]: s for s in hasil.sinyal}
        for x in dok:
            assert x["satuan_kontribusi"] == "poin_persen_probabilitas"
            assert x["probabilitas"] == pytest.approx(per[x["wilayah_kode"]]["probabilitas"], abs=1e-6)
            jumlah = sum(f["kontribusi"] for f in x["faktor"])
            assert jumlah == pytest.approx((x["probabilitas"] - x["probabilitas_dasar"]) * 100, abs=1e-2)

    @pytest.mark.parametrize("backend", BACKEND_UJI, ids=lambda b: b.nama)
    @pytest.mark.parametrize("metode", ["sigmoid", "isotonic", "tanpa_kalibrasi"])
    def test_kontribusi_tepat_pada_probabilitas_terkalibrasi(self, backend, metode):
        import warnings
        from sklearn.calibration import CalibratedClassifierCV
        rng = np.random.default_rng(0)
        X = rng.normal(size=(800, 6)).astype(np.float32)
        X[::9, 4] = np.nan
        y = (X[:, 0] + 0.7 * X[:, 1] + rng.normal(0, 0.8, 800) > 0).astype(int)
        with warnings.catch_warnings():
            warnings.simplefilter("ignore")
            model = backend.klasifikator().fit(X, y) if metode == "tanpa_kalibrasi" else CalibratedClassifierCV(backend.klasifikator(), method=metode, cv=3).fit(X, y)
        c, p, p0 = kontribusi_probabilitas(model, X[:25])
        np.testing.assert_allclose(p, model.predict_proba(X[:25])[:, 1], atol=1e-6)
        np.testing.assert_allclose(c.sum(axis=1), p - p0, atol=1e-9)
        if metode == "tanpa_kalibrasi":
            # Penskalaan tidak membalik arah: tanda kontribusi sama dengan tanda SHAP log-odds.
            from garda_ml.pipeline.xai import nilai_shap
            phi = nilai_shap(model, X[:25])
            besar = np.abs(c) > 1e-9
            assert np.all(np.sign(c[besar]) == np.sign(phi[besar]))


class TestXai:
    @pytest.mark.parametrize("fitur,harap", [("cuaca_hujan_7", "CUACA"), ("kal_is_ramadan", "KALENDER"), ("tekanan_pasokan", "PASOKAN"), ("makro_PERIKANAN_x", "PASOKAN"), ("biaya_tenaga_kerja", "TENAGA_KERJA"), ("makro_KONSUMSI_PENDAPATAN_x", "KONSUMSI_PENDAPATAN"), ("makro_NERACA_EKONOMI_x", "NERACA_EKONOMI"), ("lag_7", "TREN_HARGA_LAPANGAN"), ("komoditas_id", None)])
    def test_pemetaan(self, fitur, harap):
        assert kelompok_fitur(fitur) == harap

    def test_faktor_terurut(self):
        f = faktor_dari_nilai(["lag_7", "cuaca_hujan_7", "kal_bulan", "wilayah_id"], np.array([0.01, -0.05, 0.002, 9.0]))
        assert [x["kelompok"] for x in f] == ["CUACA", "TREN_HARGA_LAPANGAN", "KALENDER"] and f[0]["arah"] == "TURUN"

    def test_kelompok_tanpa_data_tidak_muncul(self):
        f = faktor_dari_nilai(["lag_7", "cuaca_hujan_7"], np.array([0.01, 0.5]), masukan=np.array([0.2, np.nan]))
        assert [x["kelompok"] for x in f] == ["TREN_HARGA_LAPANGAN"]

    def test_dekomposisi_menjumlah_ke_proyeksi(self):
        y = np.log(10000) + np.cumsum(np.random.default_rng(1).normal(0, 0.01, 100))
        _, nilai = dekomposisi_seri(y, 0.05)
        assert nilai.sum() == pytest.approx(0.05)


class TestAnomali:
    def _panel_lonjakan(self, faktor=1.6, bobot=1.0):
        harga = _harga(hari=300, komoditas=("CBM-KRT", "BRS-MED"))
        terakhir = harga["tanggal"].max()
        maska = (harga["komoditas_kode"] == "CBM-KRT") & (harga["tanggal"] == terakhir)
        harga.loc[maska, "nilai"] *= faktor
        harga.loc[maska, "bobot"] = bobot
        return bangun_panel(_data(harga), AKHIR)

    def test_lonjakan_besar_kritis(self):
        sinyal = deteksi_anomali(self._panel_lonjakan().seri, {})
        kritis = [s for s in sinyal if s["severity"] == "KRITIS"]
        assert [s["sinyal_kunci"] for s in kritis] == ["ANOMALI_HARGA:CBM-KRT:1171:NAIK"]

    def test_kunci_stabil_lintas_siklus(self):
        a = deteksi_anomali(self._panel_lonjakan(1.6).seri, {})
        b = deteksi_anomali(self._panel_lonjakan(1.8).seri, {})
        assert {s["sinyal_kunci"] for s in a} == {s["sinyal_kunci"] for s in b}

    def test_input_diajukan_tidak_memicu_alarm(self):
        assert not [s for s in deteksi_anomali(self._panel_lonjakan(bobot=0.3).seri, {}) if s["komoditas_kode"] == "CBM-KRT"]

    @pytest.mark.parametrize("z,dev,o,harap", [(4, 30, False, "KRITIS"), (7, 16, False, "KRITIS"), (4, 12, False, "SIAGA"), (3, 6, False, "WASPADA"), (2.1, 6, True, "WASPADA"), (2.1, 6, False, None), (9, 2, True, None)])
    def test_ambang(self, z, dev, o, harap):
        assert tingkat_anomali(z, dev, o) == harap

    def test_sumber_basi_dan_gagal(self):
        lad = datetime(2026, 5, 1, tzinfo=timezone.utc)
        kesegaran = [
            {"sumber": "PIHPS", "dataset": "harga", "status": "ERROR", "umur_hari": 2, "last_available_date": lad, "pesan": "timeout"},
            {"sumber": "OPEN_METEO", "dataset": "cuaca", "status": "STALE_CARRIED_FORWARD", "umur_hari": 10, "last_available_date": lad},
            {"sumber": "BPS", "dataset": "inflasi", "status": "STALE_CARRIED_FORWARD", "umur_hari": 40, "last_available_date": lad},
        ]
        s = {x["sinyal_kunci"]: x["severity"] for x in sinyal_sumber(kesegaran, True)}
        assert s["SUMBER_DATA:PIHPS:harga"] == "SIAGA" and s["SUMBER_DATA:OPEN_METEO:cuaca"] == "WASPADA"
        assert "SUMBER_DATA:BPS:inflasi" not in s  # kelambatan wajar data bulanan
        assert s["SUMBER_DATA:HARGA:SEMUA"] == "KRITIS"

    def test_sumber_sengaja_dilewati_bukan_alarm(self):
        kesegaran = [{"sumber": "SP2KP", "dataset": "harga_kemendag", "status": "ERROR", "umur_hari": None, "last_available_date": None, "pesan": "DILEWATI: butuh kunci resmi"}]
        s = {x["sinyal_kunci"]: x["severity"] for x in sinyal_sumber(kesegaran, True)}
        assert s["SUMBER_DATA:SP2KP:harga_kemendag"] == "INFO"

    @pytest.mark.parametrize("status,umur", [("ERROR", None), ("STALE_CARRIED_FORWARD", 400), ("ERROR", 400)])
    def test_dilewati_selalu_info_dengan_kunci_stabil(self, status, umur):
        k = [{"sumber": "BAPANAS", "dataset": "panel_harga", "status": status, "umur_hari": umur, "last_available_date": None, "pesan": "DILEWATI: butuh API key"}]
        s = next(x for x in sinyal_sumber(k, True) if x["sinyal_kunci"] == "SUMBER_DATA:BAPANAS:panel_harga")
        assert s["severity"] == "INFO" and s["aktif"] is True

    def test_kata_dilewati_di_tengah_pesan_tetap_kegagalan(self):
        k = [{"sumber": "PIHPS", "dataset": "harga", "status": "ERROR", "umur_hari": 1, "last_available_date": None, "pesan": "galat: baris DILEWATI: timeout"}]
        assert next(x for x in sinyal_sumber(k, True) if x["sinyal_kunci"] == "SUMBER_DATA:PIHPS:harga")["severity"] == "SIAGA"

    def test_sumber_harga_dilewati_tidak_dihitung_segar(self):
        k = [{"sumber": "SP2KP", "dataset": "harga_kemendag", "status": "STALE_CARRIED_FORWARD", "umur_hari": 1, "last_available_date": None, "pesan": "DILEWATI: token"}]
        assert {x["sinyal_kunci"]: x["severity"] for x in sinyal_sumber(k, True)}["SUMBER_DATA:HARGA:SEMUA"] == "KRITIS"

    def test_pulih_dicatat_sekali(self):
        lalu = [{"sinyal_kunci": "X", "jenis": "ANOMALI_HARGA", "severity": "KRITIS", "aktif": True}, {"sinyal_kunci": "Y", "jenis": "ANOMALI_HARGA", "severity": "SIAGA", "aktif": False}]
        p = tandai_pulih([], lalu)
        assert [(x["sinyal_kunci"], x["aktif"], x["severity"]) for x in p] == [("X", False, "INFO")]

    def test_proyeksi_kritis_butuh_p10_di_atas_harga(self):
        harga = _harga(hari=60, komoditas=("CBM-KRT",))
        seri = next(iter(bangun_panel(_data(harga), AKHIR).seri.values()))
        from garda_ml.pipeline.regresi import HasilRamalan
        h = seri.harga_terakhir
        yakin = HasilRamalan(seri, "x", False, "", {}, np.array([[h * 1.1, h * 1.25, h * 1.4]] * 3))
        ragu = HasilRamalan(seri, "x", False, "", {}, np.array([[h * 0.95, h * 1.25, h * 1.4]] * 3))
        assert sinyal_proyeksi([yakin], {})[0]["severity"] == "KRITIS"
        assert sinyal_proyeksi([ragu], {})[0]["severity"] == "SIAGA"


class TestWaktu:
    def test_id_siklus(self):
        assert id_siklus(datetime(2026, 3, 5, 7, 42, tzinfo=timezone.utc)) == "C20260305T07Z"

    def test_tanggal_wib_melewati_tengah_malam(self):
        assert tanggal_wib(datetime(2026, 3, 5, 18, 0, tzinfo=timezone.utc)) == date(2026, 3, 6)

    def test_detik_berikut(self):
        assert detik_ke_siklus_berikut(datetime(2026, 3, 5, 7, 0, tzinfo=timezone.utc), 5) == 300
        assert detik_ke_siklus_berikut(datetime(2026, 3, 5, 7, 5, tzinfo=timezone.utc), 5) == 3600

    def test_bersihkan_nan(self):
        d = bersihkan({"a": np.float32(1.5), "b": float("nan"), "c": [np.int64(2)], "t": pd.Timestamp("2026-01-02")})
        assert d["a"] == 1.5 and d["b"] is None and d["c"] == [2] and d["t"] == datetime(2026, 1, 2, tzinfo=timezone.utc)

    def test_konfigurasi_fallback_mongo_url(self):
        assert Konfigurasi.dari_env({"MONGO_URL": "mongodb://x"}).mongo_uri == "mongodb://x"
        with pytest.raises(ValueError):
            Konfigurasi.dari_env({})
