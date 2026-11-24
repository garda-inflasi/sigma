"""Weather features enforcing strict t - 2 availability lag to avoid leakage."""

import re

import pandas as pd

from garda_ml.config.constants import WEATHER_AVAILABILITY_LAG_DAYS

_WEATHER_PATTERNS: dict[str, re.Pattern[str]] = {
    "hujan": re.compile(r"hujan|precip|rain"),
    "suhu": re.compile(r"suhu|temp"),
    "lembap": re.compile(r"lembab|lembap|humid"),
    "angin": re.compile(r"angin|wind"),
}


def build_weather_features(weather: pd.DataFrame, date_index: pd.DatetimeIndex) -> dict[str, pd.DataFrame]:
    """Compute lagged 7d and 30d weather rolling means per regency."""
    if weather.empty:
        return {}

    df = weather.copy()
    names = df["indikator"].astype(str).str.lower()
    df["metric_type"] = None
    for metric_type, pattern in _WEATHER_PATTERNS.items():
        df.loc[names.str.contains(pattern) & df["metric_type"].isna(), "metric_type"] = metric_type
    df = df.dropna(subset=["metric_type"])

    features_by_regency: dict[str, pd.DataFrame] = {}
    for regency_code, group in df.groupby("kabupaten_kode"):
        pivoted = group.pivot_table(
            index="tanggal",
            columns="metric_type",
            values="nilai",
            aggfunc="mean",
        ).reindex(date_index)

        cols: dict[str, pd.Series] = {}
        for metric_type in _WEATHER_PATTERNS:
            if metric_type not in pivoted:
                continue
            shifted = pivoted[metric_type].shift(WEATHER_AVAILABILITY_LAG_DAYS)
            cols[f"cuaca_{metric_type}_7"] = shifted.rolling(7, min_periods=2).mean()
            cols[f"cuaca_{metric_type}_30"] = shifted.rolling(30, min_periods=7).mean()
        features_by_regency[str(regency_code)] = pd.DataFrame(cols, index=date_index, dtype="float32")

    return features_by_regency
