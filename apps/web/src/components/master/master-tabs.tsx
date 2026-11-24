"use client";

import { Inbox } from "lucide-react";
import { motion } from "motion/react";
import Link from "next/link";
import { useEffect, useRef } from "react";
import { useCan } from "@/lib/auth/session-context";
import { ENTITAS, type Entitas } from "@/lib/data/master";
import { spring } from "@/lib/motion/presets";
import { useJumlahUsulanMenunggu } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";

const LABEL_TAB: Partial<Record<Entitas, string>> = { wilayah: "Wilayah" };

/**
 * Tab entitas master plus antrean persetujuan. Antrean adalah satu-satunya pintu masuk ke `/master/perubahan`, jadi
 * ia berada di luar strip yang bergulir agar selalu terlihat; lencana jumlah usulan hanya untuk pemegang master.setujui.
 */
export function MasterTabs({ aktif }: { aktif: Entitas | "perubahan" }) {
  const bisaSetujui = useCan("master.setujui");
  const menunggu = useJumlahUsulanMenunggu(bisaSetujui).data ?? 0;
  const gulirRef = useRef<HTMLDivElement>(null);
  const diAntrean = aktif === "perubahan";
  const adaUsulan = menunggu > 0;

  // Tiap entitas adalah rute sendiri sehingga strip dirender ulang dari kiri; di layar sempit tab aktif bisa tersembunyi.
  useEffect(() => {
    const wadah = gulirRef.current;
    const tab = wadah?.querySelector<HTMLElement>('[aria-current="page"]');
    if (wadah && tab)
      wadah.scrollLeft =
        tab.offsetLeft - (wadah.clientWidth - tab.offsetWidth) / 2;
  }, [aktif]);

  return (
    <nav
      aria-label="Entitas master"
      className="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center"
    >
      <div
        ref={gulirRef}
        className="no-scrollbar relative flex min-w-0 gap-0.5 overflow-x-auto shadow-[inset_0_-1px_0_var(--border)] sm:flex-1 lg:flex-none"
      >
        {(Object.keys(ENTITAS) as Entitas[]).map((e) => (
          <Link
            key={e}
            href={`/master/${e}`}
            title={ENTITAS[e]}
            aria-current={e === aktif ? "page" : undefined}
            className={cn(
              "relative flex h-9 shrink-0 items-center rounded-t-md px-2.5 text-[13px] font-medium whitespace-nowrap transition-colors focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:outline-none",
              e === aktif
                ? "text-foreground"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            {LABEL_TAB[e] ?? ENTITAS[e]}
            {e === aktif && (
              <motion.span
                layoutId="master-tab"
                transition={spring.layout}
                className="absolute inset-x-1 bottom-0 h-0.5 rounded-full bg-navy-800 dark:bg-white"
              />
            )}
          </Link>
        ))}
      </div>
      <Link
        href="/master/perubahan"
        aria-current={diAntrean ? "page" : undefined}
        className={cn(
          "flex h-9 shrink-0 items-center justify-between gap-2 rounded-md border px-3 text-[13px] font-medium whitespace-nowrap transition-colors focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:outline-none sm:justify-start",
          diAntrean
            ? "border-navy-800 bg-navy-800 text-white dark:border-white dark:bg-white dark:text-navy-900"
            : adaUsulan
              ? "border-waspada/40 bg-waspada-soft text-waspada-ink hover:bg-waspada-soft/70"
              : "border-border bg-card text-foreground hover:bg-secondary",
        )}
      >
        <span className="inline-flex items-center gap-1.5">
          <Inbox className="size-3.5" aria-hidden />
          Antrean perubahan
        </span>
        {adaUsulan && (
          <span
            className={cn(
              "num inline-grid h-5 min-w-5 place-items-center rounded-full px-1.5 text-[11px] leading-none font-semibold",
              diAntrean
                ? "bg-white text-navy-900 dark:bg-navy-900 dark:text-white"
                : "bg-card text-waspada-ink ring-1 ring-waspada/30",
            )}
          >
            {menunggu > 99 ? "99+" : menunggu}
            <span className="sr-only"> usulan menunggu</span>
          </span>
        )}
      </Link>
    </nav>
  );
}
