import math
import random
import unittest
from datetime import date, timedelta

from garda_ml import forecast as f


def seri(n=90, dasar=40000.0, tren=0.0, musim=None, derau=0.0, mulai=date(2026, 8, 1), seed=1):
    """Deret harian sintetis yang deterministik, dalam bentuk titik permintaan."""
    rng = random.Random(seed)
    musim = musim or [0.0] * 7
    return [
        {
            "tanggal": (mulai + timedelta(days=i)).isoformat(),
            "nilai": round(dasar + tren * i + musim[i % 7] + rng.gauss(0, derau), 2),
        }
        for i in range(n)
    ]


class TestRegularkan(unittest.TestCase):
    def test_mengisi_hari_bolong_dengan_interpolasi_linear(self):
        titik = [
            {"tanggal": "2026-08-01", "nilai": 100},
            {"tanggal": "2026-08-02", "nilai": 110},
            {"tanggal": "2026-08-04", "nilai": 130},
            {"tanggal": "2026-08-05", "nilai": 140},
            {"tanggal": "2026-08-06", "nilai": 150},
        ]
        awal, y = f.regularkan(titik)
        self.assertEqual(awal, date(2026, 8, 1))
        self.assertEqual([round(v, 6) for v in y], [100, 110, 120, 130, 140, 150])

    def test_mengurutkan_masukan_acak(self):
        titik = seri(10)
        acak = random.Random(3)
        acak.shuffle(titik)
        _, y = f.regularkan(titik)
        self.assertEqual(len(y), 10)

    def test_menolak_cakupan_rendah(self):
        titik = [{"tanggal": "2026-08-01", "nilai": 1}, {"tanggal": "2026-08-10", "nilai": 1}]
        with self.assertRaises(f.DataTidakCukup):
            f.regularkan(titik)

    def test_menolak_kosong_nilai_tak_positif_dan_tanggal_ganda(self):
        with self.assertRaises(f.DataTidakCukup):
            f.regularkan([])
        with self.assertRaises(ValueError):
            f.regularkan([{"tanggal": "2026-08-01", "nilai": 0}])
        with self.assertRaises(ValueError):
            f.regularkan([{"tanggal": "bukan-tanggal", "nilai": 5}])
        with self.assertRaises(ValueError):
            f.regularkan([{"tanggal": "2026-08-01", "nilai": 5}, {"tanggal": "2026-08-01", "nilai": 6}])


class TestRamal(unittest.TestCase):
    def test_naive_mempertahankan_harga_terakhir(self):
        j = f.ramal("naive", [10.0, 12.0, 11.0], {}, 3)
        self.assertEqual(j.nilai, [11.0] * 3)

    def test_seasonal_naive_mengulang_pekan_terakhir(self):
        y = [float(i) for i in range(1, 15)]  # 1..14
        j = f.ramal("seasonal_naive", y, {}, 9)
        self.assertEqual(j.nilai, [8, 9, 10, 11, 12, 13, 14, 8, 9])

    def test_seasonal_naive_butuh_satu_pekan(self):
        with self.assertRaises(f.DataTidakCukup):
            f.ramal("seasonal_naive", [1.0] * 5, {}, 3)

    def test_holt_winters_butuh_dua_pekan(self):
        with self.assertRaises(f.DataTidakCukup):
            f.ramal("holt_winters", [1.0] * 10, {"alpha": 0.5, "beta": 0.1, "gamma": 0.1, "phi": 0.9}, 3)

    def test_algoritma_tak_dikenal(self):
        with self.assertRaises(ValueError):
            f.ramal("arima", [1.0] * 30, {}, 3)

    def test_komponen_menjumlah_ke_perubahan_dari_harga_terakhir(self):
        _, y = f.regularkan(seri(60, tren=50, musim=[0, 300, -200, 100, 0, -300, 100], derau=100))
        for nama in f.KANDIDAT:
            j = f.ramal(nama, y, f.sesuaikan(nama, y), 14)
            for k in range(14):
                jumlah = sum(v[k] for v in j.komponen.values())
                self.assertAlmostEqual(y[-1] + jumlah, j.nilai[k], places=6, msg=nama)

    def test_holt_menangkap_tren_naik_linear(self):
        _, y = f.regularkan(seri(60, dasar=40000, tren=100))
        j = f.ramal("holt", y, f.sesuaikan("holt", y), 7)
        self.assertGreater(j.nilai[-1], y[-1])

    def test_holt_winters_menangkap_pola_mingguan(self):
        musim = [0, 2000, -1000, 500, 0, -1500, 0]
        _, y = f.regularkan(seri(70, musim=musim))
        j = f.ramal("holt_winters", y, f.sesuaikan("holt_winters", y), 7)
        for k in range(7):
            # Hari ke-(n+k) mengikuti pola mingguan yang sama dengan hari ke-(n+k-7).
            self.assertAlmostEqual(j.nilai[k], y[len(y) - 7 + k], delta=300)


