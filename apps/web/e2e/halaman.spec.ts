/**
 * Uji asap per peran: untuk ke-14 peran (satu akun demo per peran), setiap halaman yang boleh dibuka
 * peran itu menurut tabel izin rute aplikasi (lib/rbac/routes.ts) terbuka tanpa dialihkan, tanpa
 * respons API >= 400, dan tanpa galat JS. Halaman di luar hak peran dialihkan ke /akses-ditolak.
 *
 * Daftar halaman diturunkan dari tabel yang sama dengan penjaga rute, jadi peran atau halaman baru
 * otomatis ikut teruji, termasuk widget yang memanggil API di luar izin peran (403).
 *
 * Sejak 02-C izin yang berlaku dibaca dari basis data, dan tabel statis di atas adalah matriks BAWAAN. Spesifikasi
 * ini benar selama matriks demo tidak diubah; rbac.spec.ts memulihkan matriks bawaan di `finally` dan afterAll, dan
 * tes pertamanya memastikan server tidak menyimpang dari bawaan. Penolakan di rute halaman kini melewati satu
 * pemulihan token (GET /api/auth/refresh) sebelum /akses-ditolak, jadi URL akhirnya sama.
 */
import { canAccessRoute } from "../src/lib/rbac/routes";
import { ROLE_IDS, type Role } from "../src/lib/rbac/roles";
import {
  AKUN,
  buka,
  expect,
  masukSebagai,
  test,
  tungguTenang,
  type Akun,
} from "./fixtures";

/** Akun demo untuk setiap peran (apps/api/internal/auth/seeder/demo.go). */
const AKUN_PERAN: Record<Role, Akun> = {
  ADMIN_SISTEM: "superadmin",
  STRATEGIS_FMIPA: "strategisFmipa",
  STRATEGIS_BI: "strategisBi",
  STRATEGIS_BPS: "strategisBps",
  STRATEGIS_DJPB: "strategisDjpb",
  MANAJER_PROGRAM: "manajerProgram",
  MANAJER_DATA: "manajerData",
  MANAJER_KEBIJAKAN: "manajerKebijakan",
  MANAJER_PEMBIAYAAN: "manajerPembiayaan",
  MANAJER_RISIKO: "manajerRisiko",
  MANAJER_MUTU: "manajerMutu",
  OPS_KOORDINATOR: "koordinator",
  OPS_AGREGATOR: "agregator",
  OPS_MAHASISWA: "mahasiswa",
};

/** Semua halaman aplikasi yang membutuhkan sesi. */
const HALAMAN = [
  "/eksekutif",
  "/operasional",
  "/data/harga",
  "/sumber-data",
  "/master/komoditas",
  "/master/wilayah",
  "/master/pasar",
  "/ews",
  "/prediksi",
  "/peta",
  "/cold-storage",
  "/intervensi",
  "/policy-brief",
  "/kpi",
  "/pengguna",
  "/audit",
  "/alur-sistem",
  "/keamanan",
  "/mfa",
  "/lapangan/tugas",
  "/lapangan/riwayat",
  "/lapangan/input",
];

test("setiap peran punya akun demo", () => {
  expect(Object.keys(AKUN_PERAN).sort()).toEqual([...ROLE_IDS].sort());
});

for (const peran of ROLE_IDS) {
  const akun = AKUN_PERAN[peran];
  test.describe(`halaman ${AKUN[akun]} (${peran})`, () => {
    for (const path of HALAMAN.filter((p) => canAccessRoute(peran, p))) {
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
  ["manajerPembiayaan", "/ews"],
  ["agregator", "/peta"],
];

for (const [akun, path] of DITOLAK) {
  test(`${AKUN[akun]} ditolak di ${path}`, async ({ page }) => {
    await masukSebagai(page, akun);
    await page.goto(path);
    await expect(page).toHaveURL(/\/akses-ditolak/);
  });
}
