from datetime import date

import httpx
import pytest

from garda_crawler.http_sopan import GalatAmbil
from garda_crawler.konfigurasi import Konfigurasi
from garda_crawler.model import ERROR, FRESH
from garda_crawler.pelari import jalankan_adapter, jalankan_dataset
from garda_crawler.referensi import (
    BMKG_WILAYAH,
    BPS_WILAYAH,
    KABUPATEN,
    KOMODITAS,
    PIHPS_ABAIKAN,
    PIHPS_KOMODITAS,
    PIHPS_WILAYAH,
    normalisasi_alias,
)
from garda_crawler.sumber import AdapterBPS, buat_adapter
from garda_crawler.sumber.bmkg import DATASET_PERINGATAN, AdapterBMKG, urai_prakiraan
from garda_crawler.sumber.bps import DATASET_BPS, PESAN_TANPA_KUNCI, cari_var_id, urai_data, url_tanpa_kunci
from garda_crawler.sumber.open_meteo import AdapterOpenMeteo, urai_arsip
from garda_crawler.sumber.pendukung import AdapterBapanas, AdapterSP2KP
from garda_crawler.sumber.pihps import AdapterPIHPS, urai_grid
from tests.conftest import SEKARANG, buat_klien, muat, tanpa_robots

# --- Pemetaan ---


def test_seluruh_23_wilayah_terpetakan():
    assert set(PIHPS_WILAYAH.values()) == set(KABUPATEN)
    assert set(BMKG_WILAYAH.values()) == set(KABUPATEN)
    assert len(KABUPATEN) == 23


def test_regency_nyata_pihps_cocok_dengan_pemetaan():
    nyata = {r["regency_id"]: r["regency_name"].strip() for r in muat("pihps_regency_aceh.json")["data"]}
    nyata.pop(0)  # "Semua Kota"
    assert set(nyata) == set(PIHPS_WILAYAH)
    harapan = {
        "Banda Aceh": "1171", "Meulaboh": "1107", "Aceh Jeumpa / Bireun": "1110", "Simeuleu": "1101",
        "Gayo Luwes": "1113", "Langsa": "1173", "Lhokseumawe": "1174", "Pidie Jaya": "1118",
    }
    for rid, nama in nyata.items():
        if nama in harapan:
            assert PIHPS_WILAYAH[rid] == harapan[nama], nama


def test_semua_kode_pihps_ada_di_master():
    assert set(PIHPS_KOMODITAS.values()) <= set(KOMODITAS)


def test_adm4_bmkg_memakai_kode_kemendagri_bukan_bps():
    assert BMKG_WILAYAH["11.09.02.2001"] == "1101"  # Simeulue.
    assert BMKG_WILAYAH["11.74.03.2001"] == "1173"  # Langsa.
    assert BMKG_WILAYAH["11.73.01.2002"] == "1174"  # Lhokseumawe.


# --- PIHPS ---


def test_pihps_urai_respons_nyata():
    muatan = muat("pihps_grid_banda_aceh.json")
    hasil = urai_grid(muatan, "1171", 1, "https://contoh/grid")
    assert not hasil.ditolak

    per_kunci = {(t.seri.kunci_seri, t.periode): t for t in hasil.titik}
    baris = next(b for b in muatan["data"] if b["name"].strip() == "Beras Kualitas Medium I")
    assert per_kunci[("BRS-MED:1171", date(2026, 11, 16))].nilai == float(baris["16/11/2026"].replace(",", ""))
    # Sel "-" tidak menjadi titik.
    assert ("BRS-MED:1171", date(2026, 11, 17)) not in per_kunci
    # Baris induk dan varian tidak ikut.
    assert {t.seri.komoditas_kode for t in hasil.titik} <= set(PIHPS_KOMODITAS.values())
    # Nama berspasi di ujung ("Cabai Merah Keriting ") tetap terpetakan.
    assert any(t.seri.komoditas_kode == "CBM-KRT" for t in hasil.titik)
    assert all(t.seri.satuan == ("Rp/liter" if t.seri.komoditas_kode == "MYK" else "Rp/kg") for t in hasil.titik)


