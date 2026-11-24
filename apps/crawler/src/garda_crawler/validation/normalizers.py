"""Pure numeric, text alias, and unit normalization helpers."""

import math
import re
from typing import Any

from garda_crawler.config.reference_catalog import normalize_alias

_THOUSANDS_COMMA_RE = re.compile(r"^-?\d{1,3}(?:,\d{3})+(?:\.\d+)?$")
_THOUSANDS_DOT_RE = re.compile(r"^-?\d{1,3}(?:\.\d{3})+$")
_ID_DECIMAL_RE = re.compile(r"^-?\d{1,3}(?:\.\d{3})*,\d+$")
_SIMPLE_DECIMAL_COMMA_RE = re.compile(r"^-?\d+,\d+$")


def normalize_price_unit(raw_unit: str) -> str:
    """Normalize display unit strings like 'Rp/Kg' or 'Kg' into lowercase unit tokens."""
    text = raw_unit.strip().lower()
    if text.startswith("rp/"):
        return text[3:]
    return text


def parse_decimal(raw: Any) -> float | None:
    """Parse PIHPS/BPS/spreadsheet numbers supporting Indonesian and US thousand/decimal formats."""
    if raw is None or isinstance(raw, bool):
        return None
    if isinstance(raw, (int, float)):
        numeric = float(raw)
        return numeric if math.isfinite(numeric) else None

    text = str(raw).strip()
    if not text or text in {"-", "--", "N/A", "na", "null", "None"}:
        return None

    text = re.sub(r"^(?:rp\.?\s*)", "", text, flags=re.IGNORECASE).replace(" ", "")
    if _THOUSANDS_COMMA_RE.match(text):
        text = text.replace(",", "")
    elif _THOUSANDS_DOT_RE.match(text):
        text = text.replace(".", "")
    elif _ID_DECIMAL_RE.match(text):
        text = text.replace(".", "").replace(",", ".")
    elif _SIMPLE_DECIMAL_COMMA_RE.match(text):
        text = text.replace(",", ".")

    try:
        numeric = float(text)
    except ValueError:
        return None
    return numeric if math.isfinite(numeric) else None


__all__ = ["normalize_alias", "normalize_price_unit", "parse_decimal"]
