"""Server HTTP kontrol internal untuk worker ML pipeline (status live & trigger manual)."""

from __future__ import annotations

import json
import logging
import os
import threading
import time
import uuid
from collections.abc import Callable
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

log = logging.getLogger("garda_ml.pipeline.kontrol")

PORT_KONTROL_BAWAAN = 8092
MAKS_LOG = 60


def _iso_utc(dt: datetime | None = None) -> str:
    return (dt or datetime.now(timezone.utc)).astimezone(timezone.utc).isoformat()


class KeadaanKontrolPipeline:
    """State progres realtime worker ML + anti double-run trigger manual."""

    def __init__(self, pelaksana: Callable[[Callable[[str, int, str, str | None], None]], dict[str, Any]]) -> None:
        self.pelaksana = pelaksana
        self._lock = threading.Lock()
        self._utas: threading.Thread | None = None
        self._state: dict[str, Any] = {
            "berjalan": False,
            "job_id": None,
            "cycle_id": None,
            "tahap": "SIAGA",
            "progress_pct": 0,
            "mulai": None,
            "selesai": None,
            "durasi_ms": None,
            "logs": [],
            "hasil": None,
            "error": None,
            "jadwal": {"interval_jam": 1},
        }

    def catat(self, tahap: str, pct: int, pesan: str, cycle_id: str | None = None) -> None:
        with self._lock:
            self._state["tahap"] = tahap
            self._state["progress_pct"] = max(0, min(100, int(pct)))
            if cycle_id:
                self._state["cycle_id"] = cycle_id
            logs: list[str] = self._state["logs"]
            logs.append(f"[{datetime.now(timezone.utc).strftime('%H:%M:%S')} UTC] {pesan}")
            if len(logs) > MAKS_LOG:
                del logs[: len(logs) - MAKS_LOG]

    def snapshot(self) -> dict[str, Any]:
        with self._lock:
            out = dict(self._state)
            out["logs"] = list(self._state["logs"])
            out["jadwal"] = dict(self._state["jadwal"])
            return out

    def mulai(self, job_id: str | None = None) -> tuple[bool, dict[str, Any]]:
        with self._lock:
            if self._state["berjalan"]:
                return False, dict(self._state)
            jid = job_id or f"MLP-{uuid.uuid4().hex[:10].upper()}"
            self._state.update({
                "berjalan": True,
                "job_id": jid,
                "cycle_id": None,
                "tahap": "ANTRIAN",
                "progress_pct": 5,
                "mulai": _iso_utc(),
                "selesai": None,
                "durasi_ms": None,
                "logs": [f"[{datetime.now(timezone.utc).strftime('%H:%M:%S')} UTC] Menyiapkan siklus training ML datasafe..."],
                "hasil": None,
                "error": None,
            })

        t = threading.Thread(target=self._jalankan_latar, args=(jid,), name=f"ml-manual-{jid}", daemon=True)
        self._utas = t
        t.start()
        return True, self.snapshot()

    def _jalankan_latar(self, job_id: str) -> None:
        t0 = time.perf_counter()
        try:
            hasil = self.pelaksana(self.catat)
            status = hasil.get("status") or ("SELESAI" if not hasil.get("error") else "GAGAL")
            self.catat(
                "SELESAI",
                100,
                f"Siklus {hasil.get('cycle_id', '')} selesai ({status}).",
                hasil.get("cycle_id"),
            )
            with self._lock:
                self._state["berjalan"] = False
                self._state["selesai"] = _iso_utc()
                self._state["durasi_ms"] = int((time.perf_counter() - t0) * 1000)
                self._state["hasil"] = {k: v for k, v in hasil.items() if k != "model_metrics"}
        except Exception as exc:  # noqa: BLE001
            self.catat("GAGAL", 100, f"Training gagal: {type(exc).__name__}: {exc}")
            with self._lock:
                self._state["berjalan"] = False
                self._state["selesai"] = _iso_utc()
                self._state["durasi_ms"] = int((time.perf_counter() - t0) * 1000)
                self._state["error"] = f"{type(exc).__name__}: {exc}"


def buat_server_kontrol(keadaan: KeadaanKontrolPipeline, host: str = "0.0.0.0", port: int = PORT_KONTROL_BAWAAN) -> ThreadingHTTPServer:
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, format: str, *args: object) -> None:  # noqa: A002
            log.debug("ml kontrol http: " + format, *args)

        def _kirim(self, code: int, payload: dict[str, Any]) -> None:
            raw = json.dumps(payload, ensure_ascii=False, default=str).encode("utf-8")
            self.send_response(code)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Content-Length", str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)

        def do_GET(self) -> None:  # noqa: N802
            path = self.path.split("?", 1)[0]
            if path == "/health":
                self._kirim(200, {"ok": True})
                return
            if path == "/status":
                self._kirim(200, keadaan.snapshot())
                return
            self._kirim(404, {"error": "NOT_FOUND"})

        def do_POST(self) -> None:  # noqa: N802
            path = self.path.split("?", 1)[0]
            if path != "/run":
                self._kirim(404, {"error": "NOT_FOUND"})
                return
            panjang = int(self.headers.get("Content-Length", "0") or 0)
            body: dict[str, Any] = {}
            if panjang > 0:
                try:
                    body = json.loads(self.rfile.read(panjang).decode("utf-8"))
                except json.JSONDecodeError:
                    self._kirim(400, {"error": "INVALID_JSON"})
                    return
            ok, snap = keadaan.mulai(job_id=body.get("job_id"))
            if not ok:
                self._kirim(409, {"error": "SEDANG_BERJALAN", "state": snap})
                return
            self._kirim(202, snap)

    return ThreadingHTTPServer((host, port), Handler)


def jalankan_server_kontrol_latar(
    pelaksana: Callable[[Callable[[str, int, str, str | None], None]], dict[str, Any]],
    port: int | None = None,
) -> tuple[ThreadingHTTPServer, KeadaanKontrolPipeline]:
    p = port if port is not None else int(os.environ.get("ML_PIPELINE_CONTROL_PORT", str(PORT_KONTROL_BAWAAN)))
    keadaan = KeadaanKontrolPipeline(pelaksana=pelaksana)
    srv = buat_server_kontrol(keadaan, port=p)
    t = threading.Thread(target=srv.serve_forever, name="ml-kontrol-http", daemon=True)
    t.start()
    log.info("ml pipeline kontrol HTTP aktif di :%d", p)
    return srv, keadaan
