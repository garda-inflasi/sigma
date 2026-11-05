FROM python:3.13-slim

# Layanan ML hanya memakai pustaka standar Python: tidak ada dependensi untuk dipasang.
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
