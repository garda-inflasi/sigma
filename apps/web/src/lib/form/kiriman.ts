/**
 * Inti kiriman form ke BFF, tanpa React: validasi skema di klien, panggilan asinkron, pemetaan galat HTTP
 * (400/422 per field), dan penentuan navigasi sesudah 2xx. Hook `useFormBff` hanya membungkus fungsi ini
 * dengan state React, sehingga seluruh aturannya teruji di node:test.
 */
import type { ZodType } from "zod";
import { safeInternalPath } from "@/lib/auth/safe-path";

/** Pesan galat per field, dikunci nama field nilai form. */
export type GalatField<TNilai> = Partial<
  Record<Extract<keyof TNilai, string>, string>
>;

/** Navigasi sesudah kiriman berhasil. Tujuan selalu disaring menjadi path internal (cegah open redirect/XSS). */
export interface Navigasi {
  ke: string;
  /** "replace" (bawaan) tidak menambah riwayat: tombol kembali tidak kembali ke form yang sudah terkirim. */
  mode?: "replace" | "push";
  /**
   * Muat ulang data Server Component (termasuk root layout) sesudah navigasi. Wajib bila kiriman mengubah
   * cookie sesi (masuk, ganti kata sandi, MFA): sesi dibaca root layout, yang tidak dirender ulang oleh
   * navigasi antargrup rute.
   */
  segarkan?: boolean;
}

/** Status HTTP yang berarti masukan ditolak dan layak ditampilkan per field atau di bawah form. */
export function adalahGalatValidasi(status: number | undefined): boolean {
  return status === 400 || status === 422;
}

/** Galat skema (zod) menjadi pesan per field: pesan pertama untuk setiap field tingkat atas. */
export function galatFieldDariIsu<TNilai>(
  isu: readonly { path: readonly PropertyKey[]; message: string }[],
): GalatField<TNilai> {
  const hasil: Record<string, string> = {};
  for (const i of isu) {
    const field = i.path[0];
    if (typeof field === "string" && !(field in hasil))
      hasil[field] = i.message;
  }
  return hasil as GalatField<TNilai>;
}

/**
 * Galat per field dari badan respons 400/422. Bentuk yang dikenal: `{ errors: { field: pesan | pesan[] } }`
 * atau `{ fields: … }` (BFF), dan `{ data: { errors: … } }` (amplop API Go). Bentuk lain = tanpa galat field.
 */
export function galatFieldDariPayload<TNilai>(
  payload: unknown,
): GalatField<TNilai> {
  if (!payload || typeof payload !== "object") return {};
  const p = payload as Record<string, unknown>;
  const data =
    p.data && typeof p.data === "object"
      ? (p.data as Record<string, unknown>)
      : undefined;
  const sumber = p.errors ?? p.fields ?? data?.errors;
  if (!sumber || typeof sumber !== "object" || Array.isArray(sumber)) return {};
  const hasil: Record<string, string> = {};
  for (const [field, nilai] of Object.entries(
    sumber as Record<string, unknown>,
  )) {
    const pesan = Array.isArray(nilai)
      ? nilai.find((v) => typeof v === "string")
      : nilai;
    if (typeof pesan === "string" && pesan) hasil[field] = pesan;
  }
  return hasil as GalatField<TNilai>;
}

/** Status dan payload dari exception klien BFF (`ApiError`), dibaca struktural. */
export function statusDanPayload(err: unknown): {
  status?: number;
  payload?: unknown;
} {
  if (!err || typeof err !== "object") return {};
  const status = (err as { status?: unknown }).status;
  return {
    status: typeof status === "number" ? status : undefined,
    payload: (err as { payload?: unknown }).payload,
  };
}

/** Navigasi dengan tujuan yang sudah disaring; `null` bila tidak ada navigasi. */
export function navigasiAman(
  nav: Navigasi | string | void | null | undefined,
): Required<Navigasi> | null {
  if (!nav) return null;
  const n = typeof nav === "string" ? { ke: nav } : nav;
  return {
    ke: safeInternalPath(n.ke),
    mode: n.mode ?? "replace",
    segarkan: n.segarkan ?? false,
  };
}

