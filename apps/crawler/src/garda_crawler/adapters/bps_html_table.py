"""Universal BPS table parser: HTML <table> (rowspan/colspan) + Next.js RSC Type 1/2/3 payloads."""

from __future__ import annotations

import codecs
import html as html_lib
import json
import re
from dataclasses import dataclass, field
from datetime import date, datetime
from html.parser import HTMLParser
from typing import Any
from urllib.parse import parse_qs, urlsplit

from garda_crawler.config.bps_table_catalog import BpsTableSourceSpec
from garda_crawler.config.reference_catalog import (
    BPS_ACEH_PROVINCE_CODE,
    INDONESIAN_MONTHS,
    match_aceh_bps_region,
)
from garda_crawler.domain.errors import SchemaValidationError
from garda_crawler.validation.normalizers import normalize_alias

_NA_VALUES = {"", "-", "--", "---", "...", "n/a", "na", "null", "none"}
_YEAR_RE = re.compile(r"\b(19\d{2}|20\d{2})\b")


def parse_bps_numeric(raw: Any) -> float | None:
    """Normalize Indonesian decimal strings (`117,80`, `1.234,50`) to float; `-`/`...`/empty -> None."""
    if raw is None or isinstance(raw, bool):
        return None
    if isinstance(raw, (int, float)):
        return float(raw)

    text = html_lib.unescape(str(raw)).replace("\xa0", " ").strip()
    if text.lower() in _NA_VALUES:
        return None

    cleaned = re.sub(r"[^\d,.\-]", "", text)
    if not cleaned or cleaned in {"-", ".", ","}:
        return None

    if "," in cleaned and "." in cleaned:
        if cleaned.rfind(",") > cleaned.rfind("."):
            cleaned = cleaned.replace(".", "").replace(",", ".")
        else:
            cleaned = cleaned.replace(",", "")
    elif "," in cleaned:
        parts = cleaned.split(",")
        if len(parts) > 2 and all(len(p) == 3 and p.isdigit() for p in parts[1:]):
            cleaned = "".join(parts)
        else:
            cleaned = cleaned.replace(",", ".")

    try:
        return float(cleaned)
    except ValueError:
        return None


@dataclass
class _HtmlCell:
    text: str
    rowspan: int = 1
    colspan: int = 1
    is_header: bool = False


class _TableDOMParser(HTMLParser):
    """Minimal zero-dependency parser extracting all <table> elements with rows and spans."""

    def __init__(self) -> None:
        super().__init__(convert_charrefs=True)
        self.tables: list[dict[str, list[list[_HtmlCell]]]] = []
        self._in_table = False
        self._table_depth = 0
        self._section = "body"
        self._current_table: dict[str, list[list[_HtmlCell]]] | None = None
        self._current_row: list[_HtmlCell] | None = None
        self._in_cell = False
        self._cell_parts: list[str] = []
        self._cell_span = (1, 1)
        self._cell_is_header = False

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        tag = tag.lower()
        attr_map = dict(attrs)
        if tag == "table":
            if not self._in_table:
                self._in_table = True
                self._current_table = {"head": [], "body": []}
                self._section = "body"
            self._table_depth += 1
            return
        if not self._in_table or self._table_depth > 1:
            return
        if tag == "thead":
            self._section = "head"
        elif tag in ("tbody", "tfoot"):
            self._section = "body"
        elif tag == "tr":
            self._current_row = []
        elif tag in ("th", "td") and self._current_row is not None:
            self._in_cell = True
            self._cell_parts = []
            self._cell_is_header = tag == "th" or self._section == "head"
            try:
                rs = max(1, int(attr_map.get("rowspan") or 1))
            except ValueError:
                rs = 1
            try:
                cs = max(1, int(attr_map.get("colspan") or 1))
            except ValueError:
                cs = 1
            self._cell_span = (rs, cs)
        elif tag == "br" and self._in_cell:
            self._cell_parts.append(" ")

    def handle_data(self, data: str) -> None:
        if self._in_cell:
            self._cell_parts.append(data)

    def handle_endtag(self, tag: str) -> None:
        tag = tag.lower()
        if tag == "table" and self._in_table:
            self._table_depth -= 1
            if self._table_depth == 0 and self._current_table is not None:
                self.tables.append(self._current_table)
                self._current_table = None
                self._in_table = False
            return
        if not self._in_table or self._table_depth > 1:
            return
        if tag in ("th", "td") and self._in_cell and self._current_row is not None:
            text = re.sub(r"\s+", " ", "".join(self._cell_parts)).strip()
            rs, cs = self._cell_span
            self._current_row.append(_HtmlCell(text=text, rowspan=rs, colspan=cs, is_header=self._cell_is_header))
            self._in_cell = False
        elif tag == "tr" and self._current_row is not None and self._current_table is not None:
            if self._current_row:
                target = "head" if self._section == "head" else "body"
                self._current_table[target].append(self._current_row)
            self._current_row = None
        elif tag == "thead":
            self._section = "body"


