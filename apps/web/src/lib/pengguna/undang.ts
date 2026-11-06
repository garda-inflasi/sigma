/**
 * Isian dialog "Undang pengguna": skema validasi di klien dan pemetaan ke masukan API. Server memeriksa ulang
 * (aturan yang sama dengan impor CSV); skema ini hanya memberi pesan lebih cepat.
 */
import { z } from "zod";
import { ROLES, ROLE_IDS, type Role } from "@/lib/rbac/roles";

/** Nilai pilihan "tanpa kabupaten" (Radix Select tidak menerima string kosong sebagai nilai). */
export const KABUPATEN_SEMUA = "semua";

/** Kabupaten/kota hanya relevan bagi peran yang cakupan datanya tidak seluruh Aceh. */
export function peranButuhKabupaten(role: Role): boolean {
  return ROLES[role].cakupan !== "SELURUH";
}

export const skemaUndang = z.object({
  nama: z
    .string()
    .trim()
    .min(1, "Nama wajib diisi.")
    .max(255, "Nama maksimal 255 karakter."),
  email: z
    .string()
    .trim()
    .min(1, "Email wajib diisi.")
    .pipe(z.email("Email tidak valid.")),
  peran: z.enum(ROLE_IDS),
  lembaga: z
    .string()
    .trim()
    .min(1, "Lembaga wajib diisi.")
    .max(255, "Lembaga maksimal 255 karakter."),
  kabupaten: z.string(),
});

export type IsianUndang = z.input<typeof skemaUndang>;
export type NilaiUndang = z.output<typeof skemaUndang>;

/** Masukan awal dialog. */
export function isianUndangAwal(): IsianUndang {
  return {
    nama: "",
    email: "",
    peran: "OPS_MAHASISWA",
    lembaga: "",
    kabupaten: KABUPATEN_SEMUA,
  };
}

/** Masukan API `POST /v1/users/undang`. Kabupaten hanya dikirim bila peran membutuhkannya dan dipilih. */
export interface UndangPenggunaInput {
  nama: string;
  email: string;
  peran: Role;
  lembaga: string;
  kabupaten_kode?: string;
}

export function keInputUndang(n: NilaiUndang): UndangPenggunaInput {
  const kab = n.kabupaten;
  const pakaiKabupaten =
    peranButuhKabupaten(n.peran) && kab !== "" && kab !== KABUPATEN_SEMUA;
  return {
    nama: n.nama,
    email: n.email.toLowerCase(),
    peran: n.peran,
    lembaga: n.lembaga,
    ...(pakaiKabupaten ? { kabupaten_kode: kab } : {}),
  };
}
