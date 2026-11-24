"""Tes end-to-end siklus terhadap MongoDB sungguhan (data sintetis)."""

from __future__ import annotations

from datetime import date, datetime, timedelta, timezone

import pytest

from garda_ml.pipeline import siklus as modul_siklus
from garda_ml.pipeline.config import Konfigurasi
from garda_ml.pipeline.penyimpanan import K_EWS, K_FORECAST, K_INFLASI, K_KUNCI, K_MAKRO, K_SIKLUS, K_XAI, KOLEKSI_HASIL, KunciSiklus
from garda_ml.pipeline.siklus import jalankan_siklus

from . import sintetis

pytestmark = pytest.mark.integrasi

AKHIR = date(2026, 6, 30)
JAM = datetime(2026, 6, 30, 10, 12, tzinfo=timezone.utc)
ID_JAM = "C20260630T10Z"
KOMODITAS = {"CBM-KRT": 45000.0, "BRS-MED": 14000.0}


def _cfg(klien, nama_db) -> Konfigurasi:
    return Konfigurasi(mongo_uri="mongodb://tidak-dipakai", db_unseen=nama_db[0], db_safe=nama_db[1], n_jobs=1, maks_baris_latih=15_000)


def _isi(klien, nama_db, **kw) -> None:
    kw.setdefault("hari", 420)
    kw.setdefault("komoditas", KOMODITAS)
    sintetis.isi_dataunseen(klien[nama_db[0]], AKHIR, **kw)


def _cuplikan(db, cycle_id: str) -> dict:
    return {k: sorted(db[k].find({"cycle_id": cycle_id}), key=lambda d: d["_id"]) for k in (*KOLEKSI_HASIL, K_SIKLUS)}


