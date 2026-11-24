import type { AuditFilter } from "@/lib/api/audit";
import type { FilterOperasional, SaringanOperasional } from "@/lib/api/dashboard";
import type { HargaParams } from "@/lib/api/harga";
import type { ColdStorageParams } from "@/lib/api/cold-storage-query";
import type { KecamatanParams, PedagangParams } from "@/lib/api/master";
import type { PerubahanParams } from "@/lib/api/master-perubahan";
import { getLingkunganData } from "@/lib/stores/ui-store";

const env = () => getLingkunganData();

/** Pabrik query key — hierarkis agar invalidasi dapat dilakukan per domain, dan terpisah per `lingkunganData` (`staging` vs `prod`). */
export const qk = {
  siklus: () => ["siklus", env()] as const,
  operasional: {
    all: ["operasional"] as const,
    // Filter dibersihkan pemanggil (bersihkanFilter) supaya filter yang sama maknanya berbagi cache.
    kpi: (filter: FilterOperasional = {}) => ["operasional", "kpi", filter, env()] as const,
    heatmap: (filter: FilterOperasional = {}) => ["operasional", "heatmap", filter, env()] as const,
    aktivitas: () => ["operasional", "aktivitas", env()] as const,
    petugas: (kabupatenKode = "") => ["operasional", "petugas", kabupatenKode, env()] as const,
    stok: (saringan: SaringanOperasional = {}) => ["operasional", "stok", saringan, env()] as const,
    produsen: (saringan: SaringanOperasional = {}) => ["operasional", "produsen", saringan, env()] as const,
    logistik: (saringan: SaringanOperasional = {}) => ["operasional", "logistik", saringan, env()] as const,
    pembanding: () => ["operasional", "pembanding-harga", env()] as const,
    cuaca: () => ["operasional", "cuaca", env()] as const,
    marker: () => ["operasional", "marker", env()] as const,
    tren: (kode: string, kab: string) =>
      ["operasional", "tren", kode, kab, env()] as const,
  },
  eksekutif: {
    ringkasan: () => ["eksekutif", "ringkasan", env()] as const,
    xaiAll: () => ["eksekutif", "xai"] as const,
    xai: (kode: string) => ["eksekutif", "xai", kode, env()] as const,
  },
  spasial: {
    rute: () => ["spasial", "rute", env()] as const,
    kandidat: () => ["spasial", "kandidat", env()] as const,
    geometri: (zoom: number) => ["spasial", "geometri", zoom, env()] as const,
    neraca: (komoditas: string) => ["spasial", "neraca", komoditas, env()] as const,
    riwayat: () => ["spasial", "riwayat", env()] as const,
    titikDesa: (jenis?: string) => ["spasial", "titik-desa", jenis ?? "ALL", env()] as const,
  },
  prediksi: {
    all: () => ["prediksi"] as const,
    ringkasan: () => ["prediksi", "ringkasan", env()] as const,
    model: () => ["prediksi", "model", env()] as const,
    ml: () => ["prediksi", "ml", env()] as const,
  },
  harga: {
    all: () => ["harga"] as const,
    list: (params: HargaParams = {}) => ["harga", "list", params, env()] as const,
    saya: () => ["harga", "saya", env()] as const,
    detail: (id: string) => ["harga", "detail", id, env()] as const,
    riwayat: (id: string) => ["harga", "riwayat", id, env()] as const,
    lineage: (id: string) => ["harga", "lineage", id, env()] as const,
    ringkasan: (hari: number) => ["harga", "ringkasan", hari, env()] as const,
  },
  ews: {
    all: () => ["ews"] as const,
    list: () => ["ews", "list", env()] as const,
    kritisBaru: () => ["ews", "kritis-baru", env()] as const,
    detail: (id: string) => ["ews", "detail", id, env()] as const,
    riwayat: (id: string) => ["ews", "riwayat", id, env()] as const,
    aturan: () => ["ews", "aturan", env()] as const,
    preferensi: () => ["ews", "preferensi", env()] as const,
  },
  intervensi: {
    list: () => ["intervensi", "list", env()] as const,
    persetujuan: () => ["intervensi", "persetujuan", env()] as const,
    tahap: () => ["intervensi", "tahap", env()] as const,
    alur: () => ["intervensi", "alur", env()] as const,
    riwayat: (id: string) => ["intervensi", "riwayat", id, env()] as const,
  },
  coldStorage: {
    all: () => ["cold-storage"] as const,
    list: (params: ColdStorageParams) =>
      ["cold-storage", "list", params, env()] as const,
    detail: (id: string) => ["cold-storage", "detail", id, env()] as const,
  },
  sumber: {
    list: () => ["sumber-data", env()] as const,
    detail: (id: string) => ["sumber-data", "detail", id, env()] as const,
    pipeline: () => ["sumber-data", "pipeline", env()] as const,
    eksekusi: (id: string) => ["sumber-data", "eksekusi", id, env()] as const,
    sampel: (id: string) => ["sumber-data", "sampel", id, env()] as const,
  },
  crawler: {
    freshness: () => ["crawler", "freshness", env()] as const,
    cycles: (limit = 24) => ["crawler", "cycles", limit, env()] as const,
    jobs: () => ["crawler", "jobs", env()] as const,
  },
  kpi: {
    list: () => ["kpi", "list", env()] as const,
    riwayat: (kode: string) => ["kpi", "riwayat", kode, env()] as const,
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
  profil: {
    saya: () => ["profil", "saya"] as const,
    aktivitas: (filter: unknown) => ["profil", "aktivitas", filter] as const,
    riwayatSesi: (status: string) => ["profil", "riwayat-sesi", status] as const,
  },
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
    lokasiSpasial: (params: unknown) => ["master", "lokasi-spasial", params] as const,
  },
};
