"""MLflow experiment tracking wrapper recording metrics, params, and Gold Parquet dataset versions."""

from __future__ import annotations

import logging
from dataclasses import dataclass
from typing import Any

logger = logging.getLogger("garda_ml.mlflow")


@dataclass(frozen=True)
class LoggedRunMetrics:
    """Structured run payload logged into MLflow tracking server."""

    run_name: str
    cycle_id: str
    model_family: str
    gold_dataset_version: str
    params: dict[str, Any]
    metrics: dict[str, float]


class MlflowExperimentTracker:
    """Logs training hyperparameters, evaluation metrics, and Parquet lineage to MLflow."""

    def __init__(
        self,
        *,
        tracking_uri: str = "",
        experiment_name: str = "garda-inflasi",
        mlflow_module: Any = None,
    ) -> None:
        self.tracking_uri = tracking_uri.strip()
        self.experiment_name = experiment_name.strip() or "garda-inflasi"
        self._mlflow = mlflow_module

    @property
    def enabled(self) -> bool:
        return bool(self.tracking_uri) or self._mlflow is not None

    def _client(self) -> Any:
        if self._mlflow is not None:
            return self._mlflow
        import mlflow

        if self.tracking_uri:
            mlflow.set_tracking_uri(self.tracking_uri)
        mlflow.set_experiment(self.experiment_name)
        return mlflow

    def log_training_run(self, payload: LoggedRunMetrics) -> str | None:
        """Record parameters, metrics, and dataset version tags; return MLflow run_id if enabled."""
        if not self.enabled:
            return None
        try:
            client = self._client()
            with client.start_run(run_name=payload.run_name) as active_run:
                client.set_tags(
                    {
                        "cycle_id": payload.cycle_id,
                        "model_family": payload.model_family,
                        "gold_dataset_version": payload.gold_dataset_version,
                    }
                )
                clean_params = {k: str(v)[:250] for k, v in payload.params.items()}
                if clean_params:
                    client.log_params(clean_params)
                clean_metrics = {
                    k: float(v)
                    for k, v in payload.metrics.items()
                    if v is not None and float(v) == float(v)
                }
                if clean_metrics:
                    client.log_metrics(clean_metrics)
                return str(active_run.info.run_id)
        except Exception as exc:
            logger.warning("gagal mencatat metrik MLflow (%s): %s", payload.run_name, exc)
            return None
