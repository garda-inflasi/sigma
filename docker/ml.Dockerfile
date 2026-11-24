# Dua target:
#   pipeline  worker training (LightGBM, statsmodels, SHAP), bangun dengan --target pipeline
#   server    layanan HTTP tanpa dependensi (target bawaan, harus tetap stage terakhir)

# ── Dependensi pipeline, terkunci oleh uv.lock ─────────────────────────
FROM python:3.12-slim AS pipeline-deps

ENV UV_PROJECT_ENVIRONMENT=/opt/venv \
    UV_PYTHON_DOWNLOADS=never \
    UV_LINK_MODE=copy

RUN pip install --no-cache-dir "uv==0.12.19"

WORKDIR /build
COPY apps/ml/pyproject.toml apps/ml/uv.lock ./
RUN uv sync --frozen --no-cache --extra pipeline

# ── Worker pipeline ────────────────────────────────────────────────────
FROM python:3.12-slim AS pipeline

# libgomp1: runtime OpenMP yang dibutuhkan LightGBM.
RUN apt-get update \
    && apt-get install -y --no-install-recommends libgomp1 \
    && rm -rf /var/lib/apt/lists/*

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PATH="/opt/venv/bin:$PATH" \
    HOME=/tmp \
    MPLCONFIGDIR=/tmp/matplotlib

RUN addgroup --system appgroup && adduser --system --ingroup appgroup appuser

COPY --from=pipeline-deps /opt/venv /opt/venv
WORKDIR /app
COPY apps/ml/garda_ml ./garda_ml

USER appuser

# Worker tidak membuka port. Kesehatannya terbaca dari ml_training_cycles.
HEALTHCHECK NONE

CMD ["python", "-m", "garda_ml.pipeline", "worker"]

# ── Server HTTP (target bawaan) ────────────────────────────────────────
FROM python:3.13-slim AS server

# Layanan HTTP hanya memakai pustaka standar Python: tidak ada dependensi untuk dipasang.
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1

RUN addgroup --system appgroup && adduser --system --ingroup appgroup appuser

WORKDIR /app
COPY apps/ml/garda_ml ./garda_ml

USER appuser

# Hanya untuk jaringan internal compose; tidak ada autentikasi, jangan dipublikasikan.
EXPOSE 8000

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD ["python", "-c", "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8000/health', timeout=3)"]

CMD ["python", "-m", "garda_ml"]
