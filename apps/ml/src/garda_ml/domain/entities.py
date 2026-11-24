"""Domain entities modeling datasets, feature panels, predictions, clusters, and signals."""

from dataclasses import dataclass, field
from typing import Any


@dataclass
class RawMarketDataset:
    """Consolidated point-in-time snapshot extracted from MongoDB dataunseen."""

    prices: Any  # pd.DataFrame
    weather: Any  # pd.DataFrame
    macro: Any  # pd.DataFrame
    latest_macro: Any  # pd.DataFrame
    freshness: list[dict[str, Any]]
    commodity_names: dict[str, str] = field(default_factory=dict)
    latest_original_dates: dict[tuple[str, str, str], Any] = field(default_factory=dict)
    new_points_count: int = 0


@dataclass
class PriceSeries:
    """Regularized daily price series for a single commodity x regency pair."""

    commodity_code: str
    regency_code: str
    dates: Any  # pd.DatetimeIndex
    log_prices: Any  # np.ndarray
    is_original: Any  # np.ndarray[bool]
    weights: Any  # np.ndarray[float]
    ages: Any  # np.ndarray[float]
    unit: str = "Rp/kg"
    source_code: str = ""

    @property
    def valid_point_count(self) -> int:
        return int(self.is_original.sum())

    @property
    def last_original_idx(self) -> int:
        import numpy as np

        return int(np.flatnonzero(self.is_original)[-1])

    @property
    def latest_age_days(self) -> int:
        return int(len(self.is_original) - 1 - self.last_original_idx)

    @property
    def latest_price(self) -> float:
        import math

        return float(math.exp(float(self.log_prices[self.last_original_idx])))

    @property
    def latest_date(self) -> Any:
        return self.dates[self.last_original_idx]


@dataclass
class FeaturePanel:
    """Zero-leakage feature matrix ready for training and cutoff inference."""

    frame: Any  # pd.DataFrame
    feature_columns: list[str]
    series_map: dict[tuple[str, str], PriceSeries] = field(default_factory=dict)
    cutoff: Any = None
    region_clusters: dict[str, "RegionCluster"] = field(default_factory=dict)

    @property
    def is_empty(self) -> bool:
        return bool(self.frame.empty)


@dataclass
class ForecastResult:
    """Multi-horizon quantile forecast output for one commodity x regency pair."""

    series: PriceSeries
    model_name: str
    is_fallback: bool
    reason: str
    metrics: dict[str, Any]
    quantiles: Any  # np.ndarray shape (len(HORIZONS), 3) in IDR
    candidate_count: int | None = None
    cluster_id: int | None = None


@dataclass(frozen=True)
class RegionCluster:
    """K-Means vulnerability cluster profile per regency."""

    regency_code: str
    cluster_id: int
    centroid_distance: float
    volatility_score: float
    climate_exposure_score: float
    cluster_label: str


@dataclass
class RiskPrediction:
    """Calibrated regional inflation/price-pressure prediction output."""

    regency_code: str
    target_type: str
    probability: float
    pressure_score: float | None
    cycle_phase: str | None
    label: str | None
    is_eligible: bool
    reason: str
    payload: dict[str, Any]


@dataclass(frozen=True)
class AlertSignal:
    """Early Warning System signal ready for datasafe persistence."""

    signal_key: str
    severity: str
    signal_type: str
    commodity_code: str | None
    regency_code: str | None
    value: float | None
    threshold: float | None
    method: str
    title: str
    message: str
    is_active: bool = True


@dataclass(frozen=True)
class XaiExplanation:
    """Unified XAI factor attribution payload across all model backends."""

    target: str
    commodity_code: str | None
    regency_code: str
    method: str
    contribution_unit: str
    factors: list[dict[str, Any]]
    narrative: str
    probability: float | None = None
    baseline_probability: float | None = None