class TestMetrik(unittest.TestCase):
    def test_mape_dan_rmse(self):
        self.assertAlmostEqual(f.mape([100, 200], [110, 180]), 10.0)
        self.assertAlmostEqual(f.rmse([100, 200], [110, 180]), math.sqrt(250))

    def test_menolak_panjang_berbeda_atau_kosong(self):
        with self.assertRaises(ValueError):
            f.mape([1], [1, 2])
        with self.assertRaises(ValueError):
            f.rmse([], [])

    def test_titik_asal_walk_forward(self):
        self.assertEqual(f.titik_asal(90, 14), [28, 35, 42, 49, 56, 63, 70])
        self.assertEqual(f.titik_asal(42, 14), [28])
        self.assertEqual(f.titik_asal(30, 14), [])


class TestFaktor(unittest.TestCase):
    def test_jumlah_kontribusi_sama_dengan_perubahan_persen(self):
        _, y = f.regularkan(seri(60, tren=40, musim=[0, 300, -200, 100, 0, -300, 100], derau=80))
        for nama in f.KANDIDAT:
            j = f.ramal(nama, y, f.sesuaikan(nama, y), 14)
            for k in (7, 14):
                total = round((j.nilai[k - 1] - y[-1]) / y[-1] * 100, 1)
                jumlah = round(sum(x["kontribusi"] for x in f.faktor(j, k, y[-1])), 1)
                self.assertEqual(jumlah, total, nama)

    def test_naive_tanpa_perubahan_struktural(self):
        j = f.ramal("naive", [100.0] * 10, {}, 7)
        self.assertEqual(f.faktor(j, 7, 100.0), [
            {"nama": "Harga terakhir dipertahankan", "kontribusi": 0.0, "detail": f.LABEL_FAKTOR["tetap"][1]},
        ])


class TestLatihSeri(unittest.TestCase):
    def test_seri_pendek_dilewati(self):
        with self.assertRaises(f.DataTidakCukup):
            f.latih_seri(seri(30), [7, 14])

    def test_horizon_tidak_valid(self):
        with self.assertRaises(ValueError):
            f.latih_seri(seri(90), [])
        with self.assertRaises(ValueError):
            f.latih_seri(seri(90), [0, 7])

    def test_hasil_lengkap_dan_konsisten(self):
        hasil = f.latih_seri(seri(90, tren=30, musim=[0, 400, -300, 200, 0, -400, 100], derau=150), [7, 14, 30])
        self.assertIn(hasil["algoritma"], f.KANDIDAT)
        self.assertEqual(hasil["tanggal_dasar"], "2026-10-29")
        self.assertEqual([r["horizon_hari"] for r in hasil["ramalan"]], [7, 14, 30])
        for r in hasil["ramalan"]:
            self.assertLessEqual(r["bawah"], r["nilai"])
            self.assertLessEqual(r["nilai"], r["atas"])
            self.assertGreaterEqual(r["bawah"], 1)
        # Tanggal target = tanggal dasar + horizon.
        self.assertEqual(hasil["ramalan"][1]["tanggal_target"], "2026-11-12")
        m = hasil["metrik"]
        self.assertGreater(m["mape"], 0)
        self.assertGreater(m["rmse"], 0)
        self.assertEqual(set(m["mape_per_horizon"]), {"7", "14", "30"})
        self.assertGreater(m["n_lipatan_uji"], 0)
        self.assertFalse(m["optimistis"])

    def test_deterministik(self):
        a = f.latih_seri(seri(90, derau=200), [7, 14])
        b = f.latih_seri(seri(90, derau=200), [7, 14])
        self.assertEqual(a, b)

    def test_model_terpilih_tidak_lebih_buruk_dari_baseline_pada_seleksi(self):
        # Syarat build: kandidat yang dipilih tidak boleh lebih buruk dari baseline naif.
        for tren, derau, seed in [(0, 100, 1), (80, 50, 2), (-60, 200, 3), (20, 600, 4)]:
            hasil = f.latih_seri(seri(90, tren=tren, derau=derau, seed=seed), [7, 14])
            skor = hasil["metrik"]["mape_seleksi"]
            self.assertLessEqual(skor[hasil["algoritma"]], skor[f.BASELINE] + 1e-9)

    def test_tren_kuat_mengalahkan_baseline_pada_lipatan_uji(self):
        hasil = f.latih_seri(seri(90, dasar=30000, tren=120, derau=30), [7, 14])
        self.assertIn(hasil["algoritma"], ("holt", "holt_winters"))
        self.assertLess(hasil["metrik"]["mape"], hasil["metrik"]["baseline_mape"])

    def test_riwayat_pendek_ditandai_optimistis(self):
        # 28 + 7 + 14 = 49 hari menghasilkan tepat dua lipatan: tidak bisa dipisah menjadi seleksi dan uji.
        hasil = f.latih_seri(seri(49, derau=100), [14])
        self.assertTrue(hasil["metrik"]["optimistis"])

    def test_interval_melebar_dengan_derau(self):
        tenang = f.latih_seri(seri(90, derau=20, seed=5), [14])["ramalan"][0]
        gaduh = f.latih_seri(seri(90, derau=900, seed=5), [14])["ramalan"][0]
        self.assertGreater(gaduh["atas"] - gaduh["bawah"], tenang["atas"] - tenang["bawah"])


