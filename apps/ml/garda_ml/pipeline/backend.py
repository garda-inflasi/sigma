"""Pemilihan pustaka boosting: LightGBM bila bisa dimuat, selain itu HistGradientBoosting.

LightGBM di macOS butuh OpenMP (``libomp``). Bila gagal dimuat, pipeline memakai fallback
dan menuliskan nama model yang benar-benar berjalan ke keluaran.
"""

from __future__ import annotations

import os
from dataclasses import dataclass
from typing import Any

from sklearn.ensemble import HistGradientBoostingClassifier, HistGradientBoostingRegressor

PARAM_BAWAAN: dict[str, float] = {
    "n_estimators": 200,
    "learning_rate": 0.04,
    "num_leaves": 31,
    "min_child_samples": 30,
}

GRID_PARAM: tuple[dict[str, float], ...] = (
    {"n_estimators": 120, "learning_rate": 0.06, "num_leaves": 15, "min_child_samples": 40},
    PARAM_BAWAAN,
    {"n_estimators": 300, "learning_rate": 0.03, "num_leaves": 63, "min_child_samples": 20},
)


@dataclass(frozen=True)
class Backend:
    nama: str  # "lightgbm" atau "hist_gradient_boosting"
    nama_kuantil: str  # nama model di keluaran
    alasan_fallback: str | None
    n_thread: int = 2

    @property
    def fallback(self) -> bool:
        return self.alasan_fallback is not None

    def regresor_kuantil(self, kuantil: float, params: dict | None = None, seed: int = 42) -> Any:
        p = {**PARAM_BAWAAN, **(params or {})}
        if self.nama == "lightgbm":
            from lightgbm import LGBMRegressor

            return LGBMRegressor(
                objective="quantile", alpha=kuantil, n_estimators=int(p["n_estimators"]),
                learning_rate=p["learning_rate"], num_leaves=int(p["num_leaves"]),
                min_child_samples=int(p["min_child_samples"]), subsample=0.8, subsample_freq=1,
                colsample_bytree=0.8, random_state=seed, n_jobs=self.n_thread, verbosity=-1,
            )
        return HistGradientBoostingRegressor(
            loss="quantile", quantile=kuantil, max_iter=int(p["n_estimators"]),
            learning_rate=p["learning_rate"], max_leaf_nodes=int(p["num_leaves"]),
            min_samples_leaf=int(p["min_child_samples"]), early_stopping=False, random_state=seed,
        )

    def klasifikator(self, seed: int = 42) -> Any:
        if self.nama == "lightgbm":
            from lightgbm import LGBMClassifier

            return LGBMClassifier(
                n_estimators=150, learning_rate=0.05, num_leaves=15, min_child_samples=10,
                subsample=0.8, subsample_freq=1, colsample_bytree=0.8, random_state=seed,
                n_jobs=self.n_thread, verbosity=-1,
            )
        return HistGradientBoostingClassifier(
            max_iter=150, learning_rate=0.05, max_leaf_nodes=15, min_samples_leaf=10,
            early_stopping=False, random_state=seed,
        )


def pilih_backend(n_thread: int = 2) -> Backend:
    """``ML_PAKSA_HGB=1`` memaksa fallback, dipakai untuk menguji jalur tanpa LightGBM."""
    if os.environ.get("ML_PAKSA_HGB", "").strip() == "1":
        return Backend("hist_gradient_boosting", "hgb_quantile", "dipaksa ML_PAKSA_HGB=1", n_thread)
    try:
        import lightgbm  # noqa: F401
    except (ImportError, OSError) as err:  # OSError: libomp tidak ditemukan
        pesan = str(err).splitlines()[0][:120]
        return Backend("hist_gradient_boosting", "hgb_quantile", f"LightGBM tidak dapat dimuat: {pesan}", n_thread)
    return Backend("lightgbm", "lightgbm_quantile", None, n_thread)
