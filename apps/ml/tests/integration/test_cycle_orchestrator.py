"""End-to-end integration tests for `CycleOrchestrator` against real MongoDB (GARDA_UJI_MONGO_URI)."""

from datetime import date, datetime, timedelta, timezone

import pytest

from garda_ml.config.settings import PipelineSettings
from garda_ml.data.lock_manager import LOCK_COLLECTION, CycleLeaseLock
from garda_ml.data.safe_writer import (
    CYCLE_COLLECTION,
    EWS_COLLECTION,
    FORECAST_COLLECTION,
    INFLATION_COLLECTION,
    MACRO_COLLECTION,
    OUTPUT_COLLECTIONS,
    REGION_CLUSTERS_COLLECTION,
    XAI_COLLECTION,
)
from garda_ml.orchestration.cycle_runner import run_cycle
from tests.integration.fixtures import synthetic

pytestmark = pytest.mark.integrasi

END_DAY = date(2026, 6, 30)
HOUR_TS = datetime(2026, 6, 30, 10, 12, tzinfo=timezone.utc)
CYCLE_ID = "C20260630T10Z"
COMMODITIES = {"CBM-KRT": 45000.0, "BRS-MED": 14000.0}


def _cfg(db_pair) -> PipelineSettings:
    return PipelineSettings(
        mongo_uri="mongodb://unused",
        db_unseen=db_pair[0],
        db_safe=db_pair[1],
        n_jobs=1,
        max_train_rows=15_000,
    )


def test_full_cycle_clustering_forecast_classification_xai(mongo_client, db_pair):
    synthetic.seed_dataunseen(mongo_client[db_pair[0]], END_DAY, days=420, commodities=COMMODITIES)
    result = run_cycle(_cfg(db_pair), mongo_client, HOUR_TS)
    db = mongo_client[db_pair[1]]

    assert result["status"] == "BERHASIL", result["pesan"]
    cycle = db[CYCLE_COLLECTION].find_one({"_id": CYCLE_ID})
    assert cycle["target_inflasi"] == "INDEKS_PANGAN"
    assert db[FORECAST_COLLECTION].count_documents({"cycle_id": CYCLE_ID}) == 4
    assert db[REGION_CLUSTERS_COLLECTION].count_documents({"cycle_id": CYCLE_ID}) == 2
    assert db[MACRO_COLLECTION].count_documents({"cycle_id": CYCLE_ID}) == 4
    assert cycle["model_metrics"]["clustering"]["model"] == "kmeans"


def test_idempotency_and_tiered_fast_refit(mongo_client, db_pair):
    synthetic.seed_dataunseen(mongo_client[db_pair[0]], END_DAY, days=420, commodities=COMMODITIES)
    cfg = _cfg(db_pair)
    first = run_cycle(cfg, mongo_client, HOUR_TS)
    second = run_cycle(cfg, mongo_client, HOUR_TS + timedelta(minutes=30))
    assert first["status"] == "BERHASIL" and second["dilewati"] is True

    next_hour = run_cycle(cfg, mongo_client, HOUR_TS + timedelta(hours=1))
    assert next_hour["model_metrics"]["regresi"]["tingkat"] == "FIT_ULANG"
    assert next_hour["model_metrics"]["regresi"]["seleksi_cycle_id"] == CYCLE_ID

    next_day = run_cycle(cfg, mongo_client, HOUR_TS + timedelta(hours=24))
    assert next_day["model_metrics"]["regresi"]["tingkat"] == "PENUH"


def test_empty_unseen_yields_partial_with_critical_ews(mongo_client, db_pair):
    result = run_cycle(_cfg(db_pair), mongo_client, HOUR_TS)
    db = mongo_client[db_pair[1]]
    assert result["status"] == "SEBAGIAN"
    assert db[EWS_COLLECTION].find_one({"_id": f"{CYCLE_ID}:SUMBER_DATA:HARGA:SEMUA"})["severity"] == "KRITIS"


def test_lease_lock_prevents_overlap_and_fences_zombie(mongo_client, db_pair):
    db = mongo_client[db_pair[1]]
    a = CycleLeaseLock(db, 60)
    assert a.acquire()
    db[LOCK_COLLECTION].update_one({"_id": CycleLeaseLock.LOCK_NAME}, {"$set": {"kedaluwarsa": datetime.now(timezone.utc) - timedelta(seconds=1)}})
    b = CycleLeaseLock(db, 60)
    assert b.acquire()
    with pytest.raises(RuntimeError):
        a.assert_held()
    b.assert_held()
    a.release()
    b.release()
