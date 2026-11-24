"""PostgreSQL `crawl_logs` repository and deterministic SHA-256 idempotency helpers."""

import hashlib
import json
from dataclasses import dataclass
from datetime import datetime
from typing import Any

import psycopg

from garda_crawler.transport.time_utils import utc_now


def canonical_json_bytes(payload: Any) -> bytes:
    """Serialize any JSON-compatible payload deterministically into UTF-8 bytes."""
    if isinstance(payload, (bytes, bytearray)):
        return bytes(payload)
    return json.dumps(payload, sort_keys=True, ensure_ascii=False, separators=(",", ":")).encode("utf-8")


def compute_payload_sha256(payload: Any) -> str:
    """Return lowercase 64-char SHA-256 hex digest of an unaltered raw API payload."""
    return hashlib.sha256(canonical_json_bytes(payload)).hexdigest()


def compute_params_hash(params: dict[str, Any] | None) -> str:
    """Return deterministic SHA-256 digest of normalized request parameters."""
    return hashlib.sha256(canonical_json_bytes(params or {})).hexdigest()


@dataclass(frozen=True)
class CrawlLogEntry:
    """One immutable audit row recorded into PostgreSQL `crawl_logs`."""

    source_code: str
    dataset_code: str
    endpoint: str
    params_hash: str
    params_json: dict[str, Any]
    payload_sha256: str
    bronze_uri: str
    status: str  # WRITTEN | SKIPPED_DUPLICATE | ERROR
    byte_size: int
    error_message: str = ""
    crawled_at: datetime | None = None


class InMemoryCrawlLogRepository:
    """In-memory `crawl_logs` backend for unit tests or offline dry runs."""

    def __init__(self) -> None:
        self.entries: list[CrawlLogEntry] = []

    def latest_checksum(self, source_code: str, dataset_code: str, params_hash: str) -> str | None:
        for entry in reversed(self.entries):
            if (
                entry.source_code == source_code
                and entry.dataset_code == dataset_code
                and entry.params_hash == params_hash
                and entry.status in {"WRITTEN", "SKIPPED_DUPLICATE"}
            ):
                return entry.payload_sha256
        return None

    def record(self, entry: CrawlLogEntry) -> None:
        self.entries.append(entry)


class PostgresCrawlLogRepository:
    """PostgreSQL persistence for crawler idempotency checksums and Bronze lineage."""

    def __init__(self, dsn: str) -> None:
        self._dsn = dsn

    def latest_checksum(self, source_code: str, dataset_code: str, params_hash: str) -> str | None:
        """Return most recent successful `payload_sha256` for the exact endpoint parameters."""
        if not self._dsn:
            return None
        with psycopg.connect(self._dsn, connect_timeout=5) as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    SELECT payload_sha256
                      FROM crawl_logs
                     WHERE source_code = %s
                       AND dataset_code = %s
                       AND params_hash = %s
                       AND status IN ('WRITTEN', 'SKIPPED_DUPLICATE')
                     ORDER BY crawled_at DESC, id DESC
                     LIMIT 1
                    """,
                    (source_code, dataset_code, params_hash),
                )
                row = cur.fetchone()
        return str(row[0]).strip() if row and row[0] else None

    def record(self, entry: CrawlLogEntry) -> None:
        """Append one execution record into PostgreSQL `crawl_logs`."""
        if not self._dsn:
            return
        ts = entry.crawled_at or utc_now()
        with psycopg.connect(self._dsn, connect_timeout=5) as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    INSERT INTO crawl_logs (
                        source_code, dataset_code, endpoint, params_hash, params_json,
                        payload_sha256, bronze_uri, status, byte_size, error_message, crawled_at
                    )
                    VALUES (%s, %s, %s, %s, %s::jsonb, %s, %s, %s, %s, %s, %s)
                    """,
                    (
                        entry.source_code,
                        entry.dataset_code,
                        entry.endpoint,
                        entry.params_hash,
                        json.dumps(entry.params_json, sort_keys=True, ensure_ascii=False),
                        entry.payload_sha256,
                        entry.bronze_uri,
                        entry.status,
                        max(0, int(entry.byte_size)),
                        entry.error_message[:500],
                        ts,
                    ),
                )
            conn.commit()
