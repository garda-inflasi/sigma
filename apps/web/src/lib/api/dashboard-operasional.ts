/**
 * Ringkasan operasional 08-B dari API Go (snake_case) ke model UI: aktivitas per petugas, stok, produsen, dan
 * logistik. Endpoint: GET /v1/dashboard/operasional/{aktivitas-petugas,stok,produsen,logistik}.
 *
 * Aturan bersama semuanya (dijaga server, tes kontrak, dan fixture nyata di __fixtures__):
 * - larik tidak pernah null (`[]` bila kosong); blok opsional berbentuk objek atau null;
 * - tanpa data hasilnya nol atau `[]`, bukan galat;
 * - waktu berformat RFC 3339 dengan zona Aceh (`+07:00`), tanggal YYYY-MM-DD menurut waktu Aceh;
 * - Koordinator dan Agregator hanya mendapat kabupaten tugasnya, peran tingkat provinsi ke atas seluruh Aceh;
 * - tidak ada data pribadi selain nama tampilan petugas (tanpa email atau telepon).
 *
 * Belum ada kartu UI untuk stok, produsen, dan logistik (todo 08-B): klien, pemeta, hook, dan mock sudah siap.
 */

import { namaRingkas } from "./dashboard";

// ── Aktivitas per petugas ─────────────────────────────────────────────

export interface AktivitasPetugasItemResponse {
  /** UUID pengguna, stabil. */
  id: string;
  /** Nama tampilan saja. */
  nama: string;
  /** Kabupaten tugas petugas. */
  kabupaten_kode: string;
  /** Nama resmi, mis. "KOTA BANDA ACEH". */
  kabupaten_nama: string;
  /** Entri harga KONSUMEN (selain DRAF) yang dicatat petugas hari ini, di pasar mana pun. */
  entri_hari_ini: number;
  /** Jumlah komoditas aktif: tiap petugas diharapkan mencatat tiap komoditas sekali sehari. */
  target_harian: number;
  /** Entri petugas berstatus DIAJUKAN saat ini, seusia apa pun. */
  menunggu_validasi: number;
  /** Entri petugas berstatus DITOLAK yang dicatat dalam `jendela_hari` hari terakhir. */
  ditolak_jendela: number;
  /** RFC 3339 (+07:00); null bila petugas belum pernah mencatat entri. */
  entri_terakhir_pada: string | null;
  /** Tugas belum selesai dan belum lewat tenggat. */
  tugas_aktif: number;
  tugas_selesai: number;
  /** Tugas belum selesai yang lewat tenggat. */
  tugas_terlambat: number;
}

export interface AktivitasPetugasResponse {
  /** Hari ini menurut waktu Aceh (YYYY-MM-DD). */
  tanggal: string;
  /** Panjang jendela (hari, hari ini termasuk) untuk `ditolak_jendela` dan tugas. */
  jendela_hari: number;
  /** Urut menurut nama kabupaten, nama petugas, lalu ID. */
  petugas: AktivitasPetugasItemResponse[];
}

/** Aktivitas satu petugas lapangan hari ini. */
export interface PetugasAktivitas {
  id: string;
  nama: string;
  kabupatenKode: string;
  /** Nama ringkas ("Banda Aceh"). */
  kabupatenNama: string;
  entriHariIni: number;
  targetHarian: number;
  menungguValidasi: number;
  /** Ditolak dalam `jendelaHari` hari terakhir (lihat AktivitasPetugas). */
  ditolakJendela: number;
  entriTerakhirPada: string | null;
  tugasAktif: number;
  tugasSelesai: number;
  tugasTerlambat: number;
}

export interface AktivitasPetugas {
  tanggal: string;
  jendelaHari: number;
  petugas: PetugasAktivitas[];
}

