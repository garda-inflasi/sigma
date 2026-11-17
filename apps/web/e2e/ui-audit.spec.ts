/**
 * Audit UI/UX (redesain "Clean Clinical"): setiap halaman utama dibuka per peran dan diperiksa tanpa galat konsol,
 * tanpa tata letak patah (gulir horizontal), dengan token gaya baru terpasang (kanvas #f8f9fa, sidebar + remah roti),
 * dan dengan animasi masuk yang selesai (opacity 1). Tangkapan layar disimpan di test-results/ui-audit/ sebagai bukti.
 *
 * Interaksi yang diuji: ciutkan sidebar, menu ruang kerja, drawer rincian (Sheet), grup tabel yang dapat dilipat,
 * bilah aksi massal mengambang, tab tampilan, modal dengan kartu pilihan, tab profil, dan reduced-motion.
 *
 * Jalankan terhadap stack yang hidup: E2E_BASE_URL=http://localhost:3000 pnpm e2e ui-audit
 */
import { mkdirSync } from "node:fs";
import { join } from "node:path";
import type { Page } from "@playwright/test";
import { canAccessRoute } from "../src/lib/rbac/routes";
import type { Role } from "../src/lib/rbac/roles";
import { buka, expect, masukSebagai, test, tungguTenang, type Akun } from "./fixtures";

const FOLDER = join("test-results", "ui-audit");
mkdirSync(FOLDER, { recursive: true });

const slug = (s: string) => s.replace(/^\//, "").replace(/[/?=&]+/g, "_") || "beranda";

/** Tangkapan layar halaman penuh sebagai bukti visual. */
async function potret(page: Page, nama: string, penuh = true) {
  await page.screenshot({ path: join(FOLDER, `${nama}.png`), fullPage: penuh });
}

/** Animasi masuk selesai: pembungkus halaman buram penuh dan judul terlihat. */
async function pastikanTampil(page: Page) {
  await expect
    .poll(
      () =>
        page.evaluate(() => {
          const w = document.querySelector("#konten > div");
          return w ? Number(getComputedStyle(w).opacity) : 1;
        }),
      { message: "animasi masuk halaman tersangkut (opacity < 1)" },
    )
    .toBe(1);
  await expect(page.locator("h1").first()).toBeVisible();
}

/** Tidak ada gulir horizontal pada dokumen (toleransi 1px). */
async function tanpaGulirHorizontal(page: Page) {
  const lebih = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(lebih, "dokumen melebar melewati viewport").toBeLessThanOrEqual(1);
}

/** Token gaya baru terpasang: kanvas off-white, bukan latar lama. */
async function gayaBaruTerpasang(page: Page) {
  const bg = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
  expect(bg).toBe("rgb(248, 249, 250)");
}

// Indikator pengembangan Next ("N Insight") menumpuk di pojok kiri bawah dan menutupi tombol sidebar saat `next dev`.
// Elemen ini tidak ada di build produksi, jadi menyembunyikannya aman dan tidak mengubah apa yang diuji.
test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    const gaya = document.createElement("style");
    gaya.textContent = "nextjs-portal{display:none!important}";
    document.addEventListener("DOMContentLoaded", () => document.head.appendChild(gaya));
  });
});

interface Skenario {
  peran: Role;
  akun: Akun;
  halaman: string[];
}

const SKENARIO: Skenario[] = [
  {
    peran: "MANAJER_PROGRAM",
    akun: "manajerProgram",
    halaman: ["/operasional", "/eksekutif", "/data/harga", "/ews", "/prediksi", "/peta", "/cold-storage", "/intervensi", "/policy-brief", "/kpi", "/alur-sistem", "/profil"],
  },
  {
    peran: "ADMIN_SISTEM",
    akun: "superadmin",
    halaman: ["/pengguna", "/audit", "/master/komoditas", "/master/perubahan", "/sumber-data", "/profil?tab=aktivitas", "/profil?tab=keamanan"],
  },
];

test.describe("audit halaman konsol", () => {
  for (const { peran, akun, halaman } of SKENARIO) {
    for (const path of halaman.filter((p) => canAccessRoute(peran, p.split("?")[0]))) {
      test(`${peran} ${path}`, async ({ page }) => {
        await masukSebagai(page, akun);
        await buka(page, path);
        await tungguTenang(page);
        await pastikanTampil(page);
        await expect(page.locator("body")).not.toContainText("Terjadi kesalahan");
        await gayaBaruTerpasang(page);
        await tanpaGulirHorizontal(page);
        // Cangkang konsol: sidebar bergrup dan remah roti.
        await expect(page.getByRole("navigation", { name: "Navigasi utama" })).toBeVisible();
        await expect(page.getByRole("navigation", { name: "Remah roti" })).toBeVisible();
        await potret(page, `${peran}-${slug(path)}`);
      });
    }
  }
});

