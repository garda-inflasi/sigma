"""Sinkronisasi baca-saja entri_harga PostgreSQL ke raw_pangan_harian (sumber LAPANGAN).

- Satu dokumen per entri, diperbarui di tempat, sehingga pembaca selalu melihat status terkini.
  `crawl_date` pada `_id` adalah tanggal sinkronisasi pertama entri itu.
- `kunci_seri` memuat id entri karena satu deret punya banyak entri per hari. Identitas deret ada di
  `atribut.deret` (`KOMODITAS:KABUPATEN:TIPE`).
- Inkremental: membandingkan (versi, periode) dari entri_harga_versi, yang ditulis pemicu pada setiap
  perubahan isi. `diupdate_pada` tidak dipakai karena tidak selalu ikut berubah.
- Entri yang mundur ke DRAF, ditolak, atau terhapus dikeluarkan beserta carried-forward turunannya.
- Baris seed demo (`kode` HRG-Dnn-...) tidak pernah ikut: kontrak melarang data rekaan di pipeline ini.
- Tanpa data pribadi: tidak membaca petugas, validator, penerbit, GPS, foto, catatan, atau kode pedagang.
"""

import logging
from collections import Counter
from collections.abc import Iterable, Sequence
from dataclasses import dataclass
from datetime import date, datetime
from typing import Any, Protocol

import psycopg
from psycopg.rows import dict_row
from pymongo import DeleteOne, ReplaceOne

from garda_crawler import __version__
from garda_crawler.model import ERROR, FRESH, STALE, pesan_dilewati
from garda_crawler.penyimpanan import Penyimpan, id_dokumen
from garda_crawler.validasi import periksa_harga
from garda_crawler.waktu import WIB, pastikan_utc, tanggal_wib, tengah_malam_utc

log = logging.getLogger(__name__)

SUMBER = "LAPANGAN"
DATASET = "entri_harga"
KOLEKSI = "raw_pangan_harian"
BOBOT_STATUS = {"DIAJUKAN": 1, "DIVALIDASI": 2, "DIPUBLIKASI": 3}
UKURAN_BATCH = 1000

_SYARAT = """
    e.sumber = 'LAPANGAN'
    AND e.status IN ('DIAJUKAN', 'DIVALIDASI', 'DIPUBLIKASI')
    AND (e.kode IS NULL OR e.kode !~ '^HRG-D[0-9]')
"""
_VERSI = "LEFT JOIN LATERAL (SELECT max(ev.versi) AS versi FROM entri_harga_versi ev WHERE ev.entri_id = e.id) v ON TRUE"

SQL_INDEKS = f"SELECT e.id, COALESCE(v.versi, 0) AS versi, e.dicatat_pada FROM entri_harga e {_VERSI} WHERE {_SYARAT}"
SQL_BARIS = f"""
SELECT e.id, e.komoditas_kode, e.pasar_id, p.kabupaten_kode, e.tipe::text AS tipe, e.harga, e.satuan,
       e.kualitas::text AS kualitas, e.status::text AS status, e.deviasi_pct, e.skor_anomali, e.dicatat_pada,
       k.satuan AS satuan_master, k.harga_acuan, COALESCE(v.versi, 0) AS versi
FROM entri_harga e
JOIN pasar p ON p.kode = e.pasar_id
JOIN komoditas k ON k.kode = e.komoditas_kode
{_VERSI}
WHERE e.id = ANY(%s) AND {_SYARAT}
"""
SQL_DEMO = "SELECT count(*) AS jumlah FROM entri_harga WHERE kode ~ '^HRG-D[0-9]'"


class PembacaEntri(Protocol):
    def __enter__(self) -> "PembacaEntri": ...
    def __exit__(self, *_: object) -> None: ...
    def indeks(self) -> list[dict[str, Any]]: ...
    def baris(self, ids: Sequence[int]) -> list[dict[str, Any]]: ...
    def jumlah_demo(self) -> int: ...


