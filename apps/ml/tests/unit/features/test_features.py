"""Unit tests for zero-leakage feature engineering, weather t-2 lag, macro as-of, calendar, and graph."""

from datetime import date

import numpy as np
import pandas as pd
import pytest

from garda_ml.config.constants import DEFAULT_HORIZONS, FIELD_STATUS_WEIGHTS
from garda_ml.data.unseen_reader import compute_conservative_release_date, map_macro_domain, merge_price_observations
from garda_ml.domain.entities import FeaturePanel, RawMarketDataset, RegionCluster
from garda_ml.features import calendar_features as cal
from garda_ml.features.builder import build_feature_panel
from garda_ml.features.cluster_features import inject_cluster_features, summarize_regency_profiles
from garda_ml.features.graph_builder import build_regency_graph
from garda_ml.features.macro_features import asof_release

END_TS = pd.Timestamp("2026-06-30")


def sample_prices(days: int = 400, commodities=("CBM-KRT", "BRS-MED"), regencies=("1171",), seed: int = 1, end_ts: pd.Timestamp = END_TS) -> pd.DataFrame:
    rng = np.random.default_rng(seed)
    parts = []
    for k in commodities:
        for w in regencies:
            t = pd.date_range(end_ts - pd.Timedelta(days=days), end_ts, freq="D")
            log_p = np.log(20000) + np.cumsum(rng.normal(0.0002, 0.005, len(t))) + 0.01 * np.sin(2 * np.pi * t.dayofweek / 7)
            df = pd.DataFrame(
                {
                    "komoditas_kode": k,
                    "kabupaten_kode": w,
                    "tanggal": t,
                    "nilai": np.exp(log_p),
                    "bobot": 1.0,
                    "nilai_lapangan": np.nan,
                    "bobot_lapangan": np.nan,
                    "sumber": "PIHPS",
                    "satuan": "Rp/kg",
                }
            )
            parts.append(df[df["tanggal"].dt.dayofweek < 5])
    return pd.concat(parts, ignore_index=True)


def wrap_dataset(prices: pd.DataFrame, macro: pd.DataFrame | None = None, weather: pd.DataFrame | None = None) -> RawMarketDataset:
    macro_cols = ["domain", "indikator", "wilayah_kode", "periode", "tanggal_rilis", "nilai", "satuan", "sumber", "frekuensi", "is_carried_forward", "last_available_date"]
    empty_macro = pd.DataFrame(columns=macro_cols)
    return RawMarketDataset(
        prices=prices,
        weather=weather if weather is not None else pd.DataFrame(columns=["kabupaten_kode", "tanggal", "indikator", "nilai"]),
        macro=macro if macro is not None else empty_macro,
        latest_macro=empty_macro,
        freshness=[],
    )


def sample_monthly_macro(start_val: float = 100.0, n: int = 30, domain: str = "TENAGA_KERJA", indicator: str = "upah_buruh", region: str = "11") -> pd.DataFrame:
    periods = pd.date_range(END_TS - pd.DateOffset(months=n), periods=n, freq="MS")
    freq = pd.Series(["BULANAN"] * n)
    releases = compute_conservative_release_date(pd.Series(periods), freq)
    return pd.DataFrame(
        {
            "domain": domain,
            "indikator": indicator,
            "wilayah_kode": region,
            "periode": periods,
            "tanggal_rilis": releases.values,
            "nilai": start_val + np.arange(n),
            "satuan": "Rp",
            "sumber": "BPS",
            "frekuensi": "BULANAN",
            "is_carried_forward": False,
            "last_available_date": periods,
        }
    )


def test_future_prices_never_mutate_past_features():
    prices = sample_prices()
    cutoff_pt = pd.Timestamp("2026-03-16")
    altered = prices.copy()
    altered.loc[altered["tanggal"] > cutoff_pt, "nilai"] *= 3.0
    p1 = build_feature_panel(wrap_dataset(prices), END_TS)
    p2 = build_feature_panel(wrap_dataset(altered), END_TS)
    cols = p1.feature_columns
    x1 = p1.frame[p1.frame["tanggal"] <= cutoff_pt].sort_values(["komoditas_kode", "tanggal"])[cols].to_numpy(float)
    x2 = p2.frame[p2.frame["tanggal"] <= cutoff_pt].sort_values(["komoditas_kode", "tanggal"])[cols].to_numpy(float)
    np.testing.assert_array_equal(x1, x2)


