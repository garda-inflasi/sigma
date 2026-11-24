"""Penulisan ke MongoDB dataunseen sesuai kontrak docs/PIPELINE-DATA-ML.md.

Selain koleksi kontrak, crawler memakai `crawl_kunci` (kunci antar-instance) dan
`crawl_jadwal` (penanda jadwal terakhir). Keduanya internal dan tidak dibaca ML.
"""

import math
from collections.abc import Iterable, Sequence
from dataclasses import dataclass
from datetime import date, datetime
from typing import Any

from pymongo import ASCENDING, DESCENDING, ReplaceOne
from pymongo.database import Database

from garda_crawler.model import ERROR, FRESH, KOLEKSI_RAW, STALE, Seri, Titik
from garda_crawler.waktu import pastikan_utc, tengah_malam_utc

UKURAN_BATCH = 1000
UKURAN_IN = 500


@dataclass(frozen=True)
class KonteksRun:
    run_id: str
    crawl_date: date
    crawled_at: datetime
    backfill: bool = False

    @property
    def crawl_date_teks(self) -> str:
        return self.crawl_date.isoformat()


def id_dokumen(sumber: str, dataset: str, kunci_seri: str, periode: date, crawl_date: date) -> str:
    return f"{sumber}:{dataset}:{kunci_seri}:{periode.isoformat()}:{crawl_date.isoformat()}"


def _potong(daftar: Sequence[Any], ukuran: int) -> Iterable[Sequence[Any]]:
    for i in range(0, len(daftar), ukuran):
        yield daftar[i : i + ukuran]


