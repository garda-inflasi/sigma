"""Physical boundary rules for climate/weather observations and forecasts."""

REASON_INCOMPLETE_DAY = "hari_tidak_lengkap"
REASON_WEATHER_PREFIX = "tidak_wajar"

_WEATHER_BOUNDS: dict[str, tuple[float, float]] = {
    "curah_hujan": (0.0, 1000.0),
    "suhu": (-10.0, 50.0),
    "kelembapan": (0.0, 100.0),
}


def is_valid_weather_value(indicator: str, value: float) -> bool:
    """Check whether a weather metric falls within realistic meteorological bounds."""
    for prefix, (low, high) in _WEATHER_BOUNDS.items():
        if indicator.startswith(prefix):
            return low <= value <= high
    return False


def weather_reject_reason(indicator: str) -> str:
    """Build deterministic counter key when rejecting out-of-range weather measurements."""
    for prefix in _WEATHER_BOUNDS:
        if indicator.startswith(prefix):
            return f"{prefix}_{REASON_WEATHER_PREFIX}"
    return f"{indicator}_{REASON_WEATHER_PREFIX}"
