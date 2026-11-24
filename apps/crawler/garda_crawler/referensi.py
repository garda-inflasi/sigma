"""Data acuan dan pemetaan sumber.

Asal data, jangan diubah sepihak:
- KOMODITAS dan KABUPATEN: apps/web/src/lib/data/reference.ts.
- PIHPS_KOMODITAS, PIHPS_ABAIKAN, BMKG_WILAYAH, BPS_WILAYAH: apps/api/internal/ingestion/domain/mapping.go.
- PIHPS_WILAYAH: daftar resmi WebSite/Home/GetRegencyAll?ref_prov_id=1 (diverifikasi 2026-11-24).
  regency_id PIHPS bukan kode BPS, dan ejaannya khas PIHPS ("Simeuleu", "Gayo Luwes").
"""

import re
from dataclasses import dataclass

PROVINSI_KODE = "11"
PIHPS_PROVINSI_ID = 1
BPS_DOMAIN_ACEH = "1100"


@dataclass(frozen=True)
class Komoditas:
    kode: str
    nama: str
    satuan: str
    harga_acuan: int


@dataclass(frozen=True)
class Kabupaten:
    kode: str
    nama: str
    ibukota: str
    lat: float
    lng: float
    jenis: str


KOMODITAS: dict[str, Komoditas] = {
    k.kode: k
    for k in (
        Komoditas("BRS-MED", "Beras medium", "kg", 13900),
        Komoditas("BRS-PRM", "Beras premium", "kg", 15800),
        Komoditas("CBM-KRT", "Cabai merah keriting", "kg", 52000),
        Komoditas("CBR-RWT", "Cabai rawit merah", "kg", 58000),
        Komoditas("BWM", "Bawang merah", "kg", 41000),
        Komoditas("BWP", "Bawang putih", "kg", 38000),
        Komoditas("TLR", "Telur ayam ras", "kg", 29500),
        Komoditas("DAG-AYM", "Daging ayam ras", "kg", 36500),
        Komoditas("DAG-SPI", "Daging sapi", "kg", 148000),
        Komoditas("IKN-TGK", "Ikan tongkol", "kg", 38000),
        Komoditas("MYK", "Minyak goreng curah", "liter", 17500),
        Komoditas("GLA", "Gula pasir", "kg", 18000),
        Komoditas("KTG", "Kentang", "kg", 16000),
        Komoditas("TMT", "Tomat", "kg", 12000),
    )
}

KABUPATEN: dict[str, Kabupaten] = {
    k.kode: k
    for k in (
        Kabupaten("1101", "Simeulue", "Sinabang", 2.48, 96.38, "Kabupaten"),
        Kabupaten("1102", "Aceh Singkil", "Singkil", 2.28, 97.79, "Kabupaten"),
        Kabupaten("1103", "Aceh Selatan", "Tapaktuan", 3.26, 97.18, "Kabupaten"),
        Kabupaten("1104", "Aceh Tenggara", "Kutacane", 3.49, 97.8, "Kabupaten"),
        Kabupaten("1105", "Aceh Timur", "Idi Rayeuk", 4.96, 97.77, "Kabupaten"),
        Kabupaten("1106", "Aceh Tengah", "Takengon", 4.62, 96.85, "Kabupaten"),
        Kabupaten("1107", "Aceh Barat", "Meulaboh", 4.14, 96.13, "Kabupaten"),
        Kabupaten("1108", "Aceh Besar", "Kota Jantho", 5.3, 95.63, "Kabupaten"),
        Kabupaten("1109", "Pidie", "Sigli", 5.38, 95.96, "Kabupaten"),
        Kabupaten("1110", "Bireuen", "Bireuen", 5.2, 96.7, "Kabupaten"),
        Kabupaten("1111", "Aceh Utara", "Lhoksukon", 5.05, 97.32, "Kabupaten"),
        Kabupaten("1112", "Aceh Barat Daya", "Blangpidie", 3.74, 96.84, "Kabupaten"),
        Kabupaten("1113", "Gayo Lues", "Blangkejeren", 3.98, 97.35, "Kabupaten"),
        Kabupaten("1114", "Aceh Tamiang", "Karang Baru", 4.29, 98.05, "Kabupaten"),
        Kabupaten("1115", "Nagan Raya", "Suka Makmue", 4.13, 96.47, "Kabupaten"),
        Kabupaten("1116", "Aceh Jaya", "Calang", 4.63, 95.58, "Kabupaten"),
        Kabupaten("1117", "Bener Meriah", "Simpang Tiga Redelong", 4.73, 96.87, "Kabupaten"),
        Kabupaten("1118", "Pidie Jaya", "Meureudu", 5.25, 96.25, "Kabupaten"),
        Kabupaten("1171", "Banda Aceh", "Banda Aceh", 5.55, 95.32, "Kota"),
        Kabupaten("1172", "Sabang", "Sabang", 5.89, 95.32, "Kota"),
        Kabupaten("1173", "Langsa", "Langsa", 4.47, 97.97, "Kota"),
        Kabupaten("1174", "Lhokseumawe", "Lhokseumawe", 5.18, 97.15, "Kota"),
        Kabupaten("1175", "Subulussalam", "Subulussalam", 2.64, 98.0, "Kota"),
    )
}

