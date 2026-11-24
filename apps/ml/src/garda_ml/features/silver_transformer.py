"""Medallion Silver transformer: wide-to-long unpivoting, Indonesian decimals, `dim_wilayah` -> `kode_bps`."""

from __future__ import annotations

import gzip
import json
import math
import re
from collections.abc import Iterable
from dataclasses import dataclass, field
from datetime import date
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd

from garda_ml.features.pandera_schemas import (
    validate_silver_macro_df,
    validate_silver_price_df,
    validate_silver_weather_df,
)

_NA_TOKENS = {"", "-", "--", "---", "n/a", "na", "null", "none", "nan"}
_MONTH_MAP = {
    "januari": 1, "jan": 1,
    "februari": 2, "feb": 2,
    "maret": 3, "mar": 3,
    "april": 4, "apr": 4,
    "mei": 5,
    "juni": 6, "jun": 6,
    "juli": 7, "jul": 7,
    "agustus": 8, "agu": 8, "agt": 8,
    "september": 9, "sep": 9,
    "oktober": 10, "okt": 10,
    "november": 11, "nov": 11,
    "desember": 12, "des": 12,
}

# Built-in canonical `dim_wilayah` seed matching db/migrations/000134_medallion_data_pipeline.sql.
_DEFAULT_DIM_WILAYAH_ROWS: tuple[dict[str, Any], ...] = (
    {"kode_bps": "0000", "nama_resmi": "Indonesia", "kode_djpk_prov": "00", "kode_djpk_pemda": "00", "kode_pihps": None},
    {"kode_bps": "11",   "nama_resmi": "Provinsi Aceh", "kode_djpk_prov": "01", "kode_djpk_pemda": "00", "kode_pihps": None},
    {"kode_bps": "1101", "nama_resmi": "Kab. Simeulue", "kode_djpk_prov": "01", "kode_djpk_pemda": "12", "kode_pihps": 491},
    {"kode_bps": "1102", "nama_resmi": "Kab. Aceh Singkil", "kode_djpk_prov": "01", "kode_djpk_pemda": "10", "kode_pihps": 493},
    {"kode_bps": "1103", "nama_resmi": "Kab. Aceh Selatan", "kode_djpk_prov": "01", "kode_djpk_pemda": "02", "kode_pihps": 494},
    {"kode_bps": "1104", "nama_resmi": "Kab. Aceh Tenggara", "kode_djpk_prov": "01", "kode_djpk_pemda": "03", "kode_pihps": 478},
    {"kode_bps": "1105", "nama_resmi": "Kab. Aceh Timur", "kode_djpk_prov": "01", "kode_djpk_pemda": "04", "kode_pihps": 479},
    {"kode_bps": "1106", "nama_resmi": "Kab. Aceh Tengah", "kode_djpk_prov": "01", "kode_djpk_pemda": "05", "kode_pihps": 484},
    {"kode_bps": "1107", "nama_resmi": "Kab. Aceh Barat", "kode_djpk_prov": "01", "kode_djpk_pemda": "01", "kode_pihps": 3},
    {"kode_bps": "1108", "nama_resmi": "Kab. Aceh Besar", "kode_djpk_prov": "01", "kode_djpk_pemda": "06", "kode_pihps": 495},
    {"kode_bps": "1109", "nama_resmi": "Kab. Pidie", "kode_djpk_prov": "01", "kode_djpk_pemda": "07", "kode_pihps": 489},
    {"kode_bps": "1110", "nama_resmi": "Kab. Bireuen", "kode_djpk_prov": "01", "kode_djpk_pemda": "11", "kode_pihps": 483},
    {"kode_bps": "1111", "nama_resmi": "Kab. Aceh Utara", "kode_djpk_prov": "01", "kode_djpk_pemda": "08", "kode_pihps": 485},
    {"kode_bps": "1112", "nama_resmi": "Kab. Aceh Barat Daya", "kode_djpk_prov": "01", "kode_djpk_pemda": "13", "kode_pihps": 477},
    {"kode_bps": "1113", "nama_resmi": "Kab. Gayo Lues", "kode_djpk_prov": "01", "kode_djpk_pemda": "14", "kode_pihps": 492},
    {"kode_bps": "1114", "nama_resmi": "Kab. Aceh Tamiang", "kode_djpk_prov": "01", "kode_djpk_pemda": "17", "kode_pihps": 488},
    {"kode_bps": "1115", "nama_resmi": "Kab. Nagan Raya", "kode_djpk_prov": "01", "kode_djpk_pemda": "16", "kode_pihps": 487},
    {"kode_bps": "1116", "nama_resmi": "Kab. Aceh Jaya", "kode_djpk_prov": "01", "kode_djpk_pemda": "15", "kode_pihps": 482},
    {"kode_bps": "1117", "nama_resmi": "Kab. Bener Meriah", "kode_djpk_prov": "01", "kode_djpk_pemda": "18", "kode_pihps": 476},
    {"kode_bps": "1118", "nama_resmi": "Kab. Pidie Jaya", "kode_djpk_prov": "01", "kode_djpk_pemda": "19", "kode_pihps": 481},
    {"kode_bps": "1171", "nama_resmi": "Kota Banda Aceh", "kode_djpk_prov": "01", "kode_djpk_pemda": "09", "kode_pihps": 1},
    {"kode_bps": "1172", "nama_resmi": "Kota Sabang", "kode_djpk_prov": "01", "kode_djpk_pemda": "51", "kode_pihps": 490},
    {"kode_bps": "1173", "nama_resmi": "Kota Langsa", "kode_djpk_prov": "01", "kode_djpk_pemda": "53", "kode_pihps": 480},
    {"kode_bps": "1174", "nama_resmi": "Kota Lhokseumawe", "kode_djpk_prov": "01", "kode_djpk_pemda": "52", "kode_pihps": 2},
    {"kode_bps": "1175", "nama_resmi": "Kota Subulussalam", "kode_djpk_prov": "01", "kode_djpk_pemda": "54", "kode_pihps": 486},
)