def test_pihps_semua_nama_nyata_terkenali():
    for b in muat("pihps_grid_banda_aceh.json")["data"]:
        alias = normalisasi_alias(b["name"])
        assert alias in PIHPS_KOMODITAS or alias in PIHPS_ABAIKAN, alias


def test_pihps_menolak_harga_tidak_sah_dan_nama_asing():
    muatan = {
        "data": [
            {"name": "Gula Pasir Lokal", "01/10/2026": "0", "02/10/2026": "999.999.999", "03/10/2026": "abc", "05/10/2026": "18,000"},
            {"name": "Ikan Segar Baru", "01/10/2026": "40,000"},
        ]
    }
    hasil = urai_grid(muatan, "1171", 1, "u")
    assert [t.nilai for t in hasil.titik] == [18000.0]
    assert hasil.ditolak["harga_non_positif"] == 1
    assert hasil.ditolak["harga_di_luar_batas"] == 1
    assert hasil.ditolak["harga_bukan_angka"] == 1
    assert hasil.ditolak["komoditas_tak_terpetakan"] == 1


def test_pihps_respons_rusak_adalah_galat():
    with pytest.raises(GalatAmbil):
        urai_grid({"pesan": "rusak"}, "1171", 1, "u")


def test_pihps_satu_wilayah_gagal_lainnya_tetap_masuk(penyimpan):
    muatan = muat("pihps_grid_banda_aceh.json")

    def handler(req):
        if req.url.params["regency_id"] == "2":
            return httpx.Response(404)
        return httpx.Response(200, json=muatan)

    adapter = AdapterPIHPS({"1171", "1174"})
    with buat_klien(tanpa_robots(handler)) as klien:
        r = jalankan_adapter(adapter, klien, penyimpan, "MANUAL", SEKARANG)[0]
    assert r.status == FRESH
    assert "gagal 1" in r.pesan
    kol = penyimpan.db["raw_pangan_harian"]
    assert kol.count_documents({"kabupaten_kode": "1171", "source_status": FRESH}) > 0
    # Deret wilayah gagal tanpa riwayat: ERROR, nilai null.
    gagal = list(kol.find({"kabupaten_kode": "1174"}))
    assert gagal and all(d["source_status"] == ERROR and d["nilai"] is None for d in gagal)


def test_pihps_backfill_dipotong_30_hari(penyimpan):
    rentang = []

    def handler(req):
        rentang.append((req.url.params["start_date"], req.url.params["end_date"]))
        return httpx.Response(200, json={"data": []})

    adapter = AdapterPIHPS({"1171"})
    with buat_klien(tanpa_robots(handler)) as klien:
        jalankan_adapter(adapter, klien, penyimpan, "BACKFILL", SEKARANG, date(2026, 1, 1), date(2026, 3, 15))
    assert rentang[0] == ("2026-01-01", "2026-01-30")
    assert rentang[-1][1] == "2026-03-15"
    assert len(rentang) == 3


def test_pihps_backfill_bawaan_hanya_kota_berdata_harian_semua_wilayah(penyimpan):
    diminta = []

    def handler(req):
        diminta.append(req.url.params["regency_id"])
        return httpx.Response(200, json={"data": []})

    with buat_klien(tanpa_robots(handler)) as klien:
        jalankan_adapter(AdapterPIHPS(), klien, penyimpan, "BACKFILL", SEKARANG, date(2026, 1, 1), date(2026, 1, 5))
        assert sorted(diminta) == ["1", "2", "3"]
        diminta.clear()
        jalankan_adapter(AdapterPIHPS(), klien, penyimpan, "JADWAL", SEKARANG)
    assert len(diminta) == 23


# --- BMKG ---


