"use client";

import { useInfiniteQuery } from "@tanstack/react-query";
import { MODE_MOCK } from "@/lib/api/client";
import {
  aktivitasApi,
  type FilterAktivitas,
  type FilterStatusSesi,
} from "@/lib/api/aktivitas";
import { qk } from "./keys";

/** Riwayat aktivitas akun sendiri, halaman demi halaman (kursor). Data keamanan: selalu segar saat dibuka. */
export function useAktivitasSaya(filter: FilterAktivitas) {
  return useInfiniteQuery({
    queryKey: qk.profil.aktivitas(filter),
    queryFn: ({ pageParam }) => aktivitasApi.aktivitas(filter, pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (terakhir) => terakhir.nextCursor,
    enabled: !MODE_MOCK,
    staleTime: 0,
  });
}

/** Riwayat sesi masuk sendiri (aktif dan berakhir), halaman demi halaman. */
export function useRiwayatSesi(status: FilterStatusSesi) {
  return useInfiniteQuery({
    queryKey: qk.profil.riwayatSesi(status),
    queryFn: ({ pageParam }) => aktivitasApi.riwayatSesi(status, pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (terakhir) => terakhir.nextCursor,
    enabled: !MODE_MOCK,
    staleTime: 0,
  });
}