def _expand_rows_with_spans(rows: list[list[_HtmlCell]]) -> list[list[str]]:
    """Expand `rowspan` and `colspan` cells into a rectangular 2D string grid."""
    grid: list[list[str | None]] = []
    for r_idx, row in enumerate(rows):
        while len(grid) <= r_idx:
            grid.append([])
        c_idx = 0
        for cell in row:
            while c_idx < len(grid[r_idx]) and grid[r_idx][c_idx] is not None:
                c_idx += 1
            for dr in range(cell.rowspan):
                while len(grid) <= r_idx + dr:
                    grid.append([])
                target_row = grid[r_idx + dr]
                while len(target_row) < c_idx + cell.colspan:
                    target_row.append(None)
                for dc in range(cell.colspan):
                    target_row[c_idx + dc] = cell.text
            c_idx += cell.colspan

    max_cols = max((len(r) for r in grid), default=0)
    return [[(r[i] if i < len(r) and r[i] is not None else "") for i in range(max_cols)] for r in grid]


@dataclass(frozen=True)
class BpsExtractedRecord:
    """Normalized record emitted by any BPS table parser with full Aceh region & temporal context."""

    judul_tabel: str
    instansi_sumber: str
    url_sumber: str
    kategori_data: str
    waktu_crawl: str
    wilayah: str
    kode_bps: str
    tahun: int | None
    periode: str
    kolom: str
    sub_kelompok: str
    nilai: float | None
    komoditi: str = ""
    extras: dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        return {
            "judul_tabel": self.judul_tabel,
            "instansi_sumber": self.instansi_sumber,
            "url_sumber": self.url_sumber,
            "kategori_data": self.kategori_data,
            "waktu_crawl": self.waktu_crawl,
            "wilayah": self.wilayah,
            "kode_bps": self.kode_bps,
            "tahun": self.tahun,
            "periode": self.periode,
            "kolom": self.kolom,
            "sub_kelompok": self.sub_kelompok,
            "komoditi": self.komoditi or None,
            "nilai": self.nilai,
            **self.extras,
        }


def _extract_year_and_period(header_levels: list[str], fallback_year: int | None) -> tuple[int | None, str]:
    joined = " | ".join(h for h in header_levels if h)
    year_match = _YEAR_RE.search(joined)
    tahun = int(year_match.group(1)) if year_match else fallback_year
    periode = header_levels[-1] if header_levels else ""
    for level in reversed(header_levels):
        if normalize_alias(level) in INDONESIAN_MONTHS or "tahunan" in level.lower():
            periode = level
            break
    return tahun, periode


