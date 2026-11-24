"""Sumber pendukung yang saat ini dilewati dengan alasan tercatat.

Keduanya diperiksa 2026-11-24. Tidak ada upaya memakai token dari bundel web, melewati reCAPTCHA,
atau mengandalkan endpoint yang lolos dari gerbang kunci API.
"""

from collections.abc import Iterable
from datetime import date

from garda_crawler.http_sopan import KlienSopan
from garda_crawler.model import DefinisiDataset, HasilAmbil, pesan_dilewati
from garda_crawler.sumber.dasar import Adapter

ALASAN_SP2KP = pesan_dilewati(
    "API SP2KP (api-sp2kp.kemendag.go.id) memakai token sesi dan reCAPTCHA, tanpa endpoint publik terdokumentasi",
    "minta akses API resmi ke Kemendag",
)
ALASAN_BAPANAS = pesan_dilewati(
    "API Panel Harga (api-panelhargav2.badanpangan.go.id) mensyaratkan API key (401 'Invalid or missing API key')",
    "minta API key resmi ke Bapanas",
)


class _AdapterDilewati(Adapter):
    alasan = ""
    dataset = DefinisiDataset("", "raw_pangan_harian")

    def daftar_dataset(self) -> list[DefinisiDataset]:
        return [self.dataset]

    def ambil_harian(self, ds: DefinisiDataset, klien: KlienSopan, tanggal: date) -> Iterable[HasilAmbil]:
        yield HasilAmbil(dilewati=self.alasan)

    def ambil_backfill(
        self, ds: DefinisiDataset, klien: KlienSopan, dari: date, sampai: date
    ) -> Iterable[HasilAmbil]:
        yield HasilAmbil(dilewati=self.alasan)


class AdapterSP2KP(_AdapterDilewati):
    sumber = "SP2KP"
    alasan = ALASAN_SP2KP
    dataset = DefinisiDataset("harga_kemendag", "raw_pangan_harian")


class AdapterBapanas(_AdapterDilewati):
    sumber = "BAPANAS"
    alasan = ALASAN_BAPANAS
    dataset = DefinisiDataset("panel_harga", "raw_pangan_harian")