export function toAktivitasPetugas(
  r: AktivitasPetugasResponse,
): AktivitasPetugas {
  return {
    tanggal: r.tanggal,
    jendelaHari: r.jendela_hari,
    petugas: r.petugas.map((p) => ({
      id: p.id,
      nama: p.nama,
      kabupatenKode: p.kabupaten_kode,
      kabupatenNama: namaRingkas(p.kabupaten_kode, p.kabupaten_nama),
      entriHariIni: p.entri_hari_ini,
      targetHarian: p.target_harian,
      menungguValidasi: p.menunggu_validasi,
      ditolakJendela: p.ditolak_jendela,
      entriTerakhirPada: p.entri_terakhir_pada,
      tugasAktif: p.tugas_aktif,
      tugasSelesai: p.tugas_selesai,
      tugasTerlambat: p.tugas_terlambat,
    })),
  };
}

// ── Stok ──────────────────────────────────────────────────────────────

export interface StokRingkasanResponse {
  /** Stok berlaku: lot belum HABIS di fasilitas OPERASIONAL, masuk <= hari ini < kedaluwarsa (satu desimal). */
  total_ton: number;
  jumlah_lot: number;
  /** Fasilitas berbeda yang menyimpan stok berlaku. */
  jumlah_fasilitas: number;
  /** Kapasitas seluruh fasilitas OPERASIONAL dalam cakupan; tidak dipengaruhi filter komoditas. */
  kapasitas_ton: number;
  fasilitas_operasional: number;
  /** 100 x total_ton / kapasitas_ton, satu desimal, 0 tanpa kapasitas. Sama dengan kartu KPI `cs`. */
  utilisasi_pct: number;
  /** Bagian stok berlaku yang kedaluwarsa paling lambat `ambang_kedaluwarsa_hari` hari lagi. */
  mendekati_kedaluwarsa_ton: number;
  /** Bagian stok berlaku dengan risiko food loss di atas `ambang_risiko`. */
  risiko_tinggi_ton: number;
  /** Lot di fasilitas OPERASIONAL yang sudah kedaluwarsa tetapi belum dikeluarkan. */
  kedaluwarsa_lot: number;
  kedaluwarsa_ton: number;
  /** Lot di fasilitas yang tidak OPERASIONAL (perawatan). */
  di_luar_operasional_ton: number;
}

export interface StokKomoditasResponse {
  komoditas_kode: string;
  komoditas_nama: string;
  total_ton: number;
  jumlah_lot: number;
  jumlah_fasilitas: number;
  mendekati_kedaluwarsa_ton: number;
  risiko_tinggi_ton: number;
}

export interface StokResponse {
  /** Hari ini menurut waktu Aceh (acuan lot berlaku). */
  tanggal: string;
  ambang_kedaluwarsa_hari: number;
  /** Pecahan 0-1: lot berisiko tinggi bila risikonya di ATAS nilai ini. */
  ambang_risiko: number;
  ringkasan: StokRingkasanResponse;
  /** Hanya komoditas yang punya stok berlaku, terbesar lebih dulu. */
  komoditas: StokKomoditasResponse[];
}

export interface StokRingkasan {
  /** Stok berlaku di fasilitas OPERASIONAL (ton). */
  totalTon: number;
  jumlahLot: number;
  jumlahFasilitas: number;
  kapasitasTon: number;
  fasilitasOperasional: number;
  /** Persen terisi; angka yang sama dengan kartu KPI `cs`. */
  utilisasiPct: number;
  mendekatiKedaluwarsaTon: number;
  risikoTinggiTon: number;
  kedaluwarsaLot: number;
  kedaluwarsaTon: number;
  diLuarOperasionalTon: number;
}

export interface StokKomoditas {
  komoditasKode: string;
  komoditasNama: string;
  totalTon: number;
  jumlahLot: number;
  jumlahFasilitas: number;
  mendekatiKedaluwarsaTon: number;
  risikoTinggiTon: number;
}

export interface StokOperasional {
  tanggal: string;
  ambangKedaluwarsaHari: number;
  /** Pecahan 0-1. */
  ambangRisiko: number;
  ringkasan: StokRingkasan;
  komoditas: StokKomoditas[];
}

