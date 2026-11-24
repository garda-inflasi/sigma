"""Fixture tes pipeline. Tes integrasi butuh ``GARDA_UJI_MONGO_URI`` ke kontainer uji sendiri."""

from __future__ import annotations

import os
import uuid

import pytest


@pytest.fixture
def klien_mongo():
    uri = os.environ.get("GARDA_UJI_MONGO_URI", "").strip()
    if not uri:
        pytest.skip("GARDA_UJI_MONGO_URI tidak diisi")
    from pymongo import MongoClient

    klien = MongoClient(uri, serverSelectionTimeoutMS=5000, tz_aware=True)
    klien.admin.command("ping")
    yield klien
    klien.close()


@pytest.fixture
def nama_db(klien_mongo):
    """Pasangan database unik per tes, dihapus setelah selesai."""
    akhiran = uuid.uuid4().hex[:10]
    unseen, safe = f"uji_unseen_{akhiran}", f"uji_safe_{akhiran}"
    yield unseen, safe
    klien_mongo.drop_database(unseen)
    klien_mongo.drop_database(safe)
