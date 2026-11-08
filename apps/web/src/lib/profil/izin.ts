/**
 * Label izin untuk daftar "hak akses" di profil. Teks dan kelompoknya mengikuti katalog izin di API Go
 * (usecase/rbac_katalog.go); `Record<Permission, …>` membuat TypeScript menolak izin baru yang belum diberi label.
 * Izin yang tidak dikenal (token dari API yang lebih baru) tetap tampil dengan kodenya.
 */
import type { Permission } from "@/lib/rbac/permissions";

export interface InfoIzin {
  label: string;
  kelompok: string;
}

export const INFO_IZIN: Record<Permission, InfoIzin> = {
  "dashboard.operasional": {
    label: "Membuka dasbor operasional",
    kelompok: "Dasbor",
  },
  "dashboard.eksekutif": {
    label: "Membuka dasbor eksekutif",
    kelompok: "Dasbor",
  },
  "harga.lihat": { label: "Melihat data harga pangan", kelompok: "Harga" },
  "harga.input": { label: "Menginput harga dari lapangan", kelompok: "Harga" },
  "harga.validasi": { label: "Memvalidasi entri harga", kelompok: "Harga" },
  "harga.publikasi": { label: "Mempublikasikan data harga", kelompok: "Harga" },
  "harga.ekspor": { label: "Mengekspor data harga", kelompok: "Harga" },
  "tugas.lihat_sendiri": {
    label: "Melihat tugas lapangan milik sendiri",
    kelompok: "Tugas lapangan",
  },
  "tugas.kelola": {
    label: "Membuat dan mengelola tugas lapangan",
    kelompok: "Tugas lapangan",
  },
  "ews.lihat": {
    label: "Melihat peringatan dini (EWS)",
    kelompok: "Peringatan dini",
  },
  "ews.akui": {
    label: "Mengakui peringatan dini",
    kelompok: "Peringatan dini",
  },
  "ews.eskalasi": {
    label: "Mengeskalasi peringatan dini",
    kelompok: "Peringatan dini",
  },
  "ews.atur_ambang": {
    label: "Mengatur ambang peringatan dini",
    kelompok: "Peringatan dini",
  },
  "prediksi.lihat": {
    label: "Melihat prediksi dan penjelasan model",
    kelompok: "Prediksi",
  },
  "model.kelola": {
    label: "Mengelola model prediksi dan siklus MLOps",
    kelompok: "Prediksi",
  },
  "spasial.lihat": {
    label: "Melihat peta dan analisis spasial",
    kelompok: "Spasial dan logistik",
  },
  "logistik.kelola": {
    label: "Merencanakan dan menyimpan rute distribusi",
    kelompok: "Spasial dan logistik",
  },
  "coldstorage.lihat": {
    label: "Melihat data cold storage",
    kelompok: "Cold storage",
  },
  "coldstorage.kelola": {
    label: "Mengelola fasilitas dan stok cold storage",
    kelompok: "Cold storage",
  },
  "fiskal.lihat": {
    label: "Melihat data dan simulasi fiskal",
    kelompok: "Fiskal",
  },
  "intervensi.lihat": {
    label: "Melihat daftar intervensi",
    kelompok: "Intervensi",
  },
  "intervensi.kelola": {
    label: "Menyusun dan memindahkan tahap intervensi",
    kelompok: "Intervensi",
  },
  "intervensi.setujui": {
    label: "Menyetujui intervensi",
    kelompok: "Intervensi",
  },
  "intervensi.setujui_fiskal": {
    label: "Menyetujui dukungan fiskal intervensi",
    kelompok: "Intervensi",
  },
  "brief.lihat": { label: "Melihat policy brief", kelompok: "Policy brief" },
  "brief.susun": { label: "Menyusun policy brief", kelompok: "Policy brief" },
  "brief.setujui": {
    label: "Menyetujui policy brief",
    kelompok: "Policy brief",
  },
  "kpi.lihat": { label: "Melihat indikator kinerja (KPI)", kelompok: "KPI" },
  "master.lihat": { label: "Melihat master data", kelompok: "Master data" },
  "master.ajukan": {
    label: "Mengajukan perubahan master data",
    kelompok: "Master data",
  },
  "master.setujui": {
    label: "Menyetujui perubahan master data",
    kelompok: "Master data",
  },
  "sumber.lihat": { label: "Melihat sumber data", kelompok: "Sumber data" },
  "sumber.kelola": {
    label: "Mengelola sumber data dan ingesti",
    kelompok: "Sumber data",
  },
  "pengguna.kelola": {
    label: "Mengelola pengguna, peran, matriks izin, dan izin langsung",
    kelompok: "Pengguna dan audit",
  },
  "audit.lihat": {
    label: "Melihat jejak audit",
    kelompok: "Pengguna dan audit",
  },
};

/** Kelompok bagi izin di luar katalog; selalu paling akhir. */
export const KELOMPOK_LAINNYA = "Lainnya";

export interface ItemIzin {
  id: string;
  label: string;
}

export interface KelompokIzin {
  nama: string;
  izin: ItemIzin[];
}

const adalahIzinKatalog = (id: string): id is Permission =>
  Object.hasOwn(INFO_IZIN, id);

/**
 * Mengelompokkan izin efektif: kelompok dan urutan izin mengikuti katalog (urutan `INFO_IZIN`); izin ganda
 * dibuang, izin yang tidak dikenal masuk "Lainnya" di akhir dengan urutan kemunculannya.
 */
export function kelompokkanIzin(izin: readonly string[]): KelompokIzin[] {
  const dimiliki = new Set(izin);
  const peta = new Map<string, ItemIzin[]>();
  const tambah = (kelompok: string, item: ItemIzin) =>
    peta.set(kelompok, [...(peta.get(kelompok) ?? []), item]);

  for (const [id, info] of Object.entries(INFO_IZIN)) {
    if (dimiliki.has(id)) tambah(info.kelompok, { id, label: info.label });
  }
  for (const id of dimiliki) {
    if (!adalahIzinKatalog(id)) tambah(KELOMPOK_LAINNYA, { id, label: id });
  }
  return [...peta].map(([nama, items]) => ({ nama, izin: items }));
}
