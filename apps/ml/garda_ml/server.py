"""Server HTTP tipis (pustaka standar) di atas ``service``.

Endpoint: GET /health, POST /train, POST /predict. Layanan hanya untuk jaringan
internal; tidak ada autentikasi, jadi tidak boleh dipublikasikan ke luar.
"""

from __future__ import annotations

import json
import logging
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from . import service

MAKS_BADAN = 8 * 1024 * 1024  # byte
log = logging.getLogger("garda_ml")

RUTE_POST = {"/train": service.latih, "/predict": service.prediksi}


class Handler(BaseHTTPRequestHandler):
    server_version = "garda-ml"

    def _kirim(self, kode: int, badan: dict) -> None:
        raw = json.dumps(badan, ensure_ascii=False).encode()
        self.send_response(kode)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def do_GET(self) -> None:  # noqa: N802 (nama diwajibkan BaseHTTPRequestHandler)
        if self.path == "/health":
            self._kirim(200, service.kesehatan())
        else:
            self._kirim(404, {"error": "tidak ditemukan"})

    def do_POST(self) -> None:  # noqa: N802
        fungsi = RUTE_POST.get(self.path)
        if fungsi is None:
            self._kirim(404, {"error": "tidak ditemukan"})
            return
        try:
            panjang = int(self.headers.get("Content-Length", ""))
        except ValueError:
            self._kirim(411, {"error": "Content-Length wajib ada"})
            return
        if panjang > MAKS_BADAN:
            self._kirim(413, {"error": "badan permintaan terlalu besar"})
            return
        try:
            muatan = json.loads(self.rfile.read(panjang))
        except json.JSONDecodeError:
            self._kirim(400, {"error": "badan bukan JSON yang valid"})
            return
        try:
            self._kirim(200, fungsi(muatan))
        except service.MuatanTidakValid as err:
            self._kirim(400, {"error": str(err)})
        except Exception:  # noqa: BLE001 - galat tak terduga tidak boleh mematikan server
            log.exception("galat tak terduga pada %s", self.path)
            self._kirim(500, {"error": "galat internal"})

    def log_message(self, format: str, *args) -> None:  # noqa: A002
        log.info("%s %s", self.address_string(), format % args)


def buat_server(host: str, port: int) -> ThreadingHTTPServer:
    return ThreadingHTTPServer((host, port), Handler)


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    host, port = os.environ.get("ML_HOST", "0.0.0.0"), int(os.environ.get("ML_PORT", "8000"))
    server = buat_server(host, port)
    log.info("layanan ML mendengarkan di %s:%d", host, port)
    server.serve_forever()
