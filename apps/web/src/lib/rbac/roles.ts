/**
 * Peran 3 lapis (+ sistem) sesuai "Arsitektur Hak Akses Multilembaga".
 * TPID dan Pemda tidak menjadi pengguna sistem; keluaran untuk mereka
 * (policy brief, rekomendasi) didistribusikan oleh Unit Hubungan Pemerintah & Kebijakan.
 */
export type Lapis = "STRATEGIS" | "MANAJERIAL" | "OPERASIONAL" | "SISTEM";

export const ROLE_IDS = [
  "STRATEGIS_FMIPA",
  "STRATEGIS_BI",
  "STRATEGIS_BPS",
  "STRATEGIS_DJPB",
  "MANAJER_PROGRAM",
  "MANAJER_DATA",
  "MANAJER_KEBIJAKAN",
  "MANAJER_PEMBIAYAAN",
  "MANAJER_RISIKO",
  "MANAJER_MUTU",
  "OPS_KOORDINATOR",
  "OPS_AGREGATOR",
  "OPS_MAHASISWA",
  "ADMIN_SISTEM",
] as const;
export type Role = (typeof ROLE_IDS)[number];

/** Cakupan data baris (row-level scope) — ditegakkan juga di API Go. */
export type Cakupan = "SELURUH" | "KABUPATEN" | "SENDIRI";

export interface RoleMeta {
  label: string;
  lapis: Lapis;
  lembaga: string;
  tugas: string;
  cakupan: Cakupan;
  beranda: string;
  mfaWajib: boolean;
}

export const ROLES: Record<Role, RoleMeta> = {
  STRATEGIS_FMIPA: {
    label: "Pimpinan FMIPA USK",
    lapis: "STRATEGIS",
    lembaga: "FMIPA USK",
    tugas: "Pemegang mandat satu komando: menyetujui intervensi, policy brief, dan perubahan master data.",
    cakupan: "SELURUH",
    beranda: "/eksekutif",
    mfaWajib: true,
  },
  STRATEGIS_BI: {
    label: "Analis Bank Indonesia",
    lapis: "STRATEGIS",
    lembaga: "Bank Indonesia",
    tugas: "Kalibrator analisis kebijakan; verifikasi cepat saat siklus tekanan harga.",
    cakupan: "SELURUH",
    beranda: "/eksekutif",
    mfaWajib: true,
  },
  STRATEGIS_BPS: {
    label: "Statistisi BPS",
    lapis: "STRATEGIS",
    lembaga: "BPS",
    tugas: "Kompas statistik resmi; verifikasi silang data lapangan dan standar kode wilayah/komoditas.",
    cakupan: "SELURUH",
    beranda: "/eksekutif",
    mfaWajib: true,
  },
  STRATEGIS_DJPB: {
    label: "Analis Kanwil DJPb",
    lapis: "STRATEGIS",
    lembaga: "Kanwil DJPb",
    tugas: "Analitik fiskal; menyetujui dukungan instrumen belanja pada siklus tekanan.",
    cakupan: "SELURUH",
    beranda: "/eksekutif",
    mfaWajib: true,
  },
  MANAJER_PROGRAM: {
    label: "Manajer Program",
    lapis: "MANAJERIAL",
    lembaga: "FMIPA USK",
    tugas: "Mengorkestrasi intervensi harian lintas unit dan menyusun usulan ke Dewan Pengarah.",
    cakupan: "SELURUH",
    beranda: "/operasional",
    mfaWajib: true,
  },
  MANAJER_DATA: {
    label: "Unit Data & Statistik",
    lapis: "MANAJERIAL",
    lembaga: "FMIPA USK",
    tugas: "Integrasi API BPS/BI/DJPb/BMKG, ambang EWS, model prediktif, dan publikasi data.",
    cakupan: "SELURUH",
    beranda: "/operasional",
    mfaWajib: true,
  },
  MANAJER_KEBIJAKAN: {
    label: "Unit Hubungan Pemerintah & Kebijakan",
    lapis: "MANAJERIAL",
    lembaga: "FMIPA USK",
    tugas: "Mengubah temuan AI menjadi policy brief dan rekomendasi anggaran untuk mitra daerah.",
    cakupan: "SELURUH",
    beranda: "/policy-brief",
    mfaWajib: true,
  },
  MANAJER_PEMBIAYAAN: {
    label: "Unit Pembiayaan & Investasi",
    lapis: "MANAJERIAL",
    lembaga: "FMIPA USK",
    tugas: "Studi kelayakan infrastruktur logistik, desain tarif, identifikasi pendanaan.",
    cakupan: "SELURUH",
    beranda: "/cold-storage",
    mfaWajib: true,
  },
  MANAJER_RISIKO: {
    label: "Unit Monitoring & Risiko",
    lapis: "MANAJERIAL",
    lembaga: "FMIPA USK",
    tugas: "Audit silang data lapangan, publikasi data tervalidasi, pemantauan utilisasi gudang.",
    cakupan: "SELURUH",
    beranda: "/data/harga",
    mfaWajib: true,
  },
  MANAJER_MUTU: {
    label: "Unit Mutu & Pascapanen",
    lapis: "MANAJERIAL",
    lembaga: "FMIPA USK",
    tugas: "Uji mutu, standar pengemasan, model shelf-life, dan reduksi food loss.",
    cakupan: "SELURUH",
    beranda: "/cold-storage",
    mfaWajib: true,
  },
  OPS_KOORDINATOR: {
    label: "Koordinator Lapangan",
    lapis: "OPERASIONAL",
    lembaga: "FMIPA USK",
    tugas: "Memvalidasi entri harga, menugaskan agen lapangan di kabupatennya.",
    cakupan: "KABUPATEN",
    beranda: "/data/harga",
    mfaWajib: true,
  },
  OPS_AGREGATOR: {
    label: "Unit Agregator",
    lapis: "OPERASIONAL",
    lembaga: "Mitra Agregator",
    tugas: "Konsolidasi pasokan produsen dan pencatatan stok gudang fisik.",
    cakupan: "KABUPATEN",
    beranda: "/cold-storage",
    mfaWajib: false,
  },
  OPS_MAHASISWA: {
    label: "Agen Lapangan Mahasiswa",
    lapis: "OPERASIONAL",
    lembaga: "FMIPA USK",
    tugas: "Mencatat harga di pasar sesuai kode etik: dilarang menentukan harga atau menjadi perantara.",
    cakupan: "SENDIRI",
    beranda: "/lapangan/input",
    mfaWajib: false,
  },
  ADMIN_SISTEM: {
    label: "Administrator Sistem",
    lapis: "SISTEM",
    lembaga: "FMIPA USK",
    tugas: "Mengelola akun, integrasi, dan master data. Tidak memiliki hak persetujuan bisnis.",
    cakupan: "SELURUH",
    beranda: "/pengguna",
    mfaWajib: true,
  },
};

export const LAPIS_META: Record<Lapis, { label: string; urutan: number }> = {
  STRATEGIS: { label: "Lapis strategis — Dewan Pengarah", urutan: 0 },
  MANAJERIAL: { label: "Lapis manajerial — Tim Pelaksana Inti", urutan: 1 },
  OPERASIONAL: { label: "Lapis operasional wilayah", urutan: 2 },
  SISTEM: { label: "Sistem", urutan: 3 },
};