class PembacaPostgres:
    """Koneksi baca-saja dengan satu snapshot konsisten per sinkronisasi."""

    def __init__(self, sambungan: dict[str, str] | str) -> None:
        self.sambungan = sambungan
        self.conn: psycopg.Connection | None = None

    def __enter__(self) -> "PembacaPostgres":
        argumen: dict[str, Any] = {"connect_timeout": 10, "application_name": "garda-crawler", "row_factory": dict_row}
        if isinstance(self.sambungan, str):
            self.conn = psycopg.connect(self.sambungan, **argumen)
        else:
            self.conn = psycopg.connect(**self.sambungan, **argumen)
        self.conn.read_only = True
        self.conn.isolation_level = psycopg.IsolationLevel.REPEATABLE_READ
        return self

    def __exit__(self, *_: object) -> None:
        if self.conn is not None:
            self.conn.rollback()
            self.conn.close()

    def indeks(self) -> list[dict[str, Any]]:
        return list(self.conn.execute(SQL_INDEKS).fetchall())

    def baris(self, ids: Sequence[int]) -> list[dict[str, Any]]:
        return list(self.conn.execute(SQL_BARIS, (list(ids),)).fetchall())

    def jumlah_demo(self) -> int:
        return int(self.conn.execute(SQL_DEMO).fetchone()["jumlah"])


@dataclass
class HasilSinkron:
    run_id: str
    status: str
    ditulis: int = 0
    dihapus: int = 0
    diteruskan: int = 0
    ditolak: int = 0
    pesan: str = ""


def _potong(daftar: Sequence[Any], ukuran: int) -> Iterable[Sequence[Any]]:
    for i in range(0, len(daftar), ukuran):
        yield daftar[i : i + ukuran]


def _periode(waktu: datetime) -> date:
    return tanggal_wib(pastikan_utc(waktu))


def dokumen_entri(baris: dict[str, Any], crawl_date: str, crawled_at: datetime, run_id: str) -> dict[str, Any]:
    periode = _periode(baris["dicatat_pada"])
    satuan = str(baris["satuan_master"]).strip().lower()
    deret = f"{baris['komoditas_kode']}:{baris['kabupaten_kode']}:{baris['tipe']}"
    kunci_seri = f"{deret}:E{baris['id']}"
    tengah_malam = tengah_malam_utc(periode)
    return {
        "_id": id_dokumen(SUMBER, DATASET, kunci_seri, periode, date.fromisoformat(crawl_date)),
        "sumber": SUMBER,
        "dataset": DATASET,
        "kunci_seri": kunci_seri,
        "indikator": f"harga_{str(baris['tipe']).lower()}_{baris['komoditas_kode']}",
        "komoditas_kode": baris["komoditas_kode"],
        "kabupaten_kode": baris["kabupaten_kode"],
        "satuan": f"Rp/{satuan}",
        "frekuensi": "HARIAN",
        "periode": tengah_malam,
        "nilai": float(baris["harga"]),
        "is_carried_forward": False,
        "last_available_date": tengah_malam,
        "crawled_at": crawled_at,
        "crawl_date": crawl_date,
        "source_status": FRESH,
        "run_id": run_id,
        "url_sumber": "postgres://entri_harga",
        "status_validasi": baris["status"],
        "atribut": {
            "entri_id": int(baris["id"]),
            "versi_entri": int(baris["versi"]),
            "deret": deret,
            "pasar_id": baris["pasar_id"],
            "tipe": baris["tipe"],
            "kualitas": baris["kualitas"],
            "deviasi_pct": baris["deviasi_pct"],
            "skor_anomali": baris["skor_anomali"],
        },
    }