def test_siklus_lengkap_klasifikasi_regresi_shap(klien_mongo, nama_db):
    _isi(klien_mongo, nama_db)
    unseen = klien_mongo[nama_db[0]]
    sebelum = {k: unseen[k].count_documents({}) for k in unseen.list_collection_names()}

    hasil = jalankan_siklus(_cfg(klien_mongo, nama_db), klien_mongo, JAM)
    db = klien_mongo[nama_db[1]]

    assert hasil["status"] == "BERHASIL", hasil["pesan"]
    assert {k: unseen[k].count_documents({}) for k in unseen.list_collection_names()} == sebelum

    c = db[K_SIKLUS].find_one({"_id": ID_JAM})
    for f in ("cycle_id", "cycle_timestamp", "data_cutoff", "duration_ms", "status", "pesan", "versi_kode", "jumlah", "source_freshness", "target_inflasi", "model_metrics"):
        assert f in c, f
    assert c["target_inflasi"] == "INDEKS_PANGAN"
    assert c["data_cutoff"] == datetime(2026, 6, 30, 10, tzinfo=timezone.utc)
    kl, rg = c["model_metrics"]["klasifikasi"], c["model_metrics"]["regresi"]
    assert kl["model"] in ("lightgbm", "hist_gradient_boosting") and kl["n_uji"] > 0 and kl["brier"] is not None
    assert isinstance(kl["layak"], bool) and kl["brier_baseline"] is not None and kl["alasan"]
    assert sum(rg["pemenang"].values()) == 4 and rg["mape_baseline_median"] is not None and rg["tingkat"] == "PENUH"
    assert rg["n_baseline"] == rg["pemenang"]["naive"] + rg["pemenang"]["seasonal_naive"]
    assert rg["porsi_baseline"] == pytest.approx(rg["n_baseline"] / rg["n_seri"])
    assert rg["n_seri"] == 4 and rg["n_dievaluasi_holdout"] == 4 and 0 <= rg["mengalahkan_baseline"] <= 4

    fc = list(db[K_FORECAST].find({"cycle_id": ID_JAM}))
    assert len(fc) == 4
    for d in fc:
        assert [x["horizon"] for x in d["kurva"]] == [7, 14, 30]
        for x in d["kurva"]:
            assert 0 < x["p10"] <= x["p50"] <= x["p90"]
        assert d["kurva"][0]["tanggal"] == datetime(2026, 7, 7, tzinfo=timezone.utc)
        assert d["riwayat"] and d["metrik"]["n_lipatan"] == 5 and d["metrik"]["n_lipatan_seleksi"] == 4 and d["metrik"]["mape_baseline"] is not None
        assert d["model"]["nama"] in ("lightgbm_quantile", "hgb_quantile", "sarimax", "holt_winters", "seasonal_naive", "naive")
        assert d["metrik"]["baseline"] in ("naive", "seasonal_naive") and d["metrik"]["evaluasi"] == "holdout_lipatan_terakhir"

    sinyal = list(db[K_INFLASI].find({"cycle_id": ID_JAM}))
    assert {s["wilayah_kode"] for s in sinyal} == {"1171", "1174"}
    for s in sinyal:
        assert s["target"] == "INDEKS_PANGAN" and s["penyumbang"] and s["kalibrasi"]["metode"] and s["layak"] is kl["layak"] and s["alasan"]
        if s["layak"]:
            assert s["kelas"] in ("INFLASI", "TIDAK_INFLASI") and s["siklus"] in ("NORMAL", "TEKANAN", "SURPLUS")
        else:
            assert s["kelas"] is None and s["skor_tekanan"] is None and s["siklus"] is None

    xai = list(db[K_XAI].find({"cycle_id": ID_JAM}))
    inflasi = [x for x in xai if x["target"] == "inflasi"]
    assert len(inflasi) == (2 if kl["layak"] else 0)
    assert all(x["metode"] == "shap" and x["satuan_kontribusi"] == "poin_persen_probabilitas" and x["narasi"] for x in inflasi)
    assert len([x for x in xai if x["target"] == "forecast"]) == 4
    assert all(x["satuan_kontribusi"] == "poin_persen_harga" for x in xai if x["target"] == "forecast")
    kelompok = {"CUACA", "PASOKAN", "KONSUMSI_PENDAPATAN", "TENAGA_KERJA", "NERACA_EKONOMI", "TREN_HARGA_LAPANGAN", "KALENDER"}
    assert all(f["kelompok"] in kelompok for x in xai for f in x["faktor"])
    for d in fc:
        x = next(v for v in xai if v["target"] == "forecast" and v["komoditas_kode"] == d["komoditas_kode"] and v["wilayah_kode"] == d["kabupaten_kode"])
        assert x["metode"] == ("shap" if d["model"]["nama"].endswith("_quantile") else "dekomposisi")

    assert db[K_MAKRO].count_documents({"cycle_id": ID_JAM}) == 4
    assert {"KONSUMSI_PENDAPATAN", "NERACA_EKONOMI", "TENAGA_KERJA", "PERTANIAN"} == set(db[K_MAKRO].distinct("domain", {"cycle_id": ID_JAM}))
    indeks = db[K_FORECAST].index_information()
    assert any(v["key"][:2] == [("komoditas_kode", 1), ("kabupaten_kode", 1)] for v in indeks.values())


def test_klasifikasi_layak_menerbitkan_xai_terkalibrasi(klien_mongo, nama_db):
    unseen = klien_mongo[nama_db[0]]
    unseen["raw_pangan_harian"].insert_many(sintetis.harga_harian(AKHIR, 1000, komoditas=KOMODITAS, wilayah=("1171", "1174", "1107"), amplitudo_tahunan=0.06, derau=0.0015))
    unseen["crawl_runs"].insert_one(sintetis.crawl_run("PIHPS", "harga_pangan", AKHIR))
    hasil = jalankan_siklus(_cfg(klien_mongo, nama_db), klien_mongo, JAM)
    db = klien_mongo[nama_db[1]]
    kl = hasil["model_metrics"]["klasifikasi"]
    assert kl["layak"] is True, kl
    sinyal = {s["wilayah_kode"]: s for s in db[K_INFLASI].find({"cycle_id": ID_JAM})}
    assert set(sinyal) == {"1171", "1174", "1107"} and all(s["layak"] and s["kelas"] for s in sinyal.values())
    xai = list(db[K_XAI].find({"cycle_id": ID_JAM, "target": "inflasi"}))
    assert len(xai) == 3
    for x in xai:
        assert x["satuan_kontribusi"] == "poin_persen_probabilitas"
        assert x["probabilitas"] == pytest.approx(sinyal[x["wilayah_kode"]]["probabilitas"], abs=1e-6)
        assert sum(f["kontribusi"] for f in x["faktor"]) == pytest.approx((x["probabilitas"] - x["probabilitas_dasar"]) * 100, abs=1e-2)


