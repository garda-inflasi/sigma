"""Unified FeaturePipelineBuilder composing price, climate, macro, and calendar features."""

import numpy as np
import pandas as pd

from garda_ml.config.constants import DEFAULT_LAGS, MAX_INFERENCE_STALE_DAYS
from garda_ml.domain.entities import FeaturePanel, PriceSeries, RawMarketDataset
from garda_ml.features.calendar_features import build_calendar_features
from garda_ml.features.macro_features import build_macro_features
from garda_ml.features.price_features import extract_price_series_features
from garda_ml.features.weather_features import build_weather_features


def build_feature_panel(
    dataset: RawMarketDataset,
    cutoff: pd.Timestamp,
    min_points: int = 2,
) -> FeaturePanel:
    """Construct a leakage-free daily feature matrix across all commodity x regency series."""
    cutoff_norm = pd.Timestamp(cutoff).normalize()
    if dataset.prices.empty:
        return FeaturePanel(pd.DataFrame(), [], {}, cutoff_norm)

    prices = dataset.prices[dataset.prices["tanggal"] <= cutoff_norm]
    if prices.empty:
        return FeaturePanel(pd.DataFrame(), [], {}, cutoff_norm)

    date_index = pd.date_range(prices["tanggal"].min(), cutoff_norm, freq="D")
    regencies = sorted(prices["kabupaten_kode"].astype(str).unique())

    weather_slice = dataset.weather[dataset.weather["tanggal"] <= cutoff_norm] if not dataset.weather.empty else dataset.weather
    weather_by_regency = build_weather_features(weather_slice, date_index)

    macro_slice = dataset.macro[dataset.macro["tanggal_rilis"] <= cutoff_norm] if not dataset.macro.empty else dataset.macro
    macro_by_regency = build_macro_features(macro_slice, regencies, date_index)

    weather_cols = sorted({col for frame in weather_by_regency.values() for col in frame.columns})
    macro_cols = sorted({col for frame in macro_by_regency.values() for col in frame.columns})
    calendar_df = build_calendar_features(date_index)

    commodity_ids = {k: idx for idx, k in enumerate(sorted(prices["komoditas_kode"].astype(str).unique()))}
    regency_ids = {w: idx for idx, w in enumerate(regencies)}

    series_map: dict[tuple[str, str], PriceSeries] = {}
    frames: list[pd.DataFrame] = []

    for (commodity, regency), group in prices.groupby(["komoditas_kode", "kabupaten_kode"], sort=True):
        commodity_str, regency_str = str(commodity), str(regency)
        if len(group) < min_points:
            continue

        latest_orig = dataset.latest_original_dates.get(
            (commodity_str, regency_str, str(group["sumber"].iloc[-1]))
        )
        series, frame = extract_price_series_features(
            commodity_str,
            regency_str,
            group,
            cutoff=cutoff_norm,
            latest_original_date=latest_orig,
        )
        series_map[(commodity_str, regency_str)] = series

        weather_df = weather_by_regency.get(regency_str)
        if weather_df is None:
            weather_df = weather_by_regency.get(regency_str[:2])
        if weather_df is not None:
            frame = frame.join(weather_df.reindex(frame.index))

        if regency_str in macro_by_regency:
            frame = frame.join(macro_by_regency[regency_str].reindex(frame.index))

        frame = frame.join(calendar_df.reindex(frame.index))

        keep_mask = (frame["umur_data"] <= MAX_INFERENCE_STALE_DAYS).to_numpy()
        keep_mask[-1] = True
        frame = frame.loc[keep_mask].copy()
        frame.insert(0, "tanggal", frame.index)
        frame.insert(0, "kabupaten_kode", regency_str)
        frame.insert(0, "komoditas_kode", commodity_str)
        frame["komoditas_id"] = np.int16(commodity_ids[commodity_str])
        frame["wilayah_id"] = np.int16(regency_ids[regency_str])
        frames.append(frame.reset_index(drop=True))

    if not frames:
        return FeaturePanel(pd.DataFrame(), [], {}, cutoff_norm)

    combined = pd.concat(frames, ignore_index=True)
    for col in [*weather_cols, *macro_cols]:
        if col not in combined:
            combined[col] = np.nan

    price_cols = (
        [f"lag_{lag}" for lag in DEFAULT_LAGS]
        + ["ma_7", "ma_30", "vol_7", "vol_30", "umur_data", "lapangan_selisih_7", "lapangan_bobot_7"]
    )
    feature_columns = price_cols + weather_cols + macro_cols + list(calendar_df.columns) + ["komoditas_id", "wilayah_id"]
    return FeaturePanel(frame=combined, feature_columns=feature_columns, series_map=series_map, cutoff=cutoff_norm)


class FeaturePipelineBuilder:
    """Stateless builder composing all causal feature extractors."""

    def __init__(self, min_points: int = 2) -> None:
        self._min_points = min_points

    def build(self, dataset: RawMarketDataset, cutoff: pd.Timestamp) -> FeaturePanel:
        """Assemble clean daily `FeaturePanel` up to `cutoff`."""
        return build_feature_panel(dataset, cutoff=cutoff, min_points=self._min_points)
