import type { ColdStorageParams } from "@/lib/api/cold-storage-query";

/** Pabrik query key — hierarkis agar invalidasi dapat dilakukan per domain. */
export const qk = {
  siklus: () => ["siklus"] as const,
  operasional: {
    all: ["operasional"] as const,
    kpi: () => ["operasional", "kpi"] as const,
    heatmap: () => ["operasional", "heatmap"] as const,
    marker: () => ["operasional", "marker"] as const,
    tren: (kode: string, kab: string) => ["operasional", "tren", kode, kab] as const,
  },
  eksekutif: {
    ringkasan: () => ["eksekutif", "ringkasan"] as const,
    xai: (kode: string) => ["eksekutif", "xai", kode] as const,
  },
  harga: { list: () => ["harga", "list"] as const },
  ews: { list: () => ["ews", "list"] as const },
  intervensi: { list: () => ["intervensi", "list"] as const },
  coldStorage: {
    all: () => ["cold-storage"] as const,
    list: (params: ColdStorageParams) => ["cold-storage", "list", params] as const,
  },
  sumber: { list: () => ["sumber-data"] as const },
  pengguna: {
    all: () => ["pengguna"] as const,
    list: (search?: string) => ["pengguna", "list", search ?? ""] as const,
  },
  audit: { list: () => ["audit"] as const },
  brief: { list: () => ["policy-brief"] as const },
  tugas: { saya: () => ["tugas", "saya"] as const },
  master: { komoditas: () => ["master", "komoditas"] as const },
};