def test_klasifikasi_tanpa_sinyal_tidak_layak(klien_mongo, nama_db):
    unseen = klien_mongo[nama_db[0]]
    unseen["raw_pangan_harian"].insert_many(sintetis.harga_harian(AKHIR, 1000, seed=21, komoditas=KOMODITAS, wilayah=("1171", "1174", "1107"), amplitudo_tahunan=0.0))
    unseen["crawl_runs"].insert_one(sintetis.crawl_run("PIHPS", "harga_pangan", AKHIR))
    hasil = jalankan_siklus(_cfg(klien_mongo, nama_db), klien_mongo, JAM)
    db = klien_mongo[nama_db[1]]
    kl = hasil["model_metrics"]["klasifikasi"]
    assert kl["layak"] is False and kl["alasan"] and kl["brier_baseline"] is not None, kl
    sinyal = list(db[K_INFLASI].find({"cycle_id": ID_JAM}))
    assert sinyal and all(s["kelas"] is None and s["layak"] is False and s["alasan"] and s["probabilitas"] is not None for s in sinyal)
    assert db[K_XAI].count_documents({"cycle_id": ID_JAM, "target": "inflasi"}) == 0
    assert db[K_EWS].count_documents({"cycle_id": ID_JAM, "jenis": "TEKANAN_INFLASI"}) == 0
    assert "belum layak" in hasil["pesan"]


def test_shap_dipakai_saat_model_kuantil_menang(klien_mongo, nama_db, monkeypatch):
    _isi(klien_mongo, nama_db)
    # Paksa model kuantil menang untuk membuktikan jalur SHAP forecast.
    monkeypatch.setattr("garda_ml.pipeline.regresi.pilih_pemenang", lambda s, *a, **k: next((n for n in s if n.endswith("_quantile")), "seasonal_naive"))
    assert jalankan_siklus(_cfg(klien_mongo, nama_db), klien_mongo, JAM)["status"] == "BERHASIL"
    db = klien_mongo[nama_db[1]]
    fc = list(db[K_FORECAST].find({"cycle_id": ID_JAM}))
    assert fc and all(d["model"]["nama"].endswith("_quantile") for d in fc)
    xai = list(db[K_XAI].find({"cycle_id": ID_JAM, "target": "forecast"}))
    assert xai and all(x["metode"] == "shap" and x["faktor"] for x in xai)


def test_bps_cukup_memakai_resmi(klien_mongo, nama_db):
    _isi(klien_mongo, nama_db, hari=900)
    klien_mongo[nama_db[0]]["raw_bps_makro"].insert_many(sintetis.inflasi_bps(AKHIR, 30, wilayah=("1171", "1174", "11")))
    assert jalankan_siklus(_cfg(klien_mongo, nama_db), klien_mongo, JAM)["status"] == "BERHASIL"
    db = klien_mongo[nama_db[1]]
    assert db[K_SIKLUS].find_one({"_id": ID_JAM})["target_inflasi"] == "RESMI_BPS"
    sinyal = list(db[K_INFLASI].find({"cycle_id": ID_JAM}))
    assert {s["target"] for s in sinyal} == {"RESMI_BPS"} and all(s["aktual"]["mtm"] is not None for s in sinyal)
    assert db[K_MAKRO].count_documents({"cycle_id": ID_JAM, "domain": "INFLASI"}) >= 3


