"""Contract time utilities: UTC hourly cycle IDs and Asia/Jakarta (WIB) business dates."""

from datetime import UTC, date, datetime, time, timedelta, timezone

WIB_TZ = timezone(timedelta(hours=7))


def ensure_utc(value: datetime) -> datetime:
    """Normalize datetimes to aware UTC (naive pymongo datetimes are treated as UTC)."""
    if value.tzinfo is None:
        return value.replace(tzinfo=UTC)
    return value.astimezone(UTC)


def hour_floor_utc(value: datetime) -> datetime:
    """Truncate a timestamp to the start of its UTC hour."""
    return ensure_utc(value).replace(minute=0, second=0, microsecond=0)


def build_cycle_id(cycle_timestamp: datetime) -> str:
    """Format canonical `C{YYYYMMDD}T{HH}Z` cycle identifier."""
    return hour_floor_utc(cycle_timestamp).strftime("C%Y%m%dT%HZ")


def to_wib_date(value: datetime) -> date:
    """Convert a timestamp to its Asia/Jakarta calendar date."""
    return ensure_utc(value).astimezone(WIB_TZ).date()


def to_midnight_utc(day: date) -> datetime:
    """Represent a WIB business calendar date as midnight UTC."""
    return datetime.combine(day, time.min, tzinfo=UTC)


def seconds_until_next_cycle(now: datetime, offset_minutes: int = 5) -> float:
    """Return positive seconds until the next scheduled hourly cycle trigger."""
    utc_now = ensure_utc(now)
    target = hour_floor_utc(utc_now) + timedelta(minutes=offset_minutes)
    if target <= utc_now:
        target += timedelta(hours=1)
    return (target - utc_now).total_seconds()
