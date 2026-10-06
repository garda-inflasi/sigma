"use client";

import { useSortable } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { motion } from "motion/react";
import { CalendarClock, MapPin, Sparkles } from "lucide-react";
import { Checkbox } from "@/components/ui/checkbox";
import { KomoditasIcon } from "@/components/garda/komoditas-icon";
import { komoditasByKode } from "@/lib/data/reference";
import { namaWilayah } from "@/lib/data/generators";
import type { KartuIntervensi, Prioritas } from "@/lib/data/types";
import { ANCHOR_DATE } from "@/lib/data/generators";
import { spring } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";

const PRIORITAS: Record<Prioritas, { label: string; cls: string }> = {
  TINGGI: { label: "Prioritas tinggi", cls: "bg-defisit-soft text-defisit-ink dark:bg-defisit/15 dark:text-rose-300" },
  SEDANG: { label: "Sedang", cls: "bg-waspada-soft text-waspada-ink dark:bg-waspada/15 dark:text-amber-300" },
  RENDAH: { label: "Rendah", cls: "bg-mist-100 text-muted-foreground dark:bg-white/5" },
};

function tenggatLabel(iso: string) {
  const hari = Math.round((new Date(iso).getTime() - ANCHOR_DATE.getTime()) / 86_400_000);
  if (hari < 0) return { text: `Terlambat ${Math.abs(hari)} hari`, cls: "text-defisit-ink dark:text-rose-300" };
  if (hari === 0) return { text: "Tenggat hari ini", cls: "text-waspada-ink dark:text-amber-300" };
  return { text: `${hari} hari lagi`, cls: "text-muted-foreground" };
}

export function KanbanCardView({ card, selected, onToggle, overlay, dimmed }: { card: KartuIntervensi; selected?: boolean; onToggle?: () => void; overlay?: boolean; dimmed?: boolean }) {
  const t = tenggatLabel(card.tenggat);
  const p = PRIORITAS[card.prioritas];
  const pct = card.progres.total ? (card.progres.selesai / card.progres.total) * 100 : 0;
  return (
    <motion.div
      whileHover={overlay ? undefined : { y: -3 }}
      transition={spring.soft}
      animate={overlay ? { rotate: 2, scale: 1.03 } : { rotate: 0, scale: 1 }}
      className={cn(
        "glass relative rounded-xl p-3.5 text-left transition-shadow",
        overlay ? "cursor-grabbing shadow-ambient-lg" : "cursor-grab hover:shadow-lift",
        selected && "ring-2 ring-siklus",
        dimmed && "opacity-40 ring-2 ring-dashed ring-siklus/40",
      )}
    >
      <span className="absolute inset-y-3 left-0 w-[3px] rounded-r-full bg-siklus" style={{ opacity: card.prioritas === "TINGGI" ? 1 : card.prioritas === "SEDANG" ? 0.6 : 0.3 }} />
      <div className="flex items-center gap-2">
        {onToggle && (
          <span onPointerDown={(e) => e.stopPropagation()} onClick={(e) => e.stopPropagation()} className="inline-flex">
            <Checkbox checked={!!selected} onCheckedChange={onToggle} aria-label={`Pilih ${card.judul}`} className="rounded-md" />
          </span>
        )}
        <span className={cn("rounded-full px-2 py-0.5 text-[11px] font-medium", p.cls)}>{p.label}</span>
        {card.rekomendasiAi && (
          <span className="ml-auto inline-flex items-center gap-1 rounded-full bg-data-50 px-2 py-0.5 text-[11px] font-medium text-data-700 dark:bg-data-500/15 dark:text-data-300">
            <Sparkles className="size-3" /> AI
          </span>
        )}
      </div>
      <p className="mt-2.5 text-sm leading-snug font-medium">{card.judul}</p>
      <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
        <span className="inline-flex items-center gap-1"><KomoditasIcon kode={card.komoditasKode} className="size-3.5" />{komoditasByKode[card.komoditasKode]?.nama}</span>
        <span className="inline-flex items-center gap-1"><MapPin className="size-3.5" />{namaWilayah(card.kabupatenKode)}</span>
      </div>
      <div className="mt-3 flex items-center gap-2">
        <div className="h-1 flex-1 overflow-hidden rounded-full bg-mist-200 dark:bg-white/10">
          <div className="h-full rounded-full bg-siklus transition-[width] duration-500" style={{ width: `${pct}%` }} />
        </div>
        <span className="num text-[11px] text-muted-foreground">{card.progres.selesai}/{card.progres.total}</span>
      </div>
      <div className="mt-3 flex items-center justify-between">
        <div className="flex -space-x-1.5">
          {card.penanggungJawab.map((u) => (
            <span key={u.nama} title={u.nama} className="grid size-6 place-items-center rounded-full bg-navy-700 text-[10px] font-semibold text-white ring-2 ring-white dark:ring-navy-900">{u.inisial}</span>
          ))}
        </div>
        <span className={cn("inline-flex items-center gap-1 text-[11px] font-medium", t.cls)}><CalendarClock className="size-3.5" />{t.text}</span>
      </div>
    </motion.div>
  );
}

export function SortableKanbanCard({ card, selected, onToggle }: { card: KartuIntervensi; selected: boolean; onToggle: () => void }) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: card.id, data: { type: "card" } });
  return (
    <div ref={setNodeRef} style={{ transform: CSS.Transform.toString(transform), transition }} {...attributes} {...listeners} className="touch-none outline-none" aria-roledescription="Kartu intervensi yang dapat dipindahkan">
      <KanbanCardView card={card} selected={selected} onToggle={onToggle} dimmed={isDragging} />
    </div>
  );
}
