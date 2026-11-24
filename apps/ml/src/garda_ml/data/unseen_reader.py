"""Point-in-time reader extracting dataunseen collections into tabular snapshots."""

import re
from datetime import datetime, timedelta
from typing import Any

import numpy as np
import pandas as pd
from pymongo.database import Database

from garda_ml.config.constants import (
    FIELD_CONSUMER_TYPE,
    FIELD_STATUS_WEIGHTS,
    OFFICIAL_PRICE_SOURCES,
    RELEASE_LAG_DAYS,
)
from garda_ml.data.time_utils import ensure_utc, to_midnight_utc, to_wib_date
from garda_ml.domain.entities import RawMarketDataset

PRICE_COLLECTION = "raw_pangan_harian"
CLIMATE_COLLECTION = "raw_iklim_bmkg"
MACRO_COLLECTIONS: tuple[str, ...] = (
    "raw_bps_makro",
    "raw_konsumsi_pendapatan",
    "raw_neraca_ekonomi",
    "raw_tenaga_kerja",
    "raw_sektor_primer",
)
RUNS_COLLECTION = "crawl_runs"

_PRICE_COLUMNS = [
    "komoditas_kode",
    "kabupaten_kode",
    "tanggal",
    "nilai",
    "bobot",
    "nilai_lapangan",
    "bobot_lapangan",
    "sumber",
    "satuan",
]

_MACRO_COLUMNS = [
    "domain",
    "indikator",
    "wilayah_kode",
    "periode",
    "tanggal_rilis",
    "nilai",
    "satuan",
    "sumber",
    "frekuensi",
    "is_carried_forward",
    "last_available_date",
]


def _to_naive_utc(value: datetime | None) -> pd.Timestamp | None:
    if value is None:
        return None
    return pd.Timestamp(ensure_utc(value).replace(tzinfo=None))


def compute_conservative_release_date(period: pd.Series, frequency: pd.Series) -> pd.Series:
    """Estimate conservative public release dates from period start and frequency."""
    period_end = period.copy()
    for freq_label, offset in (
        ("BULANAN", pd.offsets.MonthEnd(0)),
        ("TRIWULANAN", pd.offsets.QuarterEnd(0)),
        ("TAHUNAN", pd.offsets.YearEnd(0)),
    ):
        mask = frequency == freq_label
        if mask.any():
            period_end.loc[mask] = period.loc[mask] + offset
    lag_days = frequency.map(RELEASE_LAG_DAYS).fillna(RELEASE_LAG_DAYS["BULANAN"]).astype(int)
    return period_end + pd.to_timedelta(lag_days, unit="D")


