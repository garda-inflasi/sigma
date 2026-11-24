"""Aturan waktu kontrak: siklus per jam UTC dan tanggal bisnis WIB."""

from __future__ import annotations

from datetime import date, datetime, time, timedelta, timezone

WIB = timezone(timedelta(hours=7))


def utc(nilai: datetime) -> datetime:
    """Normalkan ke UTC sadar zona. Datetime naif (keluaran pymongo) dianggap UTC."""
    if nilai.tzinfo is None:
        return nilai.replace(tzinfo=timezone.utc)
    return nilai.astimezone(timezone.utc)


def awal_jam(nilai: datetime) -> datetime:
    return utc(nilai).replace(minute=0, second=0, microsecond=0)


def id_siklus(cycle_timestamp: datetime) -> str:
    """``C{YYYYMMDD}T{HH}Z``: jam yang sama selalu menghasilkan id yang sama."""
    return awal_jam(cycle_timestamp).strftime("C%Y%m%dT%HZ")


def tanggal_wib(nilai: datetime) -> date:
    return utc(nilai).astimezone(WIB).date()


def tengah_malam_utc(tanggal: date) -> datetime:
    """Tanggal bisnis disimpan sebagai tengah malam UTC dari tanggal kalender WIB."""
    return datetime.combine(tanggal, time.min, tzinfo=timezone.utc)


def detik_ke_siklus_berikut(sekarang: datetime, offset_menit: int = 5) -> float:
    """Detik menuju jam berikutnya ditambah offset, selalu positif."""
    sekarang = utc(sekarang)
    target = awal_jam(sekarang) + timedelta(minutes=offset_menit)
    if target <= sekarang:
        target += timedelta(hours=1)
    return (target - sekarang).total_seconds()
