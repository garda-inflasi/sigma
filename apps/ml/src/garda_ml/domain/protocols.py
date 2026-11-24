"""Structural capability protocols for pluggable estimators and explainers."""

from typing import Any, Protocol, runtime_checkable


@runtime_checkable
class QuantileForecaster(Protocol):
    """Interface for quantile price forecasting estimators."""

    model_key: str

    def fit(self, X: Any, y: Any, *, quantile: float = 0.5, sample_weight: Any = None) -> "QuantileForecaster": ...

    def predict(self, X: Any) -> Any: ...


@runtime_checkable
class RiskClassifier(Protocol):
    """Interface for calibrated binary risk classifiers."""

    model_key: str

    def fit(self, X: Any, y: Any, *, sample_weight: Any = None) -> "RiskClassifier": ...

    def predict_proba(self, X: Any) -> Any: ...


@runtime_checkable
class RegionClusterer(Protocol):
    """Interface for unsupervised regency clustering models."""

    def fit_predict(self, features: Any) -> dict[str, Any]: ...


@runtime_checkable
class FeatureExplainer(Protocol):
    """Interface for model-specific feature attribution strategies."""

    def explain(self, model: Any, X: Any, feature_names: list[str]) -> Any: ...