def merge_price_observations(rows: pd.DataFrame) -> pd.DataFrame:
    """Resolve official vs field prices per commodity x regency x date without level mixing."""
    if rows.empty:
        return pd.DataFrame(columns=_PRICE_COLUMNS)

    df = rows[rows["nilai"].notna() & (rows["nilai"] > 0)].copy()
    is_field = df["sumber"] == "LAPANGAN"
    df["bobot_status"] = 1.0
    df.loc[is_field, "bobot_status"] = df.loc[is_field, "status_validasi"].map(FIELD_STATUS_WEIGHTS)
    df = df[df["bobot_status"].notna()]

    if "tipe" in df:
        wrong_type = (df["sumber"] == "LAPANGAN") & df["tipe"].notna() & (df["tipe"] != FIELD_CONSUMER_TYPE)
        df = df[~wrong_type]

    # Repeat crawls for identical (source, dataset, series_key, date) keep newest crawled_at.
    df = df.sort_values("crawled_at").drop_duplicates(
        ["sumber", "dataset", "kunci_seri", "tanggal"],
        keep="last",
    )

    series_cols = ["komoditas_kode", "kabupaten_kode"]
    field_rows = df[df["sumber"] == "LAPANGAN"]
    if field_rows.empty:
        daily_field = pd.DataFrame(columns=[*series_cols, "tanggal", "nilai_lapangan", "bobot_lapangan", "bobot_maks"])
    else:
        weighted = field_rows.assign(_weighted=field_rows["nilai"] * field_rows["bobot_status"])
        grouped = weighted.groupby([*series_cols, "tanggal"], sort=False)
        daily_field = grouped.agg(
            _weighted=("_weighted", "sum"),
            bobot_lapangan=("bobot_status", "sum"),
            bobot_maks=("bobot_status", "max"),
        ).reset_index()
        daily_field["nilai_lapangan"] = daily_field["_weighted"] / daily_field["bobot_lapangan"]
        daily_field["bobot_lapangan"] = daily_field["bobot_maks"]
        daily_field = daily_field.drop(columns="_weighted")

    official_rows = df[df["sumber"] != "LAPANGAN"]
    parts: list[pd.DataFrame] = []
    if not official_rows.empty:
        counts = official_rows.groupby([*series_cols, "sumber"], sort=False).size().reset_index(name="n")
        priority = {src: idx for idx, src in enumerate(OFFICIAL_PRICE_SOURCES)}
        counts["priority"] = counts["sumber"].map(priority).fillna(len(priority))
        primary_sources = (
            counts.sort_values(["priority", "n"], ascending=[True, False])
            .drop_duplicates(series_cols)[[*series_cols, "sumber"]]
        )
        selected = official_rows.merge(primary_sources, on=[*series_cols, "sumber"])
        selected = selected.groupby([*series_cols, "tanggal"], as_index=False).agg(
            nilai=("nilai", "mean"),
            sumber=("sumber", "first"),
            satuan=("satuan", "last"),
        )
        selected["bobot"] = 1.0
        parts.append(selected)
        has_official = primary_sources[series_cols].assign(_has_official=True)
    else:
        has_official = pd.DataFrame(columns=[*series_cols, "_has_official"])

    if not daily_field.empty:
        field_only = daily_field.merge(has_official, on=series_cols, how="left")
        field_only = field_only[field_only["_has_official"].isna()]
        if not field_only.empty:
            units = field_rows.groupby(series_cols, sort=False)["satuan"].last().reset_index()
            joined = field_only.merge(units, on=series_cols, how="left")
            parts.append(
                pd.DataFrame(
                    {
                        "komoditas_kode": joined["komoditas_kode"],
                        "kabupaten_kode": joined["kabupaten_kode"],
                        "tanggal": joined["tanggal"],
                        "nilai": joined["nilai_lapangan"],
                        "sumber": "LAPANGAN",
                        "satuan": joined["satuan"],
                        "bobot": joined["bobot_maks"],
                    }
                )
            )

    if not parts:
        return pd.DataFrame(columns=_PRICE_COLUMNS)

    merged = pd.concat(parts, ignore_index=True)
    if daily_field.empty:
        merged["nilai_lapangan"] = np.nan
        merged["bobot_lapangan"] = np.nan
    else:
        merged = merged.merge(
            daily_field[[*series_cols, "tanggal", "nilai_lapangan", "bobot_lapangan"]],
            on=[*series_cols, "tanggal"],
            how="left",
        )
    return merged[_PRICE_COLUMNS].sort_values(["komoditas_kode", "kabupaten_kode", "tanggal"], ignore_index=True)


def map_macro_domain(collection: str, indicator: str, dataset: str = "") -> str:
    """Map raw macro collection & indicator descriptors into contract domain buckets."""
    fixed = {
        "raw_konsumsi_pendapatan": "KONSUMSI_PENDAPATAN",
        "raw_neraca_ekonomi": "NERACA_EKONOMI",
        "raw_tenaga_kerja": "TENAGA_KERJA",
    }
    if collection in fixed:
        return fixed[collection]

    text = f"{indicator} {dataset}".lower()
    if collection == "raw_sektor_primer":
        if re.search(r"hutan|kayu", text):
            return "KEHUTANAN"
        if re.search(r"ikan|perikanan|tangkap|budidaya", text):
            return "PERIKANAN"
        return "PERTANIAN"
    if re.search(r"inflasi|ihk|deflasi", text):
        return "INFLASI"
    if re.search(r"upah|ump|umk|tenaga kerja|pengangguran|tpt|tpak", text):
        return "TENAGA_KERJA"
    if re.search(r"konsumsi|pendapatan|pengeluaran|kemiskinan", text):
        return "KONSUMSI_PENDAPATAN"
    if re.search(r"padi|beras|produksi|panen|luas", text):
        return "PERTANIAN"
    return "NERACA_EKONOMI"


def _to_frame(docs: list[dict[str, Any]]) -> pd.DataFrame:
    if not docs:
        return pd.DataFrame()
    df = pd.DataFrame(docs)
    for col in ("periode", "crawled_at", "last_available_date"):
        if col in df:
            df[col] = pd.to_datetime(df[col], utc=True, errors="coerce").dt.tz_localize(None)
    return df


