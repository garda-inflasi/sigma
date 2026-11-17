"use client";

import { usePathname } from "next/navigation";
import { useEffect, type ReactNode } from "react";
import { PageEnter } from "@/components/garda/page-enter";
import { CommandPalette } from "@/components/command/command-palette";
import { EwsStreamMount } from "@/components/ews/use-ews-stream";
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

  return (
    <div className="flex min-h-dvh">
      <Sidebar />
      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar />
        <main id="konten" className="mx-auto w-full max-w-[1600px] flex-1 px-3 pt-5 pb-28 sm:px-5 lg:px-6">
          {/* key = rute: setiap halaman masuk dengan fade + geser naik lembut. */}
          <PageEnter key={pathname}>{children}</PageEnter>
        </main>
      </div>
      <CommandPalette />
      <EwsStreamMount />
    </div>
  );
}
