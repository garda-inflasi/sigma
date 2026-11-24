"""BPS RI & BPS Aceh 70-table crawler plugin supporting Types 1, 2 & 3 with Aceh region filtering."""

from __future__ import annotations

import logging
import re
from collections import Counter
from collections.abc import Iterable, Iterator
from datetime import UTC, date, datetime
from typing import Any

from garda_crawler.adapters.base import BaseAdapter
from garda_crawler.adapters.bps_html_table import BpsExtractedRecord, extract_bps_table_from_response
from garda_crawler.config.bps_table_catalog import BPS_70_TABLES, BpsTableSourceSpec
from garda_crawler.config.reference_catalog import (
    BPS_ACEH_PROVINCE_CODE,
    INDONESIAN_MONTHS,
    REGENCIES,
)
from garda_crawler.domain.entities import (
    DataPoint,
    DatasetDefinition,
    FetchOutcome,
    RawBronzeCapture,
    TimeSeriesSpec,
)
from garda_crawler.domain.enums import Frequency
from garda_crawler.domain.errors import CrawlerError, RobotsDisallowedError
from garda_crawler.domain.protocols import HttpTransport
from garda_crawler.validation.normalizers import normalize_alias

logger = logging.getLogger("garda_crawler.bps_tables")

_ALLOWED_REGION_CODES = frozenset({BPS_ACEH_PROVINCE_CODE, *REGENCIES.keys()})


def _spec_to_dataset(spec: BpsTableSourceSpec) -> DatasetDefinition:
    return DatasetDefinition(
        name=f"bps_tbl_{spec.index:02d}_{spec.dataset_slug}",
        collection=spec.collection,
        schedule_tier="daily",
        stale_sla_days=35,
    )


def _slug(text: str) -> str:
    return re.sub(r"[^a-z0-9]+", "_", normalize_alias(text)).strip("_")[:48] or "nilai"


def _resolve_record_period(rec: BpsExtractedRecord, fallback_date: date) -> tuple[date, str]:
    year = rec.tahun or fallback_date.year
    norm_p = normalize_alias(rec.periode)
    month = INDONESIAN_MONTHS.get(norm_p)
    if month is not None:
        return date(year, month, 1), Frequency.MONTHLY
    return date(year, 1, 1), Frequency.ANNUAL


def extract_single_bps_table(
    spec: BpsTableSourceSpec,
    client: HttpTransport,
    target_date: date,
    *,
    crawl_ts: datetime | None = None,
) -> tuple[list[BpsExtractedRecord], int, str, Any]:
    """Fetch one BPS table URL, parse headers/values, and filter strictly for Aceh."""
    now = crawl_ts or datetime.now(UTC)
    crawl_iso = now.isoformat()
    html_text = client.get_text(spec.url)
    fmt, raw_payload, records, total_rows = extract_bps_table_from_response(html_text, spec, crawl_iso)
    return records, total_rows, fmt, raw_payload


def records_to_fetch_outcome(
    records: list[BpsExtractedRecord],
    spec: BpsTableSourceSpec,
    dataset: DatasetDefinition,
    raw_payload: Any,
    fmt: str,
    target_date: date,
) -> FetchOutcome:
    """Convert extracted BPS records into Garda `DataPoint`s and Bronze raw captures."""
    points: list[DataPoint] = []
    rejected: Counter[str] = Counter()

    for rec in records:
        if rec.nilai is None:
            rejected["nilai_kosong_atau_strip"] += 1
            continue
        period, freq = _resolve_record_period(rec, target_date)
        # Sertakan komoditi (URL #70) + path kolom unik tanpa bulan/tahun berulang agar tiap sub-kelompok/golongan unik.
        col_ctx = _slug(re.sub(r"\b(19\d{2}|20\d{2})\b", "", rec.kolom).strip(" >"))
        sub_key = f"{_slug(rec.komoditi)}_{col_ctx}" if rec.komoditi else (_slug(rec.sub_kelompok) if not col_ctx else col_ctx)
        sub_key = sub_key[:72].strip("_") or "nilai"
        series_key = f"{spec.dataset_slug}:{sub_key}:{rec.kode_bps}"
        unit = "persen" if "inflasi" in spec.category or "tpt" in sub_key or "prevalensi" in spec.category else "indeks/satuan"

        series = TimeSeriesSpec(
            source_code="BPS_TABLES",
            dataset_name=dataset.name,
            collection=dataset.collection,
            series_key=series_key,
            indicator=f"{spec.dataset_slug}_{sub_key}"[:80],
            unit=unit,
            frequency=freq,
            regency_code=rec.kode_bps,
            source_url=spec.url,
        )
        attrs = {
            "judul_tabel": rec.judul_tabel,
            "instansi_sumber": rec.instansi_sumber,
            "url_sumber": rec.url_sumber,
            "kategori_data": rec.kategori_data,
            "waktu_crawl": rec.waktu_crawl,
            "tahun": rec.tahun,
            "periode_label": rec.periode,
            "kolom": rec.kolom,
            "sub_kelompok": rec.sub_kelompok,
            "wilayah": rec.wilayah,
            **({"komoditi": rec.komoditi} if rec.komoditi else {}),
            **rec.extras,
        }
        points.append(
            DataPoint(
                series=series,
                period=period,
                value=float(rec.nilai),
                source_url=spec.url,
                attributes=attrs,
            )
        )

    bronze_payload = {
        "metadata": {
            "judul_tabel": spec.title,
            "instansi_sumber": spec.institution,
            "url_sumber": spec.url,
            "kategori_data": spec.category,
            "format_sumber": fmt,
            "waktu_crawl": records[0].waktu_crawl if records else datetime.now(UTC).isoformat(),
        },
        "raw_payload": raw_payload if isinstance(raw_payload, (dict, list)) else {"html_length": len(str(raw_payload))},
        "records": [r.to_dict() for r in records],
    }
    capture = RawBronzeCapture(
        source_code="BPS_TABLES",
        dataset_name=dataset.name,
        endpoint=spec.url,
        params={"table_index": spec.index, "table_type": spec.table_type},
        payload=bronze_payload,
        capture_date=target_date,
    )
    return FetchOutcome(
        points=points,
        raw_captures=[capture],
        rejected_counts=rejected,
        request_count=1,
    )


