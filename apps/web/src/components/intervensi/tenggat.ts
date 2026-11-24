import { differenceInCalendarDays } from "date-fns";
import { MODE_MOCK } from "@/lib/api/client";
import { ANCHOR_DATE } from "@/lib/data/generators";
import type { KartuIntervensi } from "@/lib/data/types";

/** Data mock dibuat relatif terhadap ANCHOR_DATE; data server harus dibandingkan dengan tanggal hari ini. */
export function hariKeTenggat(iso: string) {
  return differenceInCalendarDays(
    new Date(iso),
    MODE_MOCK ? ANCHOR_DATE : new Date(),
  );
}

export const kartuSelesai = (k: Pick<KartuIntervensi, "tahap">) =>
  k.tahap.endsWith("selesai");

export const kartuTerlambat = (k: Pick<KartuIntervensi, "tahap" | "tenggat">) =>
  !kartuSelesai(k) && hariKeTenggat(k.tenggat) < 0;

export function labelTenggat(k: Pick<KartuIntervensi, "tahap" | "tenggat">) {
  if (kartuSelesai(k))
    return { text: "Selesai", cls: "text-normal-ink dark:text-emerald-300" };
  const hari = hariKeTenggat(k.tenggat);
  if (hari < 0)
    return {
      text: `Terlambat ${Math.abs(hari)} hari`,
      cls: "text-defisit-ink dark:text-rose-300",
    };
  if (hari === 0)
    return {
      text: "Tenggat hari ini",
      cls: "text-waspada-ink dark:text-amber-300",
    };
  return { text: `${hari} hari lagi`, cls: "text-muted-foreground" };
}
