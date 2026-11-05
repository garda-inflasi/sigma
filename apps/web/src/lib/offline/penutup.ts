/**
 * Penutup penulisan IndexedDB. Setelah pengguna keluar, sinkronisasi atau unduhan acuan yang masih berjalan
 * tidak boleh menulis hasilnya ke IndexedDB, karena itu menghidupkan kembali data yang baru dihapus.
 * Penutup berlaku sampai halaman dimuat ulang (keluar selalu berakhir dengan pemuatan penuh ke /masuk).
 */
let tertutup = false;

export function tutupPenulisanIdb(): void {
  tertutup = true;
}

export function penulisanIdbTertutup(): boolean {
  return tertutup;
}