def test_bmkg_urai_respons_nyata():
    muatan = muat("bmkg_banda_aceh.json")
    hasil = urai_prakiraan(muatan, "11.71.01.2001", "1171", date(2026, 11, 24), "u")
    # Hari pertama hanya 5 slot dan ditolak; dua hari penuh menghasilkan 3 indikator.
    assert hasil.ditolak["hari_tidak_lengkap"] == 1
    assert len(hasil.titik) == 6
    assert {t.periode for t in hasil.titik} == {date(2026, 11, 25), date(2026, 11, 26)}
    assert {t.seri.kunci_seri for t in hasil.titik} == {"curah_hujan:1171", "suhu:1171", "kelembapan:1171"}
    assert all(t.atribut["jenis_data"] == "PRAKIRAAN" for t in hasil.titik)
    assert {t.atribut["horizon_hari"] for t in hasil.titik} == {1, 2}

    slot = [s for h in muatan["data"][0]["cuaca"] for s in h if s["local_datetime"].startswith("2026-11-25")]
    harapan = round(sum(s["tp"] for s in slot), 1)
    assert next(t.nilai for t in hasil.titik if t.seri.indikator == "curah_hujan_harian" and t.periode == date(2026, 11, 25)) == harapan


def test_bmkg_tanpa_data_adalah_galat():
    with pytest.raises(GalatAmbil):
        urai_prakiraan({"data": []}, "a", "1171", date(2026, 11, 24), "u")


def test_bmkg_peringatan_dilewati_dengan_alasan(penyimpan):
    def handler(_):
        raise AssertionError("tidak boleh ada permintaan")

    adapter = AdapterBMKG({"1171"})
    with buat_klien(handler) as klien:
        r = jalankan_dataset(adapter, DATASET_PERINGATAN, klien, penyimpan, SEKARANG, "MANUAL")
    assert r.status == ERROR
    assert r.pesan.startswith("DILEWATI: ")
    assert "robots" in r.pesan and "tindakan:" in r.pesan


def test_bmkg_backfill_ditolak(penyimpan):
    adapter = AdapterBMKG({"1171"})
    with buat_klien(lambda _: httpx.Response(500)) as klien:
        r = jalankan_adapter(adapter, klien, penyimpan, "BACKFILL", SEKARANG, date(2026, 1, 1), date(2026, 1, 2))
    assert r[0].status == ERROR
    assert r[0].pesan.startswith("DILEWATI: ")
    assert "tindakan: gunakan --sumber OPEN_METEO" in r[0].pesan


# --- Open-Meteo ---


def test_open_meteo_urai_fixture():
    hasil = urai_arsip(muat("open_meteo_arsip_format_dokumentasi.json"), KABUPATEN["1171"], "u")
    assert hasil.ditolak["curah_hujan_tidak_wajar"] == 1  # -3 mm.
    curah = {t.periode: t.nilai for t in hasil.titik if t.seri.kunci_seri == "curah_hujan:1171"}
    assert curah == {date(2026, 10, 1): 12.4, date(2026, 10, 2): 0.0}  # null tidak menjadi titik.
    assert all(t.seri.sumber == "OPEN_METEO" and t.atribut["sumber_sekunder"] for t in hasil.titik)


def test_open_meteo_patuh_robots_nyata(penyimpan):
    panggilan = []

    def handler(req):
        panggilan.append(req.url.path)
        if req.url.path == "/robots.txt":
            return httpx.Response(200, text="User-agent: *\nDisallow: /", headers={"content-type": "text/plain"})
        raise AssertionError("endpoint arsip tidak boleh dipanggil")

    adapter = AdapterOpenMeteo({"1171", "1174"})
    with buat_klien(handler) as klien:
        r = jalankan_adapter(adapter, klien, penyimpan, "JADWAL", SEKARANG)[0]
    assert panggilan == ["/robots.txt"]
    assert r.status == ERROR
    assert r.pesan.startswith("DILEWATI: ")
    assert "robots" in r.pesan and "tindakan:" in r.pesan
    assert "permintaan" not in r.pesan  # Tidak ada permintaan data yang terkirim.
    docs = list(penyimpan.db["raw_iklim_bmkg"].find({"sumber": "OPEN_METEO"}))
    assert len(docs) == 6
    assert all(d["nilai"] is None and d["source_status"] == ERROR for d in docs)


# --- BPS ---

DS_INFLASI = next(d for d in DATASET_BPS if d.nama == "bps_inflasi")
IND_YOY = DATASET_BPS[DS_INFLASI][0]


