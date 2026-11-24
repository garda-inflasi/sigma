"""BPS WebAPI v1 adapter plugin for macro, inflation, labor, and primary-sector datasets."""

import base64
import json
import re
from collections import Counter
from collections.abc import Iterable, Iterator, Mapping
from datetime import date
from typing import Any

from garda_crawler.adapters.base import BaseAdapter, format_skip_message
from garda_crawler.config.reference_catalog import (
    BPS_ACEH_PROVINCE_CODE,
    BPS_REGIONS,
    INDONESIAN_MONTHS,
    REGENCIES,
)
from garda_crawler.config.source_specs import BPS_DATASET_SPECS, BpsIndicatorSpec
from garda_crawler.domain.entities import (
    DataPoint,
    DatasetDefinition,
    FetchOutcome,
    RawBronzeCapture,
    TimeSeriesSpec,
)
from garda_crawler.domain.errors import CrawlerError, HttpFetchError, RobotsDisallowedError, SchemaValidationError
from garda_crawler.domain.protocols import HttpTransport
from garda_crawler.validation.normalizers import normalize_alias

BPS_WEBAPI_BASE_URL = "https://webapi.bps.go.id/v1/api"
BPS_ACEH_DOMAIN = "1100"
BPS_NATIONAL_DOMAIN = "0000"
BPS_BASE_YEAR = 1900
MAX_VAR_PAGES = 50
BPS_IHK150_TABLE_TOKEN = "MjI0NSMy"
BPS_IHK150_TABLE_URL = (
    "https://www.bps.go.id/id/statistics-table/2/MjI0NSMy/"
    "indeks-harga-konsumen-150-kabupaten-kota-2022-100.html"
)


def decode_bps_table_token(token: str) -> tuple[str, str]:
    """Decode a base64 BPS table path token like `MjI0NSMy` into `(var_id, table_type)`."""
    padded = token.strip() + "=" * (-len(token.strip()) % 4)
    decoded = base64.b64decode(padded).decode("utf-8")
    parts = decoded.split("#", 1)
    if len(parts) != 2:
        raise SchemaValidationError(f"token tabel BPS tidak valid: {token!r}")
    return parts[0], parts[1]


def extract_bps_table_json_from_html(html: str) -> dict[str, Any]:
    """Extract the embedded Next.js RSC JSON table payload (`status`, `var`, `vervar`, `datacontent`)."""
    match = re.search(r'(\{\\"status\\":\\"OK\\".*?\\"datacontent\\":\{.*?\}\})', html)
    if not match:
        raise SchemaValidationError("payload JSON tabel BPS tidak ditemukan pada respons portal")
    raw_json = match.group(1).replace('\\"', '"').replace("\\\\", "\\")
    parsed = json.loads(raw_json)
    if not isinstance(parsed, dict) or "datacontent" not in parsed:
        raise SchemaValidationError("struktur JSON tabel BPS tidak lengkap")
    return parsed

MISSING_KEY_MESSAGE = format_skip_message(
    "BPS_API_KEY belum diisi, halaman tabel BPS tidak di-scrape sebagai gantinya",
    "isi BPS_API_KEY (daftar di webapi.bps.go.id)",
)
ROBOTS_ACTION_HINT = "minta izin akses WebAPI ke BPS"

QUARTER_START_MONTHS: dict[str, int] = {
    "i": 1,
    "ii": 4,
    "iii": 7,
    "iv": 10,
    "1": 1,
    "2": 4,
    "3": 7,
    "4": 10,
}

_ALL_ALLOWED_CODES = frozenset({"0000", BPS_ACEH_PROVINCE_CODE, *REGENCIES.keys()})


def _build_series(dataset: DatasetDefinition, spec: BpsIndicatorSpec, region_code: str) -> TimeSeriesSpec:
    return TimeSeriesSpec(
        source_code="BPS",
        dataset_name=dataset.name,
        collection=dataset.collection,
        series_key=f"{spec.key}:{region_code}",
        indicator=spec.indicator,
        unit=spec.unit,
        frequency=spec.frequency,
        regency_code=region_code,
        source_url=BPS_WEBAPI_BASE_URL,
    )


def build_bps_data_url(domain: str, var_id: str, th_id: int, api_key: str) -> str:
    """Build BPS WebAPI v1 data query URL."""
    return f"{BPS_WEBAPI_BASE_URL}/list/model/data/lang/ind/domain/{domain}/var/{var_id}/th/{th_id}/key/{api_key}"


def strip_bps_key(url: str) -> str:
    """Remove `/key/<secret>` segment from a BPS URL prior to metadata storage."""
    return re.sub(r"/key/[^/?]+", "", url)


