/**
 * Logika murni konfirmasi keluar bagi pengguna yang masih punya entri lapangan belum terkirim.
 * Tampilan dialognya ada di components/auth/dialog-keluar.tsx; sambungan ke store dan jaringan di
 * lib/auth/keluar-klien.ts.
 */

export type AksiKeluar =
  /** Tidak ada entri tertahan: keluar langsung. */
  | { jenis: "langsung" }
  /** Ada `sisa` entri yang akan hilang: minta keputusan pengguna. */
  | { jenis: "konfirmasi"; sisa: number };

/** Memutuskan apakah keluar boleh langsung atau perlu konfirmasi. Gagal membaca antrean tidak menahan pengguna. */
export async function tentukanAksiKeluar(
  hitungBelumTerkirim: () => Promise<number>,
): Promise<AksiKeluar> {
  let sisa = 0;
  try {
    sisa = await hitungBelumTerkirim();
  } catch {
    // Penyimpanan tak terbaca: tak ada yang bisa dikonfirmasi.
  }
  return sisa > 0 ? { jenis: "konfirmasi", sisa } : { jenis: "langsung" };
}

export type HasilKirimDulu =
  /** Semua entri sudah terkirim: aman keluar. */
  | { jenis: "selesai" }
  /** Perangkat offline, tidak ada yang dikirim. */
  | { jenis: "offline"; sisa: number }
  /** Sebagian belum terkirim (gagal, konflik, atau draf yang tidak dikirim otomatis). */
  | { jenis: "sisa"; sisa: number }
  /** Pengiriman melempar galat tak terduga. */
  | { jenis: "galat"; sisa: number; pesan: string };

export interface DepsKirimDulu {
  online: () => boolean;
  /** Menyiapkan antrean di memori (memulihkan dari penyimpanan bila belum). */
  siapkan: () => Promise<void>;
  /** Menjalankan sinkronisasi dan menunggu selesai, termasuk sinkronisasi yang sedang berjalan. */
  kirim: () => Promise<void>;
  /** Jumlah entri yang masih belum sampai di server. */
  hitungSisa: () => number;
}

/** "Kirim dulu": coba sinkronkan sekarang, lalu laporkan berapa yang masih tertinggal. */
export async function kirimLaluPeriksa(
  deps: DepsKirimDulu,
): Promise<HasilKirimDulu> {
  try {
    await deps.siapkan();
    if (!deps.online()) return { jenis: "offline", sisa: deps.hitungSisa() };
    await deps.kirim();
  } catch (e) {
    return {
      jenis: "galat",
      sisa: safeSisa(deps),
      pesan: e instanceof Error ? e.message : "Gagal mengirim",
    };
  }
  const sisa = deps.hitungSisa();
  return sisa === 0 ? { jenis: "selesai" } : { jenis: "sisa", sisa };
}

function safeSisa(deps: DepsKirimDulu): number {
  try {
    return deps.hitungSisa();
  } catch {
    return 0;
  }
}

/** Kalimat hasil "Kirim dulu" yang ditampilkan di dialog; null bila tidak ada yang perlu dikatakan. */
export function pesanHasilKirim(h: HasilKirimDulu): string | null {
  switch (h.jenis) {
    case "selesai":
      return null;
    case "offline":
      return "Perangkat sedang offline, entri belum bisa dikirim. Sambungkan ke jaringan lalu coba lagi.";
    case "sisa":
      return `Masih ada ${h.sisa} entri yang belum terkirim (gagal, konflik data, atau draf). Periksa di Riwayat, atau tetap keluar dan entri itu dihapus.`;
    case "galat":
      return `Pengiriman gagal: ${h.pesan}. Masih ada ${h.sisa} entri yang belum terkirim.`;
  }
}
