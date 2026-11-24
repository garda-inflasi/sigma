"""Unit tests for boosting fallback, ModelRegistry, KMeans clustering, RF, Logistic, and optional PyTorch GNN."""

import numpy as np
import pandas as pd
import pytest

from garda_ml.domain.enums import BoostingBackend, ModelFamily
from garda_ml.models.backend_resolver import resolve_boosting_backend
from garda_ml.models.classification.logistic import CalibratedLogisticClassifier
from garda_ml.models.classification.random_forest import CalibratedRandomForestClassifier
from garda_ml.models.clustering.kmeans import RegencyKMeansClusterer
from garda_ml.models.forecasting.baselines import RidgeQuantileEstimator
from garda_ml.models.forecasting.pytorch_gnn import SpatioTemporalGnnEstimator, is_torch_available
from garda_ml.models.forecasting.random_forest import RandomForestQuantileEstimator
from garda_ml.models.forecasting.statistical import (
    STATISTICAL_FORECASTERS,
    forecast_series_folds,
    sort_quantiles,
    statistical_candidates,
)
from garda_ml.models.registry import DEFAULT_MODEL_REGISTRY


def test_backend_resolver_forces_hgb_honestly(monkeypatch):
    monkeypatch.setenv("ML_PAKSA_HGB", "1")
    backend = resolve_boosting_backend()
    assert backend.name == BoostingBackend.HIST_GRADIENT_BOOSTING
    assert backend.quantile_model_key == ModelFamily.HGB_QUANTILE
    assert backend.is_fallback


def test_model_registry_filters_by_history_depth():
    hgb = resolve_boosting_backend(force_hgb=True)
    short = DEFAULT_MODEL_REGISTRY.eligible_forecast_keys(15, hgb, include_deep=False)
    assert short == [ModelFamily.NAIVE, ModelFamily.SEASONAL_NAIVE, ModelFamily.RIDGE_QUANTILE]

    medium = DEFAULT_MODEL_REGISTRY.eligible_forecast_keys(45, hgb, include_deep=False)
    assert ModelFamily.HOLT_WINTERS in medium and ModelFamily.RANDOM_FOREST_QUANTILE in medium

    deep_disabled = DEFAULT_MODEL_REGISTRY.eligible_forecast_keys(200, hgb, include_deep=False)
    assert ModelFamily.PYTORCH_GNN not in deep_disabled
    assert ModelFamily.HGB_QUANTILE in deep_disabled

    deep_enabled = DEFAULT_MODEL_REGISTRY.eligible_forecast_keys(200, hgb, include_deep=True)
    assert ModelFamily.PYTORCH_GNN in deep_enabled


def test_ridge_and_random_forest_quantile_estimators():
    rng = np.random.default_rng(7)
    X = rng.normal(size=(120, 4))
    y = 0.05 * X[:, 0] + rng.normal(0, 0.02, 120)

    ridge_lo = RidgeQuantileEstimator(quantile=0.1).fit(X, y).predict(X[:10])
    ridge_hi = RidgeQuantileEstimator(quantile=0.9).fit(X, y).predict(X[:10])
    assert (ridge_lo <= ridge_hi).all()

    rf_lo = RandomForestQuantileEstimator(quantile=0.1, params={"n_estimators": 20, "min_samples_leaf": 5}).fit(X, y).predict(X[:10])
    rf_hi = RandomForestQuantileEstimator(quantile=0.9, params={"n_estimators": 20, "min_samples_leaf": 5}).fit(X, y).predict(X[:10])
    assert (rf_lo <= rf_hi).all()


def test_logistic_and_random_forest_classifiers():
    rng = np.random.default_rng(1)
    X = rng.normal(size=(80, 4))
    X[0, 1] = np.nan
    y = (X[:, 0] + rng.normal(0, 0.3, 80) > 0).astype(int)

    logreg = CalibratedLogisticClassifier().fit(X, y)
    probs = logreg.predict_proba(X[:10])
    assert probs.shape == (10, 2) and np.allclose(probs.sum(axis=1), 1.0)

    rf = CalibratedRandomForestClassifier().fit(X, y)
    probs_rf = rf.predict_proba(X[:10])
    assert probs_rf.shape == (10, 2)


def test_kmeans_clustering_orders_vulnerability_deterministically():
    profiles = pd.DataFrame(
        {
            "regency_code": ["1101", "1102", "1171", "1173", "1174", "1107"],
            "volatility_mean": [0.005, 0.006, 0.03, 0.032, 0.015, 0.016],
            "abs_momentum_30": [0.01, 0.012, 0.08, 0.085, 0.04, 0.045],
            "climate_exposure": [4.0, 5.0, 18.0, 19.0, 10.0, 11.0],
        }
    )
    clusterer = RegencyKMeansClusterer()
    clusters = clusterer.fit_predict(profiles)
    assert len(clusters) == 6
    assert clusterer.selected_k >= 2
    # Low volatility region must have lower cluster_id rank than high volatility region.
    assert clusters["1101"].cluster_id < clusters["1171"].cluster_id


@pytest.mark.parametrize("name", list(STATISTICAL_FORECASTERS))
def test_statistical_quantiles_monotonic_and_positive(name):
    y = np.log(15000) + np.cumsum(np.random.default_rng(2).normal(0, 0.005, 150))
    preds = sort_quantiles(STATISTICAL_FORECASTERS[name](y, [7, 14, 30]))
    assert (np.diff(preds, axis=1) >= 0).all() and (np.exp(preds) > 0).all()


def test_statistical_fold_no_future_leakage():
    y = np.log(15000) + np.cumsum(np.random.default_rng(4).normal(0, 0.005, 400))
    a = forecast_series_folds("sarimax", y, [300, 307], [7, 14, 30])
    z = y.copy()
    z[308:] += 1.0
    b = forecast_series_folds("sarimax", z, [300, 307], [7, 14, 30])
    np.testing.assert_allclose(a[0], b[0])
    np.testing.assert_allclose(a[1], b[1])
    assert statistical_candidates(10) == ["naive", "seasonal_naive"]


def test_pytorch_gnn_optional_guard():
    est = SpatioTemporalGnnEstimator(epochs=2)
    if not is_torch_available():
        with pytest.raises(ImportError):
            est.fit(np.zeros((5, 3)), np.zeros(5))
    else:
        preds = est.fit(np.random.normal(size=(12, 3)), np.zeros(12)).predict(np.zeros((4, 3)))
        assert preds.shape == (4,)
