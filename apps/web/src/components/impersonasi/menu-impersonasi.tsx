"use client";

import { Undo2, UserCog } from "lucide-react";
import { DropdownMenuItem, DropdownMenuSeparator } from "@/components/ui/dropdown-menu";
import { MODE_MOCK } from "@/lib/api/client";
import { useSession } from "@/lib/auth/session-context";
import { useTargetImpersonasi } from "@/lib/query/impersonasi-hooks";
import { labelPeranAsal, useKembaliPeranAsal } from "./tombol-kembali";

/**
 * Apakah menu "Beralih / Impersonate Role" ditampilkan: selama impersonasi aktif (untuk ganti sasaran atau
 * kembali), bagi Administrator Sistem (superadmin; dialog menjelaskan bila kebijakan nonaktif), dan bagi peran
 * lain hanya bila API menjawab `boleh: true` (di-allowlist kebijakan). Pertanyaan ke API hanya diajukan peran
 * lain, satu kali dan di-cache. Mode demo tanpa API tidak punya impersonasi.
 */
export function useImpersonasiTersedia(): boolean {
  const sesi = useSession();
  const sedangImpersonate = sesi.impersonation?.active === true;
  const superadmin = sesi.role === "ADMIN_SISTEM";
  const { data } = useTargetImpersonasi(!sedangImpersonate && !superadmin);
  if (MODE_MOCK) return false;
  return sedangImpersonate || superadmin || data?.boleh === true;
}

/**
 * Item menu akun: "Beralih / Impersonate Role" (membuka dialog pemilih; dialognya dirender di luar menu oleh
 * pemanggil) dan, saat impersonasi aktif, "Kembali ke [Peran Asal]". Diakhiri pemisah; null bila tidak berhak
 * sehingga menu pengguna biasa tidak berubah sama sekali. `tersedia` (dari useImpersonasiTersedia) dihitung
 * pemanggil di luar menu: isi menu Radix baru dipasang saat menu dibuka, sehingga pertanyaan ke API di sini
 * terlambat dan itemnya muncul sesudah menu terlihat.
 */
export function ItemMenuImpersonasi({
  tersedia,
  onBuka,
}: {
  tersedia: boolean;
  onBuka: () => void;
}) {
  const imp = useSession().impersonation;
  const { kembali, sibuk } = useKembaliPeranAsal();
  if (!tersedia) return null;
  return (
    <>
      <DropdownMenuItem onSelect={onBuka} className="rounded-md">
        <UserCog className="size-4" /> Beralih / Impersonate Role
      </DropdownMenuItem>
      {imp?.active && (
        <DropdownMenuItem
          onSelect={(e) => {
            // Menu tetap terbuka selama permintaan berjalan; halaman dimuat penuh sesudahnya.
            e.preventDefault();
            kembali();
          }}
          disabled={sibuk}
          className="rounded-md"
        >
          <Undo2 className="size-4" /> Kembali ke {labelPeranAsal(imp)}
        </DropdownMenuItem>
      )}
      <DropdownMenuSeparator />
    </>
  );
}