export function toStok(r: StokResponse): StokOperasional {
  const s = r.ringkasan;
  return {
    tanggal: r.tanggal,
    ambangKedaluwarsaHari: r.ambang_kedaluwarsa_hari,
    ambangRisiko: r.ambang_risiko,
    ringkasan: {
      totalTon: s.total_ton,
      jumlahLot: s.jumlah_lot,
      jumlahFasilitas: s.jumlah_fasilitas,
      kapasitasTon: s.kapasitas_ton,
      fasilitasOperasional: s.fasilitas_operasional,
      utilisasiPct: s.utilisasi_pct,
      mendekatiKedaluwarsaTon: s.mendekati_kedaluwarsa_ton,
      risikoTinggiTon: s.risiko_tinggi_ton,
      kedaluwarsaLot: s.kedaluwarsa_lot,
      kedaluwarsaTon: s.kedaluwarsa_ton,
      diLuarOperasionalTon: s.di_luar_operasional_ton,
    },
    komoditas: r.komoditas.map((k) => ({
      komoditasKode: k.komoditas_kode,
      komoditasNama: k.komoditas_nama,
      totalTon: k.total_ton,
      jumlahLot: k.jumlah_lot,
      jumlahFasilitas: k.jumlah_fasilitas,
      mendekatiKedaluwarsaTon: k.mendekati_kedaluwarsa_ton,
      risikoTinggiTon: k.risiko_tinggi_ton,
    })),
  };
}

// ── Produsen ──────────────────────────────────────────────────────────

/** Harga rata-rata satu sisi pada jendela, tertimbang jumlah entri, rupiah penuh per satuan komoditas. */
export interface SisiHargaResponse {
  harga: number;
  jumlah_entri: number;
  /** Kabupaten yang melapor (konsumen: hanya yang juga punya harga produsen). */
  jumlah_kabupaten: number;
}

/** Hanya ada bila harga produsen dan konsumen sama-sama ada; dihitung server, jangan dihitung ulang di web. */
export interface MarginResponse {
  /** Harga konsumen dikurangi harga produsen (rupiah per satuan); negatif bila konsumen di bawah produsen. */
  selisih: number;
  /** Selisih terhadap harga produsen (markup), satu desimal. */
  persen_produsen: number;
  /** Selisih terhadap harga konsumen (definisi EKO-MARGIN), satu desimal. */
  persen_konsumen: number;
}

/** Produksi neraca pangan periode terakhir untuk satu wilayah. */
export interface ProduksiResponse {
  ton: number;
  /** Tanggal 1 bulan neraca (YYYY-MM-DD). */
  periode: string;
  sumber: "BPS" | "BI" | "DJPB" | "BAPANAS" | "DEMO";
  /** true bila angkanya contoh (sumber DEMO), bukan data resmi. */
  demo: boolean;
}

export interface ProdusenKomoditasResponse {
  komoditas_kode: string;
  komoditas_nama: string;
  satuan: string;
  produsen: SisiHargaResponse | null;
  konsumen: SisiHargaResponse | null;
  margin: MarginResponse | null;
  produksi: ProduksiResponse | null;
}

export interface ProdusenResponse {
  /** Awal jendela harga (tanggal Aceh, inklusif). */
  dari: string;
  /** Akhir jendela, hari ini (tanggal Aceh, inklusif). */
  sampai: string;
  /** Komoditas yang punya harga produsen atau produksi; pilot lebih dulu, lalu menurut nama. */
  komoditas: ProdusenKomoditasResponse[];
}

export interface SisiHarga {
  harga: number;
  jumlahEntri: number;
  jumlahKabupaten: number;
}

export interface MarginHarga {
  selisih: number;
  persenProdusen: number;
  persenKonsumen: number;
}

export interface ProduksiPangan {
  ton: number;
  periode: string;
  sumber: ProduksiResponse["sumber"];
  /** true bila angka contoh: tampilkan sebagai data contoh. */
  demo: boolean;
}

export interface ProdusenKomoditas {
  komoditasKode: string;
  komoditasNama: string;
  satuan: string;
  produsen: SisiHarga | null;
  konsumen: SisiHarga | null;
  margin: MarginHarga | null;
  produksi: ProduksiPangan | null;
}

export interface ProdusenOperasional {
  dari: string;
  sampai: string;
  komoditas: ProdusenKomoditas[];
}

