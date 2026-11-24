"""Random Forest challenger binary classifier for tabular pressure regimes."""

from typing import Any

import numpy as np
from sklearn.ensemble import RandomForestClassifier
from sklearn.impute import SimpleImputer
from sklearn.pipeline import Pipeline

from garda_ml.config.model_specs import RF_PARAM_DEFAULTS
from garda_ml.domain.enums import ModelFamily


def build_rf_classifier_estimator(n_jobs: int = 2, seed: int = 42) -> Pipeline:
    """Create median-imputed RandomForest classifier challenger."""
    return Pipeline(
        [
            ("imputer", SimpleImputer(strategy="median")),
            (
                "rf",
                RandomForestClassifier(
                    n_estimators=int(RF_PARAM_DEFAULTS["n_estimators"]),
                    max_depth=RF_PARAM_DEFAULTS["max_depth"],
                    min_samples_leaf=int(RF_PARAM_DEFAULTS["min_samples_leaf"]),
                    n_jobs=n_jobs,
                    random_state=seed,
                ),
            ),
        ]
    )


class CalibratedRandomForestClassifier:
    """Challenger Random Forest classifier wrapped behind uniform RiskClassifier API."""

    model_key = ModelFamily.RANDOM_FOREST_CLASSIFIER

    def __init__(self, n_jobs: int = 2, seed: int = 42) -> None:
        self._estimator = build_rf_classifier_estimator(n_jobs=n_jobs, seed=seed)

    @property
    def raw_estimator(self) -> Pipeline:
        return self._estimator

    def fit(self, X: Any, y: Any, *, sample_weight: Any = None) -> "CalibratedRandomForestClassifier":
        kwargs = {"rf__sample_weight": sample_weight} if sample_weight is not None else {}
        self._estimator.fit(X, y, **kwargs)
        return self

    def predict_proba(self, X: Any) -> np.ndarray:
        return np.asarray(self._estimator.predict_proba(X), dtype=float)
