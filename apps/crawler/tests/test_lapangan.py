from datetime import UTC, datetime, timedelta

import psycopg

from garda_crawler.model import ERROR, FRESH, STALE
from garda_crawler.sumber import lapangan as lp
from garda_crawler.sumber.lapangan import sinkronkan
from tests.conftest import SEKARANG

DICATAT = datetime(2026, 11, 23, 2, 0, tzinfo=UTC)
PRIBADI = {"petugas_id", "divalidasi_oleh", "dipublikasi_oleh", "ditolak_oleh", "lat", "lng", "foto_url", "catatan", "kode_pedagang", "idempotency_key"}


def entri(i, status="DIAJUKAN", versi=1, harga=50000, dicatat=DICATAT, **kw):
    return {
        "id": i,
        "komoditas_kode": "CBM-KRT",
        "pasar_id": "PSR-1171-01",
        "kabupaten_kode": "1171",
        "tipe": "KONSUMEN",
        "harga": harga,
        "satuan": "Kg",
        "kualitas": "MEDIUM",
        "status": status,
        "deviasi_pct": 1.5,
        "skor_anomali": 0.1,
        "dicatat_pada": dicatat,
        "satuan_master": "kg",
        "harga_acuan": 52000,
        "versi": versi,
        **kw,
    }


class PembacaPalsu:
    """Meniru SQL: hanya entri berstatus sah yang terlihat."""

    def __init__(self, baris=(), demo=0, galat=None):
        self.data = {b["id"]: b for b in baris}
        self.demo = demo
        self.galat = galat
        self.diminta = []

    def __enter__(self):
        if self.galat:
            raise self.galat
        return self

    def __exit__(self, *_):
        return None

    def _sah(self):
        return [b for b in self.data.values() if b["status"] in lp.BOBOT_STATUS]

    def indeks(self):
        return [{"id": b["id"], "versi": b["versi"], "dicatat_pada": b["dicatat_pada"]} for b in self._sah()]

    def baris(self, ids):
        self.diminta.extend(ids)
        return [b for b in self._sah() if b["id"] in ids]

    def jumlah_demo(self):
        return self.demo


def kol(penyimpan):
    return penyimpan.db["raw_pangan_harian"]


def asli(penyimpan):
    return list(kol(penyimpan).find({"sumber": "LAPANGAN", "is_carried_forward": False}))


def test_dokumen_sesuai_kontrak_tanpa_data_pribadi(penyimpan):
    baris = entri(1, petugas_id="uuid-petugas", lat=5.5, lng=95.3, catatan="nama pedagang", kode_pedagang="PDG-1")
    h = sinkronkan(PembacaPalsu([baris]), penyimpan, SEKARANG, "JADWAL")
    assert h.status == FRESH
    d = asli(penyimpan)[0]
    assert d["sumber"] == "LAPANGAN"
    assert d["status_validasi"] == "DIAJUKAN"
    assert d["komoditas_kode"] == "CBM-KRT" and d["kabupaten_kode"] == "1171"
    assert d["satuan"] == "Rp/kg" and d["nilai"] == 50000
    assert d["periode"] == datetime(2026, 11, 23, tzinfo=UTC)
    assert d["kunci_seri"] == "CBM-KRT:1171:KONSUMEN:E1"
    assert d["_id"] == "LAPANGAN:entri_harga:CBM-KRT:1171:KONSUMEN:E1:2026-11-23:2026-11-24"
    assert not PRIBADI & (set(d) | set(d["atribut"]))
    assert "uuid-petugas" not in str(d) and "nama pedagang" not in str(d)


def test_sql_mengecualikan_demo_ditolak_draf_dan_kolom_pribadi():
    for sql in (lp.SQL_INDEKS, lp.SQL_BARIS):
        assert "'DIAJUKAN', 'DIVALIDASI', 'DIPUBLIKASI'" in sql
        assert "DITOLAK" not in sql and "DRAF" not in sql
        assert "!~ '^HRG-D[0-9]'" in sql
        for kolom in PRIBADI:
            assert kolom not in sql


