"""Environment-driven settings for Garda Inflasi crawler."""

import json
import os
from dataclasses import dataclass, field
from pathlib import Path
from urllib.parse import quote_plus


def _default_data_raw_dir() -> str:
    # Arahkan default Bronze lokal ke <repo_root>/data/raw, bukan di dalam folder apps/crawler.
    here = Path(__file__).resolve()
    for parent in here.parents:
        if (parent / "apps").is_dir() and (parent / "db").is_dir():
            return str(parent / "data" / "raw")
    return "/tmp/garda_data/raw"


def _build_postgres_url_from_parts() -> str:
    host = os.environ.get("DB_HOST", "")
    name = os.environ.get("DB_NAME", "")
    user = os.environ.get("DB_USER", "")
    if not (host and name and user):
        return ""
    port = os.environ.get("DB_PORT", "5432")
    password = os.environ.get("DB_PASSWORD", "")
    sslmode = os.environ.get("DB_SSLMODE", "disable")
    auth = f"{quote_plus(user)}:{quote_plus(password)}" if password else quote_plus(user)
    return f"postgresql://{auth}@{host}:{port}/{name}?sslmode={sslmode}"


def _parse_bps_variable_map(raw: str) -> dict[str, str]:
    raw = raw.strip()
    if not raw:
        return {}
    parsed = json.loads(raw)
    if not isinstance(parsed, dict):
        raise ValueError("BPS_VARIABEL_JSON harus berupa objek JSON")
    return {str(k): str(v) for k, v in parsed.items() if str(v).strip()}


@dataclass(frozen=True)
class CrawlerSettings:
    """Immutable runtime settings loaded from environment variables."""

    mongo_uri: str = ""
    mongo_db_unseen: str = "dataunseen"
    bronze_only: bool = True
    user_agent: str = "garda-crawler/1.0 (Garda Inflasi Aceh; kontak admin)"
    request_delay_ms: int = 800
    bps_api_key: str = ""
    bps_variables: dict[str, str] = field(default_factory=dict)
    bapanas_api_key: str = ""
    bapanas_base_url: str = "https://panelharga.badanpangan.go.id/api/front"
    sp2kp_api_key: str = ""
    djpk_base_url: str = "https://djpk.kemenkeu.go.id/portal/data/apbd"
    postgres_url: str = ""
    bronze_dir: str = field(default_factory=_default_data_raw_dir)
    bronze_primary: bool = True
    minio_endpoint: str = ""
    minio_access_key: str = ""
    minio_secret_key: str = ""
    minio_bucket_raw: str = "data-raw"
    minio_secure: bool = False
    heartbeat_path: str = "/tmp/garda_crawler_detak"
    control_port: int = 8091

    @classmethod
    def from_env(cls, *, require_mongo: bool = False) -> "CrawlerSettings":
        """Load crawler configuration targeting Medallion Bronze + PostgreSQL (`crawl_logs`)."""
        mongo_uri = os.environ.get("MONGO_URI") or os.environ.get("MONGO_URL") or ""
        bronze_only = os.environ.get("CRAWLER_BRONZE_ONLY", "true").strip().lower() not in {"0", "false", "no"}
        if require_mongo and not bronze_only and not mongo_uri:
            raise RuntimeError("MONGO_URI atau MONGO_URL wajib diisi")

        postgres_url = (
            os.environ.get("CRAWLER_DATABASE_URL", "").strip()
            or _build_postgres_url_from_parts()
            or os.environ.get("DATABASE_URL", "").strip()
        )
        raw_dir = (
            os.environ.get("CRAWLER_BRONZE_DIR", "").strip()
            or os.environ.get("DATA_RAW_DIR", "").strip()
            or _default_data_raw_dir()
        )
        return cls(
            mongo_uri=mongo_uri,
            mongo_db_unseen=os.environ.get("MONGO_DB_UNSEEN", "dataunseen"),
            bronze_only=bronze_only,
            user_agent=os.environ.get(
                "INGEST_USER_AGENT",
                "garda-crawler/1.0 (Garda Inflasi Aceh; kontak admin)",
            ),
            request_delay_ms=max(0, int(os.environ.get("INGEST_JEDA_MS", "800"))),
            bps_api_key=os.environ.get("BPS_API_KEY", "").strip(),
            bps_variables=_parse_bps_variable_map(os.environ.get("BPS_VARIABEL_JSON", "")),
            bapanas_api_key=os.environ.get("BAPANAS_API_KEY", "").strip(),
            bapanas_base_url=os.environ.get(
                "BAPANAS_BASE_URL",
                "https://panelharga.badanpangan.go.id/api/front",
            ).strip(),
            sp2kp_api_key=os.environ.get("SP2KP_API_KEY", "").strip(),
            djpk_base_url=os.environ.get(
                "DJPK_BASE_URL",
                "https://djpk.kemenkeu.go.id/portal/data/apbd",
            ).strip(),
            postgres_url=postgres_url,
            bronze_dir=raw_dir,
            bronze_primary=os.environ.get("CRAWLER_BRONZE_PRIMARY", "true").strip().lower() not in {"0", "false", "no"},
            minio_endpoint=os.environ.get("MINIO_ENDPOINT", "").strip(),
            minio_access_key=os.environ.get("MINIO_ACCESS_KEY", "").strip(),
            minio_secret_key=os.environ.get("MINIO_SECRET_KEY", "").strip(),
            minio_bucket_raw=os.environ.get("MINIO_BUCKET_RAW", "data-raw").strip() or "data-raw",
            minio_secure=os.environ.get("MINIO_SECURE", "false").strip().lower() in {"1", "true", "yes"},
            heartbeat_path=os.environ.get("CRAWLER_BERKAS_DETAK", "/tmp/garda_crawler_detak"),
            control_port=int(os.environ.get("CRAWLER_CONTROL_PORT", "8091")),
        )

    @property
    def secrets(self) -> list[str]:
        """Sensitive credential values that must always be masked in logs/storage."""
        values: list[str] = []
        for candidate in (
            self.bps_api_key,
            self.bapanas_api_key,
            self.sp2kp_api_key,
            self.postgres_url,
            self.minio_secret_key,
        ):
            if candidate and candidate not in values:
                values.append(candidate)
        return values
