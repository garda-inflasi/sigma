"""Fixture bersama.

Asal berkas tests/fixtures:
- pihps_*.json, bmkg_banda_aceh.json: respons NYATA, diambil 2026-11-24.
- bps_inflasi_yoy_2025.json: salinan fixture konektor Go, ditulis dari dokumentasi (belum ada kunci BPS).
- open_meteo_arsip_format_dokumentasi.json: bentuk dari dokumentasi, angka contoh. robots.txt host arsip
  melarang pengambilan, sehingga respons nyata memang tidak diambil.
"""

import json
from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import httpx
import mongomock
import pytest

from garda_crawler.http_sopan import KlienSopan
from garda_crawler.penyimpanan import Penyimpan

FIXTURE = Path(__file__).parent / "fixtures"
SEKARANG = datetime(2026, 11, 24, 3, 0, tzinfo=UTC)  # 10.00 WIB

# mongomock 4.3 belum menerima argumen `sort` yang dikirim ReplaceOne pada pymongo >= 4.11.
_add_replace_asli = mongomock.collection.BulkOperationBuilder.add_replace


def _add_replace(self, selector, doc, upsert, collation=None, hint=None, sort=None, **_):
    return _add_replace_asli(self, selector, doc, upsert, collation=collation, hint=hint)


mongomock.collection.BulkOperationBuilder.add_replace = _add_replace


def muat(nama: str) -> Any:
    return json.loads((FIXTURE / nama).read_text(encoding="utf-8"))


@pytest.fixture
def db():
    return mongomock.MongoClient(tz_aware=True)["dataunseen"]


@pytest.fixture
def penyimpan(db) -> Penyimpan:
    p = Penyimpan(db)
    p.siapkan_indeks()
    return p


def buat_klien(handler: Callable[[httpx.Request], httpx.Response], **kw: Any) -> KlienSopan:
    kw.setdefault("jeda_ms", 0)
    kw.setdefault("percobaan", 1)
    return KlienSopan("garda-crawler/uji", transport=httpx.MockTransport(handler), tidur=lambda _: None, **kw)


def tanpa_robots(handler: Callable[[httpx.Request], httpx.Response]) -> Callable[[httpx.Request], httpx.Response]:
    def bungkus(req: httpx.Request) -> httpx.Response:
        if req.url.path == "/robots.txt":
            return httpx.Response(404)
        return handler(req)

    return bungkus
