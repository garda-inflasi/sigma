"""Server HTTP kontrol internal untuk crawler: memantau jadwal, progres live, dan memicu retry manual."""

from __future__ import annotations

import json
import logging
import os
import threading
import time
import uuid
from collections.abc import Callable, Sequence
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

from pymongo.database import Database

from garda_crawler.jadwal import JAM_HARIAN, pemilik_proses
from garda_crawler.kunci import KunciMongo
from garda_crawler.sumber import LAPANGAN, SUMBER_CRAWL

log = logging.getLogger(__name__)

PORT_KONTROL_BAWAAN = 8091
MAKS_LOG = 50


def _iso_utc(dt: datetime | None = None) -> str:
    return (dt or datetime.now(timezone.utc)).astimezone(timezone.utc).isoformat()


class KeadaanKontrolCrawler:
    """Menyimpan state progres crawler terkini serta memastikan hanya 1 job manual/retry berjalan."""

    def __init__(
        self,
        db: Database | None,
        pemilik: str,
        pelaksana: Callable[[list[str]], list[dict[str, Any]]],
    ) -> None:
        self.db = db
        self.pemilik = pemilik
        self.pelaksana = pelaksana
        self._lock = threading.Lock()
        self._utas: threading.Thread | None = None
        self._state: dict[str, Any] = {
            "berjalan": False,
            "job_id": None,
            "sumber": [],
            "gagal_saja": False,
            "tahap": "SIAGA",
            "progress_pct": 0,
            "dataset_aktif": None,
            "selesai_dataset": 0,
            "total_dataset": 0,
            "mulai": None,
            "selesai": None,
            "durasi_ms": None,
            "logs": [],
            "hasil": [],
            "error": None,
            "jadwal": {
                "harian": f"Setiap hari {JAM_HARIAN:02d}:00 WIB",
                "lapangan": "Tiap 1 jam (xx:00 WIB)",
            },
        }

    def catat(self, tahap: str, pct: int, pesan: str, *, dataset: str | None = None, selesai_ds: int | None = None, total_ds: int | None = None) -> None:
        with self._lock:
            self._state["tahap"] = tahap
            self._state["progress_pct"] = max(0, min(100, int(pct)))
            if dataset is not None:
                self._state["dataset_aktif"] = dataset
            if selesai_ds is not None:
                self._state["selesai_dataset"] = selesai_ds
            if total_ds is not None:
                self._state["total_dataset"] = total_ds
            logs: list[str] = self._state["logs"]
            logs.append(f"[{datetime.now(timezone.utc).strftime('%H:%M:%S')} UTC] {pesan}")
            if len(logs) > MAKS_LOG:
                del logs[: len(logs) - MAKS_LOG]

    def snapshot(self) -> dict[str, Any]:
        with self._lock:
            out = dict(self._state)
            out["logs"] = list(self._state["logs"])
            out["sumber"] = list(self._state["sumber"])
            out["hasil"] = list(self._state["hasil"])
            out["jadwal"] = dict(self._state["jadwal"])
        if self.db is not None:
            try:
                jadwal_docs = list(self.db["crawl_jadwal"].find({}, {"_id": 1, "terakhir": 1, "selesai_pada": 1}))
                out["slot_terakhir"] = {
                    d["_id"]: {
                        "terakhir": d.get("terakhir"),
                        "selesai_pada": d["selesai_pada"].isoformat() if d.get("selesai_pada") else None,
                    }
                    for d in jadwal_docs
                }
            except Exception:  # noqa: BLE001
                out["slot_terakhir"] = {}
        return out

    def sumber_gagal_terakhir(self) -> list[str]:
        if self.db is None:
            return []
        try:
            pipa = [
                {"$match": {"pemicu": {"$ne": "BACKFILL"}}},
                {"$sort": {"selesai": -1}},
                {"$group": {"_id": {"sumber": "$sumber", "dataset": "$dataset"}, "run": {"$first": "$$ROOT"}}},
            ]
            gagal: set[str] = set()
            for row in self.db["crawl_runs"].aggregate(pipa):
                r = row["run"]
                if r.get("status") in ("ERROR", "STALE_CARRIED_FORWARD") and not str(r.get("pesan", "")).startswith("DILEWATI:"):
                    gagal.add(str(r["sumber"]))
            return sorted(gagal)
        except Exception:  # noqa: BLE001
            return []

    def mulai(self, sumber: Sequence[str] | None = None, gagal_saja: bool = False, job_id: str | None = None) -> tuple[bool, dict[str, Any]]:
        target = [str(s).strip().upper() for s in (sumber or []) if str(s).strip()]
        if gagal_saja and not target:
            target = self.sumber_gagal_terakhir() or list(SUMBER_CRAWL)
        if not target:
            target = [*SUMBER_CRAWL, LAPANGAN]

        with self._lock:
            if self._state["berjalan"]:
                return False, dict(self._state)
            jid = job_id or f"CRW-{uuid.uuid4().hex[:10].upper()}"
            self._state.update({
                "berjalan": True,
                "job_id": jid,
                "sumber": target,
                "gagal_saja": gagal_saja,
                "tahap": "ANTRIAN",
                "progress_pct": 5,
                "dataset_aktif": None,
                "selesai_dataset": 0,
                "total_dataset": len(target),
                "mulai": _iso_utc(),
                "selesai": None,
                "durasi_ms": None,
                "logs": [f"[{datetime.now(timezone.utc).strftime('%H:%M:%S')} UTC] Memulai crawl manual: {', '.join(target)}"],
                "hasil": [],
                "error": None,
            })

        t = threading.Thread(target=self._jalankan_latar, args=(jid, target), name=f"crawler-manual-{jid}", daemon=True)
        self._utas = t
        t.start()
        return True, self.snapshot()

    def _jalankan_latar(self, job_id: str, target: list[str]) -> None:
        t0 = time.perf_counter()
        kunci = KunciMongo(self.db, "manual:crawler", self.pemilik) if self.db is not None else None
        terambil = True
        try:
            if kunci is not None:
                terambil = kunci.ambil()
                if not terambil:
                    raise RuntimeError("Kunci crawler dipegang instance lain; coba beberapa saat lagi.")
            self.catat("CRAWL_SUMBER", 20, f"Menjalankan adapter ({len(target)} sumber)...")
            hasil = self.pelaksana(target)
            ringkas = list(hasil)
            with self._lock:
                self._state["hasil"] = ringkas
                self._state["selesai_dataset"] = len(target)
            ada_galat = any(r.get("status") == "ERROR" for r in ringkas)
            self.catat(
                "SELESAI",
                100,
                f"Crawl selesai ({len(ringkas)} dataset" + (", sebagian error" if ada_galat else ", bersih") + ").",
                dataset=None,
                selesai_ds=len(target),
            )
            with self._lock:
                self._state["berjalan"] = False
                self._state["selesai"] = _iso_utc()
                self._state["durasi_ms"] = int((time.perf_counter() - t0) * 1000)
        except Exception as exc:  # noqa: BLE001
            self.catat("GAGAL", 100, f"Crawl gagal: {type(exc).__name__}: {exc}")
            with self._lock:
                self._state["berjalan"] = False
                self._state["selesai"] = _iso_utc()
                self._state["durasi_ms"] = int((time.perf_counter() - t0) * 1000)
                self._state["error"] = f"{type(exc).__name__}: {exc}"
        finally:
            if kunci is not None and terambil:
                try:
                    kunci.lepas()
                except Exception:  # noqa: BLE001
                    pass