class Penyimpan:
    def __init__(self, db: Database) -> None:
        self.db = db

    def siapkan_indeks(self) -> None:
        for nama in KOLEKSI_RAW:
            kol = self.db[nama]
            kol.create_index([("kunci_seri", ASCENDING), ("periode", ASCENDING)])
            kol.create_index([("sumber", ASCENDING), ("dataset", ASCENDING), ("crawl_date", ASCENDING)])
            kol.create_index([("komoditas_kode", ASCENDING), ("kabupaten_kode", ASCENDING), ("periode", ASCENDING)])
            kol.create_index([("source_status", ASCENDING), ("crawl_date", ASCENDING)])
            # Tambahan di luar kontrak: mempercepat pencarian nilai terakhir tiap deret.
            kol.create_index(
                [
                    ("sumber", ASCENDING),
                    ("dataset", ASCENDING),
                    ("kunci_seri", ASCENDING),
                    ("periode", DESCENDING),
                    ("crawled_at", DESCENDING),
                ]
            )
        self.db["crawl_runs"].create_index([("sumber", ASCENDING), ("dataset", ASCENDING), ("mulai", DESCENDING)])

    def dokumen_titik(self, t: Titik, ktx: KonteksRun) -> dict[str, Any]:
        atribut = dict(t.atribut)
        if ktx.backfill:
            atribut["backfill"] = True
        periode = tengah_malam_utc(t.periode)
        return {
            "_id": id_dokumen(t.seri.sumber, t.seri.dataset, t.seri.kunci_seri, t.periode, ktx.crawl_date),
            "sumber": t.seri.sumber,
            "dataset": t.seri.dataset,
            "kunci_seri": t.seri.kunci_seri,
            "indikator": t.seri.indikator,
            "komoditas_kode": t.seri.komoditas_kode,
            "kabupaten_kode": t.seri.kabupaten_kode,
            "satuan": t.seri.satuan,
            "frekuensi": t.seri.frekuensi,
            "periode": periode,
            "nilai": t.nilai,
            "is_carried_forward": False,
            "last_available_date": periode,
            "crawled_at": ktx.crawled_at,
            "crawl_date": ktx.crawl_date_teks,
            "source_status": FRESH,
            "run_id": ktx.run_id,
            "url_sumber": t.url_sumber,
            "status_validasi": None,
            "atribut": atribut,
        }

    def _nilai_terakhir(self, koleksi: str, sumber: str, dataset: str, titik: Sequence[Titik]) -> dict[tuple[str, datetime], float]:
        """Nilai vintage terbaru per (kunci_seri, periode) untuk titik yang akan ditulis."""
        hasil: dict[tuple[str, datetime], float] = {}
        kunci = sorted({t.seri.kunci_seri for t in titik})
        awal = tengah_malam_utc(min(t.periode for t in titik))
        akhir = tengah_malam_utc(max(t.periode for t in titik))
        for potongan in _potong(kunci, UKURAN_IN):
            kursor = (
                self.db[koleksi]
                .find(
                    {
                        "sumber": sumber,
                        "dataset": dataset,
                        "kunci_seri": {"$in": list(potongan)},
                        "periode": {"$gte": awal, "$lte": akhir},
                        "is_carried_forward": False,
                        "nilai": {"$ne": None},
                    },
                    {"kunci_seri": 1, "periode": 1, "nilai": 1, "crawled_at": 1},
                )
                .sort("crawled_at", ASCENDING)
            )
            for d in kursor:
                hasil[(d["kunci_seri"], pastikan_utc(d["periode"]))] = d["nilai"]
        return hasil

    def simpan_titik(self, koleksi: str, titik: Sequence[Titik], ktx: KonteksRun) -> tuple[int, int]:
        """Menulis titik yang baru atau berubah; mengembalikan (ditulis, tidak_berubah)."""
        if not titik:
            return 0, 0
        unik = {(t.seri.kunci_seri, t.periode): t for t in titik}
        contoh = titik[0].seri
        lama = self._nilai_terakhir(koleksi, contoh.sumber, contoh.dataset, list(unik.values()))

        operasi: list[ReplaceOne] = []
        tidak_berubah = 0
        for (kunci, periode), t in unik.items():
            nilai_lama = lama.get((kunci, tengah_malam_utc(periode)))
            if nilai_lama is not None and math.isclose(nilai_lama, t.nilai, abs_tol=1e-9):
                tidak_berubah += 1
                continue
            doc = self.dokumen_titik(t, ktx)
            operasi.append(ReplaceOne({"_id": doc["_id"]}, doc, upsert=True))
        for batch in _potong(operasi, UKURAN_BATCH):
            self.db[koleksi].bulk_write(list(batch), ordered=False)
        return len(operasi), tidak_berubah

    def riwayat_deret(self, koleksi: str, sumber: str, dataset: str) -> dict[str, dict[str, Any]]:
        """Dokumen asli terakhir per kunci_seri."""
        pipa = [
            {"$match": {"sumber": sumber, "dataset": dataset, "is_carried_forward": False, "nilai": {"$ne": None}}},
            {"$sort": {"kunci_seri": 1, "periode": -1, "crawled_at": -1}},
            {"$group": {"_id": "$kunci_seri", "doc": {"$first": "$$ROOT"}}},
        ]
        return {r["_id"]: r["doc"] for r in self.db[koleksi].aggregate(pipa)}

    def _kunci_segar(self, koleksi: str, sumber: str, dataset: str, ktx: KonteksRun) -> set[str]:
        """Deret yang punya rilis baru pada tanggal crawl ini.

        Dokumen backfill tidak dihitung karena bukan rilis baru, kecuali periodenya hari ini:
        _id-nya sama dengan dokumen carried-forward hari itu dan tidak boleh tertimpa.
        """
        kursor = self.db[koleksi].find(
            {
                "sumber": sumber,
                "dataset": dataset,
                "crawl_date": ktx.crawl_date_teks,
                "source_status": FRESH,
                "is_carried_forward": False,
                "$or": [{"atribut.backfill": {"$ne": True}}, {"periode": tengah_malam_utc(ktx.crawl_date)}],
            },
            {"kunci_seri": 1},
        )
        return {d["kunci_seri"] for d in kursor}

    def tulis_diteruskan(
        self,
        koleksi: str,
        sumber: str,
        dataset: str,
        ktx: KonteksRun,
        seri_gagal: Sequence[Seri] = (),
        wilayah: set[str] | None = None,
    ) -> tuple[int, int]:
        """Menulis carried-forward untuk deret tanpa rilis baru dan ERROR untuk deret gagal tanpa riwayat."""
        riwayat = self.riwayat_deret(koleksi, sumber, dataset)
        segar = self._kunci_segar(koleksi, sumber, dataset, ktx)
        periode = tengah_malam_utc(ktx.crawl_date)
        operasi: list[ReplaceOne] = []

        jumlah_cf = 0
        for kunci, asal in riwayat.items():
            if kunci in segar or (wilayah is not None and asal.get("kabupaten_kode") not in wilayah):
                continue
            doc = self._dokumen_diteruskan(asal, kunci, periode, ktx)
            operasi.append(ReplaceOne({"_id": doc["_id"]}, doc, upsert=True))
            jumlah_cf += 1

        kunci_gagal: set[str] = set()
        jumlah_error = 0
        for s in seri_gagal:
            if s.kunci_seri in riwayat or s.kunci_seri in kunci_gagal:
                continue
            kunci_gagal.add(s.kunci_seri)
            doc = self._dokumen_error(s, periode, ktx)
            operasi.append(ReplaceOne({"_id": doc["_id"]}, doc, upsert=True))
            jumlah_error += 1

        for batch in _potong(operasi, UKURAN_BATCH):
            self.db[koleksi].bulk_write(list(batch), ordered=False)

        filter_dasar = {"sumber": sumber, "dataset": dataset, "crawl_date": ktx.crawl_date_teks}
        for potongan in _potong(sorted(segar), UKURAN_IN):
            self.db[koleksi].delete_many(
                {**filter_dasar, "kunci_seri": {"$in": list(potongan)}, "source_status": {"$in": [STALE, ERROR]}}
            )
        # ERROR dari percobaan sebelumnya hari ini tidak berlaku lagi bila deretnya tidak lagi gagal.
        filter_error: dict[str, Any] = {**filter_dasar, "source_status": ERROR, "kunci_seri": {"$nin": sorted(kunci_gagal)}}
        if wilayah is not None:
            filter_error["kabupaten_kode"] = {"$in": sorted(wilayah)}
        self.db[koleksi].delete_many(filter_error)
        return jumlah_cf, jumlah_error

    @staticmethod
    def _dokumen_diteruskan(asal: dict[str, Any], kunci: str, periode: datetime, ktx: KonteksRun) -> dict[str, Any]:
        tanggal = periode.date()
        return {
            "_id": id_dokumen(asal["sumber"], asal["dataset"], kunci, tanggal, ktx.crawl_date),
            "sumber": asal["sumber"],
            "dataset": asal["dataset"],
            "kunci_seri": kunci,
            "indikator": asal["indikator"],
            "komoditas_kode": asal.get("komoditas_kode"),
            "kabupaten_kode": asal.get("kabupaten_kode"),
            "satuan": asal["satuan"],
            "frekuensi": asal["frekuensi"],
            "periode": periode,
            "nilai": asal["nilai"],
            "is_carried_forward": True,
            "last_available_date": pastikan_utc(asal["periode"]),
            "crawled_at": ktx.crawled_at,
            "crawl_date": ktx.crawl_date_teks,
            "source_status": STALE,
            "run_id": ktx.run_id,
            "url_sumber": asal.get("url_sumber", ""),
            "status_validasi": asal.get("status_validasi"),
            "atribut": {"id_asal": asal["_id"]},
        }

    @staticmethod
    def _dokumen_error(s: Seri, periode: datetime, ktx: KonteksRun) -> dict[str, Any]:
        return {
            "_id": id_dokumen(s.sumber, s.dataset, s.kunci_seri, periode.date(), ktx.crawl_date),
            "sumber": s.sumber,
            "dataset": s.dataset,
            "kunci_seri": s.kunci_seri,
            "indikator": s.indikator,
            "komoditas_kode": s.komoditas_kode,
            "kabupaten_kode": s.kabupaten_kode,
            "satuan": s.satuan,
            "frekuensi": s.frekuensi,
            "periode": periode,
            "nilai": None,
            "is_carried_forward": False,
            "last_available_date": None,
            "crawled_at": ktx.crawled_at,
            "crawl_date": ktx.crawl_date_teks,
            "source_status": ERROR,
            "run_id": ktx.run_id,
            "url_sumber": s.url_sumber,
            "status_validasi": None,
            "atribut": {},
        }

    def hitung_hari_ini(self, koleksi: str, sumber: str, dataset: str, ktx: KonteksRun) -> tuple[int, int]:
        """(dokumen asli baru hari ini tanpa backfill, dokumen carried-forward hari ini)."""
        dasar = {"sumber": sumber, "dataset": dataset, "crawl_date": ktx.crawl_date_teks}
        baru = self.db[koleksi].count_documents(
            {**dasar, "source_status": FRESH, "is_carried_forward": False, "atribut.backfill": {"$ne": True}}
        )
        diteruskan = self.db[koleksi].count_documents({**dasar, "is_carried_forward": True})
        return baru, diteruskan

    def tanggal_tersedia_terakhir(self, koleksi: str, sumber: str, dataset: str) -> datetime | None:
        doc = self.db[koleksi].find_one(
            {"sumber": sumber, "dataset": dataset, "is_carried_forward": False, "nilai": {"$ne": None}},
            {"periode": 1},
            sort=[("periode", DESCENDING)],
        )
        return pastikan_utc(doc["periode"]) if doc else None

    def catat_run(self, doc: dict[str, Any]) -> None:
        self.db["crawl_runs"].replace_one({"_id": doc["_id"]}, doc, upsert=True)
