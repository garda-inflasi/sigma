"""Orkestrasi satu siklus: kunci, ekstraksi, model, tulis, lalu status final terakhir.

Jaminan:

- ``cycle_id`` per jam UTC. Siklus final tidak pernah dijalankan atau ditulis ulang.
- Dokumen hasil tidak pernah diubah setelah siklus final. Sisa tulisan siklus yang terputus
  (belum pernah terlihat pembaca) dibersihkan sebelum diulang.
- Status final ditulis paling akhir, setelah jumlah dokumen terverifikasi.
- Kegagalan per seri atau per tahap menghasilkan ``SEBAGIAN``, bukan menjatuhkan siklus.
"""

from __future__ import annotations

import logging
import time
from collections.abc import Callable
from datetime import datetime, timedelta, timezone
from typing import Any

import numpy as np
import pandas as pd
from pymongo import DESCENDING, MongoClient
from pymongo.database import Database
from pymongo.errors import DuplicateKeyError
from threadpoolctl import threadpool_limits

from .. import VERSI
from . import VERSI_PIPELINE
from .anomali import deteksi_anomali, sinyal_proyeksi, sinyal_sumber, sinyal_tekanan, tandai_pulih
from .backend import pilih_backend
from .config import HORIZON, Konfigurasi
from .ekstraksi import DataMentah, ekstrak
from .fitur import Panel, bangun_panel
from .klasifikasi import HasilKlasifikasi, jalankan_klasifikasi
from .penyimpanan import (
    K_EWS,
    K_FORECAST,
    K_INFLASI,
    K_MAKRO,
    K_SIKLUS,
    K_XAI,
    KOLEKSI_HASIL,
    STATUS_FINAL_TERBACA,
    KunciSiklus,
    bersihkan,
    hapus_parsial,
    hapus_siklus,
    pastikan_indeks,
    tulis_dokumen,
)
from .regresi import BASELINES, HasilRegresi, jalankan_regresi
from .waktu import awal_jam, id_siklus, tanggal_wib, utc
from .xai import jelaskan_forecast, jelaskan_inflasi

log = logging.getLogger("garda_ml.pipeline")


def _versi_kode(cfg: Konfigurasi) -> str:
    dasar = f"{VERSI}+pipeline.{VERSI_PIPELINE}"
    return f"{dasar}+{cfg.versi_kode}" if cfg.versi_kode else dasar


def bereskan_siklus_terputus(db: Database, kecuali: str | None = None) -> list[str]:
    """Pemegang kunci menandai siklus ``BERJALAN`` milik proses mati sebagai ``GAGAL``."""
    terputus = []
    for d in db[K_SIKLUS].find({"status": "BERJALAN"}, {"_id": 1}):
        hapus_parsial(db, d["_id"])
        db[K_SIKLUS].update_one(
            {"_id": d["_id"], "status": "BERJALAN"},
            {"$set": {"status": "GAGAL", "pesan": "siklus terputus sebelum selesai; hasil parsial dibersihkan"}},
        )
        terputus.append(d["_id"])
    return terputus


def _siklus_final_terakhir(db: Database, sebelum: datetime) -> dict | None:
    return db[K_SIKLUS].find_one(
        {"status": {"$in": list(STATUS_FINAL_TERBACA)}, "cycle_timestamp": {"$lt": sebelum}},
        sort=[("cycle_timestamp", DESCENDING)],
    )


def _muat_juara(db: Database, siklus: dict | None) -> dict[tuple[str, str], dict]:
    if not siklus:
        return {}
    juara: dict[tuple[str, str], dict] = {}
    for d in db[K_FORECAST].find({"cycle_id": siklus["_id"]}, {"komoditas_kode": 1, "kabupaten_kode": 1, "model": 1, "metrik": 1}):
        juara[(d["komoditas_kode"], d["kabupaten_kode"])] = {"nama": d["model"]["nama"], "metrik": d["metrik"], "n_kandidat": d["model"].get("n_kandidat")}
    return juara


