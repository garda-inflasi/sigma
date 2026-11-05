"use client";

import { keepPreviousData, useInfiniteQuery, useMutation, useQuery, useQueryClient, type QueryKey } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, bulk } from "@/lib/api/endpoints";
import { penggunaApi, type BulkAksi, type CreatePenggunaInput, type UpdatePenggunaInput } from "@/lib/api/pengguna";
import type { SumberBulkAksi } from "@/lib/api/sumber";
import type { AuditFilter } from "@/lib/api/audit";
import type { HargaParams } from "@/lib/api/harga";
import { MODE_MOCK, mockDelay } from "@/lib/api/client";
import type { ColdStorageBulkAction, ColdStorageParams } from "@/lib/api/cold-storage-query";
import type { StokColdStorage } from "@/lib/data/types";
import { formatAngka } from "@/lib/format";
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
export const useAktivitasLapangan = () => useQuery({ queryKey: qk.operasional.aktivitas(), queryFn: api.operasional.aktivitas, refetchInterval: LIVE });
export const useMarkerWilayah = () => useQuery({ queryKey: qk.operasional.marker(), queryFn: api.operasional.marker });
export const useRuteDistribusi = () => useQuery({ queryKey: qk.spasial.rute(), queryFn: api.spasial.rute });
export const useKandidatColdStorage = () => useQuery({ queryKey: qk.spasial.kandidat(), queryFn: api.spasial.kandidatColdStorage });
export const useTrenHarga = (kode: string, kab: string) =>
  useQuery({ queryKey: qk.operasional.tren(kode, kab), queryFn: () => api.operasional.tren(kode, kab), placeholderData: (prev) => prev });
export const useRingkasanEksekutif = () => useQuery({ queryKey: qk.eksekutif.ringkasan(), queryFn: api.eksekutif.ringkasan });
export const useXai = (kode: string) => useQuery({ queryKey: qk.eksekutif.xai(kode), queryFn: () => api.eksekutif.xai(kode), placeholderData: (prev) => prev });
export const useProyeksiRingkasan = () => useQuery({ queryKey: qk.prediksi.ringkasan(), queryFn: api.prediksi.ringkasan });
export const useModelPrediksi = () => useQuery({ queryKey: qk.prediksi.model(), queryFn: api.prediksi.model });
/** Daftar entri harga yang disaring server; `enabled` mencegah 403 bagi peran tanpa `harga.lihat`. */
export const useEntriHarga = (params: HargaParams = {}, enabled = true) =>
  useQuery({ queryKey: qk.harga.list(params), queryFn: () => api.harga.list(params), enabled, placeholderData: keepPreviousData });
/** Jumlah entri per status pada `hari` terakhir. */
export const useRingkasanHarga = (hari = 7, enabled = true) =>
  useQuery({ queryKey: qk.harga.ringkasan(hari), queryFn: () => api.harga.ringkasan(hari), enabled, refetchInterval: LIVE });
/** Riwayat entri milik pengguna yang masuk. */
export const useRiwayatHargaSaya = () => useQuery({ queryKey: qk.harga.saya(), queryFn: api.harga.saya });
export const useEwsList = () => useQuery({ queryKey: qk.ews.list(), queryFn: api.ews.list });
export const useEwsAturan = () => useQuery({ queryKey: qk.ews.aturan(), queryFn: api.ews.aturan, staleTime: 5 * 60_000 });
/** Kartu kanban. Dengan backend aktif disegarkan berkala agar perpindahan oleh pengguna lain muncul. */
export const useIntervensi = () =>
  useQuery({ queryKey: qk.intervensi.list(), queryFn: api.intervensi.list, staleTime: Infinity, refetchInterval: MODE_MOCK ? false : LIVE });
/**
 * Satu layar cold storage. `keepPreviousData` menahan tampilan lama saat
 * pindah halaman/urutan/filter agar tabel tidak berkedip; kartu fasilitas
 * ikut disegarkan tiap menit untuk telemetri suhu.
 */
export const useColdStorage = (params: ColdStorageParams) =>
  useQuery({
    queryKey: qk.coldStorage.list(params),
    queryFn: () => api.coldStorage.list(params),
    placeholderData: keepPreviousData,
    // Berhenti memuat ulang otomatis setelah galat (mis. sesi berakhir) agar tidak
    // menembak BFF tiap menit; dilanjutkan begitu "Coba lagi" atau perubahan kriteria berhasil.
    refetchInterval: (query) => (query.state.status === "error" ? false : 60_000),
  });
export const useSumberData = () => useQuery({ queryKey: qk.sumber.list(), queryFn: api.sumber.list });
/** Riwayat eksekusi; dimuat ulang tiap 2 detik selama masih ada run BERJALAN. */
export const useSumberEksekusi = (id: string | null) =>
  useQuery({
    queryKey: qk.sumber.eksekusi(id ?? ""),
    queryFn: () => api.sumber.eksekusi(id!),
    enabled: !!id,
    refetchInterval: (query) => (query.state.data?.some((e) => e.status === "BERJALAN") ? 2_000 : false),
  });
export const useSumberSampel = (id: string | null) =>
  useQuery({ queryKey: qk.sumber.sampel(id ?? ""), queryFn: () => api.sumber.sampel(id!), enabled: !!id });

