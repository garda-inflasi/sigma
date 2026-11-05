/**
 * Keluar dari sesi: mencabut token di server lewat BFF (`/api/auth/logout` menghapus cookie sesi dan
 * memasukkan token ke daftar cabut), lalu memuat ulang halaman masuk. Pemuatan penuh (bukan navigasi
 * klien) membuang seluruh cache TanStack Query dan state in-memory milik pengguna sebelumnya.
 * Data offline di perangkat (IndexedDB, cache halaman berpengguna) dibersihkan lewat `bersihkanLokal`.
 */
export const LOGIN_URL = "/masuk";

export interface KeluarDeps {
  /** Memanggil BFF logout; galat jaringan tidak boleh menahan pengguna di halaman. */
  panggilLogout: () => Promise<unknown>;
  /** Memuat ulang penuh ke alamat tujuan. */
  pindah: (url: string) => void;
  /**
   * Membersihkan data milik pengguna di perangkat (antrean offline, cache). Dijalankan sesudah logout server
   * dan tetap dijalankan bila logout gagal; galatnya tidak menahan pengguna.
   */
  bersihkanLokal?: () => Promise<unknown>;
  /** Batas tunggu logout server sebelum pengguna dilepas ke halaman masuk (bawaan 8 detik). */
  batasWaktuMs?: number;
}

export const BATAS_WAKTU_LOGOUT_MS = 8_000;

/** Selalu berakhir di halaman masuk, walau pencabutan di server gagal (mis. sesi sudah kedaluwarsa). */
export async function keluar({
  panggilLogout,
  pindah,
  bersihkanLokal,
  batasWaktuMs = BATAS_WAKTU_LOGOUT_MS,
}: KeluarDeps): Promise<void> {
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    // Jaringan yang menggantung tidak boleh mengurung pengguna di halaman yang sudah ia tinggalkan.
    await Promise.race([
      panggilLogout(),
      new Promise((resolve) => {
        timer = setTimeout(resolve, batasWaktuMs);
      }),
    ]);
  } catch {
    // Sesi sudah kedaluwarsa atau server tak terjangkau: tetap arahkan ke halaman masuk.
  } finally {
    clearTimeout(timer);
  }
  try {
    await bersihkanLokal?.();
  } catch {
    // Pembersihan lokal gagal total: tetap keluar; pengguna berikutnya dibersihkan lewat pelacak pemilik.
  }
  pindah(LOGIN_URL);
}
