/**
 * Master data acuan — dipakai bersama oleh prisma/seed.ts dan mode mock UI.
 * Kode wilayah mengikuti kode BPS. Koordinat bersifat perkiraan (ibu kota/pasar)
 * untuk keperluan seed & demo; ganti dengan data geospasial resmi saat produksi.
 */
import type { Role } from "../rbac/roles";

export type KategoriKomoditas = "Pangan pokok" | "Hortikultura" | "Peternakan" | "Perikanan" | "Pangan olahan";
export type IkonKomoditas = "beras" | "cabai" | "bawang" | "telur" | "ayam" | "sapi" | "ikan" | "minyak" | "gula" | "sayur";

export interface Komoditas {
  kode: string;
  nama: string;
  satuan: "kg" | "liter";
  kategori: KategoriKomoditas;
  hargaAcuan: number; // harga konsumen acuan (Rp)
  volatilitas: number; // simpangan harian
  margin: number; // rasio konsumen/produsen
  pilot: boolean; // dikunci untuk fase 1 (2–3 komoditas)
  ikon: IkonKomoditas;
  shelfLifeHari: number; // pada cold storage
}

export const KOMODITAS: Komoditas[] = [
  { kode: "BRS-MED", nama: "Beras medium", satuan: "kg", kategori: "Pangan pokok", hargaAcuan: 13900, volatilitas: 0.004, margin: 1.12, pilot: true, ikon: "beras", shelfLifeHari: 180 },
  { kode: "BRS-PRM", nama: "Beras premium", satuan: "kg", kategori: "Pangan pokok", hargaAcuan: 15800, volatilitas: 0.004, margin: 1.12, pilot: false, ikon: "beras", shelfLifeHari: 180 },
  { kode: "CBM-KRT", nama: "Cabai merah keriting", satuan: "kg", kategori: "Hortikultura", hargaAcuan: 52000, volatilitas: 0.034, margin: 1.45, pilot: true, ikon: "cabai", shelfLifeHari: 21 },
  { kode: "CBR-RWT", nama: "Cabai rawit merah", satuan: "kg", kategori: "Hortikultura", hargaAcuan: 58000, volatilitas: 0.038, margin: 1.45, pilot: false, ikon: "cabai", shelfLifeHari: 18 },
  { kode: "BWM", nama: "Bawang merah", satuan: "kg", kategori: "Hortikultura", hargaAcuan: 41000, volatilitas: 0.021, margin: 1.35, pilot: true, ikon: "bawang", shelfLifeHari: 60 },
  { kode: "BWP", nama: "Bawang putih", satuan: "kg", kategori: "Hortikultura", hargaAcuan: 38000, volatilitas: 0.01, margin: 1.25, pilot: false, ikon: "bawang", shelfLifeHari: 120 },
  { kode: "TLR", nama: "Telur ayam ras", satuan: "kg", kategori: "Peternakan", hargaAcuan: 29500, volatilitas: 0.008, margin: 1.15, pilot: false, ikon: "telur", shelfLifeHari: 30 },
  { kode: "DAG-AYM", nama: "Daging ayam ras", satuan: "kg", kategori: "Peternakan", hargaAcuan: 36500, volatilitas: 0.012, margin: 1.2, pilot: false, ikon: "ayam", shelfLifeHari: 90 },
  { kode: "DAG-SPI", nama: "Daging sapi", satuan: "kg", kategori: "Peternakan", hargaAcuan: 148000, volatilitas: 0.003, margin: 1.1, pilot: false, ikon: "sapi", shelfLifeHari: 180 },
  { kode: "IKN-TGK", nama: "Ikan tongkol", satuan: "kg", kategori: "Perikanan", hargaAcuan: 38000, volatilitas: 0.024, margin: 1.3, pilot: false, ikon: "ikan", shelfLifeHari: 120 },
  { kode: "MYK", nama: "Minyak goreng curah", satuan: "liter", kategori: "Pangan olahan", hargaAcuan: 17500, volatilitas: 0.003, margin: 1.08, pilot: false, ikon: "minyak", shelfLifeHari: 365 },
  { kode: "GLA", nama: "Gula pasir", satuan: "kg", kategori: "Pangan olahan", hargaAcuan: 18000, volatilitas: 0.003, margin: 1.08, pilot: false, ikon: "gula", shelfLifeHari: 365 },
  { kode: "KTG", nama: "Kentang", satuan: "kg", kategori: "Hortikultura", hargaAcuan: 16000, volatilitas: 0.015, margin: 1.3, pilot: false, ikon: "sayur", shelfLifeHari: 75 },
  { kode: "TMT", nama: "Tomat", satuan: "kg", kategori: "Hortikultura", hargaAcuan: 12000, volatilitas: 0.03, margin: 1.4, pilot: false, ikon: "sayur", shelfLifeHari: 14 },
];