def test_kondisi_data_nyata_saat_ini(klien_mongo, nama_db):
    """PIHPS 3 wilayah, tanpa BPS, tanpa riwayat iklim, tanpa LAPANGAN, adapter dilewati."""
    unseen = klien_mongo[nama_db[0]]
    unseen["raw_pangan_harian"].insert_many(sintetis.harga_harian(AKHIR, 420, komoditas=KOMODITAS, wilayah=("1171", "1174", "1107")))
    # Dokumen ERROR untuk komoditas tanpa data: is_carried_forward false, nilai null.
    galat = sintetis.dokumen("PIHPS", "harga_pangan", "CBR-RWT:1171", AKHIR, None, komoditas_kode="CBR-RWT", kabupaten_kode="1171")
    galat.update(source_status="ERROR", last_available_date=None)
    unseen["raw_pangan_harian"].insert_one(galat)
    # Prakiraan BMKG: hari ini dan masa depan, tidak boleh menjadi fitur.
    prakiraan = []
    for geser in range(0, 3):
        t = AKHIR + timedelta(days=geser)
        prakiraan.append(sintetis.dokumen("BMKG", "prakiraan_cuaca_adm4", "curah_hujan:1171", t, 80.0, crawl=AKHIR, kabupaten_kode="1171", indikator="curah_hujan_harian", satuan="mm", atribut={"jenis_data": "PRAKIRAAN", "horizon_hari": geser}))
    unseen["raw_iklim_bmkg"].insert_many(prakiraan)
    dilewati = sintetis.crawl_run("SP2KP", "harga_kemendag", AKHIR, status="ERROR", pesan="DILEWATI: butuh token resmi")
    dilewati["last_available_date"] = None
    backfill = sintetis.crawl_run("PIHPS", "harga_pangan", AKHIR, status="ERROR", pesan="backfill gagal")
    backfill.update(_id="run:backfill", pemicu="BACKFILL", mulai=backfill["mulai"] + timedelta(hours=2))
    unseen["crawl_runs"].insert_many([sintetis.crawl_run("PIHPS", "harga_pangan", AKHIR), dilewati, backfill])

    hasil = jalankan_siklus(_cfg(klien_mongo, nama_db), klien_mongo, JAM)
    db = klien_mongo[nama_db[1]]
    assert hasil["status"] == "BERHASIL", hasil["pesan"]
    assert hasil["target_inflasi"] == "INDEKS_PANGAN"
    fc = list(db[K_FORECAST].find({"cycle_id": ID_JAM}))
    assert {d["kabupaten_kode"] for d in fc} == {"1171", "1174", "1107"} and len(fc) == 6
    assert not [d for d in fc if d["komoditas_kode"] == "CBR-RWT"]
    assert {s["wilayah_kode"] for s in db[K_INFLASI].find({"cycle_id": ID_JAM})} == {"1171", "1174", "1107"}
    assert all(s["target"] == "INDEKS_PANGAN" for s in db[K_INFLASI].find({"cycle_id": ID_JAM}))
    assert not [f for x in db[K_XAI].find({"cycle_id": ID_JAM}) for f in x["faktor"] if f["kelompok"] == "CUACA"]
    segar = {(s["sumber"], s["dataset"]): s for s in db[K_SIKLUS].find_one({"_id": ID_JAM})["source_freshness"]}
    assert segar[("PIHPS", "harga_pangan")]["status"] == "FRESH"  # run BACKFILL yang lebih baru diabaikan
    info = db[K_EWS].find_one({"_id": f"{ID_JAM}:SUMBER_DATA:SP2KP:harga_kemendag"})
    assert info["severity"] == "INFO" and info["aktif"] is True and info["sinyal_kunci"] == "SUMBER_DATA:SP2KP:harga_kemendag"
    assert segar[("SP2KP", "harga_kemendag")]["pesan"].startswith("DILEWATI:")
    assert not db[K_EWS].find_one({"_id": f"{ID_JAM}:SUMBER_DATA:HARGA:SEMUA"})


def test_idempoten_jam_yang_sama(klien_mongo, nama_db):
    _isi(klien_mongo, nama_db)
    cfg = _cfg(klien_mongo, nama_db)
    pertama = jalankan_siklus(cfg, klien_mongo, JAM)
    db = klien_mongo[nama_db[1]]
    cuplikan = _cuplikan(db, ID_JAM)
    kedua = jalankan_siklus(cfg, klien_mongo, JAM + timedelta(minutes=40))
    assert pertama["status"] == "BERHASIL" and kedua["dilewati"] is True and kedua["cycle_id"] == ID_JAM
    assert _cuplikan(db, ID_JAM) == cuplikan


