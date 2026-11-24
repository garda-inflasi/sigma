"""Spatial graph construction over Aceh regencies for PyTorch/GNN spillover estimation."""

import math
from dataclasses import dataclass

import numpy as np

# Canonical regency centroids across all 23 Aceh kabupaten/kota.
ACEH_REGENCY_COORDS: dict[str, tuple[float, float]] = {
    "1101": (2.48, 96.36), "1102": (2.28, 97.79), "1103": (3.25, 97.18), "1104": (3.49, 97.81),
    "1105": (4.96, 97.76), "1106": (4.63, 96.84), "1107": (4.14, 96.13), "1108": (5.38, 95.52),
    "1109": (5.38, 95.96), "1110": (5.20, 96.70), "1111": (5.14, 97.27), "1112": (3.74, 96.84),
    "1113": (3.98, 97.35), "1114": (4.29, 98.06), "1115": (4.18, 96.38), "1116": (4.65, 95.59),
    "1117": (4.73, 96.87), "1118": (5.22, 96.25), "1171": (5.55, 95.32), "1172": (5.89, 95.32),
    "1173": (4.47, 97.96), "1174": (5.18, 97.15), "1175": (2.64, 98.00),
}


@dataclass(frozen=True)
class SpatialGraph:
    """Row-normalized weighted adjacency graph for regency message passing."""

    nodes: tuple[str, ...]
    adjacency: np.ndarray  # shape (N, N) row-stochastic


def _haversine_km(c1: tuple[float, float], c2: tuple[float, float]) -> float:
    lat1, lon1 = map(math.radians, c1)
    lat2, lon2 = map(math.radians, c2)
    dlat = lat2 - lat1
    dlon = lon2 - lon1
    a = math.sin(dlat / 2) ** 2 + math.cos(lat1) * math.cos(lat2) * math.sin(dlon / 2) ** 2
    return 6371.0 * 2 * math.asin(min(1.0, math.sqrt(a)))


def build_regency_graph(regency_codes: list[str] | tuple[str, ...], bandwidth_km: float = 120.0) -> SpatialGraph:
    """Build inverse-distance Gaussian-kernel adjacency matrix with self-loops."""
    nodes = tuple(sorted(set(regency_codes)))
    n = len(nodes)
    if n == 0:
        return SpatialGraph(nodes=(), adjacency=np.zeros((0, 0), dtype=np.float32))

    weights = np.eye(n, dtype=np.float32)
    for i, r1 in enumerate(nodes):
        c1 = ACEH_REGENCY_COORDS.get(r1)
        for j, r2 in enumerate(nodes):
            if i == j or c1 is None:
                continue
            c2 = ACEH_REGENCY_COORDS.get(r2)
            if c2 is None:
                continue
            dist_km = _haversine_km(c1, c2)
            weights[i, j] = math.exp(-((dist_km / bandwidth_km) ** 2))

    row_sums = weights.sum(axis=1, keepdims=True)
    normalized = np.divide(weights, row_sums, out=np.eye(n, dtype=np.float32), where=row_sums > 0)
    return SpatialGraph(nodes=nodes, adjacency=normalized.astype(np.float32))
