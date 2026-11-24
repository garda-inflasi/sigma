"""Environment configuration loader for the Garda ML pipeline & inference services."""

import os
from collections.abc import Mapping
from dataclasses import dataclass


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

    mongo_uri: str
    db_unseen: str = "dataunseen"
    db_safe: str = "datasafe"
    n_jobs: int = 2
    max_train_rows: int = 150_000
    history_days: int = 1100
    output_history_length: int = 60
    selection_interval_hours: int = 24
    lease_seconds: int = 300
    schedule_offset_minutes: int = 5
    force_hgb_fallback: bool = False
    control_port: int = 8092
    artifact_dir: str = ""
    code_version: str = ""

    @classmethod
    def from_env(cls, env: Mapping[str, str] | None = None) -> "PipelineSettings":
        """Load pipeline settings supporting all legacy deployment env vars."""
        source = os.environ if env is None else env
        uri = source.get("MONGO_URI", "").strip() or source.get("MONGO_URL", "").strip()
        if not uri:
            raise ValueError("MONGO_URI (atau MONGO_URL) wajib diisi")

        return cls(
            mongo_uri=uri,
            db_unseen=source.get("MONGO_DB_UNSEEN", "").strip() or "dataunseen",
            db_safe=source.get("MONGO_DB_SAFE", "").strip() or "datasafe",
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
            artifact_dir=source.get("ML_ARTIFACT_DIR", "").strip(),
            code_version=source.get("GIT_SHA", "").strip(),
        )
