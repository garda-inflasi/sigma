"""Top-level module runner for `python -m garda_crawler`."""

import sys

from garda_crawler.entrypoints.cli import main

if __name__ == "__main__":
    sys.exit(main())