export interface Kabupaten {
  kode: string;
  nama: string;
  ibukota: string;
  lat: number;
  lng: number;
  jenis: "Kabupaten" | "Kota";
}

export const PROVINSI = { kode: "11", nama: "Aceh" } as const;

export const KABUPATEN: Kabupaten[] = [
  { kode: "1101", nama: "Simeulue", ibukota: "Sinabang", lat: 2.48, lng: 96.38, jenis: "Kabupaten" },
  { kode: "1102", nama: "Aceh Singkil", ibukota: "Singkil", lat: 2.28, lng: 97.79, jenis: "Kabupaten" },
  { kode: "1103", nama: "Aceh Selatan", ibukota: "Tapaktuan", lat: 3.26, lng: 97.18, jenis: "Kabupaten" },
  { kode: "1104", nama: "Aceh Tenggara", ibukota: "Kutacane", lat: 3.49, lng: 97.8, jenis: "Kabupaten" },
  { kode: "1105", nama: "Aceh Timur", ibukota: "Idi Rayeuk", lat: 4.96, lng: 97.77, jenis: "Kabupaten" },
  { kode: "1106", nama: "Aceh Tengah", ibukota: "Takengon", lat: 4.62, lng: 96.85, jenis: "Kabupaten" },
  { kode: "1107", nama: "Aceh Barat", ibukota: "Meulaboh", lat: 4.14, lng: 96.13, jenis: "Kabupaten" },
  { kode: "1108", nama: "Aceh Besar", ibukota: "Kota Jantho", lat: 5.3, lng: 95.63, jenis: "Kabupaten" },
  { kode: "1109", nama: "Pidie", ibukota: "Sigli", lat: 5.38, lng: 95.96, jenis: "Kabupaten" },
  { kode: "1110", nama: "Bireuen", ibukota: "Bireuen", lat: 5.2, lng: 96.7, jenis: "Kabupaten" },
  { kode: "1111", nama: "Aceh Utara", ibukota: "Lhoksukon", lat: 5.05, lng: 97.32, jenis: "Kabupaten" },
  { kode: "1112", nama: "Aceh Barat Daya", ibukota: "Blangpidie", lat: 3.74, lng: 96.84, jenis: "Kabupaten" },
  { kode: "1113", nama: "Gayo Lues", ibukota: "Blangkejeren", lat: 3.98, lng: 97.35, jenis: "Kabupaten" },
  { kode: "1114", nama: "Aceh Tamiang", ibukota: "Karang Baru", lat: 4.29, lng: 98.05, jenis: "Kabupaten" },
  { kode: "1115", nama: "Nagan Raya", ibukota: "Suka Makmue", lat: 4.13, lng: 96.47, jenis: "Kabupaten" },
  { kode: "1116", nama: "Aceh Jaya", ibukota: "Calang", lat: 4.63, lng: 95.58, jenis: "Kabupaten" },
  { kode: "1117", nama: "Bener Meriah", ibukota: "Simpang Tiga Redelong", lat: 4.73, lng: 96.87, jenis: "Kabupaten" },
  { kode: "1118", nama: "Pidie Jaya", ibukota: "Meureudu", lat: 5.25, lng: 96.25, jenis: "Kabupaten" },
  { kode: "1171", nama: "Banda Aceh", ibukota: "Banda Aceh", lat: 5.55, lng: 95.32, jenis: "Kota" },
  { kode: "1172", nama: "Sabang", ibukota: "Sabang", lat: 5.89, lng: 95.32, jenis: "Kota" },
  { kode: "1173", nama: "Langsa", ibukota: "Langsa", lat: 4.47, lng: 97.97, jenis: "Kota" },
  { kode: "1174", nama: "Lhokseumawe", ibukota: "Lhokseumawe", lat: 5.18, lng: 97.15, jenis: "Kota" },
  { kode: "1175", nama: "Subulussalam", ibukota: "Subulussalam", lat: 2.64, lng: 98.0, jenis: "Kota" },
];

export interface Pasar {
  id: string;
  nama: string;
  kabupatenKode: string;
  lat: number;
  lng: number;
  radiusMeter: number; // geofence
}