def parse_indonesian_decimal(raw: Any) -> float:
    """Convert Indonesian comma-decimal/grouped numbers to float64 and `-`/empty markers to NaN."""
    if raw is None or isinstance(raw, bool):
        return float("nan")
    if isinstance(raw, (int, float, np.number)):
        val = float(raw)
        return float("nan") if math.isnan(val) or math.isinf(val) else val

    text = str(raw).strip()
    if text.lower() in _NA_TOKENS:
        return float("nan")

    cleaned = re.sub(r"[^\d,.\-]", "", text)
    if not cleaned or cleaned in {"-", ".", ","}:
        return float("nan")

    if "," in cleaned and "." in cleaned:
        # Format Indonesia: "15.850,25" vs format US: "15,850.25"
        if cleaned.rfind(",") > cleaned.rfind("."):
            cleaned = cleaned.replace(".", "").replace(",", ".")
        else:
            cleaned = cleaned.replace(",", "")
    elif "," in cleaned:
        parts = cleaned.split(",")
        # Anggap pemisah ribuan bila pola "^-?\d{1,3}(,\d{3})+$" dan tidak ada desimal 1-2 digit
        if len(parts[-1]) == 3 and all(p.isdigit() for p in parts if p) and len(parts) > 2:
            cleaned = "".join(parts)
        elif len(parts) == 2 and len(parts[1]) == 3 and len(parts[0].lstrip("-")) <= 3 and float(parts[0]) >= 1:
            # Kasus umum harga PIHPS: "15,850" -> 15850; indeks IHK umumnya 2 desimal mis. "105,42"
            cleaned = "".join(parts) if len(parts[0].lstrip("-")) >= 2 else f"{parts[0]}.{parts[1]}"
        else:
            cleaned = cleaned.replace(",", ".")

    try:
        val = float(cleaned)
        return float("nan") if math.isnan(val) or math.isinf(val) else val
    except ValueError:
        return float("nan")


def _norm_label(text: str) -> str:
    return re.sub(r"\s+", " ", text.strip().lower())


