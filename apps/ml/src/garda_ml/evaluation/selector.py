"""Walk-forward champion tournament, eligibility gates, and tiered selection/fast-refit runner."""

import logging
from collections import Counter
from dataclasses import dataclass, field
from typing import Any

import numpy as np
import pandas as pd
from joblib import Parallel, delayed

from garda_ml.config.constants import (
    FORECAST_MIN_POINTS,
    GLOBAL_MIN_POINTS,
    MAX_FORECAST_STALE_DAYS,
    MAX_INFERENCE_STALE_DAYS,
    MAX_LABEL_STALE_DAYS,
    MAX_TRAIN_ROW_STALE_DAYS,
    MIN_BASELINE_MARGIN,
    MIN_SELECTION_FOLDS,
    N_HOLDOUT_FOLDS,
    SELECTION_STEP_DAYS,
    DEFAULT_HORIZONS,
    DEFAULT_QUANTILES,
)
from garda_ml.config.model_specs import BOOSTING_PARAM_GRID, DEFAULT_BOOSTING_PARAMS
from garda_ml.config.settings import PipelineSettings
from garda_ml.domain.entities import FeaturePanel, ForecastResult, PriceSeries
from garda_ml.domain.enums import ModelFamily
from garda_ml.evaluation.metrics import horizon_balanced_scores, mean_fold_metric
from garda_ml.evaluation.walk_forward import actual_log_price, build_walk_forward_folds, fold_boundaries
from garda_ml.models.backend_resolver import ResolvedBackend
from garda_ml.models.forecasting.baselines import RidgeQuantileEstimator
from garda_ml.models.forecasting.statistical import (
    STATISTICAL_FORECASTERS,
    forecast_series_folds,
    sort_quantiles,
    statistical_candidates,
)

logger = logging.getLogger("garda_ml.pipeline")

BASELINES: tuple[str, ...] = (ModelFamily.NAIVE, ModelFamily.SEASONAL_NAIVE)
FALLBACK_BASELINE: str = ModelFamily.NAIVE
MIN_GLOBAL_TRAIN_ROWS = 500


@dataclass
class RegressionSummary:
    """Consolidated forecasting tournament results and serialized artifact references."""

    results: list[ForecastResult] = field(default_factory=list)
    failed: list[dict[str, Any]] = field(default_factory=list)
    skipped: list[dict[str, Any]] = field(default_factory=list)
    tier: str = "PENUH"
    params: dict[str, Any] = field(default_factory=lambda: dict(DEFAULT_BOOSTING_PARAMS))
    global_models: dict[int, dict[float, Any]] = field(default_factory=dict)
    feature_columns: list[str] = field(default_factory=list)
    selected_count: int = 0

    def winner_counts(self) -> dict[str, int]:
        return dict(Counter(item.model_name for item in self.results))


def best_baseline(fold_scores: dict[str, dict[Any, dict[str, float]]]) -> str | None:
    """Return baseline (`naive` vs `seasonal_naive`) with lowest mean MAPE across shared folds."""
    available = [b for b in BASELINES if fold_scores.get(b)]
    if not available:
        return None
    shared_folds = sorted(set.intersection(*(set(fold_scores[b]) for b in available)))
    if not shared_folds:
        return available[0]
    return min(
        available,
        key=lambda name: (
            mean_fold_metric(fold_scores[name], shared_folds, "mape"),
            mean_fold_metric(fold_scores[name], shared_folds, "rmse"),
        ),
    )


def _beats_baseline_ratio(
    candidate_scores: dict[Any, dict[str, float]],
    baseline_scores: dict[Any, dict[str, float]],
    margin: float,
    min_folds: int,
) -> float | None:
    shared_folds = sorted(set(candidate_scores) & set(baseline_scores))
    if len(shared_folds) < min_folds:
        return None
    if any(candidate_scores[f]["mape"] >= baseline_scores[f]["mape"] for f in shared_folds):
        return None
    ratio = mean_fold_metric(candidate_scores, shared_folds) / mean_fold_metric(baseline_scores, shared_folds)
    return ratio if ratio <= 1 - margin else None


