"""Unit tests for Medallion Silver (Pandera + dim_wilayah), Gold (DuckDB + Parquet), MinIO, MLflow & PG serving."""

from datetime import date
from pathlib import Path
from types import SimpleNamespace

import numpy as np
import pandas as pd
import pytest
from sklearn.preprocessing import StandardScaler

from garda_ml.data.artifact_store import ArtifactStore
from garda_ml.data.gold_reader import GoldFeatureReader
from garda_ml.data.postgres_serving_writer import PostgresServingWriter
from garda_ml.evaluation.mlflow_tracker import LoggedRunMetrics, MlflowExperimentTracker
from garda_ml.features.gold_builder import GoldFeatureStoreBuilder
from garda_ml.features.silver_transformer import (
    DimWilayahMapper,
    SilverBundle,
    parse_indonesian_decimal,
    transform_bps_bronze_to_silver,
    transform_djpk_bronze_to_silver,
)
from garda_ml.models.clustering.kmeans import RegencyKMeansClusterer


class FakeS3Client:
    def __init__(self) -> None:
        self.buckets: set[str] = set()
        self.objects: dict[tuple[str, str], bytes] = {}

    def head_bucket(self, Bucket: str) -> None:
        if Bucket not in self.buckets:
            raise FileNotFoundError(Bucket)

    def create_bucket(self, Bucket: str) -> None:
        self.buckets.add(Bucket)

    def put_object(self, Bucket: str, Key: str, Body: bytes, **_kwargs) -> None:
        self.objects[(Bucket, Key)] = Body

    def get_object(self, Bucket: str, Key: str) -> dict:
        if (Bucket, Key) not in self.objects:
            raise FileNotFoundError(Key)
        data = self.objects[(Bucket, Key)]
        return {"Body": SimpleNamespace(read=lambda: data)}


def test_silver_indonesian_decimal_and_cross_source_dim_wilayah():
    assert parse_indonesian_decimal("15.850,25") == pytest.approx(15850.25)
    assert parse_indonesian_decimal("105,42") == pytest.approx(105.42)
    assert np.isnan(parse_indonesian_decimal("-"))
    assert np.isnan(parse_indonesian_decimal("N/A"))

    mapper = DimWilayahMapper()
    assert mapper.resolve_bps("KAB ACEH TENGAH") == "1106"
    assert mapper.resolve_bps("MEULABOH") == "1107"
    assert mapper.resolve_bps("KAB ACEH TAMIANG") == "1114"
    assert mapper.resolve_djpk("01", "12") == "1101"  # Simeulue
    assert mapper.resolve_pihps(1) == "1171"  # Kota Banda Aceh

    bps_raw = {
        "var": [{"val": 2245, "label": "Indeks Harga Konsumen 150 Kab/Kota"}],
        "turvar": [{"val": "0", "label": "Tidak ada"}],
        "vervar": [{"val": 1, "label": "KAB ACEH TENGAH"}, {"val": 4, "label": "KOTA BANDA ACEH"}],
        "tahun": [{"val": 126, "label": "2026"}],
        "turtahun": [{"val": 1, "label": "Januari"}, {"val": 2, "label": "Februari"}],
        "datacontent": {"1224501261": "108,45", "1224501262": "-", "4224501261": "109,10"},
    }
    silver_bps = transform_bps_bronze_to_silver(bps_raw, mapper=mapper)
    assert set(silver_bps["kode_bps"]) == {"1106", "1171"}
    assert silver_bps["nilai"].isna().sum() == 1
    assert silver_bps.loc[silver_bps["kode_bps"] == "1171", "nilai"].iloc[0] == pytest.approx(109.10)

    djpk_silver = transform_djpk_bronze_to_silver(
        {"data": [{"uraian": "Belanja Modal", "realisasi": "125.000.000,50"}]},
        provinsi="01",
        pemda="12",
        tahun=2025,
        mapper=mapper,
    )
    assert list(djpk_silver["kode_bps"]) == ["1101"]
    assert djpk_silver["nilai"].iloc[0] == pytest.approx(125000000.50)


