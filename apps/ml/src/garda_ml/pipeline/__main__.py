"""Module runner preserving `python -m garda_ml.pipeline {run,worker}`."""

import sys

from garda_ml.entrypoints.worker_cli import main

if __name__ == "__main__":
    sys.exit(main())
