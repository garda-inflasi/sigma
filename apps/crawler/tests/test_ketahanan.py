"""Carried-forward, idempotensi, dan isolasi kegagalan."""

from datetime import UTC, date, datetime, timedelta

import httpx

from garda_crawler.model import ERROR, FRESH, STALE, DefinisiDataset, HasilAmbil, Seri, Titik
from garda_crawler.pelari import jalankan_adapter
from garda_crawler.sumber.dasar import Adapter
from garda_crawler.waktu import tanggal_wib, tengah_malam_utc
from tests.conftest import SEKARANG, buat_klien

DS = DefinisiDataset("uji", "raw_pangan_harian")
SERI_A = Seri("PIHPS", "uji", DS.koleksi, "CBM-KRT:1171", "harga", "Rp/kg", "HARIAN", "CBM-KRT", "1171", "https://s/a")
SERI_B = Seri("PIHPS", "uji", DS.koleksi, "BWM:1171", "harga", "Rp/kg", "HARIAN", "BWM", "1171", "https://s/b")
BESOK = SEKARANG + timedelta(days=1)


class AdapterSkrip(Adapter):
    sumber = "PIHPS"

    def __init__(self, *hasil):
        super().__init__()
        self.hasil = hasil

    def daftar_dataset(self):
        return [DS]

    def seri_diharapkan(self, ds):
        return [SERI_A, SERI_B]

    def ambil_harian(self, ds, klien, tanggal):
        for h in self.hasil:
            if isinstance(h, Exception):
                raise h
            yield h

    def ambil_backfill(self, ds, klien, dari, sampai):
        yield from self.ambil_harian(ds, klien, dari)


def t(seri, hari, nilai):
    return Titik(seri, hari, nilai, seri.url_sumber, {})


def jalan(penyimpan, *hasil, sekarang=SEKARANG, pemicu="MANUAL", **kw):
    def handler(_):
        raise AssertionError("adapter skrip tidak memakai jaringan")

    with buat_klien(handler) as klien:
        return jalankan_adapter(AdapterSkrip(*hasil), klien, penyimpan, pemicu, sekarang, **kw)[0]


def kol(penyimpan):
    return penyimpan.db["raw_pangan_harian"]


def test_id_dan_field_sesuai_kontrak(penyimpan):
    jalan(penyimpan, HasilAmbil(titik=[t(SERI_A, date(2026, 11, 23), 50000)], permintaan=1))
    d = kol(penyimpan).find_one({"kunci_seri": "CBM-KRT:1171"})
    assert d["_id"] == "PIHPS:uji:CBM-KRT:1171:2026-11-23:2026-11-24"
    harus = {
        "sumber", "dataset", "kunci_seri", "indikator", "komoditas_kode", "kabupaten_kode", "satuan", "frekuensi",
        "periode", "nilai", "is_carried_forward", "last_available_date", "crawled_at", "crawl_date", "source_status",
        "run_id", "url_sumber", "status_validasi", "atribut",
    }
    assert harus <= set(d)
    assert d["periode"] == datetime(2026, 11, 23, tzinfo=UTC)
    assert d["crawl_date"] == "2026-11-24"
    assert d["source_status"] == FRESH and d["is_carried_forward"] is False
    assert d["run_id"] in {r["_id"] for r in penyimpan.db["crawl_runs"].find()}


def test_crawl_runs_sesuai_kontrak(penyimpan):
    jalan(penyimpan, HasilAmbil(titik=[t(SERI_A, date(2026, 11, 23), 50000)], permintaan=1))
    r = penyimpan.db["crawl_runs"].find_one()
    assert {
        "_id", "crawler", "sumber", "dataset", "crawl_date", "mulai", "selesai", "status", "jumlah_baru",
        "jumlah_diteruskan", "last_available_date", "pesan", "pemicu",
    } <= set(r)
    assert r["status"] == FRESH and r["jumlah_baru"] == 1


