"""Out-of-time calibration, eligibility gates, and inflation/pressure risk classification."""

import logging
import re
import warnings
from dataclasses import dataclass, field
from typing import Any

import numpy as np
import pandas as pd
from sklearn.calibration import CalibratedClassifierCV
from sklearn.metrics import brier_score_loss, f1_score, roc_auc_score

from garda_ml.config.constants import (
    CLASSIFICATION_INDEX_HORIZON_DAYS,
    CLASSIFICATION_MIN_AUX_ROWS,
    CLASSIFICATION_MIN_OFFICIAL_MONTHS,
    CLASSIFICATION_OFFICIAL_EMBARGO_DAYS,
    MAX_INFERENCE_STALE_DAYS,
    MIN_ROC_AUC_ELIGIBLE,
    PRESSURE_THRESHOLD,
    SURPLUS_THRESHOLD,
)
from garda_ml.domain.entities import FeaturePanel, RawMarketDataset
from garda_ml.evaluation.walk_forward import embargo_time_folds
from garda_ml.models.backend_resolver import ResolvedBackend
from garda_ml.models.classification.logistic import build_logistic_classifier_estimator

logger = logging.getLogger("garda_ml.pipeline")

SAMPLE_WEIGHT_COL = "_sample_weight"
_DROP_FEATURE_COLS = ("komoditas_id", "cluster_id", "cluster_distance", SAMPLE_WEIGHT_COL)
MIN_LOGISTIC_ROWS = 12
MIN_BOOSTING_ROWS = 30


@dataclass
class TrainedRiskModel:
    """Calibrated classifier artifact bound to one classification target (`RESMI_BPS` or `INDEKS_PANGAN`)."""

    target: str
    model: Any
    columns: list[str]
    base_rate: float
    calibration_method: str
    model_name: str
    metrics: dict[str, Any]


@dataclass
class ClassificationSummary:
    """Aggregated outputs of Stage-3 risk classification."""

    signals: list[dict[str, Any]] = field(default_factory=list)
    metrics: dict[str, Any] = field(default_factory=dict)
    models: dict[str, TrainedRiskModel] = field(default_factory=dict)
    inference_rows: dict[str, tuple[str, np.ndarray]] = field(default_factory=dict)
    cycle_target: str = "INDEKS_PANGAN"
    notes: list[str] = field(default_factory=list)


def compute_pressure_score(probability: float, base_rate: float) -> float:
    """Map calibrated probability [0..1] into [0..100] anchored at 50 == historical base_rate."""
    b = float(np.clip(base_rate, 0.01, 0.99))
    score = 50 + 50 * (probability - b) / (1 - b) if probability >= b else 50 * probability / b
    return float(np.clip(score, 0, 100))


def evaluate_classification_eligibility(metrics: dict[str, Any]) -> tuple[bool, str]:
    """Gate publication strictly against ROC-AUC >= 0.60 and Brier < base-rate baseline."""
    auc, brier, baseline = metrics.get("roc_auc"), metrics.get("brier"), metrics.get("brier_baseline")
    if auc is None or brier is None or baseline is None:
        return False, "holdout tidak cukup untuk menilai model"
    reasons: list[str] = []
    if auc < MIN_ROC_AUC_ELIGIBLE:
        reasons.append(f"ROC-AUC holdout {auc:.2f} < {MIN_ROC_AUC_ELIGIBLE:.2f}")
    if not brier < baseline:
        reasons.append(f"Brier {brier:.3f} tidak lebih rendah dari baseline laju dasar {baseline:.3f}")
    if reasons:
        return False, "; ".join(reasons)
    return True, f"ROC-AUC holdout {auc:.2f} >= {MIN_ROC_AUC_ELIGIBLE:.2f} dan Brier {brier:.3f} < baseline {baseline:.3f}"


def determine_cycle_phase(score: float, momentum_30d: float | None) -> str:
    """Classify regency inflation cycle into TEKANAN / SURPLUS / NORMAL."""
    if score >= PRESSURE_THRESHOLD:
        return "TEKANAN"
    if score <= SURPLUS_THRESHOLD and (momentum_30d is None or np.isnan(momentum_30d) or momentum_30d <= 0):
        return "SURPLUS"
    return "NORMAL"


