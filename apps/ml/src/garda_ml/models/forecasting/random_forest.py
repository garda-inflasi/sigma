"""Random Forest quantile forecaster (challenger estimator for noisy volatile regimes)."""

from typing import Any

import numpy as np
from sklearn.ensemble import RandomForestRegressor
from sklearn.impute import SimpleImputer

from garda_ml.config.model_specs import RF_PARAM_DEFAULTS
from garda_ml.domain.enums import ModelFamily


class RandomForestQuantileEstimator:
    """Estimates conditional quantiles across individual decision tree predictions in an ensemble."""

    model_key = ModelFamily.RANDOM_FOREST_QUANTILE

    def __init__(
        self,
        quantile: float = 0.5,
        params: dict[str, Any] | None = None,
        n_jobs: int = 2,
        seed: int = 42,
    ) -> None:
        p = {**RF_PARAM_DEFAULTS, **(params or {})}
        self.quantile = quantile
        self._imputer = SimpleImputer(strategy="median")
        self._forest = RandomForestRegressor(
            n_estimators=int(p["n_estimators"]),
            max_depth=p["max_depth"],
            min_samples_leaf=int(p["min_samples_leaf"]),
            n_jobs=n_jobs,
            random_state=seed,
        )

    def fit(
        self,
        X: Any,
        y: Any,
        *,
        quantile: float | None = None,
        sample_weight: Any = None,
    ) -> "RandomForestQuantileEstimator":
        if quantile is not None:
            self.quantile = quantile
        X_clean = self._imputer.fit_transform(X)
        self._forest.fit(X_clean, y, sample_weight=sample_weight)
        return self

    def predict(self, X: Any) -> np.ndarray:
        X_clean = self._imputer.transform(X)
        if not self._forest.estimators_:
            return np.asarray(self._forest.predict(X_clean), dtype=float)
        tree_preds = np.stack([tree.predict(X_clean) for tree in self._forest.estimators_], axis=0)
        return np.quantile(tree_preds, self.quantile, axis=0).astype(float)
