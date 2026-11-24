"""Zero-dependency HTTP prediction & simulation server (:8000) for internal API use."""

import json
import logging
import math
import os
from dataclasses import dataclass
from datetime import date, timedelta
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from itertools import product
from typing import Any

from garda_ml import __version__
from garda_ml.domain.errors import InsufficientHistoryError, InvalidPayloadError

SEASON_PERIOD = 7
MIN_TRAIN_DAYS = 28
ORIGIN_STEP = 7
MIN_FOLDS = 2
MIN_COVERAGE = 0.7
Z_80 = 1.2816

CANDIDATES = ("naive", "seasonal_naive", "holt", "holt_winters")
BASELINE_MODEL = "naive"

FACTOR_LABELS: dict[str, tuple[str, str]] = {
    "level": ("Penyesuaian level harga", "Selisih level terpulus dengan harga terakhir"),
    "tren": ("Tren harga", "Kemiringan harga terkini yang diteruskan (teredam)"),
    "musiman": ("Pola musiman mingguan", "Pola harga per hari dalam sepekan"),
    "cuaca": ("Anomali cuaca & hujan lebat", "Efek historis curah hujan tinggi terhadap harga pangan"),
    "hari_libur": ("Hari besar & libur nasional", "Pola permintaan musiman hari libur/keagamaan"),
    "tetap": ("Harga terakhir dipertahankan", "Model naif: tidak ada perubahan struktural"),
}

_GRID_ALPHA = (0.2, 0.5, 0.8)
_GRID_BETA = (0.05, 0.2)
_GRID_GAMMA = (0.1, 0.3)
_GRID_PHI = (0.8, 0.95)

MAX_SERIES = 100
MAX_POINTS = 2000
MAX_HORIZON = 90
MAX_BODY_BYTES = 8 * 1024 * 1024

logger = logging.getLogger("garda_ml")


@dataclass(frozen=True)
class ForecastTrajectory:
    """Projected path with additive change components relative to latest observed price."""

    values: list[float]
    components: dict[str, list[float]]


def regularize_series(points: list[dict[str, Any]]) -> tuple[date, list[float]]:
    """Convert irregular `{tanggal, nilai}` points into a continuous daily series via linear fill."""
    if not points:
        raise InsufficientHistoryError("riwayat kosong")

    by_day: dict[date, float] = {}
    for pt in points:
        try:
            day = date.fromisoformat(str(pt["tanggal"]))
            val = float(pt["nilai"])
        except (KeyError, TypeError, ValueError) as err:
            raise ValueError(f"titik tidak valid: {pt!r}") from err
        if not math.isfinite(val) or val <= 0:
            raise ValueError(f"nilai harus positif: {pt!r}")
        if day in by_day:
            raise ValueError(f"tanggal ganda: {day.isoformat()}")
        by_day[day] = val

    start, end = min(by_day), max(by_day)
    span = (end - start).days + 1
    if len(by_day) / span < MIN_COVERAGE:
        raise InsufficientHistoryError(
            f"cakupan data {len(by_day)}/{span} hari di bawah {int(MIN_COVERAGE * 100)}%"
        )

    dense: list[float] = []
    for i in range(span):
        day = start + timedelta(days=i)
        if day in by_day:
            dense.append(by_day[day])
            continue
        right = i + 1
        while start + timedelta(days=right) not in by_day:
            right += 1
        y_left, y_right = dense[i - 1], by_day[start + timedelta(days=right)]
        dense.append(y_left + (y_right - y_left) / (right - i + 1))
    return start, dense


def _hw_state(
    y: list[float],
    alpha: float,
    beta: float,
    gamma: float,
    phi: float,
    seasonal: bool,
) -> tuple[float, float, float, list[float]]:
    m = SEASON_PERIOD
    if seasonal:
        level = sum(y[:m]) / m
        trend = (sum(y[m : 2 * m]) / m - level) / m
        season = [y[i] - level for i in range(m)]
        start_idx = 0
    else:
        width = min(m, len(y))
        level = y[0]
        trend = (y[width - 1] - y[0]) / max(width - 1, 1)
        season = [0.0] * m
        start_idx = 1

    sse = 0.0
    for i in range(start_idx, len(y)):
        s = season[i % m] if seasonal else 0.0
        err = y[i] - (level + phi * trend + s)
        sse += err * err
        level = level + phi * trend + alpha * err
        trend = phi * trend + alpha * beta * err
        if seasonal:
            season[i % m] = s + gamma * (1 - alpha) * err
    return sse, level, trend, season