test.describe("audit halaman otentikasi dan galat", () => {
  for (const path of ["/masuk", "/lupa-password", "/akses-ditolak", "/offline"]) {
    test(`publik ${path}`, async ({ page }) => {
      await page.goto(path, { waitUntil: "domcontentloaded" });
      await tungguTenang(page);
      await expect(page.locator("h1, h2").first()).toBeVisible();
      await tanpaGulirHorizontal(page);
      await potret(page, `publik-${slug(path)}`);
    });
  }

  test("halaman tidak ada menampilkan keadaan 404 bergaya", async ({ page, galat }) => {
    galat.izinkan(/404/);
    await masukSebagai(page, "manajerProgram");
    await page.goto("/rute-yang-tidak-ada", { waitUntil: "domcontentloaded" });
    await expect(page.locator("body")).toContainText(/tidak ditemukan|404/i);
    await potret(page, "publik-404");
  });
});

test.describe("audit aplikasi lapangan (mobile)", () => {
  test.use({ viewport: { width: 390, height: 844 } });
  for (const path of ["/lapangan/tugas", "/lapangan/input", "/lapangan/riwayat"]) {
    test(`mahasiswa ${path}`, async ({ page }) => {
      await masukSebagai(page, "mahasiswa");
      await buka(page, path);
      await tungguTenang(page);
      await tanpaGulirHorizontal(page);
      await expect(page.locator("body")).not.toContainText("Terjadi kesalahan");
      await potret(page, `mobile-lapangan-${slug(path)}`);
    });
  }
});