const toSisiHarga = (s: SisiHargaResponse | null): SisiHarga | null =>
  s && {
    harga: s.harga,
    jumlahEntri: s.jumlah_entri,
    jumlahKabupaten: s.jumlah_kabupaten,
  };

export function toProdusen(r: ProdusenResponse): ProdusenOperasional {
  return {
    dari: r.dari,
    sampai: r.sampai,
    komoditas: r.komoditas.map((k) => ({
      komoditasKode: k.komoditas_kode,
      komoditasNama: k.komoditas_nama,
      satuan: k.satuan,
      produsen: toSisiHarga(k.produsen),
      konsumen: toSisiHarga(k.konsumen),
      // Margin datang dari server: tidak dihitung dari kedua sisi harga di web.
      margin: k.margin && {
        selisih: k.margin.selisih,
        persenProdusen: k.margin.persen_produsen,
        persenKonsumen: k.margin.persen_konsumen,
      },
      produksi: k.produksi && {
        ton: k.produksi.ton,
        periode: k.produksi.periode,
        sumber: k.produksi.sumber,
        demo: k.produksi.demo,
      },
    })),
  };
}

// ── Logistik ──────────────────────────────────────────────────────────

export interface GerakJenisResponse {
  jumlah: number;
  volume_ton: number;
}

/** Ketiga jenis selalu ada (nol bila tak ada transaksi). Satu PINDAH dihitung sekali sebagai pindah. */
export interface GerakRingkasanResponse {
  masuk: GerakJenisResponse;
  keluar: GerakJenisResponse;
  pindah: GerakJenisResponse;
}

export interface GerakKomoditasResponse {
  komoditas_kode: string;
  komoditas_nama: string;
  masuk: GerakJenisResponse;
  keluar: GerakJenisResponse;
  pindah: GerakJenisResponse;
}

/** Fasilitas cold storage pada sebuah pergerakan. */
export interface FasilitasLogistikResponse {
  /** UUID. */
  id: string;
  /** Kode tampilan (CS-01); boleh null. */
  kode: string | null;
  nama: string;
  kabupaten_kode: string;
  kabupaten_nama: string;
}

/** Satu transaksi stok. Tidak memuat pelaku maupun alasan. `asal` null untuk MASUK, `tujuan` null untuk KELUAR. */
export interface PergerakanResponse {
  /** ID BIGINT sebagai string desimal. */
  id: string;
  /** RFC 3339 (+07:00). */
  waktu: string;
  jenis: "MASUK" | "KELUAR" | "PINDAH";
  komoditas_kode: string;
  komoditas_nama: string;
  /** Ketelitian kilogram (3 desimal). */
  volume_ton: number;
  asal: FasilitasLogistikResponse | null;
  tujuan: FasilitasLogistikResponse | null;
}

/** Satu rute distribusi aktif; bentuknya sama dengan rute di modul spasial. Rute menghubungkan kabupaten. */
export interface RuteAktifResponse {
  id: string;
  kode: string | null;
  asal_kode: string;
  /** Nama resmi kabupaten asal. */
  asal: string;
  tujuan_kode: string;
  /** Nama resmi kabupaten tujuan. */
  tujuan: string;
  komoditas_kode: string;
  komoditas_nama: string;
  volume_ton: number;
  jarak_km: number;
  jam_tempuh: number;
  /** Rupiah. */
  biaya: number;
  status: string;
  sumber: string;
  dibuat_pada: string;
}

/** `jumlah`, `volume_ton`, dan `biaya` dihitung atas SEMUA rute aktif dalam cakupan; `items` hanya 10 terbesar biayanya. */
export interface RuteRingkasResponse {
  jumlah: number;
  volume_ton: number;
  biaya: number;
  items: RuteAktifResponse[];
}

export interface LogistikResponse {
  /** Awal jendela (tanggal Aceh, inklusif). */
  dari: string;
  /** Akhir jendela, hari ini. */
  sampai: string;
  ringkasan: GerakRingkasanResponse;
  /** Komoditas yang punya transaksi dalam jendela, tonase terbesar lebih dulu. */
  komoditas: GerakKomoditasResponse[];
  /** Paling banyak 10 transaksi terbaru dalam jendela. */
  terbaru: PergerakanResponse[];
  rute: RuteRingkasResponse;
}