def test_silver_pandera_rejects_invalid_price_and_gold_duckdb_parquet(tmp_path: Path):
    bad_prices = pd.DataFrame(
        [{"kode_bps": "1171", "komoditas_kode": "BRS-PRE", "periode": pd.Timestamp("2026-01-01"), "nilai": -100.0, "sumber": "PIHPS"}]
    )
    with pytest.raises(Exception):
        SilverBundle(prices=bad_prices, macro=pd.DataFrame(), djpk=pd.DataFrame()).validated()

    dates = pd.date_range("2026-01-01", periods=35, freq="D")
    price_rows = []
    for kode in ("1101", "1106", "1171"):
        for idx, dt in enumerate(dates):
            price_rows.append(
                {
                    "kode_bps": kode,
                    "komoditas_kode": "BRS-PRE",
                    "periode": dt,
                    "nilai": float(14000 + idx * 25),
                    "sumber": "PIHPS",
                }
            )
    macro_df = pd.DataFrame(
        [
            {
                "kode_bps": "1171",
                "domain": "INFLASI",
                "indikator": "ihk_150",
                "periode": pd.Timestamp("2026-01-01"),
                "nilai": 107.4,
                "sumber": "BPS",
            }
        ]
    )
    djpk_df = pd.DataFrame(
        [
            {
                "kode_bps": "1101",
                "domain": "FISKAL_DJPK",
                "indikator": "apbd_belanja",
                "periode": pd.Timestamp("2025-01-01"),
                "nilai": 900_000_000.0,
                "sumber": "DJPK",
            }
        ]
    )
    bundle = SilverBundle(prices=pd.DataFrame(price_rows), macro=macro_df, djpk=djpk_df)

    # Uji export lokal DuckDB -> 3 berkas Parquet
    gold_dir = tmp_path / "gold"
    builder = GoldFeatureStoreBuilder(local_dir=gold_dir)
    manifest = builder.build_and_export(bundle)
    assert manifest.tabular_rows == 105
    assert manifest.gnn_node_rows == 105
    assert manifest.gnn_edge_rows >= 3

    reader = GoldFeatureReader(local_dir=gold_dir)
    tab_df = reader.read_tabular_features()
    nodes_df, edges_df = reader.read_gnn_graph()
    assert {"lag_1", "lag_7", "lag_30", "ma_7", "ma_30", "growth_mom", "ihk_terkini", "apbd_realisasi"} <= set(tab_df.columns)
    assert {"source_bps", "target_bps", "weight"} <= set(edges_df.columns)
    assert len(nodes_df) == 105

    # Uji export ke mock S3 MinIO bucket s3://data-gold/
    s3 = FakeS3Client()
    s3_builder = GoldFeatureStoreBuilder(minio_bucket="data-gold", s3_client=s3)
    s3_manifest = s3_builder.build_and_export(bundle)
    assert s3_manifest.tabular_uri == "s3://data-gold/features_tabular_v1.parquet"
    assert GoldFeatureReader(minio_bucket="data-gold", s3_client=s3).read_tabular_features().shape[0] == 105


def test_model_bundle_with_scaler_mlflow_and_serving_outputs(tmp_path: Path):
    # 1. KMeans + StandardScaler tersimpan bundel ke mock MinIO s3://ml-models/
    profiles = pd.DataFrame(
        [
            {"regency_code": "1101", "volatility_mean": 0.01, "abs_momentum_30": 0.02, "climate_exposure": 12.0},
            {"regency_code": "1106", "volatility_mean": 0.04, "abs_momentum_30": 0.08, "climate_exposure": 35.0},
            {"regency_code": "1171", "volatility_mean": 0.02, "abs_momentum_30": 0.03, "climate_exposure": 15.0},
            {"regency_code": "1174", "volatility_mean": 0.05, "abs_momentum_30": 0.09, "climate_exposure": 40.0},
        ]
    )
    clusterer = RegencyKMeansClusterer()
    clustered = clusterer.fit_predict(profiles)
    assert clusterer.scaler is not None and clusterer.model is not None

    s3 = FakeS3Client()
    store = ArtifactStore(minio_bucket="ml-models", s3_client=s3)
    uri = store.save_model_bundle(
        "C20261124T12Z",
        "kmeans_clusters",
        clusterer.model,
        preprocessor=clusterer.scaler,
        metadata={"silhouette_score": clusterer.best_silhouette},
    )
    assert uri == "s3://ml-models/C20261124T12Z/kmeans_clusters.joblib"
    loaded = store.load_model_bundle("C20261124T12Z", "kmeans_clusters")
    assert isinstance(loaded["preprocessor"], StandardScaler)

    # 2. Mock MLflow logger
    recorded: dict[str, object] = {}

    class FakeRunCtx:
        info = SimpleNamespace(run_id="run-123")
        def __enter__(self): return self
        def __exit__(self, *_): return None

    class FakeMlflow:
        def start_run(self, run_name=None):
            recorded["run_name"] = run_name
            return FakeRunCtx()
        def set_tags(self, tags): recorded["tags"] = tags
        def log_params(self, params): recorded["params"] = params
        def log_metrics(self, metrics): recorded["metrics"] = metrics

    tracker = MlflowExperimentTracker(tracking_uri="postgresql://mlflow", mlflow_module=FakeMlflow())
    run_id = tracker.log_training_run(
        LoggedRunMetrics(
            run_name="cycle-C20261124T12Z",
            cycle_id="C20261124T12Z",
            model_family="lightgbm",
            gold_dataset_version="abc12345",
            params={"n_estimators": 100},
            metrics={"rmse": 120.5, "silhouette_score": clusterer.best_silhouette or 0.0},
        )
    )
    assert run_id == "run-123"
    assert recorded["tags"]["gold_dataset_version"] == "abc12345"

    # 3. PostgreSQL serving cursor + Parquet export
    executed_sql: list[str] = []

    class FakeCursor:
        def __enter__(self): return self
        def __exit__(self, *_): return None
        def execute(self, sql, _params=None): executed_sql.append(sql)
        def executemany(self, sql, _rows): executed_sql.append(sql)

    class FakeConn:
        def __init__(self): self.committed = False
        def cursor(self): return FakeCursor()
        def commit(self): self.committed = True

    pg_conn = FakeConn()
    writer = PostgresServingWriter(gold_dir=tmp_path / "gold")
    summary = writer.write_cycle_outputs(
        cycle_id="C20261124T12Z",
        batch_date=date(2026, 11, 24),
        predictions=[
            {
                "kode_bps": "1171",
                "komoditas_kode": "BRS-PRE",
                "periode": date(2026, 12, 1),
                "horizon_hari": 7,
                "task_type": "FORECAST",
                "yhat": 15200.0,
                "yhat_lower": 14900.0,
                "yhat_upper": 15500.0,
                "model_family": "lightgbm",
                "model_version": "C20261124T12Z",
                "parquet_version": "abc12345",
            }
        ],
        clusters=[
            {
                "kode_bps": "1171",
                "periode": date(2026, 11, 24),
                "cluster_id": clustered["1171"].cluster_id,
                "cluster_label": clustered["1171"].cluster_label,
                "silhouette_score": clusterer.best_silhouette,
                "centroid_distance": clustered["1171"].centroid_distance,
                "feature_summary": {"volatility_score": clustered["1171"].volatility_score},
                "model_version": "C20261124T12Z",
            }
        ],
        conn=pg_conn,
    )
    assert pg_conn.committed
    assert summary.predictions_written == 1
    assert summary.clusters_written == 1
    assert Path(summary.predictions_parquet_uri).is_file()
    assert Path(summary.clusters_parquet_uri).is_file()


