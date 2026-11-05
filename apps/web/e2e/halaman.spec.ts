/**
 * Uji asap per peran: setiap halaman yang menjadi hak peran itu terbuka tanpa dialihkan,
 * tanpa respons API >= 400, dan tanpa galat JS; halaman di luar hak peran dialihkan ke
 * /akses-ditolak.
 */
import {
  AKUN,
  buka,
  expect,
  masukSebagai,
  test,
  tungguTenang,
  type Akun,
} from "./fixtures";

const RENCANA: Partial<Record<Akun, string[]>> = {
  superadmin: [
    "/pengguna",
    "/audit",
    "/master/komoditas",
    "/master/wilayah",
    "/master/pasar",
    "/sumber-data",
    "/mfa",
  ],
  manajerData: [
    "/operasional",
    "/data/harga",
    "/ews",
    "/peta",
    "/prediksi",
    "/intervensi",
    "/sumber-data",
    "/kpi",
  ],
  strategisDjpb: [
    "/eksekutif",
    "/policy-brief",
    "/intervensi",
    "/kpi",
    "/cold-storage",
  ],
  strategisFmipa: [
    "/eksekutif",
    "/operasional",
    "/ews",
    "/prediksi",
    "/intervensi",
    "/policy-brief",
    "/master/komoditas",
    "/peta",
    "/alur-sistem",
  ],
  manajerMutu: ["/cold-storage", "/kpi"],
  koordinator: ["/data/harga", "/operasional", "/lapangan/tugas"],
  manajerKebijakan: ["/policy-brief", "/intervensi"],
  mahasiswa: ["/lapangan/tugas", "/lapangan/riwayat", "/lapangan/input"],
};

for (const [akun, halaman] of Object.entries(RENCANA) as [Akun, string[]][]) {
  test.describe(`halaman ${AKUN[akun]}`, () => {
    for (const path of halaman) {
      test(`${path} terbuka`, async ({ page }) => {
        await masukSebagai(page, akun);
        await buka(page, path);
        await tungguTenang(page);
        await expect(page.locator("body")).not.toContainText(
          "Terjadi kesalahan",
        );
      });
    }
  });
}

const DITOLAK: [Akun, string][] = [
  ["superadmin", "/kpi"],
  ["strategisFmipa", "/pengguna"],
  ["mahasiswa", "/operasional"],
];

for (const [akun, path] of DITOLAK) {
  test(`${AKUN[akun]} ditolak di ${path}`, async ({ page }) => {
    await masukSebagai(page, akun);
    await page.goto(path);
    await expect(page).toHaveURL(/\/akses-ditolak/);
  });
}
