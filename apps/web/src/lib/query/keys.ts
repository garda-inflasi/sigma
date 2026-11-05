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
    marker: () => ["operasional", "marker"] as const,
    tren: (kode: string, kab: string) => ["operasional", "tren", kode, kab] as const,
  },
  eksekutif: {
    ringkasan: () => ["eksekutif", "ringkasan"] as const,
    xai: (kode: string) => ["eksekutif", "xai", kode] as const,
  },
  spasial: { rute: () => ["spasial", "rute"] as const, kandidat: () => ["spasial", "kandidat"] as const },
  prediksi: { ringkasan: () => ["prediksi", "ringkasan"] as const, model: () => ["prediksi", "model"] as const },
  harga: {
    all: () => ["harga"] as const,
    list: (params: HargaParams = {}) => ["harga", "list", params] as const,
    saya: () => ["harga", "saya"] as const,
    ringkasan: (hari: number) => ["harga", "ringkasan", hari] as const,
  },
  ews: { list: () => ["ews", "list"] as const, aturan: () => ["ews", "aturan"] as const },
  intervensi: { list: () => ["intervensi", "list"] as const },
  coldStorage: {
    all: () => ["cold-storage"] as const,
    list: (params: ColdStorageParams) => ["cold-storage", "list", params] as const,
  },
  sumber: {
    list: () => ["sumber-data"] as const,
    eksekusi: (id: string) => ["sumber-data", "eksekusi", id] as const,
    sampel: (id: string) => ["sumber-data", "sampel", id] as const,
  },
  kpi: { list: () => ["kpi", "list"] as const },
  pengguna: {
    all: () => ["pengguna"] as const,
    list: (search?: string) => ["pengguna", "list", search ?? ""] as const,
  },
  rbac: { katalog: () => ["rbac", "katalog"] as const },
  audit: { list: (filter: AuditFilter) => ["audit", "list", filter] as const },
  brief: { list: () => ["policy-brief"] as const },
  tugas: { saya: () => ["tugas", "saya"] as const },
  master: {
    komoditas: () => ["master", "komoditas"] as const,
    wilayah: () => ["master", "wilayah"] as const,
    pasar: () => ["master", "pasar"] as const,
  },
};
