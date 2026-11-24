"""Shared test fixtures, mongomock compatibility patch, and HTTP test helpers."""

import json
from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import httpx
import mongomock
import pytest

from garda_crawler.storage.unseen_repository import UnseenRepository
from garda_crawler.transport.polite_http import PoliteHttpClient

FIXTURE_DIR = Path(__file__).parent / "fixtures"
NOW_UTC = datetime(2026, 11, 24, 3, 0, tzinfo=UTC)  # 10:00 WIB

# mongomock 4.3 does not accept the `sort` keyword forwarded by pymongo >= 4.11 ReplaceOne.
_orig_add_replace = mongomock.collection.BulkOperationBuilder.add_replace


def _patched_add_replace(self, selector, doc, upsert, collation=None, hint=None, sort=None, **_):
    return _orig_add_replace(self, selector, doc, upsert, collation=collation, hint=hint)


mongomock.collection.BulkOperationBuilder.add_replace = _patched_add_replace


def load_fixture(filename: str) -> Any:
    """Load recorded JSON fixture payload."""
    return json.loads((FIXTURE_DIR / filename).read_text(encoding="utf-8"))


@pytest.fixture
def db():
    return mongomock.MongoClient(tz_aware=True)["dataunseen"]


@pytest.fixture
def repository(db) -> UnseenRepository:
    repo = UnseenRepository(db)
    repo.ensure_indexes()
    return repo


def make_http_client(
    handler: Callable[[httpx.Request], httpx.Response],
    **kwargs: Any,
) -> PoliteHttpClient:
    """Create a PoliteHttpClient backed by httpx.MockTransport and no-op sleeper."""
    kwargs.setdefault("delay_ms", 0)
    kwargs.setdefault("max_attempts", 1)
    return PoliteHttpClient(
        "garda-crawler/test",
        transport=httpx.MockTransport(handler),
        sleep_fn=lambda _: None,
        **kwargs,
    )


def without_robots(
    handler: Callable[[httpx.Request], httpx.Response],
) -> Callable[[httpx.Request], httpx.Response]:
    """Wrap mock handler to return 404 for /robots.txt."""

    def _wrapped(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/robots.txt":
            return httpx.Response(404)
        return handler(request)

    return _wrapped
