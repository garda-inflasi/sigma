from datetime import UTC, datetime, timedelta

import pytest

from garda_crawler.cli import bangun_parser
from garda_crawler.jadwal import detak_sehat, jalankan_bila_jatuh_tempo, slot_harian, slot_jam, tulis_detak
from garda_crawler.konfigurasi import GalatKonfigurasi, Konfigurasi
from garda_crawler.kunci import KunciMongo

W = datetime(2026, 11, 24, 3, 0, tzinfo=UTC)


def test_kunci_eksklusif_dan_kedaluwarsa(db):
    a = KunciMongo(db, "uji", "a", sewa_detik=60)
    b = KunciMongo(db, "uji", "b", sewa_detik=60)
    assert a.ambil(W)
    assert not b.ambil(W + timedelta(seconds=30))
    assert b.ambil(W + timedelta(seconds=61))  # Sewa pemilik lama habis.
    assert not a.ambil(W + timedelta(seconds=62))


def test_kunci_lepas_hanya_oleh_pemilik(db):
    a = KunciMongo(db, "uji", "a")
    b = KunciMongo(db, "uji", "b")
    assert a.ambil()
    b.lepas()
    assert not b.ambil()
    a.lepas()
    assert b.ambil()


def test_slot_harian_mulai_0600_wib():
    assert slot_harian(datetime(2026, 11, 23, 22, 59, tzinfo=UTC)) is None  # 05.59 WIB.
    assert slot_harian(datetime(2026, 11, 23, 23, 0, tzinfo=UTC)) == "2026-11-24"  # 06.00 WIB.
    assert slot_jam(datetime(2026, 11, 23, 23, 30, tzinfo=UTC)) == "2026-11-24T06"


def test_jadwal_sekali_per_slot_lintas_instance(db):
    jalan = []
    assert jalankan_bila_jatuh_tempo(db, "harian", "2026-11-24", lambda: jalan.append(1), "a")
    assert not jalankan_bila_jatuh_tempo(db, "harian", "2026-11-24", lambda: jalan.append(2), "b")
    assert jalankan_bila_jatuh_tempo(db, "harian", "2026-11-25", lambda: jalan.append(3), "b")
    assert jalan == [1, 3]


def test_jadwal_dilewati_saat_instance_lain_memegang_kunci(db):
    assert KunciMongo(db, "jadwal:harian", "lain").ambil()
    jalan = []
    assert not jalankan_bila_jatuh_tempo(db, "harian", "2026-11-24", lambda: jalan.append(1), "a")
    assert jalan == []


def test_tugas_gagal_tidak_menandai_slot_dan_melepas_kunci(db):
    def rusak():
        raise RuntimeError("putus")

    with pytest.raises(RuntimeError):
        jalankan_bila_jatuh_tempo(db, "harian", "2026-11-24", rusak, "a")
    assert db["crawl_jadwal"].find_one({"_id": "harian"}) is None
    assert db["crawl_kunci"].find_one({"_id": "jadwal:harian"}) is None
    jalan = []
    assert jalankan_bila_jatuh_tempo(db, "harian", "2026-11-24", lambda: jalan.append(1), "b")
    assert jalan == [1]


def test_detak(tmp_path):
    berkas = tmp_path / "detak"
    assert not detak_sehat(berkas)
    tulis_detak(berkas, W)
    assert detak_sehat(berkas, sekarang=W + timedelta(seconds=100))
    assert not detak_sehat(berkas, sekarang=W + timedelta(seconds=500))


def test_konfigurasi_fallback_dan_rahasia_tidak_di_repr():
    cfg = Konfigurasi.dari_env({"MONGO_URL": "mongodb://cadangan", "BPS_API_KEY": "RAHASIA", "DB_HOST": "h", "DB_NAME": "n", "DB_PASSWORD": "SANDI"})
    assert cfg.mongo_uri == "mongodb://cadangan"
    assert cfg.db_unseen == "dataunseen"
    assert "RAHASIA" not in repr(cfg) and "SANDI" not in repr(cfg)
    assert Konfigurasi.dari_env({"MONGO_URI": "mongodb://utama", "MONGO_URL": "x"}).mongo_uri == "mongodb://utama"


@pytest.mark.parametrize("env", [{}, {"MONGO_URI": "m", "INGEST_JEDA_MS": "abc"}, {"MONGO_URI": "m", "INGEST_JEDA_MS": "-1"}])
def test_konfigurasi_tidak_sah(env):
    with pytest.raises(GalatKonfigurasi):
        Konfigurasi.dari_env(env)


def test_parser_antarmuka():
    p = bangun_parser()
    a = p.parse_args(["run", "--sumber", "PIHPS", "BMKG", "--pemicu", "JADWAL"])
    assert a.sumber == ["PIHPS", "BMKG"] and a.pemicu == "JADWAL"
    b = p.parse_args(["backfill", "--sumber", "PIHPS", "--dari", "2026-01-01", "--sampai", "2026-02-01"])
    assert b.dari.isoformat() == "2026-01-01"
    for perintah in ("sync-lapangan", "status", "worker"):
        assert p.parse_args([perintah]).perintah == perintah
    with pytest.raises(SystemExit):
        p.parse_args(["backfill", "--sumber", "LAPANGAN", "--dari", "2026-01-01", "--sampai", "2026-01-02"])