class DimWilayahMapper:
    """Cross-source region resolver mapping BPS, DJPK (`provinsi`, `pemda`), and PIHPS to `kode_bps`."""

    def __init__(self, rows: Iterable[dict[str, Any]] | None = None) -> None:
        source_rows = tuple(rows) if rows is not None else _DEFAULT_DIM_WILAYAH_ROWS
        self._known_bps: set[str] = set()
        self._by_label: dict[str, str] = {"aceh": "11", "provinsi aceh": "11", "meulaboh": "1107", "indonesia": "0000"}
        self._by_djpk: dict[tuple[str, str], str] = {}
        self._by_pihps: dict[int, str] = {}

        for row in source_rows:
            kode_bps = str(row["kode_bps"]).strip()
            self._known_bps.add(kode_bps)
            label = _norm_label(str(row.get("nama_resmi", "")))
            if label:
                self._by_label[label] = kode_bps
                self._by_label[label.replace("kab. ", "kab ")] = kode_bps
                self._by_label[re.sub(r"^(kab|kota)\.?\s+", "", label)] = kode_bps
            dprov, dpemda = row.get("kode_djpk_prov"), row.get("kode_djpk_pemda")
            if dprov and dpemda:
                self._by_djpk[(str(dprov).strip(), str(dpemda).strip())] = kode_bps
            if row.get("kode_pihps") is not None:
                self._by_pihps[int(row["kode_pihps"])] = kode_bps

    @classmethod
    def from_postgres(cls, dsn: str) -> "DimWilayahMapper":
        """Load `dim_wilayah` reference rows from PostgreSQL when DSN is supplied."""
        if not dsn:
            return cls()
        import psycopg

        with psycopg.connect(dsn, connect_timeout=5) as conn:
            with conn.cursor() as cur:
                cur.execute(
                    "SELECT kode_bps, nama_resmi, kode_djpk_prov, kode_djpk_pemda, kode_pihps FROM dim_wilayah"
                )
                rows = [
                    {
                        "kode_bps": r[0],
                        "nama_resmi": r[1],
                        "kode_djpk_prov": r[2],
                        "kode_djpk_pemda": r[3],
                        "kode_pihps": r[4],
                    }
                    for r in cur.fetchall()
                ]
        return cls(rows or None)

    def resolve_bps(self, code_or_label: Any) -> str | None:
        raw = str(code_or_label or "").strip()
        if raw in self._known_bps:
            return raw
        return self._by_label.get(_norm_label(raw))

    def resolve_djpk(self, provinsi: Any, pemda: Any) -> str | None:
        key = (str(provinsi or "").strip().zfill(2), str(pemda or "").strip().zfill(2))
        return self._by_djpk.get(key)

    def resolve_pihps(self, regency_id: Any) -> str | None:
        try:
            return self._by_pihps.get(int(regency_id))
        except (TypeError, ValueError):
            return None


def read_bronze_json_gz(path_or_bytes: Path | bytes) -> Any:
    """Decompress and decode one immutable `.json.gz` Bronze capture."""
    raw_bytes = path_or_bytes.read_bytes() if isinstance(path_or_bytes, Path) else path_or_bytes
    return json.loads(gzip.decompress(raw_bytes).decode("utf-8"))


def unpivot_wide_table(
    df: pd.DataFrame,
    *,
    id_vars: list[str],
    value_cols: list[str] | None = None,
    var_name: str = "periode_raw",
    value_name: str = "nilai",
) -> pd.DataFrame:
    """Melt wide multi-period table into tidy long format with Indonesian float64 parsing."""
    if df.empty:
        return pd.DataFrame(columns=[*id_vars, var_name, value_name])
    cols = value_cols if value_cols is not None else [c for c in df.columns if c not in id_vars]
    melted = df.melt(id_vars=id_vars, value_vars=cols, var_name=var_name, value_name=value_name)
    melted[value_name] = melted[value_name].map(parse_indonesian_decimal).astype("float64")
    return melted


