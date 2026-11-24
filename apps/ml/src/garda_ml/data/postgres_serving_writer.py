"""PostgreSQL serving writer (`ml_predictions`, `region_clusters`) + Parquet batch exporter."""

from __future__ import annotations

import io
import json
from dataclasses import dataclass
from datetime import date
from pathlib import Path
from typing import Any

import pandas as pd
import psycopg
import pyarrow as pa
import pyarrow.parquet as pq


@dataclass(frozen=True)
class ServingWriteSummary:
    """Count of persisted predictions/clusters and exported batch Parquet URIs."""

    predictions_written: int
    clusters_written: int
    predictions_parquet_uri: str | None = None
    clusters_parquet_uri: str | None = None


class PostgresServingWriter:
    """Atomically writes inference outputs to PostgreSQL (`ml_predictions`, `region_clusters`) & Parquet."""

    def __init__(
        self,
        *,
        postgres_url: str = "",
        gold_dir: str | Path | None = None,
        minio_endpoint: str = "",
        minio_access_key: str = "",
        minio_secret_key: str = "",
        minio_bucket: str = "data-gold",
        minio_secure: bool = False,
        s3_client: Any = None,
    ) -> None:
        self.postgres_url = postgres_url.strip()
        self.gold_dir = Path(gold_dir).resolve() if gold_dir else None
        self.minio_endpoint = minio_endpoint.strip()
        self.minio_bucket = minio_bucket.strip() or "data-gold"
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

    def _export_parquet(self, key: str, df: pd.DataFrame) -> str | None:
        if df.empty or (self._s3 is None and self.gold_dir is None):
            return None
        raw = io.BytesIO()
        pq.write_table(pa.Table.from_pandas(df, preserve_index=False), raw, compression="snappy")
        payload = raw.getvalue()
        if self._s3 is not None:
            try:
                self._s3.head_bucket(Bucket=self.minio_bucket)
            except Exception:
                self._s3.create_bucket(Bucket=self.minio_bucket)
            self._s3.put_object(Bucket=self.minio_bucket, Key=key, Body=payload)
            return f"s3://{self.minio_bucket}/{key}"
        assert self.gold_dir is not None
        target = self.gold_dir / key
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(payload)
        return str(target)

    def write_cycle_outputs(
        self,
        *,
        cycle_id: str,
        batch_date: date,
        predictions: list[dict[str, Any]],
        clusters: list[dict[str, Any]],
        conn: Any = None,
    ) -> ServingWriteSummary:
        """Persist `ml_predictions` and `region_clusters` inside PostgreSQL transaction + batch Parquet."""
        pred_written = 0
        clus_written = 0

        if conn is not None or self.postgres_url:
            owns_conn = conn is None
            pg_conn = conn or psycopg.connect(self.postgres_url, connect_timeout=5)
            try:
                with pg_conn.cursor() as cur:
                    cur.execute("DELETE FROM ml_predictions WHERE cycle_id = %s", (cycle_id,))
                    cur.execute("DELETE FROM region_clusters WHERE cycle_id = %s", (cycle_id,))
                    if predictions:
                        cur.executemany(
                            """
                            INSERT INTO ml_predictions (
                                cycle_id, kode_bps, komoditas_kode, periode, horizon_hari,
                                task_type, yhat, yhat_lower, yhat_upper, risk_prob,
                                risk_level, model_family, model_version, parquet_version
                            )
                            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                            """,
                            [
                                (
                                    cycle_id,
                                    row["kode_bps"],
                                    row.get("komoditas_kode", "ALL"),
                                    row["periode"],
                                    int(row.get("horizon_hari", 7)),
                                    row.get("task_type", "FORECAST"),
                                    row.get("yhat"),
                                    row.get("yhat_lower"),
                                    row.get("yhat_upper"),
                                    row.get("risk_prob"),
                                    row.get("risk_level"),
                                    row["model_family"],
                                    row["model_version"],
                                    row.get("parquet_version", ""),
                                )
                                for row in predictions
                            ],
                        )
                        pred_written = len(predictions)
                    if clusters:
                        cur.executemany(
                            """
                            INSERT INTO region_clusters (
                                cycle_id, kode_bps, periode, cluster_id, cluster_label,
                                silhouette_score, centroid_distance, feature_summary, model_version
                            )
                            VALUES (%s, %s, %s, %s, %s, %s, %s, %s::jsonb, %s)
                            """,
                            [
                                (
                                    cycle_id,
                                    row["kode_bps"],
                                    row["periode"],
                                    int(row["cluster_id"]),
                                    row["cluster_label"],
                                    row.get("silhouette_score"),
                                    float(row["centroid_distance"]),
                                    json.dumps(row.get("feature_summary") or {}, ensure_ascii=False),
                                    row["model_version"],
                                )
                                for row in clusters
                            ],
                        )
                        clus_written = len(clusters)
                pg_conn.commit()
            finally:
                if owns_conn:
                    pg_conn.close()

        dt = batch_date.isoformat()
        pred_uri = self._export_parquet(
            f"predictions/dt={dt}/predictions.parquet",
            pd.DataFrame(predictions),
        )
        clus_uri = self._export_parquet(
            f"clusters/dt={dt}/clusters.parquet",
            pd.DataFrame(clusters),
        )
        return ServingWriteSummary(
            predictions_written=pred_written,
            clusters_written=clus_written,
            predictions_parquet_uri=pred_uri,
            clusters_parquet_uri=clus_uri,
        )
