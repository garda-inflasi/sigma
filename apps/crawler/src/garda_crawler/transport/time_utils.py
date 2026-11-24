"""Timezone normalization (WIB UTC+7) and historical backfill chunk splitting."""

from collections.abc import Iterator
from datetime import UTC, date, datetime, timedelta, timezone

WIB_TZ = timezone(timedelta(hours=7))


def utc_now() -> datetime:
    """Return the current timezone-aware UTC timestamp."""
    return datetime.now(UTC)


def to_wib_date(moment: datetime) -> date:
    """Convert an aware or UTC-assumed datetime into its Asia/Jakarta (WIB) business date."""
    return ensure_utc(moment).astimezone(WIB_TZ).date()


def to_midnight_utc(day: date) -> datetime:
    """Convert a calendar date into 00:00:00Z for MongoDB `periode` / `last_available_date` fields."""
    return datetime(day.year, day.month, day.day, tzinfo=UTC)


def ensure_utc(moment: datetime) -> datetime:
    """Guarantee timezone-aware UTC datetime (treating naive instances as UTC)."""
    if moment.tzinfo is None:
        return moment.replace(tzinfo=UTC)
    return moment.astimezone(UTC)


def split_date_chunks(start_date: date, end_date: date, max_days: int = 30) -> Iterator[tuple[date, date]]:
    """Yield inclusive [chunk_start, chunk_end] windows bounded by `max_days`."""
    if start_date > end_date:
        return
    step = max(1, max_days)
    cursor = start_date
    while cursor <= end_date:
        chunk_end = min(cursor + timedelta(days=step - 1), end_date)
        yield cursor, chunk_end
        cursor = chunk_end + timedelta(days=1)
