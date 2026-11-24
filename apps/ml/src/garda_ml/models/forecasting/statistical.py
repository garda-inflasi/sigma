"""Per-series statistical forecasters (Holt-Winters, SARIMAX) in log-price space."""

import warnings
from collections.abc import Callable

import numpy as np

from garda_ml.config.constants import HOLT_WINTERS_MIN_POINTS, SARIMAX_MIN_POINTS
from garda_ml.domain.enums import ModelFamily
from garda_ml.models.forecasting.baselines import (
    SEASON_DAYS,
    Z_P90,
    forecast_naive,
    forecast_seasonal_naive,
)

MAX_STAT_POINTS = 365
SARIMAX_SCALE = 100.0


def sort_quantiles(matrix: np.ndarray) -> np.ndarray:
    """Enforce monotonic quantile ordering (P10 <= P50 <= P90) along the last axis."""
    return np.sort(matrix, axis=-1)


def forecast_holt_winters(log_y: np.ndarray, horizons: list[int]) -> np.ndarray:
    """Damped additive trend + optional weekly seasonal exponential smoothing in log space."""
    from statsmodels.tsa.holtwinters import ExponentialSmoothing

    y = log_y[-MAX_STAT_POINTS:]
    seasonal = "add" if len(y) >= 3 * SEASON_DAYS else None
    with warnings.catch_warnings():
        warnings.simplefilter("ignore")
        fitted = ExponentialSmoothing(
            y,
            trend="add",
            damped_trend=True,
            seasonal=seasonal,
            seasonal_periods=SEASON_DAYS if seasonal else None,
            initialization_method="estimated",
        ).fit(optimized=True)

    max_h = max(horizons)
    raw_fc = np.asarray(fitted.forecast(max_h))
    indices = np.asarray(horizons) - 1
    sigma = float(np.std(np.asarray(fitted.resid)))
    scale = np.maximum(sigma * np.sqrt(np.asarray(horizons, dtype=float)), 1e-4)
    p50 = raw_fc[indices]
    return np.column_stack([p50 - Z_P90 * scale, p50, p50 + Z_P90 * scale])


def _fit_sarimax(scaled_y: np.ndarray):
    from statsmodels.tsa.statespace.sarimax import SARIMAX

    with warnings.catch_warnings():
        warnings.simplefilter("ignore")
        return SARIMAX(
            scaled_y,
            order=(1, 1, 1),
            seasonal_order=(0, 1, 1, SEASON_DAYS),
            enforce_stationarity=False,
            enforce_invertibility=False,
        ).fit(disp=False, maxiter=50)


def _extract_sarimax_quantiles(fitted: object, baseline: float, latest_log: float, horizons: list[int]) -> np.ndarray:
    with warnings.catch_warnings():
        warnings.simplefilter("ignore")
        forecast_obj = fitted.get_forecast(max(horizons))  # type: ignore[attr-defined]
        mean = np.asarray(forecast_obj.predicted_mean) / SARIMAX_SCALE + baseline
        bounds = np.asarray(forecast_obj.conf_int(alpha=0.2)) / SARIMAX_SCALE + baseline

    indices = np.asarray(horizons) - 1
    out = np.column_stack([bounds[indices, 0], mean[indices], bounds[indices, 1]])
    if not np.isfinite(out).all():
        raise ValueError("SARIMAX menghasilkan nilai tidak hingga")
    if np.abs(out - latest_log).max() > np.log(5.0):
        raise ValueError("SARIMAX tidak stabil")
    return out


def forecast_sarimax(log_y: np.ndarray, horizons: list[int]) -> np.ndarray:
    """Fit and forecast seasonal ARIMA on centered percentage-scaled log prices."""
    y = log_y[-MAX_STAT_POINTS:]
    baseline = float(y[-1])
    fitted = _fit_sarimax((y - baseline) * SARIMAX_SCALE)
    return _extract_sarimax_quantiles(fitted, baseline, baseline, horizons)


def _forecast_sarimax_folds(log_y: np.ndarray, origin_positions: list[int], horizons: list[int]) -> list[np.ndarray]:
    outputs: list[np.ndarray] = []
    base_fit = None
    base_anchor = 0.0
    for pos in origin_positions:
        slice_y = log_y[max(0, pos + 1 - MAX_STAT_POINTS) : pos + 1]
        if base_fit is None:
            base_anchor = float(slice_y[-1])
            base_fit = filtered = _fit_sarimax((slice_y - base_anchor) * SARIMAX_SCALE)
        else:
            with warnings.catch_warnings():
                warnings.simplefilter("ignore")
                filtered = base_fit.apply((slice_y - base_anchor) * SARIMAX_SCALE)
        outputs.append(_extract_sarimax_quantiles(filtered, base_anchor, float(slice_y[-1]), horizons))
    return outputs


STATISTICAL_FORECASTERS: dict[str, Callable[[np.ndarray, list[int]], np.ndarray]] = {
    ModelFamily.NAIVE: forecast_naive,
    ModelFamily.SEASONAL_NAIVE: forecast_seasonal_naive,
    ModelFamily.HOLT_WINTERS: forecast_holt_winters,
    ModelFamily.SARIMAX: forecast_sarimax,
}


def forecast_series_folds(
    model_name: str,
    log_y: np.ndarray,
    origin_positions: list[int],
    horizons: list[int],
) -> list[np.ndarray]:
    """Generate leakage-free walk-forward fold predictions up to each origin position."""
    if model_name == ModelFamily.SARIMAX:
        return _forecast_sarimax_folds(log_y, origin_positions, horizons)
    fn = STATISTICAL_FORECASTERS[model_name]
    return [fn(log_y[: pos + 1], horizons) for pos in origin_positions]


def statistical_candidates(valid_point_count: int) -> list[str]:
    """Return eligible per-series statistical models given original observation depth."""
    if valid_point_count < HOLT_WINTERS_MIN_POINTS:
        return [ModelFamily.NAIVE, ModelFamily.SEASONAL_NAIVE]
    if valid_point_count < SARIMAX_MIN_POINTS:
        return [ModelFamily.NAIVE, ModelFamily.SEASONAL_NAIVE, ModelFamily.HOLT_WINTERS]
    return [
        ModelFamily.NAIVE,
        ModelFamily.SEASONAL_NAIVE,
        ModelFamily.HOLT_WINTERS,
        ModelFamily.SARIMAX,
    ]


class HoltWintersEstimator:
    """Wrapper exposing Holt-Winters via unified estimator attributes."""

    model_key = ModelFamily.HOLT_WINTERS

    def forecast(self, log_y: np.ndarray, horizons: list[int]) -> np.ndarray:
        return forecast_holt_winters(log_y, horizons)


class SarimaxEstimator:
    """Wrapper exposing SARIMAX via unified estimator attributes."""

    model_key = ModelFamily.SARIMAX

    def forecast(self, log_y: np.ndarray, horizons: list[int]) -> np.ndarray:
        return forecast_sarimax(log_y, horizons)
