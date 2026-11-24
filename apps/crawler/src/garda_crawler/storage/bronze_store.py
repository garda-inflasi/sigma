"""Immutable Bronze layer writer storing compressed `.json.gz` payloads in MinIO or isolated `data/raw/`."""

import gzip
from dataclasses import dataclass
from datetime import date, datetime
from pathlib import Path
from typing import Any, Protocol

from garda_crawler.domain.entities import DataPoint, RawBronzeCapture, TimeSeriesSpec
from garda_crawler.storage.crawl_log_repository import (
    CrawlLogEntry,
    canonical_json_bytes,
    compute_params_hash,
    compute_payload_sha256,
)

_REPO_CRAWLER_MARKER = Path("apps") / "crawler"
_VOLATILE_HASH_KEYS = frozenset({"waktu_crawl"})


def _strip_volatile_hash_keys(value: Any) -> Any:
    if isinstance(value, dict):
        return {k: _strip_volatile_hash_keys(v) for k, v in value.items() if k not in _VOLATILE_HASH_KEYS}
    if isinstance(value, list):
        return [_strip_volatile_hash_keys(item) for item in value]
    return value


class CrawlLogStore(Protocol):
    def latest_checksum(self, source_code: str, dataset_code: str, params_hash: str) -> str | None: ...
    def record(self, entry: CrawlLogEntry) -> None: ...


@dataclass(frozen=True)
class BronzePersistResult:
    """Outcome summary returned after evaluating checksum and persisting one Bronze object."""

    uri: str
    payload_sha256: str
    params_hash: str
    status: str  # WRITTEN | SKIPPED_DUPLICATE
    byte_size: int


def build_bronze_object_key(capture: RawBronzeCapture, default_date: date) -> str:
    """Build Hive-partitioned object key `<source>/<dataset>/[tahun=YYYY/]dt=YYYY-MM-DD/<file>.json.gz`."""
    dt = (capture.capture_date or default_date).isoformat()
    src = capture.source_code.strip().lower()
    ds = capture.dataset_name.strip().lower()
    parts = [src, ds]
    if capture.partition_year is not None:
        parts.append(f"tahun={int(capture.partition_year)}")
    parts.append(f"dt={dt}")
    shard_parts = [
        f"{k}={capture.params[k]}"
        for k in ("pemda", "regency_id", "var_id", "indicator", "domain")
        if k in capture.params and str(capture.params[k]).strip() != ""
    ]
    filename = "raw.json.gz" if not shard_parts else f"raw_{'_'.join(shard_parts)}.json.gz"
    parts.append(filename)
    return "/".join(parts)


