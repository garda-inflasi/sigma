/**
 * Fixture bersama uji e2e.
 *
 * Login lewat API Go langsung lalu menanam cookie sesi, bukan lewat form: form BFF dibatasi
 * 5 percobaan per menit per IP sehingga suite berisi banyak peran akan tertahan. Alur form
 * sendiri diuji di auth.spec.ts.
 *
 * Fixture `galat` aktif otomatis di setiap tes: respons API >= 400, exception JS, dan
 * console.error menggagalkan tes kecuali diizinkan lewat `galat.izinkan(pola)`.
 */
import { randomBytes } from "node:crypto";
import {
  test as base,
  expect,
  request,
  type APIRequestContext,
  type Page,
} from "@playwright/test";

export { expect };

/** Kata sandi seluruh akun seed demo (apps/api/internal/auth/seeder/demo.go). */
export const SANDI_DEMO = process.env.E2E_PASSWORD ?? "Garda2024!";

export const BASE_URL = process.env.E2E_BASE_URL ?? "http://localhost:8090";

/** Akun seed demo per peran. */
export const AKUN = {
  superadmin: "superadmin@gardainflasi.id",
  strategisFmipa: "strategis.fmipa@gardainflasi.id",
  strategisBps: "strategis.bps@gardainflasi.id",
  strategisBi: "strategis.bi@gardainflasi.id",
  strategisDjpb: "strategis.djpb@gardainflasi.id",
  manajerProgram: "manajer.program@gardainflasi.id",
  manajerData: "manajer.data@gardainflasi.id",
  manajerKebijakan: "manajer.kebijakan@gardainflasi.id",
  manajerPembiayaan: "manajer.pembiayaan@gardainflasi.id",
  manajerRisiko: "manajer.risiko@gardainflasi.id",
  manajerMutu: "manajer.mutu@gardainflasi.id",
  koordinator: "koordinator.lapangan@gardainflasi.id",
  agregator: "agregator.data@gardainflasi.id",
  mahasiswa: "mahasiswa.1@gardainflasi.id",
  mfa: "mfa@gardainflasi.id",
} as const;

export type Akun = keyof typeof AKUN;

type Sesi = { accessToken: string; refreshToken: string };

const emailDari = (akun: Akun | string) =>
  akun in AKUN ? AKUN[akun as Akun] : akun;

/** Masuk ke API Go dan mengembalikan token sesi. Gagal bila akun meminta OTP. */
export async function sesiApi(
  akun: Akun | string,
  sandi = SANDI_DEMO,
): Promise<Sesi> {
  const res = await fetch(`${BASE_URL}/api/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Origin: BASE_URL },
    body: JSON.stringify({ email: emailDari(akun), password: sandi }),
  });
  const body = (await res.json().catch(() => ({}))) as {
    data?: { access_token?: string; refresh_token?: string };
  };
  if (!res.ok || !body.data?.access_token) {
    throw new Error(
      `Login ${emailDari(akun)} gagal (${res.status}): ${JSON.stringify(body).slice(0, 200)}`,
    );
  }
  return {
    accessToken: body.data.access_token,
    refreshToken: body.data.refresh_token ?? "",
  };
}

/** Menanam cookie sesi (akses, refresh, CSRF) di konteks browser halaman ini. */
export async function masukSebagai(
  page: Page,
  akun: Akun | string,
): Promise<void> {
  const { accessToken, refreshToken } = await sesiApi(akun);
  const cookies = [
    { name: "garda_access", value: accessToken, url: BASE_URL },
    {
      name: "garda_csrf",
      value: randomBytes(32).toString("hex"),
      url: BASE_URL,
    },
  ];
  if (refreshToken)
    cookies.push({ name: "garda_refresh", value: refreshToken, url: BASE_URL });
  await page.context().clearCookies();
  await page.context().addCookies(cookies);
}

/** Klien API Go (`/api/v1/…`) sebagai akun tertentu, untuk menyiapkan atau membersihkan data uji. */
export async function apiSebagai(
  akun: Akun | string,
): Promise<APIRequestContext> {
  const { accessToken } = await sesiApi(akun);
  return request.newContext({
    baseURL: `${BASE_URL}/api/v1/`,
    extraHTTPHeaders: {
      Authorization: `Bearer ${accessToken}`,
      Origin: BASE_URL,
    },
  });
}

/** Membuka halaman dan memastikan tidak dialihkan ke /masuk atau /akses-ditolak. */
export async function buka(page: Page, path: string): Promise<void> {
  await page.goto(path, { waitUntil: "domcontentloaded" });
  await expect(page, `${path} dialihkan`).not.toHaveURL(
    /\/(masuk|akses-ditolak)(\?|$)/,
  );
}

/**
 * Menunggu sampai tidak ada request yang berjalan selama 500 ms. `networkidle` bawaan tidak
 * bisa dipakai karena koneksi SSE /ews/stream selalu terbuka.
 */
export async function tungguTenang(
  page: Page,
  batasMs = 15_000,
): Promise<void> {
  const berjalan = new Set<unknown>();
  const abaikan = (url: string) => url.includes("/ews/stream");
  const mulai = (req: { url(): string }) =>
    !abaikan(req.url()) && berjalan.add(req);
  const selesai = (req: unknown) => berjalan.delete(req);
  page.on("request", mulai);
  page.on("requestfinished", selesai);
  page.on("requestfailed", selesai);
  try {
    const akhir = Date.now() + batasMs;
    let tenangSejak = Date.now();
    while (Date.now() < akhir) {
      await page.waitForTimeout(100);
      if (berjalan.size > 0) tenangSejak = Date.now();
      else if (Date.now() - tenangSejak >= 500) return;
    }
  } finally {
    page.off("request", mulai);
    page.off("requestfinished", selesai);
    page.off("requestfailed", selesai);
  }
}

export class PantauGalat {
  private readonly galat: string[] = [];
  private readonly izin: RegExp[] = [];

  constructor(page: Page) {
    page.on("response", (res) => {
      const url = res.url().replace(BASE_URL, "");
      if (url.startsWith("/api/") && res.status() >= 400)
        this.catat(`${res.status()} ${res.request().method()} ${url}`);
    });
    page.on("pageerror", (err) => this.catat(`pageerror: ${err.message}`));
    page.on("console", (msg) => {
      if (msg.type() === "error") this.catat(`console: ${msg.text()}`);
    });
  }

  /** Mengizinkan galat yang memang diharapkan tes (mis. 403 untuk peran tanpa izin). */
  izinkan(pola: RegExp): void {
    this.izin.push(pola);
  }

  daftar(): string[] {
    return this.galat.filter((g) => !this.izin.some((p) => p.test(g)));
  }

  private catat(pesan: string): void {
    this.galat.push(pesan.slice(0, 300));
  }
}

export const test = base.extend<{ galat: PantauGalat }>({
  galat: [
    async ({ page }, use) => {
      const pantau = new PantauGalat(page);
      await use(pantau);
      expect(pantau.daftar(), "galat API/JS selama tes").toEqual([]);
    },
    { auto: true },
  ],
});
