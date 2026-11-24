"""Declarative dataset specifications and schedule SLA metadata."""

from dataclasses import dataclass

from garda_crawler.config.reference_catalog import BPS_ACEH_PROVINCE_CODE, REGENCIES
from garda_crawler.domain.entities import DatasetDefinition

_IHK_REGIONS: tuple[str, ...] = (BPS_ACEH_PROVINCE_CODE, "1171", "1174", "1107")
_ALL_BPS_REGIONS: tuple[str, ...] = (BPS_ACEH_PROVINCE_CODE, *REGENCIES.keys())
_PROVINCE_ONLY: tuple[str, ...] = (BPS_ACEH_PROVINCE_CODE,)


@dataclass(frozen=True)
class SourceSpec:
    """High-level registration entry for a data source plugin."""

    source_code: str
    schedule_tier: str
    stale_sla_days: int
    datasets: tuple[DatasetDefinition, ...]


@dataclass(frozen=True)
class BpsIndicatorSpec:
    """Declarative specification for one BPS WebAPI statistical indicator."""

    key: str
    indicator: str
    unit: str
    frequency: str
    title_pattern: str
    regions: tuple[str, ...]
    fallback_var_id: str = ""


PIHPS_DAILY_DATASET = DatasetDefinition(
    name="pihps_harga_pangan_harian",
    collection="raw_pangan_harian",
    schedule_tier="daily",
    stale_sla_days=2,
    default_backfill_days=30,
)

BMKG_FORECAST_DATASET = DatasetDefinition(
    name="prakiraan_cuaca_adm4",
    collection="raw_iklim_bmkg",
    schedule_tier="daily",
    stale_sla_days=1,
)

BMKG_WARNINGS_DATASET = DatasetDefinition(
    name="peringatan_dini_cuaca",
    collection="raw_iklim_bmkg",
    schedule_tier="daily",
    stale_sla_days=1,
)

OPEN_METEO_ARCHIVE_DATASET = DatasetDefinition(
    name="arsip_iklim_harian",
    collection="raw_iklim_bmkg",
    schedule_tier="daily",
    stale_sla_days=7,
)

BAPANAS_DATASET = DatasetDefinition(
    name="bapanas_panel_harga",
    collection="raw_pangan_harian",
    schedule_tier="daily",
    stale_sla_days=2,
)

SP2KP_DATASET = DatasetDefinition(
    name="sp2kp_kemendag_harian",
    collection="raw_pangan_harian",
    schedule_tier="daily",
    stale_sla_days=2,
)

FIELD_ENTRIES_DATASET = DatasetDefinition(
    name="entri_harga",
    collection="raw_pangan_harian",
    schedule_tier="hourly",
    stale_sla_days=1,
)

