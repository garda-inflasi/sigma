"""Backend-agnostic XAI ExplainerRouter unifying LightGBM TreeSHAP, HGB, RF, Logistic, and statistical models."""

import logging
from typing import Any

import numpy as np

from garda_ml.config.constants import DEFAULT_HORIZONS
from garda_ml.domain.entities import ForecastResult
from garda_ml.domain.enums import ModelFamily

logger = logging.getLogger("garda_ml.pipeline")

GROUP_LABELS: dict[str, str] = {
    "CUACA": "Cuaca",
    "PASOKAN": "Pasokan",
    "KONSUMSI_PENDAPATAN": "Konsumsi dan pendapatan",
    "TENAGA_KERJA": "Tenaga kerja",
    "NERACA_EKONOMI": "Neraca ekonomi",
    "TREN_HARGA_LAPANGAN": "Tren harga dan lapangan",
    "KALENDER": "Kalender hari besar",
}

_IDENTITY_FEATURES = frozenset({"komoditas_id", "wilayah_id", "cluster_id", "cluster_distance"})
HORIZON_XAI = 30


def _unwrap_estimator(model: Any) -> Any:
    return getattr(model, "raw_estimator", model)


def map_feature_group(feature_name: str) -> str | None:
    """Map technical feature name to its contract policy group, returning None for ID columns."""
    if feature_name in _IDENTITY_FEATURES:
        return None
    if feature_name.startswith("cuaca_"):
        return "CUACA"
    if feature_name.startswith("kal_"):
        return "KALENDER"
    if feature_name == "tekanan_pasokan" or feature_name.startswith(("makro_PERTANIAN", "makro_PERIKANAN", "makro_KEHUTANAN")):
        return "PASOKAN"
    if feature_name == "tekanan_neraca" or feature_name.startswith("makro_NERACA_EKONOMI"):
        return "NERACA_EKONOMI"
    if feature_name == "biaya_tenaga_kerja" or feature_name.startswith("makro_TENAGA_KERJA"):
        return "TENAGA_KERJA"
    if feature_name.startswith("makro_KONSUMSI_PENDAPATAN"):
        return "KONSUMSI_PENDAPATAN"
    return "TREN_HARGA_LAPANGAN"


def aggregate_factor_groups(
    feature_names: list[str],
    values: np.ndarray,
    *,
    scale: float = 100.0,
    max_features: int = 3,
    input_row: np.ndarray | None = None,
) -> list[dict[str, Any]]:
    """Aggregate signed feature contributions into ordered policy factor groups."""
    grouped: dict[str, list[tuple[str, float]]] = {}
    for idx, (name, val) in enumerate(zip(feature_names, values)):
        group = map_feature_group(name)
        if group is None or not np.isfinite(val):
            continue
        if input_row is not None and not np.isfinite(input_row[idx]):
            continue
        grouped.setdefault(group, []).append((name, float(val) * scale))

    factors: list[dict[str, Any]] = []
    for group, entries in grouped.items():
        total = sum(v for _, v in entries)
        if abs(total) < 1e-9 and all(abs(v) < 1e-9 for _, v in entries):
            continue
        top = sorted(entries, key=lambda item: abs(item[1]), reverse=True)[:max_features]
        factors.append(
            {
                "kelompok": group,
                "label": GROUP_LABELS[group],
                "kontribusi": round(total, 4),
                "arah": "NAIK" if total >= 0 else "TURUN",
                "fitur": [{"nama": fname, "kontribusi": round(fval, 4)} for fname, fval in top],
            }
        )

    return sorted(factors, key=lambda item: abs(item["kontribusi"]), reverse=True)


