"use client";

import { motion } from "motion/react";
import { spring } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";

interface Option<T extends string> {
  value: T;
  label: React.ReactNode;
  /** Kelas pill aktif (mis. warna siklus) */
  activeClass?: string;
}

/** Kontrol segmen dengan pill aktif yang meluncur (layoutId). */
export function Segmented<T extends string>({
  options, value, onChange, layoutId, size = "md", className,
}: { options: Option<T>[]; value: T; onChange: (v: T) => void; layoutId: string; size?: "sm" | "md"; className?: string }) {
  return (
    <div role="tablist" className={cn("inline-flex rounded-lg border border-border bg-secondary p-0.5", className)}>
      {options.map((o) => {
        const active = o.value === value;
        return (
          <button type="button"
            key={o.value}
            role="tab"
            aria-selected={active}
            onClick={() => onChange(o.value)}
            className={cn(
              "relative flex-1 whitespace-nowrap rounded-md font-medium transition-colors",
              size === "sm" ? "h-7 px-2.5 text-xs" : "h-8 px-3 text-[13px]",
              active ? (o.activeClass ? "text-white" : "text-foreground") : "text-muted-foreground hover:text-foreground",
            )}
          >
            {active && (
              <motion.span layoutId={layoutId} transition={spring.layout} className={cn("absolute inset-0 rounded-md border border-border bg-card shadow-ambient-xs", o.activeClass)} />
            )}
            <span className="relative z-10 inline-flex items-center gap-1.5">{o.label}</span>
          </button>
        );
      })}
    </div>
  );
}
