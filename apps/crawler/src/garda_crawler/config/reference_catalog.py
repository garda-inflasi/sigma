"""Canonical Aceh regency codes, commodities, and external source alias mappings."""

import re
from dataclasses import dataclass
from typing import Any

BPS_ACEH_PROVINCE_CODE = "11"


@dataclass(frozen=True)
class CommoditySpec:
    """Master commodity reference used to normalize units and outlier bounds."""

    code: str
    name: str
    unit: str
    reference_price: float


@dataclass(frozen=True)
class RegencySpec:
    """Canonical Aceh regency/municipality metadata across BPS, PIHPS, BMKG, and Open-Meteo."""

    code: str
    name: str
    lat: float
    lon: float


def normalize_alias(raw: str) -> str:
    """Normalize display labels into single-spaced lowercase keys."""
    return re.sub(r"\s+", " ", raw.strip().lower())


COMMODITIES: dict[str, CommoditySpec] = {
    item.code: item
    for item in (
        CommoditySpec("BRS-PRE", "Beras Premium", "Rp/kg", 15900),
        CommoditySpec("BRS-MED", "Beras Medium", "Rp/kg", 13900),
        CommoditySpec("CBM-KRT", "Cabai Merah Keriting", "Rp/kg", 50000),
        CommoditySpec("CBM-BSR", "Cabai Merah Besar", "Rp/kg", 50000),
        CommoditySpec("CBR-MRH", "Cabai Rawit Merah", "Rp/kg", 55000),
        CommoditySpec("CBR-HJU", "Cabai Rawit Hijau", "Rp/kg", 50000),
        CommoditySpec("BWM", "Bawang Merah", "Rp/kg", 40000),
        CommoditySpec("BWP", "Bawang Putih", "Rp/kg", 42000),
        CommoditySpec("MYK", "Minyak Goreng", "Rp/liter", 17000),
        CommoditySpec("DGG-AYM", "Daging Ayam Ras", "Rp/kg", 33000),
        CommoditySpec("DGG-SPI", "Daging Sapi", "Rp/kg", 145000),
        CommoditySpec("TLR-AYM", "Telur Ayam Ras", "Rp/kg", 29500),
        CommoditySpec("GLA", "Gula Pasir", "Rp/kg", 18000),
        CommoditySpec("TMT", "Tomat", "Rp/kg", 12000),
    )
}

REGENCIES: dict[str, RegencySpec] = {
    reg.code: reg
    for reg in (
        RegencySpec("1101", "Kab. Simeulue", 2.48, 96.36),
        RegencySpec("1102", "Kab. Aceh Singkil", 2.28, 97.79),
        RegencySpec("1103", "Kab. Aceh Selatan", 3.25, 97.18),
        RegencySpec("1104", "Kab. Aceh Tenggara", 3.49, 97.81),
        RegencySpec("1105", "Kab. Aceh Timur", 4.96, 97.76),
        RegencySpec("1106", "Kab. Aceh Tengah", 4.63, 96.84),
        RegencySpec("1107", "Kab. Aceh Barat", 4.14, 96.13),
        RegencySpec("1108", "Kab. Aceh Besar", 5.38, 95.52),
        RegencySpec("1109", "Kab. Pidie", 5.38, 95.96),
        RegencySpec("1110", "Kab. Bireuen", 5.20, 96.70),
        RegencySpec("1111", "Kab. Aceh Utara", 5.14, 97.27),
        RegencySpec("1112", "Kab. Aceh Barat Daya", 3.74, 96.84),
        RegencySpec("1113", "Kab. Gayo Lues", 3.98, 97.35),
        RegencySpec("1114", "Kab. Aceh Tamiang", 4.29, 98.06),
        RegencySpec("1115", "Kab. Nagan Raya", 4.18, 96.38),
        RegencySpec("1116", "Kab. Aceh Jaya", 4.65, 95.59),
        RegencySpec("1117", "Kab. Bener Meriah", 4.73, 96.87),
        RegencySpec("1118", "Kab. Pidie Jaya", 5.22, 96.25),
        RegencySpec("1171", "Kota Banda Aceh", 5.55, 95.32),
        RegencySpec("1172", "Kota Sabang", 5.89, 95.32),
        RegencySpec("1173", "Kota Langsa", 4.47, 97.96),
        RegencySpec("1174", "Kota Lhokseumawe", 5.18, 97.15),
        RegencySpec("1175", "Kota Subulussalam", 2.64, 98.00),
    )
}

