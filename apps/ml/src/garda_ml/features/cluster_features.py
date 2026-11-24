"""Regency vulnerability profiling and K-Means cluster feature injection."""

import numpy as np
import pandas as pd

from garda_ml.domain.entities import FeaturePanel, RegionCluster


def summarize_regency_profiles(panel: FeaturePanel) -> pd.DataFrame:
    """Aggregate causal historical volatility, price momentum, and climate risk per regency."""
    if panel.is_empty:
        return pd.DataFrame(columns=["regency_code", "volatility_mean", "abs_momentum_30", "climate_exposure"])

    df = panel.frame
    agg: dict[str, pd.Series] = {}
    grouped = df.groupby("kabupaten_kode", sort=True)
    agg["volatility_mean"] = grouped["vol_30"].mean().fillna(0.0)
    agg["abs_momentum_30"] = df["lag_30"].abs().groupby(df["kabupaten_kode"], sort=True).mean().fillna(0.0)
    if "cuaca_hujan_30" in df.columns:
        agg["climate_exposure"] = grouped["cuaca_hujan_30"].mean().fillna(0.0)
    else:
        agg["climate_exposure"] = pd.Series(0.0, index=agg["volatility_mean"].index)

    profile_df = pd.DataFrame(agg).reset_index().rename(columns={"kabupaten_kode": "regency_code"})
    return profile_df.astype({"regency_code": str})


def inject_cluster_features(panel: FeaturePanel, clusters: dict[str, RegionCluster]) -> FeaturePanel:
    """Attach `cluster_id` and `cluster_distance` into `panel.frame` without adding future leakage."""
    if panel.is_empty or not clusters:
        return panel

    df = panel.frame.copy()
    df["cluster_id"] = (
        df["kabupaten_kode"]
        .map(lambda code: clusters[code].cluster_id if code in clusters else -1)
        .astype(np.float32)
    )
    df["cluster_distance"] = (
        df["kabupaten_kode"]
        .map(lambda code: clusters[code].centroid_distance if code in clusters else 0.0)
        .astype(np.float32)
    )

    feature_cols = list(panel.feature_columns)
    for col in ("cluster_id", "cluster_distance"):
        if col not in feature_cols:
            feature_cols.append(col)

    return FeaturePanel(
        frame=df,
        feature_columns=feature_cols,
        series_map=panel.series_map,
        cutoff=panel.cutoff,
        region_clusters=clusters,
    )
