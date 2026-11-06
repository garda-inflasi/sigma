/**
 * Pembantu e2e tabel /pengguna. Tabelnya dijalankan server: kata kunci diketik di kotak cari, dikirim ke
 * `GET /api/pengguna?search=…` sesudah jeda ketik 300 ms, dan hasilnya (satu halaman) menggantikan baris.
 */
import type { Locator, Page } from "@playwright/test";
import { expect, tungguTenang } from "./fixtures";

export const PLACEHOLDER_CARI_PENGGUNA =
  "Cari nama, email, peran, atau lembaga";

export const kotakCariPengguna = (page: Page): Locator =>
  page.getByPlaceholder(PLACEHOLDER_CARI_PENGGUNA);

/**
 * Mengetik kata kunci di kotak cari tabel pengguna lalu menunggu sampai server menjawab. Menunggu keadaan,
 * bukan permintaan: kueri yang sama dalam 20 detik dijawab dari cache React Query tanpa permintaan baru, dan
 * jeda ketik membuat baris lama masih tampil beberapa saat.
 */
export async function cariPengguna(page: Page, teks: string): Promise<void> {
  await kotakCariPengguna(page).fill(teks);
  await expect
    .poll(() => new URL(page.url()).searchParams.get("search") ?? "", {
      message: "kata kunci masuk ke URL sesudah jeda ketik",
    })
    .toBe(teks.trim());
  await tungguTenang(page);
}