def _aggregate_regency_panel(panel: FeaturePanel) -> dict[str, pd.DataFrame]:
    if panel.is_empty:
        return {}
    numeric_cols = [c for c in panel.feature_columns if c not in ("komoditas_id", "wilayah_id", "cluster_id", "cluster_distance")]
    fresh = panel.frame["umur_data"] <= MAX_INFERENCE_STALE_DAYS
    df = panel.frame.loc[fresh, ["kabupaten_kode", "tanggal", "bobot_h30", *numeric_cols]]
    if df.empty:
        return {}

    def _agg(group: pd.DataFrame) -> pd.DataFrame:
        by_day = group.groupby("tanggal", sort=True)
        mean_df = by_day[numeric_cols].mean()
        mean_df["luas_kenaikan_30"] = (group["lag_30"] > 0).groupby(group["tanggal"]).mean().astype("float32")
        mean_df[SAMPLE_WEIGHT_COL] = by_day["bobot_h30"].mean()
        idx = pd.date_range(mean_df.index.min(), mean_df.index.max(), freq="D")
        mean_df = mean_df.reindex(idx)
        mean_df["indeks_log"] = mean_df["lag_1"].fillna(0).cumsum().astype("float64")
        mean_df["indeks_ma7"] = mean_df["indeks_log"].rolling(7, min_periods=3).mean()
        mean_df["indeks_mom_30"] = mean_df["indeks_log"] - mean_df["indeks_log"].shift(30)
        return mean_df

    out: dict[str, pd.DataFrame] = {}
    codes = sorted(df["kabupaten_kode"].unique())
    for regency_code, group in df.groupby("kabupaten_kode", sort=True):
        out[str(regency_code)] = _agg(group)
    for province_code in sorted({code[:2] for code in codes if len(code) > 2} - set(codes)):
        out[province_code] = _agg(df[df["kabupaten_kode"].str.startswith(province_code)])
    return out


def extract_official_inflation_series(macro: pd.DataFrame) -> dict[str, pd.DataFrame]:
    """Extract monthly BPS official `mtm`, `yoy`, `ytd` tables per region."""
    if macro.empty:
        return {}
    df = macro[(macro["domain"] == "INFLASI") & macro["wilayah_kode"].notna()].copy()
    if df.empty:
        return {}

    def _classify_indicator(name: str) -> str | None:
        lowered = name.lower()
        if re.search(r"mtm|m-to-m|month_to_month|bulanan", lowered):
            return "mtm"
        if re.search(r"yoy|y-on-y|year_on_year|tahunan", lowered):
            return "yoy"
        if re.search(r"ytd|y-to-d|year_to_date|kalender", lowered):
            return "ytd"
        if re.search(r"\bihk\b|indeks.harga.konsumen", lowered):
            return "ihk"
        return None

    df["kind"] = df["indikator"].astype(str).map(_classify_indicator)
    df = df.dropna(subset=["kind"])
    out: dict[str, pd.DataFrame] = {}

    for regency_code, group in df.groupby("wilayah_kode"):
        group = group.assign(month=group["periode"].dt.to_period("M").dt.to_timestamp())
        table = group.pivot_table(index="month", columns="kind", values="nilai", aggfunc="last").sort_index()
        if "mtm" not in table and "ihk" in table:
            month_gap = table.index.to_series().dt.to_period("M").astype("int64").diff()
            table["mtm"] = (table["ihk"] / table["ihk"].shift(1) - 1).where(month_gap == 1) * 100
        if "mtm" not in table:
            continue
        release_series = group[group["kind"].isin(["mtm", "ihk"])].groupby("month")["tanggal_rilis"].max()
        table["tanggal_rilis"] = release_series.reindex(table.index)
        for optional_col in ("yoy", "ytd"):
            if optional_col not in table:
                table[optional_col] = np.nan
        out[str(regency_code)] = table

    return out


def _select_base_estimator(backend: ResolvedBackend, n_samples: int) -> tuple[Any, str]:
    # Use small-sample logistic regression below MIN_BOOSTING_ROWS, otherwise primary tree boosting.
    if n_samples < MIN_BOOSTING_ROWS:
        return build_logistic_classifier_estimator(), "logistic_regression"
    return backend.make_classifier(), backend.name


def fit_calibrated_classifier(
    base_estimator: Any,
    X: np.ndarray,
    y: np.ndarray,
    sample_weight: np.ndarray,
    dates: np.ndarray,
    embargo_days: int,
) -> tuple[Any, str]:
    """Fit probability-calibrated classifier using embargoed time splits when sufficient folds exist."""
    folds = embargo_time_folds(dates, 3, embargo_days, y)
    if len(folds) >= 2:
        method = "isotonic" if len(y) >= 1000 else "sigmoid"
        with warnings.catch_warnings():
            warnings.simplefilter("ignore")
            calibrated = CalibratedClassifierCV(base_estimator, method=method, cv=folds, ensemble=True)
            calibrated.fit(X, y, sample_weight=sample_weight)
        return calibrated, method

    with warnings.catch_warnings():
        warnings.simplefilter("ignore")
        base_estimator.fit(X, y, sample_weight=sample_weight)
    return base_estimator, "tanpa_kalibrasi"


