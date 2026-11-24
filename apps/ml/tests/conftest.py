"""Shared fixtures for pipeline integration tests (enabled via GARDA_UJI_MONGO_URI)."""

import os
import uuid

import pytest


@pytest.fixture
def mongo_client():
    uri = os.environ.get("GARDA_UJI_MONGO_URI", "").strip()
    if not uri:
        pytest.skip("GARDA_UJI_MONGO_URI tidak diisi")
    from pymongo import MongoClient

    client = MongoClient(uri, serverSelectionTimeoutMS=5000, tz_aware=True)
    client.admin.command("ping")
    yield client
    client.close()


@pytest.fixture
def db_pair(mongo_client):
    """Unique `(unseen_db, safe_db)` database pair cleaned up after each test."""
    suffix = uuid.uuid4().hex[:10]
    unseen, safe = f"uji_unseen_{suffix}", f"uji_safe_{suffix}"
    yield unseen, safe
    mongo_client.drop_database(unseen)
    mongo_client.drop_database(safe)