def fit_smoothing_params(name: str, y: list[float]) -> dict[str, float]:
    """Grid-search exponential smoothing parameters minimizing one-step SSE."""
    if name in ("naive", "seasonal_naive"):
        return {}
    seasonal = name == "holt_winters"
    combos = (
        product(_GRID_ALPHA, _GRID_BETA, _GRID_GAMMA, _GRID_PHI)
        if seasonal
        else ((a, b, 0.0, p) for a, b, p in product(_GRID_ALPHA, _GRID_BETA, _GRID_PHI))
    )
    candidates = (
        (_hw_state(y, a, b, g, p, seasonal)[0], {"alpha": a, "beta": b, "gamma": g, "phi": p})
        for a, b, g, p in combos
    )
    return min(candidates, key=lambda c: c[0])[1]


def forecast_path(name: str, y: list[float], params: dict[str, Any], steps: int) -> ForecastTrajectory:
    """Project `steps` daily values forward from `y` with additive change attribution."""
    if name not in CANDIDATES:
        raise ValueError(f"algoritma tidak dikenal: {name}")
    latest = y[-1]
    ks = range(1, steps + 1)

    if name == "naive":
        return ForecastTrajectory([latest] * steps, {"tetap": [0.0] * steps})
    if name == "seasonal_naive":
        if len(y) < SEASON_PERIOD:
            raise InsufficientHistoryError("seasonal naive butuh minimal satu pekan riwayat")
        vals = [y[len(y) - SEASON_PERIOD + (k - 1) % SEASON_PERIOD] for k in ks]
        return ForecastTrajectory(vals, {"musiman": [v - latest for v in vals]})

    seasonal = name == "holt_winters"
    if seasonal and len(y) < 2 * SEASON_PERIOD:
        raise InsufficientHistoryError("Holt-Winters butuh minimal dua pekan riwayat")

    phi = float(params["phi"])
    _, level, trend, season = _hw_state(y, float(params["alpha"]), float(params["beta"]), float(params["gamma"]), phi, seasonal)
    n = len(y)
    comp_trend, comp_season = [], []
    accum = 0.0
    for k in ks:
        accum += phi**k
        comp_trend.append(trend * accum)
        comp_season.append(season[(n + k - 1) % SEASON_PERIOD] if seasonal else 0.0)
    comp_level = [level - latest] * steps
    vals = [latest + comp_level[i] + comp_trend[i] + comp_season[i] for i in range(steps)]
    components: dict[str, list[float]] = {"level": comp_level, "tren": comp_trend}
    if seasonal:
        components["musiman"] = comp_season
    return ForecastTrajectory(vals, components)


def compute_mape(actual: list[float], predicted: list[float]) -> float:
    """Mean absolute percentage error (%)."""
    if not actual or len(actual) != len(predicted):
        raise ValueError("deret aktual dan ramalan harus sama panjang dan tidak kosong")
    return 100 * sum(abs(a - f) / a for a, f in zip(actual, predicted)) / len(actual)


def compute_rmse(actual: list[float], predicted: list[float]) -> float:
    """Root mean square error."""
    if not actual or len(actual) != len(predicted):
        raise ValueError("deret aktual dan ramalan harus sama panjang dan tidak kosong")
    return math.sqrt(sum((a - f) ** 2 for a, f in zip(actual, predicted)) / len(actual))


def walk_forward_origins(n: int, max_horizon: int) -> list[int]:
    return list(range(MIN_TRAIN_DAYS, n - max_horizon + 1, ORIGIN_STEP))