def evaluate_classification_oot(
    backend: ResolvedBackend,
    X: np.ndarray,
    y: np.ndarray,
    sample_weight: np.ndarray,
    dates: np.ndarray,
    embargo_days: int,
) -> dict[str, Any]:
    """Evaluate classifier on final 20% out-of-time window with embargo barrier."""
    unique_dates = np.unique(dates)
    if len(unique_dates) < 10:
        return {"f1": None, "roc_auc": None, "brier": None, "brier_baseline": None, "n_uji": 0}

    split_ts = unique_dates[int(len(unique_dates) * 0.8)]
    train_mask = dates < split_ts - np.timedelta64(embargo_days, "D")
    test_mask = dates >= split_ts

    if train_mask.sum() < 10 or test_mask.sum() == 0 or len(np.unique(y[train_mask])) < 2:
        return {"f1": None, "roc_auc": None, "brier": None, "brier_baseline": None, "n_uji": int(test_mask.sum())}

    estimator, _ = _select_base_estimator(backend, int(train_mask.sum()))
    model, _ = fit_calibrated_classifier(
        estimator,
        X[train_mask],
        y[train_mask],
        sample_weight[train_mask],
        dates[train_mask],
        embargo_days,
    )
    probs = model.predict_proba(X[test_mask])[:, 1]
    preds = (probs >= 0.5).astype(int)
    auc = float(roc_auc_score(y[test_mask], probs)) if len(np.unique(y[test_mask])) == 2 else None
    train_base_rate = float(y[train_mask].mean())
    return {
        "f1": float(f1_score(y[test_mask], preds, zero_division=0)),
        "roc_auc": auc,
        "brier": float(brier_score_loss(y[test_mask], probs)),
        "brier_baseline": float(np.mean((y[test_mask] - train_base_rate) ** 2)),
        "n_uji": int(test_mask.sum()),
    }


def _build_index_rows(regency_code: str, frame: pd.DataFrame, columns: list[str]) -> pd.DataFrame:
    end_ts = frame.index.max()
    weekly_mask = ((end_ts - frame.index).days % 7) == 0
    future_ma7 = frame["indeks_ma7"].shift(-CLASSIFICATION_INDEX_HORIZON_DAYS)
    label = (future_ma7 > frame["indeks_log"]).astype(float).where(future_ma7.notna() & frame["indeks_log"].notna())

    rows = frame.loc[weekly_mask, columns].copy()
    rows["label"] = label[weekly_mask]
    rows[SAMPLE_WEIGHT_COL] = frame.loc[weekly_mask, SAMPLE_WEIGHT_COL].fillna(1.0)
    rows["wilayah_kode"] = regency_code
    rows["tanggal"] = rows.index
    return rows.reset_index(drop=True)


def _build_official_rows(
    regency_code: str,
    frame: pd.DataFrame,
    official_df: pd.DataFrame,
    columns: list[str],
    cutoff: pd.Timestamp,
) -> pd.DataFrame:
    records: list[dict[str, Any]] = []
    for month_ts, row in official_df.iterrows():
        release = row["tanggal_rilis"]
        if pd.isna(row["mtm"]) or pd.isna(release) or release > cutoff:
            continue
        prev_month_end = month_ts - pd.Timedelta(days=1)
        if prev_month_end not in frame.index:
            continue
        rec = frame.loc[prev_month_end, columns].to_dict()
        rec.update(
            {
                "label": float(row["mtm"] > 0),
                "wilayah_kode": regency_code,
                "tanggal": prev_month_end,
                SAMPLE_WEIGHT_COL: 1.0,
            }
        )
        records.append(rec)
    return pd.DataFrame(records)


def _train_risk_target(
    target: str,
    rows: pd.DataFrame,
    columns: list[str],
    backend: ResolvedBackend,
    embargo_days: int,
) -> TrainedRiskModel | None:
    train_df = rows[rows["label"].notna()].sort_values("tanggal")
    if len(train_df) < MIN_LOGISTIC_ROWS or train_df["label"].nunique() < 2:
        return None

    X = train_df[columns].to_numpy(dtype=np.float32)
    y = train_df["label"].to_numpy(dtype=int)
    w = train_df[SAMPLE_WEIGHT_COL].fillna(1.0).to_numpy(dtype=float)
    dates = train_df["tanggal"].to_numpy("datetime64[ns]")

    metrics = evaluate_classification_oot(backend, X, y, w, dates, embargo_days)
    metrics["layak"], metrics["alasan"] = evaluate_classification_eligibility(metrics)

    base_estimator, used_model_name = _select_base_estimator(backend, len(train_df))
    model, cal_method = fit_calibrated_classifier(base_estimator, X, y, w, dates, embargo_days)
    return TrainedRiskModel(
        target=target,
        model=model,
        columns=columns,
        base_rate=float(y.mean()),
        calibration_method=cal_method,
        model_name=used_model_name,
        metrics=metrics,
    )