def tentukan_tingkat(siklus_lalu: dict | None, ts: datetime, interval_jam: int, nama_model_global: str) -> tuple[bool, dict | None, str | None]:
    """Kembalikan (seleksi_penuh, params lama, seleksi_cycle_id yang dipakai ulang)."""
    if not siklus_lalu:
        return True, None, None
    reg = (siklus_lalu.get("model_metrics") or {}).get("regresi") or {}
    terakhir = reg.get("seleksi_timestamp")
    if terakhir is None or reg.get("model_global") != nama_model_global:
        return True, None, None
    if terakhir.tzinfo is None:
        terakhir = terakhir.replace(tzinfo=timezone.utc)
    if ts - terakhir >= timedelta(hours=interval_jam):
        return True, None, None
    return False, reg.get("params"), reg.get("seleksi_cycle_id")


def _dokumen_makro(data: DataMentah, meta: dict) -> list[dict]:
    dok = []
    for r in data.makro_terkini.to_dict("records"):
        w = r["wilayah_kode"] if isinstance(r["wilayah_kode"], str) else None
        dok.append({
            "_id": f"{meta['cycle_id']}:{r['domain']}:{r['indikator']}:{w or 'NASIONAL'}", **meta,
            "domain": r["domain"], "indikator": r["indikator"], "wilayah_kode": w, "periode": r["periode"], "nilai": r["nilai"],
            "satuan": r["satuan"], "perubahan": r["perubahan"], "sumber": r["sumber"],
            "is_carried_forward": bool(r["is_carried_forward"]), "last_available_date": r["last_available_date"],
        })
    return dok


def _dokumen_forecast(reg: HasilRegresi, cutoff: pd.Timestamp, panjang_riwayat: int, meta: dict) -> list[dict]:
    dok = []
    for h in reg.hasil:
        s = h.seri
        k, w = s.komoditas_kode, s.kabupaten_kode
        pos_asli = np.flatnonzero(s.asli)[-panjang_riwayat:]
        kurva = [
            {"tanggal": cutoff + pd.Timedelta(days=hz), "horizon": hz, "p10": round(float(q[0]), 2), "p50": round(float(q[1]), 2), "p90": round(float(q[2]), 2)}
            for hz, q in zip(HORIZON, h.kuantil)
        ]
        dok.append({
            "_id": f"{meta['cycle_id']}:{k}:{w}", **meta, "komoditas_kode": k, "kabupaten_kode": w,
            "harga_terakhir": round(s.harga_terakhir, 2), "tanggal_terakhir": s.tanggal_terakhir, "satuan": s.satuan,
            "model": {"nama": h.nama_model, "fallback": bool(h.fallback), "alasan": h.alasan, "n_kandidat": h.n_kandidat},
            "metrik": h.metrik, "kurva": kurva,
            "riwayat": [{"tanggal": s.tanggal[i], "nilai": round(float(np.exp(s.log_harga[i])), 2)} for i in pos_asli],
        })
    return dok


def _proyeksi_indeks(reg: HasilRegresi) -> dict[str, float]:
    """Perubahan P50 H+30 rata-rata per wilayah dan provinsinya, dalam persen."""
    per_wilayah: dict[str, list[float]] = {}
    idx = list(HORIZON).index(30)
    for h in reg.hasil:
        ubah = float(h.kuantil[idx, 1] / h.seri.harga_terakhir - 1) * 100
        w = h.seri.kabupaten_kode
        per_wilayah.setdefault(w, []).append(ubah)
        if len(w) > 2:
            per_wilayah.setdefault(w[:2], []).append(ubah)
    return {w: float(np.mean(v)) for w, v in per_wilayah.items()}


