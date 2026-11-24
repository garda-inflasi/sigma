"""LightGBM (primary) and HistGradientBoosting (fallback) global quantile regressors."""

from typing import Any

import numpy as np
from sklearn.ensemble import HistGradientBoostingRegressor

from garda_ml.config.model_specs import DEFAULT_BOOSTING_PARAMS
from garda_ml.domain.enums import ModelFamily


class LightGbmQuantileEstimator:
    """Primary LightGBM quantile regressor for multi-series tabular forecasting."""

    model_key = ModelFamily.LIGHTGBM_QUANTILE

    def __init__(
        self,
        quantile: float = 0.5,
        params: dict[str, Any] | None = None,
        n_jobs: int = 2,
        seed: int = 42,
    ) -> None:
        from lightgbm import LGBMRegressor

        p = {**DEFAULT_BOOSTING_PARAMS, **(params or {})}
        self.quantile = quantile
        self._estimator = LGBMRegressor(
            objective="quantile",
            alpha=quantile,
            n_estimators=int(p["n_estimators"]),
            learning_rate=float(p["learning_rate"]),
            num_leaves=int(p["num_leaves"]),
            min_child_samples=int(p["min_child_samples"]),
            subsample=0.8,
            subsample_freq=1,
            colsample_bytree=0.8,
            random_state=seed,
            n_jobs=n_jobs,
            verbosity=-1,
        )

    @property
    def raw_estimator(self) -> Any:
        """Expose underlying booster for TreeSHAP explainer."""
        return self._estimator

    def fit(self, X: Any, y: Any, *, quantile: float | None = None, sample_weight: Any = None) -> "LightGbmQuantileEstimator":
        if quantile is not None:
            self._estimator.set_params(alpha=quantile)
        self._estimator.fit(X, y, sample_weight=sample_weight)
        return self

    def predict(self, X: Any) -> np.ndarray:
        return np.asarray(self._estimator.predict(X), dtype=float)


class HistGradientBoostingQuantileEstimator:
    """Fallback scikit-learn HistGradientBoosting quantile regressor when libomp/LightGBM is absent."""

    model_key = ModelFamily.HGB_QUANTILE

    def __init__(
        self,
        quantile: float = 0.5,
        params: dict[str, Any] | None = None,
        seed: int = 42,
    ) -> None:
        p = {**DEFAULT_BOOSTING_PARAMS, **(params or {})}
        self.quantile = quantile
        self._estimator = HistGradientBoostingRegressor(
            loss="quantile",
            quantile=quantile,
            max_iter=int(p["n_estimators"]),
            learning_rate=float(p["learning_rate"]),
            max_leaf_nodes=int(p["num_leaves"]),
            min_samples_leaf=int(p["min_child_samples"]),
            early_stopping=False,
            random_state=seed,
        )

    @property
    def raw_estimator(self) -> Any:
        return self._estimator

    def fit(
        self,
        X: Any,
        y: Any,
        *,
        quantile: float | None = None,
        sample_weight: Any = None,
    ) -> "HistGradientBoostingQuantileEstimator":
        if quantile is not None:
            self._estimator.set_params(quantile=quantile)
        self._estimator.fit(X, y, sample_weight=sample_weight)
        return self

    def predict(self, X: Any) -> np.ndarray:
        return np.asarray(self._estimator.predict(X), dtype=float)
