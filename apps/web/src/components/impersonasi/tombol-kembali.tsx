"use client";

import { Undo2 } from "lucide-react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { ImpersonasiSesi } from "@/lib/auth/impersonasi";
import { useSession } from "@/lib/auth/session-context";
import { useAlihAkun } from "@/lib/query/impersonasi-hooks";
import { ROLES } from "@/lib/rbac/roles";
import { cn } from "@/lib/utils";

/**
 * Kembali ke akun asli (POST /api/impersonation/stop). Akun asli ditentukan server dari catatan sesi, bukan dari
 * klien. `sibuk` menahan klik ganda selama permintaan dan pemindahan halaman berjalan; galat tampil sebagai toast
 * (dari hook) dan tombol aktif lagi supaya dapat dicoba ulang.
 */
export function useKembaliPeranAsal() {
  const { berhenti, sibuk } = useAlihAkun();
  return {
    sibuk,
    kembali: () => {
      if (!sibuk) berhenti.mutate();
    },
  };
}

/** Label peran asal untuk tombol, mis. "Administrator Sistem". */
export function labelPeranAsal(imp: ImpersonasiSesi): string {
  return ROLES[imp.originalUser.role].label;
}

/**
 * Chip "Kembali ke [Peran Asal]" di topbar, tepat di sebelah lonceng dan avatar, hanya selama impersonasi aktif:
 * satu klik kembali tanpa membuka menu akun. Tooltip menjelaskan akun yang dituju.
 */
export function TombolKembaliPeran({ className }: { className?: string }) {
  const imp = useSession().impersonation;
  if (!imp?.active) return null;
  return <ChipKembali imp={imp} className={className} />;
}

function ChipKembali({ imp, className }: { imp: ImpersonasiSesi; className?: string }) {
  const { kembali, sibuk } = useKembaliPeranAsal();
  const peran = labelPeranAsal(imp);
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          onClick={kembali}
          disabled={sibuk}
          aria-label={`Kembali ke ${peran}`}
          data-testid="chip-kembali-peran"
          className={cn(
            "inline-flex h-8 shrink-0 items-center gap-1.5 rounded-md border border-waspada/60 bg-waspada-soft px-2.5 text-xs font-medium text-waspada-ink transition-colors outline-none hover:bg-waspada/20 focus-visible:ring-2 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-60 dark:bg-waspada/15 dark:text-amber-300",
            className,
          )}
        >
          <Undo2 className="size-3.5 shrink-0" strokeWidth={2} aria-hidden />
          <span aria-hidden className="hidden max-w-44 truncate md:inline">
            {sibuk ? "Mengembalikan…" : `Kembali ke ${peran}`}
          </span>
          <span aria-hidden className="md:hidden">
            {sibuk ? "…" : "Kembali"}
          </span>
        </button>
      </TooltipTrigger>
      <TooltipContent>
        Akhiri impersonasi dan kembali ke akun {imp.originalUser.name} ({peran}). Semua aksi selama impersonasi
        tetap tercatat atas nama akun asli.
      </TooltipContent>
    </Tooltip>
  );
}
