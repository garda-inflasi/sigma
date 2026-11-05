/**
 * Pelacak pemilik data offline. Logout lewat tombol Keluar membersihkan data perangkat, tetapi sesi juga
 * bisa berakhir tanpa itu (token kedaluwarsa, pengguna lain masuk di perangkat yang sama). Karena itu
 * IndexedDB mencatat id pengguna pemilik antrean (`KUNCI_PEMILIK`); begitu pengguna yang berbeda memuat
 * aplikasi, data milik pengguna sebelumnya dihapus SEBELUM antrean dipulihkan ke memori.
 *
 * Pemulihan antrean yang menunggu hasil ini penting: mesin sinkronisasi mengirim entri memakai cookie sesi
 * yang sedang aktif, jadi antrean milik A yang terpulihkan di sesi B akan terkirim atas nama B.
 *
 * Pengguna yang SAMA masuk lagi setelah sesinya habis tetap memegang antreannya (entri luring yang belum
 * terkirim tidak boleh hilang hanya karena token kedaluwarsa).
 */

export type StatusPemilik =
  /** Pemilik tercatat sama dengan pengguna sekarang. */
  | "sama"
  /** Belum ada pemilik tercatat (perangkat baru, sesudah keluar, atau data sebelum fitur ini): pengguna sekarang menjadi pemilik. */
  | "baru"
  /** Pemilik berbeda: data pengguna sebelumnya dihapus lalu pengguna sekarang menjadi pemilik. */
  | "dialihkan"
  /** IndexedDB tidak terbaca (mis. mode privat): tidak ada data yang bisa bocor, aplikasi berjalan tanpa persistensi. */
  | "tak-terbaca"
  /** Pemilik berbeda tetapi pembersihan gagal: data lama tidak boleh dipulihkan. */
  | "gagal-bersihkan";

export interface HasilPemilik {
  status: StatusPemilik;
  /** Antrean di penyimpanan boleh dipulihkan ke memori untuk pengguna ini. */
  aman: boolean;
  /** Entri belum terkirim milik pengguna sebelumnya yang ikut terhapus (hanya pada "dialihkan"). */
  entriDibuang: number;
}

export interface DepsPemilik {
  bacaPemilik(): Promise<string | undefined>;
  tulisPemilik(id: string): Promise<void>;
  hitungBelumTerkirim(): Promise<number>;
  /** Membersihkan data lokal; tidak boleh melempar, galat dilaporkan lewat `galat`. */
  bersihkan(): Promise<{ galat: string[] }>;
}

/** Menentukan apakah data offline di perangkat milik `userId`, membersihkannya bila bukan. Tidak pernah melempar. */
export async function pastikanPemilik(
  userId: string,
  deps: DepsPemilik,
): Promise<HasilPemilik> {
  let pemilik: string | undefined;
  try {
    pemilik = await deps.bacaPemilik();
  } catch {
    return { status: "tak-terbaca", aman: true, entriDibuang: 0 };
  }

  if (pemilik === userId) {
    return { status: "sama", aman: true, entriDibuang: 0 };
  }

  if (pemilik === undefined) {
    // Penanda yang gagal ditulis hanya berarti diulang pada pemuatan berikutnya.
    await deps.tulisPemilik(userId).catch(() => undefined);
    return { status: "baru", aman: true, entriDibuang: 0 };
  }

  const entriDibuang = await deps.hitungBelumTerkirim().catch(() => 0);
  const { galat } = await deps.bersihkan();
  if (galat.length > 0) {
    // Penanda lama dibiarkan: pemuatan berikutnya mencoba membersihkan lagi.
    return { status: "gagal-bersihkan", aman: false, entriDibuang: 0 };
  }
  await deps.tulisPemilik(userId).catch(() => undefined);
  return { status: "dialihkan", aman: true, entriDibuang };
}
