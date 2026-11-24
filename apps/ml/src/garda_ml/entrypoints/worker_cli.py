"""CLI entrypoint for training worker (`run`, `run --force`, `worker`)."""

import argparse
import json
import logging
import sys
from collections.abc import Sequence


def _load_pipeline():
    try:
        from garda_ml.config.settings import PipelineSettings
        from garda_ml.orchestration.cycle_runner import run_cycle
        from garda_ml.orchestration.scheduler import PipelineWorker
    except ImportError as err:
        sys.exit(f"Dependensi pipeline belum terpasang ({err}). Jalankan: uv sync --extra pipeline")
    return PipelineSettings, run_cycle, PipelineWorker


def main(argv: Sequence[str] | None = None) -> int:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
    parser = argparse.ArgumentParser(prog="python -m garda_ml.pipeline", description="Pipeline training Garda Inflasi")
    parser.add_argument("command", choices=("run", "worker"), help="run: satu siklus; worker: daemon per jam")
    parser.add_argument("--force", "--paksa", dest="force", action="store_true", help="latih ulang siklus jam berjalan")
    args = parser.parse_args(list(argv) if argv is not None else None)

    PipelineSettings, run_cycle, PipelineWorker = _load_pipeline()
    settings = PipelineSettings.from_env()

    if args.command == "worker":
        return PipelineWorker(settings).run_forever()

    result = run_cycle(settings, force=args.force)
    print(json.dumps({k: v for k, v in result.items() if k != "model_metrics"}, ensure_ascii=False, default=str))
    return 0 if result.get("status") in ("BERHASIL", "SEBAGIAN") or result.get("dilewati") else 1