def _top_commodity_contributors(
    panel: FeaturePanel,
    regency_code: str,
    commodity_names: dict[str, str],
    limit: int = 5,
) -> list[dict[str, Any]]:
    rows = panel.frame[panel.frame["tanggal"] == panel.cutoff]
    if len(regency_code) > 2:
        rows = rows[rows["kabupaten_kode"] == regency_code]
    else:
        rows = rows[rows["kabupaten_kode"].str.startswith(regency_code)]
    rows = rows[rows["umur_data"] <= MAX_INFERENCE_STALE_DAYS].dropna(subset=["lag_30"])
    if rows.empty:
        return []

    by_commodity = (np.expm1(rows["lag_30"].astype(float)) * 100).groupby(rows["komoditas_kode"]).mean()
    contribution = by_commodity / len(by_commodity)
    ordered = contribution.reindex(contribution.abs().sort_values(ascending=False).index).head(limit)
    return [
        {"komoditas_kode": code, "nama": commodity_names.get(code, code), "andil": round(float(val), 4)}
        for code, val in ordered.items()
    ]


def _actual_index_stats(frame: pd.DataFrame) -> dict[str, Any]:
    series = frame["indeks_log"].dropna()
    if series.empty:
        return {"yoy": None, "mtm": None, "ytd": None, "periode": None}
    end_ts = series.index[-1]

    def _pct_change_since(ts: pd.Timestamp) -> float | None:
        if ts < series.index[0]:
            return None
        return round(float(np.expm1(series.iloc[-1] - series.asof(ts)) * 100), 4)

    return {
        "yoy": _pct_change_since(end_ts - pd.DateOffset(years=1)),
        "mtm": _pct_change_since(end_ts - pd.Timedelta(days=30)),
        "ytd": _pct_change_since(pd.Timestamp(end_ts.year - 1, 12, 31)),
        "periode": end_ts,
    }


def _actual_official_stats(official_df: pd.DataFrame, cutoff: pd.Timestamp) -> dict[str, Any]:
    published = official_df[official_df["tanggal_rilis"] <= cutoff].dropna(subset=["mtm"])
    if published.empty:
        return {"yoy": None, "mtm": None, "ytd": None, "periode": None}
    last = published.iloc[-1]
    return {
        "yoy": None if pd.isna(last.get("yoy")) else float(last["yoy"]),
        "mtm": None if pd.isna(last.get("mtm")) else float(last["mtm"]),
        "ytd": None if pd.isna(last.get("ytd")) else float(last["ytd"]),
        "periode": published.index[-1],
    }


