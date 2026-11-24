"""Domain constants, model history thresholds, and leakage guard parameters."""

DEFAULT_HORIZONS: tuple[int, ...] = (7, 14, 30)
DEFAULT_QUANTILES: tuple[float, ...] = (0.1, 0.5, 0.9)
DEFAULT_LAGS: tuple[int, ...] = (1, 3, 7, 14, 30)

# Weighted reliability hierarchy for internal field entries (rejected entries excluded).
FIELD_STATUS_WEIGHTS: dict[str, float] = {
    "DIPUBLIKASI": 1.0,
    "DIVALIDASI": 0.8,
    "DIAJUKAN": 0.3,
}

# Official price source preference order per commodity x regency series.
OFFICIAL_PRICE_SOURCES: tuple[str, ...] = ("PIHPS", "SP2KP", "BAPANAS")
FIELD_CONSUMER_TYPE = "KONSUMEN"

# Climate reanalysis lags real time; enforce t - 2 availability cutoff for train/inference parity.
WEATHER_AVAILABILITY_LAG_DAYS = 2

# Conservative release-date lag (calendar days after period end) when explicit release_date is absent.
RELEASE_LAG_DAYS: dict[str, int] = {
    "HARIAN": 1,
    "BULANAN": 35,
    "TRIWULANAN": 60,
    "TAHUNAN": 120,
}

# Minimum valid observation counts unlocking each estimator tier.
FORECAST_MIN_POINTS = 8
HOLT_WINTERS_MIN_POINTS = 30
SARIMAX_MIN_POINTS = 90
GLOBAL_MIN_POINTS = 90
GNN_MIN_POINTS = 180

# Maximum staleness tolerances in calendar days.
MAX_TRAIN_ROW_STALE_DAYS = 7
MAX_LABEL_STALE_DAYS = 3
MAX_INFERENCE_STALE_DAYS = 14
MAX_FORECAST_STALE_DAYS = 45

# Champion selection & walk-forward backtest parameters.
MIN_BASELINE_MARGIN = 0.05
N_SELECTION_FOLDS = 4
N_HOLDOUT_FOLDS = 1
MIN_SELECTION_FOLDS = 2
SELECTION_STEP_DAYS = 30
N_ORIGINS_PER_FOLD = 4
ORIGIN_STEP_DAYS = 7

# Risk classification thresholds & embargo parameters.
CLASSIFICATION_MIN_OFFICIAL_MONTHS = 24
CLASSIFICATION_MIN_AUX_ROWS = 30
CLASSIFICATION_INDEX_HORIZON_DAYS = 30
CLASSIFICATION_OFFICIAL_EMBARGO_DAYS = 31
MIN_ROC_AUC_ELIGIBLE = 0.60
PRESSURE_THRESHOLD = 60.0
SURPLUS_THRESHOLD = 40.0
