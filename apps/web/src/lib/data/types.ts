import type { Severity, Siklus } from "@/lib/design/tokens";
import type { IkonKomoditas } from "./reference";

export type TipeHarga = "PRODUSEN" | "KONSUMEN";
export type Kualitas = "PREMIUM" | "MEDIUM" | "ASALAN";
export type StatusEntri = "DRAF" | "DIAJUKAN" | "DIVALIDASI" | "DIPUBLIKASI" | "DITOLAK";

export interface EntriHarga {
  id: string;
  komoditasKode: string;
  komoditasNama: string;
  pasarId: string;
  pasarNama: string;
  kabupatenKode: string;
  wilayah: string;
  tipe: TipeHarga;
  harga: number;
  satuan: string;
  kualitas: Kualitas;
  petugasId: string;
  petugasNama: string;
  dicatatPada: string;
  status: StatusEntri;
  deviasiPct: number; // terhadap median 7 hari
  skorAnomali: number; // 0..1
  adaFoto: boolean;
  dalamGeofence: boolean;
}

export type JenisEws = "LONJAKAN_HARGA" | "KELANGKAAN_STOK" | "ANOMALI_CUACA" | "OVER_SUPPLY" | "ANOMALI_DATA";
export type StatusEws = "BARU" | "DIAKUI" | "DIESKALASI" | "SELESAI";

export interface EwsAlert {
  id: string;
  severity: Severity;
  jenis: JenisEws;
  judul: string;
  komoditasKode?: string;
  komoditasNama?: string;
  kabupatenKode: string;
  wilayah: string;
  nilai: number;
  satuanNilai: "%" | "mm" | "ton";
  pesan: string;
  waktu: string;
  status: StatusEws;
  skorDampak: number;
  sumber: string;
}

export interface KpiOperasional {
  id: string;
  label: string;
  nilai: number;
  format: "angka" | "persen" | "rupiah";
  delta: number;
  keteranganDelta: string;
  tren: number[];
  tone: "data" | "normal" | "waspada" | "defisit" | "surplus";
  href: string;
}

export interface SelHeatmap {
  komoditasKode: string;
  kabupatenKode: string;
  delta: number;
  harga: number;
  /** LAPANGAN = entri tervalidasi; EKSTERNAL = hasil ingestion bagi sel yang belum punya data lapangan. */
  sumber: "LAPANGAN" | "EKSTERNAL";
}

export type Prioritas = "TINGGI" | "SEDANG" | "RENDAH";

export interface KartuIntervensi {
  id: string;
  siklus: Siklus;
  tahap: string;
  judul: string;
  komoditasKode: string;
  kabupatenKode: string;
  prioritas: Prioritas;
  penanggungJawab: { nama: string; inisial: string }[];
  tenggat: string;
  rekomendasiAi: boolean;
  progres: { selesai: number; total: number };
}

export interface StokColdStorage {
  id: string;
  fasilitasId: string;
  fasilitasNama: string;
  komoditasKode: string;
  komoditasNama: string;
  /** Ikon komoditas bila sudah diketahui; bila kosong ditelusuri dari `komoditasKode`. */
  ikon?: IkonKomoditas;
  volumeTon: number;
  masuk: string;
  sisaShelfLifeHari: number;
  /** Pecahan 0–1. */
  risikoFoodLoss: number;
  pemilik: string;
}

export interface FasilitasColdStorage {
  id: string;
  nama: string;
  wilayah: string;
  kabupatenKode: string;
  kapasitasTon: number;
  terisiTon: number;
  suhuTargetC: number;
  /** Kosong bila fasilitas belum memiliki telemetri (mis. berstatus rencana). */
  suhuAktualC: number | null;
  status: "OPERASIONAL" | "PERAWATAN" | "RENCANA";
  lat: number;
  lng: number;
}

/** Halaman stok yang sedang ditampilkan. */
export interface HalamanStok {
  page: number;
  limit: number;
  /** Jumlah lot yang cocok dengan filter (semua halaman). */
  total: number;
  totalPages: number;
  /** Total volume seluruh hasil filter (semua halaman). */
  volumeHasilTon: number;
}

/** Agregat seluruh stok; tidak terpengaruh filter tabel. */
export interface RingkasanStok {
  totalLot: number;
  totalVolumeTon: number;
  risikoTinggiLot: number;
  risikoTinggiTon: number;
  /** Lot dengan risiko di atas nilai ini (pecahan 0–1) dihitung berisiko tinggi. */
  ambangRisikoTinggi: number;
}

/** Data satu layar Cold storage & food loss. */
export interface ColdStorageView {
  fasilitas: FasilitasColdStorage[];
  stok: StokColdStorage[];
  halaman: HalamanStok;
  ringkasan: RingkasanStok;
}

export interface SumberData {
  id: string;
  nama: string;
  penyedia: string;
  jenis: "API" | "SCRAPER";
  cakupan: string;
  jadwal: string;
  /** Waktu eksekusi terakhir; null bila sumber belum pernah dijalankan. */
  terakhir: string | null;
  status: "BELUM_AKTIF" | "SEHAT" | "TERTUNDA" | "GAGAL" | "DIJEDA";
  /** Persen 0-100; null bila belum ada eksekusi untuk dihitung. */
  keberhasilan7h: number | null;
  latensiMs: number;
  /** Benar bila sumber punya konektor ingestion (tombol "Jalankan sekarang"). */
  bisaDijalankan: boolean;
}

/** Isi lapisan medallion dan mutu data (persen 0-100, null bila belum ada bahan untuk menghitung). */
export interface PipelineSumber {
  bronze: number;
  silver: number;
  gold: number;
  /** Benar bila hitungan memuat eksekusi contoh (data demo). */
  demo: boolean;
  mutu: {
    kelengkapan: number | null;
    akurasi: number | null;
    ketepatanWaktu: number | null;
    konsistensi: number | null;
  };
}