def _terapkan_perubahan(
    penyimpan: Penyimpan,
    pembaca: PembacaEntri,
    indeks: list[dict[str, Any]],
    crawl_date: str,
    crawled_at: datetime,
    run_id: str,
) -> tuple[int, int, Counter]:
    kol = penyimpan.db[KOLEKSI]
    ada: dict[int, dict[str, Any]] = {}
    for d in kol.find(
        {"sumber": SUMBER, "dataset": DATASET, "is_carried_forward": False},
        {"crawl_date": 1, "periode": 1, "atribut.entri_id": 1, "atribut.versi_entri": 1},
    ):
        ada[d["atribut"]["entri_id"]] = d

    sah = {int(b["id"]): b for b in indeks}
    berubah = [
        i
        for i, b in sah.items()
        if i not in ada
        or ada[i]["atribut"].get("versi_entri") != int(b["versi"])
        or pastikan_utc(ada[i]["periode"]) != tengah_malam_utc(_periode(b["dicatat_pada"]))
    ]

    ditolak: Counter = Counter()
    operasi: list[Any] = []
    keluar: set[int] = set(ada) - set(sah)
    for potongan in _potong(sorted(berubah), UKURAN_BATCH):
        for b in pembaca.baris(potongan):
            i = int(b["id"])
            acuan = float(b["harga_acuan"]) if b["harga_acuan"] is not None else None
            alasan = periksa_harga(float(b["harga"]), acuan, b["satuan"], b["satuan_master"])
            if alasan:
                ditolak[alasan] += 1
                keluar.add(i)
                continue
            lama = ada.get(i)
            doc = dokumen_entri(b, lama["crawl_date"] if lama else crawl_date, crawled_at, run_id)
            if lama and lama["_id"] != doc["_id"]:
                operasi.append(DeleteOne({"_id": lama["_id"]}))
            operasi.append(ReplaceOne({"_id": doc["_id"]}, doc, upsert=True))

    hapus = [i for i in keluar if i in ada]
    operasi.extend(DeleteOne({"_id": ada[i]["_id"]}) for i in hapus)
    for batch in _potong(operasi, UKURAN_BATCH):
        kol.bulk_write(list(batch), ordered=True)
    if keluar:
        for potongan in _potong(sorted(keluar), UKURAN_BATCH):
            kol.delete_many(
                {
                    "sumber": SUMBER,
                    "dataset": DATASET,
                    "is_carried_forward": True,
                    "atribut.entri_id_asal": {"$in": list(potongan)},
                }
            )
    ditulis = sum(isinstance(o, ReplaceOne) for o in operasi)
    return ditulis, len(hapus), ditolak


def tulis_diteruskan_lapangan(penyimpan: Penyimpan, tanggal: date, crawled_at: datetime, run_id: str) -> int:
    """Carried-forward per deret yang tidak punya entri sah pada tanggal ini."""
    kol = penyimpan.db[KOLEKSI]
    hari_ini = tengah_malam_utc(tanggal)
    teks = tanggal.isoformat()
    terbaru: dict[str, dict[str, Any]] = {}
    for d in kol.find({"sumber": SUMBER, "dataset": DATASET, "is_carried_forward": False}):
        deret = d["atribut"]["deret"]
        pembanding = (pastikan_utc(d["periode"]), BOBOT_STATUS.get(d["status_validasi"], 0), d["atribut"]["entri_id"])
        lama = terbaru.get(deret)
        if lama is None or pembanding > lama["_pembanding"]:
            terbaru[deret] = {**d, "_pembanding": pembanding}

    operasi: list[ReplaceOne] = []
    id_berlaku: list[str] = []
    for deret, d in terbaru.items():
        if pastikan_utc(d["periode"]) >= hari_ini:
            continue
        _id = id_dokumen(SUMBER, DATASET, deret, tanggal, tanggal)
        id_berlaku.append(_id)
        doc = {
            "_id": _id,
            "sumber": SUMBER,
            "dataset": DATASET,
            "kunci_seri": deret,
            "indikator": d["indikator"],
            "komoditas_kode": d["komoditas_kode"],
            "kabupaten_kode": d["kabupaten_kode"],
            "satuan": d["satuan"],
            "frekuensi": d["frekuensi"],
            "periode": hari_ini,
            "nilai": d["nilai"],
            "is_carried_forward": True,
            "last_available_date": pastikan_utc(d["periode"]),
            "crawled_at": crawled_at,
            "crawl_date": teks,
            "source_status": STALE,
            "run_id": run_id,
            "url_sumber": d["url_sumber"],
            "status_validasi": d["status_validasi"],
            "atribut": {"deret": deret, "entri_id_asal": d["atribut"]["entri_id"]},
        }
        operasi.append(ReplaceOne({"_id": _id}, doc, upsert=True))
    for batch in _potong(operasi, UKURAN_BATCH):
        kol.bulk_write(list(batch), ordered=False)
    kol.delete_many(
        {"sumber": SUMBER, "dataset": DATASET, "is_carried_forward": True, "crawl_date": teks, "_id": {"$nin": id_berlaku}}
    )
    return len(operasi)


