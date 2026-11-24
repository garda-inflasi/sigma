"use client";

import { motion } from "motion/react";
import type { ReactNode } from "react";
import { spring } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";

export interface ViewTab<T extends string = string> {
  value: T;
  label: ReactNode;
  icon?: ReactNode;
  /** Jumlah item di tampilan itu (badge kecil). */
  count?: number;
}

/**
 * Tab tampilan (Tabel, Papan, Linimasa, …) di bawah judul halaman: garis bawah yang meluncur (layoutId), ikon opsional,
 * dan badge jumlah. Pola WAI-ARIA tablist; panah kiri/kanan memindahkan pilihan.
 */
export function ViewTabs<T extends string>({
  tabs,
  value,
  onChange,
  layoutId,
  label = "Tampilan",
  className,
}: {
  tabs: ViewTab<T>[];
  value: T;
  onChange: (v: T) => void;
  layoutId: string;
  label?: string;
  className?: string;
}) {
  return (
    <div
      role="tablist"
      aria-label={label}
      className={cn("no-scrollbar -mb-px flex gap-1 overflow-x-auto shadow-[inset_0_-1px_0_var(--border)]", className)}
      onKeyDown={(e) => {
        if (e.key !== "ArrowRight" && e.key !== "ArrowLeft") return;
        const i = tabs.findIndex((t) => t.value === value);
        const n = tabs.length;
        const next = tabs[(i + (e.key === "ArrowRight" ? 1 : n - 1)) % n];
        e.preventDefault();
        onChange(next.value);
      }}
    >
      {tabs.map((t) => {
        const active = t.value === value;
        return (
          <button
            key={t.value}
            type="button"
            role="tab"
            aria-selected={active}
            tabIndex={active ? 0 : -1}
            onClick={() => onChange(t.value)}
            className={cn(
              "relative flex h-9 shrink-0 items-center gap-1.5 rounded-t-md px-3 text-[13px] font-medium whitespace-nowrap transition-colors focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:outline-none",
              active ? "text-foreground" : "text-muted-foreground hover:text-foreground",
            )}
          >
            {t.icon && <span className="[&_svg]:size-3.5" aria-hidden>{t.icon}</span>}
            {t.label}
            {t.count !== undefined && (
              <span className="num rounded bg-secondary px-1.5 text-[11px] font-medium text-muted-foreground">
                {t.count.toLocaleString("id-ID")}
              </span>
            )}
            {active && (
              <motion.span
                layoutId={layoutId}
                transition={spring.layout}
                className="absolute inset-x-1 bottom-0 h-0.5 rounded-full bg-navy-800 dark:bg-white"
              />
            )}
          </button>
        );
      })}
    </div>
  );
}
