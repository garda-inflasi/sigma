"""Daily business-date grid alignment and forward-fill staleness tracking."""

import numpy as np
import pandas as pd


def align_series_daily_grid(
    group: pd.DataFrame,
    cutoff: pd.Timestamp,
    latest_original_date: pd.Timestamp | None = None,
) -> tuple[pd.DatetimeIndex, pd.Series, np.ndarray, np.ndarray, np.ndarray, pd.Series]:
    """Reindex irregular observations onto a dense daily grid up to `cutoff` using causal ffill."""
    idx = pd.date_range(group["tanggal"].min(), cutoff, freq="D")
    by_day = group.set_index("tanggal")
    raw_values = by_day["nilai"].reindex(idx)
    is_original = raw_values.notna().to_numpy()
    log_series = np.log(raw_values.astype(float)).ffill()

    positions = np.arange(len(idx))
    last_pos = np.maximum.accumulate(np.where(is_original, positions, -1))
    ages = (positions - last_pos).astype(float)

    if latest_original_date is not None and not pd.isna(latest_original_date):
        days_since_lad = (idx - latest_original_date.normalize()).days.to_numpy(dtype=float)
        ages = np.where(days_since_lad >= 0, np.minimum(ages, days_since_lad), ages)

    weights = by_day["bobot"].reindex(idx).ffill().to_numpy(float)
    return idx, log_series, is_original, weights, ages, by_day