def run_risk_classification(
    panel: FeaturePanel,
    dataset: RawMarketDataset,
    backend: ResolvedBackend,
    projected_index: dict[str, float] | None = None,
) -> ClassificationSummary:
    """Train calibrated regional risk models (`RESMI_BPS` / `INDEKS_PANGAN`) and emit signals."""
    summary = ClassificationSummary()
    regencies = _aggregate_regency_panel(panel)
    if not regencies:
        summary.metrics = {
            "model": None,
            "f1": None,
            "roc_auc": None,
            "brier": None,
            "n_uji": 0,
            "alasan": "panel harga kosong",
        }
        summary.notes.append("klasifikasi dilewati: tidak ada data harga")
        return summary

    sample_frame = next(iter(regencies.values()))
    drop_set = {"indeks_log", "indeks_ma7", *_DROP_FEATURE_COLS}
    feature_cols = [c for c in sample_frame.columns if c not in drop_set]
    cutoff = panel.cutoff

    official_series = extract_official_inflation_series(dataset.macro)
    official_regions = {
        w: tbl
        for w, tbl in official_series.items()
        if w in regencies
        and int(tbl.loc[tbl["tanggal_rilis"] <= cutoff, "mtm"].notna().sum()) >= CLASSIFICATION_MIN_OFFICIAL_MONTHS
    }

    official_parts = [
        _build_official_rows(w, regencies[w], tbl, feature_cols, cutoff)
        for w, tbl in official_regions.items()
    ]
    official_parts = [part for part in official_parts if not part.empty]
    official_model = None
    if official_parts:
        official_model = _train_risk_target(
            "RESMI_BPS",
            pd.concat(official_parts, ignore_index=True),
            feature_cols,
            backend,
            CLASSIFICATION_OFFICIAL_EMBARGO_DAYS,
        )
        if official_model is None:
            summary.notes.append("inflasi BPS ada tetapi tidak cukup untuk melatih model")
    if official_model is None:
        official_regions = {}

    direct_regions = set(panel.frame["kabupaten_kode"].unique())
    remaining = [w for w in regencies if w not in official_regions and w in direct_regions]
    index_model = None
    if remaining:
        index_df = pd.concat([_build_index_rows(w, regencies[w], feature_cols) for w in remaining], ignore_index=True)
        if int(index_df["label"].notna().sum()) >= CLASSIFICATION_MIN_AUX_ROWS:
            index_model = _train_risk_target(
                "INDEKS_PANGAN",
                index_df,
                feature_cols,
                backend,
                CLASSIFICATION_INDEX_HORIZON_DAYS,
            )
        if index_model is None:
            summary.notes.append("indeks pangan belum cukup untuk melatih model klasifikasi")

    if official_model:
        summary.models["RESMI_BPS"] = official_model
    if index_model:
        summary.models["INDEKS_PANGAN"] = index_model

    for regency_code, frame in regencies.items():
        if regency_code not in official_regions and regency_code not in direct_regions:
            continue
        target = "RESMI_BPS" if regency_code in official_regions else "INDEKS_PANGAN"
        trained = summary.models.get(target)
        if trained is None or cutoff not in frame.index:
            continue

        x = frame.loc[[cutoff], feature_cols].to_numpy(dtype=np.float32)
        prob = float(trained.model.predict_proba(x)[0, 1])
        score = compute_pressure_score(prob, trained.base_rate)
        momentum = frame["indeks_mom_30"].iloc[-1]
        target_period = (
            cutoff + pd.offsets.MonthBegin(1)
            if target == "RESMI_BPS"
            else cutoff + pd.Timedelta(days=CLASSIFICATION_INDEX_HORIZON_DAYS)
        )
        actual = (
            _actual_official_stats(official_regions[regency_code], cutoff)
            if target == "RESMI_BPS"
            else _actual_index_stats(frame)
        )
        projection: dict[str, float | None] = {"yoy": None, "mtm": None, "ytd": None}
        if target == "INDEKS_PANGAN" and projected_index and regency_code in projected_index:
            projection["mtm"] = round(float(projected_index[regency_code]), 4)

        is_eligible = bool(trained.metrics.get("layak"))
        summary.signals.append(
            {
                "wilayah_kode": regency_code,
                "periode": target_period,
                "kelas": ("INFLASI" if prob >= 0.5 else "TIDAK_INFLASI") if is_eligible else None,
                "probabilitas": round(prob, 6),
                "skor_tekanan": round(score, 2) if is_eligible else None,
                "siklus": determine_cycle_phase(score, momentum) if is_eligible else None,
                "layak": is_eligible,
                "alasan": trained.metrics.get("alasan"),
                "target": target,
                "proyeksi": projection,
                "aktual": actual,
                "penyumbang": _top_commodity_contributors(panel, regency_code, dataset.commodity_names),
                "kalibrasi": {"metode": trained.calibration_method, "brier": trained.metrics.get("brier")},
            }
        )
        summary.inference_rows[regency_code] = (target, x)

    used_targets = {s["target"] for s in summary.signals}
    summary.cycle_target = "RESMI_BPS" if used_targets == {"RESMI_BPS"} else "INDEKS_PANGAN"
    if used_targets == {"RESMI_BPS", "INDEKS_PANGAN"}:
        summary.notes.append("target campuran: sebagian wilayah memakai indeks pangan, lihat field target per dokumen")

    for target_name, trained in summary.models.items():
        if not trained.metrics.get("layak"):
            summary.notes.append(f"klasifikasi {target_name} belum layak: {trained.metrics.get('alasan')}")

    primary = summary.models.get(summary.cycle_target) or next(iter(summary.models.values()), None)
    if primary is None:
        summary.metrics = {
            "model": None,
            "f1": None,
            "roc_auc": None,
            "brier": None,
            "brier_baseline": None,
            "n_uji": 0,
            "layak": False,
            "alasan": "; ".join(summary.notes) or "data tidak cukup",
        }
    else:
        summary.metrics = {
            "model": primary.model_name,
            **primary.metrics,
            "kalibrasi": primary.calibration_method,
        }
        if backend.is_fallback and primary.model_name == backend.name:
            summary.metrics["alasan_fallback"] = backend.fallback_reason

    return summary
