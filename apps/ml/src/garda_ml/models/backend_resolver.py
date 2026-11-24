"""Adaptive gradient boosting backend resolver (LightGBM primary -> HGB fallback)."""

import os
from dataclasses import dataclass
from typing import Any

from garda_ml.config.model_specs import DEFAULT_BOOSTING_PARAMS
from garda_ml.domain.enums import BoostingBackend, ModelFamily


@dataclass(frozen=True)
class ResolvedBackend:
    """Active tree-boosting runtime resolved at pipeline startup."""

    name: str
    quantile_model_key: str
    fallback_reason: str | None
    n_jobs: int = 2

    @property
    def is_fallback(self) -> bool:
        return self.fallback_reason is not None

    def make_quantile_regressor(
        self,
        quantile: float,
        params: dict[str, Any] | None = None,
        seed: int = 42,
    ) -> Any:
        """Instantiate an unfitted quantile regressor using the active backend."""
        from garda_ml.models.forecasting.gradient_boosting import (
            HistGradientBoostingQuantileEstimator,
            LightGbmQuantileEstimator,
        )

        merged = {**DEFAULT_BOOSTING_PARAMS, **(params or {})}
        if self.name == BoostingBackend.LIGHTGBM:
            return LightGbmQuantileEstimator(quantile=quantile, params=merged, n_jobs=self.n_jobs, seed=seed)
        return HistGradientBoostingQuantileEstimator(quantile=quantile, params=merged, seed=seed)

    def make_classifier(self, seed: int = 42) -> Any:
        """Instantiate an unfitted base classifier estimator for the active backend."""
        from garda_ml.models.classification.gradient_boosting import (
            build_hgb_classifier_estimator,
            build_lgbm_classifier_estimator,
        )

        if self.name == BoostingBackend.LIGHTGBM:
            return build_lgbm_classifier_estimator(n_jobs=self.n_jobs, seed=seed)
        return build_hgb_classifier_estimator(seed=seed)


def resolve_boosting_backend(n_jobs: int = 2, force_hgb: bool = False) -> ResolvedBackend:
    """Resolve LightGBM when available, transparently falling back to HistGradientBoosting."""
    env_forced = (
        force_hgb
        or os.environ.get("ML_PAKSA_HGB", "").strip() == "1"
        or os.environ.get("ML_FORCE_HGB_FALLBACK", "").strip() == "1"
    )
    if env_forced:
        return ResolvedBackend(
            name=BoostingBackend.HIST_GRADIENT_BOOSTING,
            quantile_model_key=ModelFamily.HGB_QUANTILE,
            fallback_reason="dipaksa ML_PAKSA_HGB=1",
            n_jobs=n_jobs,
        )
    try:
        import lightgbm  # noqa: F401
    except (ImportError, OSError) as err:
        summary = str(err).splitlines()[0][:120]
        return ResolvedBackend(
            name=BoostingBackend.HIST_GRADIENT_BOOSTING,
            quantile_model_key=ModelFamily.HGB_QUANTILE,
            fallback_reason=f"LightGBM tidak dapat dimuat: {summary}",
            n_jobs=n_jobs,
        )
    return ResolvedBackend(
        name=BoostingBackend.LIGHTGBM,
        quantile_model_key=ModelFamily.LIGHTGBM_QUANTILE,
        fallback_reason=None,
        n_jobs=n_jobs,
    )
