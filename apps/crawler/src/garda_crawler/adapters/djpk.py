"""DJPK Kemenkeu APBD portal XHR/JSON adapter plugin."""

from collections import Counter
from collections.abc import Iterable, Iterator
from datetime import date
from typing import Any

from garda_crawler.adapters.base import BaseAdapter
from garda_crawler.config.reference_catalog import (
    BPS_ACEH_PROVINCE_CODE,
    BPS_TO_DJPK_REGIONS,
    DJPK_TO_BPS_REGIONS,
    REGENCIES,
)
from garda_crawler.config.source_specs import DJPK_APBD_DATASET
from garda_crawler.domain.entities import (
    DataPoint,
    DatasetDefinition,
    FetchOutcome,
    RawBronzeCapture,
    TimeSeriesSpec,
)
from garda_crawler.domain.enums import Frequency
from garda_crawler.domain.errors import CrawlerError, RobotsDisallowedError, SchemaValidationError
from garda_crawler.domain.protocols import HttpTransport
from garda_crawler.validation.normalizers import parse_decimal

DJPK_DEFAULT_BASE_URL = "https://djpk.kemenkeu.go.id/portal/data/apbd"
DJPK_DEFAULT_PERIODE = 12


def _build_series(region_code: str, indicator: str, endpoint: str) -> TimeSeriesSpec:
    return TimeSeriesSpec(
        source_code="DJPK",
        dataset_name=DJPK_APBD_DATASET.name,
        collection=DJPK_APBD_DATASET.collection,
        series_key=f"{indicator}:{region_code}",
        indicator=indicator,
        unit="Rp",
        frequency=Frequency.ANNUAL,
        regency_code=region_code,
        source_url=endpoint,
    )


def parse_djpk_apbd_payload(
    payload: Any,
    *,
    tahun: int,
    provinsi: str,
    pemda: str,
    source_url: str,
) -> FetchOutcome:
    """Parse DJPK XHR/JSON APBD payload into normalized fiscal DataPoints."""
    if not isinstance(payload, (dict, list)):
        raise SchemaValidationError("respons DJPK harus berupa JSON dict atau list")

    region_code = DJPK_TO_BPS_REGIONS.get((provinsi, pemda))
    if region_code is None:
        return FetchOutcome(rejected_counts=Counter({"wilayah_djpk_tak_terpetakan": 1}))

    rows: list[Any]
    if isinstance(payload, list):
        rows = payload
    else:
        rows = payload.get("data") or payload.get("rows") or payload.get("aaData") or []
    if not isinstance(rows, list):
        raise SchemaValidationError("daftar baris APBD DJPK tidak valid")

    outcome = FetchOutcome()
    period = date(tahun, 1, 1)
    for row in rows:
        if not isinstance(row, dict):
            continue
        raw_name = str(
            row.get("uraian")
            or row.get("akun")
            or row.get("nama_akun")
            or row.get("label")
            or "belanja_daerah"
        ).strip()
        slug = "".join(ch if ch.isalnum() else "_" for ch in raw_name.lower()).strip("_")[:48] or "apbd_pos"
        raw_val = row.get("realisasi") if row.get("realisasi") is not None else row.get("anggaran")
        if raw_val is None:
            raw_val = row.get("nilai")
        value = parse_decimal(raw_val)
        if value is None or value < 0:
            outcome.rejected_counts["nilai_apbd_tidak_valid"] += 1
            continue

        indicator = f"apbd_{slug}"
        outcome.points.append(
            DataPoint(
                series=_build_series(region_code, indicator, source_url),
                period=period,
                value=value,
                source_url=source_url,
                attributes={"provinsi": provinsi, "pemda": pemda, "tahun": tahun, "uraian": raw_name},
            )
        )
    return outcome


class DjpkKemenkeuAdapter(BaseAdapter):
    """DJPK Kemenkeu regional budget (APBD) XHR/JSON plugin producing Bronze captures & fiscal series."""

    source_code = "DJPK"

    def __init__(
        self,
        *,
        base_url: str = DJPK_DEFAULT_BASE_URL,
        regions: Iterable[str] | None = None,
    ) -> None:
        self.base_url = base_url.strip().rstrip("/") or DJPK_DEFAULT_BASE_URL
        if regions is None:
            self.regions = (BPS_ACEH_PROVINCE_CODE, *REGENCIES.keys())
        else:
            selected = tuple(dict.fromkeys(r.strip() for r in regions if r.strip()))
            unknown = sorted(set(selected) - set(BPS_TO_DJPK_REGIONS))
            if unknown:
                raise ValueError(f"kode wilayah DJPK tidak dikenal: {', '.join(unknown)}")
            self.regions = selected

    def list_datasets(self) -> list[DatasetDefinition]:
        return [DJPK_APBD_DATASET]

    def expected_series(self, dataset: DatasetDefinition) -> list[TimeSeriesSpec]:
        return [_build_series(code, "apbd_belanja_daerah", self.base_url) for code in self.regions]

    def _fetch_years(
        self,
        client: HttpTransport,
        years: list[int],
        capture_date: date,
    ) -> FetchOutcome:
        combined = FetchOutcome()
        for region_code in self.regions:
            provinsi, pemda = BPS_TO_DJPK_REGIONS[region_code]
            for tahun in years:
                params = {
                    "periode": DJPK_DEFAULT_PERIODE,
                    "provinsi": provinsi,
                    "pemda": pemda,
                    "tahun": tahun,
                }
                combined.request_count += 1
                try:
                    payload = client.get_json(self.base_url, params=params)
                    parsed = parse_djpk_apbd_payload(
                        payload,
                        tahun=tahun,
                        provinsi=provinsi,
                        pemda=pemda,
                        source_url=self.base_url,
                    )
                except RobotsDisallowedError as exc:
                    combined.request_count -= 1
                    combined.skipped_reason = f"DILEWATI: {client.redact_secrets(str(exc))}"
                    return combined
                except CrawlerError as exc:
                    combined.failed_count += 1
                    combined.warnings.append(client.redact_secrets(f"DJPK {region_code} {tahun}: {exc}"))
                    continue

                combined.raw_captures.append(
                    RawBronzeCapture(
                        source_code=self.source_code,
                        dataset_name=DJPK_APBD_DATASET.name,
                        endpoint=self.base_url,
                        params=params,
                        payload=payload,
                        partition_year=tahun,
                        capture_date=capture_date,
                    )
                )
                combined.points.extend(parsed.points)
                combined.rejected_counts.update(parsed.rejected_counts)
        return combined

    def fetch_daily(
        self,
        dataset: DatasetDefinition,
        client: HttpTransport,
        target_date: date,
    ) -> Iterator[FetchOutcome]:
        yield self._fetch_years(client, [target_date.year - 1, target_date.year], target_date)

    def fetch_backfill(
        self,
        dataset: DatasetDefinition,
        client: HttpTransport,
        start_date: date,
        end_date: date,
    ) -> Iterator[FetchOutcome]:
        years = list(range(start_date.year, end_date.year + 1))
        yield self._fetch_years(client, years, end_date)
