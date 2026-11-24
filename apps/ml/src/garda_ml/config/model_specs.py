"""Declarative hyperparameter grids and candidate specs per model family."""

from typing import Any

DEFAULT_BOOSTING_PARAMS: dict[str, float] = {
    "n_estimators": 200,
    "learning_rate": 0.04,
    "num_leaves": 31,
    "min_child_samples": 30,
}

BOOSTING_PARAM_GRID: tuple[dict[str, float], ...] = (
    {"n_estimators": 120, "learning_rate": 0.06, "num_leaves": 15, "min_child_samples": 40},
    DEFAULT_BOOSTING_PARAMS,
    {"n_estimators": 300, "learning_rate": 0.03, "num_leaves": 63, "min_child_samples": 20},
)

RIDGE_ALPHAS: tuple[float, ...] = (0.1, 1.0, 10.0)

RF_PARAM_DEFAULTS: dict[str, Any] = {
    "n_estimators": 150,
    "max_depth": 10,
    "min_samples_leaf": 10,
}

KMEANS_CLUSTER_RANGE: tuple[int, ...] = (2, 3, 4)
