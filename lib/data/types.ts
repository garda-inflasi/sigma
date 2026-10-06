import type { Severity, Siklus } from "@/lib/design/tokens";

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
  volumeTon: number;
  masuk: string;
  sisaShelfLifeHari: number;
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
  suhuAktualC: number;
  status: "OPERASIONAL" | "PERAWATAN" | "RENCANA";
  lat: number;
  lng: number;
}

export interface SumberData {
  id: string;
  nama: string;
  penyedia: string;
  jenis: "API" | "SCRAPER";
  cakupan: string;
  jadwal: string;
  terakhir: string;
  status: "SEHAT" | "TERTUNDA" | "GAGAL" | "DIJEDA";
  keberhasilan7h: number;
  latensiMs: number;
  rekamTerakhir: number;
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
  hasil: "BERHASIL" | "DITOLAK";
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
  pilot: boolean;
  versi: number;
  status: "AKTIF" | "MENUNGGU_PERSETUJUAN" | "DIARSIPKAN";
}

export interface RingkasanEksekutif {
  inflasi: {
    yoy: number;
    mtm: number;
    ytd: number;
    sasaran: { bawah: number; atas: number };
    rilis: string;
    seri: { bulan: string; yoy: number }[];
    penyumbang: { nama: string; andil: number }[];
  };
  neraca: { komoditasKode: string; nama: string; produksi: number; kebutuhan: number }[];
  fiskal: {
    pagu: number;
    realisasi: number;
    skor: number;
    status: "MEMADAI" | "TERBATAS" | "KRITIS";
    perTanggal: string;
    instrumen: { nama: string; pagu: number; realisasi: number }[];
  };
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
}
