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

        for point in points:
            covered_keys.add(point.series.series_key)
            collection = self.db[point.series.collection]
            period_midnight = to_midnight_utc(point.period)

            # Avoid adding duplicate vintage rows if the exact (series, period, value) is unchanged.
            latest_same_period = collection.find_one(
                {
                    "sumber": point.series.source_code,
                    "dataset": point.series.dataset_name,
                    "kunci_seri": point.series.series_key,
                    "periode": period_midnight,
                    "is_carried_forward": False,
                    "nilai": {"$ne": None},
                },
                sort=[("crawled_at", -1)],
            )

            doc = serialize_point_document(point, now_utc, crawl_date, run_id, is_backfill=is_backfill)
            ops = ops_by_collection.setdefault(point.series.collection, [])

            if latest_same_period is not None and float(latest_same_period["nilai"]) == float(point.value):
                # Same day same value is idempotent; still keep `new_count` true only when inserted today.
                if latest_same_period.get("crawl_date") == crawl_date:
                    new_count += 1
                continue

            # Clear earlier-same-day carried-forward or morning ERROR placeholders for this series.
            if not is_backfill:
                ops.append(
                    DeleteMany(
                        {
                            "sumber": point.series.source_code,
                            "dataset": point.series.dataset_name,
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
            if ops:
                self.db[collection_name].bulk_write(ops, ordered=True)

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
