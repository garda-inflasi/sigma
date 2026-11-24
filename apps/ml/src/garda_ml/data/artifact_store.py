"""Versioned artifact serialization registry for scikit-learn/LightGBM/PyTorch models."""

import json
from pathlib import Path
from typing import Any

from garda_ml.domain.errors import ArtifactLoadError


class ArtifactStore:
    """Stores and loads trained models & metadata keyed by `cycle_id` and `model_key`."""

    def __init__(self, root_dir: str | Path | None = None) -> None:
        self.root = Path(root_dir) if root_dir else None

    @property
    def enabled(self) -> bool:
        return self.root is not None

    def _artifact_path(self, cycle_id: str, name: str, suffix: str) -> Path:
        if self.root is None:
            raise ArtifactLoadError("artifact_dir belum dikonfigurasi")
        return self.root / cycle_id / f"{name}{suffix}"

    def save_joblib(self, cycle_id: str, name: str, obj: Any, metadata: dict[str, Any] | None = None) -> Path | None:
        """Persist tabular estimator & optional JSON manifest if `artifact_dir` is enabled."""
        if self.root is None:
            return None
        import joblib

        target = self._artifact_path(cycle_id, name, ".joblib")
        target.parent.mkdir(parents=True, exist_ok=True)
        joblib.dump(obj, target, compress=3)
        if metadata is not None:
            meta_path = target.with_suffix(".meta.json")
            meta_path.write_text(json.dumps(metadata, default=str, indent=2), encoding="utf-8")
        return target

    def load_joblib(self, cycle_id: str, name: str) -> Any:
        """Load persisted joblib artifact or raise ArtifactLoadError."""
        import joblib

        target = self._artifact_path(cycle_id, name, ".joblib")
        if not target.exists():
            raise ArtifactLoadError(f"artefak tidak ditemukan: {target}")
        return joblib.load(target)

    def save_torch(self, cycle_id: str, name: str, state_dict: Any, metadata: dict[str, Any] | None = None) -> Path | None:
        """Persist PyTorch checkpoint cleanly when torch is installed."""
        if self.root is None:
            return None
        try:
            import torch
        except ImportError as exc:
            raise ArtifactLoadError("torch tidak terpasang untuk menyimpan checkpoint .pt") from exc

        target = self._artifact_path(cycle_id, name, ".pt")
        target.parent.mkdir(parents=True, exist_ok=True)
        torch.save({"state_dict": state_dict, "metadata": metadata or {}}, target)
        return target
