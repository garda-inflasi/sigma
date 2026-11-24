"""Antarmuka adapter."""

from collections.abc import Collection, Iterable
from datetime import date

from garda_crawler.http_sopan import KlienSopan
from garda_crawler.model import DefinisiDataset, HasilAmbil, Seri


class TidakMendukungBackfill(Exception):
    """Sumber tidak menyediakan histori."""


class Adapter:
    sumber: str = ""

    def __init__(self, wilayah: Collection[str] | None = None) -> None:
        self.wilayah = set(wilayah) if wilayah else None

    def termasuk(self, kabupaten_kode: str) -> bool:
        return self.wilayah is None or kabupaten_kode in self.wilayah

    def daftar_dataset(self) -> list[DefinisiDataset]:
        raise NotImplementedError

    def seri_diharapkan(self, ds: DefinisiDataset) -> list[Seri]:
        """Deret yang semestinya ada; dipakai untuk dokumen ERROR bila sumber gagal."""
        return []

    def ambil_harian(self, ds: DefinisiDataset, klien: KlienSopan, tanggal: date) -> Iterable[HasilAmbil]:
        raise NotImplementedError

    def ambil_backfill(
        self, ds: DefinisiDataset, klien: KlienSopan, dari: date, sampai: date
    ) -> Iterable[HasilAmbil]:
        raise TidakMendukungBackfill(
            f"{self.sumber} tidak menyediakan backfill historis; tindakan: pilih sumber yang mendukung backfill"
        )