def parse_html_table_records(
    html_text: str,
    spec: BpsTableSourceSpec,
    crawl_timestamp_iso: str,
    *,
    fallback_year: int | None = None,
) -> tuple[list[BpsExtractedRecord], int]:
    """Parse HTML <table> multi-level headers (`rowspan`/`colspan`) and apply Aceh filter."""
    parser = _TableDOMParser()
    parser.feed(html_text)
    parser.close()

    if not parser.tables:
        raise SchemaValidationError(f"elemen <table> tidak ditemukan pada {spec.url}")

    # Pilih tabel terbesar berdasarkan total baris
    best = max(parser.tables, key=lambda t: len(t["head"]) + len(t["body"]))
    head_rows = list(best["head"])
    body_rows = list(best["body"])

    if not head_rows and body_rows:
        # Banyak tabel Excel export BPS (seperti URL #70) menaruh header di <tbody> tanpa <thead>.
        full_grid = _expand_rows_with_spans(body_rows)
        split_idx = 0
        for idx, row_vals in enumerate(full_grid[:12]):
            raw_row = body_rows[idx]
            has_span = any(c.rowspan > 1 or c.colspan > 1 or c.is_header for c in raw_row)
            label0 = (row_vals[0] or "").strip().lower()
            is_label_header = label0.startswith(("komoditi", "provinsi", "kabupaten", "wilayah", "uraian", "rata-rata"))
            single_nums = sum(
                1 for cell in row_vals[1:12]
                if cell.strip()
                and "-" not in cell.strip().lstrip("-")
                and "<" not in cell
                and ">" not in cell
                and parse_bps_numeric(cell) is not None
                and not _YEAR_RE.fullmatch(cell.strip())
            )
            if has_span or is_label_header or single_nums == 0:
                split_idx = idx + 1
            else:
                break
        if split_idx == 0:
            split_idx = 1
        head_grid = full_grid[:split_idx]
        body_grid = full_grid[split_idx:]
    else:
        head_grid = _expand_rows_with_spans(head_rows)
        body_grid = _expand_rows_with_spans(body_rows)
    if not body_grid or not head_grid:
        return [], 0

    n_cols = max(len(head_grid[0]), max((len(r) for r in body_grid), default=0))
    col_headers: list[list[str]] = []
    for c_idx in range(n_cols):
        levels: list[str] = []
        for h_row in head_grid:
            val = h_row[c_idx].strip() if c_idx < len(h_row) else ""
            if val and (not levels or levels[-1] != val):
                levels.append(val)
        col_headers.append(levels)

    total_body_rows = len(body_grid)
    records: list[BpsExtractedRecord] = []

    for row in body_grid:
        first_cell = row[0].strip() if row else ""
        if not first_cell or normalize_alias(first_cell) in {"jumlah", "total", "catatan", "sumber"}:
            continue

        if spec.aceh_specific_rows:
            kode_bps, wilayah_name, komoditi = BPS_ACEH_PROVINCE_CODE, "Aceh", first_cell
        else:
            matched = match_aceh_bps_region(first_cell)
            if matched is None:
                continue
            kode_bps, wilayah_name = matched
            komoditi = ""

        for c_idx in range(1, n_cols):
            raw_cell = row[c_idx] if c_idx < len(row) else ""
            headers = col_headers[c_idx] if c_idx < len(col_headers) else []
            kolom = " > ".join(headers) if headers else f"kolom_{c_idx}"
            tahun, periode = _extract_year_and_period(headers, fallback_year)
            nilai = parse_bps_numeric(raw_cell)

            records.append(
                BpsExtractedRecord(
                    judul_tabel=spec.title,
                    instansi_sumber=spec.institution,
                    url_sumber=spec.url,
                    kategori_data=spec.category,
                    waktu_crawl=crawl_timestamp_iso,
                    wilayah=wilayah_name,
                    kode_bps=kode_bps,
                    tahun=tahun,
                    periode=periode,
                    kolom=kolom,
                    sub_kelompok=headers[0] if headers else "",
                    nilai=nilai,
                    komoditi=komoditi,
                    extras={"provinsi": "Aceh"} if spec.aceh_specific_rows else {},
                )
            )
    return records, total_body_rows


def _extract_balanced_pair(text: str, start_idx: int, open_ch: str, close_ch: str) -> str | None:
    depth = 0
    in_str = False
    esc = False
    for i in range(start_idx, len(text)):
        ch = text[i]
        if in_str:
            if esc:
                esc = False
            elif ch == "\\":
                esc = True
            elif ch == '"':
                in_str = False
        else:
            if ch == '"':
                in_str = True
            elif ch == open_ch:
                depth += 1
            elif ch == close_ch:
                depth -= 1
                if depth == 0:
                    return text[start_idx : i + 1]
    return None