export type StatusEksekusi = "BERJALAN" | "SUKSES" | "SEBAGIAN" | "GAGAL";

/** Satu run ingestion sebuah sumber (riwayat eksekusi). */
export interface EksekusiSumber {
  id: number;
  pemicu: "JADWAL" | "MANUAL" | "CLI" | "DEMO";
  status: StatusEksekusi;
  mulai: string;
  selesai: string | null;
  durasiMs: number | null;
  latensiMs: number | null;
  dokumen: number;
  barisDiambil: number;
  barisDitolak: number;
  galat: string | null;
  /** Alias sumber yang belum dipetakan ke master, dengan jumlah baris. */
  takTerpetakan: { alias: string; baris: number }[];
  /** Alasan baris ditolak karena nilai tidak valid, dengan jumlah baris. */
  tidakValid: { alasan: string; baris: number }[];
}

/** Cuplikan payload mentah (bronze) yang terakhir diambil. */
export interface SampelSumber {
  id: number;
  kunci: string;
  diambilPada: string;
  ukuran: number;
  cuplikan: string;
}

export interface PenggunaRow {
  id: string;
  nama: string;
  email: string;
  role: import("@/lib/rbac/roles").Role;
  lembaga: string;
  wilayah: string;
  status: "AKTIF" | "NONAKTIF" | "MENUNGGU";
  mfa: boolean;
  terakhirMasuk: string;
}

export interface AuditRow {
  id: string;
  waktu: string;
  aktor: string;
  peran: string;
  aksi: string;
  entitas: string;
  entitasId: string;
  hasil: "BERHASIL" | "GAGAL" | "DITOLAK";
  ip: string;
}

export interface PolicyBrief {
  id: string;
  periode: string;
  judul: string;
  status: "DRAF" | "TELAAH" | "DISETUJUI" | "DIDISTRIBUSIKAN";
  disusunOleh: string;
  diperbarui: string;
  jumlahRekomendasi: number;
  halaman: number;
  /** Ada pada brief dari API; mode mock tidak mengisinya. */
  ringkasan?: string;
}

export interface TugasLapangan {
  id: string;
  judul: string;
  pasarId: string;
  pasarNama: string;
  komoditas: string[];
  tenggat: string;
  status: "BELUM" | "BERJALAN" | "SELESAI" | "TERLAMBAT";
  target: number;
  terisi: number;
  tipe: TipeHarga;
}

export interface MasterKomoditasRow {
  id: string;
  kode: string;
  nama: string;
  kategori: string;
  satuan: string;
  hargaAcuan: number;
  pilot: boolean;
  versi: number;
  status: "AKTIF" | "MENUNGGU_PERSETUJUAN" | "DIARSIPKAN";
}

export interface MasterWilayahRow {
  id: string;
  kode: string;
  /** Label ringkas, mis. "Kab. Aceh Besar" atau "Kota Langsa". */
  nama: string;
  ibukota: string;
  lat: number;
  lng: number;
}

export interface MasterPasarRow {
  id: string;
  nama: string;
  /** Label kabupaten/kota pemilik pasar. */
  kabupaten: string;
  radiusMeter: number;
  aktif: boolean;
}

/** Asal angka dashboard eksekutif; "DEMO" = data contoh, bukan angka resmi. */
export type SumberAngka = "BPS" | "BI" | "DJPB" | "BAPANAS" | "DEMO";

/** Satu layar eksekutif. Blok null = datanya belum ada (fiskal juga null tanpa izin `fiskal.lihat`). */
export interface RingkasanEksekutif {
  /** Benar bila ada angka contoh di salah satu blok; UI wajib menampilkan label demo. */
  demo: boolean;
  inflasi: {
    /** Bulan data, ISO lokal (siang hari agar aman dari pergeseran zona waktu). */
    periode: string;
    sumber: SumberAngka;
    /** Waktu angka masuk ke sistem (ISO), untuk label "data per tanggal/jam". */
    diimporPada: string;
    yoy: number;
    mtm: number;
    ytd: number;
    sasaran: { bawah: number; atas: number };
    seri: { bulan: string; yoy: number }[];
    penyumbang: { nama: string; andil: number }[];
  } | null;
  neraca: {
    periode: string;
    sumber: SumberAngka;
    items: { komoditasKode: string; nama: string; produksi: number; kebutuhan: number }[];
  } | null;
  fiskal: {
    sumber: SumberAngka;
    pagu: number;
    realisasi: number;
    skor: number;
    status: "MEMADAI" | "TERBATAS" | "KRITIS";
    perTanggal: string;
    instrumen: { nama: string; pagu: number; realisasi: number }[];
  } | null;
  ews: Record<Severity, number>;
  intervensi: Record<Siklus, { aktif: number; selesai: number }>;
  siklusDominan: Siklus;
}

export interface FaktorXai {
  nama: string;
  kontribusi: number; // poin persen terhadap perubahan harga proyeksi
  detail: string;
}

export interface ProyeksiXai {
  komoditasKode: string;
  nama: string;
  horizonHari: number;
  perubahanPct: number;
  interval: [number, number];
  hargaKini: number;
  seri: { tanggal: string; aktual?: number; proyeksi?: number; rentang?: [number, number] }[];
  faktor: FaktorXai[];
  narasi: string;
  model: { nama: string; versi: string; mape: number; dilatih: string };
  rekomendasi: string[];
  /** True bila proyeksi berasal dari data contoh seed, bukan model sungguhan. */
  demo?: boolean;
}
