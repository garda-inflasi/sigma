"""End-to-end training cycle orchestrator with distributed lease locks and atomic commits."""

import logging
import time
from collections.abc import Callable
from datetime import UTC, datetime, timedelta
from typing import Any

import numpy as np
import pandas as pd
from pymongo import DESCENDING, MongoClient
from pymongo.database import Database
from pymongo.errors import DuplicateKeyError
from threadpoolctl import threadpool_limits

from garda_ml import __pipeline_version__, __version__
from garda_ml.config.constants import DEFAULT_HORIZONS
from garda_ml.config.settings import PipelineSettings
from garda_ml.data.artifact_store import ArtifactStore
from garda_ml.data.lock_manager import CycleLeaseLock
from garda_ml.data.safe_writer import (
    CYCLE_COLLECTION,
    EWS_COLLECTION,
    FINAL_READABLE_STATUSES,
    FORECAST_COLLECTION,
    INFLATION_COLLECTION,
    MACRO_COLLECTION,
    OUTPUT_COLLECTIONS,
    REGION_CLUSTERS_COLLECTION,
    XAI_COLLECTION,
    cleanup_partial_cycle,
    delete_cycle,
    ensure_datasafe_indexes,
    sanitize_bson,
    write_documents,
)
from garda_ml.data.time_utils import build_cycle_id, ensure_utc, hour_floor_utc, to_wib_date
from garda_ml.data.unseen_reader import extract_unseen_snapshot
from garda_ml.domain.entities import FeaturePanel, RawMarketDataset, RegionCluster
from garda_ml.domain.enums import CycleStatus, ModelFamily
from garda_ml.evaluation.calibration import ClassificationSummary, run_risk_classification
from garda_ml.evaluation.selector import BASELINES, RegressionSummary, run_regression_tournament
from garda_ml.features.builder import build_feature_panel
from garda_ml.features.cluster_features import inject_cluster_features, summarize_regency_profiles
from garda_ml.models.backend_resolver import ResolvedBackend, resolve_boosting_backend
from garda_ml.models.clustering.kmeans import RegencyKMeansClusterer
from garda_ml.postprocessing.anomaly_detector import detect_price_anomalies
from garda_ml.postprocessing.explainability import explain_forecasts, explain_inflation_signals
from garda_ml.postprocessing.signal_builder import consolidate_ews_signals

logger = logging.getLogger("garda_ml.pipeline")

ProgressCallback = Callable[[str, int, str, str | None], None]


def _compose_code_version(settings: PipelineSettings) -> str:
    base = f"{__version__}+pipeline.{__pipeline_version__}"
    return f"{base}+{settings.code_version}" if settings.code_version else base


def cleanup_interrupted_cycles(db: Database) -> list[str]:
    """Mark leftover `BERJALAN` cycles from dead workers as `GAGAL` and purge uncommitted docs."""
    cleaned: list[str] = []
    for doc in db[CYCLE_COLLECTION].find({"status": CycleStatus.RUNNING}, {"_id": 1}):
        cleanup_partial_cycle(db, doc["_id"])
        db[CYCLE_COLLECTION].update_one(
            {"_id": doc["_id"], "status": CycleStatus.RUNNING},
            {"$set": {"status": CycleStatus.FAILED, "pesan": "siklus terputus sebelum selesai; hasil parsial dibersihkan"}},
        )
        cleaned.append(doc["_id"])
    return cleaned


def _latest_final_cycle(db: Database, before: datetime) -> dict[str, Any] | None:
    return db[CYCLE_COLLECTION].find_one(
        {"status": {"$in": list(FINAL_READABLE_STATUSES)}, "cycle_timestamp": {"$lt": before}},
        sort=[("cycle_timestamp", DESCENDING)],
    )