def test_weather_enforces_two_day_lag_and_targets_excluded():
    prices = sample_prices(days=60, commodities=("CBM-KRT",))
    t = pd.date_range(END_TS - pd.Timedelta(days=60), END_TS, freq="D")
    weather = pd.DataFrame({"kabupaten_kode": "1171", "tanggal": t, "indikator": "curah_hujan_harian", "nilai": 1.0})
    mutated = weather.copy()
    mutated.loc[mutated["tanggal"] >= END_TS - pd.Timedelta(days=1), "nilai"] = 500.0
    a = build_feature_panel(wrap_dataset(prices, weather=weather), END_TS).frame.iloc[-1]["cuaca_hujan_7"]
    b = build_feature_panel(wrap_dataset(prices, weather=mutated), END_TS).frame.iloc[-1]["cuaca_hujan_7"]
    assert a == b == pytest.approx(1.0)

    panel = build_feature_panel(wrap_dataset(sample_prices()), END_TS)
    assert not [c for c in panel.feature_columns if c.startswith(("target_", "bobot_h"))]


def test_macro_asof_and_conservative_release():
    grp = pd.DataFrame({"periode": [pd.Timestamp("2026-01-01"), pd.Timestamp("2026-02-01")], "tanggal_rilis": [pd.Timestamp("2026-03-07"), pd.Timestamp("2026-04-04")], "nilai": [10.0, 20.0]})
    vals, _ = asof_release(pd.DatetimeIndex(["2026-03-06", "2026-03-07", "2026-04-03", "2026-04-04"]), grp)
    assert np.isnan(vals[0]) and vals[1] == 10 and vals[2] == 10 and vals[3] == 20

    rel = compute_conservative_release_date(pd.Series([pd.Timestamp("2026-01-01")]), pd.Series(["BULANAN"]))
    assert rel.iloc[0] == pd.Timestamp("2026-01-31") + pd.Timedelta(days=35)


def test_field_price_isolation_and_domain_mapping():
    def _row(src, val, t="2026-05-04", status=None, k="CBM-KRT", w="1171", crawl="2026-05-05", key=None):
        return {"sumber": src, "dataset": "d", "kunci_seri": key or f"{k}:{w}:{src}:{status}", "komoditas_kode": k, "kabupaten_kode": w, "tanggal": pd.Timestamp(t), "nilai": val, "satuan": "Rp/kg", "status_validasi": status, "crawled_at": pd.Timestamp(crawl)}

    assert FIELD_STATUS_WEIGHTS == {"DIPUBLIKASI": 1.0, "DIVALIDASI": 0.8, "DIAJUKAN": 0.3}
    df = pd.DataFrame([_row("PIHPS", 100.0), _row("LAPANGAN", 200.0, status="DIPUBLIKASI"), _row("LAPANGAN", 100.0, status="DIAJUKAN")])
    merged = merge_price_observations(df).iloc[0]
    assert merged["nilai"] == 100.0 and merged["bobot"] == 1.0
    assert merged["nilai_lapangan"] == pytest.approx((200 * 1.0 + 100 * 0.3) / 1.3)
    assert map_macro_domain("raw_bps_makro", "inflasi_yoy") == "INFLASI"
    assert map_macro_domain("raw_sektor_primer", "produksi_ikan_tangkap") == "PERIKANAN"


def test_calendar_and_spatial_graph_and_cluster_injection():
    assert cal.eid_fitr_date(2024) == date(2024, 4, 10)
    f = cal.build_calendar_features(pd.DatetimeIndex(["2024-03-11", "2024-03-12", "2024-04-09", "2024-04-10", "2024-12-20"]))
    assert list(f["kal_is_ramadan"]) == [0, 1, 1, 0, 0]

    graph = build_regency_graph(["1171", "1174", "1107"])
    assert graph.adjacency.shape == (3, 3)
    np.testing.assert_allclose(graph.adjacency.sum(axis=1), np.ones(3), atol=1e-5)

    panel = build_feature_panel(wrap_dataset(sample_prices(days=60, regencies=("1171", "1174"))), END_TS)
    profiles = summarize_regency_profiles(panel)
    assert set(profiles["regency_code"]) == {"1171", "1174"}
    clusters = {
        "1171": RegionCluster("1171", 0, 0.1, 0.01, 5.0, "Relatif Stabil"),
        "1174": RegionCluster("1174", 1, 0.2, 0.03, 9.0, "Volatil Sedang"),
    }
    enriched = inject_cluster_features(panel, clusters)
    assert "cluster_id" in enriched.feature_columns and "cluster_distance" in enriched.feature_columns