def _metrik_regresi(reg: HasilRegresi, nama_global: str, seleksi_id: str, seleksi_ts: datetime) -> dict:
    pemenang = {"lightgbm_quantile": 0, "sarimax": 0, "holt_winters": 0, "seasonal_naive": 0, "naive": 0, **reg.pemenang()}
    unggul = [h.metrik.get("mengalahkan_baseline") for h in reg.hasil]
    n_seri = len(reg.hasil)
    n_baseline = sum(pemenang.get(b, 0) for b in BASELINES)
    def median(kunci: str) -> float | None:
        nilai = [h.metrik.get(kunci) for h in reg.hasil if h.metrik.get(kunci) is not None]
        return float(np.median(nilai)) if nilai else None
    return {
        "pemenang": pemenang, "mape_median": median("mape"), "rmse_median": median("rmse"),
        "mape_baseline_median": median("mape_baseline"), "tingkat": reg.tingkat, "params": reg.params,
        "model_global": nama_global, "seleksi_cycle_id": seleksi_id, "seleksi_timestamp": seleksi_ts,
        "n_seri": n_seri, "n_baseline": n_baseline, "porsi_baseline": n_baseline / n_seri if n_seri else None,
        "mengalahkan_baseline": sum(u is True for u in unggul),
        "n_dievaluasi_holdout": sum(u is not None for u in unggul), "n_diseleksi": reg.n_diseleksi, "n_dilewati": len(reg.dilewati), "n_gagal": len(reg.gagal),
    }


def _jalankan_model(
    data: DataMentah,
    db: Database,
    cfg: Konfigurasi,
    ts: datetime,
    cycle_id: str,
    siklus_lalu: dict | None,
    progress_cb: Callable[[str, int, str, str | None], None] | None = None,
) -> dict[str, Any]:
    """Tahap komputasi tanpa menulis apa pun ke Mongo."""
    tahap: dict[str, int] = {}
    catatan: list[str] = []
    gagal: list[dict] = []
    def ukur(nama: str, mulai: float) -> None:
        tahap[nama] = int((time.perf_counter() - mulai) * 1000)

    def lapor(t_nama: str, pct: int, pesan: str) -> None:
        if progress_cb is not None:
            progress_cb(t_nama, pct, pesan, cycle_id)

    backend = pilih_backend(n_thread=cfg.n_jobs)
    if backend.fallback:
        catatan.append(f"backend {backend.nama}: {backend.alasan_fallback}")

    lapor("BANGUN_PANEL", 25, "Membangun panel fitur deret waktu & variabel makro...")
    t = time.perf_counter()
    cutoff_tgl = pd.Timestamp(tanggal_wib(ts))
    panel: Panel = bangun_panel(data, cutoff_tgl)
    ukur("fitur", t)

    lapor("TRAIN_REGRESI", 45, "Melatih & memilih model proyeksi harga H+7, H+14, H+30...")
    t = time.perf_counter()
    penuh, params_lama, seleksi_id = tentukan_tingkat(siklus_lalu, ts, cfg.interval_seleksi_jam, backend.nama_kuantil)
    juara = {} if penuh else _muat_juara(db, siklus_lalu)
    reg = jalankan_regresi(panel, backend, cfg, juara=juara, seleksi_penuh=penuh, params_lama=params_lama)
    ukur("regresi", t)
    gagal.extend({"tahap": "regresi", **g} for g in reg.gagal)

    lapor("TRAIN_KLASIFIKASI", 65, "Melatih model klasifikasi probabilitas tekanan inflasi...")
    t = time.perf_counter()
    try:
        kelas = jalankan_klasifikasi(panel, data, backend, _proyeksi_indeks(reg))
    except Exception as err:  # noqa: BLE001
        log.exception("klasifikasi gagal")
        gagal.append({"tahap": "klasifikasi", "alasan": str(err)[:200]})
        kelas = HasilKlasifikasi(metrik={"model": None, "f1": None, "roc_auc": None, "brier": None, "brier_baseline": None, "n_uji": 0, "layak": False, "alasan": f"gagal: {str(err)[:150]}"})
    catatan.extend(kelas.catatan)
    ukur("klasifikasi", t)

    lapor("XAI_SHAP", 80, "Menghitung atribusi faktor SHAP & penjelasan model...")
    t = time.perf_counter()
    xai_forecast: list[dict] = []
    xai_inflasi: list[dict] = []
    if reg.hasil:
        xai_forecast, g = jelaskan_forecast(reg.hasil, panel.df, reg.kolom_fitur, reg.model_global, data.nama_komoditas, panel.cutoff)
        gagal.extend(g)
    if kelas.sinyal:
        xai_inflasi, g = jelaskan_inflasi(kelas)
        gagal.extend(g)
    ukur("xai", t)

    lapor("EWS_ANOMALI", 90, "Mendeteksi anomali harga & sinyal peringatan dini...")
    t = time.perf_counter()
    ews: list[dict] = []
    try:
        ews.extend(deteksi_anomali(panel.seri, data.nama_komoditas))
    except Exception as err:  # noqa: BLE001
        log.exception("deteksi anomali gagal")
        gagal.append({"tahap": "anomali", "alasan": str(err)[:200]})
    ews.extend(sinyal_proyeksi(reg.hasil, data.nama_komoditas))
    ews.extend(sinyal_tekanan(kelas.sinyal))
    ews.extend(sinyal_sumber(data.kesegaran, not data.harga.empty))
    if siklus_lalu:
        lalu = list(db[K_EWS].find({"cycle_id": siklus_lalu["_id"], "aktif": True}))
        ews.extend(tandai_pulih(ews, lalu))
    ukur("ews", t)

    return {
        "panel": panel, "reg": reg, "kelas": kelas, "xai": xai_forecast + xai_inflasi, "ews": ews,
        "tahap": tahap, "catatan": catatan, "gagal": gagal, "backend": backend,
        "seleksi_id": cycle_id if penuh else (seleksi_id or cycle_id),
        "seleksi_ts": ts if penuh else (siklus_lalu or {}).get("model_metrics", {}).get("regresi", {}).get("seleksi_timestamp", ts),
    }


