"""Incremental PostgreSQL -> MongoDB raw_pangan_harian sync for field price entries."""

import logging
from collections import Counter
from collections.abc import Iterable, Sequence
from datetime import date, datetime
from typing import Any

import psycopg
from pymongo import DeleteOne, ReplaceOne

from garda_crawler import __version__
from garda_crawler.config.source_specs import FIELD_ENTRIES_DATASET
from garda_crawler.domain.entities import FieldSyncOutcome, TimeSeriesSpec
from garda_crawler.domain.enums import VALIDATION_STATUS_WEIGHT, Frequency, SourceStatus
from garda_crawler.domain.protocols import FieldEntriesReader
from garda_crawler.adapters.base import format_skip_message
from garda_crawler.storage.unseen_repository import UnseenRepository
from garda_crawler.transport.time_utils import WIB_TZ, ensure_utc, to_midnight_utc, to_wib_date
from garda_crawler.validation.price_rules import validate_price

logger = logging.getLogger(__name__)

SOURCE_CODE = "LAPANGAN"
DATASET_NAME = FIELD_ENTRIES_DATASET.name
COLLECTION_NAME = FIELD_ENTRIES_DATASET.collection
SYNC_BATCH_SIZE = 1000


def _chunk_items(items: Sequence[Any], size: int) -> Iterable[Sequence[Any]]:
    for idx in range(0, len(items), size):
        yield items[idx : idx + size]


def _entry_period(recorded_at: datetime) -> date:
    return to_wib_date(ensure_utc(recorded_at))


def build_field_entry_document(
    row: dict[str, Any],
    crawl_date: str,
    crawled_at: datetime,
    run_id: str,
) -> dict[str, Any]:
    """Serialize one sanitized Postgres field price row into the raw_pangan_harian BSON contract."""
    period = _entry_period(row["dicatat_pada"])
    unit_suffix = str(row["satuan_master"]).strip().lower()
    series_group = f"{row['komoditas_kode']}:{row['kabupaten_kode']}:{row['tipe']}"
    series_key = f"{series_group}:E{row['id']}"
    midnight_period = to_midnight_utc(period)
    doc_id = TimeSeriesSpec(
        source_code=SOURCE_CODE,
        dataset_name=DATASET_NAME,
        collection=COLLECTION_NAME,
        series_key=series_key,
        indicator="",
        unit="",
        frequency=Frequency.DAILY,
    ).document_id(period, crawl_date)

    return {
        "_id": doc_id,
        "sumber": SOURCE_CODE,
        "dataset": DATASET_NAME,
        "kunci_seri": series_key,
        "indikator": f"harga_{str(row['tipe']).lower()}_{row['komoditas_kode']}",
        "komoditas_kode": row["komoditas_kode"],
        "kabupaten_kode": row["kabupaten_kode"],
        "satuan": f"Rp/{unit_suffix}",
        "frekuensi": Frequency.DAILY,
        "periode": midnight_period,
        "nilai": float(row["harga"]),
        "is_carried_forward": False,
        "last_available_date": midnight_period,
        "crawled_at": crawled_at,
        "crawl_date": crawl_date,
        "source_status": SourceStatus.FRESH,
        "run_id": run_id,
        "url_sumber": "postgres://entri_harga",
        "status_validasi": row["status"],
        "atribut": {
            "entri_id": int(row["id"]),
            "versi_entri": int(row["versi"]),
            "deret": series_group,
            "pasar_id": row["pasar_id"],
            "tipe": row["tipe"],
            "kualitas": row["kualitas"],
            "deviasi_pct": row["deviasi_pct"],
            "skor_anomali": row["skor_anomali"],
        },
    }