class TestPrediksiSeri(unittest.TestCase):
    def test_sama_dengan_hasil_latih_pada_data_yang_sama(self):
        titik = seri(90, tren=30, derau=100)
        dilatih = f.latih_seri(titik, [7, 14])
        hasil = f.prediksi_seri(titik, dilatih["algoritma"], dilatih["params"], dilatih["galat_rmse"], [7, 14])
        self.assertEqual(hasil["ramalan"], dilatih["ramalan"])
        self.assertEqual(hasil["harga_dasar"], dilatih["harga_dasar"])

    def test_memakai_data_terbaru_tanpa_melatih_ulang(self):
        titik = seri(90, tren=30, derau=100)
        dilatih = f.latih_seri(titik, [7])
        lanjut = seri(97, tren=30, derau=100)
        hasil = f.prediksi_seri(lanjut, dilatih["algoritma"], dilatih["params"], dilatih["galat_rmse"], [7])
        self.assertEqual(hasil["tanggal_dasar"], "2026-11-05")

    def test_validasi_masukan(self):
        titik = seri(60)
        with self.assertRaises(ValueError):
            f.prediksi_seri(titik, "arima", {}, {"7": 1}, [7])
        with self.assertRaises(ValueError):
            f.prediksi_seri(titik, "naive", {}, {}, [7])
        with self.assertRaises(ValueError):
            f.prediksi_seri(titik, "holt", {"alpha": 0.5}, {"7": 1}, [7])


class TestRingkas(unittest.TestCase):
    def test_rata_rata_metrik_seri_ok(self):
        def ok(mape, algoritma):
            return {"status": "OK", "algoritma": algoritma, "metrik": {"mape": mape, "rmse": mape * 10, "baseline_mape": mape + 1, "baseline_rmse": mape * 10 + 5, "optimistis": False}}

        ringkasan = f.ringkas([ok(2.0, "holt"), ok(4.0, "naive"), {"status": "DILEWATI", "alasan": "x"}])
        self.assertEqual(ringkasan["n_seri_ok"], 2)
        self.assertEqual(ringkasan["n_seri_dilewati"], 1)
        self.assertAlmostEqual(ringkasan["mape"], 3.0)
        self.assertAlmostEqual(ringkasan["baseline_mape"], 4.0)
        self.assertEqual(ringkasan["algoritma"], {"naive": 1, "holt": 1})

    def test_semua_dilewati(self):
        self.assertEqual(f.ringkas([{"status": "DILEWATI"}]), {"n_seri_ok": 0, "n_seri_dilewati": 1})


if __name__ == "__main__":
    unittest.main()
