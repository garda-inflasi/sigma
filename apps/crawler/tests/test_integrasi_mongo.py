"""Uji terhadap MongoDB sungguhan. Aktif bila GARDA_UJI_MONGO_URI diisi (kontainer uji, bukan stack dev)."""

import os
import uuid
from datetime import date, timedelta

import pytest
from pymongo import MongoClient

from garda_crawler.jadwal import jalankan_bila_jatuh_tempo
from garda_crawler.kunci import KunciMongo
from garda_crawler.model import FRESH, STALE, HasilAmbil, KOLEKSI_RAW
from garda_crawler.penyimpanan import Penyimpan
from garda_crawler.sumber.lapangan import sinkronkan
from tests.conftest import SEKARANG
from tests.test_ketahanan import SERI_A, SERI_B, jalan, t
from tests.test_lapangan import PembacaPalsu, entri

URI = os.environ.get("GARDA_UJI_MONGO_URI")
pytestmark = [pytest.mark.integrasi, pytest.mark.skipif(not URI, reason="GARDA_UJI_MONGO_URI kosong")]


@pytest.fixture
def penyimpan_nyata():
    klien = MongoClient(URI, tz_aware=True, serverSelectionTimeoutMS=5000)
    nama = f"uji_{uuid.uuid4().hex[:10]}"
    p = Penyimpan(klien[nama])
    p.siapkan_indeks()
    yield p
    klien.drop_database(nama)
    klien.close()


def test_indeks_kontrak_terpasang(penyimpan_nyata):
    for nama in KOLEKSI_RAW:
        kunci = [tuple(k for k, _ in i["key"].items()) for i in penyimpan_nyata.db[nama].list_indexes()]
        assert ("kunci_seri", "periode") in kunci
        assert ("sumber", "dataset", "crawl_date") in kunci
        assert ("komoditas_kode", "kabupaten_kode", "periode") in kunci
        assert ("source_status", "crawl_date") in kunci


def test_siklus_carried_forward_dan_idempotensi(penyimpan_nyata):
    p = penyimpan_nyata
    kol = p.db["raw_pangan_harian"]
    awal = HasilAmbil(titik=[t(SERI_A, date(2026, 11, 23), 50000), t(SERI_B, date(2026, 11, 23), 40000)], permintaan=1)
    assert jalan(p, awal).status == FRESH
    besok = SEKARANG + timedelta(days=1)
    for _ in range(2):
        assert jalan(p, HasilAmbil(permintaan=1), sekarang=besok).status == STALE
    assert kol.count_documents({"is_carried_forward": True}) == 2
    assert kol.count_documents({}) == 4
    cf = kol.find_one({"is_carried_forward": True, "kunci_seri": SERI_A.kunci_seri})
    assert cf["last_available_date"].date() == date(2026, 11, 23)


def test_sinkron_lapangan_status(penyimpan_nyata):
    p = penyimpan_nyata
    pembaca = PembacaPalsu([entri(1, "DIAJUKAN"), entri(2, "DIVALIDASI")])
    sinkronkan(pembaca, p, SEKARANG, "JADWAL")
    pembaca.data[1] = entri(1, "DITOLAK", versi=2)
    pembaca.data[2] = entri(2, "DIPUBLIKASI", versi=2)
    sinkronkan(pembaca, p, SEKARANG, "JADWAL")
    dok = list(p.db["raw_pangan_harian"].find({"is_carried_forward": False}))
    assert [(d["atribut"]["entri_id"], d["status_validasi"]) for d in dok] == [(2, "DIPUBLIKASI")]


def test_kunci_dua_pemilik_di_mongo_nyata(penyimpan_nyata):
    db = penyimpan_nyata.db
    a, b = KunciMongo(db, "uji", "a"), KunciMongo(db, "uji", "b")
    assert a.ambil()
    assert not b.ambil()
    a.lepas()
    assert b.ambil()


def test_jadwal_tidak_ganda_di_mongo_nyata(penyimpan_nyata):
    db = penyimpan_nyata.db
    jalan_tugas = []
    assert jalankan_bila_jatuh_tempo(db, "harian", "2026-11-24", lambda: jalan_tugas.append("a"), "a")
    assert not jalankan_bila_jatuh_tempo(db, "harian", "2026-11-24", lambda: jalan_tugas.append("b"), "b")
    assert jalan_tugas == ["a"]