def test_periode_memakai_tanggal_wib(penyimpan):
    # 18.00 UTC 22 Nov = 01.00 WIB 23 Nov.
    sinkronkan(PembacaPalsu([entri(1, dicatat=datetime(2026, 11, 22, 18, 0, tzinfo=UTC))]), penyimpan, SEKARANG, "JADWAL")
    assert asli(penyimpan)[0]["periode"] == datetime(2026, 11, 23, tzinfo=UTC)


def test_idempoten_dan_inkremental(penyimpan):
    p = PembacaPalsu([entri(1), entri(2)])
    sinkronkan(p, penyimpan, SEKARANG, "JADWAL")
    assert sorted(p.diminta) == [1, 2]
    n = kol(penyimpan).count_documents({})
    p.diminta.clear()
    h = sinkronkan(p, penyimpan, SEKARANG, "JADWAL")
    assert p.diminta == []  # Tidak ada yang berubah, tidak ada baris yang dibaca ulang.
    assert h.ditulis == 0
    assert kol(penyimpan).count_documents({}) == n
    assert penyimpan.db["crawl_runs"].count_documents({"sumber": "LAPANGAN"}) == 1


def test_perubahan_status_tercermin_di_tempat(penyimpan):
    p = PembacaPalsu([entri(1, "DIAJUKAN", versi=1)])
    sinkronkan(p, penyimpan, SEKARANG, "JADWAL")
    _id = asli(penyimpan)[0]["_id"]

    for status, versi in (("DIVALIDASI", 2), ("DIPUBLIKASI", 3)):
        p.data[1] = entri(1, status, versi=versi)
        sinkronkan(p, penyimpan, SEKARANG + timedelta(days=1), "JADWAL")
        dok = asli(penyimpan)
        assert len(dok) == 1
        assert dok[0]["_id"] == _id
        assert dok[0]["status_validasi"] == status
        assert dok[0]["atribut"]["versi_entri"] == versi


def test_entri_ditolak_dikeluarkan_beserta_turunannya(penyimpan):
    p = PembacaPalsu([entri(1, "DIVALIDASI")])
    sinkronkan(p, penyimpan, SEKARANG, "JADWAL")
    assert kol(penyimpan).count_documents({"is_carried_forward": True}) == 1

    p.data[1] = entri(1, "DITOLAK", versi=2)
    h = sinkronkan(p, penyimpan, SEKARANG + timedelta(hours=1), "JADWAL")
    assert h.dihapus == 1
    assert kol(penyimpan).count_documents({"sumber": "LAPANGAN"}) == 0


def test_entri_dikembalikan_ke_draf_dikeluarkan(penyimpan):
    p = PembacaPalsu([entri(1, "DIAJUKAN")])
    sinkronkan(p, penyimpan, SEKARANG, "JADWAL")
    p.data[1] = entri(1, "DRAF", versi=2)
    sinkronkan(p, penyimpan, SEKARANG, "JADWAL")
    assert asli(penyimpan) == []


def test_entri_terhapus_dikeluarkan(penyimpan):
    p = PembacaPalsu([entri(1), entri(2)])
    sinkronkan(p, penyimpan, SEKARANG, "JADWAL")
    del p.data[2]
    sinkronkan(p, penyimpan, SEKARANG, "JADWAL")
    assert [d["atribut"]["entri_id"] for d in asli(penyimpan)] == [1]


def test_koreksi_harga_diperbarui(penyimpan):
    p = PembacaPalsu([entri(1, harga=50000)])
    sinkronkan(p, penyimpan, SEKARANG, "JADWAL")
    p.data[1] = entri(1, harga=51000, versi=2)
    sinkronkan(p, penyimpan, SEKARANG, "JADWAL")
    assert asli(penyimpan)[0]["nilai"] == 51000


def test_geser_tanggal_tanpa_versi_baru_tetap_tercermin(penyimpan):
    p = PembacaPalsu([entri(1)])
    sinkronkan(p, penyimpan, SEKARANG, "JADWAL")
    p.data[1] = entri(1, dicatat=DICATAT - timedelta(days=2))  # Versi sama: pemicu mengabaikan kolom waktu.
    sinkronkan(p, penyimpan, SEKARANG, "JADWAL")
    dok = asli(penyimpan)
    assert len(dok) == 1 and dok[0]["periode"] == datetime(2026, 11, 21, tzinfo=UTC)


