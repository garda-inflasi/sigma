"""ModelRegistry mapping model keys to factories, minimum point gates, and optional extras."""

from collections.abc import Callable
from dataclasses import dataclass
from typing import Any

from garda_ml.config.constants import (
    FORECAST_MIN_POINTS,
    GLOBAL_MIN_POINTS,
    GNN_MIN_POINTS,
    HOLT_WINTERS_MIN_POINTS,
    SARIMAX_MIN_POINTS,
)
from garda_ml.domain.enums import BoostingBackend, ModelFamily, TaskType
from garda_ml.models.backend_resolver import ResolvedBackend
from garda_ml.models.classification.gradient_boosting import (
    HistGradientBoostingRiskClassifier,
    LightGbmRiskClassifier,
)
from garda_ml.models.classification.logistic import CalibratedLogisticClassifier
from garda_ml.models.classification.random_forest import CalibratedRandomForestClassifier
from garda_ml.models.clustering.kmeans import RegencyKMeansClusterer
from garda_ml.models.forecasting.baselines import (
    NaiveEstimator,
    RidgeQuantileEstimator,
    SeasonalNaiveEstimator,
)
from garda_ml.models.forecasting.gradient_boosting import (
    HistGradientBoostingQuantileEstimator,
    LightGbmQuantileEstimator,
)
from garda_ml.models.forecasting.pytorch_gnn import SpatioTemporalGnnEstimator, is_torch_available
from garda_ml.models.forecasting.random_forest import RandomForestQuantileEstimator
from garda_ml.models.forecasting.statistical import HoltWintersEstimator, SarimaxEstimator


@dataclass(frozen=True)
class ModelRegistration:
    """Metadata and factory registration for a single model family."""

    model_key: str
    task_type: TaskType
    min_points: int
    factory: Callable[..., Any]
    extras_group: str = "tabular"
    is_baseline: bool = False


class ModelRegistry:
    """Central extensible registry for forecasting, classification, and clustering plugins."""

    def __init__(self) -> None:
        self._items: dict[str, ModelRegistration] = {}

    def register(self, spec: ModelRegistration) -> None:
        self._items[spec.model_key] = spec

    def get(self, model_key: str) -> ModelRegistration:
        if model_key not in self._items:
            raise KeyError(f"model tidak terdaftar: {model_key!r}")
        return self._items[model_key]

    def create(self, model_key: str, **kwargs: Any) -> Any:
        return self.get(model_key).factory(**kwargs)

    def eligible_forecast_keys(
        self,
        valid_point_count: int,
        backend: ResolvedBackend,
        *,
        include_deep: bool | None = None,
    ) -> list[str]:
        """Return candidate model keys eligible for a given observation depth and runtime."""
        deep_ready = is_torch_available() if include_deep is None else include_deep
        ordered = [
            ModelFamily.NAIVE,
            ModelFamily.SEASONAL_NAIVE,
            ModelFamily.RIDGE_QUANTILE,
            ModelFamily.HOLT_WINTERS,
            ModelFamily.RANDOM_FOREST_QUANTILE,
            ModelFamily.SARIMAX,
            backend.quantile_model_key,
            ModelFamily.PYTORCH_GNN,
        ]
        result: list[str] = []
        for key in ordered:
            reg = self._items.get(key)
            if reg is None or valid_point_count < reg.min_points:
                continue
            if reg.extras_group == "deep" and not deep_ready:
                continue
            if key in (ModelFamily.LIGHTGBM_QUANTILE, ModelFamily.HGB_QUANTILE):
                expected_key = (
                    ModelFamily.LIGHTGBM_QUANTILE
                    if backend.name == BoostingBackend.LIGHTGBM
                    else ModelFamily.HGB_QUANTILE
                )
                if key != expected_key:
                    continue
            result.append(key)
        return result


def build_default_model_registry() -> ModelRegistry:
    """Register all standard tabular, statistical, linear, ensemble, and optional deep models."""
    registry = ModelRegistry()
    for spec in (
        ModelRegistration(ModelFamily.NAIVE, TaskType.FORECASTING, FORECAST_MIN_POINTS, NaiveEstimator, is_baseline=True),
        ModelRegistration(ModelFamily.SEASONAL_NAIVE, TaskType.FORECASTING, FORECAST_MIN_POINTS, SeasonalNaiveEstimator, is_baseline=True),
        ModelRegistration(ModelFamily.RIDGE_QUANTILE, TaskType.FORECASTING, FORECAST_MIN_POINTS, RidgeQuantileEstimator, is_baseline=True),
        ModelRegistration(ModelFamily.HOLT_WINTERS, TaskType.FORECASTING, HOLT_WINTERS_MIN_POINTS, HoltWintersEstimator),
        ModelRegistration(ModelFamily.RANDOM_FOREST_QUANTILE, TaskType.FORECASTING, HOLT_WINTERS_MIN_POINTS, RandomForestQuantileEstimator),
        ModelRegistration(ModelFamily.SARIMAX, TaskType.FORECASTING, SARIMAX_MIN_POINTS, SarimaxEstimator),
        ModelRegistration(ModelFamily.LIGHTGBM_QUANTILE, TaskType.FORECASTING, GLOBAL_MIN_POINTS, LightGbmQuantileEstimator),
        ModelRegistration(ModelFamily.HGB_QUANTILE, TaskType.FORECASTING, GLOBAL_MIN_POINTS, HistGradientBoostingQuantileEstimator),
        ModelRegistration(ModelFamily.PYTORCH_GNN, TaskType.FORECASTING, GNN_MIN_POINTS, SpatioTemporalGnnEstimator, extras_group="deep"),
        ModelRegistration(ModelFamily.LOGISTIC_REGRESSION, TaskType.CLASSIFICATION, 12, CalibratedLogisticClassifier, is_baseline=True),
        ModelRegistration(ModelFamily.RANDOM_FOREST_CLASSIFIER, TaskType.CLASSIFICATION, 24, CalibratedRandomForestClassifier),
        ModelRegistration("lightgbm_classifier", TaskType.CLASSIFICATION, 30, LightGbmRiskClassifier),
        ModelRegistration("hgb_classifier", TaskType.CLASSIFICATION, 30, HistGradientBoostingRiskClassifier),
        ModelRegistration(ModelFamily.KMEANS, TaskType.CLUSTERING, 3, RegencyKMeansClusterer),
    ):
        registry.register(spec)
    return registry


DEFAULT_MODEL_REGISTRY = build_default_model_registry()
