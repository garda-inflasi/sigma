import { defineConfig, devices } from "@playwright/test";

/**
 * Uji e2e terhadap stack Docker yang sedang berjalan (web + API Go + Postgres berisi seed demo).
 * Nyalakan dulu `make docker/demo/up`, lalu `make test/e2e` (atau `pnpm e2e` di apps/web).
 *
 * Tes berjalan berurutan (satu worker): beberapa tes mengubah data demo (kartu intervensi,
 * status master, kata sandi akun uji) dan tes lain membaca data yang sama.
 */
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  forbidOnly: !!process.env.CI,
  reporter: [
    ["list"],
    ["html", { outputFolder: "playwright-report", open: "never" }],
  ],
  outputDir: "test-results",
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:8090",
    locale: "id-ID",
    timezoneId: "Asia/Jakarta",
    viewport: { width: 1440, height: 900 },
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: {
        ...devices["Desktop Chrome"],
        viewport: { width: 1440, height: 900 },
      },
    },
  ],
});