def test_sumber_kosong_menulis_carried_forward_dengan_flag(penyimpan):
    jalan(penyimpan, HasilAmbil(titik=[t(SERI_A, date(2026, 11, 23), 50000), t(SERI_B, date(2026, 11, 23), 40000)], permintaan=1))
    r = jalan(penyimpan, HasilAmbil(permintaan=1), sekarang=BESOK)
    assert r.status == STALE
    assert r.jumlah_diteruskan == 2

    cf = list(kol(penyimpan).find({"is_carried_forward": True}))
    assert len(cf) == 2
    for d in cf:
        assert d["source_status"] == STALE
        assert d["periode"] == tengah_malam_utc(tanggal_wib(BESOK))
        assert d["last_available_date"] == datetime(2026, 11, 23, tzinfo=UTC)
        assert d["crawl_date"] == "2026-11-25"
        assert d["crawled_at"] == BESOK
    assert {d["nilai"] for d in cf} == {50000, 40000}


def test_sumber_error_dengan_riwayat_diteruskan_tanpa_riwayat_null(penyimpan):
    jalan(penyimpan, HasilAmbil(titik=[t(SERI_A, date(2026, 11, 23), 50000)], permintaan=1))
    r = jalan(penyimpan, RuntimeError("putus"), sekarang=BESOK)
    assert r.status == ERROR
    assert "DILEWATI" not in r.pesan
    a = kol(penyimpan).find_one({"kunci_seri": "CBM-KRT:1171", "crawl_date": "2026-11-25"})
    assert a["is_carried_forward"] is True and a["nilai"] == 50000 and a["source_status"] == STALE
    b = kol(penyimpan).find_one({"kunci_seri": "BWM:1171", "crawl_date": "2026-11-25"})
    assert b["nilai"] is None and b["source_status"] == ERROR and b["last_available_date"] is None
    assert b["is_carried_forward"] is False


def test_dilewati_tetap_error_walau_hari_ini_sudah_ada_data(penyimpan):
    jalan(penyimpan, HasilAmbil(titik=[t(SERI_A, date(2026, 11, 23), 50000)], permintaan=1))
    r = jalan(penyimpan, HasilAmbil(dilewati="DILEWATI: kunci hilang; tindakan: isi kunci"))
    assert r.status == ERROR
    assert r.pesan.startswith("DILEWATI: ")
    assert penyimpan.db["crawl_runs"].find_one()["status"] == ERROR
    # Data yang sudah masuk hari ini tidak ikut terhapus.
    assert kol(penyimpan).count_documents({"source_status": FRESH}) == 1


def test_tidak_pernah_mengarang_nilai_saat_gagal_total(penyimpan):
    r = jalan(penyimpan, RuntimeError("putus"))
    assert r.status == ERROR
    assert all(d["nilai"] is None for d in kol(penyimpan).find())
    assert kol(penyimpan).count_documents({}) == 2


def test_idempoten_jalan_ulang_hari_sama(penyimpan):
    h = lambda: HasilAmbil(titik=[t(SERI_A, date(2026, 11, 23), 50000)], permintaan=1)
    jalan(penyimpan, h())
    jumlah = kol(penyimpan).count_documents({})
    r = jalan(penyimpan, h())
    jalan(penyimpan, h())
    assert kol(penyimpan).count_documents({}) == jumlah
    assert penyimpan.db["crawl_runs"].count_documents({}) == 1
    assert r.status == FRESH  # Rilis hari ini tetap diakui pada jalan ulang.


def test_idempoten_carried_forward(penyimpan):
    jalan(penyimpan, HasilAmbil(titik=[t(SERI_A, date(2026, 11, 23), 50000)], permintaan=1))
    jalan(penyimpan, HasilAmbil(permintaan=1), sekarang=BESOK)
    n = kol(penyimpan).count_documents({})
    jalan(penyimpan, HasilAmbil(permintaan=1), sekarang=BESOK)
    assert kol(penyimpan).count_documents({}) == n


def test_nilai_sama_besok_tidak_menambah_vintage(penyimpan):
    jalan(penyimpan, HasilAmbil(titik=[t(SERI_A, date(2026, 11, 23), 50000)], permintaan=1))
    r = jalan(penyimpan, HasilAmbil(titik=[t(SERI_A, date(2026, 11, 23), 50000)], permintaan=1), sekarang=BESOK)
    assert kol(penyimpan).count_documents({"is_carried_forward": False}) == 1
    assert r.status == STALE  # Titik lama yang sama bukan rilis baru.


