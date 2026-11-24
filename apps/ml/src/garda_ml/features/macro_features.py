"""Point-in-time as-of macroeconomic and primary-sector features."""

import re

import numpy as np
import pandas as pd

MAX_MACRO_INDICATORS = 20

_RE_WAGES = re.compile(r"upah|ump|umk|gaji|biaya.?tenaga")
_RE_SUPPLY = re.compile(r"produksi|panen")
_RE_INFLATION_FEATURE = "yoy|mtm|year_on_year|month_to_month"


def _slugify(text: str) -> str:
    return re.sub(r"[^a-z0-9]+", "_", str(text).lower()).strip("_")[:40]


def _select_macro_indicators(macro: pd.DataFrame) -> list[tuple[str, str]]:
    if macro.empty:
        return []
    counts = macro.groupby(["domain", "indikator"]).size().reset_index(name="n")
    counts = counts[(counts["domain"] != "INFLASI") | counts["indikator"].str.contains(_RE_INFLATION_FEATURE, case=False)]
    counts = counts.sort_values(["n", "domain", "indikator"], ascending=[False, True, True]).head(MAX_MACRO_INDICATORS)
    return [(d, i) for d, i in zip(counts["domain"], counts["indikator"])]


def asof_release(date_index: pd.DatetimeIndex, group: pd.DataFrame) -> tuple[np.ndarray, np.ndarray]:
    """Align macro values strictly by `tanggal_rilis`, preventing late older periods from overwriting newer ones."""
    ordered = group.dropna(subset=["tanggal_rilis", "nilai"]).sort_values(["tanggal_rilis", "periode"])
    if ordered.empty:
        empty = np.full(len(date_index), np.nan, dtype="float32")
        return empty, empty.copy()

    periods = ordered["periode"].to_numpy()
    monotonic = np.r_[True, periods[1:] > np.maximum.accumulate(periods)[:-1]]
    ordered = ordered.loc[monotonic]

    release_dates = ordered["tanggal_rilis"].to_numpy("datetime64[ns]")
    values = ordered["nilai"].to_numpy(float)
    with np.errstate(divide="ignore", invalid="ignore"):
        pct_changes = np.r_[np.nan, (values[1:] / values[:-1] - 1) * 100]
    pct_changes[~np.isfinite(pct_changes)] = np.nan

    pos = np.searchsorted(release_dates, date_index.values, side="right") - 1
    available = pos >= 0
    out_val = np.full(len(date_index), np.nan)
    out_chg = np.full(len(date_index), np.nan)
    out_val[available] = values[pos[available]]
    out_chg[available] = pct_changes[pos[available]]
    return out_val.astype("float32"), out_chg.astype("float32")


def build_macro_features(
    macro: pd.DataFrame,
    regency_codes: list[str],
    date_index: pd.DatetimeIndex,
) -> dict[str, pd.DataFrame]:
    """Assemble as-of macro features per regency using regency -> province -> national fallback."""
    selected = _select_macro_indicators(macro)
    if not selected:
        return {}

    grouped = {
        key: grp
        for key, grp in macro.groupby(["domain", "indikator", macro["wilayah_kode"].fillna("")], sort=False)
    }
    cache: dict[tuple[str, str, str], tuple[np.ndarray, np.ndarray]] = {}
    results: dict[str, pd.DataFrame] = {}

    for regency in regency_codes:
        cols: dict[str, np.ndarray] = {}
        supply_changes, balance_changes, wage_changes = [], [], []

        for domain, indicator in selected:
            for candidate_region in (regency, regency[:2], ""):
                key = (domain, indicator, candidate_region)
                if key not in grouped:
                    continue
                if key not in cache:
                    cache[key] = asof_release(date_index, grouped[key])
                values, changes = cache[key]
                col_name = f"makro_{domain}_{_slugify(indicator)}"
                cols[col_name] = values
                cols[f"{col_name}_chg"] = changes
                if domain in ("PERTANIAN", "PERIKANAN", "KEHUTANAN") and _RE_SUPPLY.search(indicator.lower()):
                    supply_changes.append(changes)
                elif domain == "NERACA_EKONOMI":
                    balance_changes.append(changes)
                elif domain == "TENAGA_KERJA" and _RE_WAGES.search(indicator.lower()):
                    wage_changes.append(changes)
                break

        for agg_col, series_list, sign in (
            ("tekanan_pasokan", supply_changes, -1.0),
            ("tekanan_neraca", balance_changes, 1.0),
            ("biaya_tenaga_kerja", wage_changes, 1.0),
        ):
            if series_list:
                stacked = np.vstack(series_list)
                valid_count = np.isfinite(stacked).sum(axis=0)
                total = np.nansum(stacked, axis=0)
                with np.errstate(invalid="ignore", divide="ignore"):
                    cols[agg_col] = (sign * total / np.where(valid_count > 0, valid_count, np.nan)).astype("float32")

        results[regency] = pd.DataFrame(cols, index=date_index)

    return results
