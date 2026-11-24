"""Konfigurasi dari variabel lingkungan. Nilai rahasia tidak pernah dicetak."""

import json
import os
from collections.abc import Mapping
from dataclasses import dataclass, field
from pathlib import Path

USER_AGENT_BAWAAN = "garda-crawler/1.0"
JEDA_MS_BAWAAN = 1500


class GalatKonfigurasi(ValueError):
    pass


@dataclass(frozen=True)
class Konfigurasi:
    mongo_uri: str
    db_unseen: str = "dataunseen"
    user_agent: str = USER_AGENT_BAWAAN
    jeda_ms: int = JEDA_MS_BAWAAN
    timeout_detik: float = 45.0
    percobaan: int = 4
    bps_api_key: str = field(default="", repr=False)
    bps_variabel: dict[str, str] = field(default_factory=dict)
    postgres: dict[str, str] | str | None = field(default=None, repr=False)
    berkas_detak: Path = Path("/tmp/garda-crawler-detak")

    @classmethod
    def dari_env(cls, env: Mapping[str, str] | None = None) -> "Konfigurasi":
        env = os.environ if env is None else env
        mongo_uri = env.get("MONGO_URI") or env.get("MONGO_URL") or ""
        if not mongo_uri:
            raise GalatKonfigurasi("MONGO_URI (atau MONGO_URL) belum diisi")

        jeda_ms = JEDA_MS_BAWAAN
        if mentah := env.get("INGEST_JEDA_MS"):
            try:
                jeda_ms = int(mentah)
            except ValueError as exc:
                raise GalatKonfigurasi("INGEST_JEDA_MS harus bilangan bulat") from exc
            if jeda_ms < 0:
                raise GalatKonfigurasi("INGEST_JEDA_MS tidak boleh negatif")

        bps_variabel: dict[str, str] = {}
        if mentah := env.get("BPS_VARIABEL_JSON"):
            try:
                terbaca = json.loads(mentah)
            except json.JSONDecodeError as exc:
                raise GalatKonfigurasi("BPS_VARIABEL_JSON bukan JSON yang sah") from exc
            if not isinstance(terbaca, dict):
                raise GalatKonfigurasi("BPS_VARIABEL_JSON harus objek {indikator: id_variabel}")
            bps_variabel = {str(k): str(v) for k, v in terbaca.items()}

        return cls(
            mongo_uri=mongo_uri,
            db_unseen=env.get("MONGO_DB_UNSEEN") or "dataunseen",
            user_agent=env.get("INGEST_USER_AGENT") or USER_AGENT_BAWAAN,
            jeda_ms=jeda_ms,
            bps_api_key=(env.get("BPS_API_KEY") or "").strip(),
            bps_variabel=bps_variabel,
            postgres=_postgres(env),
            berkas_detak=Path(env.get("CRAWLER_BERKAS_DETAK") or "/tmp/garda-crawler-detak"),
        )


def _postgres(env: Mapping[str, str]) -> dict[str, str] | str | None:
    """CRAWLER_DATABASE_URL (peran baca-saja) > DB_* > DATABASE_URL."""
    if url := env.get("CRAWLER_DATABASE_URL"):
        return url
    if env.get("DB_HOST") and env.get("DB_NAME"):
        return {
            "host": env["DB_HOST"],
            "port": env.get("DB_PORT") or "5432",
            "user": env.get("DB_USER") or "",
            "password": env.get("DB_PASSWORD") or "",
            "dbname": env["DB_NAME"],
            "sslmode": env.get("DB_SSLMODE") or "prefer",
        }
    return env.get("DATABASE_URL") or None
