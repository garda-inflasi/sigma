"use client";

import { motion } from "motion/react";
import { Check } from "lucide-react";
import type { ReactNode } from "react";
import { ease } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";

/** Bagian bertajuk di dalam drawer (Rincian, Checklist, Aktivitas). */
export function InspectorSection({ title, children, className }: { title: string; children: ReactNode; className?: string }) {
  return (
    <section className={cn("space-y-3 border-t border-border px-5 py-4 first:border-t-0", className)}>
      <h3 className="text-[11px] font-medium tracking-wide text-muted-foreground uppercase">{title}</h3>
      {children}
    </section>
  );
}

/** Grid metadata 2 kolom: label kecil di atas, nilai di bawah. */
export function MetaGrid({ children, className }: { children: ReactNode; className?: string }) {
  return <dl className={cn("grid grid-cols-2 gap-x-4 gap-y-3.5", className)}>{children}</dl>;
}

export function MetaItem({ label, children, wide }: { label: string; children: ReactNode; wide?: boolean }) {
  return (
    <div className={cn("min-w-0", wide && "col-span-2")}>
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className="mt-0.5 text-[13px] font-medium break-words text-foreground">{children}</dd>
    </div>
  );
}

export interface ChecklistItem {
  id: string;
  label: ReactNode;
  done: boolean;
}

/** Checklist dengan bilah progres tipis dan hitungan "n/m". */
export function ChecklistProgress({ items, label = "Progres" }: { items: ChecklistItem[]; label?: string }) {
  const selesai = items.filter((i) => i.done).length;
  const pct = items.length ? (selesai / items.length) * 100 : 0;
  return (
    <div className="space-y-3">
      <div className="flex items-center gap-3">
        <div
          role="progressbar"
          aria-label={label}
          aria-valuemin={0}
          aria-valuemax={items.length}
          aria-valuenow={selesai}
          className="h-1 flex-1 overflow-hidden rounded-full bg-secondary"
        >
          <motion.div
            className="h-full rounded-full bg-data-600"
            initial={{ width: 0 }}
            animate={{ width: `${pct}%` }}
            transition={{ duration: 0.5, ease: ease.garda }}
          />
        </div>
        <span className="num text-xs text-muted-foreground">
          {selesai}/{items.length}
        </span>
      </div>
      <ul className="space-y-2">
        {items.map((i) => (
          <li key={i.id} className="flex items-start gap-2.5 text-[13px]">
            <span
              aria-hidden
              className={cn(
                "mt-0.5 grid size-4 shrink-0 place-items-center rounded border",
                i.done ? "border-data-600 bg-data-600 text-white" : "border-navy-300 bg-card",
              )}
            >
              {i.done && <Check className="size-3" strokeWidth={2.5} />}
            </span>
            <span className={cn(i.done && "text-muted-foreground")}>{i.label}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

export interface TimelineEntry {
  id: string;
  title: ReactNode;
  meta?: ReactNode;
  note?: ReactNode;
  tone?: "data" | "defisit" | "netral";
}

/** Linimasa aktivitas/catatan: titik di garis vertikal, judul, meta kecil, catatan opsional. */
export function ActivityTimeline({ entries, empty = "Belum ada aktivitas.", label }: { entries: TimelineEntry[]; empty?: string; label?: string }) {
  if (entries.length === 0) return <p className="text-[13px] text-muted-foreground">{empty}</p>;
  return (
    <ol aria-label={label} className="relative space-y-4 pl-5">
      <span aria-hidden className="absolute top-1.5 bottom-1.5 left-[5px] w-px bg-border" />
      {entries.map((e) => (
        <li key={e.id} className="relative">
          <span
            aria-hidden
            className={cn(
              "absolute top-[5px] -left-5 size-[11px] rounded-full border-2 border-card",
              e.tone === "defisit" ? "bg-defisit" : e.tone === "netral" ? "bg-navy-300" : "bg-data-600",
            )}
          />
          <p className="text-[13px] font-medium">{e.title}</p>
          {e.meta && <p className="text-xs text-muted-foreground">{e.meta}</p>}
          {e.note && <p className="mt-1 rounded-md bg-secondary px-2.5 py-1.5 text-[13px] text-foreground/90">{e.note}</p>}
        </li>
      ))}
    </ol>
  );
}