export interface GerakJenis {
  jumlah: number;
  volumeTon: number;
}

export interface GerakRingkasan {
  masuk: GerakJenis;
  keluar: GerakJenis;
  pindah: GerakJenis;
}

export interface GerakKomoditas extends GerakRingkasan {
  komoditasKode: string;
  komoditasNama: string;
}

export interface FasilitasLogistik {
  id: string;
  kode: string | null;
  nama: string;
  kabupatenKode: string;
  /** Nama ringkas ("Aceh Besar"). */
  kabupatenNama: string;
}

export interface Pergerakan {
  id: string;
  waktu: string;
  jenis: PergerakanResponse["jenis"];
  komoditasKode: string;
  komoditasNama: string;
  volumeTon: number;
  asal: FasilitasLogistik | null;
  tujuan: FasilitasLogistik | null;
}

export interface RuteAktif {
  id: string;
  kode: string | null;
  asalKode: string;
  /** Nama ringkas kabupaten asal. */
  asalNama: string;
  tujuanKode: string;
  tujuanNama: string;
  komoditasKode: string;
  komoditasNama: string;
  volumeTon: number;
  jarakKm: number;
  jamTempuh: number;
  biaya: number;
  status: string;
  sumber: string;
  dibuatPada: string;
}

export interface RuteRingkas {
  jumlah: number;
  volumeTon: number;
  biaya: number;
  items: RuteAktif[];
}

export interface LogistikOperasional {
  dari: string;
  sampai: string;
  ringkasan: GerakRingkasan;
  komoditas: GerakKomoditas[];
  terbaru: Pergerakan[];
  rute: RuteRingkas;
}

const toGerakJenis = (g: GerakJenisResponse): GerakJenis => ({
  jumlah: g.jumlah,
  volumeTon: g.volume_ton,
});

const toGerakRingkasan = (g: GerakRingkasanResponse): GerakRingkasan => ({
  masuk: toGerakJenis(g.masuk),
  keluar: toGerakJenis(g.keluar),
  pindah: toGerakJenis(g.pindah),
});

const toFasilitasLogistik = (
  f: FasilitasLogistikResponse | null,
): FasilitasLogistik | null =>
  f && {
    id: f.id,
    kode: f.kode,
    nama: f.nama,
    kabupatenKode: f.kabupaten_kode,
    kabupatenNama: namaRingkas(f.kabupaten_kode, f.kabupaten_nama),
  };

export function toLogistik(r: LogistikResponse): LogistikOperasional {
  return {
    dari: r.dari,
    sampai: r.sampai,
    ringkasan: toGerakRingkasan(r.ringkasan),
    komoditas: r.komoditas.map((k) => ({
      komoditasKode: k.komoditas_kode,
      komoditasNama: k.komoditas_nama,
      ...toGerakRingkasan(k),
    })),
    terbaru: r.terbaru.map((p) => ({
      id: p.id,
      waktu: p.waktu,
      jenis: p.jenis,
      komoditasKode: p.komoditas_kode,
      komoditasNama: p.komoditas_nama,
      volumeTon: p.volume_ton,
      asal: toFasilitasLogistik(p.asal),
      tujuan: toFasilitasLogistik(p.tujuan),
    })),
    rute: {
      jumlah: r.rute.jumlah,
      volumeTon: r.rute.volume_ton,
      biaya: r.rute.biaya,
      items: r.rute.items.map((t) => ({
        id: t.id,
        kode: t.kode,
        asalKode: t.asal_kode,
        asalNama: namaRingkas(t.asal_kode, t.asal),
        tujuanKode: t.tujuan_kode,
        tujuanNama: namaRingkas(t.tujuan_kode, t.tujuan),
        komoditasKode: t.komoditas_kode,
        komoditasNama: t.komoditas_nama,
        volumeTon: t.volume_ton,
        jarakKm: t.jarak_km,
        jamTempuh: t.jam_tempuh,
        biaya: t.biaya,
        status: t.status,
        sumber: t.sumber,
        dibuatPada: t.dibuat_pada,
      })),
    },
  };
}
