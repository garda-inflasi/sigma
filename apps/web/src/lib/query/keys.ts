import type { AuditFilter } from "@/lib/api/audit";
import type { HargaParams } from "@/lib/api/harga";
import type { ColdStorageParams } from "@/lib/api/cold-storage-query";
import type { KecamatanParams, PedagangParams } from "@/lib/api/master";
import type { PerubahanParams } from "@/lib/api/master-perubahan";

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
    tren: (kode: string, kab: string) =>
      ["operasional", "tren", kode, kab] as const,
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
    neraca: (komoditas: string) => ["spasial", "neraca", komoditas] as const,
    riwayat: () => ["spasial", "riwayat"] as const,
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
    lineage: (id: string) => ["harga", "lineage", id] as const,
    ringkasan: (hari: number) => ["harga", "ringkasan", hari] as const,
  },
  ews: {
    all: () => ["ews"] as const,
    list: () => ["ews", "list"] as const,
    detail: (id: string) => ["ews", "detail", id] as const,
    riwayat: (id: string) => ["ews", "riwayat", id] as const,
    aturan: () => ["ews", "aturan"] as const,
    preferensi: () => ["ews", "preferensi"] as const,
  },
  intervensi: {
    list: () => ["intervensi", "list"] as const,
    persetujuan: () => ["intervensi", "persetujuan"] as const,
    tahap: () => ["intervensi", "tahap"] as const,
    alur: () => ["intervensi", "alur"] as const,
    riwayat: (id: string) => ["intervensi", "riwayat", id] as const,
  },
  coldStorage: {
    all: () => ["cold-storage"] as const,
    list: (params: ColdStorageParams) =>
      ["cold-storage", "list", params] as const,
    detail: (id: string) => ["cold-storage", "detail", id] as const,
  },
  sumber: {
    list: () => ["sumber-data"] as const,
    detail: (id: string) => ["sumber-data", "detail", id] as const,
    pipeline: () => ["sumber-data", "pipeline"] as const,
    eksekusi: (id: string) => ["sumber-data", "eksekusi", id] as const,
    sampel: (id: string) => ["sumber-data", "sampel", id] as const,
  },
  kpi: {
    list: () => ["kpi", "list"] as const,
    riwayat: (kode: string) => ["kpi", "riwayat", kode] as const,
  },
  pengguna: {
    all: () => ["pengguna"] as const,
    list: (search?: string) => ["pengguna", "list", search ?? ""] as const,
    halaman: (kunci: string) => ["pengguna", "halaman", kunci] as const,
    detail: (id: string) => ["pengguna", "detail", id] as const,
    riwayatPeran: (id: string) => ["pengguna", "riwayat-peran", id] as const,
  },
  rbac: { katalog: () => ["rbac", "katalog"] as const },
  sesi: { aktif: () => ["sesi", "aktif"] as const },
  /** Profil sendiri (GET /api/auth/me): data diri dan status MFA; dipakai semua tab /profil. */
  profil: { saya: () => ["profil", "saya"] as const },
  audit: {
    list: (filter: AuditFilter) => ["audit", "list", filter] as const,
    detail: (id: string) => ["audit", "detail", id] as const,
    integritas: () => ["audit", "integritas"] as const,
  },
  brief: {
    list: () => ["policy-brief"] as const,
    detail: (id: string) => ["policy-brief", "detail", id] as const,
  },
  tugas: {
    saya: () => ["tugas", "saya"] as const,
    detail: (id: string) => ["tugas", "detail", id] as const,
  },
  master: {
    /** Akar seluruh query master; invalidasinya menyegarkan daftar, detail, dan antrean usulan sekaligus. */
    all: () => ["master"] as const,
    komoditas: () => ["master", "komoditas"] as const,
    wilayah: () => ["master", "wilayah"] as const,
    pasar: () => ["master", "pasar"] as const,
    komoditasDetail: (kode: string) =>
      ["master", "komoditas", "detail", kode] as const,
    wilayahDetail: (kode: string) =>
      ["master", "wilayah", "detail", kode] as const,
    pasarDetail: (kode: string) => ["master", "pasar", "detail", kode] as const,
    kecamatan: (params: KecamatanParams) =>
      ["master", "kecamatan", "halaman", params] as const,
    kecamatanKabupaten: (kabupaten: string) =>
      ["master", "kecamatan", "kabupaten", kabupaten] as const,
    kecamatanDetail: (kode: string) =>
      ["master", "kecamatan", "detail", kode] as const,
    satuan: () => ["master", "satuan"] as const,
    pedagang: (params: PedagangParams) =>
      ["master", "pedagang", params] as const,
    perubahan: (params: PerubahanParams) =>
      ["master", "perubahan", "daftar", params] as const,
    perubahanDetail: (id: string) =>
      ["master", "perubahan", "detail", id] as const,
    perubahanMenunggu: () => ["master", "perubahan", "menunggu"] as const,
  },
};
