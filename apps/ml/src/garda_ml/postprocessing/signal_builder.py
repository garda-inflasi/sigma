"""EWS signal builder consolidating forecast spikes, pressure, source freshness, and recovery markers."""

from typing import Any

import numpy as np

from garda_ml.config.constants import DEFAULT_HORIZONS
from garda_ml.domain.entities import ForecastResult
from garda_ml.domain.enums import SeverityLevel

SEVERITIES: tuple[str, ...] = (
    SeverityLevel.CRITICAL,
    SeverityLevel.ALERT,
    SeverityLevel.WATCH,
    SeverityLevel.INFO,
)
DAILY_SOURCES: frozenset[str] = frozenset({"PIHPS", "BMKG", "OPEN_METEO", "SP2KP", "BAPANAS", "LAPANGAN"})
PRICE_SOURCES: frozenset[str] = frozenset({"PIHPS", "SP2KP", "BAPANAS"})


def build_projection_signals(results: list[ForecastResult], commodity_names: dict[str, str]) -> list[dict[str, Any]]:
    """Emit projected spike alerts (`CRITICAL` requires P10 > latest price as well)."""
    signals: list[dict[str, Any]] = []
    for res in results:
        latest_price = res.series.latest_price
        pct_changes = (res.quantiles[:, 1] / latest_price - 1) * 100
        idx = int(np.argmax(pct_changes))
        peak_pct = float(pct_changes[idx])
        p10_above = bool(res.quantiles[idx, 0] > latest_price)

        if peak_pct >= 20 and p10_above:
            severity, threshold = SeverityLevel.CRITICAL, 20.0
        elif peak_pct >= 10:
            severity, threshold = SeverityLevel.ALERT, 10.0
        elif peak_pct >= 5:
            severity, threshold = SeverityLevel.WATCH, 5.0
        else:
            continue

        k, w = res.series.commodity_code, res.series.regency_code
        horizon = DEFAULT_HORIZONS[idx]
        label = commodity_names.get(k, k)
        signals.append(
            {
                "sinyal_kunci": f"PROYEKSI_LONJAKAN:{k}:{w}",
                "severity": severity,
                "jenis": "PROYEKSI_LONJAKAN",
                "komoditas_kode": k,
                "kabupaten_kode": w,
                "nilai": round(peak_pct, 3),
                "ambang": threshold,
                "metode": res.model_name,
                "judul": f"Proyeksi lonjakan {label}",
                "pesan": f"Harga {label} di wilayah {w} diproyeksikan naik {peak_pct:.1f}% pada H+{horizon} (P50).",
                "aktif": True,
            }
        )
    return signals


