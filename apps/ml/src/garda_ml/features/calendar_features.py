"""Indonesian holiday and Islamic calendar features (100% deterministic, leakage-free)."""

import math
from datetime import date

import numpy as np
import pandas as pd

IDULFITRI = tuple(
    date.fromisoformat(d)
    for d in (
        "2018-06-15", "2019-06-05", "2020-05-24", "2021-05-13", "2022-05-02", "2023-04-22", "2024-04-10",
        "2025-03-31", "2026-03-21", "2027-03-10", "2028-02-27", "2029-02-15", "2030-02-05",
    )
)
IDULADHA = tuple(
    date.fromisoformat(d)
    for d in (
        "2018-08-22", "2019-08-11", "2020-07-31", "2021-07-20", "2022-07-10", "2023-06-29", "2024-06-17",
        "2025-06-06", "2026-05-27", "2027-05-17", "2028-05-05", "2029-04-24", "2030-04-13",
    )
)
AWAL_RAMADAN = tuple(
    date.fromisoformat(d)
    for d in (
        "2018-05-17", "2019-05-06", "2020-04-24", "2021-04-13", "2022-04-03", "2023-03-23", "2024-03-12",
        "2025-03-01", "2026-02-19", "2027-02-08", "2028-01-28", "2029-01-16", "2030-01-06", "2030-12-26",
    )
)

_HIJRI_EPOCH = 227015
_HIJRI_YEAR_RANGE = range(1430, 1491)
_TABLE_SNAP_DAYS = 3


def hijri_to_gregorian(year: int, month: int, day: int) -> date:
    """Convert tabular Hijri date into Gregorian calendar date."""
    ordinal = day + math.ceil(29.5 * (month - 1)) + (year - 1) * 354 + (3 + 11 * year) // 30 + _HIJRI_EPOCH - 1
    return date.fromordinal(ordinal)


def _build_event_dates(hijri_month: int, hijri_day: int, official_table: tuple[date, ...]) -> np.ndarray:
    dates: list[date] = []
    for hijri_year in _HIJRI_YEAR_RANGE:
        approx = hijri_to_gregorian(hijri_year, hijri_month, hijri_day)
        snapped = [d for d in official_table if abs((d - approx).days) <= _TABLE_SNAP_DAYS]
        dates.append(snapped[0] if snapped else approx)
    return np.array(sorted(set(dates)), dtype="datetime64[D]")


_EID_FITR_DATES = _build_event_dates(10, 1, IDULFITRI)
_EID_ADHA_DATES = _build_event_dates(12, 10, IDULADHA)
_RAMADAN_START_DATES = _build_event_dates(9, 1, AWAL_RAMADAN)


def _days_until(days: np.ndarray, events: np.ndarray) -> np.ndarray:
    pos = np.searchsorted(events, days, side="left")
    pos = np.clip(pos, 0, len(events) - 1)
    return (events[pos] - days).astype("timedelta64[D]").astype(float)


def _days_since(days: np.ndarray, events: np.ndarray) -> np.ndarray:
    pos = np.searchsorted(events, days, side="right") - 1
    pos = np.clip(pos, 0, len(events) - 1)
    return (days - events[pos]).astype("timedelta64[D]").astype(float)


def _annual_holiday(dates: pd.DatetimeIndex, month: int, day: int) -> np.ndarray:
    years = range(dates.year.min() - 1, dates.year.max() + 2)
    return np.array([date(y, month, day) for y in years], dtype="datetime64[D]")


def build_calendar_features(date_index: pd.DatetimeIndex) -> pd.DataFrame:
    """Extract deterministic calendar proximity features per date."""
    days = date_index.values.astype("datetime64[D]")
    days_since_ramadan = _days_since(days, _RAMADAN_START_DATES)
    days_to_eid = _days_until(days, _EID_FITR_DATES)

    cols = {
        "kal_hari_ke_idulfitri": np.clip(days_to_eid, 0, 90),
        "kal_hari_sejak_idulfitri": np.clip(_days_since(days, _EID_FITR_DATES), 0, 45),
        "kal_hari_ke_iduladha": np.clip(_days_until(days, _EID_ADHA_DATES), 0, 60),
        "kal_is_ramadan": ((days_since_ramadan >= 0) & (days_since_ramadan < 31) & (days_to_eid <= 30) & (days_to_eid > 0)).astype(float),
        "kal_hari_ke_natal": np.clip(_days_until(days, _annual_holiday(date_index, 12, 25)), 0, 45),
        "kal_hari_ke_tahun_baru": np.clip(_days_until(days, _annual_holiday(date_index, 1, 1)), 0, 45),
        "kal_hari_minggu": date_index.dayofweek.to_numpy(dtype=float),
        "kal_bulan": date_index.month.to_numpy(dtype=float),
    }
    return pd.DataFrame(cols, index=date_index, dtype="float32")


def eid_fitr_date(gregorian_year: int) -> date:
    """Return first Eid al-Fitr Gregorian date in the specified year."""
    start = np.datetime64(date(gregorian_year, 1, 1))
    return _EID_FITR_DATES[np.searchsorted(_EID_FITR_DATES, start)].astype(date)