def narrate_forecast(
    commodity_name: str,
    regency_code: str,
    pct_change: float,
    factors: list[dict[str, Any]],
    model_name: str,
) -> str:
    """Render concise Indonesian explanation narrative for commodity forecast cards."""
    if model_name == ModelFamily.NAIVE:
        return f"Harga {commodity_name} di wilayah {regency_code} diproyeksikan tetap pada harga terakhir (baseline persistensi)."

    direction = "naik" if pct_change > 0.05 else "turun" if pct_change < -0.05 else "relatif stabil"
    magnitude = "" if direction == "relatif stabil" else f" {abs(pct_change):.1f}%"
    text = f"Harga {commodity_name} di wilayah {regency_code} diproyeksikan {direction}{magnitude} dalam {HORIZON_XAI} hari (model {model_name})."
    if factors:
        highlights = ", ".join(
            f"{f['label'].lower()} ({'mendorong naik' if f['kontribusi'] >= 0 else 'menekan turun'})"
            for f in factors[:2]
        )
        text += f" Faktor terbesar: {highlights}."
    return text


def narrate_inflation(
    regency_code: str,
    signal: dict[str, Any],
    factors: list[dict[str, Any]],
    baseline_prob: float,
) -> str:
    """Render concise Indonesian explanation narrative for regional pressure signals."""
    label = "inflasi resmi BPS" if signal["target"] == "RESMI_BPS" else "indeks harga pangan (bukan inflasi resmi)"
    text = (
        f"Probabilitas kenaikan {label} di wilayah {regency_code} sebesar {signal['probabilitas'] * 100:.0f}% "
        f"(dasar {baseline_prob * 100:.0f}%), siklus {signal['siklus']}."
    )
    if factors:
        text += " Faktor terbesar: " + ", ".join(f["label"].lower() for f in factors[:2]) + "."
    return text


def _sigmoid(z: np.ndarray) -> np.ndarray:
    return 1.0 / (1.0 + np.exp(-z))


def _linear_log_odds_attribution(pipeline_model: Any, X: np.ndarray) -> tuple[np.ndarray, float]:
    """Exact linear log-odds attribution for sklearn imputer+scaler+Logistic/Ridge pipelines."""
    imputer = pipeline_model.named_steps.get("imputer")
    scaler = pipeline_model.named_steps.get("scaler")
    final_step = pipeline_model[-1]
    X_clean = imputer.transform(X) if imputer is not None else X
    X_scaled = scaler.transform(X_clean) if scaler is not None else X_clean
    coef = np.ravel(final_step.coef_)
    intercept = float(np.ravel(final_step.intercept_)[0])
    phi = np.asarray(X_scaled, dtype=float) * coef[None, :]
    return phi, intercept


def compute_shap_values(model: Any, X: np.ndarray, explainer: Any = None) -> np.ndarray:
    """Compute SHAP values for LightGBM / HGB / RandomForest estimators (positive class for binary)."""
    import shap

    raw = _unwrap_estimator(model)
    out = (explainer or shap.TreeExplainer(raw)).shap_values(X)
    if isinstance(out, list):
        out = out[-1]
    arr = np.asarray(out)
    if arr.ndim == 3:
        arr = arr[:, :, -1]
    return arr


def calibrated_probability_attributions(
    model: Any,
    X: np.ndarray,
) -> tuple[np.ndarray, np.ndarray, np.ndarray]:
    """Compute exact additive attributions on calibrated probabilities: `sum(c) == p - p0`."""
    import shap

    if hasattr(model, "calibrated_classifiers_"):
        pairs = [(c.estimator, c.calibrators[0]) for c in model.calibrated_classifiers_]
    else:
        pairs = [(model, None)]

    total_c = np.zeros(X.shape, dtype=float)
    total_p = np.zeros(len(X))
    total_p0 = np.zeros(len(X))

    for est, calibrator in pairs:
        raw_est = _unwrap_estimator(est)
        if hasattr(raw_est, "named_steps") and "logreg" in raw_est.named_steps:
            phi, base_val = _linear_log_odds_attribution(raw_est, X)
            raw_score = base_val + phi.sum(axis=1)
        else:
            explainer = shap.TreeExplainer(raw_est)
            phi = compute_shap_values(raw_est, X, explainer)
            base_val = float(np.ravel(explainer.expected_value)[-1])
            raw_score = base_val + phi.sum(axis=1)

        if calibrator is None:
            p = _sigmoid(raw_score)
            p0 = np.full(len(X), float(_sigmoid(np.array(base_val))))
        else:
            transform = (lambda z: z) if hasattr(raw_est, "decision_function") else _sigmoid
            p = np.clip(calibrator.predict(transform(raw_score)), 0.0, 1.0)
            p0 = np.full(len(X), float(np.clip(calibrator.predict(transform(np.array([base_val]))), 0.0, 1.0)[0]))

        row_sum = phi.sum(axis=1)
        scale_factor = np.divide(p - p0, row_sum, out=np.zeros(len(X)), where=np.abs(row_sum) > 1e-12)
        total_c += phi * scale_factor[:, None]
        total_p += p
        total_p0 += p0

    n = len(pairs)
    return total_c / n, total_p / n, total_p0 / n


