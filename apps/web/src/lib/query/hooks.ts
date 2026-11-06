"use client";

import { keepPreviousData, useInfiniteQuery, useMutation, useQuery, useQueryClient, type QueryKey } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, bulk } from "@/lib/api/endpoints";
import { penggunaApi, type BulkAksi, type CreatePenggunaInput, type UpdatePenggunaInput } from "@/lib/api/pengguna";
import type { AksiModel, SimulasiInput } from "@/lib/api/prediksi";
import type { UbahAturanInput } from "@/lib/api/ews";
import { ringkasHasilKartu, type AksiKartu, type BuatIntervensiInput, type KeputusanPersetujuan } from "@/lib/api/intervensi";
import type { SumberBulkAksi } from "@/lib/api/sumber";
import type { AuditFilter } from "@/lib/api/audit";
import { ringkasSebab, type HargaParams } from "@/lib/api/harga";
import { MASTER_BULK_LABEL, ringkasDitolak, type MasterBulkAksi } from "@/lib/api/master";
import { MODE_MOCK, mockDelay } from "@/lib/api/client";
import type { ColdStorageBulkAction, ColdStorageParams } from "@/lib/api/cold-storage-query";
import type { PembacaanManualPayload, TransaksiPayload, UbahFasilitasPayload } from "@/lib/api/cold-storage";
import type { OptimasiParams } from "@/lib/api/spasial";
import { unduhLaporanKpi } from "@/lib/api/kpi-laporan";
import { unduhProyeksiCsv } from "@/lib/api/prediksi-ekspor";
import type { StokColdStorage } from "@/lib/data/types";
import { formatAngka } from "@/lib/format";
import { ROLES, type Role } from "@/lib/rbac/roles";
import { kunciCachePengguna, type StateTabelPengguna } from "@/lib/pengguna/tabel";
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
export const usePembandingHarga = () => useQuery({ queryKey: qk.operasional.pembanding(), queryFn: api.operasional.pembandingHarga, refetchInterval: LIVE * 2 });
export const useCuacaTerbaru = () => useQuery({ queryKey: qk.operasional.cuaca(), queryFn: api.operasional.cuaca, refetchInterval: LIVE * 10 });
export const useMarkerWilayah = () => useQuery({ queryKey: qk.operasional.marker(), queryFn: api.operasional.marker });
export const useRuteDistribusi = () => useQuery({ queryKey: qk.spasial.rute(), queryFn: api.spasial.rute });
/** Menghitung rencana distribusi; bila disimpan, daftar rute disegarkan. */
export function useOptimasiRute() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (p: OptimasiParams) => api.spasial.optimasi(p),
    onSuccess: (res) => {
      if (res.optimasiId) {
        qc.invalidateQueries({ queryKey: qk.spasial.rute() });
        qc.invalidateQueries({ queryKey: qk.spasial.riwayat() });
        toast.success("Rencana disimpan sebagai usulan rute", { description: "Tercatat di audit trail." });
      }
    },
    onError: (err) => {
      toast.error("Optimasi gagal", { description: err instanceof Error ? err.message : "Coba lagi beberapa saat." });
    },
  });
}
export const useGeometriWilayah = (zoom: number) =>
  useQuery({ queryKey: qk.spasial.geometri(zoom), queryFn: () => api.spasial.geometri(zoom), staleTime: 60 * 60_000 });
export const useKandidatColdStorage = () => useQuery({ queryKey: qk.spasial.kandidat(), queryFn: api.spasial.kandidatColdStorage });
/** Neraca per wilayah untuk satu komoditas ("" = semua komoditas), periode terbaru. */
export const useNeracaWilayah = (komoditas: string) =>
  useQuery({ queryKey: qk.spasial.neraca(komoditas), queryFn: () => api.spasial.neraca(komoditas || undefined), placeholderData: keepPreviousData });
/** Mencari kabupaten/kota untuk satu koordinat; galat (mis. 404 tanpa data wilayah) ditampilkan oleh pemanggil. */
export const useCariLokasi = () => useMutation({ mutationFn: (t: { lat: number; lng: number }) => api.spasial.lokasi(t.lat, t.lng) });
export const useRiwayatOptimasi = () => useQuery({ queryKey: qk.spasial.riwayat(), queryFn: api.spasial.riwayatOptimasi });
export const useTrenHarga = (kode: string, kab: string) =>
  useQuery({ queryKey: qk.operasional.tren(kode, kab), queryFn: () => api.operasional.tren(kode, kab), placeholderData: (prev) => prev });
