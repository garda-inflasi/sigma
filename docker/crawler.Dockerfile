# Layanan crawler Garda Inflasi. Konteks build: akar repo.
FROM python:3.12-slim AS builder

ENV UV_LINK_MODE=copy \
    UV_PYTHON_DOWNLOADS=never \
    UV_PROJECT_ENVIRONMENT=/opt/venv

RUN pip install --no-cache-dir "uv==0.12.19"

WORKDIR /build
COPY apps/crawler/pyproject.toml apps/crawler/uv.lock ./
RUN uv sync --frozen --no-dev --no-install-project

FROM python:3.12-slim AS runtime

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PATH=/opt/venv/bin:$PATH \
    CRAWLER_BERKAS_DETAK=/tmp/garda-crawler-detak

RUN addgroup --system --gid 10001 crawler \
    && adduser --system --uid 10001 --ingroup crawler --no-create-home crawler

COPY --from=builder /opt/venv /opt/venv
WORKDIR /app
COPY apps/crawler/src/garda_crawler ./garda_crawler

USER crawler

# Sehat bila penjadwal menulis detak dalam 3 menit terakhir.
HEALTHCHECK --interval=60s --timeout=10s --start-period=60s --retries=3 \
  CMD ["python", "-m", "garda_crawler", "health"]

CMD ["python", "-m", "garda_crawler", "worker"]
