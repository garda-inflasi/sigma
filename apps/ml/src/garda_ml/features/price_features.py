"""Commodity price log-return, momentum, volatility, field gap, and horizon labels."""

import numpy as np
import pandas as pd

from garda_ml.config.constants import DEFAULT_HORIZONS, DEFAULT_LAGS, MAX_LABEL_STALE_DAYS
from garda_ml.domain.entities import PriceSeries
from garda_ml.features.alignment import align_series_daily_grid


def extract_price_series_features(
    commodity_code: str,
    regency_code: str,
    group: pd.DataFrame,
    cutoff: pd.Timestamp,
    latest_original_date: pd.Timestamp | None,
) -> tuple[PriceSeries, pd.DataFrame]:
    """Build causal price features and forward target labels for a single series."""
    idx, log_series, is_original, weights, ages, by_day = align_series_daily_grid(
        group,
        cutoff=cutoff,
        latest_original_date=latest_original_date,
    )

    columns: dict[str, np.ndarray | pd.Series] = {}
    for lag in DEFAULT_LAGS:
        columns[f"lag_{lag}"] = log_series - log_series.shift(lag)
    columns["ma_7"] = log_series - log_series.rolling(7, min_periods=3).mean()
    columns["ma_30"] = log_series - log_series.rolling(30, min_periods=7).mean()

    daily_ret = log_series.diff()
    columns["vol_7"] = daily_ret.rolling(7, min_periods=3).std()
    columns["vol_30"] = daily_ret.rolling(30, min_periods=7).std()
    columns["umur_data"] = ages

    field_price = by_day["nilai_lapangan"].reindex(idx).astype(float)
    field_weight = by_day["bobot_lapangan"].reindex(idx).astype(float)
    columns["lapangan_selisih_7"] = np.log(field_price).rolling(7, min_periods=1).mean() - log_series
    columns["lapangan_bobot_7"] = field_weight.rolling(7, min_periods=1).mean()

    frame = pd.DataFrame(columns, index=idx).astype("float32")
    weight_series = pd.Series(weights, index=idx)
    age_series = pd.Series(ages, index=idx)

    for horizon in DEFAULT_HORIZONS:
        future_age = age_series.shift(-horizon)
        valid_label = (future_age <= MAX_LABEL_STALE_DAYS) & (future_age < horizon)
        frame[f"target_h{horizon}"] = (log_series.shift(-horizon) - log_series).where(valid_label).astype("float32")
        frame[f"bobot_h{horizon}"] = np.minimum(weight_series, weight_series.shift(-horizon)).astype("float32")

    unit = str(group["satuan"].dropna().iloc[-1]) if group["satuan"].notna().any() else "Rp/kg"
    series = PriceSeries(
        commodity_code=commodity_code,
        regency_code=regency_code,
        dates=idx,
        log_prices=log_series.to_numpy(float),
        is_original=is_original,
        weights=weights,
        ages=ages,
        unit=unit,
        source_code=str(group["sumber"].iloc[-1]),
    )
    return series, frame
