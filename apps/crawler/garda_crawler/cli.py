"""CLI: python -m garda_crawler {run,backfill,sync-lapangan,status,worker,health}."""

import argparse
import logging
import sys
from collections.abc import Sequence
from datetime import date

from pymongo import MongoClient
from pymongo.errors import PyMongoError

from garda_crawler.http_sopan import KlienSopan
from garda_crawler.jadwal import Worker, detak_sehat
from garda_crawler.konfigurasi import GalatKonfigurasi, Konfigurasi
from garda_crawler.kontrol import jalankan_server_kontrol_latar
from garda_crawler.kunci import KunciMongo
from garda_crawler.model import PEMICU_BACKFILL, PEMICU_JADWAL, PEMICU_MANUAL
from garda_crawler.pelari import RingkasanDataset, jalankan_adapter
from garda_crawler.penyimpanan import Penyimpan
from garda_crawler.sumber import LAPANGAN, SEMUA_SUMBER, SUMBER_CRAWL, buat_adapter
from garda_crawler.sumber.lapangan import HasilSinkron, PembacaPostgres, sinkronkan
from garda_crawler.waktu import sekarang_utc, tanggal_wib

log = logging.getLogger("garda_crawler")

KELUAR_OK = 0
KELUAR_GALAT = 1
KELUAR_TERKUNCI = 3


def _tanggal(teks: str) -> date:
    try:
        return date.fromisoformat(teks)
    except ValueError as exc:
        raise argparse.ArgumentTypeError(f"tanggal harus YYYY-MM-DD: {teks}") from exc


def bangun_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="garda_crawler")
    sub = parser.add_subparsers(dest="perintah", required=True)

    run = sub.add_parser("run", help="crawl sekali")
    run.add_argument("--sumber", nargs="+", choices=SEMUA_SUMBER, default=list(SEMUA_SUMBER))
    run.add_argument("--pemicu", choices=[PEMICU_MANUAL, PEMICU_JADWAL], default=PEMICU_MANUAL)
    run.add_argument("--wilayah", nargs="+", help="batasi ke kode kab/kota BPS")

    backfill = sub.add_parser("backfill", help="isi riwayat bertahap")
    backfill.add_argument("--sumber", required=True, choices=SUMBER_CRAWL)
    backfill.add_argument("--dari", required=True, type=_tanggal)
    backfill.add_argument("--sampai", required=True, type=_tanggal)
    backfill.add_argument("--wilayah", nargs="+", help="batasi ke kode kab/kota BPS")

    sinkron = sub.add_parser("sync-lapangan", help="sinkronkan entri lapangan dari PostgreSQL")
    sinkron.add_argument("--pemicu", choices=[PEMICU_MANUAL, PEMICU_JADWAL], default=PEMICU_MANUAL)

    sub.add_parser("status", help="status crawl terakhir per dataset")
    sub.add_parser("worker", help="penjadwal harian dan per jam")
    sub.add_parser("health", help="pemeriksaan kesehatan worker")
    return parser


def _klien(cfg: Konfigurasi) -> KlienSopan:
    return KlienSopan(
        cfg.user_agent,
        jeda_ms=cfg.jeda_ms,
        timeout_detik=cfg.timeout_detik,
        percobaan=cfg.percobaan,
        rahasia=[cfg.bps_api_key],
    )


def _cetak(r: RingkasanDataset) -> None:
    print(
        f"{r.sumber}/{r.dataset}: {r.status} baru={r.jumlah_baru} diteruskan={r.jumlah_diteruskan} "
        f"error={r.jumlah_error} ditolak={r.ditolak}" + (f"\n  {r.pesan}" if r.pesan else "")
    )


def _cetak_sinkron(h: HasilSinkron) -> None:
    print(
        f"LAPANGAN/entri_harga: {h.status} ditulis={h.ditulis} dikeluarkan={h.dihapus} "
        f"diteruskan={h.diteruskan} ditolak={h.ditolak}\n  {h.pesan}"
    )


