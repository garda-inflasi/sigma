"""Internal HTTP control server (:8091) exposing /health, /status, and /run."""

import json
import threading
from collections.abc import Callable
from datetime import datetime
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

from pymongo.database import Database

from garda_crawler.adapters.registry import DEFAULT_ACTIVE_SOURCES
from garda_crawler.config.settings import CrawlerSettings
from garda_crawler.domain.entities import RunSummary
from garda_crawler.domain.enums import SKIPPED_MESSAGE_PREFIX, SourceStatus, TriggerType
from garda_crawler.orchestration.orchestrator import CrawlOrchestrator
from garda_crawler.storage.bronze_store import BronzeOnlyRepository
from garda_crawler.storage.lock_manager import MongoLeaseLock
from garda_crawler.storage.run_ledger import RUNS_COLLECTION, SCHEDULE_COLLECTION
from garda_crawler.storage.unseen_repository import UnseenRepository
from garda_crawler.transport.time_utils import ensure_utc, utc_now

DEFAULT_CONTROL_PORT = 8091
MAX_LOG_ENTRIES = 120

SCHEDULE_METADATA: dict[str, str] = {
    "harian": "Setiap hari 06:00 WIB",
    "lapangan": "Tiap 1 jam (xx:00 WIB)",
}


def find_last_failed_sources(db: Database | None) -> list[str]:
    """Identify retryable ERROR sources (excluding intentional DILEWATI/skipped sources)."""
    if db is None:
        return []
    latest_by_dataset: dict[tuple[str, str], dict[str, Any]] = {}
    cursor = db[RUNS_COLLECTION].find(
        {"sumber": {"$in": list(DEFAULT_ACTIVE_SOURCES)}},
        sort=[("selesai", -1)],
    )
    for run in cursor:
        key = (str(run.get("sumber", "")), str(run.get("dataset", "")))
        if key not in latest_by_dataset:
            latest_by_dataset[key] = run

    failed: list[str] = []
    seen: set[str] = set()
    for (source_code, _), run in latest_by_dataset.items():
        if run.get("status") != SourceStatus.ERROR:
            continue
        message = str(run.get("pesan", ""))
        if message.startswith(SKIPPED_MESSAGE_PREFIX):
            continue
        if source_code not in seen:
            seen.add(source_code)
            failed.append(source_code)

    ordered = [s for s in DEFAULT_ACTIVE_SOURCES if s in seen]
    return ordered or [s for s in failed]


class CrawlerControlState:
    """Thread-safe shared execution state mirrored by GET /status."""

    def __init__(self, db: Database | None = None) -> None:
        self._db = db
        self._lock = threading.Lock()
        self._running = False
        self._job_id: str | None = None
        self._sources: list[str] = []
        self._failed_only = False
        self._stage = "idle"
        self._progress_pct = 0
        self._active_dataset: str | None = None
        self._completed_datasets = 0
        self._total_datasets = 0
        self._started_at: datetime | None = None
        self._finished_at: datetime | None = None
        self._duration_ms = 0
        self._logs: list[str] = []
        self._results: list[dict[str, Any]] = []
        self._error: str | None = None

    def _read_last_slots(self) -> dict[str, str | None]:
        slots: dict[str, str | None] = {"harian": None, "lapangan": None}
        if self._db is None:
            return slots
        for doc in self._db[SCHEDULE_COLLECTION].find({"_id": {"$in": ["harian", "lapangan"]}}):
            slot_id = str(doc["_id"])
            val = doc.get("terakhir")
            slots[slot_id] = str(val) if val else None
        return slots

    def snapshot(self) -> dict[str, Any]:
        """Serialize live state using the exact API control JSON schema."""
        with self._lock:
            return {
                "berjalan": self._running,
                "job_id": self._job_id,
                "sumber": list(self._sources),
                "gagal_saja": self._failed_only,
                "tahap": self._stage,
                "progress_pct": self._progress_pct,
                "dataset_aktif": self._active_dataset,
                "selesai_dataset": self._completed_datasets,
                "total_dataset": self._total_datasets,
                "mulai": self._started_at.isoformat() if self._started_at else None,
                "selesai": self._finished_at.isoformat() if self._finished_at else None,
                "durasi_ms": self._duration_ms,
                "logs": list(self._logs),
                "hasil": list(self._results),
                "error": self._error,
                "jadwal": dict(SCHEDULE_METADATA),
                "slot_terakhir": self._read_last_slots(),
            }

    def try_start(self, *, job_id: str, sources: list[str], failed_only: bool) -> bool:
        """Atomically claim execution state if idle."""
        with self._lock:
            if self._running:
                return False
            self._running = True
            self._job_id = job_id
            self._sources = list(sources)
            self._failed_only = failed_only
            self._stage = "menyiapkan"
            self._progress_pct = 0
            self._active_dataset = None
            self._completed_datasets = 0
            self._total_datasets = 0
            self._started_at = utc_now()
            self._finished_at = None
            self._duration_ms = 0
            self._logs = [f"Memulai crawl manual job_id={job_id} sumber={','.join(sources)}"]
            self._results = []
            self._error = None
            return True

    def record_progress(
        self,
        source_code: str,
        dataset_name: str,
        completed: int,
        total: int,
        summary: RunSummary | None,
    ) -> None:
        with self._lock:
            self._total_datasets = total
            self._completed_datasets = completed
            self._stage = "mengambil"
            self._active_dataset = None if summary else f"{source_code}:{dataset_name}"
            if total > 0:
                self._progress_pct = int(round((completed / total) * 100))
            if summary is not None:
                self._results.append(
                    {
                        "sumber": summary.source_code,
                        "dataset": summary.dataset_name,
                        "status": summary.status,
                        "jumlah_baru": summary.new_count,
                        "jumlah_diteruskan": summary.carried_forward_count,
                        "pesan": summary.message,
                    }
                )
                self._logs.append(
                    f"[{summary.status}] {summary.source_code}/{summary.dataset_name}: {summary.message}"
                )
                if len(self._logs) > MAX_LOG_ENTRIES:
                    self._logs = self._logs[-MAX_LOG_ENTRIES:]

    def finish(self, error: str | None = None) -> None:
        with self._lock:
            finished = utc_now()
            self._finished_at = finished
            if self._started_at is not None:
                self._duration_ms = max(0, int((finished - ensure_utc(self._started_at)).total_seconds() * 1000))
            self._active_dataset = None
            self._error = error
            self._stage = "gagal" if error else "selesai"
            if not error:
                self._progress_pct = 100
            if error:
                self._logs.append(f"[ERROR] {error}")
            self._running = False


