"use client";

import { create } from "zustand";
import type { KartuIntervensi } from "@/lib/data/types";
import { SIKLUS_TAHAP } from "@/lib/data/siklus";

interface KanbanState {
  ready: boolean;
  cards: Record<string, KartuIntervensi>;
  order: Record<string, string[]>; // tahapId -> id kartu terurut
  selected: string[];
  /** Memuat atau mengganti seluruh papan dengan data server (juga saat polling); pilihan kartu yang masih ada dipertahankan. */
  sync: (cards: KartuIntervensi[]) => void;
  move: (id: string, toTahap: string, toIndex: number) => void;
  bulkMove: (ids: string[], toTahap: string) => void;
  toggleSelect: (id: string) => void;
  selectMany: (ids: string[]) => void;
  clearSelection: () => void;
  snapshot: () => Pick<KanbanState, "cards" | "order">;
  restore: (s: Pick<KanbanState, "cards" | "order">) => void;
}

function susun(list: KartuIntervensi[]) {
  const order: Record<string, string[]> = {};
  Object.values(SIKLUS_TAHAP).flat().forEach((t) => (order[t.id] = []));
  list.forEach((c) => order[c.tahap]?.push(c.id));
  return { cards: Object.fromEntries(list.map((c) => [c.id, c])), order };
}

export const useKanban = create<KanbanState>()((set, get) => ({
  ready: false,
  cards: {},
  order: {},
  selected: [],
  sync: (list) => {
    const baru = susun(list);
    set((s) => ({ ready: true, ...baru, selected: s.selected.filter((id) => id in baru.cards) }));
  },
  move: (id, toTahap, toIndex) =>
    set((s) => {
      const from = s.cards[id]?.tahap;
      if (!from) return s;
      const order = { ...s.order, [from]: s.order[from].filter((x) => x !== id) };
      const dest = [...(order[toTahap] ?? [])];
      dest.splice(Math.max(0, Math.min(toIndex, dest.length)), 0, id);
      order[toTahap] = dest;
      return { order, cards: { ...s.cards, [id]: { ...s.cards[id], tahap: toTahap } } };
    }),
  bulkMove: (ids, toTahap) => ids.forEach((id) => get().move(id, toTahap, Number.MAX_SAFE_INTEGER)),
  toggleSelect: (id) => set((s) => ({ selected: s.selected.includes(id) ? s.selected.filter((x) => x !== id) : [...s.selected, id] })),
  selectMany: (ids) => set({ selected: ids }),
  clearSelection: () => set({ selected: [] }),
  snapshot: () => ({ cards: get().cards, order: get().order }),
  restore: (snap) => set(snap),
}));
