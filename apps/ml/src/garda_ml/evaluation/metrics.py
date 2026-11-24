"""Forecasting and classification evaluation metrics."""

from typing import Any

import numpy as np


def price_pair_scores(pairs: list[tuple[float, float]]) -> dict[str, float]:
    """Compute price-scale RMSE and MAPE (%) from `(pred_log, actual_log)` tuples."""
    preds = np.exp(np.array([item[0] for item in pairs], dtype=float))
    actuals = np.exp(np.array([item[1] for item in pairs], dtype=float))
    return {
        "rmse": float(np.sqrt(np.mean((preds - actuals) ** 2))),
        "mape": float(np.mean(np.abs(preds - actuals) / actuals) * 100),
    }


def horizon_balanced_scores(scores_by_key: dict[Any, tuple[float, float]], keys: list[Any]) -> dict[str, float]:
    """Average RMSE and MAPE equally per horizon so short horizons do not dominate."""
    by_horizon: dict[int, list[tuple[float, float]]] = {}
    for key in keys:
        by_horizon.setdefault(int(key[-1]), []).append(scores_by_key[key])
    per_h = [price_pair_scores(pairs) for _, pairs in sorted(by_horizon.items())]
    return {
        "rmse": float(np.mean([item["rmse"] for item in per_h])),
        "mape": float(np.mean([item["mape"] for item in per_h])),
    }


def mean_fold_metric(fold_scores: dict[Any, dict[str, float]], folds: list[Any], metric: str = "mape") -> float:
    """Average a metric across the selected walk-forward folds."""
    return float(np.mean([fold_scores[f][metric] for f in folds]))


def pinball_loss(y_true: np.ndarray, y_pred: np.ndarray, quantile: float) -> float:
    """Compute pinball (quantile) loss on aligned predictions."""
    err = np.asarray(y_true, dtype=float) - np.asarray(y_pred, dtype=float)
    return float(np.mean(np.maximum(quantile * err, (quantile - 1.0) * err)))