RunExecutor = Callable[[CrawlerControlState, list[str], bool, str], None]


def _default_run_executor(
    settings: CrawlerSettings,
    state: CrawlerControlState,
    sources: list[str],
    _failed_only: bool,
    _job_id: str,
) -> None:
    def _worker() -> None:
        error_msg: str | None = None
        try:
            if settings.bronze_only or state._db is None:  # noqa: SLF001
                orchestrator = CrawlOrchestrator(settings, BronzeOnlyRepository())
                orchestrator.run_sources(
                    sources,
                    trigger=TriggerType.MANUAL,
                    on_progress=state.record_progress,
                )
            else:
                with MongoLeaseLock(state._db, "manual-control") as lock:  # noqa: SLF001
                    if not lock.acquired:
                        error_msg = "kunci crawler sedang dipakai proses lain"
                    else:
                        repo = UnseenRepository(state._db)
                        orchestrator = CrawlOrchestrator(settings, repo)
                        orchestrator.run_sources(
                            sources,
                            trigger=TriggerType.MANUAL,
                            on_progress=state.record_progress,
                        )
        except Exception as exc:
            error_msg = f"{type(exc).__name__}: {exc}"
        finally:
            state.finish(error=error_msg)

    threading.Thread(target=_worker, name="crawler-manual-run", daemon=True).start()


def create_control_server(
    state: CrawlerControlState,
    *,
    settings: CrawlerSettings | None = None,
    port: int = DEFAULT_CONTROL_PORT,
    run_executor: RunExecutor | None = None,
) -> ThreadingHTTPServer:
    """Create a ThreadingHTTPServer bound to `/health`, `/status`, and `/run`."""
    executor: RunExecutor
    if run_executor is not None:
        executor = run_executor
    else:
        cfg = settings or CrawlerSettings(mongo_uri="mongodb://localhost:27017")
        executor = lambda st, srcs, fo, jid: _default_run_executor(cfg, st, srcs, fo, jid)

    class _Handler(BaseHTTPRequestHandler):
        def log_message(self, format: str, *args: object) -> None:  # noqa: A002
            return

        def _send_json(self, status_code: int, body: dict[str, Any]) -> None:
            payload = json.dumps(body, ensure_ascii=False).encode("utf-8")
            self.send_response(status_code)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)

        def do_GET(self) -> None:
            if self.path == "/health":
                self._send_json(HTTPStatus.OK, {"ok": True})
                return
            if self.path == "/status":
                self._send_json(HTTPStatus.OK, state.snapshot())
                return
            self._send_json(HTTPStatus.NOT_FOUND, {"error": "NOT_FOUND"})

        def do_POST(self) -> None:
            if self.path != "/run":
                self._send_json(HTTPStatus.NOT_FOUND, {"error": "NOT_FOUND"})
                return

            length = int(self.headers.get("Content-Length", "0") or "0")
            raw_body = self.rfile.read(length) if length > 0 else b"{}"
            try:
                req = json.loads(raw_body.decode("utf-8") or "{}")
            except ValueError:
                self._send_json(HTTPStatus.BAD_REQUEST, {"error": "INVALID_JSON"})
                return

            failed_only = bool(req.get("gagal_saja", False))
            raw_sources = req.get("sumber")
            if isinstance(raw_sources, list) and raw_sources:
                sources = [str(item).strip().upper() for item in raw_sources if str(item).strip()]
            elif failed_only:
                sources = find_last_failed_sources(state._db)  # noqa: SLF001
            else:
                sources = list(DEFAULT_ACTIVE_SOURCES)

            job_id = str(req.get("job_id") or f"manual-{int(utc_now().timestamp())}")
            if not state.try_start(job_id=job_id, sources=sources, failed_only=failed_only):
                self._send_json(
                    HTTPStatus.CONFLICT,
                    {"error": "SEDANG_BERJALAN", "state": state.snapshot()},
                )
                return

            if not sources:
                state.finish(error=None)
                self._send_json(HTTPStatus.ACCEPTED, {"accepted": True, "state": state.snapshot()})
                return

            executor(state, sources, failed_only, job_id)
            self._send_json(HTTPStatus.ACCEPTED, {"accepted": True, "state": state.snapshot()})

    return ThreadingHTTPServer(("0.0.0.0", port), _Handler)


def start_control_server_background(
    state: CrawlerControlState,
    *,
    settings: CrawlerSettings | None = None,
    port: int = DEFAULT_CONTROL_PORT,
    run_executor: RunExecutor | None = None,
) -> ThreadingHTTPServer:
    """Bind and serve the control HTTP server in a daemon thread."""
    server = create_control_server(
        state,
        settings=settings,
        port=port,
        run_executor=run_executor,
    )
    thread = threading.Thread(target=server.serve_forever, name="crawler-control-http", daemon=True)
    thread.start()
    return server
