"""Aturan waktu kontrak: UTC di penyimpanan, tanggal bisnis menurut kalender WIB.

WIB memakai offset tetap (tanpa DST) agar tidak bergantung pada paket tzdata di image.
"""

from collections.abc import Iterator
from datetime import UTC, date, datetime, timedelta, timezone

WIB = timezone(timedelta(hours=7), "WIB")


def sekarang_utc() -> datetime:
    return datetime.now(UTC)


def tanggal_wib(waktu: datetime) -> date:
    return waktu.astimezone(WIB).date()


def tengah_malam_utc(tanggal: date) -> datetime:
    """Tanggal bisnis: tengah malam UTC dari tanggal kalender WIB."""
    return datetime(tanggal.year, tanggal.month, tanggal.day, tzinfo=UTC)


def pastikan_utc(waktu: datetime) -> datetime:
    """MongoDB mengembalikan waktu naif bila klien tidak tz_aware; nilainya selalu UTC."""
    if waktu.tzinfo is None:
        return waktu.replace(tzinfo=UTC)
    return waktu.astimezone(UTC)


def potong_rentang(dari: date, sampai: date, hari: int) -> Iterator[tuple[date, date]]:
    """Membagi rentang inklusif menjadi potongan paling panjang `hari` hari."""
    awal = dari
    while awal <= sampai:
        akhir = min(awal + timedelta(days=hari - 1), sampai)
        yield awal, akhir
        awal = akhir + timedelta(days=1)