export const useRingkasanEksekutif = () => useQuery({ queryKey: qk.eksekutif.ringkasan(), queryFn: api.eksekutif.ringkasan });
export const useXai = (kode: string) => useQuery({ queryKey: qk.eksekutif.xai(kode), queryFn: () => api.eksekutif.xai(kode), placeholderData: (prev) => prev });
export const useProyeksiRingkasan = () => useQuery({ queryKey: qk.prediksi.ringkasan(), queryFn: api.prediksi.ringkasan });
export const useModelPrediksi = () => useQuery({ queryKey: qk.prediksi.model(), queryFn: api.prediksi.model });
/** Status layanan ML; `enabled` mencegah 403 bagi peran tanpa `prediksi.lihat`. */
export const useStatusMl = (enabled = true) => useQuery({ queryKey: qk.prediksi.ml(), queryFn: api.prediksi.statusMl, enabled, staleTime: 30_000 });
/** Melatih model kandidat lalu menyegarkan registri; galat (mis. 503 layanan ML mati) ditampilkan sebagai toast. */
export function useLatihModel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (komoditasKode: string) => api.prediksi.latih(komoditasKode),
    onSuccess: (m) => {
      qc.invalidateQueries({ queryKey: qk.prediksi.all() });
      toast.success(`Model kandidat ${m.versi} dilatih`, { description: `MAPE backtest ${m.mape.toLocaleString("id-ID")}%. Promosikan setelah ditelaah.` });
    },
    onError: (err) => {
      toast.error("Pelatihan model gagal", { description: err instanceof Error ? err.message : "Coba lagi beberapa saat." });
    },
  });
}
/**
 * Meramal ulang proyeksi satu komoditas dengan parameter model produksi. Galat (503 layanan ML mati,
 * 409 model produksi masih demo, 422 data kurang) ditampilkan dengan pesan server; hasilnya dibaca dari `data`.
 */
export function useSegarkanProyeksi() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (komoditasKode: string) => api.prediksi.segarkan(komoditasKode),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.prediksi.all() });
      qc.invalidateQueries({ queryKey: qk.eksekutif.xaiAll() });
    },
    onError: (err) => {
      toast.error("Proyeksi gagal disegarkan", { description: err instanceof Error ? err.message : "Coba lagi beberapa saat." });
    },
  });
}
/** Mengevaluasi proyeksi jatuh tempo; registri (MAPE realisasi) dimuat ulang. Hasilnya dibaca dari `data`. */
export function useEvaluasiPrediksi() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.prediksi.evaluasi(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.prediksi.model() });
    },
    onError: (err) => {
      toast.error("Evaluasi akurasi gagal", { description: err instanceof Error ? err.message : "Coba lagi beberapa saat." });
    },
  });
}
/** Mengunduh proyeksi satu komoditas (CSV). */
export function useUnduhProyeksi() {
  return useMutation({
    mutationFn: (komoditasKode: string) => unduhProyeksiCsv(komoditasKode),
    onSuccess: () => toast.success("Proyeksi diunduh"),
    onError: (err) => {
      toast.error("Unduhan proyeksi gagal", { description: err instanceof Error ? err.message : "Coba lagi beberapa saat." });
    },
  });
}
/** Daftar entri harga yang disaring server; `enabled` mencegah 403 bagi peran tanpa `harga.lihat`. */
export const useEntriHarga = (params: HargaParams = {}, enabled = true) =>
  useQuery({ queryKey: qk.harga.list(params), queryFn: () => api.harga.list(params), enabled, placeholderData: keepPreviousData });
/** Jumlah entri per status pada `hari` terakhir. */
export const useRingkasanHarga = (hari = 7, enabled = true) =>
  useQuery({ queryKey: qk.harga.ringkasan(hari), queryFn: () => api.harga.ringkasan(hari), enabled, refetchInterval: LIVE });