def buat_server_kontrol(keadaan: KeadaanKontrolCrawler, host: str = "0.0.0.0", port: int = PORT_KONTROL_BAWAAN) -> ThreadingHTTPServer:
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, format: str, *args: object) -> None:  # noqa: A002
            log.debug("kontrol http: " + format, *args)

        def _kirim(self, code: int, payload: dict[str, Any]) -> None:
            raw = json.dumps(payload, ensure_ascii=False, default=str).encode("utf-8")
            self.send_response(code)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Content-Length", str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)

        def do_GET(self) -> None:  # noqa: N802
            path = self.path.split("?", 1)[0]
            if path == "/health":
                self._kirim(200, {"ok": True})
                return
            if path == "/status":
                self._kirim(200, keadaan.snapshot())
                return
            self._kirim(404, {"error": "NOT_FOUND"})

        def do_POST(self) -> None:  # noqa: N802
            path = self.path.split("?", 1)[0]
            if path != "/run":
                self._kirim(404, {"error": "NOT_FOUND"})
                return
            panjang = int(self.headers.get("Content-Length", "0") or 0)
            body: dict[str, Any] = {}
            if panjang > 0:
                try:
                    body = json.loads(self.rfile.read(panjang).decode("utf-8"))
                except json.JSONDecodeError:
                    self._kirim(400, {"error": "INVALID_JSON"})
                    return
            sumber_raw = body.get("sumber")
            sumber = [sumber_raw] if isinstance(sumber_raw, str) else list(sumber_raw or [])
            ok, snap = keadaan.mulai(
                sumber=sumber,
                gagal_saja=bool(body.get("gagal_saja", False)),
                job_id=body.get("job_id"),
            )
            if not ok:
                self._kirim(409, {"error": "SEDANG_BERJALAN", "state": snap})
                return
            self._kirim(202, snap)

    return ThreadingHTTPServer((host, port), Handler)


def jalankan_server_kontrol_latar(
    db: Database | None,
    pelaksana: Callable[[list[str]], list[dict[str, Any]]],
    port: int | None = None,
) -> tuple[ThreadingHTTPServer, KeadaanKontrolCrawler]:
    p = port if port is not None else int(os.environ.get("CRAWLER_CONTROL_PORT", str(PORT_KONTROL_BAWAAN)))
    keadaan = KeadaanKontrolCrawler(db=db, pemilik=pemilik_proses(), pelaksana=pelaksana)
    srv = buat_server_kontrol(keadaan, port=p)
    t = threading.Thread(target=srv.serve_forever, name="crawler-kontrol-http", daemon=True)
    t.start()
    log.info("crawler kontrol HTTP aktif di :%d", p)
    return srv, keadaan