def _load_previous_champions(db: Database, cycle_doc: dict[str, Any] | None) -> dict[tuple[str, str], dict[str, Any]]:
    if not cycle_doc:
        return {}
    champions: dict[tuple[str, str], dict[str, Any]] = {}
    cursor = db[FORECAST_COLLECTION].find(
        {"cycle_id": cycle_doc["_id"]},
        {"komoditas_kode": 1, "kabupaten_kode": 1, "model": 1, "metrik": 1},
    )
    for doc in cursor:
        champions[(doc["komoditas_kode"], doc["kabupaten_kode"])] = {
            "nama": doc["model"]["nama"],
            "metrik": doc["metrik"],
            "n_kandidat": doc["model"].get("n_kandidat"),
        }
    return champions


def resolve_selection_tier(
    previous_cycle: dict[str, Any] | None,
    cycle_ts: datetime,
    interval_hours: int,
    global_model_key: str,
) -> tuple[bool, dict[str, Any] | None, str | None]:
    """Decide whether to run full walk-forward selection (`True`) or fast refit (`False`)."""
    if not previous_cycle:
        return True, None, None
    reg_metrics = (previous_cycle.get("model_metrics") or {}).get("regresi") or {}
    last_sel_ts = reg_metrics.get("seleksi_timestamp")
    if last_sel_ts is None or reg_metrics.get("model_global") != global_model_key:
        return True, None, None
    if last_sel_ts.tzinfo is None:
        last_sel_ts = last_sel_ts.replace(tzinfo=UTC)
    if cycle_ts - last_sel_ts >= timedelta(hours=interval_hours):
        return True, None, None
    return False, reg_metrics.get("params"), reg_metrics.get("seleksi_cycle_id")


def _build_macro_docs(dataset: RawMarketDataset, meta: dict[str, Any]) -> list[dict[str, Any]]:
    docs: list[dict[str, Any]] = []
    for row in dataset.latest_macro.to_dict("records"):
        w = row["wilayah_kode"] if isinstance(row["wilayah_kode"], str) else None
        docs.append(
            {
                "_id": f"{meta['cycle_id']}:{row['domain']}:{row['indikator']}:{w or 'NASIONAL'}",
                **meta,
                "domain": row["domain"],
                "indikator": row["indikator"],
                "wilayah_kode": w,
                "periode": row["periode"],
                "nilai": row["nilai"],
                "satuan": row["satuan"],
                "perubahan": row["perubahan"],
                "sumber": row["sumber"],
                "is_carried_forward": bool(row["is_carried_forward"]),
                "last_available_date": row["last_available_date"],
            }
        )
    return docs


def _build_forecast_docs(
    reg: RegressionSummary,
    cutoff: pd.Timestamp,
    history_length: int,
    meta: dict[str, Any],
) -> list[dict[str, Any]]:
    docs: list[dict[str, Any]] = []
    for res in reg.results:
        s = res.series
        k, w = s.commodity_code, s.regency_code
        original_indices = np.flatnonzero(s.is_original)[-history_length:]
        curve = [
            {
                "tanggal": cutoff + pd.Timedelta(days=h),
                "horizon": h,
                "p10": round(float(q[0]), 2),
                "p50": round(float(q[1]), 2),
                "p90": round(float(q[2]), 2),
            }
            for h, q in zip(DEFAULT_HORIZONS, res.quantiles)
        ]
        doc = {
            "_id": f"{meta['cycle_id']}:{k}:{w}",
            **meta,
            "komoditas_kode": k,
            "kabupaten_kode": w,
            "harga_terakhir": round(s.latest_price, 2),
            "tanggal_terakhir": s.latest_date,
            "satuan": s.unit,
            "model": {
                "nama": res.model_name,
                "fallback": bool(res.is_fallback),
                "alasan": res.reason,
                "n_kandidat": res.candidate_count,
            },
            "metrik": res.metrics,
            "kurva": curve,
            "riwayat": [
                {"tanggal": s.dates[idx], "nilai": round(float(np.exp(s.log_prices[idx])), 2)}
                for idx in original_indices
            ],
        }
        if res.cluster_id is not None:
            doc["cluster_id"] = res.cluster_id
        docs.append(doc)
    return docs