BPS_DATASET_SPECS: dict[DatasetDefinition, tuple[BpsIndicatorSpec, ...]] = {
    DatasetDefinition("bps_inflasi", "raw_bps_makro", "daily", 35): (
        BpsIndicatorSpec(
            "inflasi_yoy",
            "inflasi_year_on_year",
            "persen",
            "BULANAN",
            r"inflasi.*(y-?o-?y|year on year|tahun ke tahun)",
            _IHK_REGIONS,
        ),
        BpsIndicatorSpec(
            "inflasi_mtm",
            "inflasi_month_to_month",
            "persen",
            "BULANAN",
            r"inflasi.*(m-?t-?m|month to month|bulan ke bulan)",
            _IHK_REGIONS,
        ),
        BpsIndicatorSpec(
            "inflasi_ytd",
            "inflasi_year_to_date",
            "persen",
            "BULANAN",
            r"inflasi.*(y-?t-?d|year to date|tahun kalender)",
            _IHK_REGIONS,
        ),
        BpsIndicatorSpec(
            "ihk",
            "indeks_harga_konsumen",
            "indeks",
            "BULANAN",
            r"^indeks harga konsumen\b",
            _IHK_REGIONS,
        ),
    ),
    DatasetDefinition("bps_konsumsi_pendapatan", "raw_konsumsi_pendapatan", "daily", 190): (
        BpsIndicatorSpec(
            "pengeluaran_per_kapita",
            "pengeluaran_per_kapita_sebulan",
            "Rp/kapita/bulan",
            "TAHUNAN",
            r"rata-?rata pengeluaran per kapita",
            _ALL_BPS_REGIONS,
        ),
        BpsIndicatorSpec(
            "pengeluaran_makanan",
            "persentase_pengeluaran_makanan",
            "persen",
            "TAHUNAN",
            r"persentase pengeluaran.*makanan",
            _ALL_BPS_REGIONS,
        ),
    ),
    DatasetDefinition("bps_neraca", "raw_neraca_ekonomi", "daily", 100): (
        BpsIndicatorSpec(
            "pdrb_adhb",
            "pdrb_atas_dasar_harga_berlaku",
            "juta Rp",
            "TAHUNAN",
            r"produk domestik regional bruto.*harga berlaku",
            _ALL_BPS_REGIONS,
        ),
        BpsIndicatorSpec(
            "pdrb_adhk",
            "pdrb_atas_dasar_harga_konstan",
            "juta Rp",
            "TAHUNAN",
            r"produk domestik regional bruto.*harga konstan",
            _ALL_BPS_REGIONS,
        ),
        BpsIndicatorSpec(
            "pertumbuhan_ekonomi",
            "laju_pertumbuhan_pdrb",
            "persen",
            "TAHUNAN",
            r"laju pertumbuhan.*(pdrb|produk domestik)",
            _ALL_BPS_REGIONS,
        ),
    ),
    DatasetDefinition("bps_tenaga_kerja", "raw_tenaga_kerja", "daily", 190): (
        BpsIndicatorSpec(
            "tpt",
            "tingkat_pengangguran_terbuka",
            "persen",
            "TAHUNAN",
            r"tingkat pengangguran terbuka",
            _ALL_BPS_REGIONS,
        ),
        BpsIndicatorSpec(
            "tpak",
            "tingkat_partisipasi_angkatan_kerja",
            "persen",
            "TAHUNAN",
            r"tingkat partisipasi angkatan kerja",
            _ALL_BPS_REGIONS,
        ),
        BpsIndicatorSpec(
            "upah_buruh",
            "rata_rata_upah_buruh",
            "Rp/bulan",
            "TAHUNAN",
            r"rata-?rata upah.*buruh",
            _PROVINCE_ONLY,
        ),
    ),
    DatasetDefinition("bps_pertanian", "raw_sektor_primer", "daily", 35): (
        BpsIndicatorSpec(
            "ntp",
            "nilai_tukar_petani",
            "indeks",
            "BULANAN",
            r"^nilai tukar petani\b",
            _PROVINCE_ONLY,
        ),
        BpsIndicatorSpec(
            "produksi_padi",
            "produksi_padi",
            "ton",
            "TAHUNAN",
            r"produksi padi",
            _ALL_BPS_REGIONS,
        ),
    ),
    DatasetDefinition("bps_kehutanan", "raw_sektor_primer", "daily", 370): (
        BpsIndicatorSpec(
            "produksi_kayu",
            "produksi_kayu_bulat",
            "m3",
            "TAHUNAN",
            r"produksi kayu",
            _PROVINCE_ONLY,
        ),
    ),
    DatasetDefinition("bps_perikanan", "raw_sektor_primer", "daily", 370): (
        BpsIndicatorSpec(
            "produksi_perikanan",
            "produksi_perikanan",
            "ton",
            "TAHUNAN",
            r"produksi perikanan",
            _ALL_BPS_REGIONS,
        ),
    ),
}

BPS_DATASETS: tuple[DatasetDefinition, ...] = tuple(BPS_DATASET_SPECS.keys())

SOURCE_SPECS: dict[str, SourceSpec] = {
    "PIHPS": SourceSpec("PIHPS", "daily", 2, (PIHPS_DAILY_DATASET,)),
    "BMKG": SourceSpec("BMKG", "daily", 1, (BMKG_FORECAST_DATASET, BMKG_WARNINGS_DATASET)),
    "OPEN_METEO": SourceSpec("OPEN_METEO", "daily", 7, (OPEN_METEO_ARCHIVE_DATASET,)),
    "BPS": SourceSpec("BPS", "daily", 35, BPS_DATASETS),
    "BAPANAS": SourceSpec("BAPANAS", "daily", 2, (BAPANAS_DATASET,)),
    "SP2KP": SourceSpec("SP2KP", "daily", 2, (SP2KP_DATASET,)),
    "LAPANGAN": SourceSpec("LAPANGAN", "hourly", 1, (FIELD_ENTRIES_DATASET,)),
}
