"""BPS WebAPI resmi (webapi.bps.go.id). Wajib BPS_API_KEY; tanpa kunci dataset berstatus ERROR.

BELUM TERVERIFIKASI KE LAYANAN NYATA karena repo belum punya kunci. Bentuk respons mengikuti fixture
konektor Go, yang juga ditulis dari dokumentasi. Id variabel tidak ditebak: diambil dari
BPS_VARIABEL_JSON, atau dicari lewat daftar variabel domain dengan pola judul dan harus tepat satu cocok.
Hasil ambigu berakhir sebagai galat yang menyebut kandidatnya.

Halaman tabel bps.go.id sengaja tidak di-scrape.
"""

import re
from collections.abc import Collection, Iterable, Mapping
from dataclasses import dataclass
from datetime import date
from typing import Any

from garda_crawler.http_sopan import GalatAmbil, GalatRobots, KlienSopan
from garda_crawler.model import DefinisiDataset, HasilAmbil, Seri, Titik, pesan_dilewati
from garda_crawler.referensi import BPS_DOMAIN_ACEH, BPS_WILAYAH, KABUPATEN, PROVINSI_KODE, normalisasi_alias
from garda_crawler.sumber.dasar import Adapter

URL_DASAR = "https://webapi.bps.go.id/v1/api"
TAHUN_DASAR_BPS = 1900
HALAMAN_MAKS = 50
PESAN_TANPA_KUNCI = pesan_dilewati(
    "BPS_API_KEY belum diisi, halaman tabel BPS tidak di-scrape sebagai gantinya",
    "isi BPS_API_KEY (daftar di webapi.bps.go.id)",
)
TINDAKAN_ROBOTS = "minta izin akses WebAPI ke BPS"

BULAN = {
    "januari": 1, "februari": 2, "maret": 3, "april": 4, "mei": 5, "juni": 6, "juli": 7, "agustus": 8,
    "september": 9, "oktober": 10, "november": 11, "desember": 12,
}
TRIWULAN = {"i": 1, "ii": 4, "iii": 7, "iv": 10, "1": 1, "2": 4, "3": 7, "4": 10}
_WILAYAH_IHK = (PROVINSI_KODE, "1171", "1174", "1107")
_WILAYAH_SEMUA = (PROVINSI_KODE, *KABUPATEN)
_WILAYAH_PROVINSI = (PROVINSI_KODE,)


@dataclass(frozen=True)
class Indikator:
    kunci: str
    nama: str
    satuan: str
    frekuensi: str
    pola_judul: str
    wilayah: tuple[str, ...]


