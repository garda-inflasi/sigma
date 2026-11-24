"""CLI pipeline: ``python -m garda_ml.pipeline {run,worker}``."""

from __future__ import annotations

import argparse
import json
import logging
import signal
import sys
import threading
from datetime import datetime, timezone

log = logging.getLogger("garda_ml.pipeline")

JEDA_ULANG_DETIK = 60
JEDA_SETELAH_GAGAL_DETIK = 300


def _muat():
    try:
        from .config import Konfigurasi
        from .siklus import jalankan_siklus
    except ImportError as err:
        sys.exit(f"Dependensi pipeline belum terpasang ({err}). Jalankan: uv sync --extra pipeline")
    return Konfigurasi, jalankan_siklus


def perintah_run() -> int:
    Konfigurasi, jalankan_siklus = _muat()
    hasil = jalankan_siklus(Konfigurasi.dari_env())
    print(json.dumps({k: v for k, v in hasil.items() if k != "model_metrics"}, ensure_ascii=False, default=str))
    return 0 if hasil.get("status") in ("BERHASIL", "SEBAGIAN") or hasil.get("dilewati") else 1


def perintah_worker() -> int:
    """Satu siklus tiap jam. Siklus berjalan berurutan dan dilindungi kunci di Mongo."""
    Konfigurasi, jalankan_siklus = _muat()
    from .kontrol import jalankan_server_kontrol_latar
    from .waktu import detik_ke_siklus_berikut

    cfg = Konfigurasi.dari_env()
    henti = threading.Event()
    srv, keadaan = jalankan_server_kontrol_latar(
        lambda cb: jalankan_siklus(cfg, progress_cb=cb),
    )

    def tangani(signum: int, _frame: object) -> None:
        log.info("sinyal %s diterima, worker berhenti setelah siklus berjalan", signum)
        henti.set()

    signal.signal(signal.SIGTERM, tangani)
    signal.signal(signal.SIGINT, tangani)
    log.info("worker pipeline mulai")
    try:
        while not henti.is_set():
            tunggu: float
            try:
                hasil = jalankan_siklus(cfg, progress_cb=keadaan.catat)
                log.info("hasil siklus: %s", {k: v for k, v in hasil.items() if k not in ("model_metrics", "pesan")})
                # Kunci dipegang pekerja lain: coba lagi sebentar lagi, bukan menunggu sejam.
                tunggu = JEDA_ULANG_DETIK if hasil.get("alasan") == "kunci dipegang pekerja lain" else detik_ke_siklus_berikut(datetime.now(timezone.utc), cfg.offset_menit)
            except Exception:  # noqa: BLE001 - worker harus bertahan, siklus diulang pada percobaan berikutnya
                log.exception("siklus gagal, dicoba lagi dalam %d detik", JEDA_SETELAH_GAGAL_DETIK)
                tunggu = JEDA_SETELAH_GAGAL_DETIK
            henti.wait(tunggu)
    finally:
        srv.shutdown()
        srv.server_close()
    log.info("worker pipeline berhenti")
    return 0


def main(argv: list[str] | None = None) -> int:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
    parser = argparse.ArgumentParser(prog="python -m garda_ml.pipeline", description="Pipeline training Garda Inflasi")
    parser.add_argument("perintah", choices=("run", "worker"), help="run: satu siklus jam berjalan; worker: siklus tiap jam")
    args = parser.parse_args(argv)
    return perintah_run() if args.perintah == "run" else perintah_worker()


if __name__ == "__main__":
    sys.exit(main())
