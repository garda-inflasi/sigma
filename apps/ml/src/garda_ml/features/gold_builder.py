"""Medallion Gold Feature Store: DuckDB in-memory analytical joins -> versioned Apache Parquet."""

from __future__ import annotations

import hashlib
import io
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import duckdb
import numpy as np
import pandas as pd
import pyarrow as pa
import pyarrow.parquet as pq

from garda_ml.features.graph_builder import ACEH_REGENCY_COORDS, build_regency_graph
from garda_ml.features.silver_transformer import SilverBundle

TABULAR_PARQUET_NAME = "features_tabular_v1.parquet"
MACRO_PARQUET_NAME = "macro_features_v1.parquet"
GNN_NODES_PARQUET_NAME = "gnn_node_features_v1.parquet"
GNN_EDGES_PARQUET_NAME = "gnn_edge_index_v1.parquet"


@dataclass(frozen=True)
class GoldBuildManifest:
    """Versioned paths and checksum of materialized Gold Parquet feature tables."""

    gold_dataset_version: str
    tabular_uri: str
    gnn_nodes_uri: str
    gnn_edges_uri: str
    tabular_rows: int
    gnn_node_rows: int
    gnn_edge_rows: int
    macro_uri: str = ""
    macro_rows: int = 0


def _duckdb_build_tabular_and_nodes(bundle: SilverBundle) -> tuple[pd.DataFrame, pd.DataFrame, pd.DataFrame]:
    """Use DuckDB SQL joins to compute price lags, macro aggregates, and GNN node features."""
    prices = bundle.prices.copy()
    macro = bundle.macro.copy()
    if macro.empty:
        macro = pd.DataFrame(columns=["kode_bps", "domain", "indikator", "periode", "nilai", "sumber"])
    djpk = bundle.djpk.copy()
    if djpk.empty:
        djpk = pd.DataFrame(columns=["kode_bps", "domain", "indikator", "periode", "nilai", "sumber"])

    con = duckdb.connect(":memory:")
    try:
        con.register("silver_macro", macro)
        con.register("silver_djpk", djpk)

        macro_features_df = con.execute(
            """
            WITH m_clean AS (
                SELECT
                    kode_bps,
                    domain,
                    indikator,
                    CAST(periode AS DATE) AS periode,
                    AVG(nilai) AS nilai
                FROM silver_macro
                WHERE nilai IS NOT NULL
                GROUP BY 1, 2, 3, 4
            )
            SELECT
                kode_bps,
                domain,
                indikator,
                periode,
                nilai,
                LAG(nilai, 1) OVER (PARTITION BY kode_bps, domain, indikator ORDER BY periode) AS nilai_prev,
                nilai - LAG(nilai, 1) OVER (PARTITION BY kode_bps, domain, indikator ORDER BY periode) AS delta_prev
            FROM m_clean
            ORDER BY kode_bps, domain, indikator, periode
            """
        ).df()

        if prices.empty:
            empty_tab = pd.DataFrame(
                columns=[
                    "kode_bps",
                    "komoditas_kode",
                    "periode",
                    "harga",
                    "lag_1",
                    "lag_7",
                    "lag_14",
                    "lag_30",
                    "ma_7",
                    "ma_30",
                    "growth_mom",
                    "growth_yoy",
                    "ihk_terkini",
                    "inflasi_yoy_terkini",
                    "inflasi_mtm_terkini",
                    "konsumsi_pengeluaran_rata",
                    "tpt_tpak_terkini",
                    "apbd_realisasi",
                ]
            )
            nodes_df = con.execute(
                """
                WITH m AS (
                    SELECT
                        kode_bps,
                        CAST(periode AS DATE) AS periode,
                        COALESCE(
                            AVG(CASE WHEN lower(indikator) LIKE '%indeks_harga_konsumen%' OR lower(indikator) LIKE '%ihk%' THEN nilai END),
                            AVG(nilai)
                        ) AS ihk_terkini,
                        AVG(CASE WHEN lower(indikator) LIKE '%yoy%' THEN nilai END) AS inflasi_yoy_terkini,
                        AVG(CASE WHEN lower(indikator) LIKE '%mtm%' THEN nilai END) AS inflasi_mtm_terkini,
                        AVG(CASE WHEN domain = 'KONSUMSI_PENDAPATAN' THEN nilai END) AS konsumsi_pengeluaran_rata,
                        AVG(CASE WHEN domain = 'TENAGA_KERJA' THEN nilai END) AS tpt_tpak_terkini
                    FROM silver_macro
                    WHERE nilai IS NOT NULL
                    GROUP BY 1, 2
                ),
                apbd AS (
                    SELECT kode_bps, SUM(nilai) AS apbd_realisasi
                    FROM silver_djpk
                    WHERE nilai IS NOT NULL
                    GROUP BY 1
                )
                SELECT
                    m.kode_bps,
                    m.periode,
                    CAST(NULL AS DOUBLE) AS harga_rata,
                    CAST(0.0 AS DOUBLE) AS volatilitas_7,
                    COALESCE(m.inflasi_mtm_terkini, 0.0) AS growth_mom_rata,
                    m.ihk_terkini,
                    m.inflasi_yoy_terkini,
                    m.inflasi_mtm_terkini,
                    m.konsumsi_pengeluaran_rata,
                    m.tpt_tpak_terkini,
                    a.apbd_realisasi
                FROM m
                LEFT JOIN apbd a ON m.kode_bps = a.kode_bps
                ORDER BY m.kode_bps, m.periode
                """
            ).df()
            return empty_tab, nodes_df, macro_features_df

        con.register("silver_prices", prices)
        tabular_df = con.execute(
            """
            WITH base AS (
                SELECT
                    kode_bps,
                    komoditas_kode,
                    CAST(periode AS DATE) AS periode,
                    AVG(nilai) AS harga
                FROM silver_prices
                WHERE nilai IS NOT NULL
                GROUP BY 1, 2, 3
            ),
            ts_features AS (
                SELECT
                    kode_bps,
                    komoditas_kode,
                    periode,
                    harga,
                    LAG(harga, 1) OVER w AS lag_1,
                    LAG(harga, 7) OVER w AS lag_7,
                    LAG(harga, 14) OVER w AS lag_14,
                    LAG(harga, 30) OVER w AS lag_30,
                    AVG(harga) OVER (PARTITION BY kode_bps, komoditas_kode ORDER BY periode ROWS BETWEEN 6 PRECEDING AND CURRENT ROW) AS ma_7,
                    AVG(harga) OVER (PARTITION BY kode_bps, komoditas_kode ORDER BY periode ROWS BETWEEN 29 PRECEDING AND CURRENT ROW) AS ma_30,
                    STDDEV_SAMP(harga) OVER (PARTITION BY kode_bps, komoditas_kode ORDER BY periode ROWS BETWEEN 6 PRECEDING AND CURRENT ROW) AS vol_7,
                    CASE WHEN LAG(harga, 30) OVER w > 0 THEN (harga - LAG(harga, 30) OVER w) / LAG(harga, 30) OVER w ELSE NULL END AS growth_mom,
                    CASE WHEN LAG(harga, 365) OVER w > 0 THEN (harga - LAG(harga, 365) OVER w) / LAG(harga, 365) OVER w ELSE NULL END AS growth_yoy
                FROM base
                WINDOW w AS (PARTITION BY kode_bps, komoditas_kode ORDER BY periode)
            ),
            ihk AS (
                SELECT
                    kode_bps,
                    CAST(periode AS DATE) AS periode,
                    COALESCE(
                        AVG(CASE WHEN lower(indikator) LIKE '%indeks_harga_konsumen%' OR lower(indikator) LIKE '%ihk%' THEN nilai END),
                        AVG(nilai)
                    ) AS ihk_terkini,
                    AVG(CASE WHEN lower(indikator) LIKE '%yoy%' THEN nilai END) AS inflasi_yoy_terkini,
                    AVG(CASE WHEN lower(indikator) LIKE '%mtm%' THEN nilai END) AS inflasi_mtm_terkini,
                    AVG(CASE WHEN domain = 'KONSUMSI_PENDAPATAN' THEN nilai END) AS konsumsi_pengeluaran_rata,
                    AVG(CASE WHEN domain = 'TENAGA_KERJA' THEN nilai END) AS tpt_tpak_terkini
                FROM silver_macro
                WHERE nilai IS NOT NULL
                GROUP BY 1, 2
            ),
            apbd AS (
                SELECT
                    kode_bps,
                    SUM(nilai) AS apbd_realisasi
                FROM silver_djpk
                WHERE nilai IS NOT NULL
                GROUP BY 1
            )
            SELECT
                t.*,
                i.ihk_terkini,
                i.inflasi_yoy_terkini,
                i.inflasi_mtm_terkini,
                i.konsumsi_pengeluaran_rata,
                i.tpt_tpak_terkini,
                a.apbd_realisasi
            FROM ts_features t
            ASOF LEFT JOIN ihk i
              ON t.kode_bps = i.kode_bps AND t.periode >= i.periode
            LEFT JOIN apbd a
              ON t.kode_bps = a.kode_bps
            ORDER BY t.kode_bps, t.komoditas_kode, t.periode
            """
        ).df()

        nodes_df = con.execute(
            """
            SELECT
                kode_bps,
                periode,
                AVG(harga) AS harga_rata,
                AVG(vol_7) AS volatilitas_7,
                AVG(growth_mom) AS growth_mom_rata,
                MAX(ihk_terkini) AS ihk_terkini,
                MAX(inflasi_yoy_terkini) AS inflasi_yoy_terkini,
                MAX(inflasi_mtm_terkini) AS inflasi_mtm_terkini,
                MAX(konsumsi_pengeluaran_rata) AS konsumsi_pengeluaran_rata,
                MAX(tpt_tpak_terkini) AS tpt_tpak_terkini,
                MAX(apbd_realisasi) AS apbd_realisasi
            FROM tabular_df
            GROUP BY 1, 2
            ORDER BY 1, 2
            """
        ).df() if not tabular_df.empty else pd.DataFrame()
        return tabular_df, nodes_df, macro_features_df
    finally:
        con.close()