DATASET_BPS: dict[DefinisiDataset, tuple[Indikator, ...]] = {
    DefinisiDataset("bps_inflasi", "raw_bps_makro"): (
        Indikator("inflasi_yoy", "inflasi_year_on_year", "persen", "BULANAN", r"inflasi.*(y-?o-?y|year on year|tahun ke tahun)", _WILAYAH_IHK),
        Indikator("inflasi_mtm", "inflasi_month_to_month", "persen", "BULANAN", r"inflasi.*(m-?t-?m|month to month|bulan ke bulan)", _WILAYAH_IHK),
        Indikator("inflasi_ytd", "inflasi_year_to_date", "persen", "BULANAN", r"inflasi.*(y-?t-?d|year to date|tahun kalender)", _WILAYAH_IHK),
        Indikator("ihk", "indeks_harga_konsumen", "indeks", "BULANAN", r"^indeks harga konsumen\b", _WILAYAH_IHK),
    ),
    DefinisiDataset("bps_konsumsi_pendapatan", "raw_konsumsi_pendapatan"): (
        Indikator("pengeluaran_per_kapita", "pengeluaran_per_kapita_sebulan", "Rp/kapita/bulan", "TAHUNAN", r"rata-?rata pengeluaran per kapita", _WILAYAH_SEMUA),
        Indikator("pengeluaran_makanan", "persentase_pengeluaran_makanan", "persen", "TAHUNAN", r"persentase pengeluaran.*makanan", _WILAYAH_SEMUA),
    ),
    DefinisiDataset("bps_neraca", "raw_neraca_ekonomi"): (
        Indikator("pdrb_adhb", "pdrb_atas_dasar_harga_berlaku", "juta Rp", "TAHUNAN", r"produk domestik regional bruto.*harga berlaku", _WILAYAH_SEMUA),
        Indikator("pdrb_adhk", "pdrb_atas_dasar_harga_konstan", "juta Rp", "TAHUNAN", r"produk domestik regional bruto.*harga konstan", _WILAYAH_SEMUA),
        Indikator("pertumbuhan_ekonomi", "laju_pertumbuhan_pdrb", "persen", "TAHUNAN", r"laju pertumbuhan.*(pdrb|produk domestik)", _WILAYAH_SEMUA),
    ),
    DefinisiDataset("bps_tenaga_kerja", "raw_tenaga_kerja"): (
        Indikator("tpt", "tingkat_pengangguran_terbuka", "persen", "TAHUNAN", r"tingkat pengangguran terbuka", _WILAYAH_SEMUA),
        Indikator("tpak", "tingkat_partisipasi_angkatan_kerja", "persen", "TAHUNAN", r"tingkat partisipasi angkatan kerja", _WILAYAH_SEMUA),
        Indikator("upah_buruh", "rata_rata_upah_buruh", "Rp/bulan", "TAHUNAN", r"rata-?rata upah.*buruh", _WILAYAH_PROVINSI),
    ),
    DefinisiDataset("bps_pertanian", "raw_sektor_primer"): (
        Indikator("ntp", "nilai_tukar_petani", "indeks", "BULANAN", r"^nilai tukar petani\b", _WILAYAH_PROVINSI),
        Indikator("produksi_padi", "produksi_padi", "ton", "TAHUNAN", r"produksi padi", _WILAYAH_SEMUA),
    ),
    DefinisiDataset("bps_kehutanan", "raw_sektor_primer"): (
        Indikator("produksi_kayu", "produksi_kayu_bulat", "m3", "TAHUNAN", r"produksi kayu", _WILAYAH_PROVINSI),
    ),
    DefinisiDataset("bps_perikanan", "raw_sektor_primer"): (
        Indikator("produksi_perikanan", "produksi_perikanan", "ton", "TAHUNAN", r"produksi perikanan", _WILAYAH_SEMUA),
    ),
}


def seri_bps(ds: DefinisiDataset, ind: Indikator, wilayah: str) -> Seri:
    return Seri(
        sumber="BPS",
        dataset=ds.nama,
        koleksi=ds.koleksi,
        kunci_seri=f"{ind.kunci}:{wilayah}",
        indikator=ind.nama,
        satuan=ind.satuan,
        frekuensi=ind.frekuensi,
        kabupaten_kode=wilayah,
        url_sumber=URL_DASAR,
    )


def url_data(domain: str, var_id: str, th_id: int, kunci: str) -> str:
    return f"{URL_DASAR}/list/model/data/lang/ind/domain/{domain}/var/{var_id}/th/{th_id}/key/{kunci}"


def url_tanpa_kunci(url: str) -> str:
    return re.sub(r"/key/[^/?]+", "", url)


def periode_turtahun(tahun: int, label: str, val: Any, frekuensi: str) -> date | None:
    teks = normalisasi_alias(label)
    if frekuensi == "BULANAN":
        if teks in BULAN:
            return date(tahun, BULAN[teks], 1)
        if "tahun" in teks:
            return None
        try:
            bulan = int(str(val))
        except ValueError:
            return None
        return date(tahun, bulan, 1) if 1 <= bulan <= 12 else None
    if frekuensi == "TRIWULANAN":
        cocok = re.search(r"triwulan\s+(iv|iii|ii|i|[1-4])\b", teks)
        return date(tahun, TRIWULAN[cocok.group(1)], 1) if cocok else None
    if teks in BULAN or "triwulan" in teks or "semester" in teks:
        return None
    return date(tahun, 1, 1)


