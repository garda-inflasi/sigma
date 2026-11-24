"""Domain data-quality rules for food prices before MongoDB persistence."""

from garda_crawler.validation.normalizers import normalize_price_unit

MAX_PRICE_IDR = 10_000_000.0
MIN_RATIO_VS_REF = 0.2
MAX_RATIO_VS_REF = 5.0

REASON_NON_POSITIVE = "harga_non_positif"
REASON_OUT_OF_BOUNDS = "harga_di_luar_batas"
REASON_UNIT_MISMATCH = "satuan_keliru"
REASON_NOT_NUMERIC = "harga_bukan_angka"


def validate_price(
    value: float,
    reference_price: float,
    entry_unit: str = "",
    expected_unit: str = "",
) -> str | None:
    """Return a canonical rejection reason key, or None when the price passes quality gates."""
    if value <= 0:
        return REASON_NON_POSITIVE
    if value > MAX_PRICE_IDR:
        return REASON_OUT_OF_BOUNDS
    if entry_unit and expected_unit and normalize_price_unit(entry_unit) != normalize_price_unit(expected_unit):
        return REASON_UNIT_MISMATCH
    if reference_price > 0 and not (reference_price * MIN_RATIO_VS_REF <= value <= reference_price * MAX_RATIO_VS_REF):
        return REASON_UNIT_MISMATCH
    return None
