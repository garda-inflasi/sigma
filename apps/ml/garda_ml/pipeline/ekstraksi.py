"""Ekstraksi ``dataunseen`` menjadi tabel siap fitur.

Aturan yang ditegakkan di sini:

- Titik latih hanya dokumen ``is_carried_forward: false`` dengan nilai positif, bukan ``ERROR``.
- Hanya dokumen dengan ``crawled_at <= cutoff`` yang terlihat, sehingga siklus yang sama
  dapat diulang dengan masukan yang sama.
- Dokumen carried-forward hanya dipakai untuk kesegaran dan umur data.
- Satu seri harga memakai satu sumber utama (PIHPS, lalu SP2KP/BAPANAS). Input LAPANGAN
  menjadi target hanya bila seri tidak punya sumber resmi; bila ada, ia menjadi fitur
  berbobot supaya perbedaan level pasar tidak tampak sebagai lonjakan harga.
- Input LAPANGAN hanya tipe ``KONSUMEN``: harga produsen atau grosir punya level berbeda
  dan tidak boleh dirata-ratakan dengan harga konsumen PIHPS.
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field
from datetime import datetime, timedelta

import numpy as np
import pandas as pd
from pymongo.database import Database

from .config import BOBOT_LAPANGAN, JEDA_RILIS_HARI, SUMBER_HARGA_RESMI, TIPE_HARGA_LAPANGAN
from .waktu import tanggal_wib, tengah_malam_utc, utc

K_HARGA = "raw_pangan_harian"
K_IKLIM = "raw_iklim_bmkg"
K_MAKRO = ("raw_bps_makro", "raw_konsumsi_pendapatan", "raw_neraca_ekonomi", "raw_tenaga_kerja", "raw_sektor_primer")
K_RUNS = "crawl_runs"

_KOLOM_HARGA = ["komoditas_kode", "kabupaten_kode", "tanggal", "nilai", "bobot", "nilai_lapangan", "bobot_lapangan", "sumber", "satuan"]
_KOLOM_MAKRO = [
    "domain", "indikator", "wilayah_kode", "periode", "tanggal_rilis", "nilai", "satuan", "sumber",
    "frekuensi", "is_carried_forward", "last_available_date",
]


@dataclass
class DataMentah:
    harga: pd.DataFrame
    cuaca: pd.DataFrame
    makro: pd.DataFrame
    makro_terkini: pd.DataFrame
    kesegaran: list[dict]
    nama_komoditas: dict[str, str] = field(default_factory=dict)
    # (komoditas, kabupaten, sumber) -> last_available_date dari dokumen carried-forward
    tanggal_asli_terakhir: dict[tuple[str, str, str], pd.Timestamp] = field(default_factory=dict)
    jumlah_baru: int = 0


def _naif(nilai: datetime | None) -> pd.Timestamp | None:
    if nilai is None:
        return None
    return pd.Timestamp(utc(nilai).replace(tzinfo=None))


def tanggal_rilis_konservatif(periode: pd.Series, frekuensi: pd.Series) -> pd.Series:
    """Akhir periode + jeda kontrak (bulanan: +35 hari)."""
    akhir = periode.copy()
    for frek, ofset in (("BULANAN", pd.offsets.MonthEnd(0)), ("TRIWULANAN", pd.offsets.QuarterEnd(0)), ("TAHUNAN", pd.offsets.YearEnd(0))):
        maska = frekuensi == frek
        if maska.any():
            akhir.loc[maska] = periode.loc[maska] + ofset
    jeda = frekuensi.map(JEDA_RILIS_HARI).fillna(JEDA_RILIS_HARI["BULANAN"]).astype(int)
    return akhir + pd.to_timedelta(jeda, unit="D")


def gabung_harga(baris: pd.DataFrame) -> pd.DataFrame:
    """Gabungkan titik harga mentah menjadi satu nilai per komoditas x kabupaten x tanggal.

    Kolom masukan: ``sumber, dataset, kunci_seri, komoditas_kode, kabupaten_kode, tanggal,
    nilai, satuan, status_validasi, crawled_at`` dan opsional ``tipe``. Keluaran: ``nilai`` target, ``bobot`` sampel
    (1,0 untuk sumber resmi; bobot status untuk seri yang hanya punya input lapangan),
    serta ``nilai_lapangan`` dan ``bobot_lapangan`` (rata-rata berbobot harian).
    """
    if baris.empty:
        return pd.DataFrame(columns=_KOLOM_HARGA)
    d = baris[baris["nilai"].notna() & (baris["nilai"] > 0)].copy()
    lapangan = d["sumber"] == "LAPANGAN"
    d["bobot_status"] = 1.0
    d.loc[lapangan, "bobot_status"] = d.loc[lapangan, "status_validasi"].map(BOBOT_LAPANGAN)
    d = d[d["bobot_status"].notna()]  # entri lapangan ditolak atau tanpa status tidak dipakai
    if "tipe" in d:
        beda_tipe = (d["sumber"] == "LAPANGAN") & d["tipe"].notna() & (d["tipe"] != TIPE_HARGA_LAPANGAN)
        d = d[~beda_tipe]
    # Crawl ulang periode yang sama: nilai terbaru menang.
    d = d.sort_values("crawled_at").drop_duplicates(["sumber", "dataset", "kunci_seri", "tanggal"], keep="last")

    kunci = ["komoditas_kode", "kabupaten_kode"]
    lap = d[d["sumber"] == "LAPANGAN"]
    if lap.empty:
        harian_lap = pd.DataFrame(columns=[*kunci, "tanggal", "nilai_lapangan", "bobot_lapangan", "bobot_maks"])
    else:
        lap = lap.assign(_kali=lap["nilai"] * lap["bobot_status"])
        grup = lap.groupby([*kunci, "tanggal"], sort=False)
        harian_lap = grup.agg(_kali=("_kali", "sum"), bobot_lapangan=("bobot_status", "sum"), bobot_maks=("bobot_status", "max")).reset_index()
        harian_lap["nilai_lapangan"] = harian_lap["_kali"] / harian_lap["bobot_lapangan"]
        harian_lap["bobot_lapangan"] = harian_lap["bobot_maks"]
        harian_lap = harian_lap.drop(columns="_kali")

    resmi = d[d["sumber"] != "LAPANGAN"]
    bagian: list[pd.DataFrame] = []
    if not resmi.empty:
        jumlah = resmi.groupby([*kunci, "sumber"], sort=False).size().reset_index(name="n")
        urutan = {s: i for i, s in enumerate(SUMBER_HARGA_RESMI)}
        jumlah["prioritas"] = jumlah["sumber"].map(urutan).fillna(len(urutan))
        utama = jumlah.sort_values(["prioritas", "n"], ascending=[True, False]).drop_duplicates(kunci)[[*kunci, "sumber"]]
        terpilih = resmi.merge(utama, on=[*kunci, "sumber"])
        terpilih = terpilih.groupby([*kunci, "tanggal"], as_index=False).agg(nilai=("nilai", "mean"), sumber=("sumber", "first"), satuan=("satuan", "last"))
        terpilih["bobot"] = 1.0
        bagian.append(terpilih)
        ada_resmi = utama[kunci].assign(_resmi=True)
    else:
        ada_resmi = pd.DataFrame(columns=[*kunci, "_resmi"])

    if not harian_lap.empty:
        hanya = harian_lap.merge(ada_resmi, on=kunci, how="left")
        hanya = hanya[hanya["_resmi"].isna()]
        if not hanya.empty:
            satuan = lap.groupby(kunci, sort=False)["satuan"].last().reset_index()
            t = hanya.merge(satuan, on=kunci, how="left")
            bagian.append(pd.DataFrame({
                "komoditas_kode": t["komoditas_kode"], "kabupaten_kode": t["kabupaten_kode"], "tanggal": t["tanggal"],
                "nilai": t["nilai_lapangan"], "sumber": "LAPANGAN", "satuan": t["satuan"], "bobot": t["bobot_maks"],
            }))
    if not bagian:
        return pd.DataFrame(columns=_KOLOM_HARGA)
    hasil = pd.concat(bagian, ignore_index=True)
    if harian_lap.empty:
        hasil["nilai_lapangan"] = np.nan
        hasil["bobot_lapangan"] = np.nan
    else:
        hasil = hasil.merge(harian_lap[[*kunci, "tanggal", "nilai_lapangan", "bobot_lapangan"]], on=[*kunci, "tanggal"], how="left")
    return hasil[_KOLOM_HARGA].sort_values(["komoditas_kode", "kabupaten_kode", "tanggal"], ignore_index=True)


def domain_makro(koleksi: str, indikator: str, dataset: str = "") -> str:
    """Petakan dokumen makro ke ``domain`` kontrak."""
    teks = f"{indikator} {dataset}".lower()
    tetap = {"raw_konsumsi_pendapatan": "KONSUMSI_PENDAPATAN", "raw_neraca_ekonomi": "NERACA_EKONOMI", "raw_tenaga_kerja": "TENAGA_KERJA"}
    if koleksi in tetap:
        return tetap[koleksi]
    if koleksi == "raw_sektor_primer":
        if re.search(r"hutan|kayu", teks):
            return "KEHUTANAN"
        if re.search(r"ikan|perikanan|tangkap|budidaya", teks):
            return "PERIKANAN"
        return "PERTANIAN"
    if re.search(r"inflasi|ihk|deflasi", teks):
        return "INFLASI"
    if re.search(r"upah|ump|umk|tenaga kerja|pengangguran|tpt|tpak", teks):
        return "TENAGA_KERJA"
    if re.search(r"konsumsi|pendapatan|pengeluaran|kemiskinan", teks):
        return "KONSUMSI_PENDAPATAN"
    if re.search(r"padi|beras|produksi|panen|luas", teks):
        return "PERTANIAN"
    return "NERACA_EKONOMI"


def _ke_frame(dokumen: list[dict]) -> pd.DataFrame:
    if not dokumen:
        return pd.DataFrame()
    d = pd.DataFrame(dokumen)
    for kolom in ("periode", "crawled_at", "last_available_date"):
        if kolom in d:
            d[kolom] = pd.to_datetime(d[kolom], utc=True, errors="coerce").dt.tz_localize(None)
    return d


def _baca_harga(db: Database, cutoff: datetime, tgl: datetime, batas: datetime) -> tuple[pd.DataFrame, dict, dict]:
    filter_asli = {
        "is_carried_forward": False, "nilai": {"$ne": None}, "source_status": {"$ne": "ERROR"},
        "periode": {"$gte": batas, "$lte": tgl}, "crawled_at": {"$lte": cutoff},
        "komoditas_kode": {"$ne": None}, "kabupaten_kode": {"$ne": None},
    }
    proyeksi = {
        "_id": 0, "sumber": 1, "dataset": 1, "kunci_seri": 1, "komoditas_kode": 1, "kabupaten_kode": 1,
        "periode": 1, "nilai": 1, "satuan": 1, "status_validasi": 1, "crawled_at": 1, "indikator": 1,
        "atribut.komoditas_nama": 1, "atribut.nama_komoditas": 1, "atribut.nama_sumber": 1, "atribut.tipe": 1,
    }
    dok = list(db[K_HARGA].find(filter_asli, proyeksi).batch_size(5000))
    nama: dict[str, str] = {}
    for x in dok:
        atribut = x.pop("atribut", None) or {}
        x["tipe"] = atribut.get("tipe")
        n = atribut.get("komoditas_nama") or atribut.get("nama_komoditas") or atribut.get("nama_sumber")
        if n:
            nama[x["komoditas_kode"]] = str(n)
    d = _ke_frame(dok)
    if not d.empty:
        d = d.rename(columns={"periode": "tanggal"})
        for kolom in ("status_validasi", "satuan", "dataset", "kunci_seri"):
            if kolom not in d:
                d[kolom] = None
        d["kunci_seri"] = d["kunci_seri"].fillna(d["komoditas_kode"].astype(str) + ":" + d["kabupaten_kode"].astype(str))

    # Per sumber: tanggal LAPANGAN yang lebih baru tidak boleh menutupi PIHPS yang basi.
    asli_terakhir: dict[tuple[str, str, str], pd.Timestamp] = {}
    pipa = [
        {"$match": {
            "is_carried_forward": True, "crawled_at": {"$lte": cutoff}, "last_available_date": {"$ne": None},
            "komoditas_kode": {"$ne": None}, "kabupaten_kode": {"$ne": None},
            "$or": [{"sumber": {"$ne": "LAPANGAN"}}, {"kunci_seri": {"$regex": f":{TIPE_HARGA_LAPANGAN}$"}}],
        }},
        {"$group": {"_id": {"k": "$komoditas_kode", "w": "$kabupaten_kode", "s": "$sumber"}, "t": {"$max": "$last_available_date"}}},
    ]
    for x in db[K_HARGA].aggregate(pipa):
        asli_terakhir[(x["_id"]["k"], x["_id"]["w"], x["_id"]["s"])] = _naif(x["t"])
    return d, nama, asli_terakhir


def _baca_cuaca(db: Database, cutoff: datetime, tgl: datetime, batas: datetime) -> pd.DataFrame:
    filter_asli = {
        "is_carried_forward": False, "nilai": {"$ne": None}, "source_status": {"$ne": "ERROR"},
        "periode": {"$gte": batas, "$lte": tgl}, "crawled_at": {"$lte": cutoff},
        # Kontrak: prakiraan BMKG tidak boleh menjadi fitur historis.
        "atribut.jenis_data": {"$ne": "PRAKIRAAN"},
    }
    proyeksi = {"_id": 0, "sumber": 1, "kabupaten_kode": 1, "indikator": 1, "periode": 1, "nilai": 1, "crawled_at": 1}
    d = _ke_frame(list(db[K_IKLIM].find(filter_asli, proyeksi).batch_size(5000)))
    if d.empty:
        return pd.DataFrame(columns=["kabupaten_kode", "tanggal", "indikator", "nilai"])
    d = d.dropna(subset=["kabupaten_kode", "indikator"]).rename(columns={"periode": "tanggal"})
    # BMKG dan OPEN_METEO berbagi kunci_seri, jadi sumber dipilih eksplisit: reanalisis diutamakan.
    d["_urut"] = (d["sumber"] == "OPEN_METEO").astype(int)
    d = d.sort_values(["_urut", "crawled_at"]).drop_duplicates(["kabupaten_kode", "indikator", "tanggal"], keep="last")
    return d[["kabupaten_kode", "tanggal", "indikator", "nilai"]].reset_index(drop=True)


def _rilis_atribut(atribut: dict | None) -> pd.Timestamp | None:
    mentah = (atribut or {}).get("tanggal_rilis")
    if mentah is None:
        return None
    t = pd.to_datetime(mentah, utc=True, errors="coerce")
    return None if pd.isna(t) else t.tz_localize(None)


def _baca_makro(db: Database, cutoff: datetime, tgl: datetime) -> tuple[pd.DataFrame, pd.DataFrame]:
    """Kembalikan (titik asli dengan tanggal rilis efektif, baris terkini tiap indikator).

    Tanggal rilis efektif = ``atribut.tanggal_rilis`` bila ada; selain itu yang lebih awal
    antara asumsi konservatif dan crawl pertama (nilai pasti sudah terbit saat ter-crawl).
    Backfill ter-crawl jauh setelah asumsi konservatif sehingga tetap memakai asumsi itu.
    """
    bagian: list[pd.DataFrame] = []
    proyeksi = {
        "_id": 0, "sumber": 1, "dataset": 1, "indikator": 1, "kabupaten_kode": 1, "periode": 1, "nilai": 1, "satuan": 1,
        "frekuensi": 1, "is_carried_forward": 1, "last_available_date": 1, "crawled_at": 1, "source_status": 1,
        "atribut.tanggal_rilis": 1,
    }
    for koleksi in K_MAKRO:
        filter_q = {"crawled_at": {"$lte": cutoff}, "nilai": {"$ne": None}, "source_status": {"$ne": "ERROR"}, "periode": {"$lte": tgl}}
        dok = list(db[koleksi].find(filter_q, proyeksi).batch_size(5000))
        if not dok:
            continue
        rilis_eks = [_rilis_atribut(x.pop("atribut", None)) for x in dok]
        d = _ke_frame(dok)
        d["rilis_eksplisit"] = pd.to_datetime(pd.Series(rilis_eks, dtype="object"), errors="coerce")
        d["domain"] = [domain_makro(koleksi, str(i), str(s)) for i, s in zip(d["indikator"], d.get("dataset", ""))]
        bagian.append(d)
    if not bagian:
        kosong = pd.DataFrame(columns=_KOLOM_MAKRO)
        return kosong, kosong.copy()
    d = pd.concat(bagian, ignore_index=True).rename(columns={"kabupaten_kode": "wilayah_kode"})
    d["wilayah_kode"] = d["wilayah_kode"].where(d["wilayah_kode"].notna(), None)
    d["frekuensi"] = d["frekuensi"].fillna("BULANAN")
    kunci = ["domain", "indikator", "wilayah_kode"]

    asli = d[~d["is_carried_forward"].fillna(False).astype(bool)].copy()
    if not asli.empty:
        pertama = asli.groupby([*kunci, "periode"], dropna=False)["crawled_at"].transform("min")
        konservatif = tanggal_rilis_konservatif(asli["periode"], asli["frekuensi"])
        asli["tanggal_rilis"] = asli["rilis_eksplisit"].fillna(pd.concat([konservatif, pertama.dt.normalize()], axis=1).min(axis=1))
        asli = asli.sort_values("crawled_at").drop_duplicates([*kunci, "periode"], keep="last")
    else:
        asli["tanggal_rilis"] = pd.NaT

    # Baris terkini per indikator untuk koleksi hasil: titik asli terakhir, ditandai bila sumbernya meneruskan.
    terkini = _ringkas_makro(asli, d, kunci)
    return asli.reindex(columns=_KOLOM_MAKRO).reset_index(drop=True), terkini


def _ringkas_makro(asli: pd.DataFrame, semua: pd.DataFrame, kunci: list[str]) -> pd.DataFrame:
    if asli.empty:
        return pd.DataFrame(columns=[*_KOLOM_MAKRO, "perubahan"])
    urut = asli.sort_values("periode")
    urut = urut.assign(perubahan=urut.groupby(kunci, dropna=False)["nilai"].diff())
    terakhir = urut.drop_duplicates(kunci, keep="last").copy()
    terakhir["last_available_date"] = terakhir["periode"]
    terakhir["is_carried_forward"] = False
    cf = semua[semua["is_carried_forward"].fillna(False).astype(bool)]
    if not cf.empty:
        cf_terbaru = cf.sort_values("crawled_at").drop_duplicates(kunci, keep="last")[[*kunci, "crawled_at"]].rename(columns={"crawled_at": "cf_at"})
        asli_at = urut.groupby(kunci, dropna=False)["crawled_at"].max().reset_index().rename(columns={"crawled_at": "asli_at"})
        tanda = cf_terbaru.merge(asli_at, on=kunci, how="left")
        tanda = tanda[tanda["cf_at"] > tanda["asli_at"]][kunci].assign(_cf=True)
        terakhir = terakhir.merge(tanda, on=kunci, how="left")
        terakhir["is_carried_forward"] = terakhir["_cf"].eq(True)
        terakhir = terakhir.drop(columns="_cf")
    return terakhir.reindex(columns=[*_KOLOM_MAKRO, "perubahan"]).reset_index(drop=True)


def baca_kesegaran(db: Database, cutoff: datetime) -> list[dict]:
    """Status sumber terakhir per (sumber, dataset) dari ``crawl_runs`` non-backfill."""
    tgl = pd.Timestamp(tanggal_wib(cutoff))
    pipa = [
        {"$match": {"mulai": {"$lte": cutoff}, "pemicu": {"$ne": "BACKFILL"}}},
        {"$sort": {"mulai": -1}},
        {"$group": {
            "_id": {"s": "$sumber", "d": "$dataset"}, "status": {"$first": "$status"},
            "lad": {"$first": "$last_available_date"}, "crawled_at": {"$first": "$selesai"},
            "mulai": {"$first": "$mulai"}, "pesan": {"$first": "$pesan"},
            "lad_maks": {"$max": "$last_available_date"},
        }},
        {"$sort": {"_id.s": 1, "_id.d": 1}},
    ]
    hasil = []
    for x in db[K_RUNS].aggregate(pipa):
        lad = x.get("lad") or x.get("lad_maks")
        umur = None if lad is None else int((tgl - _naif(lad).normalize()).days)
        hasil.append({
            "sumber": x["_id"]["s"], "dataset": x["_id"]["d"], "status": x.get("status"),
            "last_available_date": utc(lad) if lad else None,
            "crawled_at": utc(x.get("crawled_at") or x["mulai"]), "umur_hari": umur, "pesan": x.get("pesan") or "",
        })
    return hasil


def ekstrak(db: Database, cutoff: datetime, hari_riwayat: int = 1100, cutoff_sebelumnya: datetime | None = None) -> DataMentah:
    """Baca semua sumber hingga ``cutoff`` (inklusif)."""
    cutoff = utc(cutoff)
    tgl = tengah_malam_utc(tanggal_wib(cutoff))
    batas = tgl - timedelta(days=hari_riwayat)
    mentah, nama, asli_terakhir = _baca_harga(db, cutoff, tgl, batas)
    harga = gabung_harga(mentah)
    makro, terkini = _baca_makro(db, cutoff, tgl)
    jumlah_baru = 0
    if cutoff_sebelumnya is not None:
        filter_baru = {"is_carried_forward": False, "nilai": {"$ne": None}, "crawled_at": {"$gt": utc(cutoff_sebelumnya), "$lte": cutoff}}
        for k in (K_HARGA, *K_MAKRO):
            jumlah_baru += db[k].count_documents(filter_baru)
        jumlah_baru += db[K_IKLIM].count_documents({**filter_baru, "atribut.jenis_data": {"$ne": "PRAKIRAAN"}})
    elif not mentah.empty:
        jumlah_baru = len(mentah)
    return DataMentah(
        harga=harga, cuaca=_baca_cuaca(db, cutoff, tgl, batas), makro=makro, makro_terkini=terkini,
        kesegaran=baca_kesegaran(db, cutoff), nama_komoditas=nama, tanggal_asli_terakhir=asli_terakhir,
        jumlah_baru=jumlah_baru,
    )