def _build_edge_index_df(regency_codes: list[str]) -> pd.DataFrame:
    codes = sorted({c for c in regency_codes if c in ACEH_REGENCY_COORDS}) or sorted(ACEH_REGENCY_COORDS)
    graph = build_regency_graph(codes)
    rows: list[dict[str, Any]] = []
    for i, src in enumerate(graph.nodes):
        for j, dst in enumerate(graph.nodes):
            weight = float(graph.adjacency[i, j])
            if weight > 0.01:
                rows.append({"source_bps": src, "target_bps": dst, "weight": weight})
    return pd.DataFrame(rows)


def _parquet_bytes(df: pd.DataFrame) -> bytes:
    table = pa.Table.from_pandas(df, preserve_index=False)
    buf = io.BytesIO()
    pq.write_table(table, buf, compression="snappy")
    return buf.getvalue()


class GoldFeatureStoreBuilder:
    """Materializes versioned Gold Apache Parquet tables to MinIO (`s3://data-gold/`) or `data/gold/`."""

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

    def _ensure_bucket(self) -> None:
        if self._s3 is None:
            return
        try:
            self._s3.head_bucket(Bucket=self.minio_bucket)
        except Exception:
            self._s3.create_bucket(Bucket=self.minio_bucket)

    def _write_parquet(self, filename: str, df: pd.DataFrame) -> tuple[str, str]:
        raw = _parquet_bytes(df)
        digest = hashlib.sha256(raw).hexdigest()
        if self._s3 is not None:
            self._ensure_bucket()
            self._s3.put_object(
                Bucket=self.minio_bucket,
                Key=filename,
                Body=raw,
                ContentType="application/octet-stream",
            )
            return f"s3://{self.minio_bucket}/{filename}", digest
        if self.local_dir is None:
            raise RuntimeError("GoldFeatureStoreBuilder membutuhkan MinIO endpoint atau local_dir")
        target = self.local_dir / filename
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(raw)
        return str(target), digest

    def build_and_export(self, bundle: SilverBundle) -> GoldBuildManifest:
        """Transform validated Silver tables via DuckDB and export versioned Gold Apache Parquet tables."""
        clean = bundle.validated()
        tabular_df, nodes_df, macro_features_df = _duckdb_build_tabular_and_nodes(clean)
        region_source = tabular_df["kode_bps"].astype(str) if not tabular_df.empty else (
            nodes_df["kode_bps"].astype(str) if not nodes_df.empty else ACEH_REGENCY_COORDS
        )
        edges_df = _build_edge_index_df(sorted(set(region_source)))

        tab_uri, h1 = self._write_parquet(TABULAR_PARQUET_NAME, tabular_df)
        node_uri, h2 = self._write_parquet(GNN_NODES_PARQUET_NAME, nodes_df)
        edge_uri, h3 = self._write_parquet(GNN_EDGES_PARQUET_NAME, edges_df)
        macro_uri, h4 = self._write_parquet(MACRO_PARQUET_NAME, macro_features_df)

        version = hashlib.sha256(f"{h1}:{h2}:{h3}:{h4}".encode("utf-8")).hexdigest()[:16]
        return GoldBuildManifest(
            gold_dataset_version=version,
            tabular_uri=tab_uri,
            gnn_nodes_uri=node_uri,
            gnn_edges_uri=edge_uri,
            tabular_rows=len(tabular_df),
            gnn_node_rows=len(nodes_df),
            gnn_edge_rows=len(edges_df),
            macro_uri=macro_uri,
            macro_rows=len(macro_features_df),
        )
