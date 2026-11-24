"""Versioned artifact serialization registry for MinIO (`s3://ml-models/`) and local storage."""

from __future__ import annotations

import io
import json
from pathlib import Path
from typing import Any

from garda_ml.domain.errors import ArtifactLoadError


class ArtifactStore:
    """Stores and loads versioned model bundles (`model + scaler/encoder`) in MinIO or local disk."""

    def __init__(
        self,
        root_dir: str | Path | None = None,
        *,
        minio_endpoint: str = "",
        minio_access_key: str = "",
        minio_secret_key: str = "",
        minio_bucket: str = "ml-models",
        minio_secure: bool = False,
        s3_client: Any = None,
    ) -> None:
        self.root = Path(root_dir) if root_dir else None
        self.minio_endpoint = minio_endpoint.strip()
        self.minio_bucket = minio_bucket.strip() or "ml-models"
        self._s3 = s3_client

        if self._s3 is None and self.minio_endpoint:
            import boto3
            from botocore.client import Config

            scheme = "https" if minio_secure else "http"
            endpoint_url = (
                self.minio_endpoint
                if self.minio_endpoint.startswith(("http://", "https://"))
                else f"{scheme}://{self.minio_endpoint}"
            )
            self._s3 = boto3.client(
                "s3",
                endpoint_url=endpoint_url,
                aws_access_key_id=minio_access_key,
                aws_secret_access_key=minio_secret_key,
                config=Config(signature_version="s3v4"),
            )

    @property
    def enabled(self) -> bool:
        return self.root is not None or self._s3 is not None

    @property
    def uses_s3(self) -> bool:
        return self._s3 is not None

    def _artifact_path(self, cycle_id: str, name: str, suffix: str) -> Path:
        if self.root is None:
            raise ArtifactLoadError("artifact_dir belum dikonfigurasi")
        return self.root / cycle_id / f"{name}{suffix}"

    def _s3_key(self, cycle_id: str, name: str, suffix: str) -> str:
        return f"{cycle_id}/{name}{suffix}"

    def _ensure_bucket(self) -> None:
        if self._s3 is None:
            return
        try:
            self._s3.head_bucket(Bucket=self.minio_bucket)
        except Exception:
            self._s3.create_bucket(Bucket=self.minio_bucket)

    def _put_bytes(self, cycle_id: str, name: str, suffix: str, raw: bytes, content_type: str) -> str | Path:
        if self._s3 is not None:
            self._ensure_bucket()
            key = self._s3_key(cycle_id, name, suffix)
            self._s3.put_object(Bucket=self.minio_bucket, Key=key, Body=raw, ContentType=content_type)
            return f"s3://{self.minio_bucket}/{key}"
        target = self._artifact_path(cycle_id, name, suffix)
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(raw)
        return target

    def _get_bytes(self, cycle_id: str, name: str, suffix: str) -> bytes:
        if self._s3 is not None:
            key = self._s3_key(cycle_id, name, suffix)
            try:
                obj = self._s3.get_object(Bucket=self.minio_bucket, Key=key)
                return obj["Body"].read()
            except Exception as exc:
                raise ArtifactLoadError(f"artefak S3 tidak ditemukan: s3://{self.minio_bucket}/{key}") from exc
        target = self._artifact_path(cycle_id, name, suffix)
        if not target.exists():
            raise ArtifactLoadError(f"artefak tidak ditemukan: {target}")
        return target.read_bytes()

    def save_joblib(self, cycle_id: str, name: str, obj: Any, metadata: dict[str, Any] | None = None) -> Path | str | None:
        """Persist tabular estimator/bundle & optional JSON manifest if storage is enabled."""
        if not self.enabled:
            return None
        import joblib

        buf = io.BytesIO()
        joblib.dump(obj, buf, compress=3)
        location = self._put_bytes(cycle_id, name, ".joblib", buf.getvalue(), "application/octet-stream")
        if metadata is not None:
            meta_bytes = json.dumps(metadata, default=str, indent=2).encode("utf-8")
            self._put_bytes(cycle_id, name, ".meta.json", meta_bytes, "application/json")
        return location

    def load_joblib(self, cycle_id: str, name: str) -> Any:
        """Load persisted joblib artifact/bundle or raise ArtifactLoadError."""
        import joblib

        raw = self._get_bytes(cycle_id, name, ".joblib")
        return joblib.load(io.BytesIO(raw))

    def save_model_bundle(
        self,
        cycle_id: str,
        name: str,
        model: Any,
        *,
        preprocessor: Any = None,
        metadata: dict[str, Any] | None = None,
    ) -> Path | str | None:
        """Save trained model together with its fitted Scaler/Encoder preprocessor."""
        if not self.enabled:
            return None

        bundle = {
            "model": model,
            "preprocessor": preprocessor,
            "metadata": metadata or {},
        }
        loc = self.save_joblib(cycle_id, name, bundle, metadata=metadata)

        # Jika LightGBM native booster tersedia, simpan juga model teks `.txt` untuk portabilitas.
        booster = getattr(model, "booster_", None)
        if booster is not None and hasattr(booster, "model_to_string"):
            txt_bytes = str(booster.model_to_string()).encode("utf-8")
            self._put_bytes(cycle_id, name, ".txt", txt_bytes, "text/plain")
        return loc

    def load_model_bundle(self, cycle_id: str, name: str) -> dict[str, Any]:
        """Load versioned `{model, preprocessor, metadata}` bundle."""
        loaded = self.load_joblib(cycle_id, name)
        if not isinstance(loaded, dict) or "model" not in loaded:
            return {"model": loaded, "preprocessor": None, "metadata": {}}
        return loaded

    def save_torch(
        self,
        cycle_id: str,
        name: str,
        state_dict: Any,
        metadata: dict[str, Any] | None = None,
        *,
        preprocessor: Any = None,
        use_safetensors: bool = False,
    ) -> Path | str | None:
        """Persist PyTorch GNN checkpoint (`.pt` or `.safetensors`) bundled with optional preprocessor."""
        if not self.enabled:
            return None
        try:
            import torch
        except ImportError as exc:
            raise ArtifactLoadError("torch tidak terpasang untuk menyimpan checkpoint .pt") from exc

        if use_safetensors:
            from safetensors.torch import save as st_save

            raw = st_save(state_dict)
            loc = self._put_bytes(cycle_id, name, ".safetensors", raw, "application/octet-stream")
            if preprocessor is not None or metadata is not None:
                self.save_joblib(cycle_id, f"{name}.preprocessor", {"preprocessor": preprocessor, "metadata": metadata or {}})
            return loc

        buf = io.BytesIO()
        torch.save({"state_dict": state_dict, "preprocessor": preprocessor, "metadata": metadata or {}}, buf)
        return self._put_bytes(cycle_id, name, ".pt", buf.getvalue(), "application/octet-stream")
