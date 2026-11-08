/**
 * Nomor telepon profil (keputusan K3, docs/todos/20-profile.md). Disimpan sebagai E.164 Indonesia ("+62…");
 * aturannya identik dengan `NormalisasiTelepon` di API Go (usecase/profil.go), jadi pesan galat di form sama
 * dengan pesan dari server:
 *  1. dipangkas; string kosong berarti menghapus nomor (disimpan NULL);
 *  2. spasi, "-", ".", "(", dan ")" dibuang;
 *  3. awalan "+62" tetap, "62" menjadi "+62", "0" menjadi "+62"; awalan lain tidak valid;
 *  4. bagian nasional (setelah 62) harus 8–13 digit dan tidak diawali 0 (total E.164 10–15 digit).
 */
import { SPASI_GO, pangkas } from "./teks";

export const PESAN_TELEPON_TIDAK_VALID = "Nomor telepon tidak valid";

const NASIONAL = /^[1-9][0-9]{7,12}$/;
const PEMISAH = new RegExp(`[${SPASI_GO}\\-.()]`, "g");

export type HasilTelepon =
  | {
      ok: true;
      /** E.164, atau null bila nomor dihapus. */
      nilai: string | null;
    }
  | { ok: false; pesan: string };

const TIDAK_VALID: HasilTelepon = {
  ok: false,
  pesan: PESAN_TELEPON_TIDAK_VALID,
};

/** Menormalkan isian telepon (08…, 62…, +62…) ke E.164; kosong berarti hapus nomor. */
export function normalisasiTelepon(
  input: string | null | undefined,
): HasilTelepon {
  const mentah = pangkas(input ?? "");
  if (mentah === "") return { ok: true, nilai: null };

  const bersih = mentah.replace(PEMISAH, "");
  let nasional: string;
  if (bersih.startsWith("+62")) nasional = bersih.slice(3);
  else if (bersih.startsWith("62")) nasional = bersih.slice(2);
  else if (bersih.startsWith("0")) nasional = bersih.slice(1);
  else return TIDAK_VALID;

  return NASIONAL.test(nasional)
    ? { ok: true, nilai: `+62${nasional}` }
    : TIDAK_VALID;
}

/**
 * Tampilan yang mudah dibaca: "+6281234567890" menjadi "+62 812-3456-7890". Nilai yang bukan nomor Indonesia
 * yang sah dikembalikan apa adanya. Hasilnya dibaca kembali oleh `normalisasiTelepon` menjadi nilai semula.
 */
export function formatTelepon(nilai: string): string {
  const hasil = normalisasiTelepon(nilai);
  if (!hasil.ok || hasil.nilai === null) return nilai;
  const nasional = hasil.nilai.slice(3);
  const kelompok = [
    nasional.slice(0, 3),
    nasional.slice(3, 7),
    nasional.slice(7),
  ];
  return `+62 ${kelompok.filter(Boolean).join("-")}`;
}
