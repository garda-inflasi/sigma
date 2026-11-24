"""Entrypoints package exports."""

from garda_ml.entrypoints.worker_cli import main as worker_main

__all__ = ["worker_main"]