def _apply_incremental_diff(
    repository: UnseenRepository,
    reader: FieldEntriesReader,
    pg_index: list[dict[str, Any]],
    crawl_date: str,
    crawled_at: datetime,
    run_id: str,
) -> tuple[int, int, Counter[str]]:
    collection = repository.db[COLLECTION_NAME]
    existing: dict[int, dict[str, Any]] = {}
    for doc in collection.find(
        {"sumber": SOURCE_CODE, "dataset": DATASET_NAME, "is_carried_forward": False},
        {"crawl_date": 1, "periode": 1, "atribut.entri_id": 1, "atribut.versi_entri": 1},
    ):
        existing[doc["atribut"]["entri_id"]] = doc

    valid_index = {int(row["id"]): row for row in pg_index}
    changed_ids = [
        entry_id
        for entry_id, row in valid_index.items()
        if entry_id not in existing
        or existing[entry_id]["atribut"].get("versi_entri") != int(row["versi"])
        or ensure_utc(existing[entry_id]["periode"]) != to_midnight_utc(_entry_period(row["dicatat_pada"]))
    ]

    rejected: Counter[str] = Counter()
    operations: list[Any] = []
    removed_ids: set[int] = set(existing) - set(valid_index)

    for id_batch in _chunk_items(sorted(changed_ids), SYNC_BATCH_SIZE):
        for row in reader.fetch_rows_by_ids(id_batch):
            entry_id = int(row["id"])
            ref_price = float(row["harga_acuan"]) if row["harga_acuan"] is not None else 0.0
            reason = validate_price(float(row["harga"]), ref_price, row["satuan"], row["satuan_master"])
            if reason:
                rejected[reason] += 1
                removed_ids.add(entry_id)
                continue

            previous = existing.get(entry_id)
            doc = build_field_entry_document(
                row,
                previous["crawl_date"] if previous else crawl_date,
                crawled_at,
                run_id,
            )
            if previous and previous["_id"] != doc["_id"]:
                operations.append(DeleteOne({"_id": previous["_id"]}))
            operations.append(ReplaceOne({"_id": doc["_id"]}, doc, upsert=True))

    deletes = [entry_id for entry_id in removed_ids if entry_id in existing]
    operations.extend(DeleteOne({"_id": existing[entry_id]["_id"]}) for entry_id in deletes)

    for op_batch in _chunk_items(operations, SYNC_BATCH_SIZE):
        collection.bulk_write(list(op_batch), ordered=True)

    if removed_ids:
        for rm_batch in _chunk_items(sorted(removed_ids), SYNC_BATCH_SIZE):
            collection.delete_many(
                {
                    "sumber": SOURCE_CODE,
                    "dataset": DATASET_NAME,
                    "is_carried_forward": True,
                    "atribut.entri_id_asal": {"$in": list(rm_batch)},
                }
            )

    written_count = sum(isinstance(op, ReplaceOne) for op in operations)
    return written_count, len(deletes), rejected


def write_field_carried_forward(
    repository: UnseenRepository,
    day: date,
    crawled_at: datetime,
    run_id: str,
) -> int:
    """Carry forward latest validated field price per series group when today has no direct entry."""
    collection = repository.db[COLLECTION_NAME]
    today_midnight = to_midnight_utc(day)
    crawl_date_str = day.isoformat()

    latest_by_group: dict[str, dict[str, Any]] = {}
    for doc in collection.find({"sumber": SOURCE_CODE, "dataset": DATASET_NAME, "is_carried_forward": False}):
        group_key = doc["atribut"]["deret"]
        rank = (
            ensure_utc(doc["periode"]),
            VALIDATION_STATUS_WEIGHT.get(doc["status_validasi"], 0),
            doc["atribut"]["entri_id"],
        )
        previous = latest_by_group.get(group_key)
        if previous is None or rank > previous["_rank"]:
            latest_by_group[group_key] = {**doc, "_rank": rank}

    operations: list[ReplaceOne] = []
    active_ids: list[str] = []
    for group_key, doc in latest_by_group.items():
        if ensure_utc(doc["periode"]) >= today_midnight:
            continue
        cf_id = f"{SOURCE_CODE}:{DATASET_NAME}:{group_key}:{crawl_date_str}:{crawl_date_str}"
        active_ids.append(cf_id)
        cf_doc = {
            "_id": cf_id,
            "sumber": SOURCE_CODE,
            "dataset": DATASET_NAME,
            "kunci_seri": group_key,
            "indikator": doc["indikator"],
            "komoditas_kode": doc["komoditas_kode"],
            "kabupaten_kode": doc["kabupaten_kode"],
            "satuan": doc["satuan"],
            "frekuensi": doc["frekuensi"],
            "periode": today_midnight,
            "nilai": doc["nilai"],
            "is_carried_forward": True,
            "last_available_date": ensure_utc(doc["periode"]),
            "crawled_at": crawled_at,
            "crawl_date": crawl_date_str,
            "source_status": SourceStatus.STALE_CARRIED_FORWARD,
            "run_id": run_id,
            "url_sumber": doc["url_sumber"],
            "status_validasi": doc["status_validasi"],
            "atribut": {"deret": group_key, "entri_id_asal": doc["atribut"]["entri_id"]},
        }
        operations.append(ReplaceOne({"_id": cf_id}, cf_doc, upsert=True))

    for op_batch in _chunk_items(operations, SYNC_BATCH_SIZE):
        collection.bulk_write(list(op_batch), ordered=False)

    collection.delete_many(
        {
            "sumber": SOURCE_CODE,
            "dataset": DATASET_NAME,
            "is_carried_forward": True,
            "crawl_date": crawl_date_str,
            "_id": {"$nin": active_ids},
        }
    )
    return len(operations)