def jalankan_crawl(
    cfg: Konfigurasi,
    penyimpan: Penyimpan,
    daftar: Sequence[str],
    pemicu: str,
    wilayah: Sequence[str] | None = None,
) -> list[RingkasanDataset]:
    ringkasan: list[RingkasanDataset] = []
    with _klien(cfg) as klien:
        for sumber in daftar:
            try:
                adapter = buat_adapter(sumber, cfg, wilayah)
                ringkasan.extend(jalankan_adapter(adapter, klien, penyimpan, pemicu))
            except Exception as exc:  # noqa: BLE001 - satu sumber tidak boleh menghentikan yang lain
                log.error("%s gagal: %s", sumber, type(exc).__name__)
    return ringkasan


def jalankan_sinkron(cfg: Konfigurasi, penyimpan: Penyimpan, pemicu: str) -> HasilSinkron:
    pembaca = PembacaPostgres(cfg.postgres) if cfg.postgres else None
    return sinkronkan(pembaca, penyimpan, sekarang_utc(), pemicu)


def _perintah_run(args: argparse.Namespace, cfg: Konfigurasi, penyimpan: Penyimpan) -> int:
    crawl = [s for s in args.sumber if s != LAPANGAN]
    kode = KELUAR_OK
    if crawl:
        kunci = KunciMongo(penyimpan.db, "jadwal:harian")
        if not kunci.ambil():
            print("crawl harian sedang berjalan di instance lain", file=sys.stderr)
            kode = KELUAR_TERKUNCI
        else:
            with kunci:
                for r in jalankan_crawl(cfg, penyimpan, crawl, args.pemicu, args.wilayah):
                    _cetak(r)
    if LAPANGAN in args.sumber:
        kunci = KunciMongo(penyimpan.db, "jadwal:lapangan")
        if not kunci.ambil():
            print("sinkron lapangan sedang berjalan di instance lain", file=sys.stderr)
            kode = KELUAR_TERKUNCI
        else:
            with kunci:
                _cetak_sinkron(jalankan_sinkron(cfg, penyimpan, args.pemicu))
    return kode


def _perintah_backfill(args: argparse.Namespace, cfg: Konfigurasi, penyimpan: Penyimpan) -> int:
    if args.dari > args.sampai:
        print("--dari harus sebelum atau sama dengan --sampai", file=sys.stderr)
        return KELUAR_GALAT
    hari_ini = tanggal_wib(sekarang_utc())
    if args.sampai > hari_ini:
        print("--sampai tidak boleh melewati hari ini", file=sys.stderr)
        return KELUAR_GALAT
    kunci = KunciMongo(penyimpan.db, f"backfill:{args.sumber}")
    if not kunci.ambil():
        print(f"backfill {args.sumber} sedang berjalan di instance lain", file=sys.stderr)
        return KELUAR_TERKUNCI
    with kunci, _klien(cfg) as klien:
        adapter = buat_adapter(args.sumber, cfg, args.wilayah)
        for r in jalankan_adapter(
            adapter, klien, penyimpan, PEMICU_BACKFILL, dari=args.dari, sampai=args.sampai
        ):
            _cetak(r)
    return KELUAR_OK


def _perintah_sinkron(args: argparse.Namespace, cfg: Konfigurasi, penyimpan: Penyimpan) -> int:
    kunci = KunciMongo(penyimpan.db, "jadwal:lapangan")
    if not kunci.ambil():
        print("sinkron lapangan sedang berjalan di instance lain", file=sys.stderr)
        return KELUAR_TERKUNCI
    with kunci:
        _cetak_sinkron(jalankan_sinkron(cfg, penyimpan, args.pemicu))
    return KELUAR_OK


