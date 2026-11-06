/**
 * Menjadikan fungsi validasi yang sudah ada (teruji, dipakai juga untuk tampilan langsung) sebagai skema
 * zod untuk `useFormBff`, sehingga semua form memakai satu jalur validasi tanpa menulis ulang aturannya.
 */
import { z } from "zod";

/** Hasil validator: null = valid; string = galat tingkat form; peta = galat per field. */
export type HasilValidasi = string | null | undefined | Partial<Record<string, string | undefined>>;

export function skemaDariValidasi<T>(validasi: (nilai: T) => HasilValidasi): z.ZodType<T, T> {
  return z.custom<T>(() => true).superRefine((nilai, ctx) => {
    const hasil = validasi(nilai);
    if (!hasil) return;
    if (typeof hasil === "string") {
      ctx.addIssue({ code: "custom", message: hasil });
      return;
    }
    for (const [field, pesan] of Object.entries(hasil)) {
      if (pesan) ctx.addIssue({ code: "custom", path: [field], message: pesan });
    }
  }) as z.ZodType<T, T>;
}
