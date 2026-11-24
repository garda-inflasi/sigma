"use client";

import { ArrowRight, ShieldAlert } from "lucide-react";
import { useSession } from "@/lib/auth/session-context";
import { ROLES } from "@/lib/rbac/roles";
import { labelPeranAsal, useKembaliPeranAsal } from "./tombol-kembali";

/** Tinggi minimum default banner; ConsoleShell memakainya sebagai fallback offset `--bar-atas`. */
export const TINGGI_BANNER = "2.25rem";

/**
 * Bar informasi di PALING ATAS viewport selama impersonasi: dari akun dan peran asal ke akun dan peran aktif,
 * dengan aksi cepat kembali. Di mobile menempel di puncak viewport saat dokumen digulir; di desktop mengalir sebagai
 * baris pertama shell (`lg:static shrink-0`) di atas sidebar dan topbar sehingga tidak menggeser atau memotong konten.
 */
export function BannerImpersonasi() {
  const imp = useSession().impersonation;
  const { kembali, sibuk } = useKembaliPeranAsal();
  if (!imp?.active) return null;

  const asal = `${imp.originalUser.name} (${ROLES[imp.originalUser.role].label})`;
  const target = `${imp.targetUser.name} (${ROLES[imp.targetUser.role].label})`;

  return (
    <div
      role="status"
      aria-live="polite"
      data-testid="banner-impersonasi"
      style={{ minHeight: TINGGI_BANNER }}
      className="sticky top-0 z-40 flex shrink-0 items-center justify-between gap-2.5 bg-waspada px-3 py-1 text-xs text-navy-950 shadow-ambient-xs sm:px-5 lg:static"
    >
      <div className="flex min-w-0 flex-1 items-center gap-2">
        <ShieldAlert className="size-4 shrink-0" strokeWidth={2} aria-hidden />
        <p
          className="min-w-0 flex-1 truncate leading-snug"
          title={`Sedang Impersonate: ${asal} → ${target}`}
        >
          <span className="font-semibold">Sedang Impersonate:</span>{" "}
          <b className="font-semibold">{asal}</b>
          <ArrowRight
            className="mx-1.5 inline size-3.5 align-[-2px]"
            strokeWidth={2}
            aria-label="menjadi"
          />
          <b className="font-semibold">{target}</b>
        </p>
      </div>
      <button
        type="button"
        onClick={kembali}
        disabled={sibuk}
        aria-label="Kembali ke Peran Asal"
        title={`Kembali ke ${labelPeranAsal(imp)}`}
        className="inline-flex h-6 shrink-0 items-center whitespace-nowrap rounded-md bg-navy-950 px-2.5 text-[11px] font-semibold text-white transition-colors outline-none hover:bg-navy-800 focus-visible:ring-2 focus-visible:ring-navy-950/60 focus-visible:ring-offset-1 focus-visible:ring-offset-waspada disabled:pointer-events-none disabled:opacity-60"
      >
        <span aria-hidden className="hidden sm:inline">
          {sibuk ? "Mengembalikan…" : "Kembali ke Peran Asal"}
        </span>
        <span aria-hidden className="sm:hidden">
          {sibuk ? "…" : "Kembali"}
        </span>
      </button>
    </div>
  );
}