def _backtest(name: str, y: list[float], origins: list[int], max_horizon: int) -> list[tuple[list[float], list[float]]]:
    folds = []
    for o in origins:
        train_y = y[:o]
        fc = forecast_path(name, train_y, fit_smoothing_params(name, train_y), max_horizon).values
        folds.append((y[o : o + max_horizon], fc))
    return folds


def _score_folds(folds: list[tuple[list[float], list[float]]], horizons: list[int]) -> tuple[float, float, dict[str, float]]:
    actual = [a[h - 1] for a, _ in folds for h in horizons]
    pred = [f[h - 1] for _, f in folds for h in horizons]
    per_h = {
        str(h): compute_mape([a[h - 1] for a, _ in folds], [f[h - 1] for _, f in folds])
        for h in horizons
    }
    return compute_mape(actual, pred), compute_rmse(actual, pred), per_h


def _round_pct_parts(parts: list[float], total: float) -> list[float]:
    rounded = [round(b, 1) for b in parts]
    remainder = round(round(total, 1) - sum(rounded), 1)
    largest_idx = max(range(len(rounded)), key=lambda i: abs(parts[i]))
    rounded[largest_idx] = round(rounded[largest_idx] + remainder, 1)
    return rounded


def factor_contributions(trajectory: ForecastTrajectory, step: int, base_price: float) -> list[dict[str, Any]]:
    """Decompose projected percentage change at `step` into additive rounded factors."""
    names = list(trajectory.components)
    parts = [trajectory.components[n][step - 1] / base_price * 100 for n in names]
    total = (trajectory.values[step - 1] - base_price) / base_price * 100
    return [
        {"nama": FACTOR_LABELS[n][0], "kontribusi": contrib, "detail": FACTOR_LABELS[n][1]}
        for n, contrib in zip(names, _round_pct_parts(parts, total))
    ]


def _extract_exogenous(points: list[dict[str, Any]], start: date, n: int) -> tuple[list[bool], list[bool]]:
    rain = [False] * n
    holiday = [False] * n
    for pt in points:
        raw_date = pt.get("tanggal")
        if not isinstance(raw_date, str):
            continue
        try:
            day = date.fromisoformat(raw_date[:10])
        except ValueError:
            continue
        idx = (day - start).days
        if 0 <= idx < n:
            if bool(pt.get("hujan_lebat")):
                rain[idx] = True
            if bool(pt.get("hari_libur")):
                holiday[idx] = True
    return rain, holiday


def _estimate_exogenous_effects(y: list[float], rain: list[bool], holiday: list[bool]) -> tuple[float, float]:
    n = len(y)
    if n < 14 or y[-1] <= 0:
        return 0.0, 0.0
    cap = 0.05 * y[-1]

    def _mean_diff(flags: list[bool], min_samples: int) -> float:
        flagged, normal = [], []
        for i in range(7, n):
            ma7 = sum(y[i - 7 : i]) / 7.0
            diff = y[i] - ma7
            (flagged if flags[i] else normal).append(diff)
        if len(flagged) < min_samples or len(normal) < 3:
            return 0.0
        effect = (sum(flagged) / len(flagged)) - (sum(normal) / len(normal))
        return max(-cap, min(cap, round(effect, 2)))

    return _mean_diff(rain, 3), _mean_diff(holiday, 2)


def _apply_exogenous(trajectory: ForecastTrajectory, params: dict[str, Any], latest_rain: bool, steps: int) -> ForecastTrajectory:
    weather_eff = float(params.get("efek_cuaca", 0.0) or 0.0)
    holiday_eff = float(params.get("efek_libur", 0.0) or 0.0)
    if weather_eff == 0.0 and holiday_eff == 0.0:
        return trajectory

    values = list(trajectory.values)
    components = {k: list(v) for k, v in trajectory.components.items()}
    if weather_eff != 0.0 and latest_rain:
        weather_vec = [round(weather_eff * (0.7**k), 2) if k < 3 else 0.0 for k in range(steps)]
        for k in range(steps):
            values[k] += weather_vec[k]
        components["cuaca"] = weather_vec
    if holiday_eff != 0.0:
        holiday_vec = [round(holiday_eff * 0.5, 2) if k < 7 else 0.0 for k in range(steps)]
        for k in range(steps):
            values[k] += holiday_vec[k]
        components["hari_libur"] = holiday_vec
    return ForecastTrajectory(values=values, components=components)


