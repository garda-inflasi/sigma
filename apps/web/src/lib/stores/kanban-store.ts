"use client";

import { create } from "zustand";
import type { KartuIntervensi } from "@/lib/data/types";

interface KanbanState {
  ready: boolean;
  cards: Record<string, KartuIntervensi>;
  order: Record<string, string[]>; // tahapId -> id kartu terurut
  selected: string[];
  /**
   * Memuat atau mengganti seluruh papan dengan data server (juga saat polling); pilihan kartu yang masih ada
   * dipertahankan. `tahapIds` (dari GET /v1/intervensi/tahap) menjamin kolom kosong tetap ada sebagai sasaran drop.
   */
  sync: (cards: KartuIntervensi[], tahapIds?: string[]) => void;
  move: (id: string, toTahap: string, toIndex: number) => void;
  /** Mencatat versi baru kartu setelah server menyimpan pemindahan. */
  setVersi: (id: string, versi: number) => void;
  bulkMove: (ids: string[], toTahap: string) => void;
  toggleSelect: (id: string) => void;
  selectMany: (ids: string[]) => void;
  clearSelection: () => void;
  snapshot: () => Pick<KanbanState, "cards" | "order">;
  restore: (s: Pick<KanbanState, "cards" | "order">) => void;
}

function susun(list: KartuIntervensi[], tahapIds: string[]) {
  const order: Record<string, string[]> = {};
  tahapIds.forEach((id) => (order[id] = []));
  list.forEach((c) => order[c.tahap]?.push(c.id));
  return { cards: Object.fromEntries(list.map((c) => [c.id, c])), order };
}

export const useKanban = create<KanbanState>()((set, get) => ({
  ready: false,
  cards: {},
  order: {},
  selected: [],
  sync: (list, tahapIds) => {
    // Tanpa daftar tahap, kolom diturunkan dari kartu yang ada.
    const baru = susun(list, tahapIds ?? [...new Set(list.map((c) => c.tahap))]);
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
  setVersi: (id, versi) => set((s) => (s.cards[id] ? { cards: { ...s.cards, [id]: { ...s.cards[id], versi } } } : s)),
  bulkMove: (ids, toTahap) => ids.forEach((id) => get().move(id, toTahap, Number.MAX_SAFE_INTEGER)),
  toggleSelect: (id) => set((s) => ({ selected: s.selected.includes(id) ? s.selected.filter((x) => x !== id) : [...s.selected, id] })),
  selectMany: (ids) => set({ selected: ids }),
  clearSelection: () => set({ selected: [] }),
  snapshot: () => ({ cards: get().cards, order: get().order }),
  restore: (snap) => set(snap),
}));
