"""Gold Feature Store reader consuming versioned Apache Parquet via PyArrow/DuckDB."""

from __future__ import annotations

import io
from pathlib import Path
from typing import Any

import duckdb
import pandas as pd
import pyarrow.parquet as pq

from garda_ml.features.gold_builder import (
    GNN_EDGES_PARQUET_NAME,
    GNN_NODES_PARQUET_NAME,
    MACRO_PARQUET_NAME,
    TABULAR_PARQUET_NAME,
)


class GoldFeatureReader:
    """Reads `features_tabular_v1.parquet`, `gnn_node_features_v1.parquet`, and `gnn_edge_index_v1.parquet`."""

    def __init__(
        self,
        *,
        local_dir: str | Path | None = None,
        minio_endpoint: str = "",
        minio_access_key: str = "",
        minio_secret_key: str = "",
        minio_bucket: str = "data-gold",
        minio_secure: bool = False,
        s3_client: Any = None,
    ) -> None:
        self.local_dir = Path(local_dir).resolve() if local_dir else None
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

    def _read_parquet(self, filename: str) -> pd.DataFrame:
        if self._s3 is not None:
            obj = self._s3.get_object(Bucket=self.minio_bucket, Key=filename)
            raw = obj["Body"].read()
            return pq.read_table(io.BytesIO(raw)).to_pandas()
        if self.local_dir is None:
            raise FileNotFoundError(f"lokasi Gold Parquet tidak dikonfigurasi ({filename})")
        target = self.local_dir / filename
        if not target.is_file():
            raise FileNotFoundError(f"berkas Gold Parquet tidak ditemukan: {target}")
        return duckdb.sql(f"SELECT * FROM read_parquet('{target.as_posix()}')").df()

    def read_tabular_features(self) -> pd.DataFrame:
        return self._read_parquet(TABULAR_PARQUET_NAME)

    def read_macro_features(self) -> pd.DataFrame:
        return self._read_parquet(MACRO_PARQUET_NAME)

    def read_gnn_graph(self) -> tuple[pd.DataFrame, pd.DataFrame]:
        nodes = self._read_parquet(GNN_NODES_PARQUET_NAME)
        edges = self._read_parquet(GNN_EDGES_PARQUET_NAME)
        return nodes, edges
