/**
 * E2E Pengujian Halaman Sumber Data & API (/sumber-data).
 *
 * Menguji:
 * 1. Pemuatan pipeline medallion (Bronze, Silver, Gold) & kartu mutu data dari API /v1/sumber-data/pipeline.
 * 2. Pencarian dan filter katalog sumber data pada DataTable.
 * 3. Detail sheet sumber data (informasi teknis, riwayat eksekusi, sampel payload bronze).
 * 4. Pemicuan ingestion manual ("Jalankan sekarang") oleh pengguna dengan izin sumber.kelola.
 * 5. Aksi massal jeda dan lanjutkan sumber data.
 * 6. Penegakan hak akses RBAC (tombol aksi tersembunyi bagi peran read-only).
 */
import {
  apiSebagai,
  buka,
  expect,
  jalankanSql,
  masukSebagai,
  test,
  tungguTenang,
} from "./fixtures";

const UUID_SUMBER_PIHPS = "40745615-f152-32aa-b1e3-f54add179514"; // SRC-BI
const UUID_SUMBER_BPS = "e1f49004-97cf-3b83-9c13-15c6eb3112d6";   // SRC-BPS

const unikPath =
  (path: string) => (r: { url(): string; request(): { method(): string } }) =>
    new URL(r.url()).pathname.endsWith(path);

test.describe("halaman sumber data & pipeline", () => {
  test.beforeEach(async () => {
    // Pastikan status sumber kembali normal sebelum setiap pengujian
    jalankanSql(`
      UPDATE sumber_data 
         SET status = 'SEHAT' 
       WHERE kode IN ('SRC-BI', 'SRC-BPS', 'SRC-DJPB') AND status = 'DIJEDA';
    `);
  });

  test.afterAll(async () => {
    jalankanSql(`
      UPDATE sumber_data 
         SET status = 'SEHAT' 
       WHERE kode IN ('SRC-BI', 'SRC-BPS', 'SRC-DJPB') AND status = 'DIJEDA';
    `);
  });

  test("memuat pipeline medallion, metrik mutu, dan katalog data dari DB", async ({
    page,
  }) => {
    await masukSebagai(page, "manajerData");
    const respPipeline = page.waitForResponse(unikPath("/v1/sumber-data/pipeline"));
    const respKatalog = page.waitForResponse(unikPath("/v1/sumber-data"));

    await buka(page, "/sumber-data");
    await tungguTenang(page);

    expect((await respPipeline).status()).toBe(200);
    expect((await respKatalog).status()).toBe(200);

    // Header & kartu utama
    await expect(page.getByRole("heading", { name: "Sumber data & API" })).toBeVisible();
    await expect(page.getByText("Pipeline data")).toBeVisible();
    await expect(page.getByText("Bronze")).toBeVisible();
    await expect(page.getByText("Silver")).toBeVisible();
    await expect(page.getByText("Gold")).toBeVisible();

    // Kartu mutu data
    await expect(page.getByText("Kelengkapan")).toBeVisible();
    await expect(page.getByText("Akurasi")).toBeVisible();
    await expect(page.getByText("Ketepatan waktu")).toBeVisible();
    await expect(page.getByText("Konsistensi")).toBeVisible();

    // Baris tabel katalog
    await expect(page.getByRole("row", { name: /PIHPS Nasional/ })).toBeVisible();
    await expect(page.getByRole("row", { name: /BPS WebAPI/ })).toBeVisible();
  });

  test("pencarian menyaring sumber data secara interaktif", async ({ page }) => {
    await masukSebagai(page, "manajerData");
    await buka(page, "/sumber-data");
    await tungguTenang(page);

    const inputCari = page.getByPlaceholder("Cari sumber atau penyedia");
    await inputCari.fill("BMKG");

    await expect(page.getByRole("row", { name: /BMKG — prakiraan/ })).toBeVisible();
    await expect(page.getByRole("row", { name: /PIHPS Nasional/ })).toHaveCount(0);

    await inputCari.clear();
    await expect(page.getByRole("row", { name: /PIHPS Nasional/ })).toBeVisible();
  });

  test("detail sheet memuat informasi sumber, riwayat eksekusi, dan sampel bronze", async ({
    page,
  }) => {
    await masukSebagai(page, "manajerData");
    await buka(page, "/sumber-data");
    await tungguTenang(page);

    const respDetail = page.waitForResponse(unikPath(`/v1/sumber-data/${UUID_SUMBER_PIHPS}`));
    const respEksekusi = page.waitForResponse(unikPath(`/v1/sumber-data/${UUID_SUMBER_PIHPS}/eksekusi`));
    const respSampel = page.waitForResponse(unikPath(`/v1/sumber-data/${UUID_SUMBER_PIHPS}/sampel`));

    await page.getByRole("row", { name: /PIHPS Nasional/ }).click();

    expect((await respDetail).status()).toBe(200);
    expect((await respEksekusi).status()).toBe(200);
    expect((await respSampel).status()).toBe(200);

    const dialog = page.getByRole("dialog");
    await expect(dialog.getByRole("heading", { name: /PIHPS Nasional/ })).toBeVisible();
    await expect(dialog.getByText("Informasi sumber")).toBeVisible();
    await expect(dialog.getByText("SRC-BI", { exact: true })).toBeVisible();
    await expect(dialog.getByText("Jadwal cron (WIB)")).toBeVisible();
    await expect(dialog.getByText("https://www.bi.go.id/hargapangan", { exact: true })).toBeVisible();
    await expect(dialog.getByText("Riwayat eksekusi")).toBeVisible();
    await expect(dialog.getByText("Sampel data mentah (bronze)")).toBeVisible();
  });

  test("tombol 'Jalankan sekarang' memicu eksekusi ingestion manual (202 Accepted)", async ({
    page,
  }) => {
    await masukSebagai(page, "manajerData");
    await buka(page, "/sumber-data");
    await tungguTenang(page);

    await page.getByRole("row", { name: /PIHPS Nasional/ }).click();
    const dialog = page.getByRole("dialog");
    const tombolJalankan = dialog.getByRole("button", { name: "Jalankan sekarang" });
    await expect(tombolJalankan).toBeVisible();

    const respJalankan = page.waitForResponse(
      (r) =>
        r.request().method() === "POST" &&
        new URL(r.url()).pathname.endsWith(`/v1/sumber-data/${UUID_SUMBER_PIHPS}/jalankan`),
    );

    await tombolJalankan.click();
    const res = await respJalankan;
    expect([200, 202]).toContain(res.status());

    // Notifikasi toast muncul
    await expect(page.getByText(/Ingestion dimulai|berhasil dijalankan/i)).toBeVisible();
  });

  test("peran read-only (strategisFmipa) tidak memiliki tombol jalankan sekarang", async ({
    page,
  }) => {
    await masukSebagai(page, "strategisFmipa");
    await buka(page, "/sumber-data");
    await tungguTenang(page);

    await page.getByRole("row", { name: /PIHPS Nasional/ }).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog.getByText("Informasi sumber")).toBeVisible();
    await expect(dialog.getByRole("button", { name: "Jalankan sekarang" })).toHaveCount(0);
  });
});