/** Riwayat entri milik pengguna yang masuk. */
export const useRiwayatHargaSaya = () => useQuery({ queryKey: qk.harga.saya(), queryFn: api.harga.saya });
/** Bukti dan bendera validasi otomatis satu entri. */
export const useDetailHarga = (id: string | undefined) =>
  useQuery({ queryKey: qk.harga.detail(id ?? ""), queryFn: () => api.harga.detail(id!), enabled: !!id });
/** Perpindahan status satu entri (timeline detail). */
export const useRiwayatStatus = (id: string | undefined) =>
  useQuery({ queryKey: qk.harga.riwayat(id ?? ""), queryFn: () => api.harga.riwayat(id!), enabled: !!id });
export const useEwsList = () => useQuery({ queryKey: qk.ews.list(), queryFn: api.ews.list });
/** Alert lengkap dari server; mati bila `id` kosong. */
export const useEwsDetail = (id: string | null) =>
  useQuery({ queryKey: qk.ews.detail(id ?? ""), queryFn: () => api.ews.detail(id!), enabled: !!id, retry: false });
/** Riwayat perubahan status alert (timeline detail). */
export const useEwsRiwayat = (id: string | null) =>
  useQuery({ queryKey: qk.ews.riwayat(id ?? ""), queryFn: () => api.ews.riwayat(id!), enabled: !!id, retry: false });
export const useEwsAturan = () => useQuery({ queryKey: qk.ews.aturan(), queryFn: api.ews.aturan, staleTime: 5 * 60_000 });

/** Preferensi notifikasi pengguna; `enabled` mencegah 403 bagi peran tanpa `ews.akui`. */
export const usePreferensiNotifikasi = (enabled = true) =>
  useQuery({ queryKey: qk.ews.preferensi(), queryFn: api.ews.preferensi, enabled });

/** Menyimpan preferensi notifikasi; cache diganti dengan jawaban server. */
export function useSimpanPreferensi() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: api.ews.simpanPreferensi,
    onSuccess: (view) => {
      qc.setQueryData(qk.ews.preferensi(), view);
      toast.success("Preferensi notifikasi disimpan");
    },
    onError: (err) => toast.error("Preferensi gagal disimpan", { description: err instanceof Error ? err.message : "Coba lagi beberapa saat." }),
  });
}

/** Menyimpan ambang aturan. Galat (termasuk 409 versi usang) ditampilkan oleh pemanggil; aturan dimuat ulang dari server. */
export function useUbahAturanEws() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UbahAturanInput }) => api.ews.ubahAturan(id, input),
    onSuccess: () => toast.success("Ambang disimpan", { description: "Berlaku pada evaluasi berikutnya. Tercatat di audit trail." }),
    onSettled: () => qc.invalidateQueries({ queryKey: qk.ews.aturan() }),
  });
}

/** Menjalankan rule engine sekarang lalu menyegarkan daftar alert. */
export function useEvaluasiEws() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: api.ews.evaluasi,
    onSuccess: (r) => {
      qc.invalidateQueries({ queryKey: qk.ews.list() });
      toast.success(`Evaluasi selesai: ${r.dibuat} alert baru, ${r.dinaikkan} naik tingkat`, {
        description: `${r.aturan} aturan aktif, ${r.diukur} kondisi diukur${r.galat ? `, ${r.galat} galat (lihat log server)` : ""}.`,
      });
    },
    onError: (err) => toast.error("Evaluasi gagal", { description: err instanceof Error ? err.message : "Coba lagi beberapa saat." }),
  });
}
/** Kartu kanban. Dengan backend aktif disegarkan berkala agar perpindahan oleh pengguna lain muncul. */
export const useIntervensi = () =>
  useQuery({ queryKey: qk.intervensi.list(), queryFn: api.intervensi.list, staleTime: Infinity, refetchInterval: MODE_MOCK ? false : LIVE });
/** Permintaan persetujuan yang menunggu; disegarkan berkala agar keputusan pengguna lain muncul. */
export const usePersetujuan = (enabled = true) =>
  useQuery({ queryKey: qk.intervensi.persetujuan(), queryFn: () => api.intervensi.persetujuan("MENUNGGU"), enabled, refetchInterval: MODE_MOCK ? false : LIVE });

/** Definisi tahap dari server (kolom papan); jarang berubah. */
export const useTahapIntervensi = () =>
  useQuery({ queryKey: qk.intervensi.tahap(), queryFn: api.intervensi.tahap, staleTime: 5 * 60_000 });
