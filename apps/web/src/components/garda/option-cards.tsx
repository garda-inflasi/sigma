"use client";

import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export interface OptionCard<T extends string> {
  value: T;
  title: ReactNode;
  description?: ReactNode;
  icon?: LucideIcon;
  /** Kelas warna titik/ikon (mis. warna siklus). */
  accent?: string;
}

/**
 * Kartu pilihan tunggal (radio) dengan ikon dan indikator radio: untuk jenis/kategori di dalam modal.
 * Semantik `radiogroup`, panah memindahkan pilihan (perilaku bawaan input radio).
 */
export function OptionCards<T extends string>({
  options,
  value,
  onChange,
  name,
  label,
  columns = 3,
  className,
}: {
  options: OptionCard<T>[];
  value: T;
  onChange: (v: T) => void;
  name: string;
  label: string;
  columns?: 2 | 3;
  className?: string;
}) {
  return (
    <div role="radiogroup" aria-label={label} className={cn("grid gap-2", columns === 3 ? "sm:grid-cols-3" : "sm:grid-cols-2", className)}>
      {options.map((o) => {
        const active = o.value === value;
        const Ikon = o.icon;
        return (
          <label
            key={o.value}
            className={cn(
              "relative flex cursor-pointer flex-col gap-1.5 rounded-lg border bg-card p-3 text-left transition-[border-color,box-shadow,background-color] duration-150 focus-within:ring-2 focus-within:ring-ring/40",
              active ? "border-navy-800 shadow-ambient-sm dark:border-white" : "border-border hover:border-navy-300",
            )}
          >
            <input
              type="radio"
              name={name}
              value={o.value}
              checked={active}
              onChange={() => onChange(o.value)}
              className="peer sr-only"
            />
            <span className="flex items-center gap-2">
              {Ikon && <Ikon className={cn("size-4", o.accent ?? "text-muted-foreground")} strokeWidth={1.75} aria-hidden />}
              <span className="flex-1 text-[13px] font-medium">{o.title}</span>
              <span
                aria-hidden
                className={cn(
                  "grid size-4 place-items-center rounded-full border",
                  active ? "border-navy-800 dark:border-white" : "border-navy-300",
                )}
              >
                {active && <span className="size-2 rounded-full bg-navy-800 dark:bg-white" />}
              </span>
            </span>
            {o.description && <span className="text-xs leading-snug text-muted-foreground">{o.description}</span>}
          </label>
        );
      })}
    </div>
  );
}