def _extract_balanced_json(text: str, start_idx: int) -> str | None:
    return _extract_balanced_pair(text, start_idx, "{", "}")


def extract_rsc_table_payload(html_text: str, spec: BpsTableSourceSpec) -> tuple[str, Any]:
    """Locate embedded Type-1 HTML <table>, Type-2 JSON (`datacontent`), or Type-3 regional JSON."""
    if "<table" in html_text.lower() and spec.table_type == 1:
        return "html_table", html_text

    # Cek format JSON Tipe 2 (`"status":"OK"... "datacontent":{...}`)
    m_type2 = re.search(r'\{\\+"status\\+":\\+"OK\\+".*?\\+"datacontent\\+":', html_text)
    if m_type2:
        raw_obj = html_text[m_type2.start() :]
        clean = raw_obj.replace('\\"', '"').replace("\\\\", "\\")
        balanced = _extract_balanced_json(clean, 0)
        if balanced:
            return "bps_type2_json", json.loads(balanced)

    # Cek format JSON Tipe 3 (`{"columns":{...},"data":[{"label":...,"variables":...}]}` atau `"kolom"/"nama_variabel"`)
    m_type3_data = re.search(r'\\+"data\\+":(\[\{\\+"label\\+":)', html_text)
    if m_type3_data:
        arr_slice = html_text[m_type3_data.start(1) :].replace('\\"', '"').replace("\\\\", "\\")
        arr_json = _extract_balanced_pair(arr_slice, 0, "[", "]")
        if arr_json:
            rows = json.loads(arr_json)
            prefix = html_text[max(0, m_type3_data.start() - 65536) : m_type3_data.start()].replace('\\"', '"').replace("\\\\", "\\")
            columns_meta: dict[str, dict[str, Any]] = {}
            for km in re.finditer(r'"([A-Za-z0-9_]+)"\s*:\s*\{\s*"nama_variabel"\s*:\s*"([^"]+)"', prefix):
                columns_meta[km.group(1)] = {"nama_variabel": km.group(2)}
            if rows and isinstance(rows[0], dict) and "variables" in rows[0]:
                return "bps_type3_json", {"columns": columns_meta, "data": rows}

    # Cek format HTML <table> ter-escape di Type 1 (`\u003ctable ... \u003c/table\u003e`)
    t_start = html_text.lower().find("u003ctable")
    if t_start != -1:
        t_start = max(0, t_start - 1)  # sertakan backslash sebelum u003c
        t_end_rel = html_text.lower().find("u003c/table\\u003e", t_start)
        if t_end_rel == -1:
            t_end_rel = html_text.lower().find("u003c/table", t_start)
        end_idx = (t_end_rel + 18) if t_end_rel != -1 else min(len(html_text), t_start + 15_000_000)
        raw_slice = html_text[t_start:end_idx]
        decoded = raw_slice.replace("\\u003c", "<").replace("\\u003e", ">").replace("\\u0026", "&")
        decoded = decoded.replace('\\"', '"').replace("\\r\\n", "\n").replace("\\n", "\n")
        return "html_table", decoded

    if "<table" in html_text.lower():
        return "html_table", html_text

    raise SchemaValidationError(f"payload tabel BPS tidak ditemukan pada {spec.url}")


