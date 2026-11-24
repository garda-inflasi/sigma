"""Hourly pipeline worker loop with graceful shutdown and HTTP control plane."""

import logging
import signal
import threading
from datetime import UTC, datetime

from garda_ml.config.settings import PipelineSettings
from garda_ml.data.time_utils import seconds_until_next_cycle
from garda_ml.orchestration.control_server import start_control_server_background
from garda_ml.orchestration.cycle_runner import CycleOrchestrator

logger = logging.getLogger("garda_ml.pipeline")

RETRY_LOCKED_SECONDS = 60
RETRY_FAILED_SECONDS = 300


class PipelineWorker:
    """24/7 daemon scheduling hourly datasafe ML cycles and serving HTTP :8092."""

    def __init__(self, settings: PipelineSettings) -> None:
        self._settings = settings
        self._stop_event = threading.Event()
        self._orchestrator = CycleOrchestrator(settings)

    def stop(self) -> None:
        self._stop_event.set()

    def run_forever(self) -> int:
        """Install signal handlers, start control HTTP, and run hourly training cycles."""
        server, state = start_control_server_background(
            lambda cb: self._orchestrator.run(progress_cb=cb, force=True),
            port=self._settings.control_port,
        )

        def _handle_signal(signum: int, _frame: object) -> None:
            logger.info("signal %s received; stopping worker after active cycle", signum)
            self.stop()

        signal.signal(signal.SIGTERM, _handle_signal)
        signal.signal(signal.SIGINT, _handle_signal)
        logger.info("garda-ml pipeline worker started")

        try:
            while not self._stop_event.is_set():
                wait_seconds: float
                try:
                    result = self._orchestrator.run(progress_cb=state.record_progress)
                    logger.info(
                        "cycle summary: %s",
                        {k: v for k, v in result.items() if k not in ("model_metrics", "pesan")},
                    )
                    if result.get("alasan") == "kunci dipegang pekerja lain":
                        wait_seconds = RETRY_LOCKED_SECONDS
                    else:
                        wait_seconds = seconds_until_next_cycle(
                            datetime.now(UTC),
                            self._settings.schedule_offset_minutes,
                        )
                except Exception:
                    logger.exception("cycle failed; retrying in %d seconds", RETRY_FAILED_SECONDS)
                    wait_seconds = RETRY_FAILED_SECONDS
                self._stop_event.wait(wait_seconds)
        finally:
            server.shutdown()
            server.server_close()
        logger.info("garda-ml pipeline worker stopped")
        return 0
