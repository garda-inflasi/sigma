/**
 * Kunci IndexedDB milik aplikasi (idb-keyval, basis data `keyval-store`, store `keyval`).
 * Dipakai bersama oleh penyimpanan (field-queue-store), pembersihan saat keluar, dan pelacak pemilik data,
 * supaya nama kunci tidak bisa berbeda antara penulis dan pembacanya.
 *
 *  - `garda-antrean-lapangan`: antrean entri harga (zustand persist, lihat stores/field-queue-store.ts)
 *  - `garda-data-acuan`: cache data acuan unduhan server (offline/acuan-server.ts)
 *  - `garda-pemilik-offline`: id pengguna pemilik data di atas (offline/pemilik.ts)
 */
export const KUNCI_ANTREAN = "garda-antrean-lapangan";
export const KUNCI_ACUAN = "garda-data-acuan";
export const KUNCI_PEMILIK = "garda-pemilik-offline";