class BronzeStore:
    """Writes immutable `.json.gz` raw payloads to MinIO (`s3://data-raw/`) or isolated local `data/raw/`."""

    def __init__(
        self,
        *,
        local_dir: str | Path | None = None,
        minio_endpoint: str = "",
        minio_access_key: str = "",
        minio_secret_key: str = "",
        minio_bucket: str = "data-raw",
        minio_secure: bool = False,
        crawl_logs: CrawlLogStore | None = None,
        s3_client: Any = None,
    ) -> None:
        resolved_dir = Path(local_dir).resolve() if local_dir else None
        if resolved_dir is not None:
            parts = resolved_dir.parts
            for i in range(len(parts) - 1):
                if parts[i] == "apps" and parts[i + 1] == "crawler":
                    raise ValueError(
                        f"dilarang menyimpan data Bronze di dalam folder apps/crawler: {resolved_dir}"
                    )

        self.local_dir = resolved_dir
        self.minio_endpoint = minio_endpoint.strip()
        self.minio_bucket = minio_bucket.strip() or "data-raw"
        self.crawl_logs = crawl_logs
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

    @property
    def uses_s3(self) -> bool:
        return self._s3 is not None

    def _exists(self, key: str) -> bool:
        if self._s3 is not None:
            try:
                self._s3.head_object(Bucket=self.minio_bucket, Key=key)
                return True
            except Exception:
                return False
        if self.local_dir is None:
            return False
        return (self.local_dir / key).is_file()

    def _uri_for(self, key: str) -> str:
        if self._s3 is not None:
            return f"s3://{self.minio_bucket}/{key}"
        assert self.local_dir is not None
        return str(self.local_dir / key)

    def _ensure_bucket(self) -> None:
        if self._s3 is None:
            return
        try:
            self._s3.head_bucket(Bucket=self.minio_bucket)
        except Exception:
            self._s3.create_bucket(Bucket=self.minio_bucket)

    def persist_capture(self, capture: RawBronzeCapture, default_date: date) -> BronzePersistResult:
        """Persist unaltered payload to Bronze unless an identical SHA-256 was already stored."""
        raw_bytes = canonical_json_bytes(capture.payload)
        hash_target = (
            raw_bytes
            if isinstance(capture.payload, (bytes, bytearray))
            else _strip_volatile_hash_keys(capture.payload)
        )
        payload_hash = compute_payload_sha256(hash_target)
        params_hash = compute_params_hash(capture.params)
        key = build_bronze_object_key(capture, default_date)
        uri = self._uri_for(key)

        if self.crawl_logs is not None:
            previous_hash = self.crawl_logs.latest_checksum(
                capture.source_code,
                capture.dataset_name,
                params_hash,
            )
            if previous_hash == payload_hash and self._exists(key):
                self.crawl_logs.record(
                    CrawlLogEntry(
                        source_code=capture.source_code,
                        dataset_code=capture.dataset_name,
                        endpoint=capture.endpoint,
                        params_hash=params_hash,
                        params_json=capture.params,
                        payload_sha256=payload_hash,
                        bronze_uri=uri,
                        status="SKIPPED_DUPLICATE",
                        byte_size=len(raw_bytes),
                    )
                )
                return BronzePersistResult(
                    uri=uri,
                    payload_sha256=payload_hash,
                    params_hash=params_hash,
                    status="SKIPPED_DUPLICATE",
                    byte_size=len(raw_bytes),
                )

        compressed = gzip.compress(raw_bytes, mtime=0)
        if self._s3 is not None:
            self._ensure_bucket()
            self._s3.put_object(
                Bucket=self.minio_bucket,
                Key=key,
                Body=compressed,
                ContentType="application/gzip",
                Metadata={"sha256": payload_hash},
            )
        elif self.local_dir is not None:
            target = self.local_dir / key
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(compressed)
        else:
            raise RuntimeError("BronzeStore membutuhkan MinIO endpoint atau local_dir")

        if self.crawl_logs is not None:
            self.crawl_logs.record(
                CrawlLogEntry(
                    source_code=capture.source_code,
                    dataset_code=capture.dataset_name,
                    endpoint=capture.endpoint,
                    params_hash=params_hash,
                    params_json=capture.params,
                    payload_sha256=payload_hash,
                    bronze_uri=uri,
                    status="WRITTEN",
                    byte_size=len(compressed),
                )
            )

        return BronzePersistResult(
            uri=uri,
            payload_sha256=payload_hash,
            params_hash=params_hash,
            status="WRITTEN",
            byte_size=len(compressed),
        )


class BronzeOnlyRepository:
    """In-memory `RawStorageRepository` used with `--bronze-only` when running pure Medallion Bronze + PG."""

    def __init__(self) -> None:
        self._counts: dict[tuple[str, str, str, str], int] = {}
        self._latest: dict[tuple[str, str, str], date] = {}
        self.runs: list[dict[str, Any]] = []

    def ensure_indexes(self) -> None:
        return None

    def save_points(
        self,
        points: list[DataPoint],
        now_utc: datetime,
        crawl_date: str,
        run_id: str,
        *,
        is_backfill: bool = False,
    ) -> tuple[int, set[str]]:
        covered: set[str] = set()
        for p in points:
            covered.add(p.series.series_key)
            k = (p.series.collection, p.series.source_code, p.series.dataset_name, crawl_date)
            self._counts[k] = self._counts.get(k, 0) + 1
            lk = (p.series.collection, p.series.source_code, p.series.dataset_name)
            prev = self._latest.get(lk)
            if prev is None or p.period > prev:
                self._latest[lk] = p.period
        return len(points), covered

    def write_carried_forward(
        self,
        expected: list[TimeSeriesSpec],
        covered_keys: set[str],
        now_utc: datetime,
        crawl_date: str,
        run_id: str,
    ) -> int:
        return 0

    def count_today_points(self, collection: str, source_code: str, dataset_name: str, crawl_date: str) -> int:
        return self._counts.get((collection, source_code, dataset_name, crawl_date), 0)

    def latest_available_date(self, collection: str, source_code: str, dataset_name: str) -> date | None:
        return self._latest.get((collection, source_code, dataset_name))

    def record_run(self, **kwargs: Any) -> None:
        self.runs.append(kwargs)
