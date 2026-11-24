"""Baseline persistence, seasonal naive, and regularized Ridge benchmark forecasters."""

from typing import Any

import numpy as np
from sklearn.impute import SimpleImputer
from sklearn.linear_model import Ridge
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import StandardScaler

from garda_ml.domain.enums import ModelFamily

Z_P90 = 1.2815515655446004
SEASON_DAYS = 7


def _interval_band(p50: np.ndarray, scale: np.ndarray) -> np.ndarray:
    safe_scale = np.maximum(np.nan_to_num(scale, nan=0.0), 1e-4)
    return np.column_stack([p50 - Z_P90 * safe_scale, p50, p50 + Z_P90 * safe_scale])


def forecast_naive(log_y: np.ndarray, horizons: list[int]) -> np.ndarray:
    """Persistence baseline: last observed value with random-walk uncertainty band."""
    sigma = float(np.std(np.diff(log_y))) if len(log_y) > 2 else 0.0
    p50 = np.full(len(horizons), float(log_y[-1]))
    scale = sigma * np.sqrt(np.asarray(horizons, dtype=float))
    return _interval_band(p50, scale)


def forecast_seasonal_naive(log_y: np.ndarray, horizons: list[int]) -> np.ndarray:
    """Weekly seasonal persistence repeating same day-of-week one week prior."""
    n = len(log_y)
    period = SEASON_DAYS if n >= 2 * SEASON_DAYS else 1
    p50 = np.array([log_y[n - period + ((h - 1) % period)] for h in horizons], dtype=float)
    diffs = log_y[period:] - log_y[:-period] if n > period else np.array([0.0])
    sigma = float(np.std(diffs)) if len(diffs) >= 2 else 0.02
    scale = sigma * np.sqrt(np.ceil(np.asarray(horizons, dtype=float) / period))
    return _interval_band(p50, scale)


class NaiveEstimator:
    """Baseline persistence quantile forecaster wrapper."""

    model_key = ModelFamily.NAIVE

    def forecast(self, log_y: np.ndarray, horizons: list[int]) -> np.ndarray:
        return forecast_naive(log_y, horizons)


class SeasonalNaiveEstimator:
    """Baseline seasonal-naive quantile forecaster wrapper."""

    model_key = ModelFamily.SEASONAL_NAIVE

    def forecast(self, log_y: np.ndarray, horizons: list[int]) -> np.ndarray:
        return forecast_seasonal_naive(log_y, horizons)


class RidgeQuantileEstimator:
    """Regularized linear regression benchmark with empirical residual quantile intervals."""

    model_key = ModelFamily.RIDGE_QUANTILE

    def __init__(self, quantile: float = 0.5, alpha: float = 1.0) -> None:
        self.quantile = quantile
        self.alpha = alpha
        self._pipeline = Pipeline(
            [
                ("imputer", SimpleImputer(strategy="median")),
                ("scaler", StandardScaler()),
                ("ridge", Ridge(alpha=alpha)),
            ]
        )
        self._quantile_offset = 0.0

    def fit(self, X: Any, y: Any, *, quantile: float | None = None, sample_weight: Any = None) -> "RidgeQuantileEstimator":
        target_q = self.quantile if quantile is None else quantile
        y_arr = np.asarray(y, dtype=float)
        fit_kwargs = {"ridge__sample_weight": sample_weight} if sample_weight is not None else {}
        self._pipeline.fit(X, y_arr, **fit_kwargs)
        preds = self._pipeline.predict(X)
        residuals = y_arr - preds
        self._quantile_offset = float(np.quantile(residuals, target_q)) if abs(target_q - 0.5) > 1e-6 else 0.0
        return self

    def predict(self, X: Any) -> np.ndarray:
        return np.asarray(self._pipeline.predict(X), dtype=float) + self._quantile_offset
