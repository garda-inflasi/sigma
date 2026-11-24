"""Seasonal robust Z-score & Isolation Forest anomaly detector."""

from typing import Any

import numpy as np
import pandas as pd
from sklearn.ensemble import IsolationForest

from garda_ml.domain.entities import PriceSeries
from garda_ml.domain.enums import SeverityLevel

MAX_ANOMALY_AGE_DAYS = 3
MIN_ANOMALY_WEIGHT = 0.8
ROBUST_WINDOW_DAYS = 180
MIN_ANOMALY_POINTS = 45
ANOMALY_FEATURE_COLS = ["dev", "ret_1", "ret_7", "rasio_vol"]


def compute_anomaly_features(series: PriceSeries) -> pd.DataFrame:
    """Compute seasonal log-deviation and trailing robust Z-score per day."""
    log_p = pd.Series(series.log_prices, index=series.dates)
    seasonal_ref = pd.concat([log_p.shift(7 * k) for k in range(1, 9)], axis=1).median(axis=1, skipna=True)
    dev = log_p - seasonal_ref

    trailing = dev.shift(1).rolling(ROBUST_WINDOW_DAYS, min_periods=30)
    median = trailing.median()
    iqr = trailing.quantile(0.75) - trailing.quantile(0.25)
    scale = (iqr / 1.349).clip(lower=0.005)

    ret_1 = log_p.diff()
    vol_7 = ret_1.rolling(7, min_periods=3).std()
    vol_90 = ret_1.shift(1).rolling(90, min_periods=20).std().clip(lower=1e-4)
    return pd.DataFrame(
        {
            "dev": dev,
            "z": (dev - median) / scale,
            "ret_1": ret_1,
            "ret_7": log_p - log_p.shift(7),
            "rasio_vol": vol_7 / vol_90,
        }
    )


def classify_anomaly_severity(z_score: float, dev_pct: float, is_if_outlier: bool) -> str | None:
    """Map Z-score, deviation percentage, and IsolationForest flag into EWS severity."""
    az, ad = abs(z_score), abs(dev_pct)
    if (az >= 3.5 and ad >= 25) or (az >= 6 and ad >= 15):
        return SeverityLevel.CRITICAL
    if az >= 3.5 and ad >= 10:
        return SeverityLevel.ALERT
    if (az >= 2.5 and ad >= 5) or (is_if_outlier and az >= 2 and ad >= 5):
        return SeverityLevel.WATCH
    return None


def detect_price_anomalies(
    series_map: dict[tuple[str, str], PriceSeries],
    commodity_names: dict[str, str],
    max_if_rows: int = 50_000,
) -> list[dict[str, Any]]:
    """Detect active seasonal and multivariate price anomalies across verified series."""
    candidates: list[tuple[PriceSeries, pd.Series]] = []
    history_blocks: list[pd.DataFrame] = []

    for series in series_map.values():
        if series.valid_point_count < MIN_ANOMALY_POINTS or series.latest_age_days > MAX_ANOMALY_AGE_DAYS:
            continue
        if series.weights[series.last_original_idx] < MIN_ANOMALY_WEIGHT:
            continue
        feats = compute_anomaly_features(series)
        latest = feats.iloc[series.last_original_idx]
        if not np.isfinite(latest["z"]) or not np.isfinite(latest["dev"]):
            continue
        candidates.append((series, latest))
        hist = feats.iloc[: series.last_original_idx][ANOMALY_FEATURE_COLS].replace([np.inf, -np.inf], np.nan).dropna()
        history_blocks.append(hist.iloc[-ROBUST_WINDOW_DAYS:])

    if not candidates:
        return []

    is_outlier = np.zeros(len(candidates), dtype=bool)
    merged_hist = pd.concat(history_blocks) if history_blocks else pd.DataFrame()
    if len(merged_hist) >= 200:
        if len(merged_hist) > max_if_rows:
            merged_hist = merged_hist.sample(max_if_rows, random_state=42)
        forest = IsolationForest(n_estimators=100, contamination=0.01, random_state=42, n_jobs=1).fit(
            merged_hist.to_numpy()
        )
        test_mat = (
            pd.DataFrame([row[ANOMALY_FEATURE_COLS] for _, row in candidates])
            .replace([np.inf, -np.inf], np.nan)
            .fillna(0.0)
            .to_numpy()
        )
        is_outlier = forest.predict(test_mat) == -1

    signals: list[dict[str, Any]] = []
    for (series, latest), outlier_flag in zip(candidates, is_outlier):
        dev_pct = float(np.expm1(latest["dev"]) * 100)
        severity = classify_anomaly_severity(float(latest["z"]), dev_pct, bool(outlier_flag))
        if severity is None:
            continue

        direction = "NAIK" if dev_pct > 0 else "TURUN"
        k, w = series.commodity_code, series.regency_code
        method = "zscore_musiman+isolation_forest" if outlier_flag else "zscore_musiman"
        threshold = 25.0 if severity == SeverityLevel.CRITICAL else 10.0 if severity == SeverityLevel.ALERT else 5.0
        label = commodity_names.get(k, k)
        signals.append(
            {
                "sinyal_kunci": f"ANOMALI_HARGA:{k}:{w}:{direction}",
                "severity": severity,
                "jenis": "ANOMALI_HARGA",
                "komoditas_kode": k,
                "kabupaten_kode": w,
                "nilai": round(dev_pct, 3),
                "ambang": threshold,
                "metode": method,
                "judul": f"Anomali harga {label} ({direction.lower()})",
                "pesan": (
                    f"Harga {label} di wilayah {w} pada {series.latest_date.date()} "
                    f"menyimpang {dev_pct:+.1f}% dari pola musimannya (z={float(latest['z']):.1f})."
                ),
                "aktif": True,
            }
        )
    return signals
