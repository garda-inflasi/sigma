"""Calibrated Logistic Regression classifier for small-sample macro & inflation regimes."""

from typing import Any

import numpy as np
from sklearn.impute import SimpleImputer
from sklearn.linear_model import LogisticRegression
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import StandardScaler

from garda_ml.domain.enums import ModelFamily


def build_logistic_classifier_estimator(C: float = 1.0, seed: int = 42) -> Pipeline:
    """Create median-imputed, standardized L2 logistic regression classifier."""
    return Pipeline(
        [
            ("imputer", SimpleImputer(strategy="median")),
            ("scaler", StandardScaler()),
            ("logreg", LogisticRegression(C=C, max_iter=1000, random_state=seed)),
        ]
    )


class CalibratedLogisticClassifier:
    """Primary linear risk classifier when BPS official monthly history is short."""

    model_key = ModelFamily.LOGISTIC_REGRESSION

    def __init__(self, C: float = 1.0, seed: int = 42) -> None:
        self._estimator = build_logistic_classifier_estimator(C=C, seed=seed)

    @property
    def raw_estimator(self) -> Pipeline:
        return self._estimator

    def fit(self, X: Any, y: Any, *, sample_weight: Any = None) -> "CalibratedLogisticClassifier":
        kwargs = {"logreg__sample_weight": sample_weight} if sample_weight is not None else {}
        self._estimator.fit(X, y, **kwargs)
        return self

    def predict_proba(self, X: Any) -> np.ndarray:
        return np.asarray(self._estimator.predict_proba(X), dtype=float)
