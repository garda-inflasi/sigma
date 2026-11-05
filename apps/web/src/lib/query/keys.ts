import type { AuditFilter } from "@/lib/api/audit";
import type { HargaParams } from "@/lib/api/harga";
import type { ColdStorageParams } from "@/lib/api/cold-storage-query";

/** Pabrik query key — hierarkis agar invalidasi dapat dilakukan per domain. */
export const qk = {
  siklus: () => ["siklus"] as const,
  operasional: {
    all: ["operasional"] as const,
    kpi: () => ["operasional", "kpi"] as const,
    heatmap: () => ["operasional", "heatmap"] as const,
    aktivitas: () => ["operasional", "aktivitas"] as const,
    pembanding: () => ["operasional", "pembanding-harga"] as const,
    cuaca: () => ["operasional", "cuaca"] as const,
    marker: () => ["operasional", "marker"] as const,
    tren: (kode: string, kab: string) => ["operasional", "tren", kode, kab] as const,
  },
  eksekutif: {
    ringkasan: () => ["eksekutif", "ringkasan"] as const,
    xaiAll: () => ["eksekutif", "xai"] as const,
    xai: (kode: string) => ["eksekutif", "xai", kode] as const,
  },
  spasial: {
    rute: () => ["spasial", "rute"] as const,
    kandidat: () => ["spasial", "kandidat"] as const,
    geometri: (zoom: number) => ["spasial", "geometri", zoom] as const,
  },
  prediksi: {
    all: () => ["prediksi"] as const,
    ringkasan: () => ["prediksi", "ringkasan"] as const,
    model: () => ["prediksi", "model"] as const,
    ml: () => ["prediksi", "ml"] as const,
  },
  harga: {
    all: () => ["harga"] as const,
    list: (params: HargaParams = {}) => ["harga", "list", params] as const,
    saya: () => ["harga", "saya"] as const,
    detail: (id: string) => ["harga", "detail", id] as const,
    riwayat: (id: string) => ["harga", "riwayat", id] as const,
    ringkasan: (hari: number) => ["harga", "ringkasan", hari] as const,
  },
  ews: { list: () => ["ews", "list"] as const, aturan: () => ["ews", "aturan"] as const, preferensi: () => ["ews", "preferensi"] as const },
  intervensi: { list: () => ["intervensi", "list"] as const, persetujuan: () => ["intervensi", "persetujuan"] as const },
  coldStorage: {
    all: () => ["cold-storage"] as const,
    list: (params: ColdStorageParams) => ["cold-storage", "list", params] as const,
    detail: (id: string) => ["cold-storage", "detail", id] as const,
  },
  sumber: {
    list: () => ["sumber-data"] as const,
    pipeline: () => ["sumber-data", "pipeline"] as const,
    eksekusi: (id: string) => ["sumber-data", "eksekusi", id] as const,
    sampel: (id: string) => ["sumber-data", "sampel", id] as const,
  },
  kpi: { list: () => ["kpi", "list"] as const, riwayat: (kode: string) => ["kpi", "riwayat", kode] as const },
  pengguna: {
    all: () => ["pengguna"] as const,
    list: (search?: string) => ["pengguna", "list", search ?? ""] as const,
  },
  rbac: { katalog: () => ["rbac", "katalog"] as const },
  audit: {
    list: (filter: AuditFilter) => ["audit", "list", filter] as const,
    detail: (id: string) => ["audit", "detail", id] as const,
  },
  brief: { list: () => ["policy-brief"] as const, detail: (id: string) => ["policy-brief", "detail", id] as const },
  tugas: { saya: () => ["tugas", "saya"] as const },
  master: {
    komoditas: () => ["master", "komoditas"] as const,
    wilayah: () => ["master", "wilayah"] as const,
    pasar: () => ["master", "pasar"] as const,
  },
};