test.describe("interaksi UI baru", () => {
  test("sidebar: ciutkan dan bentangkan; menu ruang kerja terbuka", async ({ page }) => {
    await masukSebagai(page, "manajerProgram");
    await buka(page, "/operasional");
    await tungguTenang(page);
    await pastikanTampil(page);
    const aside = page.locator("aside").first();
    await expect(aside).toBeVisible();
    // Label kelompok kecil kapital.
    await expect(aside.getByText("Ringkasan", { exact: true })).toBeVisible();

    const lebarAwal = (await aside.boundingBox())!.width;
    await page.getByRole("button", { name: "Ciutkan sidebar" }).click();
    await expect.poll(async () => (await aside.boundingBox())!.width).toBeLessThan(lebarAwal - 100);
    await potret(page, "interaksi-sidebar-ciut", false);
    await page.getByRole("button", { name: "Bentangkan sidebar" }).click();
    await expect.poll(async () => (await aside.boundingBox())!.width).toBeGreaterThan(220);

    await page.getByRole("button", { name: /Ruang kerja/ }).click();
    await expect(page.getByText("Wilayah kerja")).toBeVisible();
    await expect(page.getByRole("menuitem", { name: /Provinsi Aceh/ })).toBeVisible();
    await potret(page, "interaksi-ruang-kerja", false);
    await page.keyboard.press("Escape");
  });

  test("validasi harga: grup lipat, bilah aksi massal, dan drawer rincian", async ({ page }) => {
    await masukSebagai(page, "manajerProgram");
    await buka(page, "/data/harga");
    await tungguTenang(page);
    await pastikanTampil(page);

    // Grup per status dengan badge jumlah; melipat menyembunyikan barisnya.
    const pemicu = page.locator("tbody tr button[aria-expanded]").first();
    await expect(pemicu).toHaveAttribute("aria-expanded", "true");
    const jumlahAwal = await page.locator("tbody tr").count();
    await pemicu.click();
    await expect(pemicu).toHaveAttribute("aria-expanded", "false");
    await expect.poll(() => page.locator("tbody tr").count()).toBeLessThan(jumlahAwal);
    await pemicu.click();
    await expect(pemicu).toHaveAttribute("aria-expanded", "true");
    await expect.poll(() => page.locator("tbody tr").count()).toBe(jumlahAwal);

    // Bilah aksi massal: gelap, mengambang di bawah, muncul saat ada pilihan, hilang saat dibatalkan.
    const centang = page.getByRole("checkbox", { name: "Pilih baris" });
    await centang.nth(0).click();
    await centang.nth(1).click();
    const bar = page.getByRole("toolbar", { name: /Aksi untuk 2 entri terpilih/ });
    await expect(bar).toBeVisible();
    await expect.poll(() => bar.evaluate((e) => getComputedStyle(e).opacity)).toBe("1");
    expect(await bar.evaluate((e) => getComputedStyle(e).backgroundColor)).toBe("rgb(24, 24, 27)");
    await potret(page, "interaksi-bulk-bar", false);
    await bar.getByRole("button", { name: /Batalkan pilihan/ }).click();
    await expect(bar).toBeHidden();

    // Drawer rincian: klik baris, tanpa pindah halaman.
    const url = page.url();
    await page.locator("tbody tr.row-in").nth(2).click();
    const drawer = page.getByRole("dialog");
    await expect(drawer).toBeVisible();
    await expect(drawer.getByText("Rincian", { exact: true })).toBeVisible();
    await expect(drawer.getByText("Progres alur")).toBeVisible();
    await expect(drawer.getByRole("progressbar")).toBeVisible();
    expect(page.url()).toBe(url);
    // Tunggu drawer selesai meluncur masuk (tepi kanannya di dalam viewport) sebelum difoto.
    await expect
      .poll(async () => {
        const kotak = await drawer.boundingBox();
        return kotak ? Math.round(kotak.x + kotak.width) : Infinity;
      })
      .toBeLessThanOrEqual(1440);
    await page.waitForTimeout(250);
    await potret(page, "interaksi-drawer-harga", false);
    await page.keyboard.press("Escape");
    await expect(drawer).toBeHidden();
  });

  test("intervensi: tab tampilan dan modal dengan kartu pilihan", async ({ page }) => {
    await masukSebagai(page, "manajerProgram");
    await buka(page, "/intervensi");
    await tungguTenang(page);
    await pastikanTampil(page);

    const tabs = page.getByRole("tablist", { name: "Tampilan intervensi" });
    await expect(tabs.getByRole("tab", { name: /Papan/ })).toHaveAttribute("aria-selected", "true");
    await tabs.getByRole("tab", { name: /Tabel/ }).click();
    await expect(tabs.getByRole("tab", { name: /Tabel/ })).toHaveAttribute("aria-selected", "true");
    await expect(page.locator("table")).toBeVisible();
    await tabs.getByRole("tab", { name: /Papan/ }).click();

    await page.getByRole("button", { name: /Tambah intervensi/ }).click();
    const modal = page.getByRole("dialog");
    await expect(modal).toBeVisible();
    const kartu = modal.getByRole("radiogroup", { name: "Siklus" });
    await expect(kartu.getByRole("radio")).toHaveCount(3);
    await kartu.getByText("Surplus").first().click();
    await expect(kartu.getByRole("radio", { checked: true })).toHaveCount(1);
    await potret(page, "interaksi-modal-intervensi", false);
    await page.keyboard.press("Escape");
    await expect(modal).toBeHidden();
  });

  test("profil: tab Aktivitas Akun terbuka dari tablist", async ({ page }) => {
    await masukSebagai(page, "superadmin");
    await buka(page, "/profil");
    await tungguTenang(page);
    await pastikanTampil(page);
    await page.getByRole("tab", { name: "Aktivitas Akun" }).click();
    await expect(page).toHaveURL(/tab=aktivitas/);
    await expect(page.getByRole("group", { name: "Saring aktivitas" })).toBeVisible();
    await tungguTenang(page);
    await potret(page, "interaksi-profil-aktivitas", false);
  });

  test("navigasi mobile: laci menu terbuka dan tanpa gulir horizontal", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await masukSebagai(page, "manajerProgram");
    await buka(page, "/data/harga");
    await tungguTenang(page);
    await pastikanTampil(page);
    await tanpaGulirHorizontal(page);
    await page.getByRole("button", { name: "Buka navigasi" }).click();
    await expect(page.getByRole("navigation", { name: "Navigasi utama" })).toBeVisible();
    await potret(page, "mobile-navigasi", false);
  });

  test("reduced-motion: konten langsung terlihat tanpa menunggu animasi", async ({ page }) => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    await masukSebagai(page, "manajerProgram");
    await buka(page, "/kpi");
    await expect(page.locator("h1").first()).toBeVisible();
    await pastikanTampil(page);
  });
});