def _read_prices(
    db: Database,
    cutoff: datetime,
    end_midnight: datetime,
    start_midnight: datetime,
) -> tuple[pd.DataFrame, dict[str, str], dict[tuple[str, str, str], pd.Timestamp]]:
    query = {
        "is_carried_forward": False,
        "nilai": {"$ne": None},
        "source_status": {"$ne": "ERROR"},
        "periode": {"$gte": start_midnight, "$lte": end_midnight},
        "crawled_at": {"$lte": cutoff},
        "komoditas_kode": {"$ne": None},
        "kabupaten_kode": {"$ne": None},
    }
    projection = {
        "_id": 0,
        "sumber": 1,
        "dataset": 1,
        "kunci_seri": 1,
        "komoditas_kode": 1,
        "kabupaten_kode": 1,
        "periode": 1,
        "nilai": 1,
        "satuan": 1,
        "status_validasi": 1,
        "crawled_at": 1,
        "indikator": 1,
        "atribut.komoditas_nama": 1,
        "atribut.nama_komoditas": 1,
        "atribut.nama_sumber": 1,
        "atribut.tipe": 1,
    }
    docs = list(db[PRICE_COLLECTION].find(query, projection).batch_size(5000))
    commodity_names: dict[str, str] = {}
    for doc in docs:
        attrs = doc.pop("atribut", None) or {}
        doc["tipe"] = attrs.get("tipe")
        name = attrs.get("komoditas_nama") or attrs.get("nama_komoditas") or attrs.get("nama_sumber")
        if name:
            commodity_names[doc["komoditas_kode"]] = str(name)

    frame = _to_frame(docs)
    if not frame.empty:
        frame = frame.rename(columns={"periode": "tanggal"})
        for col in ("status_validasi", "satuan", "dataset", "kunci_seri"):
            if col not in frame:
                frame[col] = None
        frame["kunci_seri"] = frame["kunci_seri"].fillna(
            frame["komoditas_kode"].astype(str) + ":" + frame["kabupaten_kode"].astype(str)
        )

    latest_original_dates: dict[tuple[str, str, str], pd.Timestamp] = {}
    pipeline = [
        {
            "$match": {
                "is_carried_forward": True,
                "crawled_at": {"$lte": cutoff},
                "last_available_date": {"$ne": None},
                "komoditas_kode": {"$ne": None},
                "kabupaten_kode": {"$ne": None},
                "$or": [
                    {"sumber": {"$ne": "LAPANGAN"}},
                    {"kunci_seri": {"$regex": f":{FIELD_CONSUMER_TYPE}$"}},
                ],
            }
        },
        {
            "$group": {
                "_id": {"k": "$komoditas_kode", "w": "$kabupaten_kode", "s": "$sumber"},
                "t": {"$max": "$last_available_date"},
            }
        },
    ]
    for row in db[PRICE_COLLECTION].aggregate(pipeline):
        latest_original_dates[(row["_id"]["k"], row["_id"]["w"], row["_id"]["s"])] = _to_naive_utc(row["t"])  # type: ignore[assignment]
    return frame, commodity_names, latest_original_dates


def _read_weather(
    db: Database,
    cutoff: datetime,
    end_midnight: datetime,
    start_midnight: datetime,
) -> pd.DataFrame:
    query = {
        "is_carried_forward": False,
        "nilai": {"$ne": None},
        "source_status": {"$ne": "ERROR"},
        "periode": {"$gte": start_midnight, "$lte": end_midnight},
        "crawled_at": {"$lte": cutoff},
        "atribut.jenis_data": {"$ne": "PRAKIRAAN"},
    }
    projection = {"_id": 0, "sumber": 1, "kabupaten_kode": 1, "indikator": 1, "periode": 1, "nilai": 1, "crawled_at": 1}
    frame = _to_frame(list(db[CLIMATE_COLLECTION].find(query, projection).batch_size(5000)))
    if frame.empty:
        return pd.DataFrame(columns=["kabupaten_kode", "tanggal", "indikator", "nilai"])
    frame = frame.dropna(subset=["kabupaten_kode", "indikator"]).rename(columns={"periode": "tanggal"})
    frame["_priority"] = (frame["sumber"] == "OPEN_METEO").astype(int)
    frame = frame.sort_values(["_priority", "crawled_at"]).drop_duplicates(
        ["kabupaten_kode", "indikator", "tanggal"],
        keep="last",
    )
    return frame[["kabupaten_kode", "tanggal", "indikator", "nilai"]].reset_index(drop=True)