def _build_cluster_docs(clusters: dict[str, RegionCluster], meta: dict[str, Any]) -> list[dict[str, Any]]:
    return [
        {
            "_id": f"{meta['cycle_id']}:{code}",
            **meta,
            "regency_code": c.regency_code,
            "cluster_id": c.cluster_id,
            "cluster_label": c.cluster_label,
            "centroid_distance": c.centroid_distance,
            "volatility_score": c.volatility_score,
            "climate_exposure_score": c.climate_exposure_score,
        }
        for code, c in sorted(clusters.items())
    ]


def _project_regency_index_change(reg: RegressionSummary) -> dict[str, float]:
    by_region: dict[str, list[float]] = {}
    idx = list(DEFAULT_HORIZONS).index(30)
    for res in reg.results:
        pct = float(res.quantiles[idx, 1] / res.series.latest_price - 1) * 100
        w = res.series.regency_code
        by_region.setdefault(w, []).append(pct)
        if len(w) > 2:
            by_region.setdefault(w[:2], []).append(pct)
    return {w: float(np.mean(vals)) for w, vals in by_region.items()}


def _build_regression_metrics(
    reg: RegressionSummary,
    global_model_key: str,
    selection_id: str,
    selection_ts: datetime,
) -> dict[str, Any]:
    winners = {
        ModelFamily.LIGHTGBM_QUANTILE: 0,
        ModelFamily.SARIMAX: 0,
        ModelFamily.HOLT_WINTERS: 0,
        ModelFamily.SEASONAL_NAIVE: 0,
        ModelFamily.NAIVE: 0,
        **reg.winner_counts(),
    }
    beats = [res.metrics.get("mengalahkan_baseline") for res in reg.results]
    n_series = len(reg.results)
    n_baseline = sum(winners.get(b, 0) for b in BASELINES)

    def _median(field_name: str) -> float | None:
        vals = [res.metrics.get(field_name) for res in reg.results if res.metrics.get(field_name) is not None]
        return float(np.median(vals)) if vals else None

    return {
        "pemenang": winners,
        "mape_median": _median("mape"),
        "rmse_median": _median("rmse"),
        "mape_baseline_median": _median("mape_baseline"),
        "tingkat": reg.tier,
        "params": reg.params,
        "model_global": global_model_key,
        "seleksi_cycle_id": selection_id,
        "seleksi_timestamp": selection_ts,
        "n_seri": n_series,
        "n_baseline": n_baseline,
        "porsi_baseline": n_baseline / n_series if n_series else None,
        "mengalahkan_baseline": sum(flag is True for flag in beats),
        "n_dievaluasi_holdout": sum(flag is not None for flag in beats),
        "n_diseleksi": reg.selected_count,
        "n_dilewati": len(reg.skipped),
        "n_gagal": len(reg.failed),
    }


def _resolve_final_status(
    dataset: RawMarketDataset,
    reg: RegressionSummary,
    cls: ClassificationSummary,
    failures: list[dict[str, Any]],
) -> str:
    if dataset.prices.empty or not reg.results or failures or not cls.signals:
        return CycleStatus.PARTIAL
    return CycleStatus.SUCCEEDED