def _pilih_turvar(turvar: list[dict[str, Any]]) -> dict[str, Any] | None:
    """Memilih kategori total; tanpa total, deret dianggap ambigu dan tidak ditebak."""
    if not turvar:
        return {"val": 0, "label": "Tidak Ada"}
    if len(turvar) == 1:
        return turvar[0]
    for t in turvar:
        if re.search(r"\b(jumlah|total|tidak ada)\b", str(t.get("label", "")).lower()):
            return t
    return None


def urai_data(muatan: Any, ds: DefinisiDataset, ind: Indikator, url_aman: str) -> HasilAmbil:
    if not isinstance(muatan, dict):
        raise GalatAmbil("respons BPS tidak terbaca")
    status = str(muatan.get("status", "OK"))
    if status.upper() != "OK":
        raise GalatAmbil(f"BPS menjawab status {status!r}")
    hasil = HasilAmbil()
    if "not-available" in str(muatan.get("data-availability", "")):
        return hasil  # Tahun ini belum ada rilis; bukan galat.

    daftar_var = muatan.get("var") or []
    if len(daftar_var) != 1:
        raise GalatAmbil(f"respons BPS harus memuat tepat satu variabel, ada {len(daftar_var)}")
    turvar = _pilih_turvar(muatan.get("turvar") or [])
    if turvar is None:
        hasil.ditolak["turvar_ambigu"] += 1
        return hasil

    isi = muatan.get("datacontent") or {}
    v = daftar_var[0]["val"]
    for th in muatan.get("tahun") or []:
        try:
            tahun = int(str(th.get("label")).strip())
        except ValueError:
            hasil.ditolak["tahun_rusak"] += 1
            continue
        for tt in muatan.get("turtahun") or []:
            periode = periode_turtahun(tahun, str(tt.get("label", "")), tt.get("val"), ind.frekuensi)
            if periode is None:
                continue
            for vv in muatan.get("vervar") or []:
                kunci = f"{vv['val']}{v}{turvar['val']}{th['val']}{tt['val']}"
                if kunci not in isi:
                    continue
                wilayah = BPS_WILAYAH.get(normalisasi_alias(str(vv.get("label", ""))))
                if wilayah is None:
                    hasil.ditolak["wilayah_tak_terpetakan"] += 1
                    continue
                try:
                    nilai = float(str(isi[kunci]).strip())
                except ValueError:
                    hasil.ditolak["nilai_bukan_angka"] += 1
                    continue
                if not _dalam_batas(ind, nilai):
                    hasil.ditolak["nilai_di_luar_batas"] += 1
                    continue
                hasil.titik.append(
                    Titik(
                        seri_bps(ds, ind, wilayah),
                        periode,
                        nilai,
                        url_aman,
                        {"var_id": str(v), "label_vervar": vv.get("label"), "turtahun": tt.get("label")},
                    )
                )
    return hasil


def _dalam_batas(ind: Indikator, nilai: float) -> bool:
    if ind.satuan == "persen":
        return abs(nilai) <= 100
    if ind.satuan == "indeks":
        return 0 < nilai < 1000
    return nilai >= 0


def cari_var_id(klien: KlienSopan, kunci_api: str, domain: str, ind: Indikator, cache: dict[str, list[dict]]) -> str:
    if domain not in cache:
        semua: list[dict] = []
        for halaman in range(1, HALAMAN_MAKS + 1):
            url = f"{URL_DASAR}/list/model/var/lang/ind/domain/{domain}/page/{halaman}/key/{kunci_api}"
            muatan = klien.get_json(url)
            # Kunci yang ditolak dijawab 200 berstatus Error; jangan dilaporkan sebagai "tidak ditemukan".
            if isinstance(muatan, dict) and str(muatan.get("status", "OK")).upper() != "OK":
                raise GalatAmbil(f"BPS menjawab status {muatan.get('status')!r} saat membaca daftar variabel")
            data = muatan.get("data") if isinstance(muatan, dict) else None
            if not isinstance(data, list) or len(data) < 2:
                break
            semua.extend(data[1])
            if halaman >= int(data[0].get("pages", 1)):
                break
        cache[domain] = semua
    cocok = [v for v in cache[domain] if re.search(ind.pola_judul, str(v.get("title", "")), re.IGNORECASE)]
    if len(cocok) == 1:
        return str(cocok[0]["var_id"])
    if not cocok:
        raise GalatAmbil(f"variabel {ind.kunci} tidak ditemukan; isi BPS_VARIABEL_JSON")
    kandidat = ", ".join(str(v.get("var_id")) for v in cocok[:8])
    raise GalatAmbil(f"variabel {ind.kunci} ambigu (kandidat var_id: {kandidat}); isi BPS_VARIABEL_JSON")