# PIHPS BI commodity name aliases -> master codes.
PIHPS_COMMODITIES: dict[str, str] = {
    "beras kualitas super i": "BRS-PRE",
    "beras kualitas medium i": "BRS-MED",
    "daging ayam ras segar": "DGG-AYM",
    "daging sapi kualitas 1": "DGG-SPI",
    "telur ayam ras segar": "TLR-AYM",
    "bawang merah ukuran sedang": "BWM",
    "bawang putih ukuran sedang": "BWP",
    "cabai merah besar": "CBM-BSR",
    "cabai merah keriting": "CBM-KRT",
    "cabai rawit hijau": "CBR-HJU",
    "cabai rawit merah": "CBR-MRH",
    "minyak goreng kemasan bermerk 1": "MYK",
    "minyak goreng kemasan bermerek 1": "MYK",
    "gula pasir lokal": "GLA",
}

# Parent/redundant PIHPS grid rows intentionally skipped during parsing.
PIHPS_IGNORED_ROWS: frozenset[str] = frozenset(
    {
        "beras",
        "beras kualitas bawah i",
        "beras kualitas bawah ii",
        "beras kualitas medium ii",
        "beras kualitas super ii",
        "daging ayam",
        "daging sapi",
        "daging sapi kualitas 2",
        "telur ayam",
        "bawang merah",
        "bawang putih",
        "cabai merah",
        "cabai rawit",
        "minyak goreng",
        "minyak goreng curah",
        "minyak goreng kemasan bermerk 2",
        "minyak goreng kemasan bermerek 2",
        "gula pasir",
        "gula pasir kualitas premium",
    }
)

# PIHPS regency_id (from PIHPS master API) -> canonical BPS regency_code.
PIHPS_REGIONS: dict[int, str] = {
    1: "1171",   # Banda Aceh
    2: "1174",   # Lhokseumawe
    3: "1107",   # Meulaboh / Aceh Barat
    476: "1117", # Bener Meriah
    477: "1112", # Aceh Barat Daya
    478: "1104", # Aceh Tenggara
    479: "1105", # Aceh Timur
    480: "1173", # Langsa
    481: "1118", # Pidie Jaya
    482: "1116", # Aceh Jaya
    483: "1110", # Bireuen
    484: "1106", # Aceh Tengah
    485: "1111", # Aceh Utara
    486: "1175", # Subulussalam
    487: "1115", # Nagan Raya
    488: "1114", # Aceh Tamiang
    489: "1109", # Pidie
    490: "1172", # Sabang
    491: "1101", # Simeulue
    492: "1113", # Gayo Lues
    493: "1102", # Aceh Singkil
    494: "1103", # Aceh Selatan
    495: "1108", # Aceh Besar
}

# Regencies with verified historical observations on PIHPS (used for default backfills).
PIHPS_REGENCIES_WITH_DATA: frozenset[str] = frozenset({"1171", "1174", "1107"})

# BMKG ADM4 Kemendagri village/kelurahan codes -> canonical BPS regency_code.
# Note: BMKG uses Kemendagri coding where Simeulue=11.09, Langsa=11.74, Lhokseumawe=11.73.
BMKG_REGIONS: dict[str, str] = {
    "11.09.02.2001": "1101",
    "11.10.04.2001": "1102",
    "11.01.08.2001": "1103",
    "11.02.04.2001": "1104",
    "11.03.03.2001": "1105",
    "11.04.17.2001": "1106",
    "11.05.01.2001": "1107",
    "11.06.03.2001": "1108",
    "11.07.16.2001": "1109",
    "11.11.13.2001": "1110",
    "11.08.02.2001": "1111",
    "11.12.01.2001": "1112",
    "11.13.01.2001": "1113",
    "11.16.05.2001": "1114",
    "11.15.01.2001": "1115",
    "11.14.01.2001": "1116",
    "11.17.05.2001": "1117",
    "11.18.01.2001": "1118",
    "11.71.01.2001": "1171",
    "11.72.01.2001": "1172",
    "11.74.03.2001": "1173",
    "11.73.01.2002": "1174",
    "11.75.01.2001": "1175",
}


def _build_bps_region_aliases() -> dict[str, str]:
    aliases: dict[str, str] = {
        "aceh": BPS_ACEH_PROVINCE_CODE,
        "provinsi aceh": BPS_ACEH_PROVINCE_CODE,
        "meulaboh": "1107",
        "indonesia": "0000",
    }
    for regency in REGENCIES.values():
        key = normalize_alias(regency.name)
        aliases[key] = regency.code
        aliases[key.replace("kab. ", "kab ")] = regency.code
        short = re.sub(r"^(kab|kota)\.?\s+", "", key)
        aliases[short] = regency.code
    return aliases


BPS_REGIONS: dict[str, str] = _build_bps_region_aliases()


