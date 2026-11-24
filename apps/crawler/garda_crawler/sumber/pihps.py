"""PIHPS Bank Indonesia: harga konsumen pasar tradisional per kab/kota Aceh.

Terverifikasi ke endpoint nyata 2026-11-24. Grid mengembalikan {"data":[baris]} dengan kolom
tanggal "dd/mm/yyyy" hanya untuk hari kerja, dan angka berformat "15,850". Sel "-" berarti tidak
ada laporan, bukan galat.
"""

import re
from collections.abc import Iterable
from datetime import date, datetime, timedelta
from typing import Any
from urllib.parse import urlencode

from garda_crawler.http_sopan import GalatAmbil, GalatRobots, KlienSopan
from garda_crawler.model import DefinisiDataset, HasilAmbil, Seri, Titik, pesan_dilewati
from garda_crawler.referensi import (
    KOMODITAS,
    PIHPS_ABAIKAN,
    PIHPS_KAB_BERDATA,
    PIHPS_KOMODITAS,
    PIHPS_PROVINSI_ID,
    PIHPS_WILAYAH,
    normalisasi_alias,
)
from garda_crawler.sumber.dasar import Adapter
from garda_crawler.validasi import baca_angka, periksa_harga
from garda_crawler.waktu import potong_rentang

URL_GRID = "https://www.bi.go.id/hargapangan/WebSite/TabelHarga/GetGridDataDaerah"
DATASET = DefinisiDataset("harga_pasar_tradisional", "raw_pangan_harian")
JENIS_PASAR_TRADISIONAL = 1
JENDELA_HARIAN = 7
POTONGAN_BACKFILL_HARI = 30
_POLA_TANGGAL = re.compile(r"^\d{2}/\d{2}/\d{4}$")


def params_grid(regency_id: int, dari: date, sampai: date) -> dict[str, Any]:
    return {
        "price_type_id": JENIS_PASAR_TRADISIONAL,
        "comcat_id": "",
        "province_id": PIHPS_PROVINSI_ID,
        "regency_id": regency_id,
        "market_id": "",
        "tipe_laporan": 1,
        "start_date": dari.isoformat(),
        "end_date": sampai.isoformat(),
    }


def seri_pihps(komoditas_kode: str, kabupaten_kode: str) -> Seri:
    kom = KOMODITAS[komoditas_kode]
    return Seri(
        sumber="PIHPS",
        dataset=DATASET.nama,
        koleksi=DATASET.koleksi,
        kunci_seri=f"{komoditas_kode}:{kabupaten_kode}",
        indikator=f"harga_konsumen_{komoditas_kode}",
        satuan=f"Rp/{kom.satuan}",
        frekuensi="HARIAN",
        komoditas_kode=komoditas_kode,
        kabupaten_kode=kabupaten_kode,
        url_sumber=URL_GRID,
    )


def urai_grid(muatan: Any, kabupaten_kode: str, regency_id: int, url: str) -> HasilAmbil:
    baris = muatan.get("data") if isinstance(muatan, dict) else muatan
    if not isinstance(baris, list):
        raise GalatAmbil("respons PIHPS bukan {data:[...]}")

    hasil = HasilAmbil()
    for row in baris:
        if not isinstance(row, dict):
            hasil.ditolak["baris_rusak"] += 1
            continue
        alias = normalisasi_alias(str(row.get("name") or ""))
        if not alias or alias in PIHPS_ABAIKAN:
            continue
        kode = PIHPS_KOMODITAS.get(alias)
        if kode is None:
            hasil.ditolak["komoditas_tak_terpetakan"] += 1
            continue
        kom = KOMODITAS[kode]
        for kolom, mentah in row.items():
            if not _POLA_TANGGAL.match(str(kolom)):
                continue
            if mentah in ("-", "", None):
                continue  # Tidak ada laporan pada tanggal ini.
            try:
                periode = datetime.strptime(kolom, "%d/%m/%Y").date()
            except ValueError:
                hasil.ditolak["tanggal_rusak"] += 1
                continue
            nilai = baca_angka(mentah)
            alasan = periksa_harga(nilai, kom.harga_acuan)
            if alasan:
                hasil.ditolak[alasan] += 1
                continue
            hasil.titik.append(
                Titik(
                    seri=seri_pihps(kode, kabupaten_kode),
                    periode=periode,
                    nilai=float(nilai),
                    url_sumber=url,
                    atribut={"regency_id": regency_id, "nama_sumber": str(row["name"]).strip(), "jenis_pasar": "TRADISIONAL"},
                )
            )
    return hasil


class AdapterPIHPS(Adapter):
    sumber = "PIHPS"

    def daftar_dataset(self) -> list[DefinisiDataset]:
        return [DATASET]

    def _wilayah(self) -> list[tuple[int, str]]:
        return [(r, k) for r, k in PIHPS_WILAYAH.items() if self.termasuk(k)]

    def seri_diharapkan(self, ds: DefinisiDataset) -> list[Seri]:
        return [seri_pihps(kom, kab) for _, kab in self._wilayah() for kom in PIHPS_KOMODITAS.values()]

    def _ambil_rentang(
        self, klien: KlienSopan, dari: date, sampai: date, wilayah: list[tuple[int, str]] | None = None
    ) -> HasilAmbil:
        gabungan = HasilAmbil()
        for regency_id, kab in self._wilayah() if wilayah is None else wilayah:
            gabungan.permintaan += 1
            params = params_grid(regency_id, dari, sampai)
            try:
                muatan = klien.get_json(URL_GRID, params)
                hasil = urai_grid(muatan, kab, regency_id, f"{URL_GRID}?{urlencode(params)}")
            except GalatRobots as exc:
                gabungan.permintaan -= 1  # Diblokir sebelum permintaan data terkirim.
                gabungan.dilewati = pesan_dilewati(str(exc), "minta izin akses data PIHPS ke Bank Indonesia")
                gabungan.seri_gagal.extend(self.seri_diharapkan(DATASET))
                return gabungan
            except GalatAmbil as exc:
                gabungan.permintaan_gagal += 1
                gabungan.galat.append(f"{kab}: {exc}")
                gabungan.seri_gagal.extend(seri_pihps(k, kab) for k in PIHPS_KOMODITAS.values())
                continue
            gabungan.titik.extend(hasil.titik)
            gabungan.ditolak.update(hasil.ditolak)
        return gabungan

    def ambil_harian(self, ds: DefinisiDataset, klien: KlienSopan, tanggal: date) -> Iterable[HasilAmbil]:
        yield self._ambil_rentang(klien, tanggal - timedelta(days=JENDELA_HARIAN - 1), tanggal)

    def ambil_backfill(
        self, ds: DefinisiDataset, klien: KlienSopan, dari: date, sampai: date
    ) -> Iterable[HasilAmbil]:
        # Tanpa --wilayah, backfill hanya meminta kota yang memang berisi data.
        wilayah = self._wilayah()
        if self.wilayah is None:
            wilayah = [(r, k) for r, k in wilayah if k in PIHPS_KAB_BERDATA]
        for awal, akhir in potong_rentang(dari, sampai, POTONGAN_BACKFILL_HARI):
            hasil = self._ambil_rentang(klien, awal, akhir, wilayah)
            yield hasil
            if hasil.dilewati:
                return