/** Alur persetujuan berjenjang: rantai langkah dan izin tiap tahap persetujuan. */
export const useAlurPersetujuan = (enabled = true) =>
  useQuery({ queryKey: qk.intervensi.alur(), queryFn: api.intervensi.alur, enabled, staleTime: 5 * 60_000 });
/** Riwayat satu kartu; selalu dimuat ulang saat lembar detail dibuka agar pemindahan terbaru tampil. */
export const useRiwayatKartu = (id: string | undefined) =>
  useQuery({ queryKey: qk.intervensi.riwayat(id ?? ""), queryFn: () => api.intervensi.riwayat(id!), enabled: !!id, staleTime: 0, refetchOnMount: "always" });

/** Membuat kartu intervensi; papan dimuat ulang agar kartu baru muncul di tahap pertama siklusnya. */
export function useBuatIntervensi() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: BuatIntervensiInput) => api.intervensi.buat(input),
    onSuccess: (kartu) => toast.success("Intervensi ditambahkan", { description: `${kartu.id} masuk ke tahap pertama siklusnya. Tercatat di riwayat dan audit trail.` }),
    onError: (err) => toast.error("Intervensi gagal ditambahkan", { description: err instanceof Error ? err.message : "Coba lagi beberapa saat." }),
    onSettled: () => qc.invalidateQueries({ queryKey: qk.intervensi.list() }),
  });
}

/** Mengajukan persetujuan pemindahan kartu; papan dan antrean dimuat ulang. */
export function useAjukanPersetujuan() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, tahap, catatan }: { id: string; tahap: string; catatan?: string }) => api.intervensi.ajukan(id, tahap, catatan),
    onSuccess: (p) => toast.success("Persetujuan diajukan", { description: `${p.kartuJudul}: ${p.langkah.length} langkah persetujuan.` }),
    onError: (err) => toast.error("Pengajuan gagal", { description: err instanceof Error ? err.message : "Coba lagi beberapa saat." }),
    onSettled: () => qc.invalidateQueries({ queryKey: qk.intervensi.persetujuan() }),
  });
}

/** Memutuskan langkah persetujuan; kartu dapat berpindah tahap, jadi papan ikut dimuat ulang. */
export function usePutusPersetujuan() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, keputusan, catatan }: { id: number; keputusan: KeputusanPersetujuan; catatan?: string }) => api.intervensi.putus(id, keputusan, catatan),
    onSuccess: (p, { keputusan }) => {
      const selesai = p.status !== "MENUNGGU";
      toast.success(keputusan === "SETUJU" ? (selesai ? "Persetujuan selesai" : "Langkah disetujui") : "Permintaan ditolak", {
        description: selesai && keputusan === "SETUJU" && p.tahapTujuan ? "Kartu dipindahkan ke tahap tujuan." : `${p.kartuJudul}. Tercatat di audit trail.`,
      });
    },
    onError: (err) => toast.error("Keputusan gagal disimpan", { description: err instanceof Error ? err.message : "Coba lagi beberapa saat." }),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: qk.intervensi.persetujuan() });
      qc.invalidateQueries({ queryKey: qk.intervensi.list() });
    },
  });
}

/**
 * Menugaskan kartu ke diri sendiri atau mengeskalasinya (aksi massal); toast merangkum hasil per kartu
 * dan papan dimuat ulang karena versi kartu berubah.
 */
export function useAksiKartu() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ aksi, ids }: { aksi: AksiKartu; ids: string[]; label: string }) => api.intervensi.aksi(aksi, ids),
    onSuccess: (res, { label }) => {
      const { judul, deskripsi } = ringkasHasilKartu(label, res);
      (res.berhasil ? toast.success : toast.warning)(judul, { description: deskripsi });
    },
    onError: (err) => toast.error("Aksi gagal", { description: err instanceof Error ? err.message : "Coba lagi beberapa saat." }),
    onSettled: () => qc.invalidateQueries({ queryKey: qk.intervensi.list() }),
  });
}

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
/** Sumber lengkap dari server (dimuat ulang bersama katalog lewat prefix key). */
export const useSumberDetail = (id: string | null) =>
  useQuery({ queryKey: qk.sumber.detail(id ?? ""), queryFn: () => api.sumber.detail(id!), enabled: !!id, retry: false });