def choose_champion(
    fold_scores: dict[str, dict[Any, dict[str, float]]],
    margin: float = MIN_BASELINE_MARGIN,
    min_folds: int = MIN_SELECTION_FOLDS,
) -> str:
    """Select champion model strictly beating the best baseline across every selection fold by `margin`."""
    anchor = best_baseline(fold_scores)
    if anchor is None:
        raise ValueError("seleksi membutuhkan minimal satu baseline")

    eligible: dict[str, float] = {}
    for model_name, scores in fold_scores.items():
        if model_name in BASELINES:
            continue
        ratio = _beats_baseline_ratio(scores, fold_scores[anchor], margin, min_folds)
        if ratio is not None:
            eligible[model_name] = ratio

    if not eligible:
        return anchor

    return min(
        eligible,
        key=lambda name: (eligible[name], mean_fold_metric(fold_scores[name], sorted(fold_scores[name]), "rmse")),
    )


def score_series_candidate_folds(cv_results: dict[str, dict[Any, tuple[float, float]]]) -> dict[str, Any] | None:
    """Select champion on selection folds and evaluate unbiased metrics on final holdout fold."""
    anchors = [b for b in BASELINES if cv_results.get(b)]
    if not anchors:
        return None
    keys = sorted(set.intersection(*(set(cv_results[b]) for b in anchors)), key=lambda item: (item[0], item[1]))
    if not keys:
        return None

    by_fold: dict[Any, list[Any]] = {}
    for key in keys:
        by_fold.setdefault(key[0], []).append(key)

    folds = sorted(by_fold)
    has_holdout = len(folds) > N_HOLDOUT_FOLDS
    holdout_folds = folds[-N_HOLDOUT_FOLDS:] if has_holdout else folds
    selection_folds = (folds[:-N_HOLDOUT_FOLDS] if has_holdout else folds)[-4:]
    holdout_keys = [k for f in holdout_folds for k in by_fold[f]]

    participants = {
        name: vals
        for name, vals in cv_results.items()
        if name in BASELINES or all(k in vals for k in holdout_keys)
    }
    fold_scores = {
        name: {
            fold: horizon_balanced_scores(vals, by_fold[fold])
            for fold in selection_folds
            if all(k in vals for k in by_fold[fold])
        }
        for name, vals in participants.items()
    }
    fold_scores = {name: fs for name, fs in fold_scores.items() if fs}
    champion = choose_champion(fold_scores)
    baseline = best_baseline(fold_scores)
    assert baseline is not None

    champ_eval = horizon_balanced_scores(participants[champion], holdout_keys)
    base_eval = horizon_balanced_scores(participants[baseline], holdout_keys)
    beats_baseline = None
    if has_holdout:
        beats_baseline = champion not in BASELINES and champ_eval["mape"] < base_eval["mape"] * (1 - MIN_BASELINE_MARGIN)

    return {
        "nama": champion,
        "n_kandidat": len(fold_scores),
        "metrik": {
            "rmse": champ_eval["rmse"],
            "mape": champ_eval["mape"],
            "baseline": baseline,
            "mape_baseline": base_eval["mape"],
            "mengalahkan_baseline": beats_baseline,
            "evaluasi": "holdout_lipatan_terakhir" if has_holdout else "tanpa_holdout",
            "n_lipatan": len(folds),
            "n_lipatan_seleksi": len(selection_folds),
            "horizon": list(DEFAULT_HORIZONS),
        },
    }


def _cv_statistical_series(
    series: PriceSeries,
    folds: list[list[pd.Timestamp]],
) -> tuple[tuple[str, str], dict[str, dict[Any, tuple[float, float]]]]:
    out: dict[str, dict[Any, tuple[float, float]]] = {}
    disqualified: set[str] = set()

    for fold_idx, block, boundary in fold_boundaries(folds):
        positions: list[tuple[pd.Timestamp, int]] = []
        for ts in block:
            if series.dates[0] <= ts <= series.dates[-1]:
                pos = int((ts - series.dates[0]).days)
                if int(series.is_original[: pos + 1].sum()) >= FORECAST_MIN_POINTS:
                    positions.append((ts, pos))
        if not positions:
            continue

        for name in statistical_candidates(int(series.is_original[: positions[0][1] + 1].sum())):
            if name in disqualified:
                continue
            try:
                preds = forecast_series_folds(name, series.log_prices, [p for _, p in positions], list(DEFAULT_HORIZONS))
            except Exception:
                disqualified.add(name)
                out.pop(name, None)
                continue
            for (origin_ts, pos), pred_mat in zip(positions, preds):
                for j, h in enumerate(DEFAULT_HORIZONS):
                    if boundary is not None and origin_ts + pd.Timedelta(days=h) > boundary:
                        continue
                    actual = actual_log_price(series, pos, h)
                    if actual is not None:
                        out.setdefault(name, {})[(fold_idx, origin_ts, h)] = (float(pred_mat[j, 1]), actual)

    return (series.commodity_code, series.regency_code), out