class CycleOrchestrator:
    """Executes one deterministic training, clustering, forecasting, XAI, and EWS commit cycle."""

    def __init__(self, settings: PipelineSettings) -> None:
        self._settings = settings
        self._artifacts = ArtifactStore(settings.artifact_dir or None)

    def _run_models(
        self,
        dataset: RawMarketDataset,
        db_safe: Database,
        ts: datetime,
        cycle_id: str,
        previous_cycle: dict[str, Any] | None,
        progress_cb: ProgressCallback | None = None,
    ) -> dict[str, Any]:
        stage_ms: dict[str, int] = {}
        notes: list[str] = []
        failures: list[dict[str, Any]] = []

        def _report(stage: str, pct: int, message: str) -> None:
            if progress_cb is not None:
                progress_cb(stage, pct, message, cycle_id)

        backend: ResolvedBackend = resolve_boosting_backend(
            n_jobs=self._settings.n_jobs,
            force_hgb=self._settings.force_hgb_fallback,
        )
        if backend.is_fallback:
            notes.append(f"backend {backend.name}: {backend.fallback_reason}")

        _report("BANGUN_PANEL", 25, "Membangun panel fitur deret waktu, klaster K-Means & variabel makro...")
        t0 = time.perf_counter()
        cutoff_date = pd.Timestamp(to_wib_date(ts))
        panel: FeaturePanel = build_feature_panel(dataset, cutoff_date)

        clusterer = RegencyKMeansClusterer()
        clusters: dict[str, RegionCluster] = {}
        if not panel.is_empty:
            profiles = summarize_regency_profiles(panel)
            clusters = clusterer.fit_predict(profiles)
            panel = inject_cluster_features(panel, clusters)
        stage_ms["fitur"] = int((time.perf_counter() - t0) * 1000)

        _report("TRAIN_REGRESI", 45, "Melatih & memilih model proyeksi harga H+7, H+14, H+30...")
        t0 = time.perf_counter()
        is_full, prev_params, sel_cycle_id = resolve_selection_tier(
            previous_cycle,
            ts,
            self._settings.selection_interval_hours,
            backend.quantile_model_key,
        )
        champions = {} if is_full else _load_previous_champions(db_safe, previous_cycle)
        reg = run_regression_tournament(
            panel,
            backend,
            self._settings,
            previous_champions=champions,
            full_selection=is_full,
            previous_params=prev_params,
        )
        stage_ms["regresi"] = int((time.perf_counter() - t0) * 1000)
        failures.extend({"tahap": "regresi", **item} for item in reg.failed)

        _report("TRAIN_KLASIFIKASI", 65, "Melatih model klasifikasi probabilitas tekanan inflasi...")
        t0 = time.perf_counter()
        try:
            cls = run_risk_classification(panel, dataset, backend, _project_regency_index_change(reg))
        except Exception as err:
            logger.exception("risk classification failed")
            failures.append({"tahap": "klasifikasi", "alasan": str(err)[:200]})
            cls = ClassificationSummary(
                metrics={
                    "model": None,
                    "f1": None,
                    "roc_auc": None,
                    "brier": None,
                    "brier_baseline": None,
                    "n_uji": 0,
                    "layak": False,
                    "alasan": f"gagal: {str(err)[:150]}",
                }
            )
        notes.extend(cls.notes)
        stage_ms["klasifikasi"] = int((time.perf_counter() - t0) * 1000)

        _report("XAI_SHAP", 80, "Menghitung atribusi faktor SHAP & penjelasan model...")
        t0 = time.perf_counter()
        xai_fc, xai_inf = [], []
        if reg.results:
            xai_fc, err_fc = explain_forecasts(
                reg.results,
                panel.frame,
                reg.feature_columns,
                reg.global_models,
                dataset.commodity_names,
                panel.cutoff,
            )
            failures.extend(err_fc)
        if cls.signals:
            xai_inf, err_inf = explain_inflation_signals(cls)
            failures.extend(err_inf)
        stage_ms["xai"] = int((time.perf_counter() - t0) * 1000)

        _report("EWS_ANOMALI", 90, "Mendeteksi anomali harga & sinyal peringatan dini...")
        t0 = time.perf_counter()
        anomalies: list[dict[str, Any]] = []
        try:
            anomalies = detect_price_anomalies(panel.series_map, dataset.commodity_names)
        except Exception as err:
            logger.exception("anomaly detection failed")
            failures.append({"tahap": "anomali", "alasan": str(err)[:200]})

        prev_active_ews = (
            list(db_safe[EWS_COLLECTION].find({"cycle_id": previous_cycle["_id"], "aktif": True}))
            if previous_cycle
            else []
        )
        ews = consolidate_ews_signals(
            anomaly_signals=anomalies,
            forecast_results=reg.results,
            inflation_signals=cls.signals,
            freshness=dataset.freshness,
            has_prices=not dataset.prices.empty,
            commodity_names=dataset.commodity_names,
            previous_active_signals=prev_active_ews,
        )
        stage_ms["ews"] = int((time.perf_counter() - t0) * 1000)

        return {
            "panel": panel,
            "clusters": clusters,
            "clusterer": clusterer,
            "reg": reg,
            "cls": cls,
            "xai": xai_fc + xai_inf,
            "ews": ews,
            "stage_ms": stage_ms,
            "notes": notes,
            "failures": failures,
            "backend": backend,
            "selection_id": cycle_id if is_full else (sel_cycle_id or cycle_id),
            "selection_ts": (
                ts
                if is_full
                else (previous_cycle or {}).get("model_metrics", {}).get("regresi", {}).get("seleksi_timestamp", ts)
            ),
        }

    def _commit_datasafe(
        self,
        db_safe: Database,
        lock: CycleLeaseLock,
        ts: datetime,
        cycle_id: str,
        dataset: RawMarketDataset,
        res: dict[str, Any],
        previous_cycle: dict[str, Any] | None,
        started_perf: float,
    ) -> dict[str, Any]:
        reg: RegressionSummary = res["reg"]
        cls: ClassificationSummary = res["cls"]
        panel: FeaturePanel = res["panel"]
        cutoff_date = panel.cutoff if panel.cutoff is not None else pd.Timestamp(to_wib_date(ts))
        meta = {"cycle_id": cycle_id, "cycle_timestamp": ts}

        documents: dict[str, list[dict[str, Any]]] = {
            MACRO_COLLECTION: _build_macro_docs(dataset, meta),
            FORECAST_COLLECTION: _build_forecast_docs(reg, cutoff_date, self._settings.output_history_length, meta),
            XAI_COLLECTION: [
                {"_id": f"{cycle_id}:{x['target']}:{x['komoditas_kode'] or '-'}:{x['wilayah_kode']}", **meta, **x}
                for x in res["xai"]
            ],
            INFLATION_COLLECTION: [
                {"_id": f"{cycle_id}:{sig['wilayah_kode']}", **meta, **sig}
                for sig in cls.signals
            ],
            EWS_COLLECTION: [
                {"_id": f"{cycle_id}:{sig['sinyal_kunci']}", **meta, **sig}
                for sig in res["ews"]
            ],
        }

        t_write = time.perf_counter()
        for col_name in OUTPUT_COLLECTIONS:
            lock.assert_held()
            write_documents(db_safe, col_name, documents[col_name])

        cluster_docs = _build_cluster_docs(res["clusters"], meta)
        if cluster_docs:
            lock.assert_held()
            write_documents(db_safe, REGION_CLUSTERS_COLLECTION, cluster_docs)

        res["stage_ms"]["tulis"] = int((time.perf_counter() - t_write) * 1000)

        if self._artifacts.enabled and reg.global_models:
            self._artifacts.save_joblib(
                cycle_id,
                f"global_{res['backend'].quantile_model_key}",
                reg.global_models,
                metadata={"cycle_id": cycle_id, "features": reg.feature_columns},
            )

        counts = {col_name: db_safe[col_name].count_documents({"cycle_id": cycle_id}) for col_name in OUTPUT_COLLECTIONS}
        for col_name, items in documents.items():
            if counts[col_name] != len(items):
                raise RuntimeError(f"jumlah dokumen {col_name} tidak cocok: {counts[col_name]} != {len(items)}")

        status = _resolve_final_status(dataset, reg, cls, res["failures"])
        notes = list(res["notes"])
        if dataset.prices.empty:
            notes.insert(0, "tidak ada titik harga asli yang dapat dipakai")
        elif previous_cycle is not None and dataset.new_points_count == 0:
            notes.insert(0, f"tidak ada data baru sejak siklus {previous_cycle['_id']}")
        if reg.skipped:
            notes.append(f"{len(reg.skipped)} seri dilewati karena riwayat pendek atau data terlalu basi")
        if res["failures"]:
            notes.append(f"{len(res['failures'])} kegagalan parsial")

        duration_ms = int((time.perf_counter() - started_perf) * 1000)
        clusterer: RegencyKMeansClusterer = res["clusterer"]
        metrics = {
            "klasifikasi": cls.metrics,
            "regresi": _build_regression_metrics(
                reg,
                res["backend"].quantile_model_key,
                res["selection_id"],
                res["selection_ts"],
            ),
            "clustering": {
                "model": ModelFamily.KMEANS,
                "k": clusterer.selected_k,
                "silhouette": clusterer.best_silhouette,
                "n_regions": len(res["clusters"]),
            },
            "durasi_tahap_ms": res["stage_ms"],
            "kegagalan": res["failures"][:50],
            "jumlah_data_baru": dataset.new_points_count,
        }

        lock.assert_held()
        update_res = db_safe[CYCLE_COLLECTION].update_one(
            {"_id": cycle_id, "status": CycleStatus.RUNNING},
            {
                "$set": sanitize_bson(
                    {
                        "status": status,
                        "pesan": "; ".join(notes),
                        "duration_ms": duration_ms,
                        "jumlah": counts,
                        "source_freshness": dataset.freshness,
                        "target_inflasi": cls.cycle_target if cls.signals else None,
                        "model_metrics": metrics,
                    }
                )
            },
        )
        if update_res.modified_count != 1:
            raise RuntimeError(f"gagal memfinalkan siklus {cycle_id}")

        logger.info("siklus %s %s dalam %.1f dtk: %s", cycle_id, status, duration_ms / 1000, counts)
        return {
            "cycle_id": cycle_id,
            "status": status,
            "dilewati": False,
            "duration_ms": duration_ms,
            "jumlah": counts,
            "pesan": "; ".join(notes),
            "model_metrics": metrics,
            "target_inflasi": cls.cycle_target if cls.signals else None,
        }

    def run(
        self,
        *,
        client: MongoClient[Any] | None = None,
        now: datetime | None = None,
        progress_cb: ProgressCallback | None = None,
        force: bool = False,
    ) -> dict[str, Any]:
        """Run one isolated cycle guarded by MongoDB lease lock."""
        owns_client = client is None
        mongo_client = client or MongoClient(self._settings.mongo_uri, serverSelectionTimeoutMS=5000, tz_aware=True)
        try:
            moment = ensure_utc(now or datetime.now(UTC))
            ts = hour_floor_utc(moment)
            cutoff = moment if force else ts
            cycle_id = build_cycle_id(ts)
            db_safe, db_unseen = mongo_client[self._settings.db_safe], mongo_client[self._settings.db_unseen]
            ensure_datasafe_indexes(db_safe)

            if not force:
                existing = db_safe[CYCLE_COLLECTION].find_one({"_id": cycle_id}, {"status": 1})
                if existing and existing["status"] in FINAL_READABLE_STATUSES:
                    logger.info("siklus %s sudah final (%s), dilewati", cycle_id, existing["status"])
                    return {"cycle_id": cycle_id, "status": existing["status"], "dilewati": True, "alasan": "sudah final"}

            lock = CycleLeaseLock(db_safe, self._settings.lease_seconds)
            if not lock.acquire():
                logger.info("siklus lain sedang berjalan, %s dilewati", cycle_id)
                return {"cycle_id": cycle_id, "status": None, "dilewati": True, "alasan": "kunci dipegang pekerja lain"}

            started_perf = time.perf_counter()
            try:
                if force:
                    delete_cycle(db_safe, cycle_id)
                else:
                    existing = db_safe[CYCLE_COLLECTION].find_one({"_id": cycle_id}, {"status": 1})
                    if existing and existing["status"] in FINAL_READABLE_STATUSES:
                        return {"cycle_id": cycle_id, "status": existing["status"], "dilewati": True, "alasan": "sudah final"}
                    cleanup_interrupted_cycles(db_safe)
                    cleanup_partial_cycle(db_safe, cycle_id)

                base_doc = {
                    "cycle_id": cycle_id,
                    "cycle_timestamp": ts,
                    "data_cutoff": cutoff,
                    "duration_ms": None,
                    "status": CycleStatus.RUNNING,
                    "pesan": "",
                    "versi_kode": _compose_code_version(self._settings),
                    "jumlah": {},
                    "source_freshness": [],
                    "target_inflasi": None,
                    "model_metrics": {},
                }
                try:
                    db_safe[CYCLE_COLLECTION].insert_one({"_id": cycle_id, **base_doc})
                except DuplicateKeyError:
                    db_safe[CYCLE_COLLECTION].update_one({"_id": cycle_id, "status": CycleStatus.FAILED}, {"$set": base_doc})

                try:
                    if progress_cb is not None:
                        progress_cb("EKSTRAKSI_FITUR", 12, f"Mengekstrak data mentah dataunseen ({cycle_id})...", cycle_id)
                    previous_cycle = _latest_final_cycle(db_safe, ts)
                    dataset = extract_unseen_snapshot(
                        db_unseen,
                        cutoff=cutoff,
                        history_days=self._settings.history_days,
                        previous_cutoff=previous_cycle["data_cutoff"] if previous_cycle else None,
                    )
                    with threadpool_limits(limits=self._settings.n_jobs):
                        computed = self._run_models(
                            dataset,
                            db_safe,
                            ts,
                            cycle_id,
                            previous_cycle,
                            progress_cb=progress_cb,
                        )
                    if progress_cb is not None:
                        progress_cb("SIMPAN_DATASAFE", 96, f"Menyimpan hasil & memfinalkan siklus {cycle_id} di datasafe...", cycle_id)
                    return self._commit_datasafe(
                        db_safe,
                        lock,
                        ts,
                        cycle_id,
                        dataset,
                        computed,
                        previous_cycle,
                        started_perf,
                    )
                except Exception as err:
                    logger.exception("siklus %s gagal", cycle_id)
                    try:
                        lock.assert_held()
                        cleanup_partial_cycle(db_safe, cycle_id)
                        db_safe[CYCLE_COLLECTION].update_one(
                            {"_id": cycle_id, "status": CycleStatus.RUNNING},
                            {
                                "$set": {
                                    "status": CycleStatus.FAILED,
                                    "pesan": f"{type(err).__name__}: {str(err)[:400]}",
                                    "duration_ms": int((time.perf_counter() - started_perf) * 1000),
                                }
                            },
                        )
                    except Exception:
                        logger.exception("gagal menandai siklus %s sebagai GAGAL", cycle_id)
                    raise
            finally:
                lock.release()
        finally:
            if owns_client:
                mongo_client.close()


def run_cycle(
    settings: PipelineSettings,
    client: MongoClient[Any] | None = None,
    now: datetime | None = None,
    progress_cb: ProgressCallback | None = None,
    force: bool = False,
) -> dict[str, Any]:
    """Convenience entrypoint running a single cycle via `CycleOrchestrator`."""
    return CycleOrchestrator(settings).run(client=client, now=now, progress_cb=progress_cb, force=force)