def decompose_series_projection(log_y: np.ndarray, projected_log_change: float) -> tuple[list[str], np.ndarray]:
    """Decompose statistical forecasts into medium-term trend, 7-day momentum, and weekly seasonality."""
    n = len(log_y)
    momentum = float(log_y[-1] - log_y[-8]) if n >= 8 else 0.0
    seasonal = 0.0
    if n >= 21:
        ma7 = np.convolve(log_y, np.ones(7) / 7, mode="valid")
        residuals = log_y[6:] - ma7
        target_day = (len(residuals) - 1 + HORIZON_XAI) % 7
        day_effects = [float(np.mean(residuals[i::7])) for i in range(7)]
        seasonal = day_effects[target_day] - day_effects[(len(residuals) - 1) % 7]

    remainder = projected_log_change - momentum - seasonal
    return ["tren_jangka_menengah", "momentum_7_hari", "kal_pola_mingguan"], np.array([remainder, momentum, seasonal])


class ExplainerRouter:
    """Backend-agnostic facade routing XAI requests to TreeSHAP, linear weights, or decomposition."""

    def explain_quantile_batch(
        self,
        model: Any,
        X: np.ndarray,
        feature_names: list[str],
    ) -> np.ndarray:
        """Compute local feature attributions across a cutoff batch."""
        raw = _unwrap_estimator(model)
        if hasattr(raw, "named_steps") and "ridge" in raw.named_steps:
            imputer = raw.named_steps.get("imputer")
            scaler = raw.named_steps.get("scaler")
            X_clean = imputer.transform(X) if imputer is not None else X
            X_scaled = scaler.transform(X_clean) if scaler is not None else X_clean
            return np.asarray(X_scaled, dtype=float) * np.ravel(raw.named_steps["ridge"].coef_)[None, :]
        del feature_names
        return compute_shap_values(raw, X)


