/**
 * Keluar dari sesi: mencabut token di server lewat BFF (`/api/auth/logout` menghapus cookie sesi dan
 * memasukkan token ke daftar cabut), lalu memuat ulang halaman masuk. Pemuatan penuh (bukan navigasi
 * klien) membuang seluruh cache TanStack Query dan state in-memory milik pengguna sebelumnya.
 */
export const LOGIN_URL = "/masuk";

export interface KeluarDeps {
  /** Memanggil BFF logout; galat jaringan tidak boleh menahan pengguna di halaman. */
  panggilLogout: () => Promise<unknown>;
  /** Memuat ulang penuh ke alamat tujuan. */
  pindah: (url: string) => void;
}

/** Selalu berakhir di halaman masuk, walau pencabutan di server gagal (mis. sesi sudah kedaluwarsa). */
export async function keluar({
  panggilLogout,
  pindah,
}: KeluarDeps): Promise<void> {
  try {
    await panggilLogout();
  } catch {
    // Sesi sudah kedaluwarsa atau server tak terjangkau: tetap arahkan ke halaman masuk.
  }
  pindah(LOGIN_URL);
}