def _extract_explicit_release(attrs: dict[str, Any] | None) -> pd.Timestamp | None:
    raw = (attrs or {}).get("tanggal_rilis")
    if raw is None:
        return None
    parsed = pd.to_datetime(raw, utc=True, errors="coerce")
    return None if pd.isna(parsed) else parsed.tz_localize(None)


def _summarize_latest_macro(original: pd.DataFrame, all_rows: pd.DataFrame, key_cols: list[str]) -> pd.DataFrame:
    if original.empty:
        return pd.DataFrame(columns=[*_MACRO_COLUMNS, "perubahan"])
    ordered = original.sort_values("periode")
    ordered = ordered.assign(perubahan=ordered.groupby(key_cols, dropna=False)["nilai"].diff())
    latest = ordered.drop_duplicates(key_cols, keep="last").copy()
    latest["last_available_date"] = latest["periode"]
    latest["is_carried_forward"] = False

    carried = all_rows[all_rows["is_carried_forward"].fillna(False).astype(bool)]
    if not carried.empty:
        latest_cf = (
            carried.sort_values("crawled_at")
            .drop_duplicates(key_cols, keep="last")[[*key_cols, "crawled_at"]]
            .rename(columns={"crawled_at": "cf_at"})
        )
        latest_orig = (
            ordered.groupby(key_cols, dropna=False)["crawled_at"]
            .max()
            .reset_index()
            .rename(columns={"crawled_at": "orig_at"})
        )
        flags = latest_cf.merge(latest_orig, on=key_cols, how="left")
        flags = flags[flags["cf_at"] > flags["orig_at"]][key_cols].assign(_cf=True)
        latest = latest.merge(flags, on=key_cols, how="left")
        latest["is_carried_forward"] = latest["_cf"].eq(True)
        latest = latest.drop(columns="_cf")
    return latest.reindex(columns=[*_MACRO_COLUMNS, "perubahan"]).reset_index(drop=True)


def _read_macro(db: Database, cutoff: datetime, end_midnight: datetime) -> tuple[pd.DataFrame, pd.DataFrame]:
    parts: list[pd.DataFrame] = []
    projection = {
        "_id": 0,
        "sumber": 1,
        "dataset": 1,
        "indikator": 1,
        "kabupaten_kode": 1,
        "periode": 1,
        "nilai": 1,
        "satuan": 1,
        "frekuensi": 1,
        "is_carried_forward": 1,
        "last_available_date": 1,
        "crawled_at": 1,
        "source_status": 1,
        "atribut.tanggal_rilis": 1,
    }
    query = {
        "crawled_at": {"$lte": cutoff},
        "nilai": {"$ne": None},
        "source_status": {"$ne": "ERROR"},
        "periode": {"$lte": end_midnight},
    }
    for collection_name in MACRO_COLLECTIONS:
        docs = list(db[collection_name].find(query, projection).batch_size(5000))
        if not docs:
            continue
        explicit_releases = [_extract_explicit_release(doc.pop("atribut", None)) for doc in docs]
        frame = _to_frame(docs)
        frame["explicit_release"] = pd.to_datetime(pd.Series(explicit_releases, dtype="object"), errors="coerce")
        frame["domain"] = [
            map_macro_domain(collection_name, str(ind), str(ds))
            for ind, ds in zip(frame["indikator"], frame.get("dataset", ""))
        ]
        parts.append(frame)

    if not parts:
        empty = pd.DataFrame(columns=_MACRO_COLUMNS)
        return empty, empty.copy()

    combined = pd.concat(parts, ignore_index=True).rename(columns={"kabupaten_kode": "wilayah_kode"})
    combined["wilayah_kode"] = combined["wilayah_kode"].where(combined["wilayah_kode"].notna(), None)
    combined["frekuensi"] = combined["frekuensi"].fillna("BULANAN")
    key_cols = ["domain", "indikator", "wilayah_kode"]

    original = combined[~combined["is_carried_forward"].fillna(False).astype(bool)].copy()
    if not original.empty:
        first_seen = original.groupby([*key_cols, "periode"], dropna=False)["crawled_at"].transform("min")
        conservative = compute_conservative_release_date(original["periode"], original["frekuensi"])
        original["tanggal_rilis"] = original["explicit_release"].fillna(
            pd.concat([conservative, first_seen.dt.normalize()], axis=1).min(axis=1)
        )
        original = original.sort_values("crawled_at").drop_duplicates([*key_cols, "periode"], keep="last")
    else:
        original["tanggal_rilis"] = pd.NaT

    latest = _summarize_latest_macro(original, combined, key_cols)
    return original.reindex(columns=_MACRO_COLUMNS).reset_index(drop=True), latest


