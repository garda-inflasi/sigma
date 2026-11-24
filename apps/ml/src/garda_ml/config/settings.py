"""Environment configuration loader for the Garda ML pipeline & inference services."""

import os
from collections.abc import Mapping
from dataclasses import dataclass, field
from pathlib import Path
from urllib.parse import quote_plus


def _default_data_dir(subdir: str) -> str:
    here = Path(__file__).resolve()
    for parent in here.parents:
        if (parent / "apps").is_dir() and (parent / "db").is_dir():
            return str(parent / "data" / subdir)
    return f"/tmp/garda_data/{subdir}"


def _build_postgres_url_from_env(source: Mapping[str, str]) -> str:
    host = source.get("DB_HOST", "").strip()
    name = source.get("DB_NAME", "").strip()
    user = source.get("DB_USER", "").strip()
    if not (host and name and user):
        return ""
    port = source.get("DB_PORT", "5432").strip() or "5432"
    password = source.get("DB_PASSWORD", "")
    sslmode = source.get("DB_SSLMODE", "disable").strip() or "disable"
    auth = f"{quote_plus(user)}:{quote_plus(password)}" if password else quote_plus(user)
    return f"postgresql://{auth}@{host}:{port}/{name}?sslmode={sslmode}"


def _parse_int(env: Mapping[str, str], name: str, default: int, minimum: int = 1) -> int:
    raw = env.get(name, "").strip()
    if not raw:
        return default
    try:
        parsed = int(raw)
    except ValueError as err:
        raise ValueError(f"{name} harus bilangan bulat, didapat {raw!r}") from err
    if parsed < minimum:
        raise ValueError(f"{name} minimal {minimum}, didapat {parsed}")
    return parsed


@dataclass(frozen=True)
class PipelineSettings:
    """Immutable runtime settings loaded from environment variables."""

    mongo_uri: str = ""
    db_unseen: str = "dataunseen"
    db_safe: str = "ml_predictions"
    n_jobs: int = 2
    max_train_rows: int = 150_000
    history_days: int = 1100
    output_history_length: int = 60
    selection_interval_hours: int = 24
    lease_seconds: int = 300
    schedule_offset_minutes: int = 5
    force_hgb_fallback: bool = False
    control_port: int = 8092
    artifact_dir: str = field(default_factory=lambda: _default_data_dir("models"))
    code_version: str = ""
    postgres_url: str = ""
    data_raw_dir: str = field(default_factory=lambda: _default_data_dir("raw"))
    data_silver_dir: str = field(default_factory=lambda: _default_data_dir("silver"))
    data_gold_dir: str = field(default_factory=lambda: _default_data_dir("gold"))
    minio_endpoint: str = ""
    minio_access_key: str = ""
    minio_secret_key: str = ""
    minio_bucket_raw: str = "data-raw"
    minio_bucket_silver: str = "data-silver"
    minio_bucket_gold: str = "data-gold"
    minio_bucket_models: str = "ml-models"
    minio_secure: bool = False
    mlflow_tracking_uri: str = ""
    mlflow_experiment: str = "garda-inflasi"

    @classmethod
    def from_env(cls, env: Mapping[str, str] | None = None) -> "PipelineSettings":
        """Load pipeline settings for Medallion Bronze -> Silver -> Gold -> ml_predictions."""
        source = os.environ if env is None else env
        uri = source.get("MONGO_URI", "").strip() or source.get("MONGO_URL", "").strip()
        minio_endpoint = source.get("MINIO_ENDPOINT", "").strip()
        raw_artifact = source.get("ML_ARTIFACT_DIR", "").strip()
        if raw_artifact.lower() in {"off", "none", "disabled"}:
            artifact_dir = ""
        elif raw_artifact:
            artifact_dir = raw_artifact
        elif minio_endpoint:
            artifact_dir = ""
        else:
            artifact_dir = _default_data_dir("models")

        postgres_url = (
            source.get("ML_DATABASE_URL", "").strip()
            or source.get("DATABASE_URL", "").strip()
            or _build_postgres_url_from_env(source)
        )
        db_serving = (
            source.get("MONGO_DB_PREDICTIONS", "").strip()
            or source.get("MONGO_DB_SAFE", "").strip()
            or "ml_predictions"
        )

        return cls(
            mongo_uri=uri,
            db_unseen=source.get("MONGO_DB_UNSEEN", "").strip() or "dataunseen",
            db_safe=db_serving,
            n_jobs=_parse_int(source, "ML_N_JOBS", min(4, os.cpu_count() or 1)),
            max_train_rows=_parse_int(source, "ML_MAKS_BARIS_LATIH", 150_000, 1000),
            history_days=_parse_int(source, "ML_HARI_RIWAYAT", 1100, 60),
            output_history_length=_parse_int(source, "ML_PANJANG_RIWAYAT", 60),
            selection_interval_hours=_parse_int(source, "ML_INTERVAL_SELEKSI_JAM", 24),
            lease_seconds=_parse_int(source, "ML_LEASE_DETIK", 300, 30),
            schedule_offset_minutes=_parse_int(source, "ML_OFFSET_MENIT", 5, 0),
            force_hgb_fallback=source.get("ML_PAKSA_HGB", "").strip() == "1"
            or source.get("ML_FORCE_HGB_FALLBACK", "").strip() == "1",
            control_port=_parse_int(source, "ML_PIPELINE_CONTROL_PORT", 8092, 1),
            artifact_dir=artifact_dir,
            code_version=source.get("GIT_SHA", "").strip(),
            postgres_url=postgres_url,
            data_raw_dir=source.get("DATA_RAW_DIR", "").strip() or _default_data_dir("raw"),
            data_silver_dir=source.get("DATA_SILVER_DIR", "").strip() or _default_data_dir("silver"),
            data_gold_dir=source.get("DATA_GOLD_DIR", "").strip() or _default_data_dir("gold"),
            minio_endpoint=minio_endpoint,
            minio_access_key=source.get("MINIO_ACCESS_KEY", "").strip(),
            minio_secret_key=source.get("MINIO_SECRET_KEY", "").strip(),
            minio_bucket_raw=source.get("MINIO_BUCKET_RAW", "").strip() or "data-raw",
            minio_bucket_silver=source.get("MINIO_BUCKET_SILVER", "").strip() or "data-silver",
            minio_bucket_gold=source.get("MINIO_BUCKET_GOLD", "").strip() or "data-gold",
            minio_bucket_models=source.get("MINIO_BUCKET_MODELS", "").strip() or "ml-models",
            minio_secure=source.get("MINIO_SECURE", "").strip().lower() in {"1", "true", "yes"},
            mlflow_tracking_uri=source.get("MLFLOW_TRACKING_URI", "").strip(),
            mlflow_experiment=source.get("MLFLOW_EXPERIMENT", "").strip() or "garda-inflasi",
        )