def transform_bps_bronze_to_silver(
    payload: dict[str, Any],
    *,
    mapper: DimWilayahMapper,
    domain: str = "INFLASI",
    indicator_fallback: str = "ihk_150",
) -> pd.DataFrame:
    """Normalize raw BPS WebAPI/IHK150 JSON into a Pandera-validated Silver long DataFrame."""
    variables = payload.get("var") or []
    turvar = payload.get("turvar") or [{"val": 0, "label": "Tidak ada"}]
    datacontent = payload.get("datacontent") or {}
    if not variables or not datacontent:
        return pd.DataFrame(columns=["kode_bps", "domain", "indikator", "periode", "nilai", "sumber"])

    var_val = variables[0]["val"]
    indikator = str(variables[0].get("label") or indicator_fallback).strip().lower().replace(" ", "_")[:64]
    tur_val = turvar[0]["val"]

    rows: list[dict[str, Any]] = []
    for th in payload.get("tahun") or []:
        try:
            year = int(str(th.get("label", "")).strip())
        except ValueError:
            continue
        for tt in payload.get("turtahun") or []:
            month = _MONTH_MAP.get(_norm_label(str(tt.get("label", ""))))
            if month is None:
                continue
            period = pd.Timestamp(date(year, month, 1))
            for vv in payload.get("vervar") or []:
                kode_bps = mapper.resolve_bps(vv.get("label"))
                if kode_bps is None:
                    continue
                cell_key = f"{vv['val']}{var_val}{tur_val}{th['val']}{tt['val']}"
                if cell_key not in datacontent:
                    continue
                val = parse_indonesian_decimal(datacontent[cell_key])
                rows.append(
                    {
                        "kode_bps": kode_bps,
                        "domain": domain,
                        "indikator": indikator,
                        "periode": period,
                        "nilai": val,
                        "sumber": "BPS",
                    }
                )

    df = pd.DataFrame(rows)
    return validate_silver_macro_df(df) if not df.empty else df


def transform_djpk_bronze_to_silver(
    payload: dict[str, Any] | list[Any],
    *,
    provinsi: str,
    pemda: str,
    tahun: int,
    mapper: DimWilayahMapper,
) -> pd.DataFrame:
    """Normalize raw DJPK APBD JSON into Silver macro format keyed by canonical `kode_bps`."""
    kode_bps = mapper.resolve_djpk(provinsi, pemda)
    if kode_bps is None:
        return pd.DataFrame(columns=["kode_bps", "domain", "indikator", "periode", "nilai", "sumber"])

    raw_rows = payload if isinstance(payload, list) else (payload.get("data") or payload.get("rows") or [])
    rows: list[dict[str, Any]] = []
    period = pd.Timestamp(date(tahun, 1, 1))
    for item in raw_rows:
        if not isinstance(item, dict):
            continue
        label = str(item.get("uraian") or item.get("akun") or "apbd_total").strip()
        slug = re.sub(r"[^a-z0-9]+", "_", label.lower()).strip("_")[:48] or "apbd"
        raw_val = item.get("realisasi") if item.get("realisasi") is not None else item.get("anggaran")
        rows.append(
            {
                "kode_bps": kode_bps,
                "domain": "FISKAL_DJPK",
                "indikator": f"apbd_{slug}",
                "periode": period,
                "nilai": parse_indonesian_decimal(raw_val),
                "sumber": "DJPK",
            }
        )
    df = pd.DataFrame(rows)
    return validate_silver_macro_df(df) if not df.empty else df