def test_bps_tanpa_kunci_error_tanpa_permintaan(penyimpan):
    def handler(_):
        raise AssertionError("tanpa kunci tidak boleh ada permintaan")

    adapter = AdapterBPS("")
    with buat_klien(handler) as klien:
        hasil = jalankan_adapter(adapter, klien, penyimpan, "JADWAL", SEKARANG)
    assert len(hasil) == len(DATASET_BPS)
    assert all(r.status == ERROR and PESAN_TANPA_KUNCI in r.pesan for r in hasil)
    runs = list(penyimpan.db["crawl_runs"].find({"sumber": "BPS"}))
    assert len(runs) == len(DATASET_BPS)
    assert all(r["status"] == ERROR and r["pesan"].startswith("DILEWATI: ") for r in runs)
    assert all("tindakan: isi BPS_API_KEY" in r["pesan"] for r in runs)
    assert penyimpan.db["raw_bps_makro"].count_documents({"nilai": None, "source_status": ERROR}) > 0
    assert penyimpan.db["raw_bps_makro"].count_documents({"nilai": {"$ne": None}}) == 0


def test_bps_pemetaan_fixture():
    hasil = urai_data(muat("bps_inflasi_yoy_2025.json"), DS_INFLASI, IND_YOY, "https://webapi.test/data")
    nilai = {(t.seri.kunci_seri, t.periode): t.nilai for t in hasil.titik}
    assert nilai[("inflasi_yoy:11", date(2025, 8, 1))] == 2.3
    assert nilai[("inflasi_yoy:11", date(2025, 9, 1))] == 2.84
    assert nilai[("inflasi_yoy:1171", date(2025, 9, 1))] == 2.5
    assert hasil.ditolak["wilayah_tak_terpetakan"] >= 1


def test_bps_alias_semua_kab_kota():
    for k in KABUPATEN.values():
        assert BPS_WILAYAH[normalisasi_alias(k.nama)] == k.kode
    assert BPS_WILAYAH["aceh"] == "11"
    assert BPS_WILAYAH["kota banda aceh"] == "1171"


def test_bps_data_tidak_tersedia_bukan_galat():
    hasil = urai_data({"status": "OK", "data-availability": "list-not-available"}, DS_INFLASI, IND_YOY, "u")
    assert not hasil.titik


def test_bps_status_galat():
    with pytest.raises(GalatAmbil):
        urai_data({"status": "Error", "message": "invalid key"}, DS_INFLASI, IND_YOY, "u")


def test_bps_kunci_tidak_bocor(penyimpan):
    kunci = "KUNCIBPSRAHASIA999"
    muatan = muat("bps_inflasi_yoy_2025.json")

    def handler(req):
        if "/var/12/" in req.url.path and "/th/125/" in req.url.path:
            return httpx.Response(200, json=muatan)
        return httpx.Response(500)

    variabel = {i.kunci: "12" for i in DATASET_BPS[DS_INFLASI]}
    adapter = AdapterBPS(kunci, variabel)
    with buat_klien(tanpa_robots(handler), rahasia=[kunci]) as klien:
        jalankan_adapter(adapter, klien, penyimpan, "MANUAL", SEKARANG)

    assert penyimpan.db["raw_bps_makro"].count_documents({"source_status": FRESH}) > 0
    for nama in penyimpan.db.list_collection_names():
        for doc in penyimpan.db[nama].find():
            assert kunci not in str(doc), nama
    assert url_tanpa_kunci(f"https://x/th/1/key/{kunci}") == "https://x/th/1"


def test_bps_variabel_ambigu_tidak_ditebak():
    daftar = {
        "status": "OK",
        "data": [
            {"page": 1, "pages": 1},
            [
                {"var_id": 1, "title": "Inflasi Year on Year Kota A"},
                {"var_id": 2, "title": "Inflasi Year on Year Kota B"},
            ],
        ],
    }
    with buat_klien(tanpa_robots(lambda _: httpx.Response(200, json=daftar)), rahasia=["K"]) as klien:
        with pytest.raises(GalatAmbil, match="ambigu"):
            cari_var_id(klien, "K", "1100", IND_YOY, {})


def test_bps_kunci_ditolak_dilaporkan_apa_adanya():
    jawaban = {"status": "Error", "data-availability": "not-available", "message": "Invalid key"}
    with buat_klien(tanpa_robots(lambda _: httpx.Response(200, json=jawaban)), rahasia=["K"]) as klien:
        with pytest.raises(GalatAmbil, match="status 'Error'"):
            cari_var_id(klien, "K", "1100", IND_YOY, {})


