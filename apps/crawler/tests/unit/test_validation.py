"""Unit tests for numeric parsing, alias resolution, price rules, and physical climate bounds."""

from datetime import date

import pytest

from garda_crawler.config.reference_catalog import (
    BMKG_REGIONS,
    BPS_REGIONS,
    PIHPS_COMMODITIES,
    PIHPS_REGIONS,
    REGENCIES,
    COMMODITIES,
)
from garda_crawler.validation.macro_rules import REASON_FUTURE_PERIOD, validate_macro_period
from garda_crawler.validation.normalizers import normalize_alias, parse_decimal
from garda_crawler.validation.price_rules import (
    REASON_NON_POSITIVE,
    REASON_OUT_OF_BOUNDS,
    REASON_UNIT_MISMATCH,
    validate_price,
)
from garda_crawler.validation.weather_rules import is_valid_weather_value
from tests.conftest import load_fixture


@pytest.mark.parametrize(
    ("raw", "expected"),
    [
        ("15,850", 15850.0),
        ("14.350", 14350.0),
        ("Rp 14.350,50", 14350.5),
        ("14350.5", 14350.5),
        ("999.999.999", 999999999.0),
        (54500, 54500.0),
        ("-", None),
        ("", None),
        ("abc", None),
        (None, None),
        (True, None),
    ],
)
def test_parse_decimal_formats(raw, expected):
    assert parse_decimal(raw) == expected


def test_validate_price_rules():
    assert validate_price(15000, 13900) is None
    assert validate_price(0, 13900) == REASON_NON_POSITIVE
    assert validate_price(-5, 13900) == REASON_NON_POSITIVE
    assert validate_price(999_999_999, 13900) == REASON_OUT_OF_BOUNDS
    assert validate_price(2000, 29500) == REASON_UNIT_MISMATCH
    assert validate_price(15000, 13900, "liter", "kg") == REASON_UNIT_MISMATCH
    assert validate_price(15000, 13900, "Kg", "kg") is None


def test_weather_physical_bounds():
    assert is_valid_weather_value("curah_hujan_harian", 12.4)
    assert not is_valid_weather_value("curah_hujan_harian", -3.0)
    assert not is_valid_weather_value("curah_hujan_harian", 1500.0)
    assert is_valid_weather_value("suhu_rata", 27.5)
    assert not is_valid_weather_value("suhu_rata", 75.0)
    assert is_valid_weather_value("kelembapan_rata", 82.0)
    assert not is_valid_weather_value("kelembapan_rata", 105.0)


def test_macro_period_rejects_far_future():
    assert validate_macro_period(date(2026, 9, 1), date(2026, 11, 24)) is None
    assert validate_macro_period(date(2030, 1, 1), date(2026, 11, 24)) == REASON_FUTURE_PERIOD


def test_all_23_aceh_regencies_mapped_across_sources():
    assert len(REGENCIES) == 23
    assert set(PIHPS_REGIONS.values()) == set(REGENCIES)
    assert set(BMKG_REGIONS.values()) == set(REGENCIES)
    assert set(PIHPS_COMMODITIES.values()) <= set(COMMODITIES)
    for regency in REGENCIES.values():
        assert BPS_REGIONS[normalize_alias(regency.name)] == regency.code
    assert BPS_REGIONS["aceh"] == "11"
    assert BPS_REGIONS["kota banda aceh"] == "1171"


def test_real_pihps_regencies_match_catalog():
    actual = {row["regency_id"]: row["regency_name"].strip() for row in load_fixture("pihps_regency_aceh.json")["data"]}
    actual.pop(0)
    assert set(actual) == set(PIHPS_REGIONS)
    expected = {
        "Banda Aceh": "1171",
        "Meulaboh": "1107",
        "Aceh Jeumpa / Bireun": "1110",
        "Simeuleu": "1101",
        "Gayo Luwes": "1113",
        "Langsa": "1173",
        "Lhokseumawe": "1174",
        "Pidie Jaya": "1118",
    }
    for rid, name in actual.items():
        if name in expected:
            assert PIHPS_REGIONS[rid] == expected[name], name


def test_bmkg_adm4_uses_kemendagri_codes():
    assert BMKG_REGIONS["11.09.02.2001"] == "1101"
    assert BMKG_REGIONS["11.74.03.2001"] == "1173"
    assert BMKG_REGIONS["11.73.01.2002"] == "1174"
