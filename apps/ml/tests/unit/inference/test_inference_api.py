"""Zero-dependency stdlib unit tests for stateless HTTP inference service (:8000)."""

import json
import math
import random
import threading
import unittest
from datetime import date, timedelta
from http.client import HTTPConnection

from garda_ml.domain.errors import InsufficientHistoryError, InvalidPayloadError
from garda_ml.entrypoints import inference_api as api


def make_series(
    n: int = 90,
    base: float = 40000.0,
    trend: float = 0.0,
    season: list[float] | None = None,
    noise: float = 0.0,
    start: date = date(2026, 8, 1),
    seed: int = 1,
):
    rng = random.Random(seed)
    season = season or [0.0] * 7
    return [
        {
            "tanggal": (start + timedelta(days=i)).isoformat(),
            "nilai": round(base + trend * i + season[i % 7] + rng.gauss(0, noise), 2),
        }
        for i in range(n)
    ]


class TestRegularizeSeries(unittest.TestCase):
    def test_linearly_interpolates_gaps(self):
        points = [
            {"tanggal": "2026-08-01", "nilai": 100},
            {"tanggal": "2026-08-02", "nilai": 110},
            {"tanggal": "2026-08-04", "nilai": 130},
            {"tanggal": "2026-08-05", "nilai": 140},
            {"tanggal": "2026-08-06", "nilai": 150},
        ]
        start, y = api.regularize_series(points)
        self.assertEqual(start, date(2026, 8, 1))
        self.assertEqual([round(v, 6) for v in y], [100, 110, 120, 130, 140, 150])

    def test_sorts_unsorted_points(self):
        pts = make_series(10)
        random.Random(3).shuffle(pts)
        _, y = api.regularize_series(pts)
        self.assertEqual(len(y), 10)

    def test_rejects_low_coverage_and_bad_values(self):
        with self.assertRaises(InsufficientHistoryError):
            api.regularize_series([{"tanggal": "2026-08-01", "nilai": 1}, {"tanggal": "2026-08-10", "nilai": 1}])
        with self.assertRaises(InsufficientHistoryError):
            api.regularize_series([])
        with self.assertRaises(ValueError):
            api.regularize_series([{"tanggal": "2026-08-01", "nilai": 0}])
        with self.assertRaises(ValueError):
            api.regularize_series([{"tanggal": "bad-date", "nilai": 5}])


class TestForecastPathAndFactors(unittest.TestCase):
    def test_naive_and_seasonal_naive(self):
        self.assertEqual(api.forecast_path("naive", [10.0, 12.0, 11.0], {}, 3).values, [11.0] * 3)
        y = [float(i) for i in range(1, 15)]
        self.assertEqual(api.forecast_path("seasonal_naive", y, {}, 9).values, [8, 9, 10, 11, 12, 13, 14, 8, 9])

    def test_components_sum_to_delta_and_factors(self):
        _, y = api.regularize_series(make_series(60, trend=50, season=[0, 300, -200, 100, 0, -300, 100], noise=100))
        for name in api.CANDIDATES:
            traj = api.forecast_path(name, y, api.fit_smoothing_params(name, y), 14)
            for k in range(14):
                total = sum(v[k] for v in traj.components.values())
                self.assertAlmostEqual(y[-1] + total, traj.values[k], places=6, msg=name)
            total_pct = round((traj.values[6] - y[-1]) / y[-1] * 100, 1)
            sum_pct = round(sum(f["kontribusi"] for f in api.factor_contributions(traj, 7, y[-1])), 1)
            self.assertEqual(sum_pct, total_pct, name)

    def test_strong_trend_beats_naive(self):
        out = api.train_series(make_series(90, base=30000, trend=120, noise=30), [7, 14])
        self.assertIn(out["algoritma"], ("holt", "holt_winters"))
        self.assertLess(out["metrik"]["mape"], out["metrik"]["baseline_mape"])


class TestHttpInferenceEndpoints(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.server = api.create_inference_server("127.0.0.1", 0)
        cls.port = cls.server.server_address[1]
        threading.Thread(target=cls.server.serve_forever, daemon=True).start()

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()
        cls.server.server_close()

    def request(self, method, path, body=None, headers=None):
        conn = HTTPConnection("127.0.0.1", self.port, timeout=10)
        try:
            raw = body if isinstance(body, bytes) else (json.dumps(body).encode() if body is not None else None)
            conn.request(method, path, body=raw, headers=headers or {"Content-Type": "application/json"})
            resp = conn.getresponse()
            return resp.status, json.loads(resp.read())
        finally:
            conn.close()

    def test_health_train_and_predict_contract(self):
        code, health = self.request("GET", "/health")
        self.assertEqual((code, health["status"]), (200, "ok"))

        pts = make_series(90, noise=50)
        code, trained = self.request("POST", "/train", {"horizon": [7, 14], "seri": [{"id": "1171", "titik": pts}, {"id": "1101", "titik": make_series(20)}]})
        self.assertEqual(code, 200)
        ok, skipped = trained["seri"]
        self.assertEqual(ok["status"], "OK")
        self.assertEqual(skipped["status"], "DILEWATI")

        code, predicted = self.request(
            "POST",
            "/predict",
            {"horizon": [7, 14], "seri": [{"id": "1171", "titik": pts, "algoritma": ok["algoritma"], "params": ok["params"], "galat_rmse": ok["galat_rmse"]}]},
        )
        self.assertEqual(code, 200)
        self.assertEqual(predicted["seri"][0]["ramalan"], ok["ramalan"])

    def test_invalid_payloads_return_400(self):
        self.assertEqual(self.request("POST", "/predict", {"horizon": [7], "seri": []})[0], 400)
        self.assertEqual(self.request("POST", "/train", b"{bad json")[0], 400)
        self.assertEqual(self.request("GET", "/train")[0], 404)


if __name__ == "__main__":
    unittest.main()