# --- Pendukung ---


@pytest.mark.parametrize(("kelas", "kata"), [(AdapterSP2KP, "reCAPTCHA"), (AdapterBapanas, "API key")])
def test_pendukung_dilewati_tanpa_permintaan(penyimpan, kelas, kata):
    def handler(_):
        raise AssertionError("tidak boleh ada permintaan")

    with buat_klien(handler) as klien:
        r = jalankan_adapter(kelas(), klien, penyimpan, "JADWAL", SEKARANG)[0]
    assert r.status == ERROR
    assert r.pesan.startswith("DILEWATI: ") and kata in r.pesan and "tindakan:" in r.pesan
    assert penyimpan.db["raw_pangan_harian"].count_documents({"sumber": kelas.sumber}) == 0


# --- Batas awalan DILEWATI ---


def _robots_larang(req):
    if req.url.path == "/robots.txt":
        return httpx.Response(200, text="User-agent: *\nDisallow: /", headers={"content-type": "text/plain"})
    raise AssertionError("endpoint terlarang tidak boleh dipanggil")


@pytest.mark.parametrize(
    "adapter",
    [AdapterPIHPS({"1171"}), AdapterBMKG({"1171"}), AdapterBPS("KUNCI", {"inflasi_yoy": "12"}, {"11"})],
    ids=["pihps", "bmkg", "bps"],
)
def test_robots_memakai_awalan_dilewati_di_semua_adapter(penyimpan, adapter):
    ds = adapter.daftar_dataset()[0]
    with buat_klien(_robots_larang, rahasia=["KUNCI"]) as klien:
        r = jalankan_dataset(adapter, ds, klien, penyimpan, SEKARANG, "MANUAL")
    assert r.status == ERROR
    assert r.pesan.startswith("DILEWATI: ") and "tindakan:" in r.pesan
    assert "KUNCI" not in r.pesan


@pytest.mark.parametrize(
    "handler",
    [
        tanpa_robots(lambda _: httpx.Response(500)),
        tanpa_robots(lambda _: httpx.Response(200, json={"bukan": "grid"})),
        tanpa_robots(lambda _: httpx.Response(200, text="<html>")),
        lambda _: httpx.Response(503),  # robots.txt tidak terjangkau: gangguan, bukan larangan.
    ],
    ids=["http_500", "bentuk_salah", "bukan_json", "robots_503"],
)
def test_kegagalan_respons_tidak_memakai_awalan(penyimpan, handler):
    with buat_klien(handler) as klien:
        r = jalankan_adapter(AdapterPIHPS({"1171"}), klien, penyimpan, "MANUAL", SEKARANG)[0]
    assert r.status == ERROR
    assert "DILEWATI" not in r.pesan


def test_kegagalan_jaringan_tidak_memakai_awalan(penyimpan):
    def putus(_):
        raise httpx.ConnectError("putus")

    with buat_klien(putus) as klien:
        r = jalankan_adapter(AdapterOpenMeteo({"1171"}), klien, penyimpan, "MANUAL", SEKARANG)[0]
    assert r.status == ERROR
    assert "DILEWATI" not in r.pesan


def test_bps_berkunci_gagal_http_tidak_memakai_awalan(penyimpan):
    ds = DS_INFLASI
    variabel = {i.kunci: "12" for i in DATASET_BPS[ds]}
    adapter = AdapterBPS("KUNCI", variabel, {"11"})
    with buat_klien(tanpa_robots(lambda _: httpx.Response(500)), rahasia=["KUNCI"]) as klien:
        r = jalankan_dataset(adapter, ds, klien, penyimpan, SEKARANG, "MANUAL")
    assert r.status == ERROR
    assert "DILEWATI" not in r.pesan
    assert "KUNCI" not in r.pesan


def test_registri_adapter():
    cfg = Konfigurasi(mongo_uri="mongodb://x")
    for s in ("PIHPS", "BMKG", "OPEN_METEO", "BPS", "SP2KP", "BAPANAS"):
        assert buat_adapter(s, cfg).sumber == s
