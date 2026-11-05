"""Kontrak layanan: memvalidasi muatan JSON lalu memanggil inti peramalan.

Fungsi di sini murni (muatan masuk, dict keluar) sehingga dapat diuji tanpa
membuka soket. ``server.py`` hanya menghubungkannya ke HTTP.
"""

from __future__ import annotations

from . import VERSI
from .forecast import DataTidakCukup, latih_seri, prediksi_seri, ringkas

MAKS_SERI = 100
MAKS_TITIK = 2000
MAKS_HORIZON = 90


class MuatanTidakValid(ValueError):
    """Muatan permintaan salah bentuk; dijawab 400."""


def _objek(muatan: object, nama: str) -> dict:
    if not isinstance(muatan, dict):
        raise MuatanTidakValid(f"{nama} harus berupa objek JSON")
    return muatan


def _horizon(muatan: dict) -> list[int]:
    nilai = muatan.get("horizon")
    if (
        not isinstance(nilai, list)
        or not nilai
        or not all(isinstance(h, int) and not isinstance(h, bool) and 1 <= h <= MAKS_HORIZON for h in nilai)
    ):
        raise MuatanTidakValid(f"horizon harus berisi bilangan bulat 1..{MAKS_HORIZON}")
    return nilai


def _daftar_seri(muatan: dict) -> list[dict]:
    seri = muatan.get("seri")
    if not isinstance(seri, list) or not seri:
        raise MuatanTidakValid("seri harus berisi minimal satu deret")
    if len(seri) > MAKS_SERI:
        raise MuatanTidakValid(f"seri maksimal {MAKS_SERI} per permintaan")
    for s in seri:
        s = _objek(s, "seri[]")
        if not isinstance(s.get("id"), str) or not s["id"]:
            raise MuatanTidakValid("setiap seri wajib punya id berupa teks")
        titik = s.get("titik")
        if not isinstance(titik, list) or len(titik) > MAKS_TITIK:
            raise MuatanTidakValid(f"titik harus berupa larik maksimal {MAKS_TITIK} elemen")
    return seri


def kesehatan() -> dict:
    return {"status": "ok", "versi": VERSI}


def latih(muatan: object) -> dict:
    """POST /train: melatih tiap seri, memilih model terbaik lewat backtest, dan meramal."""
    muatan = _objek(muatan, "muatan")
    horizon = _horizon(muatan)
    hasil = []
    for s in _daftar_seri(muatan):
        try:
            hasil.append({"id": s["id"], "status": "OK", **latih_seri(s["titik"], horizon)})
        except DataTidakCukup as err:
            hasil.append({"id": s["id"], "status": "DILEWATI", "alasan": str(err)})
        except ValueError as err:
            raise MuatanTidakValid(f"seri {s['id']}: {err}") from err
    return {"versi_layanan": VERSI, "komoditas_kode": muatan.get("komoditas_kode", ""), "seri": hasil, "ringkasan": ringkas(hasil)}


def prediksi(muatan: object) -> dict:
    """POST /predict: meramal dengan algoritma dan parameter yang sudah dilatih, tanpa melatih ulang."""
    muatan = _objek(muatan, "muatan")
    horizon = _horizon(muatan)
    hasil = []
    for s in _daftar_seri(muatan):
        galat = s.get("galat_rmse")
        params = s.get("params")
        if not isinstance(s.get("algoritma"), str) or not isinstance(params, dict) or not isinstance(galat, dict):
            raise MuatanTidakValid(f"seri {s['id']}: algoritma, params, dan galat_rmse wajib ada")
        try:
            hasil.append({"id": s["id"], "status": "OK", **prediksi_seri(s["titik"], s["algoritma"], params, galat, horizon)})
        except DataTidakCukup as err:
            hasil.append({"id": s["id"], "status": "DILEWATI", "alasan": str(err)})
        except ValueError as err:
            raise MuatanTidakValid(f"seri {s['id']}: {err}") from err
    return {"versi_layanan": VERSI, "komoditas_kode": muatan.get("komoditas_kode", ""), "seri": hasil}
