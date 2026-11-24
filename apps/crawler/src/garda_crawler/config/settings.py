"""Environment-driven settings for Garda Inflasi crawler."""

import json
import os
from dataclasses import dataclass, field
from urllib.parse import quote_plus


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

    mongo_uri: str
    mongo_db_unseen: str = "dataunseen"
    user_agent: str = "garda-crawler/1.0 (Garda Inflasi Aceh; kontak admin)"
    request_delay_ms: int = 800
    bps_api_key: str = ""
    bps_variables: dict[str, str] = field(default_factory=dict)
    bapanas_api_key: str = ""
    bapanas_base_url: str = "https://panelharga.badanpangan.go.id/api/front"
    sp2kp_api_key: str = ""
    postgres_url: str = ""
    heartbeat_path: str = "/tmp/garda_crawler_detak"
    control_port: int = 8091

    @classmethod
    def from_env(cls) -> "CrawlerSettings":
        """Load crawler configuration with backward-compatible env fallbacks."""
        mongo_uri = os.environ.get("MONGO_URI") or os.environ.get("MONGO_URL") or ""
        if not mongo_uri:
            raise RuntimeError("MONGO_URI atau MONGO_URL wajib diisi")

        postgres_url = (
            os.environ.get("CRAWLER_DATABASE_URL", "").strip()
            or _build_postgres_url_from_parts()
            or os.environ.get("DATABASE_URL", "").strip()
        )
        return cls(
            mongo_uri=mongo_uri,
            mongo_db_unseen=os.environ.get("MONGO_DB_UNSEEN", "dataunseen"),
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
            postgres_url=postgres_url,
            heartbeat_path=os.environ.get("CRAWLER_BERKAS_DETAK", "/tmp/garda_crawler_detak"),
            control_port=int(os.environ.get("CRAWLER_CONTROL_PORT", "8091")),
        )

    @property
    def secrets(self) -> list[str]:
        """Sensitive credential values that must always be masked in logs/storage."""
        values: list[str] = []
        for candidate in (self.bps_api_key, self.bapanas_api_key, self.sp2kp_api_key, self.postgres_url):
            if candidate and candidate not in values:
                values.append(candidate)
        return values
