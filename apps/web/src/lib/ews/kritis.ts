/**
 * Aturan tampilan peringatan KRITIS sebagai fungsi murni (dipakai toast, banner, feed, dan halaman EWS).
 * - "Belum ditindaklanjuti" = KRITIS berstatus BARU: hanya itu yang menahan toast dan banner.
 * - "Aktif" = KRITIS yang belum SELESAI: selalu di urutan teratas dan tidak pernah terpotong batas tampilan.
 */

import type { EwsAlert } from "@/lib/data/types";

/** Batas toast KRITIS yang tampil bersamaan; sisanya terwakili oleh banner global. */
export const MAKS_TOAST_KRITIS = 3;

type SeverityStatus = Pick<EwsAlert, "severity" | "status">;

export const kritisBelumDitindaklanjuti = (a: SeverityStatus) =>
  a.severity === "KRITIS" && a.status === "BARU";

export const kritisAktif = (a: SeverityStatus) =>
  a.severity === "KRITIS" && a.status !== "SELESAI";

/** Urutan stabil dengan KRITIS aktif di depan. */
export function kritisDulu<T extends SeverityStatus>(daftar: readonly T[]): T[] {
  return [...daftar.filter(kritisAktif), ...daftar.filter((a) => !kritisAktif(a))];
}

/**
 * Isi feed: alert yang perlu tindakan, KRITIS aktif di depan. `batas` hanya memotong alert selain KRITIS aktif,
 * jadi puluhan KRITIS tetap tampil semua walau melebihi batas.
 */
export function isiFeed<T extends SeverityStatus & { id: string }>(
  daftar: readonly T[],
  baru: readonly string[],
  batas: number,
): T[] {
  const perluTindakan = daftar.filter(
    (a) =>
      kritisAktif(a) ||
      a.status === "BARU" ||
      a.status === "DIESKALASI" ||
      baru.includes(a.id),
  );
  const kritis = perluTindakan.filter(kritisAktif);
  const lain = perluTindakan.filter((a) => !kritisAktif(a));
  return [...kritis, ...lain.slice(0, Math.max(batas - kritis.length, 0))];
}

/** ID alert yang tampil sebagai toast: yang belum dikecilkan, paling banyak `maks`, mengikuti urutan daftar. */
export function pilihToastKritis(
  kritis: readonly Pick<EwsAlert, "id">[],
  dikecilkan: readonly string[],
  maks = MAKS_TOAST_KRITIS,
): string[] {
  return kritis
    .map((a) => a.id)
    .filter((id) => !dikecilkan.includes(id))
    .slice(0, maks);
}

/** Menerapkan satu peristiwa aliran pada daftar KRITIS belum ditindaklanjuti. Daftar belum dimuat → tetap. */
export function terapkanPeristiwaKritis(
  daftar: EwsAlert[] | undefined,
  alert: EwsAlert,
): EwsAlert[] | undefined {
  if (!daftar) return daftar;
  const tanpa = daftar.filter((a) => a.id !== alert.id);
  if (!kritisBelumDitindaklanjuti(alert)) {
    return tanpa.length === daftar.length ? daftar : tanpa;
  }
  return tanpa.length === daftar.length
    ? [alert, ...daftar]
    : daftar.map((a) => (a.id === alert.id ? alert : a));
}

/** Kalimat konfirmasi "Tandai selesai" yang menyebut jumlah KRITIS terdampak. */
export function pesanTutupMassal(
  n: number,
  baris: readonly SeverityStatus[],
): string {
  const dasar = `${n} alert ditandai selesai dan keluar dari feed peringatan aktif.`;
  const kritis = baris.filter(kritisAktif).length;
  return kritis > 0
    ? `${dasar} Termasuk ${kritis} peringatan KRITIS: pastikan kondisinya sudah ditangani.`
    : dasar;
}
