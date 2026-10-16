"use client";

import { useMutation, useQuery, useQueryClient, type QueryKey } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, bulk } from "@/lib/api/endpoints";
import { penggunaApi, type BulkAksi, type CreatePenggunaInput, type UpdatePenggunaInput } from "@/lib/api/pengguna";
import { MODE_MOCK, mockDelay } from "@/lib/api/client";
import { ROLES, type Role } from "@/lib/rbac/roles";
import { qk } from "./keys";

const LIVE = 30_000;

/**
 * Siklus aktif menggerakkan tema ambient. Hanya boleh dipanggil bila peran
 * memiliki `dashboard.operasional`; shell meneruskan hasil `can()` sebagai
 * `enabled` supaya peran tanpa izin tidak memicu 403 di setiap halaman.
 */
export const useSiklusAktif = (enabled = true) =>
  useQuery({ queryKey: qk.siklus(), queryFn: api.siklus.aktif, staleTime: 5 * 60_000, enabled });
export const useKpiOperasional = () => useQuery({ queryKey: qk.operasional.kpi(), queryFn: api.operasional.kpi, refetchInterval: LIVE });
export const useHeatmap = () => useQuery({ queryKey: qk.operasional.heatmap(), queryFn: api.operasional.heatmap, refetchInterval: LIVE * 2 });
export const useMarkerWilayah = () => useQuery({ queryKey: qk.operasional.marker(), queryFn: api.operasional.marker });
export const useTrenHarga = (kode: string, kab: string) =>
  useQuery({ queryKey: qk.operasional.tren(kode, kab), queryFn: () => api.operasional.tren(kode, kab), placeholderData: (prev) => prev });
export const useRingkasanEksekutif = () => useQuery({ queryKey: qk.eksekutif.ringkasan(), queryFn: api.eksekutif.ringkasan });
export const useXai = (kode: string) => useQuery({ queryKey: qk.eksekutif.xai(kode), queryFn: () => api.eksekutif.xai(kode), placeholderData: (prev) => prev });
/** Daftar entri harga; `enabled` dipakai sidebar agar peran tanpa `harga.lihat` tidak memicu 403. */
export const useEntriHarga = (enabled = true) => useQuery({ queryKey: qk.harga.list(), queryFn: api.harga.list, enabled });
export const useEwsList = () => useQuery({ queryKey: qk.ews.list(), queryFn: api.ews.list });
export const useIntervensi = () => useQuery({ queryKey: qk.intervensi.list(), queryFn: api.intervensi.list, staleTime: Infinity });
export const useColdStorage = () => useQuery({ queryKey: qk.coldStorage.ringkasan(), queryFn: api.coldStorage.ringkasan });
export const useSumberData = () => useQuery({ queryKey: qk.sumber.list(), queryFn: api.sumber.list });
export const usePengguna = (search?: string) =>
  useQuery({
    queryKey: qk.pengguna.list(search),
    queryFn: () => (MODE_MOCK ? api.pengguna.list() : penggunaApi.list({ search })),
  });
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

// ── Mutasi pengguna ───────────────────────────────────────────────────

/** Aksi massal pengguna — memanggil BFF lalu menyegarkan daftar. */
export function usePenggunaBulk() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ aksi, ids }: { aksi: BulkAksi; ids: string[]; label: string }) =>
      MODE_MOCK
        ? mockDelay({ berhasil: ids.length, gagal: 0 }, 500, 1100)
        : penggunaApi.bulk(aksi, ids),
    onSuccess: (res, { label }) => {
      qc.invalidateQueries({ queryKey: qk.pengguna.all() });
      toast.success(`${label}: ${res.berhasil.toLocaleString("id-ID")} pengguna`, {
        description: res.gagal
          ? `${res.gagal} pengguna gagal diproses.`
          : "Tercatat di audit trail.",
      });
    },
    onError: (err) => {
      toast.error("Aksi massal gagal", {
        description: err instanceof Error ? err.message : "Coba lagi beberapa saat.",
      });
    },
  });
}

export function useCreatePengguna() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CreatePenggunaInput) => penggunaApi.create(input),
    onSuccess: (_res, input) => {
      qc.invalidateQueries({ queryKey: qk.pengguna.all() });
      toast.success("Pengguna dibuat", { description: `${input.nama} (${input.email})` });
    },
    onError: (err) => {
      toast.error("Gagal membuat pengguna", {
        description: err instanceof Error ? err.message : "Coba lagi beberapa saat.",
      });
    },
  });
}

export function useUpdatePengguna() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdatePenggunaInput }) =>
      penggunaApi.update(id, input),
    onSuccess: (row) => {
      qc.invalidateQueries({ queryKey: qk.pengguna.all() });
      toast.success("Perubahan disimpan", { description: row.nama });
    },
    onError: (err) => {
      toast.error("Gagal menyimpan perubahan", {
        description: err instanceof Error ? err.message : "Coba lagi beberapa saat.",
      });
    },
  });
}

export function useDeletePengguna() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => penggunaApi.remove(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.pengguna.all() });
      toast.success("Pengguna dihapus", { description: "Tercatat di audit trail." });
    },
    onError: (err) => {
      toast.error("Gagal menghapus pengguna", {
        description: err instanceof Error ? err.message : "Coba lagi beberapa saat.",
      });
    },
  });
}

export function useResetPasswordPengguna() {
  return useMutation({
    mutationFn: ({ id, password }: { id: string; password: string }) =>
      penggunaApi.resetPassword(id, password),
    onSuccess: () => {
      toast.success("Kata sandi direset", {
        description: "Sesi pengguna diakhiri — mereka harus masuk ulang.",
      });
    },
    onError: (err) => {
      toast.error("Gagal mereset kata sandi", {
        description: err instanceof Error ? err.message : "Coba lagi beberapa saat.",
      });
    },
  });
}

export function useAssignRolePengguna() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, peran }: { id: string; peran: Role }) =>
      penggunaApi.assignRole(id, peran),
    onSuccess: (row) => {
      qc.invalidateQueries({ queryKey: qk.pengguna.all() });
      toast.success("Peran diperbarui", { description: `${row.nama} → ${ROLES[row.role].label}` });
    },
    onError: (err) => {
      toast.error("Gagal memperbarui peran", {
        description: err instanceof Error ? err.message : "Coba lagi beberapa saat.",
      });
    },
  });
}