_PIHPS_COMMODITIES: dict[str, tuple[str, str]] = {
    "beras": ("BRS-MED", "Rp/kg"),
    "beras kualitas bawah i": ("BRS-BWH1", "Rp/kg"),
    "beras kualitas bawah ii": ("BRS-BWH2", "Rp/kg"),
    "beras kualitas medium i": ("BRS-MED", "Rp/kg"),
    "beras kualitas medium ii": ("BRS-MED2", "Rp/kg"),
    "beras kualitas super i": ("BRS-PRE", "Rp/kg"),
    "beras kualitas super ii": ("BRS-PRE2", "Rp/kg"),
    "bawang merah": ("BWM", "Rp/kg"),
    "bawang merah ukuran sedang": ("BWM", "Rp/kg"),
    "bawang putih": ("BWP", "Rp/kg"),
    "bawang putih ukuran sedang": ("BWP", "Rp/kg"),
    "cabai merah keriting": ("CBM-KRT", "Rp/kg"),
    "cabai merah besar": ("CBM-BSR", "Rp/kg"),
    "cabai rawit merah": ("CBR-MRH", "Rp/kg"),
    "cabai rawit hijau": ("CBR-HJU", "Rp/kg"),
    "daging sapi kualitas 1": ("DGG-SPI", "Rp/kg"),
    "daging sapi kualitas 2": ("DGG-SPI2", "Rp/kg"),
    "daging sapi murni": ("DGG-SPI", "Rp/kg"),
    "daging ayam ras segar": ("DGG-AYM", "Rp/kg"),
    "daging ayam": ("DGG-AYM", "Rp/kg"),
    "telur ayam ras segar": ("TLR-AYM", "Rp/kg"),
    "telur ayam": ("TLR-AYM", "Rp/kg"),
    "gula pasir lokal": ("GLA", "Rp/kg"),
    "gula pasir kualitas premium": ("GLA-PRE", "Rp/kg"),
    "gula konsumsi": ("GLA", "Rp/kg"),
    "minyak goreng kemasan bermerek 1": ("MYK", "Rp/liter"),
    "minyak goreng kemasan bermerek 2": ("MYK-2", "Rp/liter"),
    "minyak goreng curah": ("MYK-CRH", "Rp/liter"),
    "minyak goreng kemasan sederhana": ("MYK", "Rp/liter"),
}


def _slug(text: str, limit: int = 64) -> str:
    return re.sub(r"[^a-z0-9]+", "_", _norm_label(text)).strip("_")[:limit] or "nilai"


def _map_bps_category_to_domain(category: str, text: str) -> str:
    cat = category.lower()
    if cat.startswith(("ihk", "inflasi")):
        return "INFLASI"
    if "tenaga_kerja" in cat or re.search(r"tpt|tpak|upah|ump|umk|pengangguran", text):
        return "TENAGA_KERJA"
    if "konsumsi" in cat or "pengeluaran" in cat:
        return "KONSUMSI_PENDAPATAN"
    if re.search(r"ikan|perikanan", text):
        return "PERIKANAN"
    if re.search(r"hutan|kayu", text):
        return "KEHUTANAN"
    if re.search(r"padi|beras|panen|produksi|luas", text):
        return "PERTANIAN"
    return "NERACA_EKONOMI"


