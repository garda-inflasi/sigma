"""Menjalankan adapter per dataset. Kegagalan satu dataset tidak menghentikan yang lain."""

import logging
from collections import Counter
from collections.abc import Sequence
from dataclasses import dataclass, field
from datetime import date, datetime
from typing import Any

from garda_crawler import __version__
from garda_crawler.http_sopan import KlienSopan
from garda_crawler.model import AWALAN_DILEWATI, ERROR, FRESH, PEMICU_BACKFILL, STALE, DefinisiDataset, Seri
from garda_crawler.penyimpanan import KonteksRun, Penyimpan
from garda_crawler.sumber.dasar import Adapter, TidakMendukungBackfill
from garda_crawler.waktu import sekarang_utc, tanggal_wib

log = logging.getLogger(__name__)

BATAS_PESAN = 1500


@dataclass
class RingkasanDataset:
    sumber: str
    dataset: str
    status: str
    jumlah_baru: int = 0
    jumlah_diteruskan: int = 0
    jumlah_error: int = 0
    ditolak: int = 0
    pesan: str = ""
    extra: dict[str, Any] = field(default_factory=dict)


def _id_run(ktx_tanggal: date, sumber: str, dataset: str, pemicu: str, dari: date | None, sampai: date | None) -> str:
    dasar = f"{ktx_tanggal.isoformat()}:{sumber}:{dataset}:{pemicu}"
    if pemicu == PEMICU_BACKFILL:
        return f"{dasar}:{dari}:{sampai}"
    return dasar


def _susun_pesan(
    galat: Sequence[str], ditolak: Counter, dilewati: str | None, permintaan: int, gagal: int, wilayah: set[str] | None
) -> str:
    bagian: list[str] = []
    if dilewati:
        bagian.append(dilewati)
    if permintaan:
        bagian.append(f"permintaan {permintaan}, gagal {gagal}")
    if wilayah:
        bagian.append("wilayah dibatasi: " + ",".join(sorted(wilayah)))
    if ditolak:
        bagian.append(f"ditolak {sum(ditolak.values())} (" + ", ".join(f"{k}={v}" for k, v in sorted(ditolak.items())) + ")")
    if galat:
        contoh = "; ".join(galat[:5])
        lebih = f" (+{len(galat) - 5} lainnya)" if len(galat) > 5 else ""
        bagian.append(f"galat: {contoh}{lebih}")
    return " | ".join(bagian)[:BATAS_PESAN]


def jalankan_dataset(
    adapter: Adapter,
    ds: DefinisiDataset,
    klien: KlienSopan,
    penyimpan: Penyimpan,
    sekarang: datetime,
    pemicu: str,
    dari: date | None = None,
    sampai: date | None = None,
) -> RingkasanDataset:
    tanggal = tanggal_wib(sekarang)
    backfill = pemicu == PEMICU_BACKFILL
    run_id = _id_run(tanggal, adapter.sumber, ds.nama, pemicu, dari, sampai)
    ktx = KonteksRun(run_id=run_id, crawl_date=tanggal, crawled_at=sekarang, backfill=backfill)

    ditolak: Counter = Counter()
    galat: list[str] = []
    seri_gagal: list[Seri] = []
    dilewati: str | None = None
    permintaan = permintaan_gagal = ditulis = 0
    gagal_total = False

    try:
        potongan = (
            adapter.ambil_backfill(ds, klien, dari, sampai) if backfill else adapter.ambil_harian(ds, klien, tanggal)
        )
        for hasil in potongan:
            # Ditulis per potongan agar backfill yang terputus tidak kehilangan kemajuan.
            ditulis += penyimpan.simpan_titik(ds.koleksi, hasil.titik, ktx)[0]
            ditolak.update(hasil.ditolak)
            galat.extend(hasil.galat)
            seri_gagal.extend(hasil.seri_gagal)
            permintaan += hasil.permintaan
            permintaan_gagal += hasil.permintaan_gagal
            dilewati = dilewati or hasil.dilewati
            if backfill:
                log.info("%s/%s: tertulis %d", adapter.sumber, ds.nama, ditulis)
    except TidakMendukungBackfill as exc:
        dilewati = f"{AWALAN_DILEWATI}{exc}"
    except Exception as exc:  # noqa: BLE001 - batas isolasi antarsumber
        gagal_total = True
        galat.append(klien.samarkan(f"{type(exc).__name__}: {exc}"))
        seri_gagal = adapter.seri_diharapkan(ds)
        log.error("%s/%s gagal: %s", adapter.sumber, ds.nama, type(exc).__name__)

    if dilewati or (permintaan and permintaan_gagal == permintaan):
        gagal_total = True

    jumlah_cf = jumlah_error = 0
    if backfill:
        jumlah_baru = ditulis
    else:
        wilayah = getattr(adapter, "wilayah", None)
        jumlah_cf, jumlah_error = penyimpan.tulis_diteruskan(
            ds.koleksi, adapter.sumber, ds.nama, ktx, seri_gagal, wilayah
        )
        jumlah_baru, jumlah_cf = penyimpan.hitung_hari_ini(ds.koleksi, adapter.sumber, ds.nama, ktx)

    if dilewati:
        status = ERROR  # Kontrak tidak punya status "dilewati"; alasannya ada di awalan pesan.
    elif jumlah_baru > 0:
        status = FRESH
    elif gagal_total or permintaan_gagal > 0:
        status = ERROR
    else:
        status = STALE

    pesan = klien.samarkan(
        _susun_pesan(galat, ditolak, dilewati, permintaan, permintaan_gagal, getattr(adapter, "wilayah", None))
    )
    penyimpan.catat_run(
        {
            "_id": run_id,
            "crawler": f"garda-crawler/{__version__}",
            "sumber": adapter.sumber,
            "dataset": ds.nama,
            "crawl_date": ktx.crawl_date_teks,
            "mulai": sekarang,
            "selesai": sekarang_utc(),
            "status": status,
            "jumlah_baru": jumlah_baru,
            "jumlah_diteruskan": jumlah_cf,
            "last_available_date": penyimpan.tanggal_tersedia_terakhir(ds.koleksi, adapter.sumber, ds.nama),
            "pesan": pesan,
            "pemicu": pemicu,
        }
    )
    return RingkasanDataset(
        adapter.sumber, ds.nama, status, jumlah_baru, jumlah_cf, jumlah_error, sum(ditolak.values()), pesan
    )


def jalankan_adapter(
    adapter: Adapter,
    klien: KlienSopan,
    penyimpan: Penyimpan,
    pemicu: str,
    sekarang: datetime | None = None,
    dari: date | None = None,
    sampai: date | None = None,
) -> list[RingkasanDataset]:
    waktu = sekarang or sekarang_utc()
    ringkasan: list[RingkasanDataset] = []
    for ds in adapter.daftar_dataset():
        try:
            ringkasan.append(jalankan_dataset(adapter, ds, klien, penyimpan, waktu, pemicu, dari, sampai))
        except Exception as exc:  # noqa: BLE001 - mis. MongoDB putus di tengah dataset
            log.error("%s/%s tidak tercatat: %s", adapter.sumber, ds.nama, type(exc).__name__)
            ringkasan.append(
                RingkasanDataset(adapter.sumber, ds.nama, ERROR, pesan=f"gagal menyimpan: {type(exc).__name__}")
            )
    return ringkasan