def test_append_only_dan_fit_ulang(klien_mongo, nama_db):
    _isi(klien_mongo, nama_db)
    cfg = _cfg(klien_mongo, nama_db)
    jalankan_siklus(cfg, klien_mongo, JAM)
    db = klien_mongo[nama_db[1]]
    cuplikan = _cuplikan(db, ID_JAM)
    hasil = jalankan_siklus(cfg, klien_mongo, JAM + timedelta(hours=1))
    assert hasil["status"] == "BERHASIL" and hasil["cycle_id"] == "C20260630T11Z"
    assert _cuplikan(db, ID_JAM) == cuplikan
    assert db[K_FORECAST].count_documents({}) == 8
    reg = hasil["model_metrics"]["regresi"]
    assert reg["tingkat"] == "FIT_ULANG" and reg["n_diseleksi"] == 0 and reg["seleksi_cycle_id"] == ID_JAM
    assert "tidak ada data baru" in hasil["pesan"]
    juara_1 = {(d["komoditas_kode"], d["kabupaten_kode"]): d["model"]["nama"] for d in db[K_FORECAST].find({"cycle_id": ID_JAM})}
    juara_2 = {(d["komoditas_kode"], d["kabupaten_kode"]): d["model"]["nama"] for d in db[K_FORECAST].find({"cycle_id": "C20260630T11Z"})}
    assert juara_1 == juara_2
    # Setelah interval seleksi, seleksi penuh berjalan lagi.
    nanti = jalankan_siklus(cfg, klien_mongo, JAM + timedelta(hours=24))
    assert nanti["model_metrics"]["regresi"]["tingkat"] == "PENUH"


def test_kritis_tetap_ada_lintas_siklus(klien_mongo, nama_db):
    _isi(klien_mongo, nama_db)
    unseen = klien_mongo[nama_db[0]]
    unseen["raw_pangan_harian"].update_one(
        {"komoditas_kode": "CBM-KRT", "kabupaten_kode": "1171", "periode": datetime(2026, 6, 30, tzinfo=timezone.utc)},
        {"$mul": {"nilai": 1.7}},
    )
    cfg = _cfg(klien_mongo, nama_db)
    jalankan_siklus(cfg, klien_mongo, JAM)
    jalankan_siklus(cfg, klien_mongo, JAM + timedelta(hours=1))
    db = klien_mongo[nama_db[1]]
    kunci = "ANOMALI_HARGA:CBM-KRT:1171:NAIK"
    for cid in (ID_JAM, "C20260630T11Z"):
        d = db[K_EWS].find_one({"_id": f"{cid}:{kunci}"})
        assert d and d["severity"] == "KRITIS" and d["aktif"] is True and d["sinyal_kunci"] == kunci


def test_sumber_basi_memakai_last_available_date(klien_mongo, nama_db):
    """Dokumen carried-forward tidak menjadi titik latih, tetapi umurnya tercatat."""
    akhir_asli = AKHIR - timedelta(days=20)
    unseen = klien_mongo[nama_db[0]]
    unseen["raw_pangan_harian"].insert_many(sintetis.harga_harian(akhir_asli, 400, komoditas=KOMODITAS))
    lad = datetime(akhir_asli.year, akhir_asli.month, akhir_asli.day, tzinfo=timezone.utc)
    cf = []
    for i in range(1, 21):
        t = akhir_asli + timedelta(days=i)
        d = sintetis.dokumen("PIHPS", "harga_pangan", "CBM-KRT:1171", t, 999999.0, komoditas_kode="CBM-KRT", kabupaten_kode="1171")
        d.update(is_carried_forward=True, last_available_date=lad, source_status="STALE_CARRIED_FORWARD")
        cf.append(d)
    unseen["raw_pangan_harian"].insert_many(cf)
    unseen["crawl_runs"].insert_one(sintetis.crawl_run("PIHPS", "harga_pangan", AKHIR, status="STALE_CARRIED_FORWARD", lad=akhir_asli, pesan="sumber belum rilis"))

    hasil = jalankan_siklus(_cfg(klien_mongo, nama_db), klien_mongo, JAM)
    db = klien_mongo[nama_db[1]]
    assert hasil["status"] in ("BERHASIL", "SEBAGIAN")
    d = db[K_FORECAST].find_one({"cycle_id": ID_JAM, "komoditas_kode": "CBM-KRT", "kabupaten_kode": "1171"})
    assert d["tanggal_terakhir"] == lad and d["harga_terakhir"] < 100000
    assert all(x["nilai"] < 100000 for x in d["riwayat"]) and all(x["p90"] < 200000 for x in d["kurva"])
    assert "20 hari" in d["model"]["alasan"]
    segar = db[K_SIKLUS].find_one({"_id": ID_JAM})["source_freshness"]
    assert segar[0]["status"] == "STALE_CARRIED_FORWARD" and segar[0]["umur_hari"] == 20 and segar[0]["last_available_date"] == lad
    s = db[K_EWS].find_one({"_id": f"{ID_JAM}:SUMBER_DATA:PIHPS:harga_pangan"})
    assert s and s["severity"] == "WASPADA"
    assert db[K_EWS].find_one({"_id": f"{ID_JAM}:SUMBER_DATA:HARGA:SEMUA"})["severity"] == "KRITIS"