export const useSumberPipeline = () => useQuery({ queryKey: qk.sumber.pipeline(), queryFn: api.sumber.pipeline, refetchInterval: LIVE * 2 });
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

/** Mengunduh laporan KPI (CSV) bulan berjalan. */
export function useEksporKpi() {
  return useMutation({
    mutationFn: unduhLaporanKpi,
    onSuccess: () => toast.success("Laporan KPI diunduh", { description: "Ekspor tercatat di audit trail." }),
    onError: (err) => {
      toast.error("Ekspor laporan gagal", { description: err instanceof Error ? err.message : "Coba lagi beberapa saat." });
    },
  });
}
export const useKpi = () => useQuery({ queryKey: qk.kpi.list(), queryFn: api.kpi.list, staleTime: 60_000 });
/** Riwayat satu indikator; baru diminta saat `kode` terisi (lembar detail terbuka). */
export const useKpiRiwayat = (kode: string | undefined) =>
  useQuery({ queryKey: qk.kpi.riwayat(kode ?? ""), queryFn: () => api.kpi.riwayat(kode as string), enabled: !!kode, staleTime: 60_000 });
export const usePengguna = (search?: string) =>
  useQuery({
    queryKey: qk.pengguna.list(search),
    queryFn: () => (MODE_MOCK ? api.pengguna.list() : penggunaApi.list({ search })),
  });
/**
 * Satu halaman daftar pengguna yang dicari, disaring, diurutkan, dan dipaginasi server. Halaman sebelumnya tetap
 * tampil selama halaman baru dimuat (tabel tidak berkedip), dan `isPlaceholderData` menandainya.
 */
export const usePenggunaHalaman = (state: StateTabelPengguna) =>
  useQuery({
    queryKey: qk.pengguna.halaman(kunciCachePengguna(state)),
    queryFn: () => (MODE_MOCK ? api.pengguna.halaman(state) : penggunaApi.daftar(state)),
    placeholderData: keepPreviousData,
  });
/** Riwayat perubahan peran satu pengguna dari audit trail; mati bila `id` kosong. Tanpa cache agar selalu segar. */
export const useRiwayatPeran = (id: string | null) =>
  useQuery({ queryKey: qk.pengguna.riwayatPeran(id ?? ""), queryFn: () => api.pengguna.riwayatPeran(id!), enabled: !!id, retry: false, gcTime: 0 });
/** Pengguna lengkap dari server untuk dialog ubah; mati bila `id` kosong. Tanpa cache (gcTime 0): formulir selalu memakai data terbaru. */
export const usePenggunaDetail = (id: string | null) =>
  useQuery({ queryKey: qk.pengguna.detail(id ?? ""), queryFn: () => api.pengguna.detail(id!), enabled: !!id, retry: false, gcTime: 0 });
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
/** Detail satu entri audit (dengan diff); kosong sampai `id` dipilih. */
export const useAuditDetail = (id: string | undefined) =>
  useQuery({
    queryKey: qk.audit.detail(id ?? ""),
    queryFn: () => api.audit.detail(id as string),
    enabled: !!id,
    staleTime: Infinity, // baris audit tidak pernah berubah
  });
/** Ekspor CSV sesuai filter yang sedang aktif. */
export function useEksporAudit() {
  return useMutation({
    mutationFn: (filter: AuditFilter) => api.audit.ekspor(filter),
    onSuccess: (res) =>
      res.terpotong
        ? toast.warning(`Ekspor dipotong pada ${res.baris.toLocaleString("id-ID")} baris`, { description: "Persempit filter (rentang tanggal, aksi) untuk mendapat sisanya." })
        : toast.success(`${res.baris.toLocaleString("id-ID")} baris diekspor`, { description: "Ekspor tercatat di audit trail." }),
    onError: (err) => {
      toast.error("Ekspor gagal", { description: err instanceof Error ? err.message : "Coba lagi beberapa saat." });
    },
  });
}
/** Daftar brief; dimuat ulang berkala selama ada brief yang masih disusun server (DIPROSES). */
export const useBriefs = () =>
  useQuery({
    queryKey: qk.brief.list(),
    queryFn: api.brief.list,
    refetchInterval: (query) => (query.state.status !== "error" && query.state.data?.some((b) => b.status === "DIPROSES") ? 3000 : false),
  });