def test_bronze_reader_to_silver_and_gold_macro_without_dataunseen(tmp_path: Path):
    import gzip
    import json
    from datetime import UTC, datetime

    from garda_ml.config.settings import PipelineSettings
    from garda_ml.data.bronze_reader import BronzeReader
    from garda_ml.features.silver_transformer import SilverParquetStore

    settings = PipelineSettings.from_env({})
    assert settings.db_safe == "ml_predictions"
    assert settings.artifact_dir.endswith("models")

    raw_dir = tmp_path / "raw"
    bps_path = raw_dir / "bps_tables" / "bps_tbl_69_tpt_tpak_aceh" / "dt=2026-11-24" / "raw.json.gz"
    bps_path.parent.mkdir(parents=True, exist_ok=True)
    bps_payload = {
        "metadata": {
            "kategori_data": "ihk",
            "judul_tabel": "IHK Makanan",
            "url_sumber": "https://www.bps.go.id/contoh",
            "waktu_crawl": "2026-11-24T12:00:00+00:00",
        },
        "records": [
            {"kode_bps": "1171", "wilayah": "Kota Banda Aceh", "tahun": 2026, "periode": "Januari", "nilai": "108,25", "sub_kelompok": "Makanan", "komoditi": None},
            {"kode_bps": "1106", "wilayah": "Kab. Aceh Tengah", "tahun": 2026, "periode": "Januari", "nilai": "110,50", "sub_kelompok": "Makanan", "komoditi": None},
        ],
    }
    bps_path.write_bytes(gzip.compress(json.dumps(bps_payload).encode("utf-8")))

    reader = BronzeReader(data_raw_dir=raw_dir)
    bundle, market = reader.load_silver_and_market(cutoff=datetime(2026, 11, 24, 23, 0, tzinfo=UTC))
    assert not bundle.macro.empty
    assert set(bundle.macro["kode_bps"]) == {"1106", "1171"}
    assert not market.latest_macro.empty
    assert len(market.freshness) == 1

    silver_uris = SilverParquetStore(local_dir=tmp_path / "silver").write_bundle(bundle)
    assert Path(silver_uris["silver_macro.parquet"]).is_file()

    gold_dir = tmp_path / "gold"
    manifest = GoldFeatureStoreBuilder(local_dir=gold_dir).build_and_export(bundle)
    assert manifest.macro_rows == 2
    assert manifest.gnn_node_rows == 2
    assert Path(manifest.macro_uri).is_file()
    macro_gold = GoldFeatureReader(local_dir=gold_dir).read_macro_features()
    assert len(macro_gold) == 2