def transform_bps_tables_bronze_to_silver(
    payload: dict[str, Any],
    *,
    mapper: DimWilayahMapper,
    dataset_code: str = "",
) -> pd.DataFrame:
    """Normalize 70-table BPS Bronze JSON (`metadata` + `records`) into Pandera-validated Silver."""
    records = payload.get("records") if isinstance(payload, dict) else None
    if not records:
        return pd.DataFrame(
            columns=["kode_bps", "domain", "indikator", "periode", "nilai", "sumber", "frekuensi", "satuan"]
        )

    meta = payload.get("metadata") or {}
    category = str(meta.get("kategori_data") or "").strip().lower()
    rows: list[dict[str, Any]] = []
    for rec in records:
        if not isinstance(rec, dict):
            continue
        kode_bps = mapper.resolve_bps(rec.get("kode_bps") or rec.get("wilayah"))
        if kode_bps is None:
            continue
        val = parse_indonesian_decimal(rec.get("nilai"))
        if math.isnan(val):
            continue

        tahun_raw = rec.get("tahun")
        try:
            year = int(tahun_raw) if tahun_raw is not None else 2025
        except (TypeError, ValueError):
            year = 2025
        periode_str = str(rec.get("periode") or "").strip()
        month = _MONTH_MAP.get(_norm_label(periode_str))
        if month is not None:
            period = pd.Timestamp(date(year, month, 1))
            freq = "BULANAN"
        else:
            period = pd.Timestamp(date(year, 1, 1))
            freq = "TAHUNAN"

        sub_kel = str(rec.get("sub_kelompok") or "").strip()
        komoditi = str(rec.get("komoditi") or "").strip()
        ds_slug = _slug(dataset_code or category or str(meta.get("judul_tabel") or "bps"), 40)

        # Preserve recognizable tags (`indeks_harga_konsumen`, `yoy`, `mtm`, `ytd`) for calibration & macro features.
        if category == "ihk" or ds_slug.startswith("ihk"):
            ind_prefix = f"indeks_harga_konsumen_{ds_slug}"
        elif "yoy" in category or "yoy" in ds_slug:
            ind_prefix = f"inflasi_yoy_{ds_slug}"
        elif "mtm" in category or "mtm" in ds_slug:
            ind_prefix = f"inflasi_mtm_{ds_slug}"
        elif "ytd" in category or "ytd" in ds_slug:
            ind_prefix = f"inflasi_ytd_{ds_slug}"
        else:
            ind_prefix = ds_slug

        suffix_parts = [p for p in (_slug(komoditi, 24) if komoditi else "", _slug(sub_kel, 28) if sub_kel and not category.startswith(("ihk", "inflasi")) else "") if p]
        if suffix_parts:
            indikator = f"{ind_prefix}_{'_'.join(suffix_parts)}"[:96]
        else:
            indikator = ind_prefix[:96]

        domain = _map_bps_category_to_domain(category or ds_slug, f"{indikator} {sub_kel} {komoditi}".lower())
        satuan = "indeks" if category == "ihk" else ("%" if "inflasi" in category or "tenaga_kerja" in category else "satuan")
        rows.append(
            {
                "kode_bps": kode_bps,
                "domain": domain,
                "indikator": indikator,
                "periode": period,
                "nilai": val,
                "sumber": "BPS_TABLES",
                "frekuensi": freq,
                "satuan": satuan,
            }
        )

    df = pd.DataFrame(rows)
    return validate_silver_macro_df(df) if not df.empty else pd.DataFrame(
        columns=["kode_bps", "domain", "indikator", "periode", "nilai", "sumber", "frekuensi", "satuan"]
    )


def transform_pihps_bronze_to_silver(
    payload: dict[str, Any],
    *,
    kode_bps: str,
    sumber: str = "PIHPS",
) -> pd.DataFrame:
    """Normalize raw PIHPS grid JSON capture into Pandera-validated Silver daily commodity prices."""
    raw_rows = payload.get("data") if isinstance(payload, dict) else None
    if not raw_rows:
        return pd.DataFrame(columns=["kode_bps", "komoditas_kode", "periode", "nilai", "sumber", "satuan"])

    rows: list[dict[str, Any]] = []
    for entry in raw_rows:
        if not isinstance(entry, dict):
            continue
        alias = _norm_label(str(entry.get("name") or ""))
        mapped = _PIHPS_COMMODITIES.get(alias)
        if mapped is None:
            continue
        komoditas_kode, satuan = mapped
        for key, raw_val in entry.items():
            if not re.fullmatch(r"\d{2}/\d{2}/\d{4}", str(key)):
                continue
            val = parse_indonesian_decimal(raw_val)
            if math.isnan(val) or val <= 0 or val > 5_000_000:
                continue
            day_ts = pd.to_datetime(key, format="%d/%m/%Y", errors="coerce")
            if pd.isna(day_ts):
                continue
            rows.append(
                {
                    "kode_bps": kode_bps,
                    "komoditas_kode": komoditas_kode,
                    "periode": day_ts.normalize(),
                    "nilai": float(val),
                    "sumber": sumber,
                    "satuan": satuan,
                }
            )
    df = pd.DataFrame(rows)
    return validate_silver_price_df(df) if not df.empty else pd.DataFrame(
        columns=["kode_bps", "komoditas_kode", "periode", "nilai", "sumber", "satuan"]
    )