def parse_type2_bps_json(
    payload: dict[str, Any],
    spec: BpsTableSourceSpec,
    crawl_timestamp_iso: str,
) -> tuple[list[BpsExtractedRecord], int]:
    """Parse BPS Type 2 JSON iterating ALL `turvar`, `tahun`, `turtahun`, and Aceh `vervar` rows."""
    variables = payload.get("var") or []
    if not variables:
        return [], 0
    var_val = variables[0]["val"]
    turvars = payload.get("turvar") or [{"val": 0, "label": "Tidak ada"}]
    vervars = payload.get("vervar") or []
    tahuns = payload.get("tahun") or []
    turtahuns = payload.get("turtahun") or [{"val": 0, "label": "Tahunan"}]
    datacontent = payload.get("datacontent") or {}

    records: list[BpsExtractedRecord] = []
    for vv in vervars:
        matched = match_aceh_bps_region(str(vv.get("label", "")))
        if matched is None:
            continue
        kode_bps, wilayah_name = matched
        for tv in turvars:
            sub_label = str(tv.get("label", "")).strip()
            for th in tahuns:
                try:
                    year = int(str(th.get("label", "")).strip())
                except ValueError:
                    year = None
                for tt in turtahuns:
                    tt_label = str(tt.get("label", "")).strip()
                    cell_key = f"{vv['val']}{var_val}{tv['val']}{th['val']}{tt['val']}"
                    raw_val = datacontent.get(cell_key)
                    nilai = parse_bps_numeric(raw_val)
                    kolom = f"{sub_label} > {year} > {tt_label}" if sub_label.lower() != "tidak ada" else f"{year} > {tt_label}"
                    records.append(
                        BpsExtractedRecord(
                            judul_tabel=spec.title,
                            instansi_sumber=spec.institution,
                            url_sumber=spec.url,
                            kategori_data=spec.category,
                            waktu_crawl=crawl_timestamp_iso,
                            wilayah=wilayah_name,
                            kode_bps=kode_bps,
                            tahun=year,
                            periode=tt_label,
                            kolom=kolom,
                            sub_kelompok=sub_label,
                            nilai=nilai,
                        )
                    )
    return records, len(vervars)


def parse_type3_bps_json(
    payload: dict[str, Any],
    spec: BpsTableSourceSpec,
    crawl_timestamp_iso: str,
) -> tuple[list[BpsExtractedRecord], int]:
    """Parse BPS Type 3 regional JSON (`columns` + `data` with `kode_wilayah` & `variables`)."""
    qs = parse_qs(urlsplit(spec.url).query)
    fallback_year = int(qs["year"][0]) if "year" in qs and qs["year"][0].isdigit() else None
    columns_meta = payload.get("columns") or {}
    rows = payload.get("data") or []

    records: list[BpsExtractedRecord] = []
    for row in rows:
        if not isinstance(row, dict):
            continue
        matched = match_aceh_bps_region(str(row.get("label", "")), row.get("kode_wilayah"))
        if matched is None:
            continue
        kode_bps, wilayah_name = matched
        variables = row.get("variables") or {}
        for var_key, cell in variables.items():
            meta = columns_meta.get(var_key) or {}
            col_name = str(meta.get("nama_variabel") or var_key).strip()
            tahun, periode = _extract_year_and_period([col_name], fallback_year)
            raw_val = cell.get("value") if isinstance(cell, dict) else cell
            nilai = parse_bps_numeric(raw_val)
            records.append(
                BpsExtractedRecord(
                    judul_tabel=spec.title,
                    instansi_sumber=spec.institution,
                    url_sumber=spec.url,
                    kategori_data=spec.category,
                    waktu_crawl=crawl_timestamp_iso,
                    wilayah=wilayah_name,
                    kode_bps=kode_bps,
                    tahun=tahun,
                    periode=periode or str(tahun or ""),
                    kolom=col_name,
                    sub_kelompok=col_name,
                    nilai=nilai,
                )
            )
    return records, len(rows)


def extract_bps_table_from_response(
    html_text: str,
    spec: BpsTableSourceSpec,
    crawl_timestamp_iso: str,
) -> tuple[str, Any, list[BpsExtractedRecord], int]:
    """Auto-detect BPS Type 1/2/3 payload or HTML <table>, extract records, and filter for Aceh."""
    qs = parse_qs(urlsplit(spec.url).query)
    fallback_year = int(qs["year"][0]) if "year" in qs and qs["year"][0].isdigit() else None

    fmt, payload = extract_rsc_table_payload(html_text, spec)
    if fmt == "bps_type2_json":
        records, total_rows = parse_type2_bps_json(payload, spec, crawl_timestamp_iso)
    elif fmt == "bps_type3_json":
        records, total_rows = parse_type3_bps_json(payload, spec, crawl_timestamp_iso)
    else:
        records, total_rows = parse_html_table_records(
            payload, spec, crawl_timestamp_iso, fallback_year=fallback_year
        )
    return fmt, payload, records, total_rows
