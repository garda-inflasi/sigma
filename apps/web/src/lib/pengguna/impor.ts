/**
 * Impor pengguna dari CSV: konstanta format (sama dengan server, apps/api/internal/auth/usecase/impor_pengguna.go),
 * templat unduhan, pemeriksaan berkas sebelum dikirim, dan pemetaan hasil server ke model UI. Murni (tanpa
 * DOM) supaya teruji di Node.
 */

/** Kolom berkas impor, urut. Empat pertama wajib ada di header; kabupaten_kode boleh tidak ada. */
export const KOLOM_IMPOR = [
  "nama",
  "email",
  "peran",
  "lembaga",
  "kabupaten_kode",
] as const;

/** Batas server: lebih dari ini ditolak seluruhnya. */
export const MAKS_BARIS_IMPOR = 500;
export const MAKS_BYTE_IMPOR = 512 * 1024;

export const NAMA_BERKAS_TEMPLATE = "template-impor-pengguna.csv";

/** Penanda UTF-8 yang dipakai Excel; tanpanya Excel membaca berkas sebagai ANSI dan merusak huruf non-ASCII. */
const BOM = String.fromCharCode(0xfeff);

/**
 * Isi templat: hanya header (titik koma + BOM, terbuka benar di Excel berlokal Indonesia). Tanpa baris contoh,
 * supaya templat yang diunggah tanpa diisi tidak membuat akun contoh.
 */
export function templateImporCsv(): string {
  return `${BOM}${KOLOM_IMPOR.join(";")}\r\n`;
}

/** Pesan galat untuk berkas yang tidak layak dikirim, atau null bila layak. Server tetap memeriksa isinya. */
export function periksaBerkasImpor(berkas: {
  name: string;
  size: number;
}): string | null {
  if (!/\.csv$/i.test(berkas.name))
    return "Pilih berkas berekstensi .csv (simpan dari spreadsheet sebagai CSV UTF-8).";
  if (berkas.size === 0) return "Berkas kosong.";
  if (berkas.size > MAKS_BYTE_IMPOR)
    return `Berkas terlalu besar (maksimal ${MAKS_BYTE_IMPOR / 1024} KB).`;
  return null;
}

/** Status satu baris hasil impor: valid (lolos pemeriksaan), galat (ditolak), dibuat (akun sudah dibuat). */
export type StatusBarisImpor = "valid" | "galat" | "dibuat";

export interface BarisImporWire {
  baris: number;
  nama: string;
  email: string;
  peran: string;
  lembaga: string;
  kabupaten_kode: string;
  status: string;
  galat?: string[] | null;
  id?: string;
}

export interface ImporWire {
  dry_run: boolean;
  total: number;
  valid: number;
  galat: number;
  dibuat: number;
  email_aktif: boolean;
  baris?: BarisImporWire[] | null;
}

export interface BarisImpor {
  /** Nomor baris di berkas (header = 1), sama dengan nomor di spreadsheet. */
  baris: number;
  nama: string;
  email: string;
  peran: string;
  lembaga: string;
  kabupatenKode: string;
  status: StatusBarisImpor;
  galat: string[];
  id?: string;
}

export interface HasilImpor {
  dryRun: boolean;
  total: number;
  valid: number;
  galat: number;
  dibuat: number;
  /** False bila SMTP server tidak dikonfigurasi: akun dibuat tetapi tidak ada email yang dikirim. */
  emailAktif: boolean;
  baris: BarisImpor[];
}

const STATUS_BARIS: readonly StatusBarisImpor[] = ["valid", "galat", "dibuat"];

function keStatusBaris(status: string): StatusBarisImpor {
  // Status tak dikenal diperlakukan sebagai galat: tidak pernah menampilkan baris yang tak jelas sebagai sukses.
  return (STATUS_BARIS as readonly string[]).includes(status)
    ? (status as StatusBarisImpor)
    : "galat";
}

export function toHasilImpor(w: ImporWire): HasilImpor {
  return {
    dryRun: w.dry_run,
    total: w.total,
    valid: w.valid,
    galat: w.galat,
    dibuat: w.dibuat,
    emailAktif: w.email_aktif,
    baris: (w.baris ?? []).map((b) => ({
      baris: b.baris,
      nama: b.nama,
      email: b.email,
      peran: b.peran,
      lembaga: b.lembaga,
      kabupatenKode: b.kabupaten_kode,
      status: keStatusBaris(b.status),
      galat: b.galat ?? [],
      id: b.id || undefined,
    })),
  };
}

/** Label status baris di tabel pratinjau dan hasil. */
export const LABEL_STATUS_BARIS: Record<StatusBarisImpor, string> = {
  valid: "Valid",
  galat: "Ditolak",
  dibuat: "Dibuat",
};

/** Ringkasan satu kalimat untuk pratinjau. */
export function ringkasPratinjau(
  h: Pick<HasilImpor, "total" | "valid" | "galat">,
): string {
  const n = (v: number) => v.toLocaleString("id-ID");
  if (h.valid === 0)
    return `${n(h.total)} baris diperiksa, tidak ada yang valid.`;
  if (h.galat === 0) return `${n(h.total)} baris diperiksa, semuanya valid.`;
  return `${n(h.total)} baris diperiksa: ${n(h.valid)} valid, ${n(h.galat)} ditolak dan tidak akan diimpor.`;
}

/** Ringkasan satu kalimat untuk hasil komit. */
export function ringkasHasil(
  h: Pick<HasilImpor, "dibuat" | "galat" | "emailAktif">,
): string {
  const n = (v: number) => v.toLocaleString("id-ID");
  const bagian = [`${n(h.dibuat)} akun dibuat sebagai undangan`];
  if (h.galat > 0) bagian.push(`${n(h.galat)} baris ditolak`);
  const kalimat = `${bagian.join(", ")}.`;
  return h.emailAktif
    ? kalimat
    : `${kalimat} SMTP belum dikonfigurasi, jadi email undangan tidak dikirim.`;
}
