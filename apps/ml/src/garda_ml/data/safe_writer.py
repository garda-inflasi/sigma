"""Append-only MongoDB datasafe writer, BSON sanitizer, and index setup."""

import math
from datetime import UTC, date, datetime
from typing import Any

import numpy as np
import pandas as pd
from pymongo import ASCENDING, DESCENDING
from pymongo.database import Database

from garda_ml.domain.enums import CycleStatus

CYCLE_COLLECTION = "ml_training_cycles"
INFLATION_COLLECTION = "datasafe_inflasi_signals"
FORECAST_COLLECTION = "datasafe_forecasts"
XAI_COLLECTION = "datasafe_xai_explanations"
EWS_COLLECTION = "datasafe_ews_signals"
MACRO_COLLECTION = "datasafe_macro_indicators"
REGION_CLUSTERS_COLLECTION = "datasafe_region_clusters"

OUTPUT_COLLECTIONS: tuple[str, ...] = (
    MACRO_COLLECTION,
    FORECAST_COLLECTION,
    XAI_COLLECTION,
    INFLATION_COLLECTION,
    EWS_COLLECTION,
)

FINAL_READABLE_STATUSES: tuple[str, ...] = (
    CycleStatus.SUCCEEDED,
    CycleStatus.PARTIAL,
)

_BUSINESS_INDEX_KEYS: dict[str, list[tuple[str, int]]] = {
    INFLATION_COLLECTION: [("wilayah_kode", ASCENDING)],
    FORECAST_COLLECTION: [("komoditas_kode", ASCENDING), ("kabupaten_kode", ASCENDING)],
    XAI_COLLECTION: [("target", ASCENDING), ("komoditas_kode", ASCENDING), ("wilayah_kode", ASCENDING)],
    EWS_COLLECTION: [("sinyal_kunci", ASCENDING)],
    MACRO_COLLECTION: [("domain", ASCENDING), ("indikator", ASCENDING), ("wilayah_kode", ASCENDING)],
    REGION_CLUSTERS_COLLECTION: [("regency_code", ASCENDING)],
}


def ensure_datasafe_indexes(db: Database) -> None:
    """Create read/query indexes across all datasafe collections and training cycles."""
    for name, business_key in _BUSINESS_INDEX_KEYS.items():
        col = db[name]
        col.create_index([("cycle_id", ASCENDING)])
        col.create_index([("cycle_timestamp", DESCENDING)])
        col.create_index([*business_key, ("cycle_timestamp", DESCENDING)])

    db[EWS_COLLECTION].create_index([("severity", ASCENDING), ("cycle_timestamp", DESCENDING)])

    cycles = db[CYCLE_COLLECTION]
    cycles.create_index([("cycle_id", ASCENDING)])
    cycles.create_index([("cycle_timestamp", DESCENDING)])
    cycles.create_index([("status", ASCENDING), ("cycle_timestamp", DESCENDING)])


def sanitize_bson(value: Any) -> Any:
    """Convert numpy/pandas scalars and NaN/Inf values into MongoDB-safe Python BSON types."""
    if isinstance(value, dict):
        return {k: sanitize_bson(v) for k, v in value.items()}
    if isinstance(value, (list, tuple)):
        return [sanitize_bson(v) for v in value]
    if isinstance(value, (np.bool_, bool)):
        return bool(value)
    if isinstance(value, (np.integer,)):
        return int(value)
    if isinstance(value, (float, np.floating)):
        numeric = float(value)
        return None if math.isnan(numeric) or math.isinf(numeric) else numeric
    if value is pd.NaT:
        return None
    if isinstance(value, pd.Timestamp):
        dt = value.to_pydatetime()
        return dt.replace(tzinfo=UTC) if dt.tzinfo is None else dt
    if isinstance(value, datetime):
        return value
    if isinstance(value, date):
        return datetime(value.year, value.month, value.day, tzinfo=UTC)
    return value


def write_documents(
    db: Database,
    collection_name: str,
    documents: list[dict[str, Any]],
    batch_size: int = 1000,
) -> int:
    """Insert append-only documents in deterministic batches."""
    if not documents:
        return 0
    for start in range(0, len(documents), batch_size):
        chunk = [sanitize_bson(doc) for doc in documents[start : start + batch_size]]
        db[collection_name].insert_many(chunk, ordered=True)
    return len(documents)


def cleanup_partial_cycle(db: Database, cycle_id: str) -> None:
    """Delete leftover uncommitted documents from interrupted non-final cycles."""
    status = (db[CYCLE_COLLECTION].find_one({"_id": cycle_id}, {"status": 1}) or {}).get("status")
    if status in FINAL_READABLE_STATUSES:
        raise RuntimeError(f"menolak menghapus siklus final {cycle_id}")
    for col_name in (*OUTPUT_COLLECTIONS, REGION_CLUSTERS_COLLECTION):
        db[col_name].delete_many({"cycle_id": cycle_id})


def delete_cycle(db: Database, cycle_id: str) -> None:
    """Delete one cycle and all associated documents (explicit `--force` retrain only)."""
    for col_name in (*OUTPUT_COLLECTIONS, REGION_CLUSTERS_COLLECTION):
        db[col_name].delete_many({"cycle_id": cycle_id})
    db[CYCLE_COLLECTION].delete_one({"_id": cycle_id})