def _format_forecasts(
    name: str,
    y: list[float],
    params: dict[str, Any],
    rmse_errors: dict[str, float],
    horizons: list[int],
    base_date: date,
    latest_rain: bool = False,
) -> list[dict[str, Any]]:
    traj = _apply_exogenous(forecast_path(name, y, params, horizons[-1]), params, latest_rain, horizons[-1])
    out = []
    for h in horizons:
        raw = traj.values[h - 1]
        val = max(1, round(raw))
        sd = rmse_errors[str(h)]
        out.append(
            {
                "horizon_hari": h,
                "tanggal_target": (base_date + timedelta(days=h)).isoformat(),
                "nilai": val,
                "bawah": max(1, min(val, math.floor(raw - Z_80 * sd))),
                "atas": max(val, math.ceil(raw + Z_80 * sd)),
                "faktor": factor_contributions(traj, h, y[-1]),
            }
        )
    return out


def _is_finite_num(v: object) -> bool:
    return isinstance(v, (int, float)) and not isinstance(v, bool) and math.isfinite(v)


def _validate_horizons(horizons: list[int]) -> list[int]:
    ordered = sorted(set(horizons))
    if not ordered or ordered[0] < 1:
        raise ValueError("horizon harus berisi bilangan bulat positif")
    return ordered


def train_series(points: list[dict[str, Any]], horizons: list[int]) -> dict[str, Any]:
    """Train and select best baseline/exponential-smoothing model via walk-forward CV."""
    horizons = _validate_horizons(horizons)
    start, y = regularize_series(points)
    base_date = start + timedelta(days=len(y) - 1)
    origins = walk_forward_origins(len(y), horizons[-1])
    if len(origins) < MIN_FOLDS:
        raise InsufficientHistoryError(
            f"riwayat {len(y)} hari kurang untuk backtest horizon {horizons[-1]} hari "
            f"(butuh minimal {MIN_TRAIN_DAYS + ORIGIN_STEP * (MIN_FOLDS - 1) + horizons[-1]})"
        )

    n_test = len(origins) // 3
    optimistic = n_test == 0
    sel_idx = list(range(len(origins) - n_test))
    test_idx = sel_idx if optimistic else list(range(len(origins) - n_test, len(origins)))

    fold_preds = {name: _backtest(name, y, origins, horizons[-1]) for name in CANDIDATES}
    sel_scores = {name: _score_folds([folds[i] for i in sel_idx], horizons)[0] for name, folds in fold_preds.items()}
    chosen = min(sel_scores, key=lambda n: (sel_scores[n], CANDIDATES.index(n)))

    test_mape, test_rmse, per_h = _score_folds([fold_preds[chosen][i] for i in test_idx], horizons)
    base_mape, base_rmse, _ = _score_folds([fold_preds[BASELINE_MODEL][i] for i in test_idx], horizons)

    params: dict[str, Any] = dict(fit_smoothing_params(chosen, y))
    rain, holiday = _extract_exogenous(points, start, len(y))
    w_eff, h_eff = _estimate_exogenous_effects(y, rain, holiday)
    if w_eff != 0.0:
        params["efek_cuaca"] = w_eff
    if h_eff != 0.0:
        params["efek_libur"] = h_eff

    rmse_errors = {
        str(h): math.sqrt(sum((a[h - 1] - f[h - 1]) ** 2 for a, f in fold_preds[chosen]) / len(fold_preds[chosen]))
        for h in horizons
    }
    return {
        "algoritma": chosen,
        "params": params,
        "galat_rmse": rmse_errors,
        "metrik": {
            "mape": test_mape,
            "rmse": test_rmse,
            "baseline_mape": base_mape,
            "baseline_rmse": base_rmse,
            "mape_per_horizon": per_h,
            "n_lipatan_seleksi": len(sel_idx),
            "n_lipatan_uji": len(test_idx),
            "optimistis": optimistic,
            "mape_seleksi": sel_scores,
        },
        "harga_dasar": round(y[-1]),
        "tanggal_dasar": base_date.isoformat(),
        "n_hari": len(y),
        "ramalan": _format_forecasts(chosen, y, params, rmse_errors, horizons, base_date, rain[-1] if rain else False),
    }


