"use client";

import { usePathname } from "next/navigation";
import { useEffect, type CSSProperties, type ReactNode } from "react";
import { PageEnter } from "@/components/garda/page-enter";
import { CommandPalette } from "@/components/command/command-palette";
import { EwsStreamMount } from "@/components/ews/use-ews-stream";
import {
  BannerImpersonasi,
  TINGGI_BANNER,
} from "@/components/impersonasi/banner-impersonasi";
import { useSession } from "@/lib/auth/session-context";
import { useSiklusAktif } from "@/lib/query/hooks";
import { can } from "@/lib/rbac/permissions";
import { useUi } from "@/lib/stores/ui-store";
import { Sidebar } from "./sidebar";
import { Topbar } from "./topbar";

/** Cangkang konsol: sidebar datar, topbar dengan remah roti, palet ⌘K, aliran EWS, siklus ambient. */
export function ConsoleShell({ children }: { children: ReactNode }) {
  const sesi = useSession();
  const pathname = usePathname();
  const { data } = useSiklusAktif(can(sesi, "dashboard.operasional"));
  useEffect(() => {
    useUi.persist.rehydrate();
  }, []);
  useEffect(() => {
    if (data) document.documentElement.dataset.siklus = data.siklus;
  }, [data]);

  // Selama impersonasi banner menempel di puncak viewport; sidebar dan topbar (sticky) turun sebesar tingginya
  // lewat variabel CSS `--bar-atas` supaya tidak tertutup.
  const impersonasi = sesi.impersonation?.active === true;

  return (
    <div
      style={
        impersonasi
          ? ({ "--bar-atas": TINGGI_BANNER } as CSSProperties)
          : undefined
      }
    >
      <BannerImpersonasi />
      {/* Desktop: cangkang setinggi viewport, halaman mengisi <main> dan panelnya menggulir sendiri. Mobile: gulir dokumen. */}
      <div className="flex min-h-[calc(100dvh-var(--bar-atas,0px))] lg:h-[calc(100dvh-var(--bar-atas,0px))] lg:overflow-hidden">
        <Sidebar />
        <div className="flex min-w-0 flex-1 flex-col lg:min-h-0">
          <Topbar />
          <main
            id="konten"
            className="mx-auto flex w-full max-w-[1600px] flex-1 flex-col px-3 pt-4 pb-28 sm:px-5 lg:min-h-0 lg:overflow-y-auto lg:px-4 lg:py-3"
          >
            {/* key = rute: setiap halaman masuk dengan fade + geser naik lembut. */}
            <PageEnter
              key={pathname}
              className="flex min-w-0 flex-1 flex-col lg:min-h-0 [&>*]:w-full [&>*]:min-w-0"
            >
              {children}
            </PageEnter>
          </main>
        </div>
      </div>
      <CommandPalette />
      <EwsStreamMount />
    </div>
  );
}