PIHPS_KOMODITAS: dict[str, str] = {
    "beras kualitas medium i": "BRS-MED",
    "beras kualitas super i": "BRS-PRM",
    "cabai merah keriting": "CBM-KRT",
    "cabai rawit merah": "CBR-RWT",
    "bawang merah ukuran sedang": "BWM",
    "bawang putih ukuran sedang": "BWP",
    "telur ayam ras segar": "TLR",
    "daging ayam ras segar": "DAG-AYM",
    "daging sapi kualitas 1": "DAG-SPI",
    "minyak goreng curah": "MYK",
    "gula pasir lokal": "GLA",
}

PIHPS_ABAIKAN: frozenset[str] = frozenset(
    {
        # Baris induk kategori.
        "beras", "daging ayam", "daging sapi", "telur ayam", "bawang merah", "bawang putih",
        "cabai merah", "cabai rawit", "minyak goreng", "gula pasir",
        # Varian yang tidak dipakai master.
        "beras kualitas bawah i", "beras kualitas bawah ii", "beras kualitas medium ii",
        "beras kualitas super ii", "cabai merah besar", "cabai rawit hijau", "daging sapi kualitas 2",
        "minyak goreng kemasan bermerk 1", "minyak goreng kemasan bermerk 2", "gula pasir kualitas premium",
    }
)

# regency_id PIHPS -> kode BPS.
PIHPS_WILAYAH: dict[int, str] = {
    1: "1171",  # Banda Aceh
    2: "1174",  # Lhokseumawe
    3: "1107",  # Meulaboh (ibu kota Aceh Barat)
    476: "1117",  # Bener Meriah
    477: "1112",  # Aceh Barat Daya
    478: "1104",  # Aceh Tenggara
    479: "1105",  # Aceh Timur
    480: "1173",  # Langsa
    481: "1118",  # Pidie Jaya
    482: "1116",  # Aceh Jaya
    483: "1110",  # Aceh Jeumpa / Bireun
    484: "1106",  # Aceh Tengah
    485: "1111",  # Aceh Utara
    486: "1175",  # Subulussalam
    487: "1115",  # Nagan Raya
    488: "1114",  # Aceh Tamiang
    489: "1109",  # Pidie
    490: "1172",  # Sabang
    491: "1101",  # Simeuleu
    492: "1113",  # Gayo Luwes
    493: "1102",  # Aceh Singkil
    494: "1103",  # Aceh Selatan
    495: "1108",  # Aceh Besar
}

# Dari 23 wilayah, hanya tiga ini yang berisi harga pasar tradisional (seluruh regency diperiksa 2026-11-24).
# Backfill memakai daftar ini agar tidak membebani server; crawl harian tetap meminta semua wilayah
# supaya perluasan cakupan PIHPS langsung tertangkap.
PIHPS_KAB_BERDATA: frozenset[str] = frozenset({"1171", "1174", "1107"})

# Kode adm4 memakai kode Kemendagri, BUKAN kode BPS: urutannya berbeda, jadi jangan diturunkan dari kode master.
BMKG_WILAYAH: dict[str, str] = {
    "11.09.02.2001": "1101",
    "11.10.01.2001": "1102",
    "11.01.01.2001": "1103",
    "11.02.01.2001": "1104",
    "11.03.01.2001": "1105",
    "11.04.02.2001": "1106",
    "11.05.01.2001": "1107",
    "11.06.01.2001": "1108",
    "11.07.03.2001": "1109",
    "11.11.01.2001": "1110",
    "11.08.01.2001": "1111",
    "11.12.02.2001": "1112",
    "11.13.01.2001": "1113",
    "11.16.01.2001": "1114",
    "11.15.02.2001": "1115",
    "11.14.01.2001": "1116",
    "11.17.01.2001": "1117",
    "11.18.01.2001": "1118",
    "11.71.01.2001": "1171",
    "11.72.03.2001": "1172",
    "11.74.03.2001": "1173",
    "11.73.01.2002": "1174",
    "11.75.01.2001": "1175",
}


def normalisasi_alias(teks: str) -> str:
    return re.sub(r"\s+", " ", teks).strip().lower()


def _alias_bps() -> dict[str, str]:
    alias = {
        "aceh": PROVINSI_KODE,
        "provinsi aceh": PROVINSI_KODE,
        "meulaboh": "1107",
        "kota meulaboh": "1107",
    }
    for k in KABUPATEN.values():
        nama = normalisasi_alias(k.nama)
        alias[nama] = k.kode
        alias[f"{k.jenis.lower()} {nama}"] = k.kode
    return alias


BPS_WILAYAH: dict[str, str] = _alias_bps()