/** Jalankan ingestion sebuah sumber sekarang, lalu segarkan riwayat dan katalog. */
export function useSumberJalankan() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.sumber.jalankan(id),
    onSuccess: (_, id) => {
      qc.invalidateQueries({ queryKey: qk.sumber.eksekusi(id) });
      qc.invalidateQueries({ queryKey: qk.sumber.sampel(id) });
      qc.invalidateQueries({ queryKey: qk.sumber.list() });
      toast.success("Ingestion dimulai", { description: "Hasilnya muncul di riwayat eksekusi. Tercatat di audit trail." });
    },
    onError: (err) => {
      toast.error("Tidak bisa menjalankan sumber", { description: err instanceof Error ? err.message : "Coba lagi beberapa saat." });
    },
  });
}

/** Jeda atau lanjutkan sumber lalu segarkan katalog dari server. */
export function useSumberBulk() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ aksi, ids }: { aksi: SumberBulkAksi; ids: string[]; label: string }) => api.sumber.bulk(aksi, ids),
    onSuccess: (res, { label }) => {
      qc.invalidateQueries({ queryKey: qk.sumber.list() });
      toast.success(`${label}: ${res.berhasil.toLocaleString("id-ID")} sumber`, {
        description: res.gagal ? `${res.gagal} sumber tidak berubah (tidak ada, atau sudah dalam status itu).` : "Tercatat di audit trail.",
      });
    },
    onError: (err) => {
      toast.error("Aksi massal gagal", { description: err instanceof Error ? err.message : "Coba lagi beberapa saat." });
    },
  });
}

export const useKpi = () => useQuery({ queryKey: qk.kpi.list(), queryFn: api.kpi.list, staleTime: 60_000 });
export const usePengguna = (search?: string) =>
  useQuery({
    queryKey: qk.pengguna.list(search),
    queryFn: () => (MODE_MOCK ? api.pengguna.list() : penggunaApi.list({ search })),
  });
export const useKatalogAkses = () => useQuery({ queryKey: qk.rbac.katalog(), queryFn: api.rbac.katalog, staleTime: 5 * 60_000 });
/** Audit trail dengan filter dan paginasi cursor di server; `fetchNextPage` memuat halaman berikutnya. */
export const useAudit = (filter: AuditFilter) =>
  useInfiniteQuery({
    queryKey: qk.audit.list(filter),
    queryFn: ({ pageParam }) => api.audit.page(filter, pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.nextCursor,
    placeholderData: keepPreviousData,
  });
export const useBriefs = () => useQuery({ queryKey: qk.brief.list(), queryFn: api.brief.list });
/** Menyusun draf brief; galat ditampilkan oleh pemanggil (dialog generator). */
export function useSusunBrief() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (periode: string) => api.brief.susun(periode),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.brief.list() }),
  });
}
export const useTugasSaya = () => useQuery({ queryKey: qk.tugas.saya(), queryFn: api.tugas.saya });
/** Master data; `enabled` memuat hanya entitas yang sedang dibuka. */
export const useMasterKomoditas = (enabled = true) => useQuery({ queryKey: qk.master.komoditas(), queryFn: api.master.komoditas, enabled });
export const useMasterWilayah = (enabled = true) => useQuery({ queryKey: qk.master.wilayah(), queryFn: api.master.wilayah, enabled });
export const useMasterPasar = (enabled = true) => useQuery({ queryKey: qk.master.pasar(), queryFn: api.master.pasar, enabled });

/**
 * Mutasi aksi massal dengan pembaruan optimistis.
 * `patch` mengubah baris terpilih di cache; rollback otomatis bila gagal.
 */
export function useBulkAction<T extends { id: string }>(opts: {
  sumber: string;
  queryKey: QueryKey;
  patch?: (row: T, aksi: string) => T | null; // null = hapus dari daftar
  /** Segarkan query berawalan key ini dari server setelah selesai (tidak di mode mock). */
  invalidate?: QueryKey;
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
        description: res.gagal ? `${res.gagal} data dilewati karena status atau pemisahan tugas.` : "Tercatat di audit trail.",
      });
    },
    onSettled: () => {
      if (opts.invalidate && !MODE_MOCK) qc.invalidateQueries({ queryKey: opts.invalidate });
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

// ── Mutasi cold storage ───────────────────────────────────────────────

/** Aksi massal pada lot stok — memanggil BFF lalu menyegarkan data layar. */
export function useColdStorageBulk() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ action, rows }: { action: ColdStorageBulkAction; rows: StokColdStorage[]; label: string }) =>
      api.coldStorage.bulk(action, rows),
    onSuccess: (res, { label }) => {
      qc.invalidateQueries({ queryKey: qk.coldStorage.all() });
      toast.success(`${label}: ${res.berhasil.toLocaleString("id-ID")} lot (${formatAngka(res.volumeTon, 1)} ton)`, {
        description: res.gagal ? `${res.gagal} lot tidak ditemukan — muat ulang halaman untuk melihat data terbaru.` : "Tercatat di audit trail.",
      });
    },
    onError: (err) => {
      toast.error("Aksi massal gagal", {
        description: err instanceof Error ? err.message : "Coba lagi beberapa saat.",
      });
    },
  });
}