def test_tanpa_data_tetap_siklus_jujur(klien_mongo, nama_db):
    hasil = jalankan_siklus(_cfg(klien_mongo, nama_db), klien_mongo, JAM)
    db = klien_mongo[nama_db[1]]
    assert hasil["status"] == "SEBAGIAN" and "tidak ada titik harga" in hasil["pesan"]
    assert db[K_FORECAST].count_documents({}) == 0 and db[K_INFLASI].count_documents({}) == 0
    assert db[K_SIKLUS].find_one({"_id": ID_JAM})["target_inflasi"] is None
    assert db[K_EWS].find_one({"_id": f"{ID_JAM}:SUMBER_DATA:HARGA:SEMUA"})["severity"] == "KRITIS"


def test_lapangan_digabung_dengan_bobot(klien_mongo, nama_db):
    _isi(klien_mongo, nama_db)
    unseen = klien_mongo[nama_db[0]]
    dok = sintetis.harga_harian(AKHIR, 60, komoditas={"IKN-TGK": 30000.0}, wilayah=("1171",), sumber="LAPANGAN")
    for i, d in enumerate(dok):
        d["status_validasi"] = "DIVALIDASI" if i % 2 else "DIPUBLIKASI"
    tolak = sintetis.dokumen("LAPANGAN", "harga_pangan", "IKN-TGK:1174", AKHIR, 30000.0, komoditas_kode="IKN-TGK", kabupaten_kode="1174", status_validasi="DITOLAK")
    unseen["raw_pangan_harian"].insert_many([*dok, tolak])
    jalankan_siklus(_cfg(klien_mongo, nama_db), klien_mongo, JAM)
    db = klien_mongo[nama_db[1]]
    assert db[K_FORECAST].find_one({"cycle_id": ID_JAM, "komoditas_kode": "IKN-TGK", "kabupaten_kode": "1171"})
    assert not db[K_FORECAST].find_one({"cycle_id": ID_JAM, "komoditas_kode": "IKN-TGK", "kabupaten_kode": "1174"})


def test_kegagalan_seri_menjadi_sebagian(klien_mongo, nama_db, monkeypatch):
    _isi(klien_mongo, nama_db)
    asli = modul_siklus.jalankan_regresi.__globals__["_final_statistik"]

    def rusak(seri, nama):
        if seri.komoditas_kode == "CBM-KRT" and seri.kabupaten_kode == "1171":
            return (seri.komoditas_kode, seri.kabupaten_kode), nama, None, "uji: fit gagal"
        return asli(seri, nama)

    monkeypatch.setitem(modul_siklus.jalankan_regresi.__globals__, "_final_statistik", rusak)
    monkeypatch.setitem(modul_siklus.jalankan_regresi.__globals__, "pilih_pemenang", lambda *a, **k: "seasonal_naive")
    hasil = jalankan_siklus(_cfg(klien_mongo, nama_db), klien_mongo, JAM)
    db = klien_mongo[nama_db[1]]
    assert hasil["status"] == "SEBAGIAN" and db[K_FORECAST].count_documents({"cycle_id": ID_JAM}) == 3
    assert hasil["model_metrics"]["regresi"]["n_gagal"] == 1


def test_klasifikasi_gagal_menjadi_sebagian(klien_mongo, nama_db, monkeypatch):
    _isi(klien_mongo, nama_db)

    def rusak(*_a, **_k):
        raise RuntimeError("uji: klasifikasi rusak")

    monkeypatch.setattr(modul_siklus, "jalankan_klasifikasi", rusak)
    hasil = jalankan_siklus(_cfg(klien_mongo, nama_db), klien_mongo, JAM)
    db = klien_mongo[nama_db[1]]
    assert hasil["status"] == "SEBAGIAN" and db[K_FORECAST].count_documents({"cycle_id": ID_JAM}) == 4
    assert db[K_INFLASI].count_documents({"cycle_id": ID_JAM}) == 0
    assert db[K_SIKLUS].find_one({"_id": ID_JAM})["target_inflasi"] is None