export const PASAR: Pasar[] = [
  { id: "PSR-1171-01", nama: "Pasar Peunayong", kabupatenKode: "1171", lat: 5.5603, lng: 95.3237, radiusMeter: 400 },
  { id: "PSR-1171-02", nama: "Pasar Al-Mahirah Lamdingin", kabupatenKode: "1171", lat: 5.5765, lng: 95.3352, radiusMeter: 400 },
  { id: "PSR-1108-01", nama: "Pasar Induk Lambaro", kabupatenKode: "1108", lat: 5.5085, lng: 95.364, radiusMeter: 600 },
  { id: "PSR-1174-01", nama: "Pasar Inpres Lhokseumawe", kabupatenKode: "1174", lat: 5.179, lng: 97.146, radiusMeter: 450 },
  { id: "PSR-1110-01", nama: "Pasar Inpres Bireuen", kabupatenKode: "1110", lat: 5.203, lng: 96.703, radiusMeter: 450 },
  { id: "PSR-1109-01", nama: "Pasar Kota Sigli", kabupatenKode: "1109", lat: 5.386, lng: 95.96, radiusMeter: 450 },
  { id: "PSR-1107-01", nama: "Pasar Bina Usaha Meulaboh", kabupatenKode: "1107", lat: 4.145, lng: 96.129, radiusMeter: 450 },
  { id: "PSR-1106-01", nama: "Pasar Inpres Takengon", kabupatenKode: "1106", lat: 4.627, lng: 96.846, radiusMeter: 450 },
  { id: "PSR-1173-01", nama: "Pasar Kota Langsa", kabupatenKode: "1173", lat: 4.47, lng: 97.968, radiusMeter: 450 },
  { id: "PSR-1103-01", nama: "Pasar Tapaktuan", kabupatenKode: "1103", lat: 3.26, lng: 97.183, radiusMeter: 400 },
  { id: "PSR-1104-01", nama: "Pasar Kutacane", kabupatenKode: "1104", lat: 3.493, lng: 97.8, radiusMeter: 400 },
  { id: "PSR-1101-01", nama: "Pasar Sinabang", kabupatenKode: "1101", lat: 2.48, lng: 96.38, radiusMeter: 400 },
];

export interface ColdStorageRef {
  id: string;
  nama: string;
  kabupatenKode: string;
  lat: number;
  lng: number;
  kapasitasTon: number;
  suhuTargetC: number;
  status: "OPERASIONAL" | "PERAWATAN" | "RENCANA";
  fokus: string[]; // kode komoditas
}

export const COLD_STORAGE: ColdStorageRef[] = [
  { id: "CS-01", nama: "Cold Storage Lambaro", kabupatenKode: "1108", lat: 5.505, lng: 95.37, kapasitasTon: 400, suhuTargetC: 4, status: "OPERASIONAL", fokus: ["CBM-KRT", "CBR-RWT", "BWM"] },
  { id: "CS-02", nama: "Cold Storage Perikanan Lhokseumawe", kabupatenKode: "1174", lat: 5.17, lng: 97.13, kapasitasTon: 300, suhuTargetC: -20, status: "OPERASIONAL", fokus: ["IKN-TGK"] },
  { id: "CS-03", nama: "Sentra Hortikultura Takengon", kabupatenKode: "1106", lat: 4.64, lng: 96.86, kapasitasTon: 250, suhuTargetC: 6, status: "OPERASIONAL", fokus: ["KTG", "TMT", "CBM-KRT"] },
  { id: "CS-04", nama: "Gudang Dingin Meulaboh", kabupatenKode: "1107", lat: 4.15, lng: 96.14, kapasitasTon: 150, suhuTargetC: 4, status: "PERAWATAN", fokus: ["BWM", "TLR"] },
  { id: "CS-05", nama: "Gudang Dingin Langsa", kabupatenKode: "1173", lat: 4.48, lng: 97.96, kapasitasTon: 120, suhuTargetC: 4, status: "RENCANA", fokus: ["CBM-KRT", "TLR"] },
];

export interface UserRef {
  id: string;
  nama: string;
  email: string;
  role: Role;
  lembaga: string;
  kabupatenKode?: string;
}

