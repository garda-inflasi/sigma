"""Carry-forward and ERROR marker persistence preserving original `last_available_date`."""

from datetime import datetime
from typing import Any

from pymongo import DeleteOne, ReplaceOne
from pymongo.database import Database

from garda_crawler.domain.entities import TimeSeriesSpec
from garda_crawler.domain.enums import SourceStatus
from garda_crawler.transport.time_utils import ensure_utc, to_midnight_utc


def _has_fresh_point_today(collection: Any, series: TimeSeriesSpec, crawl_date: str) -> bool:
    return (
        collection.count_documents(
            {
                "sumber": series.source_code,
                "dataset": series.dataset_name,
                "kunci_seri": series.series_key,
                "crawl_date": crawl_date,
                "is_carried_forward": False,
                "atribut.backfill": {"$ne": True},
                "nilai": {"$ne": None},
            },
            limit=1,
        )
        > 0
    )


def _find_latest_valid_observation(collection: Any, series: TimeSeriesSpec) -> dict[str, Any] | None:
    return collection.find_one(
        {
            "sumber": series.source_code,
            "dataset": series.dataset_name,
            "kunci_seri": series.series_key,
            "is_carried_forward": False,
            "nilai": {"$ne": None},
        },
        sort=[("periode", -1), ("crawled_at", -1)],
    )


def write_carry_forward_and_errors(
    db: Database,
    expected_series: list[TimeSeriesSpec],
    covered_keys: set[str],
    now_utc: datetime,
    crawl_date: str,
    run_id: str,
) -> int:
    """Write STALE_CARRIED_FORWARD (or ERROR if no history) docs for missing daily series."""
    from datetime import date

    crawl_day = date.fromisoformat(crawl_date)
    today_midnight = to_midnight_utc(crawl_day)
    carried_count = 0
    ops_by_collection: dict[str, list[Any]] = {}

    for series in expected_series:
        if series.series_key in covered_keys:
            continue

        collection = db[series.collection]
        if _has_fresh_point_today(collection, series, crawl_date):
            continue

        latest = _find_latest_valid_observation(collection, series)
        doc_id = series.document_id(crawl_day, crawl_date)
        ops = ops_by_collection.setdefault(series.collection, [])

        if latest is not None:
            carried_count += 1
            last_date = ensure_utc(latest["periode"])
            doc = {
                "_id": doc_id,
                "sumber": series.source_code,
                "dataset": series.dataset_name,
                "kunci_seri": series.series_key,
                "indikator": series.indicator,
                "komoditas_kode": series.commodity_code,
                "kabupaten_kode": series.regency_code,
                "satuan": series.unit,
                "frekuensi": series.frequency,
                "periode": today_midnight,
                "nilai": latest["nilai"],
                "is_carried_forward": True,
                "last_available_date": last_date,
                "crawled_at": now_utc,
                "crawl_date": crawl_date,
                "source_status": SourceStatus.STALE_CARRIED_FORWARD,
                "run_id": run_id,
                "url_sumber": series.source_url,
                "status_validasi": latest.get("status_validasi", series.validation_status),
                "atribut": {"id_asal": latest["_id"]},
            }
        else:
            doc = {
                "_id": doc_id,
                "sumber": series.source_code,
                "dataset": series.dataset_name,
                "kunci_seri": series.series_key,
                "indikator": series.indicator,
                "komoditas_kode": series.commodity_code,
                "kabupaten_kode": series.regency_code,
                "satuan": series.unit,
                "frekuensi": series.frequency,
                "periode": today_midnight,
                "nilai": None,
                "is_carried_forward": False,
                "last_available_date": None,
                "crawled_at": now_utc,
                "crawl_date": crawl_date,
                "source_status": SourceStatus.ERROR,
                "run_id": run_id,
                "url_sumber": series.source_url,
                "status_validasi": series.validation_status,
                "atribut": {},
            }

        # Clear opposite stale/error placeholder for the same series & crawl_date without touching historical points.
        ops.append(
            DeleteOne(
                {
                    "sumber": series.source_code,
                    "dataset": series.dataset_name,
                    "kunci_seri": series.series_key,
                    "crawl_date": crawl_date,
                    "_id": {"$ne": doc_id},
                    "$or": [
                        {"is_carried_forward": True},
                        {"source_status": SourceStatus.ERROR},
                    ],
                }
            )
        )
        ops.append(ReplaceOne({"_id": doc_id}, doc, upsert=True))

    for collection_name, ops in ops_by_collection.items():
        if ops:
            db[collection_name].bulk_write(ops, ordered=True)

    return carried_count
