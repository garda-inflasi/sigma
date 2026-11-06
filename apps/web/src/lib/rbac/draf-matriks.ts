/**
 * Draf suntingan matriks akses yang belum disimpan (02-C). Disimpan di luar komponen supaya tidak hilang diam-diam
 * saat admin berpindah tab "Daftar pengguna" ↔ "Matriks akses" di /pengguna (komponen matriks dilepas). Setiap
 * draf mencatat versi peran yang menjadi dasarnya: bila versi di server sudah lain (admin lain menyimpan lebih
 * dulu) draf itu usang dan dibuang, bukan disimpan menimpa perubahan mereka.
 */

import { useSyncExternalStore } from "react";
import type { Role } from "./roles";

export interface DrafPeran {
  /** Versi peran di server saat draf dimulai. */
  versi: number;
  izin: ReadonlySet<string>;
}

export type DrafMatriks = ReadonlyMap<Role, DrafPeran>;

export function buatStoreDraf() {
  let state: DrafMatriks = new Map();
  const pendengar = new Set<() => void>();
  const ubah = (baru: Map<Role, DrafPeran>) => {
    state = baru;
    pendengar.forEach((p) => p());
  };
  return {
    ambil: (): DrafMatriks => state,
    atur(peran: Role, draf: DrafPeran) {
      ubah(new Map(state).set(peran, draf));
    },
    hapus(peran: Role) {
      if (!state.has(peran)) return;
      const baru = new Map(state);
      baru.delete(peran);
      ubah(baru);
    },
    /** Membuang draf yang versi dasarnya tidak sama dengan versi di server; mengembalikan peran yang dibuang. */
    buangUsang(versiServer: ReadonlyMap<Role, number>): Role[] {
      const usang = [...state]
        .filter(
          ([peran, d]) =>
            versiServer.has(peran) && versiServer.get(peran) !== d.versi,
        )
        .map(([peran]) => peran);
      if (usang.length === 0) return [];
      const baru = new Map(state);
      usang.forEach((p) => baru.delete(p));
      ubah(baru);
      return usang;
    },
    bersihkan() {
      if (state.size === 0) return;
      ubah(new Map());
    },
    langgan(p: () => void) {
      pendengar.add(p);
      return () => {
        pendengar.delete(p);
      };
    },
  };
}

/** Store tunggal untuk halaman /pengguna. */
export const drafMatriks = buatStoreDraf();

const kosong: DrafMatriks = new Map();

export function useDrafMatriks(): DrafMatriks {
  return useSyncExternalStore(
    drafMatriks.langgan,
    drafMatriks.ambil,
    () => kosong,
  );
}
