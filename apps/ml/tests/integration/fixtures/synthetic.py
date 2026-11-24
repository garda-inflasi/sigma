"""Synthetic test-data builders for end-to-end MongoDB pipeline verification."""

from datetime import date, datetime, timedelta, timezone
from typing import Any

import numpy as np

COMMODITIES = {"CBM-KRT": 45000.0, "BRS-MED": 14000.0, "DAG-AYM": 36000.0, "BWM-MRH": 38000.0}
REGENCIES = ("1171", "1174")


def _dt(day: date) -> datetime:
    return datetime(day.year, day.month, day.day, tzinfo=timezone.utc)


def make_doc(
    source_code: str,
    dataset: str,
    series_key: str,
    day: date,
    value: float | None,
    *,
    crawl: date | None = None,
    **extra: Any,
) -> dict[str, Any]:
    crawl_day = crawl or day
    crawled_at = _dt(crawl_day) + timedelta(hours=1)
    return {
        "_id": f"{source_code}:{dataset}:{series_key}:{day.isoformat()}:{crawl_day.isoformat()}",
        "sumber": source_code,
        "dataset": dataset,
        "kunci_seri": series_key,
        "indikator": dataset,
        "komoditas_kode": None,
        "kabupaten_kode": None,
        "satuan": "Rp/kg",
        "frekuensi": "HARIAN",
        "periode": _dt(day),
        "nilai": value,
        "is_carried_forward": False,
        "last_available_date": _dt(day),
        "crawled_at": crawled_at,
        "crawl_date": crawl_day.isoformat(),
        "source_status": "FRESH",
        "run_id": f"uji-{crawl_day.isoformat()}",
        "url_sumber": "https://uji.invalid",
        "status_validasi": None,
        "atribut": {},
        **extra,
    }


def daily_prices(
    end_day: date,
    days: int,
    seed: int = 7,
    commodities: dict[str, float] | None = None,
    regencies: tuple[str, ...] = REGENCIES,
    source_code: str = "PIHPS",
    annual_amp: float = 0.02,
    noise: float = 0.004,
) -> list[dict[str, Any]]:
    rng = np.random.default_rng(seed)
    docs: list[dict[str, Any]] = []
    for k, base in (commodities or COMMODITIES).items():
        for w in regencies:
            level = np.log(base) + rng.normal(0, 0.03)
            for i in range(days, -1, -1):
                day = end_day - timedelta(days=i)
                level += rng.normal(0.00012, noise)
                if day.weekday() >= 5:
                    continue
                val = float(
                    np.exp(
                        level
                        + 0.006 * np.sin(2 * np.pi * day.weekday() / 7)
                        + annual_amp * np.sin(2 * np.pi * day.timetuple().tm_yday / 365)
                    )
                )
                docs.append(
                    make_doc(
                        source_code,
                        "harga_pangan",
                        f"{k}:{w}",
                        day,
                        round(val, 0),
                        komoditas_kode=k,
                        kabupaten_kode=w,
                        atribut={"komoditas_nama": k},
                    )
                )
    return docs


def daily_weather(end_day: date, days: int, regencies: tuple[str, ...] = REGENCIES, seed: int = 11) -> list[dict[str, Any]]:
    rng = np.random.default_rng(seed)
    docs: list[dict[str, Any]] = []
    for w in regencies:
        for i in range(days, -1, -1):
            day = end_day - timedelta(days=i)
            docs.append(make_doc("OPEN_METEO", "arsip_iklim_harian", f"curah_hujan:{w}", day, float(rng.gamma(2, 4)), kabupaten_kode=w, satuan="mm", indikator="curah_hujan_harian"))
            docs.append(make_doc("OPEN_METEO", "arsip_iklim_harian", f"suhu:{w}", day, float(rng.normal(27, 1)), kabupaten_kode=w, satuan="celsius", indikator="suhu_rata_harian"))
    return docs


def _previous_months(end_day: date, count: int) -> list[date]:
    months = []
    y, m = end_day.year, end_day.month
    for _ in range(count):
        m -= 1
        if m == 0:
            y, m = y - 1, 12
        months.append(date(y, m, 1))
    return list(reversed(months))


def bps_inflation(end_day: date, months: int, regencies: tuple[str, ...] = ("1171", "1174"), seed: int = 5) -> list[dict[str, Any]]:
    rng = np.random.default_rng(seed)
    docs: list[dict[str, Any]] = []
    for w in regencies:
        for day in _previous_months(end_day, months):
            mtm = float(rng.normal(0.18, 0.35))
            docs.append(make_doc("BPS", "bps_inflasi", f"inflasi_mtm:{w}", day, mtm, crawl=end_day - timedelta(days=1), indikator="inflasi_month_to_month", kabupaten_kode=w, satuan="persen", frekuensi="BULANAN"))
            docs.append(make_doc("BPS", "bps_inflasi", f"inflasi_yoy:{w}", day, float(rng.normal(2.5, 0.6)), crawl=end_day - timedelta(days=1), indikator="inflasi_year_on_year", kabupaten_kode=w, satuan="persen", frekuensi="BULANAN"))
    return docs


def supporting_macro(end_day: date, months: int) -> dict[str, list[dict[str, Any]]]:
    rng = np.random.default_rng(3)
    specs = {
        "raw_tenaga_kerja": ("upah_buruh_tani", "Rp"),
        "raw_neraca_ekonomi": ("ekspor_impor", "juta USD"),
        "raw_sektor_primer": ("produksi_padi", "ton"),
        "raw_konsumsi_pendapatan": ("pengeluaran_per_kapita", "Rp"),
    }
    out: dict[str, list[dict[str, Any]]] = {}
    for col_name, (ind, unit) in specs.items():
        val = 1000.0
        docs = []
        for day in _previous_months(end_day, months):
            val *= 1 + rng.normal(0.003, 0.01)
            docs.append(make_doc("BPS", ind, f"{ind}:11", day, float(val), crawl=end_day - timedelta(days=1), indikator=ind, kabupaten_kode="11", satuan=unit, frekuensi="BULANAN"))
        out[col_name] = docs
    return out


def crawl_run(source_code: str, dataset: str, end_day: date, status: str = "FRESH", lad: date | None = None, message: str = "") -> dict[str, Any]:
    started = _dt(end_day) + timedelta(hours=1)
    return {
        "_id": f"run:{source_code}:{dataset}:{end_day.isoformat()}",
        "crawler": "uji",
        "sumber": source_code,
        "dataset": dataset,
        "crawl_date": end_day.isoformat(),
        "mulai": started,
        "selesai": started + timedelta(minutes=2),
        "status": status,
        "jumlah_baru": 1,
        "jumlah_diteruskan": 0,
        "last_available_date": _dt(lad or end_day),
        "pesan": message,
        "pemicu": "JADWAL",
    }


def seed_dataunseen(db, end_day: date, days: int = 500, bps_months: int = 0, commodities: dict[str, float] | None = None) -> None:
    db["raw_pangan_harian"].insert_many(daily_prices(end_day, days, commodities=commodities))
    db["raw_iklim_bmkg"].insert_many(daily_weather(end_day, days))
    for col_name, docs in supporting_macro(end_day, max(days // 30, 6)).items():
        db[col_name].insert_many(docs)
    if bps_months:
        db["raw_bps_makro"].insert_many(bps_inflation(end_day, bps_months))
    db["crawl_runs"].insert_many([crawl_run("PIHPS", "harga_pangan", end_day), crawl_run("OPEN_METEO", "arsip_iklim_harian", end_day)])
