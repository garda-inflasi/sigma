"""Pembuat data SINTETIS khusus tes. Tidak boleh diimpor dari kode produksi."""

from __future__ import annotations

from datetime import date, datetime, timedelta, timezone

import numpy as np

KOMODITAS = {"CBM-KRT": 45000.0, "BRS-MED": 14000.0, "DAG-AYM": 36000.0, "BWM-MRH": 38000.0}
WILAYAH = ("1171", "1174")


def _dt(t: date) -> datetime:
    return datetime(t.year, t.month, t.day, tzinfo=timezone.utc)


def dokumen(koleksi_sumber: str, dataset: str, kunci: str, t: date, nilai: float | None, *, crawl: date | None = None, **tambahan) -> dict:
    crawl = crawl or t
    crawled_at = _dt(crawl) + timedelta(hours=1)
    return {
        "_id": f"{koleksi_sumber}:{dataset}:{kunci}:{t.isoformat()}:{crawl.isoformat()}",
        "sumber": koleksi_sumber, "dataset": dataset, "kunci_seri": kunci, "indikator": dataset,
        "komoditas_kode": None, "kabupaten_kode": None, "satuan": "Rp/kg", "frekuensi": "HARIAN",
        "periode": _dt(t), "nilai": nilai, "is_carried_forward": False, "last_available_date": _dt(t),
        "crawled_at": crawled_at, "crawl_date": crawl.isoformat(), "source_status": "FRESH",
        "run_id": f"uji-{crawl.isoformat()}", "url_sumber": "https://uji.invalid", "status_validasi": None, "atribut": {},
        **tambahan,
    }


def harga_harian(akhir: date, hari: int, seed: int = 7, komoditas: dict | None = None, wilayah: tuple = WILAYAH, sumber: str = "PIHPS", amplitudo_tahunan: float = 0.02, derau: float = 0.004) -> list[dict]:
    """Harga hari kerja dengan tren, pola mingguan, dan derau."""
    rng = np.random.default_rng(seed)
    hasil = []
    for k, dasar in (komoditas or KOMODITAS).items():
        for w in wilayah:
            level = np.log(dasar) + rng.normal(0, 0.03)
            for i in range(hari, -1, -1):
                t = akhir - timedelta(days=i)
                level += rng.normal(0.00012, derau)
                if t.weekday() >= 5:
                    continue
                nilai = float(np.exp(level + 0.006 * np.sin(2 * np.pi * t.weekday() / 7) + amplitudo_tahunan * np.sin(2 * np.pi * t.timetuple().tm_yday / 365)))
                hasil.append(dokumen(sumber, "harga_pangan", f"{k}:{w}", t, round(nilai, 0), komoditas_kode=k, kabupaten_kode=w, atribut={"komoditas_nama": k}))
    return hasil


def cuaca_harian(akhir: date, hari: int, wilayah: tuple = WILAYAH, seed: int = 11) -> list[dict]:
    rng = np.random.default_rng(seed)
    hasil = []
    for w in wilayah:
        for i in range(hari, -1, -1):
            t = akhir - timedelta(days=i)
            hasil.append(dokumen("OPEN_METEO", "arsip_iklim_harian", f"curah_hujan:{w}", t, float(rng.gamma(2, 4)), kabupaten_kode=w, satuan="mm", indikator="curah_hujan_harian"))
            hasil.append(dokumen("OPEN_METEO", "arsip_iklim_harian", f"suhu:{w}", t, float(rng.normal(27, 1)), kabupaten_kode=w, satuan="celsius", indikator="suhu_rata_harian"))
    return hasil


def _bulan_lalu(akhir: date, n: int) -> list[date]:
    hasil = []
    th, bl = akhir.year, akhir.month
    for _ in range(n):
        bl -= 1
        if bl == 0:
            th, bl = th - 1, 12
        hasil.append(date(th, bl, 1))
    return list(reversed(hasil))


def inflasi_bps(akhir: date, bulan: int, wilayah: tuple = ("1171", "1174"), seed: int = 5) -> list[dict]:
    rng = np.random.default_rng(seed)
    hasil = []
    for w in wilayah:
        for t in _bulan_lalu(akhir, bulan):
            mtm = float(rng.normal(0.18, 0.35))
            # Backfill: crawl jauh setelah rilis, sehingga asumsi konservatif yang berlaku.
            hasil.append(dokumen("BPS", "bps_inflasi", f"inflasi_mtm:{w}", t, mtm, crawl=akhir - timedelta(days=1), indikator="inflasi_month_to_month", kabupaten_kode=w, satuan="persen", frekuensi="BULANAN"))
            hasil.append(dokumen("BPS", "bps_inflasi", f"inflasi_yoy:{w}", t, float(rng.normal(2.5, 0.6)), crawl=akhir - timedelta(days=1), indikator="inflasi_year_on_year", kabupaten_kode=w, satuan="persen", frekuensi="BULANAN"))
    return hasil


def makro_pendukung(akhir: date, bulan: int) -> dict[str, list[dict]]:
    rng = np.random.default_rng(3)
    kolom = {"raw_tenaga_kerja": ("upah_buruh_tani", "Rp"), "raw_neraca_ekonomi": ("ekspor_impor", "juta USD"), "raw_sektor_primer": ("produksi_padi", "ton"), "raw_konsumsi_pendapatan": ("pengeluaran_per_kapita", "Rp")}
    hasil: dict[str, list[dict]] = {}
    for koleksi, (indikator, satuan) in kolom.items():
        nilai = 1000.0
        dok = []
        for t in _bulan_lalu(akhir, bulan):
            nilai *= 1 + rng.normal(0.003, 0.01)
            dok.append(dokumen("BPS", indikator, f"{indikator}:11", t, float(nilai), crawl=akhir - timedelta(days=1), indikator=indikator, kabupaten_kode="11", satuan=satuan, frekuensi="BULANAN"))
        hasil[koleksi] = dok
    return hasil


def crawl_run(sumber: str, dataset: str, akhir: date, status: str = "FRESH", lad: date | None = None, pesan: str = "") -> dict:
    mulai = _dt(akhir) + timedelta(hours=1)
    return {
        "_id": f"run:{sumber}:{dataset}:{akhir.isoformat()}", "crawler": "uji", "sumber": sumber, "dataset": dataset,
        "crawl_date": akhir.isoformat(), "mulai": mulai, "selesai": mulai + timedelta(minutes=2), "status": status,
        "jumlah_baru": 1, "jumlah_diteruskan": 0, "last_available_date": _dt(lad or akhir), "pesan": pesan, "pemicu": "JADWAL",
    }


def isi_dataunseen(db, akhir: date, hari: int = 500, bps_bulan: int = 0, komoditas: dict | None = None) -> None:
    db["raw_pangan_harian"].insert_many(harga_harian(akhir, hari, komoditas=komoditas))
    db["raw_iklim_bmkg"].insert_many(cuaca_harian(akhir, hari))
    for koleksi, dok in makro_pendukung(akhir, max(hari // 30, 6)).items():
        db[koleksi].insert_many(dok)
    if bps_bulan:
        db["raw_bps_makro"].insert_many(inflasi_bps(akhir, bps_bulan))
    runs = [crawl_run("PIHPS", "harga_pangan", akhir), crawl_run("OPEN_METEO", "arsip_iklim_harian", akhir)]
    db["crawl_runs"].insert_many(runs)