def _status_akhir(data: DataMentah, reg: HasilRegresi, kelas: HasilKlasifikasi, gagal: list[dict]) -> str:
    if data.harga.empty or not reg.hasil or gagal or not kelas.sinyal:
        return "SEBAGIAN"
    return "BERHASIL"


def jalankan_siklus(
    cfg: Konfigurasi,
    klien: MongoClient | None = None,
    sekarang: datetime | None = None,
    progress_cb: Callable[[str, int, str, str | None], None] | None = None,
    paksa: bool = False,
) -> dict:
    """Jalankan siklus untuk jam ``sekarang``. Aman diulang pada jam yang sama."""
    milik_sendiri = klien is None
    klien = klien or MongoClient(cfg.mongo_uri, serverSelectionTimeoutMS=5000, tz_aware=True)
    try:
        return _jalankan(cfg, klien, sekarang or datetime.now(timezone.utc), progress_cb=progress_cb, paksa=paksa)
    finally:
        if milik_sendiri:
            klien.close()


def _jalankan(
    cfg: Konfigurasi,
    klien: MongoClient,
    sekarang: datetime,
    progress_cb: Callable[[str, int, str, str | None], None] | None = None,
    paksa: bool = False,
) -> dict:
    aktual = utc(sekarang)
    ts = awal_jam(aktual)
    cutoff = aktual if paksa else ts
    cycle_id = id_siklus(ts)
    db_safe, db_unseen = klien[cfg.db_safe], klien[cfg.db_unseen]
    pastikan_indeks(db_safe)
    if not paksa:
        ada = db_safe[K_SIKLUS].find_one({"_id": cycle_id}, {"status": 1})
        if ada and ada["status"] in STATUS_FINAL_TERBACA:
            log.info("siklus %s sudah final (%s), dilewati", cycle_id, ada["status"])
            return {"cycle_id": cycle_id, "status": ada["status"], "dilewati": True, "alasan": "sudah final"}

    kunci = KunciSiklus(db_safe, cfg.lease_detik)
    if not kunci.ambil():
        log.info("siklus lain sedang berjalan, %s dilewati", cycle_id)
        return {"cycle_id": cycle_id, "status": None, "dilewati": True, "alasan": "kunci dipegang pekerja lain"}
    try:
        return _dengan_kunci(cfg, db_safe, db_unseen, kunci, ts, cutoff, cycle_id, progress_cb=progress_cb, paksa=paksa)
    finally:
        kunci.lepas()


