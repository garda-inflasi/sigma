"use client";

import { ArrowRight, Siren } from "lucide-react";
import Link from "next/link";
import { useSession } from "@/lib/auth/session-context";
import { useKritisBaru } from "@/lib/query/hooks";
import { can } from "@/lib/rbac/permissions";
import { PengelolaToastKritis } from "./ews-toast";

/**
 * Banner global peringatan KRITIS yang belum ditindaklanjuti, untuk semua pemegang `ews.lihat`. Tidak punya tombol
 * tutup: hilang hanya bila server menyatakan tidak ada lagi KRITIS berstatus BARU. Menempel di bawah topbar saat
 * dokumen digulir (mobile). Toast KRITIS dikelola dari data yang sama.
 */
export function BannerKritis() {
  const sesi = useSession();
  const bolehLihat = can(sesi, "ews.lihat");
  const { data } = useKritisBaru(bolehLihat);
  if (!bolehLihat || !data) return null;

  return (
    <>
      <PengelolaToastKritis alerts={data} bolehAkui={can(sesi, "ews.akui")} />
      {data.length > 0 && (
        <div
          role="alert"
          data-testid="banner-kritis"
          className="sticky top-[calc(var(--bar-atas,0px)+3.5rem)] z-10 flex shrink-0 items-center gap-2 bg-kritis px-3 py-1.5 text-[13px] font-medium text-white sm:px-5 lg:static"
        >
          <Siren aria-hidden className="size-4 shrink-0" />
          <span className="min-w-0 flex-1 truncate">
            <span className="num">{data.length.toLocaleString("id-ID")}</span>{" "}
            peringatan kritis belum ditindaklanjuti
          </span>
          <Link
            href="/ews?severity=KRITIS"
            className="inline-flex shrink-0 items-center gap-1 rounded-sm font-semibold underline underline-offset-2 outline-none focus-visible:ring-2 focus-visible:ring-white"
          >
            Tinjau <ArrowRight aria-hidden className="size-3.5" />
          </Link>
        </div>
      )}
    </>
  );
}