def read_source_freshness(db: Database, cutoff: datetime) -> list[dict[str, Any]]:
    """Aggregate latest non-backfill `crawl_runs` state per source and dataset."""
    cutoff_day = pd.Timestamp(to_wib_date(cutoff))
    pipeline = [
        {"$match": {"mulai": {"$lte": cutoff}, "pemicu": {"$ne": "BACKFILL"}}},
        {"$sort": {"mulai": -1}},
        {
            "$group": {
                "_id": {"s": "$sumber", "d": "$dataset"},
                "status": {"$first": "$status"},
                "lad": {"$first": "$last_available_date"},
                "crawled_at": {"$first": "$selesai"},
                "mulai": {"$first": "$mulai"},
                "pesan": {"$first": "$pesan"},
                "lad_maks": {"$max": "$last_available_date"},
            }
        },
        {"$sort": {"_id.s": 1, "_id.d": 1}},
    ]
    results: list[dict[str, Any]] = []
    for row in db[RUNS_COLLECTION].aggregate(pipeline):
        lad = row.get("lad") or row.get("lad_maks")
        age_days = None if lad is None else int((cutoff_day - _to_naive_utc(lad).normalize()).days)  # type: ignore[union-attr]
        results.append(
            {
                "sumber": row["_id"]["s"],
                "dataset": row["_id"]["d"],
                "status": row.get("status"),
                "last_available_date": ensure_utc(lad) if lad else None,
                "crawled_at": ensure_utc(row.get("crawled_at") or row["mulai"]),
                "umur_hari": age_days,
                "pesan": row.get("pesan") or "",
            }
        )
    return results


def extract_unseen_snapshot(
    db: Database,
    cutoff: datetime,
    history_days: int = 1100,
    previous_cutoff: datetime | None = None,
) -> RawMarketDataset:
    """Extract point-in-time prices, climate, macro indicators, and source freshness up to `cutoff`."""
    utc_cutoff = ensure_utc(cutoff)
    end_midnight = to_midnight_utc(to_wib_date(utc_cutoff))
    start_midnight = end_midnight - timedelta(days=history_days)

    raw_prices, commodity_names, latest_original = _read_prices(db, utc_cutoff, end_midnight, start_midnight)
    merged_prices = merge_price_observations(raw_prices)
    macro, latest_macro = _read_macro(db, utc_cutoff, end_midnight)

    new_points_count = 0
    if previous_cutoff is not None:
        fresh_filter = {
            "is_carried_forward": False,
            "nilai": {"$ne": None},
            "crawled_at": {"$gt": ensure_utc(previous_cutoff), "$lte": utc_cutoff},
        }
        for col_name in (PRICE_COLLECTION, *MACRO_COLLECTIONS):
            new_points_count += db[col_name].count_documents(fresh_filter)
        new_points_count += db[CLIMATE_COLLECTION].count_documents(
            {**fresh_filter, "atribut.jenis_data": {"$ne": "PRAKIRAAN"}}
        )
    elif not raw_prices.empty:
        new_points_count = len(raw_prices)

    return RawMarketDataset(
        prices=merged_prices,
        weather=_read_weather(db, utc_cutoff, end_midnight, start_midnight),
        macro=macro,
        latest_macro=latest_macro,
        freshness=read_source_freshness(db, utc_cutoff),
        commodity_names=commodity_names,
        latest_original_dates=latest_original,
        new_points_count=new_points_count,
    )


class UnseenReader:
    """Repository facade wrapping point-in-time extraction against `dataunseen`."""

    def __init__(self, db: Database) -> None:
        self._db = db

    def load_snapshot(
        self,
        cutoff: datetime,
        history_days: int = 1100,
        previous_cutoff: datetime | None = None,
    ) -> RawMarketDataset:
        return extract_unseen_snapshot(
            self._db,
            cutoff=cutoff,
            history_days=history_days,
            previous_cutoff=previous_cutoff,
        )
