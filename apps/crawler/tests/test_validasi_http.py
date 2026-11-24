import httpx
import pytest

from garda_crawler.http_sopan import GalatAmbil, GalatRobots, GalatSementara, parse_robots
from garda_crawler.validasi import (
    ALASAN_DI_LUAR_BATAS,
    ALASAN_NON_POSITIF,
    ALASAN_SATUAN,
    baca_angka,
    periksa_harga,
)
from tests.conftest import buat_klien, tanpa_robots


@pytest.mark.parametrize(
    ("mentah", "harapan"),
    [
        ("15,850", 15850.0),  # Format nyata PIHPS.
        ("14.350", 14350.0),
        ("Rp 14.350,50", 14350.5),
        ("14350.5", 14350.5),
        ("999.999.999", 999999999.0),
        (54500, 54500.0),
        ("-", None),
        ("", None),
        ("abc", None),
        (None, None),
        (True, None),
    ],
)
def test_baca_angka(mentah, harapan):
    assert baca_angka(mentah) == harapan


def test_periksa_harga():
    assert periksa_harga(15000, 13900) is None
    assert periksa_harga(0, 13900) == ALASAN_NON_POSITIF
    assert periksa_harga(-5, 13900) == ALASAN_NON_POSITIF
    assert periksa_harga(999_999_999, 13900) == ALASAN_DI_LUAR_BATAS
    # Telur per butir, bukan per kg.
    assert periksa_harga(2000, 29500) == ALASAN_SATUAN
    assert periksa_harga(15000, 13900, "liter", "kg") == ALASAN_SATUAN
    assert periksa_harga(15000, 13900, "Kg", "kg") is None


def test_robots_open_meteo_melarang_semua():
    aturan = parse_robots("User-agent: *\nDisallow: /", "garda-crawler/1.0")
    assert not aturan.boleh("/v1/archive?latitude=5.55")


def test_robots_bi_aturan_terpanjang_menang():
    aturan = parse_robots("User-agent: *\nAllow: /\nDisallow:/_layouts\n", "garda-crawler/1.0")
    assert aturan.boleh("/hargapangan/WebSite/TabelHarga/GetGridDataDaerah")
    assert not aturan.boleh("/_layouts/x")


def test_robots_bmkg_grup_khusus_tidak_mengenai_agen_kita():
    isi = "User-agent: *\nDisallow: /api\nDisallow: /alerts\n\nUser-agent: Go-http-client\nUser-agent: dotbot\nDisallow: /\n"
    aturan = parse_robots(isi, "garda-crawler/1.0")
    assert not aturan.boleh("/alerts/nowcast/id")
    assert aturan.boleh("/cuaca")
    assert not parse_robots(isi, "Go-http-client/2.0").boleh("/cuaca")


def test_klien_menolak_jalur_terlarang_tanpa_memanggil_endpoint():
    panggilan = []

    def handler(req):
        panggilan.append(req.url.path)
        if req.url.path == "/robots.txt":
            return httpx.Response(200, text="User-agent: *\nDisallow: /", headers={"content-type": "text/plain"})
        return httpx.Response(200, json={})

    with buat_klien(handler) as klien, pytest.raises(GalatRobots):
        klien.get_json("https://contoh.test/data")
    assert panggilan == ["/robots.txt"]


def test_robots_5xx_gagal_tertutup():
    panggilan = []

    def handler(req):
        panggilan.append(req.url.path)
        return httpx.Response(503)

    with buat_klien(handler) as klien, pytest.raises(GalatSementara):
        klien.get_json("https://contoh.test/data")
    assert "/data" not in panggilan


def test_retry_lalu_berhasil_dengan_user_agent_generik():
    percobaan = []

    def handler(req):
        percobaan.append(req.headers["user-agent"])
        if len(percobaan) < 3:
            return httpx.Response(503)
        return httpx.Response(200, json={"ok": True})

    with buat_klien(tanpa_robots(handler), percobaan=4) as klien:
        assert klien.get_json("https://contoh.test/data") == {"ok": True}
    assert percobaan == ["garda-crawler/uji"] * 3


def test_4xx_tidak_diulang():
    percobaan = []

    def handler(req):
        percobaan.append(1)
        return httpx.Response(401, json={"message": "unauthorized"})

    with buat_klien(tanpa_robots(handler), percobaan=4) as klien, pytest.raises(GalatAmbil):
        klien.get_json("https://contoh.test/data")
    assert len(percobaan) == 1


def test_pesan_galat_tidak_memuat_kunci():
    rahasia = "KUNCIRAHASIA123"

    def handler(req):
        raise httpx.ConnectError(f"gagal ke {req.url}")

    with buat_klien(tanpa_robots(handler), rahasia=[rahasia]) as klien:
        with pytest.raises(GalatAmbil) as info:
            klien._robots["https://contoh.test"] = parse_robots("", "x")
            klien.get_json(f"https://contoh.test/data/key/{rahasia}")
    assert rahasia not in str(info.value)
    assert "***" in str(info.value)


def test_redirect_tidak_diikuti():
    tujuan = []

    def handler(req):
        tujuan.append(req.url.host)
        return httpx.Response(302, headers={"location": "https://lain.test/x"})

    with buat_klien(tanpa_robots(handler)) as klien, pytest.raises(GalatAmbil):
        klien.get_json("https://contoh.test/data/key/RAHASIA")
    assert tujuan == ["contoh.test"]