def _select_train_indices(
    df: pd.DataFrame,
    horizon: int,
    cutoff_limit: pd.Timestamp,
    max_rows: int,
    seed: int = 42,
) -> np.ndarray:
    dates = df["tanggal"].to_numpy()
    mask = (
        df[f"target_h{horizon}"].notna().to_numpy()
        & (dates <= np.datetime64(cutoff_limit - pd.Timedelta(days=horizon)))
        & (df["umur_data"].to_numpy() <= MAX_TRAIN_ROW_STALE_DAYS)
    )
    indices = np.flatnonzero(mask)
    if len(indices) <= max_rows:
        return indices
    ordered = indices[np.argsort(dates[indices], kind="stable")]
    half = max_rows // 2
    recent, older = ordered[-half:], ordered[:-half]
    chosen_old = np.random.default_rng(seed).choice(older, size=max_rows - half, replace=False)
    return np.sort(np.r_[chosen_old, recent])


def _fit_one_global(
    df: pd.DataFrame,
    feature_cols: list[str],
    horizon: int,
    quantile: float,
    cutoff_limit: pd.Timestamp,
    estimator_factory: Any,
    max_rows: int,
) -> Any | None:
    idx = _select_train_indices(df, horizon, cutoff_limit, max_rows)
    if len(idx) < MIN_GLOBAL_TRAIN_ROWS:
        return None
    X = df[feature_cols].iloc[idx].to_numpy(dtype=np.float32)
    y = df[f"target_h{horizon}"].iloc[idx].to_numpy(dtype=float)
    w = df[f"bobot_h{horizon}"].iloc[idx].fillna(1.0).to_numpy(dtype=float)
    return estimator_factory(quantile).fit(X, y, sample_weight=w)


def _tune_boosting_hyperparams(
    panel: FeaturePanel,
    origin_ts: pd.Timestamp,
    backend: ResolvedBackend,
    settings: PipelineSettings,
) -> dict[str, Any]:
    df, cols = panel.frame, panel.feature_columns
    horizon = 14
    eval_rows = df[(df["tanggal"] == origin_ts) & df[f"target_h{horizon}"].notna()]
    if eval_rows.empty:
        return dict(DEFAULT_BOOSTING_PARAMS)

    best_params, best_mae = dict(DEFAULT_BOOSTING_PARAMS), np.inf
    X_eval = eval_rows[cols].to_numpy(dtype=np.float32)
    y_eval = eval_rows[f"target_h{horizon}"].to_numpy(dtype=float)

    for candidate in BOOSTING_PARAM_GRID:
        model = _fit_one_global(
            df,
            cols,
            horizon,
            0.5,
            origin_ts,
            lambda q, p=candidate: backend.make_quantile_regressor(q, p),
            settings.max_train_rows,
        )
        if model is None:
            return dict(DEFAULT_BOOSTING_PARAMS)
        mae = float(np.mean(np.abs(np.asarray(model.predict(X_eval), dtype=float) - y_eval)))
        if mae < best_mae:
            best_params, best_mae = dict(candidate), mae

    return best_params


def _cv_global_model(
    panel: FeaturePanel,
    folds: list[list[pd.Timestamp]],
    estimator_factory: Any,
    min_points_required: int,
    max_rows: int,
) -> dict[tuple[str, str], dict[Any, tuple[float, float]]]:
    df, cols = panel.frame, panel.feature_columns
    results: dict[tuple[str, str], dict[Any, tuple[float, float]]] = {}

    for fold_idx, block, boundary in fold_boundaries(folds):
        block_rows = df[df["tanggal"].isin(block)]
        if block_rows.empty:
            continue
        X = block_rows[cols].to_numpy(dtype=np.float32)
        for h in DEFAULT_HORIZONS:
            model = _fit_one_global(df, cols, h, 0.5, block[0], estimator_factory, max_rows)
            if model is None:
                continue
            preds = np.asarray(model.predict(X), dtype=float)
            for k, w, ts, pred in zip(
                block_rows["komoditas_kode"],
                block_rows["kabupaten_kode"],
                block_rows["tanggal"],
                preds,
            ):
                if boundary is not None and ts + pd.Timedelta(days=h) > boundary:
                    continue
                series = panel.series_map.get((k, w))
                if series is None:
                    continue
                pos = int((ts - series.dates[0]).days)
                if int(series.is_original[: pos + 1].sum()) < min_points_required:
                    continue
                actual = actual_log_price(series, pos, h)
                if actual is not None:
                    results.setdefault((k, w), {})[(fold_idx, ts, h)] = (float(series.log_prices[pos]) + float(pred), actual)

    return results


