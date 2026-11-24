"""Medallion Bronze reader: loads immutable `.json.gz` captures (`crawl_logs` / disk / MinIO) into Silver & RawMarketDataset."""

from __future__ import annotations

import logging
import re
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any
from urllib.parse import urlparse

import pandas as pd

from garda_ml.config.constants import FIELD_CONSUMER_TYPE, FIELD_STATUS_WEIGHTS, OFFICIAL_PRICE_SOURCES
from garda_ml.data.time_utils import ensure_utc, to_wib_date
from garda_ml.data.unseen_reader import (
    _MACRO_COLUMNS,
    _PRICE_COLUMNS,
    _summarize_latest_macro,
    compute_conservative_release_date,
)
from garda_ml.domain.entities import RawMarketDataset
from garda_ml.features.silver_transformer import (
    DimWilayahMapper,
    SilverBundle,
    parse_indonesian_decimal,
    read_bronze_json_gz,
    transform_bps_bronze_to_silver,
    transform_bps_tables_bronze_to_silver,
    transform_djpk_bronze_to_silver,
    transform_open_meteo_bronze_to_silver,
    transform_pihps_bronze_to_silver,
)

logger = logging.getLogger("garda_ml.bronze_reader")

_COMMODITY_LABELS: dict[str, str] = {
    "BRS-PRE": "Beras Kualitas Super I",
    "BRS-MED": "Beras Kualitas Medium I",
    "BWM": "Bawang Merah",
    "BWP": "Bawang Putih",
    "CBM-KRT": "Cabai Merah Keriting",
    "CBM-BSR": "Cabai Merah Besar",
    "CBR-MRH": "Cabai Rawit Merah",
    "DGG-SPI": "Daging Sapi Kualitas 1",
    "DGG-AYM": "Daging Ayam Ras Segar",
    "TLR-AYM": "Telur Ayam Ras Segar",
    "GLA": "Gula Pasir Lokal",
    "MYK": "Minyak Goreng Kemasan Bermerek 1",
}


@dataclass(frozen=True)
class BronzeCaptureRef:
    """Reference to one persisted Bronze object and its crawl metadata."""

    source_code: str
    dataset_code: str
    params: dict[str, Any]
    uri: str
    status: str
    crawled_at: datetime
    error_message: str = ""


def _parse_dt_from_uri(uri: str) -> datetime:
    match = re.search(r"dt=(\d{4}-\d{2}-\d{2})", uri)
    if match:
        return datetime.fromisoformat(f"{match.group(1)}T00:00:00+00:00")
    return datetime.now(UTC)


def _merge_bronze_prices(raw_prices: pd.DataFrame) -> pd.DataFrame:
    if raw_prices.empty:
        return pd.DataFrame(columns=_PRICE_COLUMNS)

    df = raw_prices[raw_prices["nilai"].notna() & (raw_prices["nilai"] > 0)].copy()
    if df.empty:
        return pd.DataFrame(columns=_PRICE_COLUMNS)

    df["tanggal"] = pd.to_datetime(df["periode"], errors="coerce").dt.normalize()
    df = df.dropna(subset=["tanggal"])
    df["kabupaten_kode"] = df["kode_bps"].astype(str)
    df["bobot"] = df.get("bobot", pd.Series(1.0, index=df.index)).fillna(1.0).astype(float)

    field = df[df["sumber"] == "LAPANGAN"]
    if field.empty:
        daily_field = pd.DataFrame(
            columns=["komoditas_kode", "kabupaten_kode", "tanggal", "nilai_lapangan", "bobot_lapangan"]
        )
    else:
        w = field.assign(_w=field["nilai"] * field["bobot"])
        grp = w.groupby(["komoditas_kode", "kabupaten_kode", "tanggal"], as_index=False).agg(
            _w=("_w", "sum"),
            b_sum=("bobot", "sum"),
            bobot_lapangan=("bobot", "max"),
        )
        grp["nilai_lapangan"] = grp["_w"] / grp["b_sum"]
        daily_field = grp[["komoditas_kode", "kabupaten_kode", "tanggal", "nilai_lapangan", "bobot_lapangan"]]

    official = df[df["sumber"] != "LAPANGAN"].copy()
    if official.empty:
        merged = daily_field.rename(columns={"nilai_lapangan": "nilai", "bobot_lapangan": "bobot"})
        merged["nilai_lapangan"] = merged["nilai"]
        merged["bobot_lapangan"] = merged["bobot"]
        merged["sumber"] = "LAPANGAN"
        merged["satuan"] = "Rp/kg"
        return merged.reindex(columns=_PRICE_COLUMNS).sort_values(
            ["komoditas_kode", "kabupaten_kode", "tanggal"]
        ).reset_index(drop=True)

    priority = {src: idx for idx, src in enumerate(OFFICIAL_PRICE_SOURCES)}
    official["_prio"] = official["sumber"].map(lambda s: priority.get(str(s), 99))
    official = (
        official.sort_values(["_prio", "tanggal"])
        .drop_duplicates(["komoditas_kode", "kabupaten_kode", "tanggal"], keep="first")
        .drop(columns=["_prio"])
    )
    merged = official.merge(daily_field, on=["komoditas_kode", "kabupaten_kode", "tanggal"], how="left")
    return merged.reindex(columns=_PRICE_COLUMNS).sort_values(
        ["komoditas_kode", "kabupaten_kode", "tanggal"]
    ).reset_index(drop=True)