def _dengan_kunci(
    cfg: Konfigurasi,
    db_safe: Database,
    db_unseen: Database,
    kunci: KunciSiklus,
    ts: datetime,
    cutoff: datetime,
    cycle_id: str,
    progress_cb: Callable[[str, int, str, str | None], None] | None = None,
    paksa: bool = False,
) -> dict:
    mulai = time.perf_counter()
    if paksa:
        hapus_siklus(db_safe, cycle_id)
    else:
        # Pemeriksaan ulang setelah memegang kunci: pekerja lain mungkin baru menyelesaikannya.
        ada = db_safe[K_SIKLUS].find_one({"_id": cycle_id}, {"status": 1})
        if ada and ada["status"] in STATUS_FINAL_TERBACA:
            return {"cycle_id": cycle_id, "status": ada["status"], "dilewati": True, "alasan": "sudah final"}
        bereskan_siklus_terputus(db_safe)
        hapus_parsial(db_safe, cycle_id)

    dasar = {
        "cycle_id": cycle_id, "cycle_timestamp": ts, "data_cutoff": cutoff, "duration_ms": None, "status": "BERJALAN",
        "pesan": "", "versi_kode": _versi_kode(cfg), "jumlah": {}, "source_freshness": [], "target_inflasi": None, "model_metrics": {},
    }
    try:
        db_safe[K_SIKLUS].insert_one({"_id": cycle_id, **dasar})
    except DuplicateKeyError:
        db_safe[K_SIKLUS].update_one({"_id": cycle_id, "status": "GAGAL"}, {"$set": dasar})

    try:
        if progress_cb is not None:
            progress_cb("EKSTRAKSI_FITUR", 12, f"Mengekstrak data mentah dataunseen ({cycle_id})...", cycle_id)
        lalu = _siklus_final_terakhir(db_safe, ts)
        data = ekstrak(db_unseen, cutoff, cfg.hari_riwayat, lalu["data_cutoff"] if lalu else None)
        # Batasi OpenMP/BLAS agar worker tidak merebut seluruh CPU VM.
        with threadpool_limits(limits=cfg.n_jobs):
            hasil = _jalankan_model(data, db_safe, cfg, ts, cycle_id, lalu, progress_cb=progress_cb)
        if progress_cb is not None:
            progress_cb("SIMPAN_DATASAFE", 96, f"Menyimpan hasil & memfinalkan siklus {cycle_id} di datasafe...", cycle_id)
        return _tulis_dan_finalkan(cfg, db_safe, kunci, ts, cycle_id, data, hasil, lalu, mulai)
    except Exception as err:
        log.exception("siklus %s gagal", cycle_id)
        _tandai_gagal(db_safe, kunci, cycle_id, err, mulai)
        raise


def _tandai_gagal(db: Database, kunci: KunciSiklus, cycle_id: str, err: Exception, mulai: float) -> None:
    try:
        kunci.pastikan()
        hapus_parsial(db, cycle_id)
        db[K_SIKLUS].update_one(
            {"_id": cycle_id, "status": "BERJALAN"},
            {"$set": {"status": "GAGAL", "pesan": f"{type(err).__name__}: {str(err)[:400]}", "duration_ms": int((time.perf_counter() - mulai) * 1000)}},
        )
    except Exception:  # noqa: BLE001 - Mongo mungkin yang gagal; siklus akan dibereskan pekerja berikutnya
        log.exception("gagal menandai siklus %s sebagai GAGAL", cycle_id)