def resolve_bps_period(year: int, label: str, raw_val: Any, frequency: str) -> date | None:
    """Convert BPS `turtahun` labels into canonical first-day period dates."""
    text = normalize_alias(label)
    if frequency == "BULANAN":
        if text in INDONESIAN_MONTHS:
            return date(year, INDONESIAN_MONTHS[text], 1)
        if "tahun" in text:
            return None
        try:
            month = int(str(raw_val))
        except ValueError:
            return None
        return date(year, month, 1) if 1 <= month <= 12 else None

    if frequency == "TRIWULANAN":
        matched = re.search(r"triwulan\s+(iv|iii|ii|i|[1-4])\b", text)
        return date(year, QUARTER_START_MONTHS[matched.group(1)], 1) if matched else None

    if text in INDONESIAN_MONTHS or "triwulan" in text or "semester" in text:
        return None
    return date(year, 1, 1)


def _select_total_turvar(turvar: list[dict[str, Any]]) -> dict[str, Any] | None:
    if not turvar:
        return {"val": 0, "label": "Tidak Ada"}
    if len(turvar) == 1:
        return turvar[0]
    for item in turvar:
        if re.search(r"\b(jumlah|total|tidak ada)\b", str(item.get("label", "")).lower()):
            return item
    return None


def _within_indicator_bounds(spec: BpsIndicatorSpec, value: float) -> bool:
    if spec.unit == "persen":
        return abs(value) <= 100
    if spec.unit == "indeks":
        return 0 < value < 1000
    return value >= 0


def parse_bps_payload(
    payload: Any,
    dataset: DatasetDefinition,
    spec: BpsIndicatorSpec,
    safe_url: str,
) -> FetchOutcome:
    """Pure parser translating a single BPS WebAPI variable payload into DataPoints."""
    if not isinstance(payload, dict):
        raise SchemaValidationError("respons BPS tidak terbaca")

    status = str(payload.get("status", "OK"))
    if status.upper() != "OK":
        raise HttpFetchError(f"BPS menjawab status {status!r}")

    outcome = FetchOutcome()
    if "not-available" in str(payload.get("data-availability", "")):
        return outcome

    variables = payload.get("var") or []
    if len(variables) != 1:
        raise SchemaValidationError(f"respons BPS harus memuat tepat satu variabel, ada {len(variables)}")

    turvar = _select_total_turvar(payload.get("turvar") or [])
    if turvar is None:
        outcome.rejected_counts["turvar_ambigu"] += 1
        return outcome

    datacontent = payload.get("datacontent") or {}
    var_val = variables[0]["val"]

    for th in payload.get("tahun") or []:
        try:
            year = int(str(th.get("label")).strip())
        except ValueError:
            outcome.rejected_counts["tahun_rusak"] += 1
            continue

        for tt in payload.get("turtahun") or []:
            period = resolve_bps_period(year, str(tt.get("label", "")), tt.get("val"), spec.frequency)
            if period is None:
                continue
            for vv in payload.get("vervar") or []:
                cell_key = f"{vv['val']}{var_val}{turvar['val']}{th['val']}{tt['val']}"
                if cell_key not in datacontent:
                    continue

                region_code = BPS_REGIONS.get(normalize_alias(str(vv.get("label", ""))))
                if region_code is None:
                    outcome.rejected_counts["wilayah_tak_terpetakan"] += 1
                    continue

                try:
                    value = float(str(datacontent[cell_key]).strip())
                except ValueError:
                    outcome.rejected_counts["nilai_bukan_angka"] += 1
                    continue

                if not _within_indicator_bounds(spec, value):
                    outcome.rejected_counts["nilai_di_luar_batas"] += 1
                    continue

                outcome.points.append(
                    DataPoint(
                        series=_build_series(dataset, spec, region_code),
                        period=period,
                        value=value,
                        source_url=safe_url,
                        attributes={
                            "var_id": str(var_val),
                            "label_vervar": vv.get("label"),
                            "turtahun": tt.get("label"),
                        },
                    )
                )
    return outcome


def lookup_var_id(
    client: HttpTransport,
    api_key: str,
    domain: str,
    spec: BpsIndicatorSpec,
    var_cache: dict[str, list[dict[str, Any]]],
) -> str:
    """Resolve an unambiguous BPS `var_id` from domain variable catalog using title regex."""
    if domain not in var_cache:
        all_vars: list[dict[str, Any]] = []
        for page in range(1, MAX_VAR_PAGES + 1):
            url = f"{BPS_WEBAPI_BASE_URL}/list/model/var/lang/ind/domain/{domain}/page/{page}/key/{api_key}"
            payload = client.get_json(url)
            if isinstance(payload, dict) and str(payload.get("status", "OK")).upper() != "OK":
                raise HttpFetchError(f"BPS menjawab status {payload.get('status')!r} saat membaca daftar variabel")
            data = payload.get("data") if isinstance(payload, dict) else None
            if not isinstance(data, list) or len(data) < 2:
                break
            all_vars.extend(data[1])
            if page >= int(data[0].get("pages", 1)):
                break
        var_cache[domain] = all_vars

    matches = [item for item in var_cache[domain] if re.search(spec.title_pattern, str(item.get("title", "")), re.IGNORECASE)]
    if len(matches) == 1:
        return str(matches[0]["var_id"])
    if not matches:
        raise HttpFetchError(f"variabel {spec.key} tidak ditemukan; isi BPS_VARIABEL_JSON")
    candidates = ", ".join(str(item.get("var_id")) for item in matches[:8])
    raise HttpFetchError(f"variabel {spec.key} ambigu (kandidat var_id: {candidates}); isi BPS_VARIABEL_JSON")