def transform_open_meteo_bronze_to_silver(
    payload: dict[str, Any],
    *,
    kode_bps: str,
) -> pd.DataFrame:
    """Normalize raw Open-Meteo archive JSON capture into Silver daily weather DataFrame."""
    daily = payload.get("daily") if isinstance(payload, dict) else None
    if not isinstance(daily, dict) or not daily.get("time"):
        return pd.DataFrame(columns=["kabupaten_kode", "tanggal", "indikator", "nilai"])

    times = daily.get("time") or []
    field_map = {
        "precipitation_sum": "curah_hujan_harian",
        "temperature_2m_mean": "suhu_rata",
        "relative_humidity_2m_mean": "kelembapan_rata",
    }
    rows: list[dict[str, Any]] = []
    for raw_col, indikator in field_map.items():
        vals = daily.get(raw_col) or []
        for dt_str, raw_val in zip(times, vals):
            val = parse_indonesian_decimal(raw_val)
            if math.isnan(val):
                continue
            ts = pd.to_datetime(dt_str, errors="coerce")
            if pd.isna(ts):
                continue
            rows.append(
                {
                    "kabupaten_kode": kode_bps,
                    "tanggal": ts.normalize(),
                    "indikator": indikator,
                    "nilai": float(val),
                }
            )
    df = pd.DataFrame(rows)
    return validate_silver_weather_df(df) if not df.empty else pd.DataFrame(
        columns=["kabupaten_kode", "tanggal", "indikator", "nilai"]
    )


@dataclass(frozen=True)
class SilverBundle:
    """Validated Silver DataFrames ready for DuckDB Gold join & feature generation."""

    prices: pd.DataFrame
    macro: pd.DataFrame
    djpk: pd.DataFrame
    weather: pd.DataFrame = field(
        default_factory=lambda: pd.DataFrame(columns=["kabupaten_kode", "tanggal", "indikator", "nilai"])
    )

    def validated(self) -> "SilverBundle":
        return SilverBundle(
            prices=validate_silver_price_df(self.prices),
            macro=validate_silver_macro_df(self.macro),
            djpk=validate_silver_macro_df(self.djpk),
            weather=validate_silver_weather_df(self.weather),
        )


class SilverParquetStore:
    """Materializes standardized Silver tables into Parquet via DuckDB (`data/silver/` or MinIO)."""

    def __init__(
        self,
        *,
        local_dir: str | Path | None = None,
        minio_endpoint: str = "",
        minio_access_key: str = "",
        minio_secret_key: str = "",
        minio_bucket: str = "data-silver",
        minio_secure: bool = False,
        s3_client: Any = None,
    ) -> None:
        self.local_dir = Path(local_dir).resolve() if local_dir else None
        self.minio_endpoint = minio_endpoint.strip()
        self.minio_bucket = minio_bucket.strip() or "data-silver"
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

    def _export_one(self, filename: str, df: pd.DataFrame) -> str | None:
        if self._s3 is None and self.local_dir is None:
            return None
        import io
        import duckdb
        import pyarrow as pa
        import pyarrow.parquet as pq

        clean_df = df if not df.empty else df.copy()
        if self._s3 is not None:
            try:
                self._s3.head_bucket(Bucket=self.minio_bucket)
            except Exception:
                self._s3.create_bucket(Bucket=self.minio_bucket)
            buf = io.BytesIO()
            pq.write_table(pa.Table.from_pandas(clean_df, preserve_index=False), buf, compression="snappy")
            self._s3.put_object(Bucket=self.minio_bucket, Key=filename, Body=buf.getvalue())
            return f"s3://{self.minio_bucket}/{filename}"

        assert self.local_dir is not None
        target = self.local_dir / filename
        target.parent.mkdir(parents=True, exist_ok=True)
        if clean_df.empty:
            pq.write_table(pa.Table.from_pandas(clean_df, preserve_index=False), target, compression="snappy")
            return str(target)
        con = duckdb.connect(":memory:")
        try:
            con.register("_silver_tbl", clean_df)
            con.execute(f"COPY _silver_tbl TO '{target.as_posix()}' (FORMAT PARQUET, COMPRESSION 'snappy')")
        finally:
            con.close()
        return str(target)

    def write_bundle(self, bundle: SilverBundle) -> dict[str, str]:
        clean = bundle.validated()
        uris: dict[str, str] = {}
        for name, df in (
            ("silver_prices.parquet", clean.prices),
            ("silver_macro.parquet", clean.macro),
            ("silver_djpk.parquet", clean.djpk),
            ("silver_weather.parquet", clean.weather),
        ):
            uri = self._export_one(name, df)
            if uri:
                uris[name] = uri
        return uris
