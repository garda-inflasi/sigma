/**
 * Pembantu e2e halaman /profil (dipakai profil.spec.ts, boleh dipakai spec lain):
 *  - sesi API Go dengan User-Agent tertentu dan cookie browser yang menyerupai produksi,
 *  - pengguna uji sementara yang sudah lepas dari kewajiban ganti kata sandi,
 *  - masuk lewat form /masuk dengan anggaran laju BFF, dan
 *  - pembaca meter kekuatan kata sandi serta pemantau galat HTTP yang memang diharapkan tes.
 *
 * Semua yang mengubah profil, kata sandi, MFA, atau sesi HARUS memakai pengguna uji sementara dari sini,
 * bukan akun seed: akun seed dipakai bersama oleh spec lain (nama "Super Admin", kata sandi demo).
 */
import { randomBytes } from "node:crypto";
import type { BrowserContext, Locator, Page } from "@playwright/test";
import { ROLES, type Role } from "../src/lib/rbac/roles";
import {
  anggaranBff,
  buatPenggunaUji,
  hapusPenggunaUji,
  sandiAcak,
  type PenggunaUji,
} from "./auth-helpers";
import {
  BASE_URL,
  expect,
  PETUNJUK_BATAS_LAJU,
  tungguTenang,
  type PantauGalat,
} from "./fixtures";

// ── Klien API Go ─────────────────────────────────────────────────────────

export const UA_MAC =
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36";
export const UA_ANDROID =
  "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36";
export const UA_IPHONE =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1";

export interface SesiUji {
  access: string;
  refresh: string;
  sid: string;
}

export interface JawabanApi<T = unknown> {
  status: number;
  /** `code` dan `message` amplop API Go (`{ code, message, data }`); kosong bila badan bukan JSON. */
  code: string;
  message: string;
  data: T;
}

/** Klaim (payload) sebuah JWT, tanpa memeriksa tanda tangan. */
export function klaimToken(access: string): Record<string, unknown> {
  return JSON.parse(
    Buffer.from(access.split(".")[1], "base64url").toString("utf8"),
  ) as Record<string, unknown>;
}

