import { toneClass, type Tone } from "@/lib/design/tokens";
import { cn } from "@/lib/utils";

export function StatusBadge({ tone, children, dot = true, className }: { tone: Tone; children: React.ReactNode; dot?: boolean; className?: string }) {
  const t = toneClass[tone];
  return (
    <span className={cn("inline-flex h-6 items-center gap-1.5 rounded-full px-2.5 text-xs font-medium", t.soft, t.text, className)}>
      {dot && <span className={cn("size-1.5 rounded-full", t.dot)} />}
      {children}
    </span>
  );
}

/** Peta status domain → tone + label Indonesia. */
export const STATUS_ENTRI: Record<string, { tone: Tone; label: string }> = {
  DRAF: { tone: "netral", label: "Draf" },
  DIAJUKAN: { tone: "waspada", label: "Menunggu validasi" },
  DIVALIDASI: { tone: "data", label: "Divalidasi" },
  DIPUBLIKASI: { tone: "normal", label: "Dipublikasi" },
  DITOLAK: { tone: "defisit", label: "Ditolak" },
};

export const STATUS_EWS: Record<string, { tone: Tone; label: string }> = {
  BARU: { tone: "defisit", label: "Baru" },
  DIAKUI: { tone: "data", label: "Diakui" },
  DIESKALASI: { tone: "waspada", label: "Dieskalasi" },
  SELESAI: { tone: "normal", label: "Selesai" },
};
