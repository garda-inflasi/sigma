"""Unsupervised K-Means clustering for regency price volatility & climate vulnerability."""

import numpy as np
import pandas as pd
from sklearn.cluster import KMeans
from sklearn.metrics import silhouette_score
from sklearn.preprocessing import StandardScaler

from garda_ml.config.model_specs import KMEANS_CLUSTER_RANGE
from garda_ml.domain.entities import RegionCluster
from garda_ml.domain.enums import ModelFamily


class RegencyKMeansClusterer:
    """Profiles regencies into vulnerability clusters using standardized market & climate metrics."""

    model_key = ModelFamily.KMEANS

    def __init__(self, k_range: tuple[int, ...] = KMEANS_CLUSTER_RANGE, seed: int = 42) -> None:
        self._k_range = k_range
        self._seed = seed
        self.best_silhouette: float | None = None
        self.selected_k: int = 1

    def fit_predict(self, profiles: pd.DataFrame) -> dict[str, RegionCluster]:
        """Cluster regency profiles choosing k that maximizes silhouette score."""
        if profiles.empty:
            return {}

        codes = profiles["regency_code"].astype(str).tolist()
        numeric_cols = ["volatility_mean", "abs_momentum_30", "climate_exposure"]
        X_raw = profiles[numeric_cols].to_numpy(dtype=float)

        if len(codes) < 3:
            self.selected_k = 1
            self.best_silhouette = None
            return {
                code: RegionCluster(
                    regency_code=code,
                    cluster_id=0,
                    centroid_distance=0.0,
                    volatility_score=float(X_raw[i, 0]),
                    climate_exposure_score=float(X_raw[i, 2]),
                    cluster_label="Tunggal",
                )
                for i, code in enumerate(codes)
            }

        X_scaled = StandardScaler().fit_transform(X_raw)
        valid_ks = [k for k in self._k_range if 2 <= k < len(codes)]
        if not valid_ks:
            valid_ks = [2]

        best_model: KMeans | None = None
        best_score = -2.0
        for k in valid_ks:
            km = KMeans(n_clusters=k, n_init="auto", random_state=self._seed)
            labels = km.fit_predict(X_scaled)
            if len(set(labels)) < 2:
                continue
            score = float(silhouette_score(X_scaled, labels))
            if score > best_score:
                best_score = score
                best_model = km

        if best_model is None:
            best_model = KMeans(n_clusters=min(2, len(codes)), n_init="auto", random_state=self._seed).fit(X_scaled)
            best_score = 0.0

        self.selected_k = int(best_model.n_clusters)
        self.best_silhouette = round(best_score, 4)

        raw_labels = best_model.predict(X_scaled)
        # Order clusters deterministically from lowest volatility to highest risk.
        cluster_vol = {
            cid: float(X_raw[raw_labels == cid, 0].mean()) if (raw_labels == cid).any() else 0.0
            for cid in range(self.selected_k)
        }
        rank_map = {cid: rank for rank, cid in enumerate(sorted(cluster_vol, key=cluster_vol.__getitem__))}

        label_names = {0: "Relatif Stabil", 1: "Volatil Sedang", 2: "Risiko Tinggi", 3: "Risiko Ekstrem"}
        distances = np.linalg.norm(X_scaled - best_model.cluster_centers_[raw_labels], axis=1)

        clusters: dict[str, RegionCluster] = {}
        for i, code in enumerate(codes):
            ordered_id = rank_map[int(raw_labels[i])]
            clusters[code] = RegionCluster(
                regency_code=code,
                cluster_id=ordered_id,
                centroid_distance=round(float(distances[i]), 4),
                volatility_score=round(float(X_raw[i, 0]), 6),
                climate_exposure_score=round(float(X_raw[i, 2]), 4),
                cluster_label=label_names.get(ordered_id, f"Klaster {ordered_id}"),
            )
        return clusters