/** Satu panggilan ke API Go (`/api/v1/…`) lewat Caddy, tanpa BFF dan tanpa batas laju BFF. */
export async function goApi<T = unknown>(
  method: string,
  path: string,
  token: string | undefined,
  body?: unknown,
): Promise<JawabanApi<T>> {
  const res = await fetch(`${BASE_URL}/api/v1/${path}`, {
    method,
    headers: {
      "Content-Type": "application/json",
      Origin: BASE_URL,
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const json = (await res.json().catch(() => ({}))) as {
    code?: string;
    message?: string;
    data?: T;
  };
  return {
    status: res.status,
    code: json.code ?? "",
    message: json.message ?? "",
    data: json.data as T,
  };
}

/** Masuk ke API Go dengan User-Agent tertentu: satu sesi baru. Gagal bila akun meminta OTP. */
export async function masukApi(
  email: string,
  sandi: string,
  ua: string = UA_MAC,
): Promise<SesiUji> {
  const res = await fetch(`${BASE_URL}/api/v1/auth/login`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Origin: BASE_URL,
      "User-Agent": ua,
    },
    body: JSON.stringify({ email, password: sandi }),
  });
  const json = (await res.json().catch(() => ({}))) as {
    data?: { access_token?: string; refresh_token?: string };
  };
  if (!res.ok || !json.data?.access_token || !json.data.refresh_token)
    throw new Error(
      `Login ${email} gagal (${res.status})${res.status === 429 ? ` — ${PETUNJUK_BATAS_LAJU}` : ""}`,
    );
  const sid = String(klaimToken(json.data.access_token).sid ?? "");
  expect(sid, "access token membawa klaim sid").not.toBe("");
  return {
    access: json.data.access_token,
    refresh: json.data.refresh_token,
    sid,
  };
}

/** Status HTTP GET /v1/auth/me untuk sebuah access token: 200 bila sesinya hidup, 401 bila dicabut. */
export const statusMe = (access: string): Promise<number> =>
  goApi("GET", "auth/me", access).then((r) => r.status);

/** Status HTTP penukaran refresh token: 200 bila sesinya hidup, 401 bila dicabut. */
export async function statusRefresh(refresh: string): Promise<number> {
  return (
    await goApi("POST", "auth/refresh", undefined, { refresh_token: refresh })
  ).status;
}

/**
 * Menanam cookie sesi di konteks browser seperti yang dipasang BFF di produksi (refresh: path /api/auth,
 * httpOnly, SameSite=Strict). Cookie yang ada dihapus lebih dulu.
 */
export async function tanamSesi(
  konteks: BrowserContext,
  sesi: Pick<SesiUji, "access" | "refresh">,
): Promise<void> {
  const { hostname, protocol } = new URL(BASE_URL);
  const dasar = { domain: hostname, secure: protocol === "https:" };
  await konteks.clearCookies();
  await konteks.addCookies([
    {
      ...dasar,
      name: "garda_csrf",
      value: randomBytes(32).toString("hex"),
      path: "/",
      httpOnly: false,
      sameSite: "Lax",
    },
    {
      ...dasar,
      name: "garda_access",
      value: sesi.access,
      path: "/",
      httpOnly: true,
      sameSite: "Lax",
    },
    {
      ...dasar,
      name: "garda_refresh",
      value: sesi.refresh,
      path: "/api/auth",
      httpOnly: true,
      sameSite: "Strict",
    },
  ]);
}

/** Nilai satu cookie di konteks browser (cookie httpOnly ikut terbaca lewat API Playwright). */
export async function cookieBrowser(
  konteks: BrowserContext,
  nama: string,
): Promise<string | undefined> {
  return (await konteks.cookies()).find((c) => c.name === nama)?.value;
}

// ── Pengguna uji sementara ───────────────────────────────────────────────

/** Membuat pengguna uji (masih wajib ganti kata sandi) dan langsung mencatatnya untuk dibersihkan. */
export async function buatPengguna(
  daftar: PenggunaUji[],
): Promise<PenggunaUji> {
  const pengguna = await buatPenggunaUji();
  daftar.push(pengguna);
  return pengguna;
}

/**
 * Menuntaskan kewajiban ganti kata sandi akun baru (selama itu API menolak endpoint lain) dan menutup
 * sesinya, sehingga akun tanpa sesi tersisa dan hitungan sesi pasti. `pengguna.sandi` diperbarui.
 */
export async function lepasKewajibanGantiSandi(
  pengguna: PenggunaUji,
): Promise<void> {
  const awal = await masukApi(pengguna.email, pengguna.sandi);
  const baru = sandiAcak();
  const ganti = await goApi<{ access_token: string }>(
    "POST",
    "auth/change-password",
    awal.access,
    { old_password: pengguna.sandi, new_password: baru },
  );
  expect(ganti.status, "ganti kata sandi wajib").toBe(200);
  pengguna.sandi = baru;
  const keluar = await goApi("POST", "auth/logout", ganti.data.access_token);
  expect(keluar.status, "keluar dari sesi penyiapan").toBe(200);
}

/** Pengguna uji OPS_AGREGATOR yang siap dipakai: tanpa kewajiban ganti kata sandi dan tanpa sesi. */
export async function penggunaSiap(
  daftar: PenggunaUji[],
): Promise<PenggunaUji> {
  const pengguna = await buatPengguna(daftar);
  await lepasKewajibanGantiSandi(pengguna);
  return pengguna;
}

/** Nama awal pengguna uji: "Pengguna Uji <timestamp>.<n>" (lihat buatPenggunaUji). */
export function namaAwalUji(pengguna: PenggunaUji): string {
  return `Pengguna Uji ${pengguna.email.slice("e2e.".length, pengguna.email.indexOf("@"))}`;
}

/** Menghapus semua pengguna uji; gagal bila ada yang tersisa supaya sampah tidak menumpuk diam-diam. */
export async function bersihkanPengguna(daftar: PenggunaUji[]): Promise<void> {
  const gagal: string[] = [];
  for (const pengguna of daftar) {
    await hapusPenggunaUji(pengguna).catch((err: Error) =>
      gagal.push(`${pengguna.email}: ${err.message}`),
    );
  }
  expect(gagal, "pengguna uji yang gagal dihapus").toEqual([]);
}

// ── Galat yang diharapkan ────────────────────────────────────────────────

/** Mengizinkan satu respons HTTP gagal yang memang diharapkan, beserta baris console bawaan browser untuknya. */
export function izinkanHttp(
  galat: PantauGalat,
  status: number,
  metode: string,
  path: string,
): void {
  galat.izinkan(
    new RegExp(`^${status} ${metode} ${path.replace(/\//g, "\\/")}`),
  );
  galat.izinkan(new RegExp(`Failed to load resource.*status of ${status}`));
}

/** 401 sementara sebelum klien memulihkan sesi, beserta baris console bawaan browser. */
export function izinkan401(galat: PantauGalat): void {
  galat.izinkan(/^401 /);
  galat.izinkan(/Failed to load resource.*status of 401/);
}

// ── Halaman /profil ──────────────────────────────────────────────────────

/** Nama aksesibel keempat tab (kontrak UI 20) menurut urutannya. */
export const LABEL_TAB = {
  profil: "Edit Profil",
  gantiSandi: "Ganti Kata Sandi",
  mfa: "Verifikasi 2 Langkah",
  sesi: "Perangkat & Sesi Aktif",
} as const;

export const URUTAN_TAB: readonly string[] = [
  LABEL_TAB.profil,
  LABEL_TAB.gantiSandi,
  LABEL_TAB.mfa,
  LABEL_TAB.sesi,
];

export const tabProfil = (page: Page, nama: string): Locator =>
  page.getByRole("tab", { name: nama, exact: true });

/** Tab yang sedang terpilih (`aria-selected="true"`); tepat satu pada halaman yang sehat. */
export const tabTerpilih = (page: Page): Locator =>
  page.getByRole("tab", { selected: true });

export const panelProfil = (page: Page): Locator => page.getByRole("tabpanel");

export const tombolMenuAkun = (page: Page): Locator =>
  page.getByRole("button", { name: "Menu akun" });

/** Tombol kirim satu-satunya form di panel aktif; tidak bergantung pada tulisan tombolnya. */
export const tombolKirimPanel = (page: Page): Locator =>
  panelProfil(page).locator('button[type="submit"]');

/**
 * Kata kekuatan yang ditampilkan meter "Kekuatan kata sandi" (Lemah, Sedang, atau Kuat). Dibaca dari
 * `aria-valuetext`, isi meter, atau teks pembungkusnya, supaya tidak bergantung pada tata letaknya.
 */
export async function bacaKekuatanSandi(page: Page): Promise<string> {
  const meter = page.getByRole("meter", { name: "Kekuatan kata sandi" });
  if ((await meter.count()) === 0) return "(meter tidak ada)";
  const sumber = await meter
    .first()
    .evaluate((el) =>
      [
        el.getAttribute("aria-valuetext") ?? "",
        el.textContent ?? "",
        el.parentElement?.textContent ?? "",
      ].join(" "),
    );
  return (
    /\b(Lemah|Sedang|Kuat)\b/.exec(sumber)?.[1] ??
    `(tanpa kata kekuatan: "${sumber.trim().slice(0, 60)}")`
  );
}

/** Profil sendiri menurut BFF (GET /api/auth/me, memakai cookie konteks halaman ini). */
export interface ProfilBff {
  id: string;
  nama: string;
  email: string;
  peran: string;
  lapis: string;
  lembaga: string;
  status: string;
  kabupaten_kode?: string;
  telepon?: string;
  mfa_aktif: boolean;
  kode_cadangan_tersisa?: number;
  wajib_ganti_password: boolean;
}

export async function bacaProfilBff(page: Page): Promise<ProfilBff> {
  const res = await page.request.get(`${BASE_URL}/api/auth/me`);
  expect(res.status(), "GET /api/auth/me").toBe(200);
  return ((await res.json()) as { user: ProfilBff }).user;
}

// ── Masuk lewat form ─────────────────────────────────────────────────────

/** Mengisi form /masuk, lalu mengirimnya (satu pemakaian anggaran laju login BFF). */
export async function masukLewatForm(
  page: Page,
  email: string,
  sandi: string,
): Promise<void> {
  await page.goto("/masuk", { waitUntil: "domcontentloaded" });
  await tungguTenang(page); // tunggu hidrasi supaya isian tidak tertimpa
  await page.getByLabel("Email institusi").fill(email);
  await page.getByLabel("Kata sandi", { exact: true }).fill(sandi);
  await expect(page.getByLabel("Email institusi")).toHaveValue(email);
  await anggaranBff.pakai("login");
  await page.getByRole("button", { name: "Masuk", exact: true }).click();
}

/** Menunggu sampai halaman berada di beranda peran (mengikuti ROLES, bukan URL harfiah). */
export async function harapBeranda(page: Page, role: Role): Promise<void> {
  await expect(page).toHaveURL((url) => url.pathname === ROLES[role].beranda, {
    timeout: 20_000,
  });
  await tungguTenang(page);
}
