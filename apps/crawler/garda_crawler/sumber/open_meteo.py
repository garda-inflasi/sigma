"""Open-Meteo Historical Weather: sumber iklim SEKUNDER untuk backfill.

Nilainya reanalisis pada koordinat ibu kota kab/kota, bukan pengamatan BMKG, sehingga ditandai
`sumber: OPEN_METEO` dan `atribut.sumber_sekunder: true`.

PERHATIAN: per 2026-11-24 robots.txt archive-api.open-meteo.com berisi `Disallow: /`. Klien mematuhinya,
sehingga adapter ini berakhir ERROR sampai pemilik proyek memutuskan jalur akses yang sah. Bentuk respons
pada fixture berasal dari dokumentasi Open-Meteo, belum diuji ke layanan nyata.
"""

from collections.abc import Iterable
from datetime import date, datetime, timedelta
from typing import Any
from urllib.parse import urlencode

from garda_crawler.http_sopan import GalatAmbil, GalatRobots, KlienSopan
from garda_crawler.model import DefinisiDataset, HasilAmbil, Seri, Titik, pesan_dilewati
from garda_crawler.referensi import KABUPATEN, Kabupaten
from garda_crawler.sumber.dasar import Adapter
from garda_crawler.validasi import iklim_sah
from garda_crawler.waktu import potong_rentang

URL_ARSIP = "https://archive-api.open-meteo.com/v1/archive"
DATASET = DefinisiDataset("arsip_iklim_harian", "raw_iklim_bmkg")
POTONGAN_BACKFILL_HARI = 366
# Arsip reanalisis tertinggal beberapa hari dari hari ini.
JENDELA_AWAL_HARI = 14
JENDELA_AKHIR_HARI = 2

VARIABEL = {
    "precipitation_sum": ("curah_hujan", "curah_hujan_harian", "mm"),
    "temperature_2m_mean": ("suhu", "suhu_rata_harian", "celsius"),
    "relative_humidity_2m_mean": ("kelembapan", "kelembapan_rata_harian", "persen"),
}


def seri_open_meteo(variabel: str, kabupaten_kode: str) -> Seri:
    kunci, nama, satuan = VARIABEL[variabel]
    return Seri(
        sumber="OPEN_METEO",
        dataset=DATASET.nama,
        koleksi=DATASET.koleksi,
        kunci_seri=f"{kunci}:{kabupaten_kode}",
        indikator=nama,
        satuan=satuan,
        frekuensi="HARIAN",
        kabupaten_kode=kabupaten_kode,
        url_sumber=URL_ARSIP,
    )


def params_arsip(kab: Kabupaten, dari: date, sampai: date) -> dict[str, Any]:
    return {
        "latitude": kab.lat,
        "longitude": kab.lng,
        "start_date": dari.isoformat(),
        "end_date": sampai.isoformat(),
        "daily": ",".join(VARIABEL),
        "timezone": "Asia/Jakarta",
    }


def urai_arsip(muatan: Any, kab: Kabupaten, url: str) -> HasilAmbil:
    harian = muatan.get("daily") if isinstance(muatan, dict) else None
    if not isinstance(harian, dict) or not isinstance(harian.get("time"), list):
        raise GalatAmbil("respons Open-Meteo tanpa blok daily")

    hasil = HasilAmbil()
    atribut = {
        "sumber_sekunder": True,
        "jenis_data": "REANALISIS",
        "lat": kab.lat,
        "lng": kab.lng,
        "ibukota": kab.ibukota,
    }
    for variabel, (indikator, _, _) in VARIABEL.items():
        nilai = harian.get(variabel)
        if not isinstance(nilai, list) or len(nilai) != len(harian["time"]):
            raise GalatAmbil(f"respons Open-Meteo: kolom {variabel} tidak sepanjang time")
        for teks, v in zip(harian["time"], nilai, strict=True):
            if v is None:
                continue  # Arsip belum tersedia untuk tanggal ini.
            if isinstance(v, bool) or not isinstance(v, (int, float)) or not iklim_sah(indikator, float(v)):
                hasil.ditolak[f"{indikator}_tidak_wajar"] += 1
                continue
            try:
                periode = datetime.strptime(teks, "%Y-%m-%d").date()
            except (TypeError, ValueError):
                hasil.ditolak["tanggal_rusak"] += 1
                continue
            hasil.titik.append(Titik(seri_open_meteo(variabel, kab.kode), periode, float(v), url, dict(atribut)))
    return hasil


class AdapterOpenMeteo(Adapter):
    sumber = "OPEN_METEO"

    def daftar_dataset(self) -> list[DefinisiDataset]:
        return [DATASET]

    def _wilayah(self) -> list[Kabupaten]:
        return [k for k in KABUPATEN.values() if self.termasuk(k.kode)]

    def seri_diharapkan(self, ds: DefinisiDataset) -> list[Seri]:
        return [seri_open_meteo(v, k.kode) for k in self._wilayah() for v in VARIABEL]

    def _ambil_rentang(self, klien: KlienSopan, dari: date, sampai: date) -> HasilAmbil:
        gabungan = HasilAmbil()
        for kab in self._wilayah():
            gabungan.permintaan += 1
            params = params_arsip(kab, dari, sampai)
            try:
                muatan = klien.get_json(URL_ARSIP, params)
                hasil = urai_arsip(muatan, kab, f"{URL_ARSIP}?{urlencode(params)}")
            except GalatRobots as exc:
                gabungan.permintaan -= 1  # Diblokir sebelum permintaan data terkirim.
                gabungan.dilewati = pesan_dilewati(
                    str(exc), "putuskan jalur akses yang sah: izin eksplisit, paket berbayar, atau ERA5 Copernicus"
                )
                gabungan.seri_gagal.extend(self.seri_diharapkan(DATASET))
                return gabungan
            except GalatAmbil as exc:
                gabungan.permintaan_gagal += 1
                gabungan.galat.append(f"{kab.kode}: {exc}")
                gabungan.seri_gagal.extend(seri_open_meteo(v, kab.kode) for v in VARIABEL)
                continue
            gabungan.titik.extend(hasil.titik)
            gabungan.ditolak.update(hasil.ditolak)
        return gabungan

    def ambil_harian(self, ds: DefinisiDataset, klien: KlienSopan, tanggal: date) -> Iterable[HasilAmbil]:
        yield self._ambil_rentang(
            klien, tanggal - timedelta(days=JENDELA_AWAL_HARI), tanggal - timedelta(days=JENDELA_AKHIR_HARI)
        )

    def ambil_backfill(
        self, ds: DefinisiDataset, klien: KlienSopan, dari: date, sampai: date
    ) -> Iterable[HasilAmbil]:
        for awal, akhir in potong_rentang(dari, sampai, POTONGAN_BACKFILL_HARI):
            hasil = self._ambil_rentang(klien, awal, akhir)
            yield hasil
            if hasil.dilewati or (hasil.permintaan and hasil.permintaan_gagal == hasil.permintaan):
                return  # Potongan berikutnya hampir pasti bernasib sama.
