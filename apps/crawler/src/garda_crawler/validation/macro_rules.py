"""Validation gates for BPS macroeconomic indicators and period alignment."""

import math
from datetime import date

REASON_FUTURE_PERIOD = "periode_masa_depan"
REASON_MACRO_INVALID = "nilai_makro_tidak_sah"


def is_valid_macro_value(value: float) -> bool:
    """Ensure macroeconomic observations are finite real numbers."""
    return math.isfinite(value)


def validate_macro_period(period: date, reference_today: date | None = None) -> str | None:
    """Reject future-dated macro periods beyond one year ahead."""
    if reference_today is not None and period.year > reference_today.year + 1:
        return REASON_FUTURE_PERIOD
    return None