def _build_aceh_bps_whitelist() -> dict[str, str]:
    """Strict whitelist mapping Provinsi Aceh + 18 Kabupaten + 5 Kota + alias Meulaboh -> kode_bps."""
    mapping: dict[str, str] = {
        "aceh": BPS_ACEH_PROVINCE_CODE,
        "provinsi aceh": BPS_ACEH_PROVINCE_CODE,
        "prov aceh": BPS_ACEH_PROVINCE_CODE,
        "prov. aceh": BPS_ACEH_PROVINCE_CODE,
        "meulaboh": "1107",
    }
    for regency in REGENCIES.values():
        norm = normalize_alias(regency.name)
        bare = re.sub(r"^(kabupaten|kab|kota)\.?\s+", "", norm).strip()
        for variant in (
            norm,
            bare,
            f"kab {bare}",
            f"kab. {bare}",
            f"kabupaten {bare}",
            f"kota {bare}",
        ):
            mapping[variant] = regency.code
    return mapping


ACEH_BPS_WHITELIST: dict[str, str] = _build_aceh_bps_whitelist()


def match_aceh_bps_region(raw_label: str, wilayah_code: Any = None) -> tuple[str, str] | None:
    """Match row label/code strictly against Aceh whitelist; return `(kode_bps, canonical_name)` or None."""
    if wilayah_code is not None:
        code_str = re.sub(r"\D", "", str(wilayah_code))
        if len(code_str) >= 4 and code_str[:4] in REGENCIES:
            code4 = code_str[:4]
            return code4, REGENCIES[code4].name
        if code_str in {"11", "1100", "1100000"}:
            return BPS_ACEH_PROVINCE_CODE, "Provinsi Aceh"

    cleaned = normalize_alias(str(raw_label or ""))
    # Bersihkan prefix spasi/titik berlebih pada KAB. / KABUPATEN / PROV.
    cleaned_nodot = re.sub(r"\s+", " ", cleaned.replace(".", " ")).strip()
    for candidate in (cleaned, cleaned_nodot):
        kode = ACEH_BPS_WHITELIST.get(candidate)
        if kode is not None:
            name = "Provinsi Aceh" if kode == BPS_ACEH_PROVINCE_CODE else REGENCIES[kode].name
            return kode, name

    # Coba buang prefix "kabupaten " / "kab " / "kota " / "provinsi "
    stripped = re.sub(r"^(provinsi|prov|kabupaten|kab|kota)\.?\s+", "", cleaned_nodot).strip()
    kode = ACEH_BPS_WHITELIST.get(stripped)
    if kode is not None:
        name = "Provinsi Aceh" if kode == BPS_ACEH_PROVINCE_CODE else REGENCIES[kode].name
        return kode, name
    return None

# Cross-source DJPK (provinsi, pemda) <-> canonical BPS code mapping matching dim_wilayah.
DJPK_TO_BPS_REGIONS: dict[tuple[str, str], str] = {
    ("01", "00"): "11",
    ("01", "12"): "1101",
    ("01", "10"): "1102",
    ("01", "02"): "1103",
    ("01", "03"): "1104",
    ("01", "04"): "1105",
    ("01", "05"): "1106",
    ("01", "01"): "1107",
    ("01", "06"): "1108",
    ("01", "07"): "1109",
    ("01", "11"): "1110",
    ("01", "08"): "1111",
    ("01", "13"): "1112",
    ("01", "14"): "1113",
    ("01", "17"): "1114",
    ("01", "16"): "1115",
    ("01", "15"): "1116",
    ("01", "18"): "1117",
    ("01", "19"): "1118",
    ("01", "09"): "1171",
    ("01", "51"): "1172",
    ("01", "53"): "1173",
    ("01", "52"): "1174",
    ("01", "54"): "1175",
}
BPS_TO_DJPK_REGIONS: dict[str, tuple[str, str]] = {
    bps: djpk for djpk, bps in DJPK_TO_BPS_REGIONS.items()
}

# Common Indonesian month names for parsing BPS monthly labels.
INDONESIAN_MONTHS: dict[str, int] = {
    "januari": 1,
    "jan": 1,
    "februari": 2,
    "feb": 2,
    "maret": 3,
    "mar": 3,
    "april": 4,
    "apr": 4,
    "mei": 5,
    "juni": 6,
    "jun": 6,
    "juli": 7,
    "jul": 7,
    "agustus": 8,
    "agu": 8,
    "agt": 8,
    "september": 9,
    "sep": 9,
    "oktober": 10,
    "okt": 10,
    "november": 11,
    "nov": 11,
    "desember": 12,
    "des": 12,
}
