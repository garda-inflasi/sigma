"use client";

import { useMutation, useQuery, useQueryClient, type QueryKey } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, bulk } from "@/lib/api/endpoints";
import { qk } from "./keys";

const LIVE = 30_000;

export const useSiklusAktif = () => useQuery({ queryKey: qk.siklus(), queryFn: api.siklus.aktif, staleTime: 5 * 60_000 });
export const useKpiOperasional = () => useQuery({ queryKey: qk.operasional.kpi(), queryFn: api.operasional.kpi, refetchInterval: LIVE });
export const useHeatmap = () => useQuery({ queryKey: qk.operasional.heatmap(), queryFn: api.operasional.heatmap, refetchInterval: LIVE * 2 });
export const useMarkerWilayah = () => useQuery({ queryKey: qk.operasional.marker(), queryFn: api.operasional.marker });
export const useTrenHarga = (kode: string, kab: string) =>
  useQuery({ queryKey: qk.operasional.tren(kode, kab), queryFn: () => api.operasional.tren(kode, kab), placeholderData: (prev) => prev });
export const useRingkasanEksekutif = () => useQuery({ queryKey: qk.eksekutif.ringkasan(), queryFn: api.eksekutif.ringkasan });
export const useXai = (kode: string) => useQuery({ queryKey: qk.eksekutif.xai(kode), queryFn: () => api.eksekutif.xai(kode), placeholderData: (prev) => prev });
export const useEntriHarga = () => useQuery({ queryKey: qk.harga.list(), queryFn: api.harga.list });
export const useEwsList = () => useQuery({ queryKey: qk.ews.list(), queryFn: api.ews.list });
export const useIntervensi = () => useQuery({ queryKey: qk.intervensi.list(), queryFn: api.intervensi.list, staleTime: Infinity });
export const useColdStorage = () => useQuery({ queryKey: qk.coldStorage.ringkasan(), queryFn: api.coldStorage.ringkasan });
export const useSumberData = () => useQuery({ queryKey: qk.sumber.list(), queryFn: api.sumber.list });
export const usePengguna = () => useQuery({ queryKey: qk.pengguna.list(), queryFn: api.pengguna.list });
export const useAudit = () => useQuery({ queryKey: qk.audit.list(), queryFn: api.audit.list });
export const useBriefs = () => useQuery({ queryKey: qk.brief.list(), queryFn: api.brief.list });
export const useTugasSaya = () => useQuery({ queryKey: qk.tugas.saya(), queryFn: api.tugas.saya });
export const useMasterKomoditas = () => useQuery({ queryKey: qk.master.komoditas(), queryFn: api.master.komoditas });

/**
 * Mutasi aksi massal dengan pembaruan optimistis.
 * `patch` mengubah baris terpilih di cache; rollback otomatis bila gagal.
 */
export function useBulkAction<T extends { id: string }>(opts: {
  sumber: string;
  queryKey: QueryKey;
  patch?: (row: T, aksi: string) => T | null; // null = hapus dari daftar
}) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ aksi, ids }: { aksi: string; ids: string[]; label: string }) => bulk(opts.sumber, aksi, ids),
    onMutate: async ({ aksi, ids }) => {
      await qc.cancelQueries({ queryKey: opts.queryKey });
      const prev = qc.getQueryData<T[]>(opts.queryKey);
      if (prev && opts.patch) {
        const set = new Set(ids);
        qc.setQueryData<T[]>(opts.queryKey, prev.flatMap((r) => (set.has(r.id) ? (opts.patch!(r, aksi) ?? []) : [r])));
      }
      return { prev };
    },
    onError: (err, _v, ctx) => {
      if (ctx?.prev) qc.setQueryData(opts.queryKey, ctx.prev);
      toast.error("Aksi massal gagal", { description: err instanceof Error ? err.message : "Coba lagi beberapa saat." });
    },
    onSuccess: (res, { label }) => {
      toast.success(`${label}: ${res.berhasil.toLocaleString("id-ID")} data`, {
        description: res.gagal ? `${res.gagal} data gagal diproses — lihat log audit.` : "Tercatat di audit trail.",
      });
    },
  });
}