/**
 * `TMasukan` adalah nilai mentah form, `TNilai` nilai sesudah skema (mis. sesudah trim/transform). Tanpa
 * skema keduanya sama.
 */
export interface LangkahKiriman<TNilai, THasil, TGalat, TMasukan = TNilai> {
  nilai: TMasukan;
  skema?: ZodType<TNilai, TMasukan>;
  kirim: (nilai: TNilai) => Promise<THasil>;
  /** Dipanggil sesudah 2xx; nilai kembaliannya menentukan navigasi. */
  sukses?: (
    hasil: THasil,
    nilai: TNilai,
  ) => Navigasi | string | void | Promise<Navigasi | string | void>;
  /** Galat (selain galat skema) menjadi galat tingkat form yang ditampilkan. */
  petaGalat: (err: unknown) => TGalat;
}

export type HasilKiriman<THasil, TGalat, TMasukan> =
  | {
      jenis: "tidak-valid";
      galatField: GalatField<TMasukan>;
      /** Galat skema yang tidak menunjuk field (aturan lintas field, mis. "pilih sedikitnya satu kanal"). */
      pesanForm?: string;
    }
  | {
      jenis: "gagal";
      galat: TGalat;
      galatField: GalatField<TMasukan>;
      error: unknown;
    }
  | { jenis: "berhasil"; hasil: THasil; navigasi: Required<Navigasi> | null };

/**
 * Menjalankan satu kiriman. Tidak pernah melempar: setiap kegagalan menjadi hasil `gagal` agar form selalu
 * kembali ke keadaan siap kirim (tanpa reload, tanpa state yang tertahan).
 */
export async function jalankanKiriman<TNilai, THasil, TGalat, TMasukan = TNilai>(
  langkah: LangkahKiriman<TNilai, THasil, TGalat, TMasukan>,
): Promise<HasilKiriman<THasil, TGalat, TMasukan>> {
  let nilai: TNilai;
  if (langkah.skema) {
    const cek = langkah.skema.safeParse(langkah.nilai);
    if (!cek.success)
      return {
        jenis: "tidak-valid",
        galatField: galatFieldDariIsu<TMasukan>(cek.error.issues),
        pesanForm: cek.error.issues.find((i) => typeof i.path[0] !== "string")?.message,
      };
    nilai = cek.data;
  } else {
    // Tanpa skema tidak ada transformasi: nilai mentah adalah nilai kiriman (TMasukan bawaan = TNilai).
    nilai = langkah.nilai as unknown as TNilai;
  }
  try {
    const hasil = await langkah.kirim(nilai);
    const navigasi = navigasiAman(await langkah.sukses?.(hasil, nilai));
    return { jenis: "berhasil", hasil, navigasi };
  } catch (error) {
    const { status, payload } = statusDanPayload(error);
    const galatField = adalahGalatValidasi(status)
      ? galatFieldDariPayload<TMasukan>(payload)
      : {};
    return {
      jenis: "gagal",
      galat: langkah.petaGalat(error),
      galatField,
      error,
    };
  }
}

/** Galat tingkat form bawaan: satu pesan siap tampil. */
export interface GalatForm {
  pesan: string;
}

export const PESAN_GALAT_SERVER = "Tidak dapat terhubung ke server. Coba lagi.";

/**
 * Pesan bawaan untuk galat kiriman: pesan dari server untuk galat klien (4xx, termasuk 400/422 validasi),
 * pesan umum untuk galat server dan jaringan (isinya tidak layak ditampilkan apa adanya).
 */
export function galatFormBawaan(err: unknown): GalatForm {
  const { status } = statusDanPayload(err);
  if (status !== undefined && status < 500 && err instanceof Error && err.message) return { pesan: err.message };
  return { pesan: PESAN_GALAT_SERVER };
}
