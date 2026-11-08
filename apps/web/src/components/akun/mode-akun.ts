/**
 * Tempat komponen keamanan akun dipakai, yang menentukan apa yang terjadi sesudah aksi berhasil:
 *  - `halaman`: halaman auth alur wajib (/ganti-password, /mfa). Berpindah ke `lanjut` sesudah sukses.
 *  - `tab`: tab /profil. Tetap di tempat; hanya data (profil, sesi) yang disegarkan.
 */
export type ModeAkun = { jenis: "halaman"; lanjut: string } | { jenis: "tab" };