def _perintah_status(penyimpan: Penyimpan) -> int:
    pipa = [
        {"$sort": {"selesai": -1}},
        {"$group": {"_id": {"sumber": "$sumber", "dataset": "$dataset"}, "run": {"$first": "$$ROOT"}}},
        {"$sort": {"_id.sumber": 1, "_id.dataset": 1}},
    ]
    baris = list(penyimpan.db["crawl_runs"].aggregate(pipa))
    if not baris:
        print("belum ada crawl_runs")
        return KELUAR_OK
    for b in baris:
        r = b["run"]
        terakhir = r.get("last_available_date")
        print(
            f"{r['sumber']}/{r['dataset']}: {r['status']} crawl={r['crawl_date']} pemicu={r['pemicu']} "
            f"baru={r['jumlah_baru']} diteruskan={r['jumlah_diteruskan']} "
            f"data_terakhir={terakhir.date().isoformat() if terakhir else '-'}"
        )
        if r.get("pesan"):
            print(f"  {r['pesan']}")
    return KELUAR_OK


def _perintah_worker(cfg: Konfigurasi, penyimpan: Penyimpan) -> int:
    def harian() -> None:
        for r in jalankan_crawl(cfg, penyimpan, SUMBER_CRAWL, PEMICU_JADWAL):
            log.info("%s/%s: %s baru=%d", r.sumber, r.dataset, r.status, r.jumlah_baru)

    def lapangan() -> None:
        h = jalankan_sinkron(cfg, penyimpan, PEMICU_JADWAL)
        log.info("LAPANGAN: %s ditulis=%d", h.status, h.ditulis)

    def manual(daftar_sumber: list[str]) -> list[dict]:
        ringkas: list[dict] = []
        luar = [s for s in daftar_sumber if s in SUMBER_CRAWL]
        if luar:
            for r in jalankan_crawl(cfg, penyimpan, luar, PEMICU_MANUAL):
                ringkas.append({"sumber": r.sumber, "dataset": r.dataset, "status": r.status, "jumlah_baru": r.jumlah_baru})
        if LAPANGAN in daftar_sumber:
            h = jalankan_sinkron(cfg, penyimpan, PEMICU_MANUAL)
            ringkas.append({"sumber": LAPANGAN, "dataset": "harga_lapangan", "status": h.status, "jumlah_baru": h.ditulis})
        return ringkas

    srv, _ = jalankan_server_kontrol_latar(penyimpan.db, manual)
    try:
        Worker(penyimpan.db, harian, lapangan, cfg.berkas_detak).jalan()
    finally:
        srv.shutdown()
        srv.server_close()
    return KELUAR_OK


def main(argv: Sequence[str] | None = None) -> int:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
    # Log permintaan httpx memuat URL lengkap, termasuk kunci API BPS di jalur.
    logging.getLogger("httpx").setLevel(logging.WARNING)
    logging.getLogger("httpcore").setLevel(logging.WARNING)
    logging.getLogger("pymongo").setLevel(logging.WARNING)

    args = bangun_parser().parse_args(argv)
    try:
        cfg = Konfigurasi.dari_env()
    except GalatKonfigurasi as exc:
        print(f"konfigurasi tidak sah: {exc}", file=sys.stderr)
        return KELUAR_GALAT

    if args.perintah == "health":
        return KELUAR_OK if detak_sehat(cfg.berkas_detak) else KELUAR_GALAT

    klien_mongo: MongoClient = MongoClient(cfg.mongo_uri, serverSelectionTimeoutMS=10_000, tz_aware=True)
    try:
        klien_mongo.admin.command("ping")
        penyimpan = Penyimpan(klien_mongo[cfg.db_unseen])
        if args.perintah != "status":
            penyimpan.siapkan_indeks()
        if args.perintah == "run":
            return _perintah_run(args, cfg, penyimpan)
        if args.perintah == "backfill":
            return _perintah_backfill(args, cfg, penyimpan)
        if args.perintah == "sync-lapangan":
            return _perintah_sinkron(args, cfg, penyimpan)
        if args.perintah == "status":
            return _perintah_status(penyimpan)
        return _perintah_worker(cfg, penyimpan)
    except PyMongoError as exc:
        print(f"MongoDB gagal: {type(exc).__name__}", file=sys.stderr)
        return KELUAR_GALAT
    finally:
        klien_mongo.close()
