"""LightGBM (primary) and HistGradientBoosting (fallback) risk classifiers."""

from typing import Any

import numpy as np
from sklearn.ensemble import HistGradientBoostingClassifier

from garda_ml.domain.enums import ModelFamily


def build_lgbm_classifier_estimator(n_jobs: int = 2, seed: int = 42) -> Any:
    """Construct LightGBM binary risk classifier tuned for tabular macro/price panels."""
    from lightgbm import LGBMClassifier

    return LGBMClassifier(
        n_estimators=150,
        learning_rate=0.05,
        num_leaves=15,
        min_child_samples=10,
        subsample=0.8,
        subsample_freq=1,
        colsample_bytree=0.8,
        random_state=seed,
        n_jobs=n_jobs,
        verbosity=-1,
    )


def build_hgb_classifier_estimator(seed: int = 42) -> HistGradientBoostingClassifier:
    """Construct HistGradientBoosting fallback binary risk classifier."""
    return HistGradientBoostingClassifier(
        max_iter=150,
        learning_rate=0.05,
        max_leaf_nodes=15,
        min_samples_leaf=10,
        early_stopping=False,
        random_state=seed,
    )


class LightGbmRiskClassifier:
    """Primary LightGBM inflation pressure risk classifier."""

    model_key = ModelFamily.LIGHTGBM_QUANTILE

    def __init__(self, n_jobs: int = 2, seed: int = 42) -> None:
        self._estimator = build_lgbm_classifier_estimator(n_jobs=n_jobs, seed=seed)

    @property
    def raw_estimator(self) -> Any:
        return self._estimator

    def fit(self, X: Any, y: Any, *, sample_weight: Any = None) -> "LightGbmRiskClassifier":
        self._estimator.fit(X, y, sample_weight=sample_weight)
        return self

    def predict_proba(self, X: Any) -> np.ndarray:
        return np.asarray(self._estimator.predict_proba(X), dtype=float)


class HistGradientBoostingRiskClassifier:
    """Fallback HistGradientBoosting inflation pressure classifier."""

    model_key = ModelFamily.HGB_QUANTILE

    def __init__(self, seed: int = 42) -> None:
        self._estimator = build_hgb_classifier_estimator(seed=seed)

    @property
    def raw_estimator(self) -> HistGradientBoostingClassifier:
        return self._estimator

    def fit(self, X: Any, y: Any, *, sample_weight: Any = None) -> "HistGradientBoostingRiskClassifier":
        self._estimator.fit(X, y, sample_weight=sample_weight)
        return self

    def predict_proba(self, X: Any) -> np.ndarray:
        return np.asarray(self._estimator.predict_proba(X), dtype=float)