def explain_forecasts(
    results: list[ForecastResult],
    panel_df: Any,
    feature_cols: list[str],
    global_models: dict[int, dict[float, Any]],
    commodity_names: dict[str, str],
    cutoff: Any,
) -> tuple[list[dict[str, Any]], list[dict[str, Any]]]:
    """Build XAI explanations for every forecast series using SHAP or series decomposition."""
    router = ExplainerRouter()
    explanations: list[dict[str, Any]] = []
    failures: list[dict[str, Any]] = []
    idx_h = list(DEFAULT_HORIZONS).index(HORIZON_XAI)

    shap_targets = [r for r in results if r.model_name.endswith("_quantile")]
    shap_map: dict[tuple[str, str], tuple[np.ndarray, np.ndarray]] = {}
    if shap_targets and global_models and HORIZON_XAI in global_models:
        cutoff_rows = panel_df[panel_df["tanggal"] == cutoff]
        row_keys = list(zip(cutoff_rows["komoditas_kode"], cutoff_rows["kabupaten_kode"]))
        key_to_row = {k: i for i, k in enumerate(row_keys)}
        selected_indices = [
            key_to_row[(r.series.commodity_code, r.series.regency_code)]
            for r in shap_targets
            if (r.series.commodity_code, r.series.regency_code) in key_to_row
        ]
        if selected_indices:
            X_batch = cutoff_rows.iloc[selected_indices][feature_cols].to_numpy(dtype=np.float32)
            try:
                attrs = router.explain_quantile_batch(global_models[HORIZON_XAI][0.5], X_batch, feature_cols)
                for batch_i, row_i in enumerate(selected_indices):
                    shap_map[row_keys[row_i]] = (attrs[batch_i], X_batch[batch_i])
            except Exception as err:
                logger.exception("forecast XAI attribution failed")
                failures.append({"tahap": "shap_forecast", "alasan": str(err)[:200]})

    for res in results:
        k, w = res.series.commodity_code, res.series.regency_code
        try:
            p50 = float(res.quantiles[idx_h, 1])
            log_change = float(np.log(p50 / res.series.latest_price))
            if (k, w) in shap_map:
                method = "shap"
                factors = aggregate_factor_groups(
                    feature_cols,
                    shap_map[(k, w)][0],
                    input_row=shap_map[(k, w)][1],
                )
            elif res.model_name == ModelFamily.NAIVE:
                method, factors = "dekomposisi", []
            else:
                method = "dekomposisi"
                names, vals = decompose_series_projection(
                    res.series.log_prices[: res.series.last_original_idx + 1],
                    log_change,
                )
                factors = aggregate_factor_groups(names, vals)

            display_name = commodity_names.get(k, k)
            explanations.append(
                {
                    "target": "forecast",
                    "komoditas_kode": k,
                    "wilayah_kode": w,
                    "metode": method,
                    "satuan_kontribusi": "poin_persen_harga",
                    "faktor": factors,
                    "narasi": narrate_forecast(
                        display_name,
                        w,
                        float(np.expm1(log_change) * 100),
                        factors,
                        res.model_name,
                    ),
                }
            )
        except Exception as err:
            failures.append({"tahap": "xai_forecast", "komoditas_kode": k, "kabupaten_kode": w, "alasan": str(err)[:200]})

    return explanations, failures


def explain_inflation_signals(classification: Any) -> tuple[list[dict[str, Any]], list[dict[str, Any]]]:
    """Build exact calibrated SHAP factor explanations for eligible inflation risk classifiers."""
    explanations: list[dict[str, Any]] = []
    failures: list[dict[str, Any]] = []
    by_regency = {s["wilayah_kode"]: s for s in classification.signals}

    for target, trained in classification.models.items():
        if not trained.metrics.get("layak"):
            continue
        regencies = [w for w, (t, _) in classification.inference_rows.items() if t == target]
        if not regencies:
            continue

        X = np.vstack([classification.inference_rows[w][1] for w in regencies])
        try:
            attributions, probs, base_probs = calibrated_probability_attributions(trained.model, X)
            published = trained.model.predict_proba(X)[:, 1]
            if not np.allclose(probs, published, atol=1e-6):
                diff = float(np.abs(probs - published).max())
                raise ValueError(f"atribusi tidak cocok dengan probabilitas terpublikasi (selisih {diff:.2e})")
        except Exception as err:
            logger.exception("classification XAI attribution failed")
            failures.append({"tahap": "shap_inflasi", "target": target, "alasan": str(err)[:200]})
            continue

        for row_i, regency_code in enumerate(regencies):
            factors = aggregate_factor_groups(trained.columns, attributions[row_i])
            explanations.append(
                {
                    "target": "inflasi",
                    "komoditas_kode": None,
                    "wilayah_kode": regency_code,
                    "metode": "shap",
                    "satuan_kontribusi": "poin_persen_probabilitas",
                    "probabilitas": round(float(probs[row_i]), 6),
                    "probabilitas_dasar": round(float(base_probs[row_i]), 6),
                    "faktor": factors,
                    "narasi": narrate_inflation(regency_code, by_regency[regency_code], factors, float(base_probs[row_i])),
                }
            )

    return explanations, failures