def sinkronkan(
    pembaca: PembacaEntri | None,
    penyimpan: Penyimpan,
    sekarang: datetime,
    pemicu: str,
) -> HasilSinkron:
    tanggal = tanggal_wib(sekarang)
    crawl_date = tanggal.isoformat()
    jam = sekarang.astimezone(WIB).strftime("%H")
    run_id = f"{crawl_date}T{jam}:{SUMBER}:{DATASET}:{pemicu}"
    hasil = HasilSinkron(run_id=run_id, status=STALE)
    catatan: list[str] = []
    gagal = False

    if pembaca is None:
        gagal = True
        catatan.append(pesan_dilewati("koneksi PostgreSQL belum dikonfigurasi", "isi CRAWLER_DATABASE_URL atau DB_*"))
    else:
        try:
            with pembaca as p:
                indeks = p.indeks()
                demo = p.jumlah_demo()
                hasil.ditulis, hasil.dihapus, ditolak = _terapkan_perubahan(
                    penyimpan, p, indeks, crawl_date, sekarang, run_id
                )
            hasil.ditolak = sum(ditolak.values())
            catatan.append(f"entri sah {len(indeks)}, ditulis {hasil.ditulis}, dikeluarkan {hasil.dihapus}")
            if demo:
                catatan.append(f"{demo} entri seed demo diabaikan")
            if ditolak:
                catatan.append("ditolak " + ", ".join(f"{k}={v}" for k, v in sorted(ditolak.items())))
        except psycopg.Error as exc:
            gagal = True
            # Pesan psycopg bisa memuat host atau nama pengguna, jadi hanya jenis galat yang dicatat.
            catatan.append(f"gagal membaca PostgreSQL: {type(exc).__name__}")
            log.warning("sinkron lapangan gagal: %s", type(exc).__name__)

    hasil.diteruskan = tulis_diteruskan_lapangan(penyimpan, tanggal, sekarang, run_id)
    baru_hari_ini = penyimpan.db[KOLEKSI].count_documents(
        {
            "sumber": SUMBER,
            "dataset": DATASET,
            "is_carried_forward": False,
            "periode": tengah_malam_utc(tanggal),
        }
    )
    if gagal:
        hasil.status = ERROR
    elif hasil.ditulis > 0 or baru_hari_ini > 0:
        hasil.status = FRESH
    else:
        hasil.status = STALE
    hasil.pesan = "; ".join(catatan)

    penyimpan.catat_run(
        {
            "_id": run_id,
            "crawler": f"garda-crawler/{__version__}",
            "sumber": SUMBER,
            "dataset": DATASET,
            "crawl_date": crawl_date,
            "mulai": sekarang,
            "selesai": datetime.now(sekarang.tzinfo),
            "status": hasil.status,
            "jumlah_baru": hasil.ditulis,
            "jumlah_diteruskan": hasil.diteruskan,
            "last_available_date": penyimpan.tanggal_tersedia_terakhir(KOLEKSI, SUMBER, DATASET),
            "pesan": hasil.pesan,
            "pemicu": pemicu,
        }
    )
    return hasil
