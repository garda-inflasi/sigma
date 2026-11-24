"""Pandera schema validators for standardized Medallion Silver tables."""

from __future__ import annotations

import pandas as pd

try:
    import pandera.pandas as pa
except ImportError:  # pragma: no cover - kompatibel dengan pandera <0.24
    import pandera as pa  # type: ignore[no-redef]


SILVER_PRICE_SCHEMA = pa.DataFrameSchema(
    columns={
        "kode_bps": pa.Column(str, checks=pa.Check.str_matches(r"^[0-9]{2,10}$"), nullable=False),
        "komoditas_kode": pa.Column(str, checks=pa.Check.str_length(min_value=1), nullable=False),
        "periode": pa.Column("datetime64[ns]", nullable=False),
        "nilai": pa.Column(float, checks=pa.Check.ge(0.0), nullable=True),
        "sumber": pa.Column(str, nullable=False),
    },
    coerce=True,
    strict=False,
)

SILVER_MACRO_SCHEMA = pa.DataFrameSchema(
    columns={
        "kode_bps": pa.Column(str, checks=pa.Check.str_matches(r"^[0-9]{2,10}$"), nullable=False),
        "domain": pa.Column(str, nullable=False),
        "indikator": pa.Column(str, checks=pa.Check.str_length(min_value=1), nullable=False),
        "periode": pa.Column("datetime64[ns]", nullable=False),
        "nilai": pa.Column(float, nullable=True),
        "sumber": pa.Column(str, nullable=False),
    },
    coerce=True,
    strict=False,
)

SILVER_WEATHER_SCHEMA = pa.DataFrameSchema(
    columns={
        "kabupaten_kode": pa.Column(str, checks=pa.Check.str_matches(r"^[0-9]{2,10}$"), nullable=False),
        "tanggal": pa.Column("datetime64[ns]", nullable=False),
        "indikator": pa.Column(str, checks=pa.Check.str_length(min_value=1), nullable=False),
        "nilai": pa.Column(float, nullable=True),
    },
    coerce=True,
    strict=False,
)


def validate_silver_price_df(df: pd.DataFrame) -> pd.DataFrame:
    """Validate and coerce standardized daily commodity prices before Gold promotion."""
    if df.empty:
        return df.copy()
    return SILVER_PRICE_SCHEMA.validate(df, lazy=True)


def validate_silver_macro_df(df: pd.DataFrame) -> pd.DataFrame:
    """Validate and coerce standardized BPS/DJPK/macro indicators before Gold promotion."""
    if df.empty:
        return df.copy()
    return SILVER_MACRO_SCHEMA.validate(df, lazy=True)


def validate_silver_weather_df(df: pd.DataFrame) -> pd.DataFrame:
    """Validate and coerce standardized climate/weather series before Gold promotion."""
    if df.empty:
        return df.copy()
    return SILVER_WEATHER_SCHEMA.validate(df, lazy=True)
