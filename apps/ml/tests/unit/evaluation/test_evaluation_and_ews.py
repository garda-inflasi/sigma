"""Unit tests for walk-forward evaluation, calibration eligibility, XAI, and EWS signals."""

from datetime import date, datetime, timezone

import numpy as np
import pandas as pd
import pytest

from garda_ml.config.constants import MIN_ROC_AUC_ELIGIBLE
from garda_ml.data.safe_writer import sanitize_bson
from garda_ml.data.time_utils import build_cycle_id, seconds_until_next_cycle, to_wib_date
from garda_ml.domain.entities import ForecastResult
from garda_ml.domain.enums import BoostingBackend, ModelFamily
from garda_ml.evaluation.calibration import (
    compute_pressure_score,
    determine_cycle_phase,
    evaluate_classification_eligibility,
    run_risk_classification,
)
from garda_ml.evaluation.metrics import horizon_balanced_scores
from garda_ml.evaluation.selector import (
    BASELINES,
    best_baseline,
    choose_champion,
    run_regression_tournament,
    score_series_candidate_folds,
)
from garda_ml.evaluation.walk_forward import actual_log_price, build_walk_forward_folds, embargo_time_folds
from garda_ml.features.builder import build_feature_panel
from garda_ml.models.backend_resolver import ResolvedBackend, resolve_boosting_backend
from garda_ml.config.settings import PipelineSettings
from garda_ml.postprocessing.anomaly_detector import classify_anomaly_severity, detect_price_anomalies
from garda_ml.postprocessing.explainability import (
    aggregate_factor_groups,
    calibrated_probability_attributions,
    compute_shap_values,
    decompose_series_projection,
    explain_inflation_signals,
    map_feature_group,
)
from garda_ml.postprocessing.signal_builder import build_freshness_signals, build_projection_signals, mark_recovered_signals
from tests.unit.features.test_features import END_TS, sample_prices, wrap_dataset

HGB = ResolvedBackend(BoostingBackend.HIST_GRADIENT_BOOSTING, ModelFamily.HGB_QUANTILE, "test", 1)
CFG = PipelineSettings(mongo_uri="mongodb://unused", n_jobs=1, max_train_rows=20_000)
ACTIVE = resolve_boosting_backend(1)
TEST_BACKENDS = [HGB] + ([] if ACTIVE.is_fallback else [ACTIVE])


def _folds(*mapes: float) -> dict[int, dict[str, float]]:
    return {i: {"mape": float(m), "rmse": float(m)} for i, m in enumerate(mapes)}


def test_walk_forward_embargo_and_champion_margin_gates():
    folds = build_walk_forward_folds(END_TS)
    assert len(folds) == 5 and folds[0][0] == END_TS - pd.Timedelta(days=150)

    t = np.array(pd.date_range("2025-01-01", periods=120, freq="7D").values)
    emb = embargo_time_folds(t, 3, 30, np.arange(120) % 2)
    assert len(emb) == 3 and all(t[tr].max() < t[te].min() - np.timedelta64(30, "D") for tr, te in emb)

    base = _folds(5, 5, 5, 5)
    assert choose_champion({"seasonal_naive": base, "sarimax": _folds(4.8, 4.8, 4.8, 4.8)}) == "seasonal_naive"
    assert choose_champion({"seasonal_naive": base, "sarimax": _folds(4.7, 4.7, 4.7, 4.7)}) == "sarimax"
    assert choose_champion({"naive": base, "sarimax": _folds(1, 1, 1, 5.0)}) == "naive"
    assert best_baseline({"naive": _folds(2, 2, 2, 2), "seasonal_naive": _folds(3, 3, 3, 3)}) == "naive"


def test_holdout_metrics_are_out_of_selection_sample():
    dates = [pd.Timestamp("2026-03-01"), pd.Timestamp("2026-04-01"), pd.Timestamp("2026-05-01")]
    a = np.log(100.0)
    cv = {
        "seasonal_naive": {(x, 7): (np.log(105.0), a) for x in dates},
        "sarimax": {(dates[0], 7): (np.log(101.0), a), (dates[1], 7): (np.log(101.0), a), (dates[2], 7): (np.log(120.0), a)},
    }
    res = score_series_candidate_folds(cv)
    assert res["nama"] == "sarimax"
    assert res["metrik"]["mape"] == pytest.approx(20.0)
    assert res["metrik"]["mengalahkan_baseline"] is False