def _forecast_final_statistical(series: PriceSeries, model_name: str) -> tuple[tuple[str, str], str, np.ndarray | None, str | None]:
    key = (series.commodity_code, series.regency_code)

    def _run(name: str) -> np.ndarray:
        y = series.log_prices[: series.last_original_idx + 1]
        shifted_horizons = [h + series.latest_age_days for h in DEFAULT_HORIZONS]
        return np.exp(sort_quantiles(STATISTICAL_FORECASTERS[name](y, shifted_horizons)))

    try:
        return key, model_name, _run(model_name), None
    except Exception as err:
        if model_name == FALLBACK_BASELINE:
            return key, model_name, None, f"baseline gagal: {err}"
        try:
            return key, FALLBACK_BASELINE, _run(FALLBACK_BASELINE), f"{model_name} gagal saat fit final: {str(err)[:100]}"
        except Exception as err2:
            return key, model_name, None, f"baseline gagal: {err2}"


def _fit_global_final_quantiles(
    panel: FeaturePanel,
    backend: ResolvedBackend,
    params: dict[str, Any],
    settings: PipelineSettings,
) -> dict[int, dict[float, Any]]:
    models: dict[int, dict[float, Any]] = {}
    for h in DEFAULT_HORIZONS:
        per_q: dict[float, Any] = {}
        for q in DEFAULT_QUANTILES:
            fitted = _fit_one_global(
                panel.frame,
                panel.feature_columns,
                h,
                q,
                panel.cutoff,
                lambda q_val: backend.make_quantile_regressor(q_val, params),
                settings.max_train_rows,
            )
            if fitted is None:
                return {}
            per_q[q] = fitted
        models[h] = per_q
    return models


def _predict_global_cutoff(panel: FeaturePanel, models: dict[int, dict[float, Any]]) -> dict[tuple[str, str], np.ndarray]:
    rows = panel.frame[panel.frame["tanggal"] == panel.cutoff]
    if rows.empty or not models:
        return {}
    X = rows[panel.feature_columns].to_numpy(dtype=np.float32)
    mat = np.empty((len(rows), len(DEFAULT_HORIZONS), len(DEFAULT_QUANTILES)))
    for i, h in enumerate(DEFAULT_HORIZONS):
        for j, q in enumerate(DEFAULT_QUANTILES):
            mat[:, i, j] = np.asarray(models[h][q].predict(X), dtype=float)
    mat = sort_quantiles(mat)

    out: dict[tuple[str, str], np.ndarray] = {}
    for idx, (k, w) in enumerate(zip(rows["komoditas_kode"], rows["kabupaten_kode"])):
        series = panel.series_map.get((k, w))
        if series is None or series.latest_age_days > MAX_INFERENCE_STALE_DAYS:
            continue
        out[(k, w)] = np.exp(float(series.log_prices[-1]) + mat[idx])
    return out


def _run_parallel(n_jobs: int, tasks: list[tuple[Any, tuple[Any, ...], Any]]) -> list[Any]:
    if n_jobs <= 1 or len(tasks) < 8:
        return [fn(*args) for fn, args, _ in tasks]
    return Parallel(n_jobs=n_jobs, backend="loky", batch_size="auto")(delayed(fn)(*args) for fn, args, _ in tasks)


