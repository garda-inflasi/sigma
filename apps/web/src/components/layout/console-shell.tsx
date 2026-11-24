"use client";

import { usePathname } from "next/navigation";
import { useEffect, useRef, type CSSProperties, type ReactNode } from "react";
import { PageEnter } from "@/components/garda/page-enter";
import { CommandPalette } from "@/components/command/command-palette";
import { BannerKritis } from "@/components/ews/banner-kritis";
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

  // Di mobile (<lg), banner menempel sticky di puncak viewport dan topbar mengikutinya lewat `--bar-atas`
  // yang diselaraskan ke tinggi aktual banner. Di desktop (`lg+`), cangkang flex-col setinggi `100dvh`
  // menempatkan banner sebagai baris pertama sehingga sidebar, topbar, dan <main> tidak saling menimpa.
  const impersonasi = sesi.impersonation?.active === true;
  const shellRef = useRef<HTMLDivElement>(null);
  const mainRef = useRef<HTMLElement>(null);

  useEffect(() => {
    const shell = shellRef.current;
    if (!shell) return;
    if (!impersonasi) {
      shell.style.removeProperty("--bar-atas");
      return;
    }

    const banner = shell.querySelector<HTMLElement>('[data-testid="banner-impersonasi"]');
    if (!banner) return;

    const sinkronTinggi = () => {
      const h = banner.getBoundingClientRect().height;
      if (h > 0) shell.style.setProperty("--bar-atas", `${h}px`);
    };
    sinkronTinggi();

    if (typeof ResizeObserver !== "function") return;
    const ro = new ResizeObserver(sinkronTinggi);
    ro.observe(banner);
    return () => ro.disconnect();
  }, [impersonasi]);

  useEffect(() => {
    if (mainRef.current) {
      mainRef.current.scrollTop = 0;
    }
  }, [pathname, impersonasi]);

  return (
    <div
      ref={shellRef}
      style={
        impersonasi
          ? ({ "--bar-atas": TINGGI_BANNER } as CSSProperties)
          : undefined
      }
      className="flex min-h-dvh flex-col lg:h-dvh lg:overflow-hidden"
    >
      <BannerImpersonasi />
      {/* Desktop: baris isi mengisi sisa tinggi viewport di bawah banner; Mobile: mengalir mengikuti gulir dokumen. */}
      <div className="flex min-h-0 flex-1">
        <Sidebar />
        <div className="flex min-w-0 flex-1 flex-col lg:min-h-0">
          <Topbar />
          <BannerKritis />
          <main
            ref={mainRef}
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
