"""BMKG: prakiraan cuaca adm4 per kab/kota Aceh.

Terverifikasi ke endpoint nyata 2026-11-24. Slot 3-jaman diringkas menjadi nilai harian; hanya hari
dengan 8 slot lengkap yang dipakai, seperti konektor Go. Nilainya PRAKIRAAN (`atribut.jenis_data`),
bukan observasi, dan `periode` bisa berada di masa depan relatif terhadap `crawl_date`.

BMKG tidak menyediakan histori, sehingga backfill iklim memakai OPEN_METEO. Peringatan cuaca dilewati:
robots.txt www.bmkg.go.id melarang /alerts dan /api.
"""

from collections import defaultdict
from collections.abc import Iterable
from datetime import date, datetime
from typing import Any
from urllib.parse import urlencode

from garda_crawler.http_sopan import GalatAmbil, GalatRobots, KlienSopan
from garda_crawler.model import DefinisiDataset, HasilAmbil, Seri, Titik, pesan_dilewati
from garda_crawler.referensi import BMKG_WILAYAH
from garda_crawler.sumber.dasar import Adapter, TidakMendukungBackfill
from garda_crawler.validasi import iklim_sah

URL_PRAKIRAAN = "https://api.bmkg.go.id/publik/prakiraan-cuaca"
DATASET_PRAKIRAAN = DefinisiDataset("prakiraan_cuaca_adm4", "raw_iklim_bmkg")
DATASET_PERINGATAN = DefinisiDataset("peringatan_cuaca", "raw_iklim_bmkg")
SLOT_PER_HARI = 8
ALASAN_PERINGATAN = pesan_dilewati(
    "robots.txt www.bmkg.go.id melarang /alerts dan /api (diperiksa 2026-11-24)",
    "minta akses resmi data peringatan cuaca ke BMKG",
)

INDIKATOR = {
    "curah_hujan": ("mm", "curah_hujan_harian"),
    "suhu": ("celsius", "suhu_rata_harian"),
    "kelembapan": ("persen", "kelembapan_rata_harian"),
}


def seri_bmkg(indikator: str, kabupaten_kode: str) -> Seri:
    satuan, nama = INDIKATOR[indikator]
    return Seri(
        sumber="BMKG",
        dataset=DATASET_PRAKIRAAN.nama,
        koleksi=DATASET_PRAKIRAAN.koleksi,
        kunci_seri=f"{indikator}:{kabupaten_kode}",
        indikator=nama,
        satuan=satuan,
        frekuensi="HARIAN",
        kabupaten_kode=kabupaten_kode,
        url_sumber=URL_PRAKIRAAN,
    )


def _angka(mentah: Any) -> float | None:
    if isinstance(mentah, bool) or not isinstance(mentah, (int, float)):
        return None
    return float(mentah)


def urai_prakiraan(muatan: Any, adm4: str, kabupaten_kode: str, tanggal_crawl: date, url: str) -> HasilAmbil:
    if not isinstance(muatan, dict) or not isinstance(muatan.get("data"), list) or not muatan["data"]:
        raise GalatAmbil("respons BMKG tanpa data prakiraan")

    per_hari: dict[date, list[dict[str, Any]]] = defaultdict(list)
    hasil = HasilAmbil()
    for blok in muatan["data"]:
        for harian in blok.get("cuaca") or []:
            for slot in harian:
                try:
                    hari = datetime.strptime(str(slot.get("local_datetime"))[:10], "%Y-%m-%d").date()
                except ValueError:
                    hasil.ditolak["slot_tanpa_tanggal"] += 1
                    continue
                per_hari[hari].append(slot)

    for hari, slot_hari in sorted(per_hari.items()):
        if len(slot_hari) != SLOT_PER_HARI:
            hasil.ditolak["hari_tidak_lengkap"] += 1
            continue
        curah = [_angka(s.get("tp")) for s in slot_hari]
        suhu = [_angka(s.get("t")) for s in slot_hari]
        lembap = [_angka(s.get("hu")) for s in slot_hari]
        if any(v is None for v in (*curah, *suhu, *lembap)):
            hasil.ditolak["slot_tanpa_nilai"] += 1
            continue
        nilai = {
            "curah_hujan": round(sum(curah), 1),
            "suhu": round(sum(suhu) / SLOT_PER_HARI, 1),
            "kelembapan": round(sum(lembap) / SLOT_PER_HARI, 1),
        }
        atribut = {
            "jenis_data": "PRAKIRAAN",
            "adm4": adm4,
            "horizon_hari": (hari - tanggal_crawl).days,
            "analysis_date": slot_hari[0].get("analysis_date"),
            "jumlah_slot": SLOT_PER_HARI,
        }
        for indikator, angka in nilai.items():
            if not iklim_sah(indikator, angka):
                hasil.ditolak[f"{indikator}_tidak_wajar"] += 1
                continue
            hasil.titik.append(
                Titik(seri_bmkg(indikator, kabupaten_kode), hari, float(angka), url, dict(atribut))
            )
    return hasil


class AdapterBMKG(Adapter):
    sumber = "BMKG"

    def daftar_dataset(self) -> list[DefinisiDataset]:
        return [DATASET_PRAKIRAAN, DATASET_PERINGATAN]

    def _wilayah(self) -> list[tuple[str, str]]:
        return [(a, k) for a, k in BMKG_WILAYAH.items() if self.termasuk(k)]

    def seri_diharapkan(self, ds: DefinisiDataset) -> list[Seri]:
        if ds != DATASET_PRAKIRAAN:
            return []
        return [seri_bmkg(i, kab) for _, kab in self._wilayah() for i in INDIKATOR]

    def ambil_harian(self, ds: DefinisiDataset, klien: KlienSopan, tanggal: date) -> Iterable[HasilAmbil]:
        if ds == DATASET_PERINGATAN:
            yield HasilAmbil(dilewati=ALASAN_PERINGATAN)
            return

        gabungan = HasilAmbil()
        for adm4, kab in self._wilayah():
            gabungan.permintaan += 1
            params = {"adm4": adm4}
            try:
                muatan = klien.get_json(URL_PRAKIRAAN, params)
                hasil = urai_prakiraan(muatan, adm4, kab, tanggal, f"{URL_PRAKIRAAN}?{urlencode(params)}")
            except GalatRobots as exc:
                gabungan.permintaan -= 1  # Diblokir sebelum permintaan data terkirim.
                gabungan.dilewati = pesan_dilewati(str(exc), "minta izin akses API prakiraan ke BMKG")
                gabungan.seri_gagal.extend(self.seri_diharapkan(ds))
                break
            except GalatAmbil as exc:
                gabungan.permintaan_gagal += 1
                gabungan.galat.append(f"{kab}: {exc}")
                gabungan.seri_gagal.extend(seri_bmkg(i, kab) for i in INDIKATOR)
                continue
            gabungan.titik.extend(hasil.titik)
            gabungan.ditolak.update(hasil.ditolak)
        yield gabungan

    def ambil_backfill(
        self, ds: DefinisiDataset, klien: KlienSopan, dari: date, sampai: date
    ) -> Iterable[HasilAmbil]:
        raise TidakMendukungBackfill(
            "BMKG tidak menyediakan histori; tindakan: gunakan --sumber OPEN_METEO untuk backfill iklim"
        )