def sync_field_entries(
    reader: FieldEntriesReader | None,
    repository: UnseenRepository,
    now_utc: datetime,
    trigger: str,
) -> FieldSyncOutcome:
    """Execute one incremental Postgres-to-MongoDB sync and record crawl_runs metadata."""
    today = to_wib_date(now_utc)
    crawl_date = today.isoformat()
    hour = ensure_utc(now_utc).astimezone(WIB_TZ).strftime("%H")
    run_id = f"{crawl_date}T{hour}:{SOURCE_CODE}:{DATASET_NAME}:{trigger}"

    written = 0
    deleted = 0
    rejected_total = 0
    notes: list[str] = []
    failed = False

    if reader is None:
        failed = True
        notes.append(
            format_skip_message(
                "koneksi PostgreSQL belum dikonfigurasi",
                "isi CRAWLER_DATABASE_URL atau DB_*",
            )
        )
    else:
        try:
            with reader as conn:
                index_rows = conn.fetch_index()
                demo_count = conn.count_demo_entries()
                written, deleted, rejected = _apply_incremental_diff(
                    repository, conn, index_rows, crawl_date, now_utc, run_id
                )
            rejected_total = sum(rejected.values())
            notes.append(f"entri sah {len(index_rows)}, ditulis {written}, dikeluarkan {deleted}")
            if demo_count:
                notes.append(f"{demo_count} entri seed demo diabaikan")
            if rejected:
                notes.append("ditolak " + ", ".join(f"{k}={v}" for k, v in sorted(rejected.items())))
        except psycopg.Error as exc:
            failed = True
            notes.append(f"gagal membaca PostgreSQL: {type(exc).__name__}")
            logger.warning("field sync failed: %s", type(exc).__name__)

    carried_forward = write_field_carried_forward(repository, today, now_utc, run_id)
    fresh_today = repository.db[COLLECTION_NAME].count_documents(
        {
            "sumber": SOURCE_CODE,
            "dataset": DATASET_NAME,
            "is_carried_forward": False,
            "periode": to_midnight_utc(today),
        }
    )

    if failed:
        status = SourceStatus.ERROR
    elif written > 0 or fresh_today > 0:
        status = SourceStatus.FRESH
    else:
        status = SourceStatus.STALE_CARRIED_FORWARD

    message = "; ".join(notes)
    finished_at = datetime.now(now_utc.tzinfo)
    repository.record_run(
        run_id=run_id,
        source_code=SOURCE_CODE,
        dataset_name=DATASET_NAME,
        crawl_date=crawl_date,
        started_at=now_utc,
        finished_at=finished_at,
        status=status,
        new_count=written,
        carried_forward_count=carried_forward,
        last_available_date=repository.latest_available_date(COLLECTION_NAME, SOURCE_CODE, DATASET_NAME),
        message=message,
        trigger=trigger,
    )

    return FieldSyncOutcome(
        run_id=run_id,
        status=status,
        written_count=written,
        deleted_count=deleted,
        rejected_count=rejected_total,
        carried_forward_count=carried_forward,
        message=message,
    )