def test_gagal_tulis_tidak_terlihat_pembaca(klien_mongo, nama_db, monkeypatch):
    _isi(klien_mongo, nama_db)
    cfg = _cfg(klien_mongo, nama_db)
    jalankan_siklus(cfg, klien_mongo, JAM)
    db = klien_mongo[nama_db[1]]
    asli = modul_siklus.tulis_dokumen

    def rusak(d, koleksi, dokumen, *a):
        if koleksi == K_EWS:
            raise RuntimeError("uji: gagal tulis")
        return asli(d, koleksi, dokumen, *a)

    monkeypatch.setattr(modul_siklus, "tulis_dokumen", rusak)
    id_baru = "C20260630T11Z"
    with pytest.raises(RuntimeError):
        jalankan_siklus(cfg, klien_mongo, JAM + timedelta(hours=1))
    assert db[K_SIKLUS].find_one({"_id": id_baru})["status"] == "GAGAL"
    assert all(db[k].count_documents({"cycle_id": id_baru}) == 0 for k in KOLEKSI_HASIL)
    terbaru = db[K_SIKLUS].find_one({"status": {"$in": ["BERHASIL", "SEBAGIAN"]}}, sort=[("cycle_timestamp", -1)])
    assert terbaru["_id"] == ID_JAM
    # Jam yang sama dapat dicoba ulang setelah gangguan hilang.
    monkeypatch.setattr(modul_siklus, "tulis_dokumen", asli)
    assert jalankan_siklus(cfg, klien_mongo, JAM + timedelta(hours=1))["status"] == "BERHASIL"


def test_restart_membereskan_siklus_terputus(klien_mongo, nama_db):
    _isi(klien_mongo, nama_db)
    db = klien_mongo[nama_db[1]]
    ts = JAM.replace(minute=0) - timedelta(hours=1)
    db[K_SIKLUS].insert_one({"_id": "C20260630T09Z", "cycle_id": "C20260630T09Z", "cycle_timestamp": ts, "status": "BERJALAN"})
    db[K_FORECAST].insert_one({"_id": "C20260630T09Z:X:Y", "cycle_id": "C20260630T09Z", "cycle_timestamp": ts, "komoditas_kode": "X", "kabupaten_kode": "Y"})
    assert jalankan_siklus(_cfg(klien_mongo, nama_db), klien_mongo, JAM)["status"] == "BERHASIL"
    assert db[K_SIKLUS].find_one({"_id": "C20260630T09Z"})["status"] == "GAGAL"
    assert db[K_FORECAST].count_documents({"cycle_id": "C20260630T09Z"}) == 0


def test_kunci_mencegah_tumpang_tindih_dan_pulih_setelah_kedaluwarsa(klien_mongo, nama_db):
    _isi(klien_mongo, nama_db)
    db = klien_mongo[nama_db[1]]
    kini = datetime.now(timezone.utc)
    db[K_KUNCI].insert_one({"_id": KunciSiklus.NAMA, "pemilik": "pekerja-lain", "kedaluwarsa": kini + timedelta(minutes=5)})
    cfg = _cfg(klien_mongo, nama_db)
    hasil = jalankan_siklus(cfg, klien_mongo, JAM)
    assert hasil["dilewati"] is True and hasil["alasan"] == "kunci dipegang pekerja lain"
    assert db[K_SIKLUS].count_documents({}) == 0
    # Pekerja lain mati: lease kedaluwarsa sehingga siklus bisa jalan.
    db[K_KUNCI].update_one({"_id": KunciSiklus.NAMA}, {"$set": {"kedaluwarsa": kini - timedelta(seconds=1)}})
    assert jalankan_siklus(cfg, klien_mongo, JAM)["status"] == "BERHASIL"


def test_zombie_tidak_bisa_memfinalkan(klien_mongo, nama_db):
    db = klien_mongo[nama_db[1]]
    a = KunciSiklus(db, 60)
    assert a.ambil()
    db[K_KUNCI].update_one({"_id": KunciSiklus.NAMA}, {"$set": {"kedaluwarsa": datetime.now(timezone.utc) - timedelta(seconds=1)}})
    b = KunciSiklus(db, 60)
    assert b.ambil()
    with pytest.raises(RuntimeError):
        a.pastikan()
    b.pastikan()
    a.lepas()
    b.lepas()
