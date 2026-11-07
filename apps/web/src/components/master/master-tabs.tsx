"use client";

import { motion } from "motion/react";
import Link from "next/link";
import { useCan } from "@/lib/auth/session-context";
import { ENTITAS, type Entitas } from "@/lib/data/master";
import { spring } from "@/lib/motion/presets";
import { useJumlahUsulanMenunggu } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";

/** Tab entitas master plus antrean persetujuan; lencana jumlah usulan menunggu hanya untuk pemegang master.setujui. */
export function MasterTabs({ aktif }: { aktif: Entitas | "perubahan" }) {
  const bisaSetujui = useCan("master.setujui");
  const menunggu = useJumlahUsulanMenunggu(bisaSetujui).data ?? 0;
  const tab: { id: Entitas | "perubahan"; label: string }[] = [
    ...(Object.keys(ENTITAS) as Entitas[]).map((e) => ({
      id: e,
      label: ENTITAS[e],
    })),
    { id: "perubahan", label: "Antrean perubahan" },
  ];
  return (
    <nav
      className="glass-subtle flex max-w-full overflow-x-auto rounded-xl p-1 sm:inline-flex"
      aria-label="Entitas master"
    >
      {tab.map((t) => (
        <Link
          key={t.id}
          href={`/master/${t.id}`}
          aria-current={t.id === aktif ? "page" : undefined}
          className={cn(
            "relative h-9 shrink-0 rounded-lg px-4 text-sm font-medium leading-9 whitespace-nowrap",
            t.id === aktif
              ? "text-white"
              : "text-muted-foreground hover:text-foreground",
          )}
        >
          {t.id === aktif && (
            <motion.span
              layoutId="master-tab"
              transition={spring.layout}
              className="absolute inset-0 rounded-lg bg-navy-800 dark:bg-navy-600"
            />
          )}
          <span className="relative">{t.label}</span>
          {t.id === "perubahan" && menunggu > 0 && (
            <span className="num relative ml-2 inline-grid h-5 min-w-5 place-items-center rounded-full bg-waspada-soft px-1.5 align-middle text-[11px] leading-none font-semibold text-waspada-ink">
              {menunggu > 99 ? "99+" : menunggu}
            </span>
          )}
        </Link>
      ))}
    </nav>
  );
}
