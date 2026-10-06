import type { Siklus } from "@/lib/design/tokens";
import type { Permission } from "@/lib/rbac/permissions";

export interface Tahap {
  id: string;
  judul: string;
  petunjuk: string;
  /** Izin untuk memindahkan kartu MASUK ke tahap ini (default: intervensi.kelola). */
  izinMasuk?: Permission;
}

/** Tahapan alur kerja sesuai Layer 4: Smart Intervention & Workflow Engine. */
export const SIKLUS_TAHAP: Record<Siklus, Tahap[]> = {
  normal: [
    { id: "n-pantau", judul: "Pantau rutin data produsen", petunjuk: "Agregasi harian dari agen lapangan" },
    { id: "n-verifikasi", judul: "Verifikasi silang BPS", petunjuk: "Cocokkan dengan rilis statistik resmi" },
    { id: "n-jadwal", judul: "Optimasi jadwal pengiriman", petunjuk: "Susun rute & slot armada" },
    { id: "n-agregasi", judul: "Agregasi volume reguler", petunjuk: "Konsolidasi pasokan produsen" },
    { id: "n-selesai", judul: "Selesai", petunjuk: "Ditutup dan terdokumentasi" },
  ],
  tekanan: [
    { id: "t-ews", judul: "EWS terpicu", petunjuk: "Alarm otomatis dari model" },
    { id: "t-verifikasi", judul: "Verifikasi cepat BI/BPS", petunjuk: "Konfirmasi sinyal dengan data resmi" },
    { id: "t-petakan", judul: "Petakan suplai sentra surplus", petunjuk: "Butuh persetujuan Dewan Pengarah", izinMasuk: "intervensi.setujui" },
    { id: "t-logistik", judul: "Usulan intervensi logistik kilat", petunjuk: "Rute & volume pengiriman darurat" },
    { id: "t-fiskal", judul: "Dukungan instrumen belanja", petunjuk: "Persetujuan Kanwil DJPb", izinMasuk: "intervensi.setujui_fiskal" },
    { id: "t-selesai", judul: "Selesai", petunjuk: "Harga kembali ke koridor", izinMasuk: "intervensi.setujui" },
  ],
  surplus: [
    { id: "s-deteksi", judul: "Deteksi over-supply", petunjuk: "Harga produsen jatuh di bawah koridor" },
    { id: "s-coldstorage", judul: "Arahkan ke cold storage", petunjuk: "Alokasi kapasitas & shelf-life" },
    { id: "s-olahan", judul: "Rekomendasi pengolahan turunan", petunjuk: "Uji mutu oleh Unit Mutu & Pascapanen" },
    { id: "s-ekspansi", judul: "Ekspansi ke pembeli antardaerah", petunjuk: "Pencocokan pembeli & logistik" },
    { id: "s-selesai", judul: "Selesai", petunjuk: "Surplus terserap", izinMasuk: "intervensi.setujui" },
  ],
};

export const SIKLUS_URUTAN: Siklus[] = ["normal", "tekanan", "surplus"];