def test_harga_tidak_sah_ditolak_dan_dilaporkan(penyimpan):
    baris = [entri(1, harga=500), entri(2, satuan="liter"), entri(3)]
    h = sinkronkan(PembacaPalsu(baris), penyimpan, SEKARANG, "JADWAL")
    assert h.ditolak == 2
    assert "satuan_keliru=2" in h.pesan
    assert [d["atribut"]["entri_id"] for d in asli(penyimpan)] == [3]


def test_koreksi_menjadi_tidak_sah_mengeluarkan_dokumen(penyimpan):
    p = PembacaPalsu([entri(1)])
    sinkronkan(p, penyimpan, SEKARANG, "JADWAL")
    p.data[1] = entri(1, harga=500, versi=2)
    sinkronkan(p, penyimpan, SEKARANG, "JADWAL")
    assert asli(penyimpan) == []


def test_carried_forward_per_deret_memilih_entri_terbaru(penyimpan):
    baris = [
        entri(1, "DIPUBLIKASI", harga=50000, dicatat=DICATAT - timedelta(days=1)),
        entri(2, "DIAJUKAN", harga=51000),
        entri(3, "DIPUBLIKASI", harga=52000),
    ]
    sinkronkan(PembacaPalsu(baris), penyimpan, SEKARANG, "JADWAL")
    cf = list(kol(penyimpan).find({"is_carried_forward": True}))
    assert len(cf) == 1
    d = cf[0]
    assert d["kunci_seri"] == "CBM-KRT:1171:KONSUMEN"
    assert d["nilai"] == 52000 and d["status_validasi"] == "DIPUBLIKASI"
    assert d["source_status"] == STALE
    assert d["last_available_date"] == datetime(2026, 11, 23, tzinfo=UTC)
    assert d["periode"] == datetime(2026, 11, 24, tzinfo=UTC)


def test_entri_hari_ini_menghapus_carried_forward(penyimpan):
    p = PembacaPalsu([entri(1)])
    sinkronkan(p, penyimpan, SEKARANG, "JADWAL")
    assert kol(penyimpan).count_documents({"is_carried_forward": True}) == 1
    p.data[2] = entri(2, dicatat=SEKARANG)
    sinkronkan(p, penyimpan, SEKARANG, "JADWAL")
    assert kol(penyimpan).count_documents({"is_carried_forward": True}) == 0


def test_postgres_gagal_error_tetap_meneruskan_riwayat(penyimpan):
    sinkronkan(PembacaPalsu([entri(1)]), penyimpan, SEKARANG, "JADWAL")
    besok = SEKARANG + timedelta(days=1)
    h = sinkronkan(PembacaPalsu(galat=psycopg.OperationalError("host rahasia")), penyimpan, besok, "JADWAL")
    assert h.status == ERROR
    assert "DILEWATI" not in h.pesan  # Gangguan koneksi, bukan konfigurasi.
    assert "host rahasia" not in h.pesan
    assert kol(penyimpan).count_documents({"is_carried_forward": True, "crawl_date": "2026-11-25"}) == 1
    # Dokumen asli tidak dihapus hanya karena Postgres tidak terjangkau.
    assert len(asli(penyimpan)) == 1


def test_tanpa_konfigurasi_postgres(penyimpan):
    h = sinkronkan(None, penyimpan, SEKARANG, "JADWAL")
    assert h.status == ERROR
    run = penyimpan.db["crawl_runs"].find_one()
    assert run["status"] == ERROR
    assert run["pesan"].startswith("DILEWATI: ")
    assert "tindakan: isi CRAWLER_DATABASE_URL" in run["pesan"]


def test_hanya_seed_demo_tidak_menulis_apa_pun(penyimpan):
    h = sinkronkan(PembacaPalsu(demo=32289), penyimpan, SEKARANG, "JADWAL")
    assert h.status == STALE
    assert "32289 entri seed demo diabaikan" in h.pesan
    assert kol(penyimpan).count_documents({}) == 0