def _tulis_dan_finalkan(cfg: Konfigurasi, db: Database, kunci: KunciSiklus, ts: datetime, cycle_id: str, data: DataMentah, h: dict, lalu: dict | None, mulai: float) -> dict:
    reg: HasilRegresi = h["reg"]
    kelas: HasilKlasifikasi = h["kelas"]
    panel: Panel = h["panel"]
    meta = {"cycle_id": cycle_id, "cycle_timestamp": ts}
    dokumen = {
        K_MAKRO: _dokumen_makro(data, meta),
        K_FORECAST: _dokumen_forecast(reg, panel.cutoff if panel.cutoff is not None else pd.Timestamp(tanggal_wib(ts)), cfg.panjang_riwayat_keluaran, meta),
        K_XAI: [{"_id": f"{cycle_id}:{x['target']}:{x['komoditas_kode'] or '-'}:{x['wilayah_kode']}", **meta, **x} for x in h["xai"]],
        K_INFLASI: [{"_id": f"{cycle_id}:{s['wilayah_kode']}", **meta, **s} for s in kelas.sinyal],
        K_EWS: [{"_id": f"{cycle_id}:{s['sinyal_kunci']}", **meta, **s} for s in h["ews"]],
    }
    t = time.perf_counter()
    for nama in KOLEKSI_HASIL:
        kunci.pastikan()
        tulis_dokumen(db, nama, dokumen[nama])
    h["tahap"]["tulis"] = int((time.perf_counter() - t) * 1000)

    jumlah = {nama: db[nama].count_documents({"cycle_id": cycle_id}) for nama in KOLEKSI_HASIL}
    for nama, isi in dokumen.items():
        if jumlah[nama] != len(isi):
            raise RuntimeError(f"jumlah dokumen {nama} tidak cocok: {jumlah[nama]} != {len(isi)}")

    status = _status_akhir(data, reg, kelas, h["gagal"])
    catatan = list(h["catatan"])
    if data.harga.empty:
        catatan.insert(0, "tidak ada titik harga asli yang dapat dipakai")
    elif lalu is not None and data.jumlah_baru == 0:
        catatan.insert(0, f"tidak ada data baru sejak siklus {lalu['_id']}")
    if reg.dilewati:
        catatan.append(f"{len(reg.dilewati)} seri dilewati karena riwayat pendek atau data terlalu basi")
    if h["gagal"]:
        catatan.append(f"{len(h['gagal'])} kegagalan parsial")
    durasi = int((time.perf_counter() - mulai) * 1000)
    metrik = {
        "klasifikasi": kelas.metrik,
        "regresi": _metrik_regresi(reg, h["backend"].nama_kuantil, h["seleksi_id"], h["seleksi_ts"]),
        "durasi_tahap_ms": h["tahap"], "kegagalan": h["gagal"][:50], "jumlah_data_baru": data.jumlah_baru,
    }
    kunci.pastikan()
    hasil = db[K_SIKLUS].update_one(
        {"_id": cycle_id, "status": "BERJALAN"},
        {"$set": bersihkan({
            "status": status, "pesan": "; ".join(catatan), "duration_ms": durasi, "jumlah": jumlah,
            "source_freshness": data.kesegaran, "target_inflasi": kelas.target_siklus if kelas.sinyal else None,
            "model_metrics": metrik,
        })},
    )
    if hasil.modified_count != 1:
        raise RuntimeError(f"gagal memfinalkan siklus {cycle_id}")
    log.info("siklus %s %s dalam %.1f dtk: %s", cycle_id, status, durasi / 1000, jumlah)
    return {"cycle_id": cycle_id, "status": status, "dilewati": False, "duration_ms": durasi, "jumlah": jumlah, "pesan": "; ".join(catatan), "model_metrics": metrik, "target_inflasi": kelas.target_siklus if kelas.sinyal else None}
