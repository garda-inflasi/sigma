"""Optional PyTorch Spatio-Temporal Graph Neural Network spillover estimator ([deep] extra)."""

from typing import Any

import numpy as np

from garda_ml.domain.enums import ModelFamily


def is_torch_available() -> bool:
    """Return True if PyTorch runtime can be imported cleanly."""
    try:
        import torch  # noqa: F401
    except ImportError:
        return False
    return True


def _pinball_loss(pred: Any, target: Any, quantile: float) -> Any:
    import torch

    err = target - pred
    return torch.mean(torch.maximum(quantile * err, (quantile - 1.0) * err))


class SpatioTemporalGnnEstimator:
    """Multi-regency message-passing neural regressor using pinball loss.

    Registered as an optional `[deep]` plugin in `ModelRegistry`. Requires `torch`;
    without `torch`, instantiation or `fit()` raises a clear `ImportError`.
    """

    model_key = ModelFamily.PYTORCH_GNN

    def __init__(
        self,
        quantile: float = 0.5,
        hidden_dim: int = 32,
        epochs: int = 25,
        learning_rate: float = 1e-3,
        seed: int = 42,
    ) -> None:
        self.quantile = quantile
        self.hidden_dim = hidden_dim
        self.epochs = epochs
        self.learning_rate = learning_rate
        self.seed = seed
        self._net: Any = None
        self._col_means: np.ndarray | None = None
        self._col_stds: np.ndarray | None = None

    def fit(
        self,
        X: Any,
        y: Any,
        *,
        quantile: float | None = None,
        adjacency: np.ndarray | None = None,
        sample_weight: Any = None,
    ) -> "SpatioTemporalGnnEstimator":
        import torch
        import torch.nn as nn

        del sample_weight
        if quantile is not None:
            self.quantile = quantile

        torch.manual_seed(self.seed)
        x_np = np.asarray(X, dtype=np.float32)
        y_np = np.asarray(y, dtype=np.float32)

        self._col_means = np.nanmean(x_np, axis=0)
        self._col_stds = np.nanstd(x_np, axis=0)
        self._col_stds[self._col_stds < 1e-6] = 1.0
        x_norm = np.nan_to_num((x_np - self._col_means) / self._col_stds, nan=0.0)

        in_dim = x_norm.shape[1]

        class _GnnBlock(nn.Module):
            def __init__(self, features: int, hidden: int) -> None:
                super().__init__()
                self.proj = nn.Linear(features, hidden)
                self.message = nn.Linear(hidden, hidden)
                self.head = nn.Linear(hidden * 2, 1)

            def forward(self, x: torch.Tensor, adj: torch.Tensor | None) -> torch.Tensor:
                h = torch.relu(self.proj(x))
                if adj is not None and adj.shape[0] == h.shape[0]:
                    neighbor_msg = torch.relu(self.message(adj @ h))
                else:
                    neighbor_msg = torch.relu(self.message(h))
                return self.head(torch.cat([h, neighbor_msg], dim=-1)).squeeze(-1)

        net = _GnnBlock(in_dim, self.hidden_dim)
        optimizer = torch.optim.Adam(net.parameters(), lr=self.learning_rate)

        x_tensor = torch.from_numpy(x_norm)
        y_tensor = torch.from_numpy(y_np)
        adj_tensor = torch.from_numpy(np.asarray(adjacency, dtype=np.float32)) if adjacency is not None else None

        net.train()
        for _ in range(self.epochs):
            optimizer.zero_grad()
            out = net(x_tensor, adj_tensor)
            loss = _pinball_loss(out, y_tensor, self.quantile)
            loss.backward()
            optimizer.step()

        net.eval()
        self._net = net
        return self

    def predict(self, X: Any, adjacency: np.ndarray | None = None) -> np.ndarray:
        import torch

        if self._net is None or self._col_means is None or self._col_stds is None:
            raise RuntimeError("SpatioTemporalGnnEstimator belum dilatih")

        x_np = np.asarray(X, dtype=np.float32)
        x_norm = np.nan_to_num((x_np - self._col_means) / self._col_stds, nan=0.0)
        with torch.no_grad():
            x_tensor = torch.from_numpy(x_norm)
            adj_tensor = torch.from_numpy(np.asarray(adjacency, dtype=np.float32)) if adjacency is not None else None
            preds = self._net(x_tensor, adj_tensor).numpy()
        return np.asarray(preds, dtype=float)
