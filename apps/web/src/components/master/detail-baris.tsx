import { AlertTriangle } from "lucide-react";
import type { ReactNode } from "react";
import { InspectorSection } from "@/components/garda/inspector";
import { Shimmer } from "@/components/garda/shimmer";

/** Koordinat "lintang, bujur", paling banyak 5 desimal; nol di belakang dibuang agar tidak terkesan lebih presisi. */
export const formatKoordinat = (lat: number, lng: number) =>
  `${Number(lat.toFixed(5))}, ${Number(lng.toFixed(5))}`;

/** `InspectorSection` yang tetap menjadi region bernama; garis pemisah ada di pembungkus. */
export function SeksiBernama({
  label,
  title,
  children,
}: {
  label: string;
  title: string;
  children: ReactNode;
}) {
  return (
    <section
      aria-label={label}
      className="border-t border-border first:border-t-0"
    >
      <InspectorSection title={title}>{children}</InspectorSection>
    </section>
  );
}

/** Kerangka isi panel detail selama data dimuat. */
export function MemuatSheet() {
  return (
    <div
      role="status"
      aria-label="Memuat detail"
      className="space-y-3 px-5 py-4"
    >
      <Shimmer className="h-4 w-1/3" />
      <Shimmer className="h-24 w-full" />
      <Shimmer className="h-32 w-full" />
    </div>
  );
}

/**
 * Pesan satu bagian sheet. Varian galat setampilan `PanelGalat`, tetapi memakai pesan yang sudah dipetakan
 * `pesanGalatMaster` (403, 404, 5xx); `PanelGalat` selalu menurunkan keterangannya sendiri dari objek galat.
 */
export function CatatanSheet({
  children,
  galat,
}: {
  children: ReactNode;
  galat?: boolean;
}) {
  if (!galat)
    return <p className="text-xs text-muted-foreground">{children}</p>;
  return (
    <div
      role="alert"
      className="flex flex-col items-center justify-center gap-2 rounded-lg bg-defisit/5 px-4 py-8 text-center"
    >
      <span className="grid size-9 place-items-center rounded-full bg-defisit/10 text-defisit">
        <AlertTriangle className="size-4" aria-hidden />
      </span>
      <p className="text-sm font-medium text-foreground">{children}</p>
    </div>
  );
}
