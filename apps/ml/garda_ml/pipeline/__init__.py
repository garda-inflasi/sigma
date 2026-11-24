"""Pipeline training per jam: ``dataunseen`` -> model -> ``datasafe``.

Kontrak data ada di ``docs/PIPELINE-DATA-ML.md``. Subpaket ini memerlukan extras
``pipeline`` (lihat ``pyproject.toml``). Berkas ini sengaja tidak mengimpor pustaka
ML agar server HTTP lama dan ``make test/ml`` tetap berjalan tanpa dependensi.

Jalankan dari ``apps/ml``::

    python -m garda_ml.pipeline run      # satu siklus untuk jam berjalan
    python -m garda_ml.pipeline worker   # siklus tiap jam
"""

VERSI_PIPELINE = "1.0.0"