class AdapterBPS(Adapter):
    sumber = "BPS"

    def __init__(
        self,
        kunci_api: str,
        variabel: Mapping[str, str] | None = None,
        wilayah: Collection[str] | None = None,
    ) -> None:
        super().__init__(wilayah)
        self.kunci_api = kunci_api
        self.variabel = dict(variabel or {})
        self._cache_var: dict[str, list[dict]] = {}

    def daftar_dataset(self) -> list[DefinisiDataset]:
        return list(DATASET_BPS)

    def seri_diharapkan(self, ds: DefinisiDataset) -> list[Seri]:
        return [seri_bps(ds, ind, w) for ind in DATASET_BPS[ds] for w in ind.wilayah if self.termasuk(w)]

    def _ambil_tahun(self, ds: DefinisiDataset, klien: KlienSopan, tahun_awal: int, tahun_akhir: int) -> HasilAmbil:
        gabungan = HasilAmbil()
        if not self.kunci_api:
            gabungan.dilewati = PESAN_TANPA_KUNCI
            gabungan.seri_gagal.extend(self.seri_diharapkan(ds))
            return gabungan

        for ind in DATASET_BPS[ds]:
            seri_ind = [seri_bps(ds, ind, w) for w in ind.wilayah if self.termasuk(w)]
            try:
                var_id = self.variabel.get(ind.kunci) or cari_var_id(
                    klien, self.kunci_api, BPS_DOMAIN_ACEH, ind, self._cache_var
                )
            except GalatRobots as exc:
                gabungan.dilewati = pesan_dilewati(klien.samarkan(str(exc)), TINDAKAN_ROBOTS)
                gabungan.seri_gagal.extend(self.seri_diharapkan(ds))
                return gabungan
            except GalatAmbil as exc:
                gabungan.permintaan += 1
                gabungan.permintaan_gagal += 1
                gabungan.galat.append(klien.samarkan(f"{ind.kunci}: {exc}"))
                gabungan.seri_gagal.extend(seri_ind)
                continue

            berhasil = False
            for tahun in range(tahun_awal, tahun_akhir + 1):
                gabungan.permintaan += 1
                url = url_data(BPS_DOMAIN_ACEH, var_id, tahun - TAHUN_DASAR_BPS, self.kunci_api)
                try:
                    muatan = klien.get_json(url)
                    hasil = urai_data(muatan, ds, ind, url_tanpa_kunci(url))
                except GalatRobots as exc:
                    gabungan.permintaan -= 1  # Diblokir sebelum permintaan data terkirim.
                    gabungan.dilewati = pesan_dilewati(klien.samarkan(str(exc)), TINDAKAN_ROBOTS)
                    gabungan.seri_gagal.extend(self.seri_diharapkan(ds))
                    return gabungan
                except GalatAmbil as exc:
                    gabungan.permintaan_gagal += 1
                    gabungan.galat.append(klien.samarkan(f"{ind.kunci} {tahun}: {exc}"))
                    continue
                berhasil = True
                gabungan.titik.extend(t for t in hasil.titik if self.termasuk(t.seri.kabupaten_kode or ""))
                gabungan.ditolak.update(hasil.ditolak)
            if not berhasil:
                gabungan.seri_gagal.extend(seri_ind)
        return gabungan

    def ambil_harian(self, ds: DefinisiDataset, klien: KlienSopan, tanggal: date) -> Iterable[HasilAmbil]:
        yield self._ambil_tahun(ds, klien, tanggal.year - 1, tanggal.year)

    def ambil_backfill(
        self, ds: DefinisiDataset, klien: KlienSopan, dari: date, sampai: date
    ) -> Iterable[HasilAmbil]:
        yield self._ambil_tahun(ds, klien, dari.year, sampai.year)