/** Akun demo (fiktif). Domain .test agar tidak pernah tertukar dengan alamat nyata. */
export const USERS: UserRef[] = [
  // ===== PIMPINAN FMIPA USK (Update 2026) =====
  { id: "usr-str-01", nama: "Prof. Dr.rer.nat. Ilham Maulana, S.Si.", email: "ilham.maulana@fmipa.garda.test", role: "STRATEGIS_FMIPA", lembaga: "FMIPA USK" },
  // Wakil Dekan Bidang Akademik
  { id: "usr-str-05", nama: "Prof. Dr. Eng. Elin Yusibani, S.Si., M.Eng.", email: "elin.yusibani@fmipa.garda.test", role: "STRATEGIS_FMIPA", lembaga: "FMIPA USK" },
  // Wakil Dekan Bidang SDM dan Keuangan
  { id: "usr-str-06", nama: "Dr. Munawar, S.Si., M.App.Stats.", email: "munawar@fmipa.garda.test", role: "STRATEGIS_FMIPA", lembaga: "FMIPA USK" },
  // Wakil Dekan Bidang Kemahasiswaan, Alumni, dan Kemitraan
  { id: "usr-str-07", nama: "Dr. Meutia Faradilla, S.Si., M.Si., Apt.", email: "meutia.faradilla@fmipa.garda.test", role: "STRATEGIS_FMIPA", lembaga: "FMIPA USK" },

  // ===== STAKEHOLDER EKSTERNAL (Tidak Berubah) =====
  { id: "usr-str-02", nama: "Nadia Safitri", email: "nadia.safitri@bi.garda.test", role: "STRATEGIS_BI", lembaga: "Bank Indonesia" },
  { id: "usr-str-03", nama: "Iskandar Muda Putra", email: "iskandar.putra@bps.garda.test", role: "STRATEGIS_BPS", lembaga: "BPS Provinsi Aceh" },
  { id: "usr-str-04", nama: "Maulana Akbar", email: "maulana.akbar@djpb.garda.test", role: "STRATEGIS_DJPB", lembaga: "Kanwil DJPb Aceh" },

  // ===== MANAJER PROGRAM & DATA (Internal FMIPA) =====
  { id: "usr-man-01", nama: "Cut Rina Mayasari", email: "rina.mayasari@fmipa.garda.test", role: "MANAJER_PROGRAM", lembaga: "FMIPA USK" },
  { id: "usr-man-02", nama: "Fajar Ramadhan", email: "fajar.ramadhan@fmipa.garda.test", role: "MANAJER_DATA", lembaga: "FMIPA USK" },
  { id: "usr-man-03", nama: "Siti Humaira", email: "siti.humaira@fmipa.garda.test", role: "MANAJER_KEBIJAKAN", lembaga: "FMIPA USK" },
  { id: "usr-man-04", nama: "Teuku Arif Hidayat", email: "arif.hidayat@fmipa.garda.test", role: "MANAJER_PEMBIAYAAN", lembaga: "FMIPA USK" },
  { id: "usr-man-05", nama: "Dewi Kartika", email: "dewi.kartika@fmipa.garda.test", role: "MANAJER_RISIKO", lembaga: "FMIPA USK" },
  { id: "usr-man-06", nama: "Zulfikar Hamdani", email: "zulfikar.hamdani@fmipa.garda.test", role: "MANAJER_MUTU", lembaga: "FMIPA USK" },

  // ===== OPERASIONAL LAPANGAN =====
  { id: "usr-ops-01", nama: "Rizki Maulana", email: "rizki.maulana@lapangan.garda.test", role: "OPS_KOORDINATOR", lembaga: "FMIPA USK", kabupatenKode: "1171" },
  { id: "usr-ops-02", nama: "Aulia Rahmah", email: "aulia.rahmah@lapangan.garda.test", role: "OPS_KOORDINATOR", lembaga: "FMIPA USK", kabupatenKode: "1174" },
  { id: "usr-ops-03", nama: "Hendra Saputra", email: "hendra.saputra@lapangan.garda.test", role: "OPS_KOORDINATOR", lembaga: "FMIPA USK", kabupatenKode: "1106" },
  { id: "usr-ops-04", nama: "Koperasi Tani Gayo Makmur", email: "admin@gayomakmur.garda.test", role: "OPS_AGREGATOR", lembaga: "Mitra Agregator", kabupatenKode: "1106" },
  { id: "usr-ops-05", nama: "CV Lambaro Segar", email: "gudang@lambarosegar.garda.test", role: "OPS_AGREGATOR", lembaga: "Mitra Agregator", kabupatenKode: "1108" },
  ...[
    ["Annisa Putri", "1171"], ["Muhammad Ikhsan", "1171"], ["Rahmi Yanti", "1108"], ["Fadli Akbar", "1174"],
    ["Nurul Hidayah", "1110"], ["Agus Salim", "1109"], ["Intan Permata", "1107"], ["Dimas Pratama", "1106"],
    ["Liza Ananda", "1173"], ["Yusuf Haikal", "1103"], ["Mutia Sari", "1104"], ["Rafi Alfarizi", "1101"],
  ].map(([nama, kab], i) => ({
    id: `usr-mhs-${String(i + 1).padStart(2, "0")}`,
    nama,
    email: `${nama.toLowerCase().replace(/\s+/g, ".")}@mhs.garda.test`,
    role: "OPS_MAHASISWA" as Role,
    lembaga: "FMIPA USK",
    kabupatenKode: kab,
  })),
  { id: "usr-sys-01", nama: "Admin Sistem Garda", email: "admin@garda.test", role: "ADMIN_SISTEM", lembaga: "FMIPA USK" },
];

export const kabupatenByKode = Object.fromEntries(KABUPATEN.map((k) => [k.kode, k])) as Record<string, Kabupaten>;
export const komoditasByKode = Object.fromEntries(KOMODITAS.map((k) => [k.kode, k])) as Record<string, Komoditas>;
export const pasarById = Object.fromEntries(PASAR.map((p) => [p.id, p])) as Record<string, Pasar>;
