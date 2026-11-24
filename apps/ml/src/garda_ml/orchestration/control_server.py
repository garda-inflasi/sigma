"""Internal HTTP control plane (:8092) for live progress tracking and manual triggers."""

import json
import logging
import os
import threading
import time
import uuid
from collections.abc import Callable
from datetime import UTC, datetime
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

logger = logging.getLogger("garda_ml.pipeline.control")

DEFAULT_CONTROL_PORT = 8092
MAX_LOG_ENTRIES = 60

ProgressCallback = Callable[[str, int, str, str | None], None]
CycleExecutor = Callable[[ProgressCallback], dict[str, Any]]


def _iso_utc(dt: datetime | None = None) -> str:
    return (dt or datetime.now(UTC)).astimezone(UTC).isoformat()


class PipelineControlState:
    """Thread-safe execution snapshot & anti-double-run guard exposed on :8092."""

    def __init__(self, executor: CycleExecutor) -> None:
        self._executor = executor
        self._lock = threading.Lock()
        self._thread: threading.Thread | None = None
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

    def record_progress(self, stage: str, pct: int, message: str, cycle_id: str | None = None) -> None:
        """Append progress update formatted for UI live log streaming."""
        with self._lock:
            self._state["tahap"] = stage
            self._state["progress_pct"] = max(0, min(100, int(pct)))
            if cycle_id:
                self._state["cycle_id"] = cycle_id
            logs: list[str] = self._state["logs"]
            logs.append(f"[{datetime.now(UTC).strftime('%H:%M:%S')} UTC] {message}")
            if len(logs) > MAX_LOG_ENTRIES:
                del logs[: len(logs) - MAX_LOG_ENTRIES]

    def snapshot(self) -> dict[str, Any]:
        """Return copy of current control state matching API DTO schema."""
        with self._lock:
            out = dict(self._state)
            out["logs"] = list(self._state["logs"])
            out["jadwal"] = dict(self._state["jadwal"])
            return out

    def start(self, job_id: str | None = None) -> tuple[bool, dict[str, Any]]:
        """Claim exclusive manual job slot and launch execution thread."""
        with self._lock:
            if self._state["berjalan"]:
                return False, dict(self._state)
            jid = job_id or f"MLP-{uuid.uuid4().hex[:10].upper()}"
            self._state.update(
                {
                    "berjalan": True,
                    "job_id": jid,
                    "cycle_id": None,
                    "tahap": "ANTRIAN",
                    "progress_pct": 5,
                    "mulai": _iso_utc(),
                    "selesai": None,
                    "durasi_ms": None,
                    "logs": [
                        f"[{datetime.now(UTC).strftime('%H:%M:%S')} UTC] Menyiapkan siklus training ML datasafe..."
                    ],
                    "hasil": None,
                    "error": None,
                }
            )

        thread = threading.Thread(
            target=self._run_background,
            args=(jid,),
            name=f"ml-manual-{jid}",
            daemon=True,
        )
        self._thread = thread
        thread.start()
        return True, self.snapshot()

    def _run_background(self, job_id: str) -> None:
        del job_id
        t0 = time.perf_counter()
        try:
            result = self._executor(self.record_progress)
            status = result.get("status") or ("SELESAI" if not result.get("error") else "GAGAL")
            self.record_progress(
                "SELESAI",
                100,
                f"Siklus {result.get('cycle_id', '')} selesai ({status}).",
                result.get("cycle_id"),
            )
            with self._lock:
                self._state["berjalan"] = False
                self._state["selesai"] = _iso_utc()
                self._state["durasi_ms"] = int((time.perf_counter() - t0) * 1000)
                self._state["hasil"] = {k: v for k, v in result.items() if k != "model_metrics"}
        except Exception as exc:
            self.record_progress("GAGAL", 100, f"Training gagal: {type(exc).__name__}: {exc}")
            with self._lock:
                self._state["berjalan"] = False
                self._state["selesai"] = _iso_utc()
                self._state["durasi_ms"] = int((time.perf_counter() - t0) * 1000)
                self._state["error"] = f"{type(exc).__name__}: {exc}"


def create_control_server(
    state: PipelineControlState,
    host: str = "0.0.0.0",
    port: int = DEFAULT_CONTROL_PORT,
) -> ThreadingHTTPServer:
    """Bind control plane HTTP server serving `/health`, `/status`, and `/run`."""

    class _Handler(BaseHTTPRequestHandler):
        def log_message(self, format: str, *args: object) -> None:  # noqa: A002
            logger.debug("ml control http: " + format, *args)

        def _send_json(self, code: int, payload: dict[str, Any]) -> None:
            raw = json.dumps(payload, ensure_ascii=False, default=str).encode("utf-8")
            self.send_response(code)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Content-Length", str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)

        def do_GET(self) -> None:  # noqa: N802
            path = self.path.split("?", 1)[0]
            if path == "/health":
                self._send_json(200, {"ok": True})
                return
            if path == "/status":
                self._send_json(200, state.snapshot())
                return
            self._send_json(404, {"error": "NOT_FOUND"})

        def do_POST(self) -> None:  # noqa: N802
            path = self.path.split("?", 1)[0]
            if path != "/run":
                self._send_json(404, {"error": "NOT_FOUND"})
                return
            length = int(self.headers.get("Content-Length", "0") or 0)
            body: dict[str, Any] = {}
            if length > 0:
                try:
                    body = json.loads(self.rfile.read(length).decode("utf-8"))
                except json.JSONDecodeError:
                    self._send_json(400, {"error": "INVALID_JSON"})
                    return

            ok, snap = state.start(job_id=body.get("job_id"))
            if not ok:
                self._send_json(409, {"error": "SEDANG_BERJALAN", "state": snap})
                return
            self._send_json(202, snap)

    return ThreadingHTTPServer((host, port), _Handler)


def start_control_server_background(
    executor: CycleExecutor,
    port: int | None = None,
) -> tuple[ThreadingHTTPServer, PipelineControlState]:
    """Start daemon control HTTP server and return `(server, state)`."""
    resolved_port = port if port is not None else int(os.environ.get("ML_PIPELINE_CONTROL_PORT", str(DEFAULT_CONTROL_PORT)))
    state = PipelineControlState(executor=executor)
    server = create_control_server(state, port=resolved_port)
    thread = threading.Thread(target=server.serve_forever, name="ml-control-http", daemon=True)
    thread.start()
    logger.info("ml pipeline control HTTP active on :%d", resolved_port)
    return server, state
