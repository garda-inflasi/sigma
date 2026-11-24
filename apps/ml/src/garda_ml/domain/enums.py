"""Domain enumerations shared across models, evaluation, and datasafe outputs."""

from enum import StrEnum


class ModelFamily(StrEnum):
    """Canonical model keys recorded in `datasafe_forecasts` and `model_metrics`."""

    NAIVE = "naive"
    SEASONAL_NAIVE = "seasonal_naive"
    HOLT_WINTERS = "holt_winters"
    SARIMAX = "sarimax"
    RIDGE_QUANTILE = "ridge_quantile"
    RANDOM_FOREST_QUANTILE = "random_forest_quantile"
    LIGHTGBM_QUANTILE = "lightgbm_quantile"
    HGB_QUANTILE = "hgb_quantile"
    PYTORCH_GNN = "pytorch_gnn"
    LOGISTIC_REGRESSION = "logistic_regression"
    RANDOM_FOREST_CLASSIFIER = "random_forest_classifier"
    KMEANS = "kmeans"


class BoostingBackend(StrEnum):
    """Active tree-boosting engine resolved at runtime."""

    LIGHTGBM = "lightgbm"
    HIST_GRADIENT_BOOSTING = "hist_gradient_boosting"


class TaskType(StrEnum):
    """ML pipeline task category."""

    FORECASTING = "forecasting"
    CLASSIFICATION = "classification"
    CLUSTERING = "clustering"


class CycleStatus(StrEnum):
    """Atomic training cycle lifecycle status stored in `ml_training_cycles`."""

    RUNNING = "BERJALAN"
    SUCCEEDED = "BERHASIL"
    PARTIAL = "SEBAGIAN"
    FAILED = "GAGAL"


class SeverityLevel(StrEnum):
    """Early Warning System severity tiers."""

    CRITICAL = "KRITIS"
    ALERT = "SIAGA"
    WATCH = "WASPADA"
    INFO = "INFO"


class EligibilityStatus(StrEnum):
    """Champion eligibility status for holdout model publication."""

    ELIGIBLE = "ELIGIBLE"
    BELOW_AUC = "BELOW_AUC"
    WORSE_THAN_BASELINE = "WORSE_THAN_BASELINE"
    INSUFFICIENT_FOLDS = "INSUFFICIENT_FOLDS"