class BpsWebApiAdapter(BaseAdapter):
    """Official BPS WebAPI adapter supporting dynamic variable lookup and year-based backfills."""

    source_code = "BPS"

    def __init__(
        self,
        api_key: str = "",
        variables: Mapping[str, str] | None = None,
        regions: Iterable[str] | None = None,
    ) -> None:
        # BPS supports province code "11" in addition to the 23 regencies.
        if regions is None:
            self.regions = ("0000", BPS_ACEH_PROVINCE_CODE, *REGENCIES.keys())
        else:
            selected = tuple(dict.fromkeys(code.strip() for code in regions if code.strip()))
            unknown = sorted(set(selected) - _ALL_ALLOWED_CODES)
            if unknown:
                raise ValueError(f"kode wilayah BPS tidak dikenal: {', '.join(unknown)}")
            self.regions = selected
        self.api_key = api_key.strip()
        self.variables = dict(variables or {})
        self._var_cache: dict[str, list[dict[str, Any]]] = {}

    def _includes_region(self, code: str) -> bool:
        return code in self.regions

    def list_datasets(self) -> list[DatasetDefinition]:
        return list(BPS_DATASET_SPECS.keys())

    def expected_series(self, dataset: DatasetDefinition) -> list[TimeSeriesSpec]:
        return [
            _build_series(dataset, spec, region_code)
            for spec in BPS_DATASET_SPECS[dataset]
            for region_code in spec.regions
            if self._includes_region(region_code)
        ]

    def _fetch_year_range(
        self,
        dataset: DatasetDefinition,
        client: HttpTransport,
        start_year: int,
        end_year: int,
        capture_date: date | None = None,
    ) -> FetchOutcome:
        combined = FetchOutcome()
        if not self.api_key:
            combined.skipped_reason = MISSING_KEY_MESSAGE
            combined.failed_series.extend(self.expected_series(dataset))
            return combined

        for spec in BPS_DATASET_SPECS[dataset]:
            expected_for_spec = [_build_series(dataset, spec, w) for w in spec.regions if self._includes_region(w)]
            domain = spec.domain_override or BPS_ACEH_DOMAIN
            try:
                var_id = (
                    self.variables.get(spec.key)
                    or spec.fallback_var_id
                    or lookup_var_id(client, self.api_key, domain, spec, self._var_cache)
                )
            except RobotsDisallowedError as exc:
                combined.skipped_reason = format_skip_message(client.redact_secrets(str(exc)), ROBOTS_ACTION_HINT)
                combined.failed_series.extend(self.expected_series(dataset))
                return combined
            except CrawlerError as exc:
                combined.request_count += 1
                combined.failed_count += 1
                combined.warnings.append(client.redact_secrets(f"{spec.key}: {exc}"))
                combined.failed_series.extend(expected_for_spec)
                continue

            any_year_succeeded = False
            for year in range(start_year, end_year + 1):
                combined.request_count += 1
                th_id = year - BPS_BASE_YEAR
                url = build_bps_data_url(domain, var_id, th_id, self.api_key)
                safe_endpoint = strip_bps_key(url)
                try:
                    payload = client.get_json(url)
                    parsed = parse_bps_payload(payload, dataset, spec, safe_endpoint)
                except RobotsDisallowedError as exc:
                    combined.request_count -= 1
                    combined.skipped_reason = format_skip_message(client.redact_secrets(str(exc)), ROBOTS_ACTION_HINT)
                    combined.failed_series.extend(self.expected_series(dataset))
                    return combined
                except CrawlerError as exc:
                    combined.failed_count += 1
                    combined.warnings.append(client.redact_secrets(f"{spec.key} {year}: {exc}"))
                    continue

                any_year_succeeded = True
                combined.raw_captures.append(
                    RawBronzeCapture(
                        source_code=self.source_code,
                        dataset_name=dataset.name,
                        endpoint=safe_endpoint,
                        params={"domain": domain, "var_id": var_id, "th": th_id, "tahun": year},
                        payload=payload,
                        partition_year=year,
                        capture_date=capture_date,
                    )
                )
                combined.points.extend(p for p in parsed.points if self._includes_region(p.series.regency_code or ""))
                combined.rejected_counts.update(parsed.rejected_counts)

            if not any_year_succeeded:
                combined.failed_series.extend(expected_for_spec)

        return combined

    def fetch_daily(
        self,
        dataset: DatasetDefinition,
        client: HttpTransport,
        target_date: date,
    ) -> Iterator[FetchOutcome]:
        yield self._fetch_year_range(
            dataset,
            client,
            target_date.year - 1,
            target_date.year,
            capture_date=target_date,
        )

    def fetch_backfill(
        self,
        dataset: DatasetDefinition,
        client: HttpTransport,
        start_date: date,
        end_date: date,
    ) -> Iterator[FetchOutcome]:
        yield self._fetch_year_range(
            dataset,
            client,
            start_date.year,
            end_date.year,
            capture_date=end_date,
        )