def test_regression_tournament_skips_short_or_stale_series():
    prices = pd.concat(
        [
            sample_prices(days=300, commodities=("CBM-KRT",)),
            sample_prices(days=16, commodities=("BRS-MED",), seed=3),
            sample_prices(days=4, commodities=("DAG-AYM",), seed=4),
        ],
        ignore_index=True,
    )
    out = run_regression_tournament(build_feature_panel(wrap_dataset(prices), END_TS), HGB, CFG)
    by_code = {r.series.commodity_code: r for r in out.results}
    assert "DAG-AYM" not in by_code and [s["komoditas_kode"] for s in out.skipped] == ["DAG-AYM"]
    assert by_code["BRS-MED"].model_name == ModelFamily.NAIVE and by_code["BRS-MED"].is_fallback


@pytest.mark.parametrize("backend", TEST_BACKENDS, ids=lambda b: b.name)
def test_classification_eligibility_and_exact_xai_sum(backend):
    rng = np.random.default_rng(3)
    t = pd.date_range(END_TS - pd.Timedelta(days=1100), END_TS, freq="D")
    seasonal_parts = []
    for k in ("CBM-KRT", "BRS-MED", "DAG-AYM"):
        for w in ("1171", "1174", "1107"):
            log_p = np.log(20000) + np.cumsum(rng.normal(0, 0.002, len(t))) + 0.05 * np.sin(2 * np.pi * t.dayofyear / 365.25)
            d = pd.DataFrame({"komoditas_kode": k, "kabupaten_kode": w, "tanggal": t, "nilai": np.exp(log_p), "bobot": 1.0, "nilai_lapangan": np.nan, "bobot_lapangan": np.nan, "sumber": "PIHPS", "satuan": "Rp/kg"})
            seasonal_parts.append(d[d["tanggal"].dt.dayofweek < 5])
    data = wrap_dataset(pd.concat(seasonal_parts, ignore_index=True))
    cls = run_risk_classification(build_feature_panel(data, END_TS), data, backend)
    assert cls.metrics["layak"] is True and cls.metrics["roc_auc"] >= MIN_ROC_AUC_ELIGIBLE
    docs, errs = explain_inflation_signals(cls)
    assert not errs and len(docs) == len(cls.signals)
    by_w = {s["wilayah_kode"]: s for s in cls.signals}
    for doc in docs:
        total = sum(f["kontribusi"] for f in doc["faktor"])
        assert total == pytest.approx((doc["probabilitas"] - doc["probabilitas_dasar"]) * 100, abs=1e-2)
        assert doc["probabilitas"] == pytest.approx(by_w[doc["wilayah_kode"]]["probabilitas"], abs=1e-6)


def test_xai_helpers_and_ews_signals():
    assert map_feature_group("cuaca_hujan_7") == "CUACA"
    assert map_feature_group("cluster_id") is None
    _, vals = decompose_series_projection(np.log(10000) + np.cumsum(np.random.default_rng(1).normal(0, 0.01, 100)), 0.05)
    assert vals.sum() == pytest.approx(0.05)

    assert classify_anomaly_severity(4, 30, False) == "KRITIS"
    assert determine_cycle_phase(compute_pressure_score(1.0, 0.5), 0.1) == "TEKANAN"

    freshness = [{"sumber": "SP2KP", "dataset": "harga_kemendag", "status": "ERROR", "umur_hari": None, "last_available_date": None, "pesan": "DILEWATI: butuh token"}]
    sigs = {x["sinyal_kunci"]: x["severity"] for x in build_freshness_signals(freshness, True)}
    assert sigs["SUMBER_DATA:SP2KP:harga_kemendag"] == "INFO"
    assert sigs["SUMBER_DATA:HARGA:SEMUA"] == "KRITIS"

    recovered = mark_recovered_signals([], [{"sinyal_kunci": "X", "jenis": "ANOMALI_HARGA", "severity": "KRITIS", "aktif": True}])
    assert [(r["sinyal_kunci"], r["aktif"]) for r in recovered] == [("X", False)]

    cleaned = sanitize_bson({"a": np.float32(1.5), "b": float("nan"), "t": pd.Timestamp("2026-01-02")})
    assert cleaned == {"a": 1.5, "b": None, "t": datetime(2026, 1, 2, tzinfo=timezone.utc)}
    assert build_cycle_id(datetime(2026, 3, 5, 7, 42, tzinfo=timezone.utc)) == "C20260305T07Z"
    assert to_wib_date(datetime(2026, 3, 5, 18, 0, tzinfo=timezone.utc)) == date(2026, 3, 6)
    assert seconds_until_next_cycle(datetime(2026, 3, 5, 7, 0, tzinfo=timezone.utc), 5) == 300
