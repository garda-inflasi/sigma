"""Leakage-proof expanding walk-forward and embargoed time-series cross-validation splits."""

import numpy as np
import pandas as pd

from garda_ml.config.constants import (
    MAX_LABEL_STALE_DAYS,
    N_HOLDOUT_FOLDS,
    N_ORIGINS_PER_FOLD,
    N_SELECTION_FOLDS,
    ORIGIN_STEP_DAYS,
    SELECTION_STEP_DAYS,
)
from garda_ml.domain.entities import PriceSeries


def build_walk_forward_folds(
    end_ts: pd.Timestamp,
    n_folds: int = N_SELECTION_FOLDS + N_HOLDOUT_FOLDS,
    step_days: int = SELECTION_STEP_DAYS,
    n_origins: int = N_ORIGINS_PER_FOLD,
    origin_step_days: int = ORIGIN_STEP_DAYS,
) -> list[list[pd.Timestamp]]:
    """Build non-overlapping walk-forward origin blocks moving chronologically to `end_ts`."""
    starts = [end_ts - pd.Timedelta(days=step_days * k) for k in range(n_folds, 0, -1)]
    return [[start + pd.Timedelta(days=origin_step_days * j) for j in range(n_origins)] for start in starts]


def fold_boundaries(
    folds: list[list[pd.Timestamp]],
) -> list[tuple[int, list[pd.Timestamp], pd.Timestamp | None]]:
    """Pair each fold block with the upper target boundary so folds never share future targets."""
    return [
        (idx, block, folds[idx + 1][0] if idx + 1 < len(folds) else None)
        for idx, block in enumerate(folds)
    ]


def actual_log_price(series: PriceSeries, pos: int, horizon: int) -> float | None:
    """Return observed log price at `pos + horizon` only when supported by a fresh genuine point."""
    target_idx = pos + horizon
    if target_idx >= len(series.log_prices):
        return None
    age = series.ages[target_idx]
    if age > MAX_LABEL_STALE_DAYS or age >= horizon:
        return None
    return float(series.log_prices[target_idx])


def embargo_time_folds(
    dates: np.ndarray,
    n_folds: int,
    embargo_days: int,
    y: np.ndarray | None = None,
) -> list[tuple[np.ndarray, np.ndarray]]:
    """Build forward time splits separated by at least `embargo_days` between train end and test start."""
    unique_dates = np.unique(dates)
    block_size = max(1, len(unique_dates) // (n_folds + 1))
    folds: list[tuple[np.ndarray, np.ndarray]] = []
    embargo = np.timedelta64(embargo_days, "D")

    for idx in range(n_folds):
        start_pos = len(unique_dates) - (n_folds - idx) * block_size
        if start_pos <= 0:
            continue
        fold_start = unique_dates[start_pos]
        fold_end = unique_dates[min(start_pos + block_size - 1, len(unique_dates) - 1)]
        train_idx = np.flatnonzero(dates < fold_start - embargo)
        test_idx = np.flatnonzero((dates >= fold_start) & (dates <= fold_end))
        if len(train_idx) == 0 or len(test_idx) == 0:
            continue
        if y is not None and (len(np.unique(y[train_idx])) < 2 or len(np.unique(y[test_idx])) < 2):
            continue
        folds.append((train_idx, test_idx))

    return folds