def predict_series(
    points: list[dict[str, Any]],
    algorithm: str,
    params: dict[str, Any],
    rmse_errors: dict[str, float],
    horizons: list[int],
) -> dict[str, Any]:
    """Generate forward forecasts from pretrained parameters without refitting."""
    horizons = _validate_horizons(horizons)
    if algorithm not in CANDIDATES:
        raise ValueError(f"algoritma tidak dikenal: {algorithm}")
    missing = [h for h in horizons if not _is_finite_num(rmse_errors.get(str(h)))]
    if missing:
        raise ValueError(f"galat_rmse harus berisi angka untuk horizon {missing}")
    if algorithm in ("holt", "holt_winters") and not all(_is_finite_num(params.get(k)) for k in ("alpha", "beta", "gamma", "phi")):
        raise ValueError("params harus berisi angka alpha, beta, gamma, phi")

    start, y = regularize_series(points)
    rain, _ = _extract_exogenous(points, start, len(y))
    base_date = start + timedelta(days=len(y) - 1)
    return {
        "algoritma": algorithm,
        "harga_dasar": round(y[-1]),
        "tanggal_dasar": base_date.isoformat(),
        "ramalan": _format_forecasts(algorithm, y, params, rmse_errors, horizons, base_date, rain[-1] if rain else False),
    }


def summarize_batch(results: list[dict[str, Any]]) -> dict[str, Any]:
    """Aggregate summary metrics across all successfully trained series."""
    ok = [r for r in results if r.get("status") == "OK"]
    summary: dict[str, Any] = {"n_seri_ok": len(ok), "n_seri_dilewati": len(results) - len(ok)}
    if not ok:
        return summary
    for key in ("mape", "rmse", "baseline_mape", "baseline_rmse"):
        summary[key] = sum(r["metrik"][key] for r in ok) / len(ok)
    summary["optimistis"] = any(r["metrik"]["optimistis"] for r in ok)
    summary["algoritma"] = {
        name: sum(1 for r in ok if r["algoritma"] == name)
        for name in CANDIDATES
        if any(r["algoritma"] == name for r in ok)
    }
    return summary


def _require_dict(payload: object, name: str) -> dict[str, Any]:
    if not isinstance(payload, dict):
        raise InvalidPayloadError(f"{name} harus berupa objek JSON")
    return payload


def _parse_horizons_payload(payload: dict[str, Any]) -> list[int]:
    vals = payload.get("horizon")
    if (
        not isinstance(vals, list)
        or not vals
        or not all(isinstance(h, int) and not isinstance(h, bool) and 1 <= h <= MAX_HORIZON for h in vals)
    ):
        raise InvalidPayloadError(f"horizon harus berisi bilangan bulat 1..{MAX_HORIZON}")
    return vals


def _parse_series_list(payload: dict[str, Any]) -> list[dict[str, Any]]:
    series = payload.get("seri")
    if not isinstance(series, list) or not series:
        raise InvalidPayloadError("seri harus berisi minimal satu deret")
    if len(series) > MAX_SERIES:
        raise InvalidPayloadError(f"seri maksimal {MAX_SERIES} per permintaan")
    for item in series:
        obj = _require_dict(item, "seri[]")
        if not isinstance(obj.get("id"), str) or not obj["id"]:
            raise InvalidPayloadError("setiap seri wajib punya id berupa teks")
        pts = obj.get("titik")
        if not isinstance(pts, list) or len(pts) > MAX_POINTS:
            raise InvalidPayloadError(f"titik harus berupa larik maksimal {MAX_POINTS} elemen")
    return series