def test_revisi_nilai_menjadi_vintage_baru(penyimpan):
    jalan(penyimpan, HasilAmbil(titik=[t(SERI_A, date(2026, 11, 23), 50000)], permintaan=1))
    r = jalan(penyimpan, HasilAmbil(titik=[t(SERI_A, date(2026, 11, 23), 51000)], permintaan=1), sekarang=BESOK)
    assert r.status == FRESH
    nilai = {d["crawl_date"]: d["nilai"] for d in kol(penyimpan).find({"is_carried_forward": False})}
    assert nilai == {"2026-11-24": 50000, "2026-11-25": 51000}


def test_rilis_terlambat_menggantikan_carried_forward_hari_itu(penyimpan):
    jalan(penyimpan, HasilAmbil(titik=[t(SERI_A, date(2026, 11, 23), 50000)], permintaan=1))
    jalan(penyimpan, HasilAmbil(permintaan=1), sekarang=BESOK)
    assert kol(penyimpan).count_documents({"is_carried_forward": True}) == 1
    r = jalan(penyimpan, HasilAmbil(titik=[t(SERI_A, date(2026, 11, 24), 52000)], permintaan=1), sekarang=BESOK)
    assert r.status == FRESH
    assert kol(penyimpan).count_documents({"is_carried_forward": True}) == 0


def test_error_pagi_dibersihkan_saat_jalan_ulang_sukses(penyimpan):
    jalan(penyimpan, RuntimeError("putus"))
    assert kol(penyimpan).count_documents({"source_status": ERROR}) == 2
    jalan(penyimpan, HasilAmbil(titik=[t(SERI_A, date(2026, 11, 23), 50000)], permintaan=1))
    assert kol(penyimpan).count_documents({"source_status": ERROR}) == 0


def test_backfill_tidak_menulis_carried_forward(penyimpan):
    r = jalan(
        penyimpan,
        HasilAmbil(titik=[t(SERI_A, date(2026, 1, 5), 45000)], permintaan=1),
        pemicu="BACKFILL",
        dari=date(2026, 1, 1),
        sampai=date(2026, 1, 31),
    )
    assert r.status == FRESH and r.jumlah_baru == 1
    assert kol(penyimpan).count_documents({"is_carried_forward": True}) == 0
    d = kol(penyimpan).find_one()
    assert d["atribut"]["backfill"] is True


def test_backfill_hari_ini_bukan_rilis_baru_untuk_crawl_harian(penyimpan):
    jalan(
        penyimpan,
        HasilAmbil(titik=[t(SERI_A, date(2026, 1, 5), 45000)], permintaan=1),
        pemicu="BACKFILL",
        dari=date(2026, 1, 1),
        sampai=date(2026, 1, 31),
    )
    r = jalan(penyimpan, HasilAmbil(permintaan=1))
    assert r.status == STALE
    d = kol(penyimpan).find_one({"is_carried_forward": True})
    assert d["last_available_date"] == datetime(2026, 1, 5, tzinfo=UTC)


def test_satu_adapter_gagal_lainnya_tetap_jalan(penyimpan):
    class Rusak(AdapterSkrip):
        sumber = "BMKG"

        def daftar_dataset(self):
            return [DefinisiDataset("rusak", "raw_iklim_bmkg")]

        def seri_diharapkan(self, ds):
            return []

    def handler(_):
        return httpx.Response(500)

    with buat_klien(handler) as klien:
        a = jalankan_adapter(Rusak(RuntimeError("putus")), klien, penyimpan, "JADWAL", SEKARANG)
        b = jalankan_adapter(
            AdapterSkrip(HasilAmbil(titik=[t(SERI_A, date(2026, 11, 23), 50000)], permintaan=1)),
            klien,
            penyimpan,
            "JADWAL",
            SEKARANG,
        )
    assert a[0].status == ERROR
    assert b[0].status == FRESH
    assert penyimpan.db["crawl_runs"].count_documents({}) == 2