def run_regression_tournament(
    panel: FeaturePanel,
    backend: ResolvedBackend,
    settings: PipelineSettings,
    *,
    previous_champions: dict[tuple[str, str], dict[str, Any]] | None = None,
    full_selection: bool = True,
    previous_params: dict[str, Any] | None = None,
) -> RegressionSummary:
    """Execute full walk-forward selection or fast refit across all eligible price series."""
    champions = previous_champions or {}
    summary = RegressionSummary(
        tier="PENUH" if full_selection else "FIT_ULANG",
        params=dict(previous_params or DEFAULT_BOOSTING_PARAMS),
    )
    if panel.is_empty:
        return summary
    summary.feature_columns = list(panel.feature_columns)

    eligible_series: dict[tuple[str, str], PriceSeries] = {}
    for key, series in panel.series_map.items():
        if series.valid_point_count < FORECAST_MIN_POINTS:
            summary.skipped.append(
                {
                    "komoditas_kode": key[0],
                    "kabupaten_kode": key[1],
                    "alasan": f"riwayat {series.valid_point_count} titik (< {FORECAST_MIN_POINTS})",
                }
            )
        elif series.latest_age_days > MAX_FORECAST_STALE_DAYS:
            summary.skipped.append(
                {
                    "komoditas_kode": key[0],
                    "kabupaten_kode": key[1],
                    "alasan": f"data asli terakhir {series.latest_age_days} hari lalu (> {MAX_FORECAST_STALE_DAYS})",
                }
            )
        else:
            eligible_series[key] = series

    if not eligible_series:
        return summary

    to_select = list(eligible_series) if full_selection else [k for k in eligible_series if k not in champions]
    summary.selected_count = len(to_select)

    max_end = max(s.latest_date for s in eligible_series.values())
    folds = build_walk_forward_folds(max_end)
    cv_by_series: dict[tuple[str, str], dict[str, dict[Any, tuple[float, float]]]] = {}

    if to_select:
        stat_tasks = [(_cv_statistical_series, (eligible_series[k], folds), k) for k in to_select]
        for key, stat_scores in _run_parallel(settings.n_jobs, stat_tasks):
            cv_by_series[key] = stat_scores

        if full_selection:
            summary.params = _tune_boosting_hyperparams(
                panel,
                folds[0][0] - pd.Timedelta(days=SELECTION_STEP_DAYS),
                backend,
                settings,
            )

        # Evaluate primary boosting global candidate
        boosting_cv = _cv_global_model(
            panel,
            folds,
            lambda q: backend.make_quantile_regressor(q, summary.params),
            GLOBAL_MIN_POINTS,
            settings.max_train_rows,
        )
        for key, score_map in boosting_cv.items():
            if key in cv_by_series:
                cv_by_series[key][backend.quantile_model_key] = score_map

        # Evaluate regularized Ridge benchmark when global panel rows suffice
        if len(panel.frame) >= MIN_GLOBAL_TRAIN_ROWS:
            ridge_cv = _cv_global_model(
                panel,
                folds,
                lambda q: RidgeQuantileEstimator(quantile=q),
                GLOBAL_MIN_POINTS,
                settings.max_train_rows,
            )
            for key, score_map in ridge_cv.items():
                if key in cv_by_series:
                    cv_by_series[key][ModelFamily.RIDGE_QUANTILE] = score_map

    chosen: dict[tuple[str, str], dict[str, Any]] = {}
    for key, series in eligible_series.items():
        if key in cv_by_series:
            chosen[key] = score_series_candidate_folds(cv_by_series[key]) or {
                "nama": FALLBACK_BASELINE,
                "n_kandidat": 1,
                "metrik": {
                    "rmse": None,
                    "mape": None,
                    "baseline": None,
                    "mape_baseline": None,
                    "mengalahkan_baseline": None,
                    "evaluasi": "tanpa_evaluasi",
                    "n_lipatan": 0,
                    "n_lipatan_seleksi": 0,
                    "horizon": list(DEFAULT_HORIZONS),
                },
            }
        else:
            prev = champions[key]
            chosen[key] = {"nama": prev["nama"], "n_kandidat": prev.get("n_kandidat"), "metrik": prev["metrik"]}

    need_global = any(item["nama"] == backend.quantile_model_key for item in chosen.values())
    if need_global or any(s.valid_point_count >= GLOBAL_MIN_POINTS for s in eligible_series.values()):
        summary.global_models = _fit_global_final_quantiles(panel, backend, summary.params, settings)
    global_preds = _predict_global_cutoff(panel, summary.global_models)

    ridge_preds: dict[tuple[str, str], np.ndarray] = {}
    if any(item["nama"] == ModelFamily.RIDGE_QUANTILE for item in chosen.values()):
        ridge_models: dict[int, dict[float, Any]] = {}
        for h in DEFAULT_HORIZONS:
            per_q = {}
            for q in DEFAULT_QUANTILES:
                m = _fit_one_global(
                    panel.frame,
                    panel.feature_columns,
                    h,
                    q,
                    panel.cutoff,
                    lambda q_val: RidgeQuantileEstimator(quantile=q_val),
                    settings.max_train_rows,
                )
                if m is None:
                    ridge_models = {}
                    break
                per_q[q] = m
            if not per_q:
                break
            ridge_models[h] = per_q
        ridge_preds = _predict_global_cutoff(panel, ridge_models)

    stat_jobs = []
    for key, choice in chosen.items():
        name = choice["nama"]
        if name == backend.quantile_model_key and key in global_preds:
            continue
        if name == ModelFamily.RIDGE_QUANTILE and key in ridge_preds:
            continue
        if name not in STATISTICAL_FORECASTERS:
            name = choice["metrik"].get("baseline") or FALLBACK_BASELINE
        stat_jobs.append((_forecast_final_statistical, (eligible_series[key], name), key))
    final_stat = {res[0]: res for res in _run_parallel(settings.n_jobs, stat_jobs)} if stat_jobs else {}

    for key, choice in chosen.items():
        series = eligible_series[key]
        notes: list[str] = []
        metrics = choice["metrik"]
        is_fallback = series.valid_point_count < GLOBAL_MIN_POINTS
        if series.valid_point_count < GLOBAL_MIN_POINTS:
            notes.append(f"riwayat {series.valid_point_count} titik: kandidat dibatasi")

        cluster_info = panel.region_clusters.get(series.regency_code)
        cluster_id = cluster_info.cluster_id if cluster_info else None

        if choice["nama"] == backend.quantile_model_key and key in global_preds:
            final_name, quantiles = choice["nama"], global_preds[key]
            if backend.is_fallback:
                is_fallback = True
                notes.append(backend.fallback_reason or "")
        elif choice["nama"] == ModelFamily.RIDGE_QUANTILE and key in ridge_preds:
            final_name, quantiles = ModelFamily.RIDGE_QUANTILE, ridge_preds[key]
        else:
            _, final_name, quantiles, err = final_stat[key]
            if quantiles is None:
                summary.failed.append({"komoditas_kode": key[0], "kabupaten_kode": key[1], "alasan": err})
                continue
            if err:
                is_fallback = True
                notes.append(err)
            if choice["nama"] != final_name:
                is_fallback = True
                if not err:
                    notes.append(f"{choice['nama']} tidak tersedia pada siklus ini")
                metrics = {
                    **metrics,
                    "rmse": None,
                    "mape": metrics.get("mape_baseline") if final_name == metrics.get("baseline") else None,
                    "mengalahkan_baseline": False,
                    "evaluasi": "model_pengganti",
                }

        if final_name in BASELINES and (metrics.get("n_lipatan_seleksi") or 0) < 2:
            notes.append("lipatan seleksi kurang dari 2: baseline dipakai")
        if final_name in BASELINES and not notes:
            notes.append(
                "tidak ada kandidat yang mengalahkan baseline"
                if (choice.get("n_kandidat") or 1) > len(BASELINES)
                else "baseline"
            )
        if series.latest_age_days > 0:
            notes.append(f"data asli terakhir {series.latest_age_days} hari lalu")

        if not np.isfinite(quantiles).all() or (quantiles <= 0).any():
            summary.failed.append({"komoditas_kode": key[0], "kabupaten_kode": key[1], "alasan": "ramalan tidak valid"})
            continue

        summary.results.append(
            ForecastResult(
                series=series,
                model_name=final_name,
                is_fallback=is_fallback,
                reason="; ".join(n for n in notes if n),
                metrics=metrics,
                quantiles=quantiles,
                candidate_count=choice.get("n_kandidat"),
                cluster_id=cluster_id,
            )
        )

    return summary


class ChampionSelector:
    """Facade wrapping walk-forward evaluation and tiered tournament execution."""

    def select_champion(
        self,
        fold_scores: dict[str, dict[Any, dict[str, float]]],
        margin: float = MIN_BASELINE_MARGIN,
        min_folds: int = MIN_SELECTION_FOLDS,
    ) -> str:
        return choose_champion(fold_scores, margin=margin, min_folds=min_folds)