class BpsStatTablesAdapter(BaseAdapter):
    """Plugin crawler covering all 70 BPS RI & BPS Aceh statistical table URLs."""

    source_code = "BPS_TABLES"

    def __init__(
        self,
        regions: Iterable[str] | None = None,
        *,
        specs: tuple[BpsTableSourceSpec, ...] = BPS_70_TABLES,
    ) -> None:
        if regions is None:
            self.regions = (BPS_ACEH_PROVINCE_CODE, *REGENCIES.keys())
        else:
            selected = tuple(dict.fromkeys(r.strip() for r in regions if r.strip()))
            unknown = sorted(set(selected) - _ALLOWED_REGION_CODES)
            if unknown:
                raise ValueError(f"kode wilayah Aceh tidak dikenal: {', '.join(unknown)}")
            self.regions = selected
        self._specs = specs
        self._by_dataset_name = {_spec_to_dataset(s).name: s for s in self._specs}

    def list_datasets(self) -> list[DatasetDefinition]:
        return [_spec_to_dataset(s) for s in self._specs]

    def expected_series(self, dataset: DatasetDefinition) -> list[TimeSeriesSpec]:
        spec = self._by_dataset_name[dataset.name]
        codes = (BPS_ACEH_PROVINCE_CODE,) if spec.aceh_specific_rows else self.regions
        return [
            TimeSeriesSpec(
                source_code=self.source_code,
                dataset_name=dataset.name,
                collection=dataset.collection,
                series_key=f"{spec.dataset_slug}:{code}",
                indicator=spec.dataset_slug,
                unit="indeks/satuan",
                frequency=Frequency.MONTHLY if spec.category.startswith(("ihk", "inflasi")) else Frequency.ANNUAL,
                regency_code=code,
                source_url=spec.url,
            )
            for code in codes
        ]

    def fetch_daily(
        self,
        dataset: DatasetDefinition,
        client: HttpTransport,
        target_date: date,
    ) -> Iterator[FetchOutcome]:
        spec = self._by_dataset_name[dataset.name]
        try:
            records, _total_rows, fmt, raw_payload = extract_single_bps_table(spec, client, target_date)
        except RobotsDisallowedError as exc:
            logger.warning("[BPS_TABLES #%02d] DILEWATI robots.txt: %s", spec.index, spec.url)
            yield FetchOutcome(
                skipped_reason=f"DILEWATI: {client.redact_secrets(str(exc))}",
                failed_series=self.expected_series(dataset),
            )
            return
        except CrawlerError as exc:
            logger.error("[BPS_TABLES #%02d] GAGAL %s: %s", spec.index, spec.url, exc)
            yield FetchOutcome(
                request_count=1,
                failed_count=1,
                warnings=[client.redact_secrets(f"#{spec.index} {spec.dataset_slug}: {exc}")],
                failed_series=self.expected_series(dataset),
            )
            return

        filtered = [r for r in records if r.kode_bps in self.regions]
        logger.info(
            "[BPS_TABLES #%02d] OK (%s) -> %d sel Aceh diekstrak dari %s",
            spec.index,
            fmt,
            len(filtered),
            spec.url,
        )
        yield records_to_fetch_outcome(filtered, spec, dataset, raw_payload, fmt, target_date)
