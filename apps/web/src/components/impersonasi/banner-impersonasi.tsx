"use client";

import { ArrowRight, ShieldAlert } from "lucide-react";
import { useSession } from "@/lib/auth/session-context";
import { ROLES } from "@/lib/rbac/roles";
import { labelPeranAsal, useKembaliPeranAsal } from "./tombol-kembali";

/** Tinggi tetap banner; ConsoleShell memakainya sebagai offset sticky sidebar dan topbar (`--bar-atas`). */
export const TINGGI_BANNER = "2.25rem";

/**
 * Bar informasi di PALING ATAS viewport selama impersonasi: dari akun dan peran asal ke akun dan peran aktif,
 * dengan aksi cepat kembali. Sticky dan berwarna peringatan (kontras tinggi) supaya tidak mungkin terlewat bahwa
 * sesi ini bukan akun sendiri. Tinggi tetap satu baris (teks dipotong di layar sempit; isi lengkap ada di `title`
 * dan teks layar-pembaca), sehingga offset sticky di bawahnya selalu pas.
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
      style={{ height: TINGGI_BANNER }}
      className="sticky top-0 z-40 flex items-center gap-2.5 bg-waspada px-3 text-xs text-navy-950 shadow-ambient-xs sm:px-5"
    >
      <ShieldAlert className="size-4 shrink-0" strokeWidth={2} aria-hidden />
      <p className="min-w-0 flex-1 truncate" title={`Sedang Impersonate: ${asal} → ${target}`}>
        <span className="font-semibold">Sedang Impersonate:</span>{" "}
        <b className="font-semibold">{asal}</b>
        <ArrowRight className="mx-1.5 inline size-3.5 align-[-2px]" strokeWidth={2} aria-label="menjadi" />
        <b className="font-semibold">{target}</b>
      </p>
      <button
        type="button"
        onClick={kembali}
        disabled={sibuk}
        aria-label="Kembali ke Peran Asal"
        title={`Kembali ke ${labelPeranAsal(imp)}`}
        className="inline-flex h-6 shrink-0 items-center rounded-md bg-navy-950 px-2.5 text-[11px] font-semibold text-white transition-colors outline-none hover:bg-navy-800 focus-visible:ring-2 focus-visible:ring-navy-950/60 focus-visible:ring-offset-1 focus-visible:ring-offset-waspada disabled:pointer-events-none disabled:opacity-60"
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
