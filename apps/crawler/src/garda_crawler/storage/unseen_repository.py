"""MongoDB `dataunseen` repository for raw collections, idempotent upserts, and index setup."""

from datetime import date, datetime
from typing import Any

from pymongo import DeleteMany, ReplaceOne
from pymongo.database import Database

from garda_crawler.domain.entities import DataPoint, TimeSeriesSpec
from garda_crawler.domain.enums import RAW_COLLECTIONS, SourceStatus
from garda_crawler.storage.carry_forward import write_carry_forward_and_errors
from garda_crawler.storage.run_ledger import RunLedger
from garda_crawler.transport.time_utils import ensure_utc, to_midnight_utc


def serialize_point_document(
    point: DataPoint,
    now_utc: datetime,
    crawl_date: str,
    run_id: str,
    *,
    is_backfill: bool = False,
) -> dict[str, Any]:
    """Serialize a validated domain DataPoint into the shared MongoDB BSON document contract."""
    attrs = dict(point.attributes)
    if is_backfill:
        attrs["backfill"] = True
    period_midnight = to_midnight_utc(point.period)

    return {
        "_id": point.series.document_id(point.period, crawl_date),
        "sumber": point.series.source_code,
        "dataset": point.series.dataset_name,
        "kunci_seri": point.series.series_key,
        "indikator": point.series.indicator,
        "komoditas_kode": point.series.commodity_code,
        "kabupaten_kode": point.series.regency_code,
        "satuan": point.series.unit,
        "frekuensi": point.series.frequency,
        "periode": period_midnight,
        "nilai": float(point.value),
        "is_carried_forward": False,
        "last_available_date": period_midnight,
        "crawled_at": now_utc,
        "crawl_date": crawl_date,
        "source_status": SourceStatus.FRESH,
        "run_id": run_id,
        "url_sumber": point.source_url,
        "status_validasi": point.series.validation_status,
        "atribut": attrs,
    }


