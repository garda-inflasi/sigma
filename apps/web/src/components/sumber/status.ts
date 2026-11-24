import type { SumberData } from "@/lib/data/types";
import type { Tone } from "@/lib/design/tokens";

export const STATUS_SUMBER: Record<
  SumberData["status"],
  { tone: Tone; label: string; urutan: number }
> = {
  GAGAL: { tone: "defisit", label: "Gagal", urutan: 0 },
  TERTUNDA: { tone: "waspada", label: "Tertunda", urutan: 1 },
  BELUM_AKTIF: { tone: "netral", label: "Belum aktif", urutan: 2 },
  DIJEDA: { tone: "netral", label: "Dijeda", urutan: 3 },
  SEHAT: { tone: "normal", label: "Sehat", urutan: 4 },
};

/**
 * Langkah berikutnya menurut status dan hak pengguna. TERTUNDA berarti eksekusi terakhir hanya sebagian berhasil,
 * atau sumber baru dilanjutkan dan menunggu eksekusi berikutnya.
 */
export function langkahSumber(s: SumberData, bolehKelola: boolean): string {
  switch (s.status) {
    case "GAGAL":
      if (!bolehKelola) return "Laporkan ke pengelola data";
      return s.bisaDijalankan
        ? "Periksa galat, lalu jalankan ulang"
        : "Siapkan konektor agar bisa dijalankan";
    case "TERTUNDA":
      if (!s.bisaDijalankan) return "Konektor belum tersedia";
      return bolehKelola
        ? "Periksa hasil terakhir, jalankan bila perlu"
        : "Pantau eksekusi berikutnya";
    case "BELUM_AKTIF":
      if (!s.bisaDijalankan) return "Konektor belum tersedia";
      return bolehKelola ? "Jalankan pertama kali" : "Menunggu dijalankan";
    case "DIJEDA":
      return bolehKelola ? "Centang baris, lalu Lanjutkan" : "Dijeda pengelola";
    case "SEHAT":
      return "Tidak perlu tindakan";
  }
}
