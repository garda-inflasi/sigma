import type { Tone } from "@/lib/design/tokens";
import type { MasterKomoditasRow } from "@/lib/data/types";

export const STATUS_KOMODITAS: Record<
  MasterKomoditasRow["status"],
  { tone: Tone; label: string }
> = {
  AKTIF: { tone: "normal", label: "Aktif" },
  MENUNGGU_PERSETUJUAN: { tone: "waspada", label: "Menunggu persetujuan" },
  DIARSIPKAN: { tone: "netral", label: "Diarsipkan" },
};

export const STATUS_PASAR: Record<
  "AKTIF" | "NONAKTIF",
  { tone: Tone; label: string }
> = {
  AKTIF: { tone: "normal", label: "Aktif" },
  NONAKTIF: { tone: "netral", label: "Nonaktif" },
};