class UnseenRepository:
    """Repository managing all `raw_*` collections and execution metadata in `dataunseen`."""

    def __init__(self, db: Database) -> None:
        self.db = db
        self.ledger = RunLedger(db)

    def ensure_indexes(self) -> None:
        """Bootstrap contract indexes on every raw collection plus crawl_runs."""
        for name in RAW_COLLECTIONS:
            col = self.db[name]
            col.create_index([("kunci_seri", 1), ("periode", 1)])
            col.create_index([("sumber", 1), ("dataset", 1), ("crawl_date", 1)])
            col.create_index([("komoditas_kode", 1), ("kabupaten_kode", 1), ("periode", 1)])
            col.create_index([("source_status", 1), ("crawl_date", 1)])
        self.ledger.ensure_indexes()

    def save_points(
        self,
        points: list[DataPoint],
        now_utc: datetime,
        crawl_date: str,
        run_id: str,
        *,
        is_backfill: bool = False,
    ) -> tuple[int, set[str]]:
        """Idempotently persist validated points, skipping unchanged repeat vintages."""
        covered_keys: set[str] = set()
        new_count = 0
        ops_by_collection: dict[str, list[Any]] = {}
        latest_cache: dict[tuple[str, str, str], dict[tuple[str, Any], tuple[float, str]]] = {}
        seen_doc_ids: set[tuple[str, str]] = set()
        cleared_daily_series: set[tuple[str, str, str, str]] = set()

        for point in points:
            covered_keys.add(point.series.series_key)
            col_name = point.series.collection
            src = point.series.source_code
            ds = point.series.dataset_name
            cache_key = (col_name, src, ds)
            if cache_key not in latest_cache:
                existing_map: dict[tuple[str, Any], tuple[float, str]] = {}
                cursor = self.db[col_name].find(
                    {
                        "sumber": src,
                        "dataset": ds,
                        "is_carried_forward": False,
                        "nilai": {"$ne": None},
                    },
                    {"kunci_seri": 1, "periode": 1, "nilai": 1, "crawl_date": 1, "crawled_at": 1},
                    sort=[("crawled_at", -1)],
                )
                for row in cursor:
                    k = (row.get("kunci_seri", ""), row.get("periode"))
                    if k not in existing_map:
                        existing_map[k] = (float(row["nilai"]), str(row.get("crawl_date", "")))
                latest_cache[cache_key] = existing_map

            collection = self.db[col_name]
            period_midnight = to_midnight_utc(point.period)

            cached = latest_cache[cache_key].get((point.series.series_key, period_midnight))
            doc = serialize_point_document(point, now_utc, crawl_date, run_id, is_backfill=is_backfill)
            if (col_name, doc["_id"]) in seen_doc_ids:
                continue
            seen_doc_ids.add((col_name, doc["_id"]))

            ops = ops_by_collection.setdefault(col_name, [])

            if cached is not None and cached[0] == float(point.value):
                # Same day same value is idempotent; still keep `new_count` true only when inserted today.
                if cached[1] == crawl_date:
                    new_count += 1
                continue

            # Clear earlier-same-day carried-forward or morning ERROR placeholders once per series.
            series_marker = (col_name, src, ds, point.series.series_key)
            if not is_backfill and series_marker not in cleared_daily_series:
                cleared_daily_series.add(series_marker)
                ops.append(
                    DeleteMany(
                        {
                            "sumber": src,
                            "dataset": ds,
                            "kunci_seri": point.series.series_key,
                            "crawl_date": crawl_date,
                            "$or": [
                                {"is_carried_forward": True},
                                {"source_status": SourceStatus.ERROR},
                            ],
                        }
                    )
                )

            ops.append(ReplaceOne({"_id": doc["_id"]}, doc, upsert=True))
            new_count += 1

        for collection_name, ops in ops_by_collection.items():
            for i in range(0, len(ops), 2000):
                self.db[collection_name].bulk_write(ops[i : i + 2000], ordered=True)

        return new_count, covered_keys

    def write_carried_forward(
        self,
        expected: list[TimeSeriesSpec],
        covered_keys: set[str],
        now_utc: datetime,
        crawl_date: str,
        run_id: str,
    ) -> int:
        """Persist carried-forward or ERROR placeholder documents for uncovered daily series."""
        return write_carry_forward_and_errors(
            self.db,
            expected,
            covered_keys,
            now_utc,
            crawl_date,
            run_id,
        )

    def count_today_points(
        self,
        collection: str,
        source_code: str,
        dataset_name: str,
        crawl_date: str,
    ) -> int:
        """Count non-backfill valid points recorded today for freshness checks."""
        return int(
            self.db[collection].count_documents(
                {
                    "sumber": source_code,
                    "dataset": dataset_name,
                    "crawl_date": crawl_date,
                    "is_carried_forward": False,
                    "atribut.backfill": {"$ne": True},
                    "nilai": {"$ne": None},
                }
            )
        )

    def latest_available_date(self, collection: str, source_code: str, dataset_name: str) -> date | None:
        """Find the newest valid observation period across non-carried-forward documents."""
        doc = self.db[collection].find_one(
            {
                "sumber": source_code,
                "dataset": dataset_name,
                "is_carried_forward": False,
                "nilai": {"$ne": None},
            },
            sort=[("periode", -1)],
        )
        if doc and doc.get("periode"):
            return ensure_utc(doc["periode"]).date()
        return None

    def record_run(
        self,
        *,
        run_id: str,
        source_code: str,
        dataset_name: str,
        crawl_date: str,
        started_at: datetime,
        finished_at: datetime,
        status: str,
        new_count: int,
        carried_forward_count: int,
        last_available_date: date | None,
        message: str,
        trigger: str,
    ) -> None:
        """Delegate execution logging to RunLedger."""
        self.ledger.record_run(
            run_id=run_id,
            source_code=source_code,
            dataset_name=dataset_name,
            crawl_date=crawl_date,
            started_at=started_at,
            finished_at=finished_at,
            status=status,
            new_count=new_count,
            carried_forward_count=carried_forward_count,
            last_available_date=last_available_date,
            message=message,
            trigger=trigger,
        )