/** Brief lengkap; dipantau tiap 2 detik selama server masih menyusun drafnya (status DIPROSES). */
export const useBriefDetail = (id: string | undefined) =>
  useQuery({
    queryKey: qk.brief.detail(id ?? ""),
    queryFn: () => api.brief.detail(id!),
    enabled: !!id,
    refetchInterval: (query) => (query.state.status !== "error" && query.state.data?.status === "DIPROSES" ? 2000 : false),
  });
/** Memulai penyusunan draf brief (juga untuk mengulang brief GAGAL); galat ditampilkan oleh pemanggil (dialog generator). */
export function useSusunBrief() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (periode: string) => api.brief.susun(periode),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.brief.list() }),
  });
}
export const useTugasSaya = () => useQuery({ queryKey: qk.tugas.saya(), queryFn: api.tugas.saya });
/** Detail tugas dengan status terisi tiap komoditas; gagal (mis. offline) tidak diulang agar sheet cepat jatuh ke data daftar. */
export const useTugasDetail = (id: string | null) =>
  useQuery({ queryKey: qk.tugas.detail(id ?? ""), queryFn: () => api.tugas.detail(id!), enabled: !!id, retry: false });
/** Master data; `enabled` memuat hanya entitas yang sedang dibuka. */
export const useMasterKomoditas = (enabled = true) => useQuery({ queryKey: qk.master.komoditas(), queryFn: api.master.komoditas, enabled });
export const useMasterWilayah = (enabled = true) => useQuery({ queryKey: qk.master.wilayah(), queryFn: api.master.wilayah, enabled });
export const useMasterPasar = (enabled = true) => useQuery({ queryKey: qk.master.pasar(), queryFn: api.master.pasar, enabled });
/** Ajukan/setujui/arsipkan komoditas; daftar dimuat ulang dari server, kode yang ditolak dilaporkan satu per satu. */
export function useBulkMasterKomoditas() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ aksi, kode }: { aksi: MasterBulkAksi; kode: string[] }) => api.master.bulkKomoditas(aksi, kode),
    onSuccess: (hasil, { aksi }) => {
      const kata = MASTER_BULK_LABEL[aksi];
      if (hasil.berhasil > 0) toast.success(`${hasil.berhasil.toLocaleString("id-ID")} komoditas ${kata}`, { description: hasil.gagal ? undefined : "Tercatat di audit trail." });
      if (hasil.gagal > 0) toast.error(`${hasil.gagal.toLocaleString("id-ID")} komoditas ditolak`, { description: ringkasDitolak(hasil.ditolak) });
    },
    onError: (err) => toast.error("Aksi master data gagal", { description: err instanceof Error ? err.message : "Coba lagi beberapa saat." }),
    onSettled: () => qc.invalidateQueries({ queryKey: qk.master.komoditas() }),
  });
}

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
    mutationFn: ({ aksi, ids, alasan }: { aksi: string; ids: string[]; label: string; alasan?: string }) => bulk(opts.sumber, aksi, ids, alasan),
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
        description: res.gagal ? `${res.gagal} data dilewati: ${ringkasSebab(res.rincian) || "status atau pemisahan tugas"}.` : "Tercatat di audit trail.",
      });
    },
    onSettled: () => {
      if (opts.invalidate && !MODE_MOCK) qc.invalidateQueries({ queryKey: opts.invalidate });
    },
  });
}

// ── Mutasi prediksi ───────────────────────────────────────────────────

/** Promosi/arsip model; menyegarkan registri dan proyeksi (model produksi menentukan proyeksi yang tampil). */
export function useAksiModel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ aksi, ids }: { aksi: AksiModel; ids: string[]; label: string }) => api.prediksi.aksiModel(aksi, ids),
    onSuccess: (res, { label }) => {
      qc.invalidateQueries({ queryKey: qk.prediksi.all() });
      qc.invalidateQueries({ queryKey: qk.eksekutif.xaiAll() });
      toast.success(`${label}: ${res.berhasil.toLocaleString("id-ID")} model`, { description: res.berhasil ? "Tercatat di audit trail." : undefined });
      if (res.gagal) toast.warning(`${res.gagal} model dilewati`, { description: res.detail.map((d) => d.alasan).join("; ") });
    },
    onError: (err) => {
      toast.error("Aksi model gagal", { description: err instanceof Error ? err.message : "Coba lagi beberapa saat." });
    },
  });
}

