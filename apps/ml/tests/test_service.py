import json
import threading
import unittest
from http.client import HTTPConnection

from garda_ml import service
from garda_ml.server import buat_server

from .test_forecast import seri


class TestLatih(unittest.TestCase):
    def test_melatih_banyak_seri_dan_melewati_yang_pendek(self):
        hasil = service.latih(
            {
                "komoditas_kode": "CBM-KRT",
                "horizon": [7, 14],
                "seri": [{"id": "1171", "titik": seri(90, derau=100)}, {"id": "1101", "titik": seri(20)}],
            }
        )
        self.assertEqual(hasil["komoditas_kode"], "CBM-KRT")
        ok, lewat = hasil["seri"]
        self.assertEqual((ok["id"], ok["status"]), ("1171", "OK"))
        self.assertEqual((lewat["id"], lewat["status"]), ("1101", "DILEWATI"))
        self.assertIn("backtest", lewat["alasan"])
        self.assertEqual(hasil["ringkasan"]["n_seri_ok"], 1)
        self.assertEqual(hasil["ringkasan"]["n_seri_dilewati"], 1)

    def test_muatan_tidak_valid(self):
        titik = seri(90)
        kasus = [
            [],
            {"horizon": [7], "seri": []},
            {"horizon": [], "seri": [{"id": "a", "titik": titik}]},
            {"horizon": [0], "seri": [{"id": "a", "titik": titik}]},
            {"horizon": [True], "seri": [{"id": "a", "titik": titik}]},
            {"horizon": [91], "seri": [{"id": "a", "titik": titik}]},
            {"horizon": [7], "seri": [{"titik": titik}]},
            {"horizon": [7], "seri": [{"id": "a", "titik": "bukan larik"}]},
            {"horizon": [7], "seri": [{"id": "a", "titik": [{"tanggal": "x", "nilai": 1}]}]},
            {"horizon": [7], "seri": [{"id": str(i), "titik": []} for i in range(service.MAKS_SERI + 1)]},
        ]
        for muatan in kasus:
            with self.subTest(muatan=str(muatan)[:60]), self.assertRaises(service.MuatanTidakValid):
                service.latih(muatan)


class TestPrediksi(unittest.TestCase):
    def test_meramal_dari_parameter_terlatih(self):
        titik = seri(90, tren=30, derau=100)
        dilatih = service.latih({"horizon": [7], "seri": [{"id": "1171", "titik": titik}]})["seri"][0]
        hasil = service.prediksi(
            {
                "horizon": [7],
                "seri": [{"id": "1171", "titik": titik, "algoritma": dilatih["algoritma"], "params": dilatih["params"], "galat_rmse": dilatih["galat_rmse"]}],
            }
        )
        self.assertEqual(hasil["seri"][0]["ramalan"], dilatih["ramalan"])

    def test_butuh_algoritma_params_dan_galat(self):
        with self.assertRaises(service.MuatanTidakValid):
            service.prediksi({"horizon": [7], "seri": [{"id": "a", "titik": seri(60)}]})

    def test_galat_rmse_tidak_lengkap_ditolak(self):
        seri_a = {"id": "a", "titik": seri(60), "algoritma": "naive", "params": {}, "galat_rmse": {}}
        with self.assertRaises(service.MuatanTidakValid):
            service.prediksi({"horizon": [7], "seri": [seri_a]})


class TestServerHTTP(unittest.TestCase):
    """Satu server di loopback; tidak menyentuh jaringan luar."""

    @classmethod
    def setUpClass(cls):
        cls.server = buat_server("127.0.0.1", 0)
        cls.port = cls.server.server_address[1]
        threading.Thread(target=cls.server.serve_forever, daemon=True).start()

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()
        cls.server.server_close()

    def minta(self, metode, path, badan=None, header=None):
        koneksi = HTTPConnection("127.0.0.1", self.port, timeout=10)
        try:
            raw = badan if isinstance(badan, bytes) else (json.dumps(badan).encode() if badan is not None else None)
            koneksi.request(metode, path, body=raw, headers=header or {"Content-Type": "application/json"})
            r = koneksi.getresponse()
            return r.status, json.loads(r.read())
        finally:
            koneksi.close()

    def test_health(self):
        status, badan = self.minta("GET", "/health")
        self.assertEqual((status, badan["status"]), (200, "ok"))

    def test_train(self):
        status, badan = self.minta("POST", "/train", {"horizon": [7], "seri": [{"id": "1171", "titik": seri(90, derau=50)}]})
        self.assertEqual(status, 200)
        self.assertEqual(badan["seri"][0]["status"], "OK")

    def test_predict_muatan_salah_dijawab_400(self):
        status, badan = self.minta("POST", "/predict", {"horizon": [7], "seri": []})
        self.assertEqual(status, 400)
        self.assertIn("seri", badan["error"])

    def test_json_rusak_dijawab_400(self):
        status, _ = self.minta("POST", "/train", b"{bukan json")
        self.assertEqual(status, 400)

    def test_badan_kosong_dijawab_400(self):
        status, _ = self.minta("POST", "/train", None, header={})
        self.assertEqual(status, 400)

    def test_rute_tidak_dikenal(self):
        self.assertEqual(self.minta("GET", "/train")[0], 404)
        self.assertEqual(self.minta("POST", "/lain", {})[0], 404)


if __name__ == "__main__":
    unittest.main()