class BronzeReader:
    """Loads Bronze `.json.gz` captures directly (PostgreSQL `crawl_logs` / disk / MinIO) without MongoDB `dataunseen`."""

    def __init__(
        self,
        *,
        data_raw_dir: str | Path | None = None,
        postgres_url: str = "",
        minio_endpoint: str = "",
        minio_access_key: str = "",
        minio_secret_key: str = "",
        minio_bucket_raw: str = "data-raw",
        minio_secure: bool = False,
        s3_client: Any = None,
    ) -> None:
        self.local_dir = Path(data_raw_dir).resolve() if data_raw_dir else None
        self.postgres_url = postgres_url.strip()
        self.minio_endpoint = minio_endpoint.strip()
        self.minio_bucket = minio_bucket_raw.strip() or "data-raw"
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

    def _list_from_postgres(self, cutoff: datetime) -> list[BronzeCaptureRef]:
        if not self.postgres_url:
            return []
        import psycopg

        refs: list[BronzeCaptureRef] = []
        with psycopg.connect(self.postgres_url, connect_timeout=5) as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    SELECT DISTINCT ON (source_code, dataset_code, params_hash)
                        source_code, dataset_code, params_json, bronze_uri, status, crawled_at, error_message
                    FROM crawl_logs
                    WHERE crawled_at <= %s
                      AND status IN ('WRITTEN', 'SKIPPED_DUPLICATE')
                      AND bronze_uri <> ''
                    ORDER BY source_code, dataset_code, params_hash, crawled_at DESC, id DESC
                    """,
                    (cutoff,),
                )
                for row in cur.fetchall():
                    refs.append(
                        BronzeCaptureRef(
                            source_code=str(row[0]).upper(),
                            dataset_code=str(row[1]),
                            params=dict(row[2] or {}),
                            uri=str(row[3]),
                            status=str(row[4]),
                            crawled_at=ensure_utc(row[5]),
                            error_message=str(row[6] or ""),
                        )
                    )
        return refs

    def _list_from_local_disk(self) -> list[BronzeCaptureRef]:
        if self.local_dir is None or not self.local_dir.is_dir():
            return []
        latest_by_key: dict[tuple[str, str, str], tuple[datetime, BronzeCaptureRef]] = {}
        for path in sorted(self.local_dir.rglob("*.json.gz")):
            try:
                rel = path.relative_to(self.local_dir)
            except ValueError:
                continue
            parts = rel.parts
            if len(parts) < 3:
                continue
            src = parts[0].upper()
            ds = parts[1]
            dt = _parse_dt_from_uri(rel.as_posix())
            key = (src, ds, path.name)
            ref = BronzeCaptureRef(
                source_code=src,
                dataset_code=ds,
                params={},
                uri=str(path),
                status="WRITTEN",
                crawled_at=dt,
            )
            prev = latest_by_key.get(key)
            if prev is None or dt >= prev[0]:
                latest_by_key[key] = (dt, ref)
        return [item[1] for item in latest_by_key.values()]

    def _read_payload(self, uri: str) -> Any:
        if uri.startswith("s3://"):
            if self._s3 is None:
                raise RuntimeError(f"S3 client tidak dikonfigurasi untuk membaca {uri}")
            parsed = urlparse(uri)
            bucket = parsed.netloc or self.minio_bucket
            key = parsed.path.lstrip("/")
            raw = self._s3.get_object(Bucket=bucket, Key=key)["Body"].read()
            return read_bronze_json_gz(raw)
        path = Path(uri)
        if not path.is_file() and self.local_dir is not None:
            # Handle container vs host path prefix differences (e.g. /app/data/raw/... vs local DATA_RAW_DIR).
            marker = "/data/raw/"
            if marker in uri:
                candidate = self.local_dir / uri.split(marker, 1)[1]
                if candidate.is_file():
                    path = candidate
        return read_bronze_json_gz(path)

    def _read_pg_external_prices(self, cutoff_day: pd.Timestamp) -> pd.DataFrame:
        if not self.postgres_url:
            return pd.DataFrame()
        import psycopg

        try:
            with psycopg.connect(self.postgres_url, connect_timeout=5) as conn:
                with conn.cursor() as cur:
                    cur.execute(
                        """
                        SELECT h.komoditas_kode, h.kabupaten_kode, h.tanggal, h.harga::double precision, h.satuan
                        FROM harga_eksternal h
                        WHERE h.tipe = 'KONSUMEN' AND h.tanggal <= %s
                        """,
                        (cutoff_day.date(),),
                    )
                    rows = cur.fetchall()
        except Exception as exc:
            logger.debug("opsional harga_eksternal PG dilewati: %s", exc)
            return pd.DataFrame()

        if not rows:
            return pd.DataFrame()
        return pd.DataFrame(
            [
                {
                    "kode_bps": str(r[1]),
                    "komoditas_kode": str(r[0]),
                    "periode": pd.Timestamp(r[2]),
                    "nilai": float(r[3]),
                    "sumber": "PIHPS",
                    "satuan": str(r[4] or "Rp/kg"),
                    "bobot": 1.0,
                }
                for r in rows
            ]
        )

    def _build_freshness(
        self,
        refs: list[BronzeCaptureRef],
        cutoff_day: pd.Timestamp,
        macro_df: pd.DataFrame,
        prices_df: pd.DataFrame,
    ) -> list[dict[str, Any]]:
        latest_date_by_source: dict[str, pd.Timestamp] = {}
        if not prices_df.empty:
            for src, grp in prices_df.groupby("sumber"):
                latest_date_by_source[str(src)] = pd.Timestamp(grp["tanggal"].max()).normalize()
        if not macro_df.empty:
            for src, grp in macro_df.groupby("sumber"):
                latest_date_by_source[str(src)] = pd.Timestamp(grp["periode"].max()).normalize()

        grouped: dict[tuple[str, str], BronzeCaptureRef] = {}
        for ref in refs:
            key = (ref.source_code, ref.dataset_code)
            prev = grouped.get(key)
            if prev is None or ref.crawled_at >= prev.crawled_at:
                grouped[key] = ref

        out: list[dict[str, Any]] = []
        for (src, ds), ref in sorted(grouped.items()):
            lad = latest_date_by_source.get(src) or pd.Timestamp(to_wib_date(ref.crawled_at))
            age_days = max(0, int((cutoff_day - lad.normalize()).days))
            out.append(
                {
                    "sumber": src,
                    "dataset": ds,
                    "status": "FRESH" if ref.status in {"WRITTEN", "SKIPPED_DUPLICATE"} else "ERROR",
                    "last_available_date": lad.to_pydatetime().replace(tzinfo=UTC),
                    "crawled_at": ref.crawled_at,
                    "umur_hari": age_days,
                    "pesan": f"Bronze {ref.status}",
                }
            )
        return out

    def load_silver_and_market(
        self,
        *,
        cutoff: datetime,
        history_days: int = 1100,
    ) -> tuple[SilverBundle, RawMarketDataset]:
        """Read Bronze captures, transform to Silver + Pandera, and assemble RawMarketDataset."""
        utc_cutoff = ensure_utc(cutoff)
        cutoff_day = pd.Timestamp(to_wib_date(utc_cutoff)).normalize()
        start_day = cutoff_day - timedelta(days=history_days)
        mapper = DimWilayahMapper.from_postgres(self.postgres_url)

        refs = self._list_from_postgres(utc_cutoff) or self._list_from_local_disk()
        price_parts: list[pd.DataFrame] = []
        macro_parts: list[pd.DataFrame] = []
        djpk_parts: list[pd.DataFrame] = []
        weather_parts: list[pd.DataFrame] = []
        commodity_names: dict[str, str] = dict(_COMMODITY_LABELS)

        for ref in refs:
            try:
                payload = self._read_payload(ref.uri)
            except Exception as exc:
                logger.warning("gagal membaca Bronze %s (%s): %s", ref.uri, ref.dataset_code, exc)
                continue

            if ref.source_code == "BPS_TABLES":
                df = transform_bps_tables_bronze_to_silver(payload, mapper=mapper, dataset_code=ref.dataset_code)
                if not df.empty:
                    df["crawled_at"] = pd.Timestamp(ref.crawled_at.replace(tzinfo=None))
                    macro_parts.append(df)
            elif ref.source_code == "BPS":
                df = transform_bps_bronze_to_silver(payload, mapper=mapper, indicator_fallback=ref.dataset_code)
                if not df.empty:
                    df["frekuensi"] = "BULANAN"
                    df["satuan"] = "indeks"
                    df["crawled_at"] = pd.Timestamp(ref.crawled_at.replace(tzinfo=None))
                    macro_parts.append(df)
            elif ref.source_code == "DJPK":
                prov = str(ref.params.get("provinsi") or "01")
                pemda = str(ref.params.get("pemda") or "00")
                tahun = int(ref.params.get("tahun") or cutoff_day.year)
                df = transform_djpk_bronze_to_silver(payload, provinsi=prov, pemda=pemda, tahun=tahun, mapper=mapper)
                if not df.empty:
                    df["frekuensi"] = "TAHUNAN"
                    df["satuan"] = "Rp"
                    df["crawled_at"] = pd.Timestamp(ref.crawled_at.replace(tzinfo=None))
                    djpk_parts.append(df)
            elif ref.source_code == "PIHPS":
                kode_bps = str(ref.params.get("kabupaten_kode") or "").strip() or (
                    mapper.resolve_pihps(ref.params.get("regency_id")) or ""
                )
                if not kode_bps:
                    m_reg = re.search(r"regency_id=(\d+)", ref.uri)
                    if m_reg:
                        kode_bps = mapper.resolve_pihps(m_reg.group(1)) or ""
                if kode_bps:
                    df = transform_pihps_bronze_to_silver(payload, kode_bps=kode_bps, sumber="PIHPS")
                    if not df.empty:
                        df["bobot"] = 1.0
                        price_parts.append(df)
            elif ref.source_code == "OPEN_METEO":
                kode_bps = str(ref.params.get("kabupaten_kode") or "").strip()
                if kode_bps:
                    df = transform_open_meteo_bronze_to_silver(payload, kode_bps=kode_bps)
                    if not df.empty:
                        weather_parts.append(df)
            elif ref.source_code == "LAPANGAN" and isinstance(payload, dict):
                rows = []
                for entry in payload.get("records") or []:
                    if not isinstance(entry, dict) or entry.get("tipe") != FIELD_CONSUMER_TYPE:
                        continue
                    weight = FIELD_STATUS_WEIGHTS.get(str(entry.get("status") or ""))
                    kode_bps = mapper.resolve_bps(entry.get("kabupaten_kode"))
                    val = parse_indonesian_decimal(entry.get("harga"))
                    ts = pd.to_datetime(entry.get("dicatat_pada"), utc=True, errors="coerce")
                    if weight is None or kode_bps is None or pd.isna(ts) or pd.isna(val) or val <= 0:
                        continue
                    rows.append(
                        {
                            "kode_bps": kode_bps,
                            "komoditas_kode": str(entry.get("komoditas_kode") or ""),
                            "periode": ts.tz_localize(None).normalize(),
                            "nilai": float(val),
                            "sumber": "LAPANGAN",
                            "satuan": f"Rp/{str(entry.get('satuan_master') or 'kg').lower()}",
                            "bobot": float(weight),
                        }
                    )
                if rows:
                    price_parts.append(pd.DataFrame(rows))

        raw_prices = pd.concat(price_parts, ignore_index=True) if price_parts else self._read_pg_external_prices(cutoff_day)
        if not raw_prices.empty:
            raw_prices = raw_prices[
                (raw_prices["periode"] >= start_day) & (raw_prices["periode"] <= cutoff_day)
            ].copy()
        merged_prices = _merge_bronze_prices(raw_prices)

        silver_prices = (
            raw_prices[["kode_bps", "komoditas_kode", "periode", "nilai", "sumber"]].copy()
            if not raw_prices.empty
            else pd.DataFrame(columns=["kode_bps", "komoditas_kode", "periode", "nilai", "sumber"])
        )
        all_macro_parts = [*macro_parts, *djpk_parts]
        combined_macro = (
            pd.concat(all_macro_parts, ignore_index=True)
            if all_macro_parts
            else pd.DataFrame(columns=["kode_bps", "domain", "indikator", "periode", "nilai", "sumber", "frekuensi", "satuan", "crawled_at"])
        )
        if not combined_macro.empty:
            combined_macro = combined_macro[combined_macro["periode"] <= cutoff_day].copy()
            combined_macro["wilayah_kode"] = combined_macro["kode_bps"].astype(str)
            combined_macro["frekuensi"] = combined_macro["frekuensi"].fillna("BULANAN")
            combined_macro["satuan"] = combined_macro["satuan"].fillna("satuan")
            combined_macro["is_carried_forward"] = False
            combined_macro["last_available_date"] = combined_macro["periode"]
            combined_macro["tanggal_rilis"] = compute_conservative_release_date(
                combined_macro["periode"], combined_macro["frekuensi"]
            )
            key_cols = ["domain", "indikator", "wilayah_kode"]
            combined_macro = combined_macro.sort_values("crawled_at").drop_duplicates(
                [*key_cols, "periode"], keep="last"
            )
            latest_macro = _summarize_latest_macro(combined_macro, combined_macro, key_cols)
            macro_market = combined_macro.reindex(columns=_MACRO_COLUMNS).reset_index(drop=True)
        else:
            macro_market = pd.DataFrame(columns=_MACRO_COLUMNS)
            latest_macro = pd.DataFrame(columns=[*_MACRO_COLUMNS, "perubahan"])

        silver_macro = (
            pd.concat(macro_parts, ignore_index=True)[["kode_bps", "domain", "indikator", "periode", "nilai", "sumber"]]
            if macro_parts
            else pd.DataFrame(columns=["kode_bps", "domain", "indikator", "periode", "nilai", "sumber"])
        )
        silver_djpk = (
            pd.concat(djpk_parts, ignore_index=True)[["kode_bps", "domain", "indikator", "periode", "nilai", "sumber"]]
            if djpk_parts
            else pd.DataFrame(columns=["kode_bps", "domain", "indikator", "periode", "nilai", "sumber"])
        )
        weather_df = (
            pd.concat(weather_parts, ignore_index=True)
            .drop_duplicates(["kabupaten_kode", "indikator", "tanggal"], keep="last")
            .reset_index(drop=True)
            if weather_parts
            else pd.DataFrame(columns=["kabupaten_kode", "tanggal", "indikator", "nilai"])
        )

        bundle = SilverBundle(
            prices=silver_prices,
            macro=silver_macro,
            djpk=silver_djpk,
            weather=weather_df,
        ).validated()

        freshness = self._build_freshness(refs, cutoff_day, macro_market, merged_prices)
        new_points = len(merged_prices) + len(macro_market)
        dataset = RawMarketDataset(
            prices=merged_prices,
            weather=weather_df,
            macro=macro_market,
            latest_macro=latest_macro,
            freshness=freshness,
            commodity_names=commodity_names,
            new_points_count=new_points,
        )
        return bundle, dataset