/** Menghitung simulasi skenario di server; hasilnya dibaca dari `data` mutasi. */
export function useSimulasi() {
  return useMutation({
    mutationFn: (input: SimulasiInput) => api.prediksi.simulasi(input),
    onError: (err) => {
      toast.error("Simulasi gagal", { description: err instanceof Error ? err.message : "Coba lagi beberapa saat." });
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

/** Membuka kunci login pengguna (lockout) lalu menyegarkan daftar agar lencana "Terkunci" hilang. */
export function useBukaKunciPengguna() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id }: { id: string; nama: string }) =>
      MODE_MOCK
        ? mockDelay({ id, terkunci_sebelumnya: true }, 300, 700)
        : penggunaApi.bukaKunci(id),
    onSuccess: (_res, { nama }) => {
      qc.invalidateQueries({ queryKey: qk.pengguna.all() });
      toast.success("Kunci dibuka", {
        description: `${nama} dapat masuk lagi. Tercatat di audit trail.`,
      });
    },
    onError: (err) => {
      toast.error("Gagal membuka kunci", {
        description: err instanceof Error ? err.message : "Coba lagi beberapa saat.",
      });
    },
  });
}

/** Reset MFA oleh admin lalu menyegarkan daftar (kolom MFA) dan riwayat; toast gagal datang dari sini. */
export function useResetMfaPengguna() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id }: { id: string; nama: string }) => (MODE_MOCK ? mockDelay({ id, mfa_aktif_sebelumnya: true, sesi_dicabut: 1 }, 300, 700) : penggunaApi.resetMfa(id)),
    onSuccess: (res, { nama }) => {
      qc.invalidateQueries({ queryKey: qk.pengguna.all() });
      toast.success("MFA direset", {
        description: res.mfa_aktif_sebelumnya
          ? `${nama} masuk dengan kata sandi dan mendaftarkan autentikator baru. ${res.sesi_dicabut.toLocaleString("id-ID")} sesi diakhiri. Tercatat di audit trail.`
          : `${nama} memang belum memakai MFA, tidak ada yang berubah. Tercatat di audit trail.`,
      });
    },
    onError: (err) => {
      toast.error("Gagal mereset MFA", {
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

/** Detail satu fasilitas (riwayat suhu, lot, transaksi); kosong sampai `id` dipilih. */
export const useFasilitasDetail = (id: string | undefined) =>
  useQuery({
    queryKey: qk.coldStorage.detail(id ?? ""),
    queryFn: () => api.coldStorage.detail(id as string),
    enabled: !!id,
    refetchInterval: LIVE,
  });

/** Mencatat transaksi stok lalu menyegarkan kartu, tabel, dan detail. */
export function useCatatTransaksi() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (p: TransaksiPayload) => api.coldStorage.transaksi(p),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.coldStorage.all() });
      toast.success("Transaksi stok tercatat", { description: "Tercatat di audit trail." });
    },
    onError: (err) => {
      toast.error("Transaksi gagal", { description: err instanceof Error ? err.message : "Coba lagi beberapa saat." });
    },
  });
}

/** Mengubah kapasitas/batas alarm/status fasilitas lalu menyegarkan kartu dan detail. */
export function useUbahFasilitas(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (p: UbahFasilitasPayload) => api.coldStorage.ubahFasilitas(id, p),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.coldStorage.all() });
      toast.success("Fasilitas diperbarui", { description: "Tercatat di audit trail." });
    },
    onError: (err) => {
      toast.error("Fasilitas tidak dapat diubah", { description: err instanceof Error ? err.message : "Coba lagi beberapa saat." });
    },
  });
}

/** Mencatat pembacaan suhu manual lalu menyegarkan detail dan kartu fasilitas. */
export function useCatatTelemetri() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (p: PembacaanManualPayload) => api.coldStorage.catatTelemetri(p),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.coldStorage.all() }),
    onError: (err) => {
      toast.error("Suhu tidak dapat dicatat", { description: err instanceof Error ? err.message : "Coba lagi beberapa saat." });
    },
  });
}

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
