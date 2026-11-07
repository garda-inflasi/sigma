"use client";

import { useCallback, useMemo, useState } from "react";
import type { ServerTable } from "@/components/data-table/server-mode";

export interface StateTabelServer {
  search: string;
  /** Mulai dari 1. */
  page: number;
  limit: number;
  filter: Record<string, string | undefined>;
}

const AWAL: StateTabelServer = { search: "", page: 1, limit: 25, filter: {} };

/** Mengubah pencarian: kembali ke halaman 1 (hasil baru bisa lebih pendek dari halaman sekarang). */
export const ubahCari = (
  s: StateTabelServer,
  search: string,
): StateTabelServer => ({ ...s, search, page: 1 });
export const ubahFilter = (
  s: StateTabelServer,
  kunci: string,
  nilai: string | undefined,
): StateTabelServer => ({
  ...s,
  filter: { ...s.filter, [kunci]: nilai },
  page: 1,
});
export const ubahUkuran = (
  s: StateTabelServer,
  limit: number,
): StateTabelServer => ({ ...s, limit, page: 1 });
export const ubahHalaman = (
  s: StateTabelServer,
  page: number,
): StateTabelServer => ({ ...s, page: Math.max(1, page) });

/**
 * Keadaan tabel yang dijalankan server (pencarian, filter, halaman) untuk tab kecamatan dan pedagang. Disimpan di
 * state komponen, bukan URL. `tabel(...)` menyusun kontrak `ServerTable` untuk DataTable; urutan tetap (menurut
 * kode di server), jadi kepala kolom tidak bisa diurutkan.
 */
export function useTabelServer() {
  const [state, setState] = useState<StateTabelServer>(AWAL);
  const setCari = useCallback(
    (search: string) => setState((s) => ubahCari(s, search)),
    [],
  );
  const setFilter = useCallback(
    (kunci: string, nilai: string | undefined) =>
      setState((s) => ubahFilter(s, kunci, nilai)),
    [],
  );
  const kunci = useMemo(() => JSON.stringify(state), [state]);
  const tabel = useCallback(
    (total: number, sibuk: boolean): ServerTable => ({
      total,
      page: state.page,
      pageSize: state.limit,
      onPageChange: (page) => setState((s) => ubahHalaman(s, page)),
      onPageSizeChange: (limit) => setState((s) => ubahUkuran(s, limit)),
      search: state.search,
      onSearchChange: setCari,
      sorting: [],
      onSortToggle: () => {},
      kunciKueri: kunci,
      isFetching: sibuk,
    }),
    [state, kunci, setCari],
  );
  return { state, setFilter, tabel };
}