def build_pressure_signals(inflation_signals: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Convert eligible calibrated pressure predictions into EWS alerts."""
    signals: list[dict[str, Any]] = []
    for sig in inflation_signals:
        score = sig.get("skor_tekanan")
        if score is None or not sig.get("layak", True):
            continue
        if score >= 90:
            severity, threshold = SeverityLevel.CRITICAL, 90.0
        elif score >= 75:
            severity, threshold = SeverityLevel.ALERT, 75.0
        elif score >= 60:
            severity, threshold = SeverityLevel.WATCH, 60.0
        else:
            continue

        label = "inflasi resmi BPS" if sig["target"] == "RESMI_BPS" else "indeks harga pangan"
        signals.append(
            {
                "sinyal_kunci": f"TEKANAN_INFLASI:{sig['wilayah_kode']}",
                "severity": severity,
                "jenis": "TEKANAN_INFLASI",
                "komoditas_kode": None,
                "kabupaten_kode": sig["wilayah_kode"],
                "nilai": score,
                "ambang": threshold,
                "metode": "klasifikasi_terkalibrasi",
                "judul": f"Tekanan {label}",
                "pesan": f"Skor tekanan {label} wilayah {sig['wilayah_kode']} sebesar {score:.0f} (probabilitas {sig['probabilitas'] * 100:.0f}%).",
                "aktif": True,
            }
        )
    return signals


def build_freshness_signals(freshness: list[dict[str, Any]], has_prices: bool) -> list[dict[str, Any]]:
    """Build operational health signals without treating intentionally skipped adapters as failures."""
    signals: list[dict[str, Any]] = []
    for row in freshness:
        source, dataset, age_days = row["sumber"], row["dataset"], row.get("umur_hari")
        is_daily = source in DAILY_SOURCES
        watch_limit, alert_limit = (7, 30) if is_daily else (75, 120)
        severity, threshold = None, None
        is_skipped = str(row.get("pesan") or "").startswith("DILEWATI:")

        if is_skipped:
            severity, threshold = SeverityLevel.INFO, 0.0
        elif row.get("status") == "ERROR":
            severity, threshold = SeverityLevel.ALERT, 0.0
        elif age_days is not None and age_days > alert_limit:
            severity, threshold = SeverityLevel.ALERT, float(alert_limit)
        elif age_days is not None and age_days > watch_limit:
            severity, threshold = SeverityLevel.WATCH, float(watch_limit)

        if severity is None:
            continue

        lad = row.get("last_available_date")
        lad_text = lad.date().isoformat() if lad is not None else "tidak diketahui"
        state_word = "dilewati" if is_skipped else "gagal" if row.get("status") == "ERROR" else "basi"
        signals.append(
            {
                "sinyal_kunci": f"SUMBER_DATA:{source}:{dataset}",
                "severity": severity,
                "jenis": "SUMBER_DATA",
                "komoditas_kode": None,
                "kabupaten_kode": None,
                "nilai": None if age_days is None else float(age_days),
                "ambang": threshold,
                "metode": "kesegaran_sumber",
                "judul": f"Sumber {source} {state_word}: {dataset}",
                "pesan": f"Status {row.get('status')}, data asli terakhir {lad_text}. {row.get('pesan') or ''}".strip(),
                "aktif": True,
            }
        )

    fresh_price_sources = [
        row
        for row in freshness
        if row["sumber"] in PRICE_SOURCES
        and row.get("status") != "ERROR"
        and not str(row.get("pesan") or "").startswith("DILEWATI:")
        and row.get("umur_hari") is not None
        and row["umur_hari"] <= 7
    ]
    if not has_prices or not fresh_price_sources:
        msg = "Tidak ada data harga yang dapat dilatih." if not has_prices else "Tidak ada sumber harga yang segar dalam 7 hari."
        signals.append(
            {
                "sinyal_kunci": "SUMBER_DATA:HARGA:SEMUA",
                "severity": SeverityLevel.CRITICAL,
                "jenis": "SUMBER_DATA",
                "komoditas_kode": None,
                "kabupaten_kode": None,
                "nilai": None,
                "ambang": 7.0,
                "metode": "kesegaran_sumber",
                "judul": "Tidak ada sumber harga yang segar",
                "pesan": msg,
                "aktif": True,
            }
        )
    return signals


def mark_recovered_signals(current: list[dict[str, Any]], previous_active: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Emit one-time `aktif: false` recovery entries for active alerts resolved in this cycle."""
    active_keys = {sig["sinyal_kunci"] for sig in current}
    recovered: list[dict[str, Any]] = []
    for old in previous_active:
        if not old.get("aktif") or old["sinyal_kunci"] in active_keys:
            continue
        recovered.append(
            {
                "sinyal_kunci": old["sinyal_kunci"],
                "severity": SeverityLevel.INFO,
                "jenis": old["jenis"],
                "komoditas_kode": old.get("komoditas_kode"),
                "kabupaten_kode": old.get("kabupaten_kode"),
                "nilai": None,
                "ambang": old.get("ambang"),
                "metode": old.get("metode"),
                "judul": f"Pulih: {old.get('judul', '')}",
                "pesan": f"Kondisi {old['sinyal_kunci']} (sebelumnya {old.get('severity')}) tidak lagi terdeteksi.",
                "aktif": False,
            }
        )
    return recovered


def consolidate_ews_signals(
    *,
    anomaly_signals: list[dict[str, Any]],
    forecast_results: list[ForecastResult],
    inflation_signals: list[dict[str, Any]],
    freshness: list[dict[str, Any]],
    has_prices: bool,
    commodity_names: dict[str, str],
    previous_active_signals: list[dict[str, Any]] | None = None,
) -> list[dict[str, Any]]:
    """Aggregate all anomaly, projection, inflation pressure, freshness, and recovery EWS signals."""
    signals = [
        *anomaly_signals,
        *build_projection_signals(forecast_results, commodity_names),
        *build_pressure_signals(inflation_signals),
        *build_freshness_signals(freshness, has_prices),
    ]
    if previous_active_signals:
        signals.extend(mark_recovered_signals(signals, previous_active_signals))
    return signals