def handle_health() -> dict[str, str]:
    return {"status": "ok", "versi": __version__}


def handle_train(payload: object) -> dict[str, Any]:
    body = _require_dict(payload, "muatan")
    horizons = _parse_horizons_payload(body)
    results = []
    for item in _parse_series_list(body):
        try:
            results.append({"id": item["id"], "status": "OK", **train_series(item["titik"], horizons)})
        except InsufficientHistoryError as err:
            results.append({"id": item["id"], "status": "DILEWATI", "alasan": str(err)})
        except ValueError as err:
            raise InvalidPayloadError(f"seri {item['id']}: {err}") from err
    return {
        "versi_layanan": __version__,
        "komoditas_kode": body.get("komoditas_kode", ""),
        "seri": results,
        "ringkasan": summarize_batch(results),
    }


def handle_predict(payload: object) -> dict[str, Any]:
    body = _require_dict(payload, "muatan")
    horizons = _parse_horizons_payload(body)
    results = []
    for item in _parse_series_list(body):
        err_rmse = item.get("galat_rmse")
        params = item.get("params")
        if not isinstance(item.get("algoritma"), str) or not isinstance(params, dict) or not isinstance(err_rmse, dict):
            raise InvalidPayloadError(f"seri {item['id']}: algoritma, params, dan galat_rmse wajib ada")
        try:
            results.append(
                {"id": item["id"], "status": "OK", **predict_series(item["titik"], item["algoritma"], params, err_rmse, horizons)}
            )
        except InsufficientHistoryError as err:
            results.append({"id": item["id"], "status": "DILEWATI", "alasan": str(err)})
        except ValueError as err:
            raise InvalidPayloadError(f"seri {item['id']}: {err}") from err
    return {
        "versi_layanan": __version__,
        "komoditas_kode": body.get("komoditas_kode", ""),
        "seri": results,
    }


def create_inference_server(host: str = "0.0.0.0", port: int = 8000) -> ThreadingHTTPServer:
    """Create stateless HTTP inference server exposing `/health`, `/train`, and `/predict`."""
    post_routes = {"/train": handle_train, "/predict": handle_predict}

    class _Handler(BaseHTTPRequestHandler):
        server_version = "garda-ml"

        def _send_json(self, code: int, body: dict[str, Any]) -> None:
            raw = json.dumps(body, ensure_ascii=False).encode("utf-8")
            self.send_response(code)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Content-Length", str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)

        def do_GET(self) -> None:  # noqa: N802
            if self.path == "/health":
                self._send_json(200, handle_health())
            else:
                self._send_json(404, {"error": "tidak ditemukan"})

        def do_POST(self) -> None:  # noqa: N802
            fn = post_routes.get(self.path)
            if fn is None:
                self._send_json(404, {"error": "tidak ditemukan"})
                return
            try:
                length = int(self.headers.get("Content-Length", ""))
            except ValueError:
                self._send_json(411, {"error": "Content-Length wajib ada"})
                return
            if length > MAX_BODY_BYTES:
                self._send_json(413, {"error": "badan permintaan terlalu besar"})
                return
            try:
                body = json.loads(self.rfile.read(length))
            except json.JSONDecodeError:
                self._send_json(400, {"error": "badan bukan JSON yang valid"})
                return
            try:
                self._send_json(200, fn(body))
            except InvalidPayloadError as err:
                self._send_json(400, {"error": str(err)})
            except Exception:
                logger.exception("unexpected error on %s", self.path)
                self._send_json(500, {"error": "galat internal"})

        def log_message(self, format: str, *args: Any) -> None:  # noqa: A002
            logger.info("%s %s", self.address_string(), format % args)

    return ThreadingHTTPServer((host, port), _Handler)


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    host = os.environ.get("ML_HOST", "0.0.0.0")
    port = int(os.environ.get("ML_PORT", "8000"))
    server = create_inference_server(host, port)
    logger.info("ML inference HTTP server listening on %s:%d", host, port)
    server.serve_forever()
