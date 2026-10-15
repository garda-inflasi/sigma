"use client";

import { useEffect, type ReactNode } from "react";
import { CommandPalette } from "@/components/command/command-palette";
import { EwsStreamMount } from "@/components/ews/use-ews-stream";
import { useSiklusAktif } from "@/lib/query/hooks";
import { useUi } from "@/lib/stores/ui-store";
import { Sidebar } from "./sidebar";
import { Topbar } from "./topbar";

/** Cangkang konsol: sidebar kaca, topbar, palet ⌘K, aliran EWS, siklus ambient. */
export function ConsoleShell({ children }: { children: ReactNode }) {
  const { data } = useSiklusAktif();
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
        <main id="konten" className="flex-1 px-3 pt-6 pb-28 sm:px-5 lg:pr-6 lg:pl-6">
          {children}
        </main>
      </div>
      <CommandPalette />
      <EwsStreamMount />
    </div>
  );
}
