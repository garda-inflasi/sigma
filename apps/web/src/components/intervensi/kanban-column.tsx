"use client";

import { useDroppable } from "@dnd-kit/core";
import { SortableContext, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { AnimatePresence, motion } from "motion/react";
import { Lock } from "lucide-react";
import type { KartuIntervensi } from "@/lib/data/types";
import type { Tahap } from "@/lib/data/siklus";
import { fadeIn } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";
import { SortableKanbanCard } from "./kanban-card";

export function KanbanColumn({
  tahap, index, total, cards, selected, onToggle, canDrop, isTarget, izinLabel,
}: {
  tahap: Tahap; index: number; total: number; cards: KartuIntervensi[]; selected: string[]; onToggle: (id: string) => void;
  canDrop: boolean; isTarget: boolean; izinLabel?: string;
}) {
  const { setNodeRef } = useDroppable({ id: tahap.id, data: { type: "column" } });
  const intensitas = 0.35 + 0.65 * (index / Math.max(1, total - 1));
  const terminal = tahap.id.endsWith("selesai");

  return (
    <div
      ref={setNodeRef}
      className={cn(
        "glass-subtle relative flex w-[296px] shrink-0 flex-col rounded-xl p-2.5 transition-[box-shadow,background-color] duration-200",
        isTarget && canDrop && "bg-siklus-soft/40 ring-2 ring-siklus/60 dark:bg-siklus/10",
        isTarget && !canDrop && "bg-defisit-soft/40 ring-2 ring-defisit/60",
      )}
    >
      <div className="px-2 pt-1.5 pb-3">
        <div className="flex items-center gap-2">
          <span className={cn("num grid size-6 place-items-center rounded-lg text-[11px] font-semibold", terminal ? "bg-navy-800 text-white dark:bg-white/15" : "text-white")} style={terminal ? undefined : { background: "var(--siklus)", opacity: intensitas }}>
            {terminal ? "✓" : index + 1}
          </span>
          <h3 className="flex-1 truncate font-sans text-sm font-semibold" title={tahap.judul}>{tahap.judul}</h3>
          {!canDrop && izinLabel && <Lock className="size-3.5 text-muted-foreground" aria-label={izinLabel} />}
          <span className="num rounded-full bg-white/70 px-2 text-xs text-muted-foreground dark:bg-white/10">{cards.length}</span>
        </div>
        <p className="mt-1 truncate pl-8 text-xs text-muted-foreground">{tahap.petunjuk}</p>
      </div>

      <SortableContext items={cards.map((c) => c.id)} strategy={verticalListSortingStrategy}>
        <div className="scrollbar-thin flex min-h-28 flex-1 flex-col gap-2.5 overflow-y-auto px-0.5 pb-1">
          {cards.map((c) => (
            <SortableKanbanCard key={c.id} card={c} selected={selected.includes(c.id)} onToggle={() => onToggle(c.id)} />
          ))}
          {cards.length === 0 && (
            <div className="grid min-h-24 place-items-center rounded-xl border border-dashed border-border text-xs text-muted-foreground">Tarik kartu ke sini</div>
          )}
        </div>
      </SortableContext>

      <AnimatePresence>
        {isTarget && !canDrop && (
          <motion.div variants={fadeIn} initial="hidden" animate="show" exit="exit" className="absolute inset-x-3 bottom-3 rounded-xl bg-defisit px-3 py-2 text-xs font-medium text-white shadow-glow-defisit">
            {izinLabel ?? "Peran Anda tidak dapat memindahkan kartu ke tahap ini"}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
