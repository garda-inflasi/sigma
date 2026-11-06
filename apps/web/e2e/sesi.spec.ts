/**
 * Sesi: daftar dan pencabutan sesi aktif, rotasi refresh token, penegakan pencabutan di API Go, dan
 * pemulihan senyap di web.
 *
 * Aturan suite ini:
 *  - Semua yang mengubah keadaan sesi memakai pengguna sementara `e2e.<timestamp>.<n>@gardainflasi.id`
 *    (OPS_AGREGATOR, bukan wajib MFA) yang dibuat lewat API dan dihapus di afterAll. Akun seed hanya
 *    dipakai sebagai admin (`superadmin`, sesi baru milik tes) dan tidak pernah dikeluarkan dari sesi lain.
 *  - Sesi "perangkat lain" dibuat langsung ke API Go dengan User-Agent berbeda, supaya daftar sesi
 *    membedakannya ("Chrome di macOS" untuk sesi browser, "Chrome di Android" untuk sesi lain). Cookie
 *    browser ditanam menyerupai produksi (refresh: path /api/auth, httpOnly, SameSite=Strict).
 *  - Login lewat API Go tidak berbatas laju BFF; tidak ada form login yang disubmit di sini. API Go sendiri
 *    membatasi percobaan auth per IP (AUTH_RATE_LIMIT_PER_MENIT, bawaan 10/menit) dan suite ini membuat banyak
 *    sesi: stack harus dinyalakan lewat `make docker/demo/up`, yang menyetelnya tinggi.
 *  - Tes yang memakai jendela toleransi refresh (10 detik) menunggu sungguhan, tanpa mengubah waktu.
 *  - Tes yang sengaja membuat access token hilang mengizinkan 401 sementara sebelum klien memulihkan sesi.
 */
import { randomBytes } from "node:crypto";
import type { Page } from "@playwright/test";
import {
  buatPenggunaUji,
  hapusPenggunaUji,
  sandiAcak,
  type PenggunaUji,
} from "./auth-helpers";
import {
  BASE_URL,
  buka,
  expect,
  masukSebagai,
  PETUNJUK_BATAS_LAJU,
  test,
  tungguTenang,
} from "./fixtures";

test.describe.configure({ mode: "serial", timeout: 120_000 });

const UA_MAC =
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36";
const UA_ANDROID =
  "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36";
const UA_IPHONE =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1";

/** Jendela toleransi refresh di API (usecase.DefaultRefreshGraceWindow) ditambah cadangan. */
const LEWAT_TOLERANSI_MS = 11_000;

// ── Klien API Go ─────────────────────────────────────────────────────────

interface SesiApi {
  access: string;
  refresh: string;
  sid: string;
}

interface JawabanApi {
  status: number;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  body: any;
}

function klaim(access: string): Record<string, unknown> {
  return JSON.parse(
    Buffer.from(access.split(".")[1], "base64url").toString("utf8"),
  ) as Record<string, unknown>;
}

async function goApi(
  method: string,
  path: string,
  token: string | undefined,
  body?: unknown,
): Promise<JawabanApi> {
  const res = await fetch(`${BASE_URL}/api/v1/${path}`, {
    method,
    headers: {
      "Content-Type": "application/json",
      Origin: BASE_URL,
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  return { status: res.status, body: await res.json().catch(() => ({})) };
}

/** Masuk ke API Go dengan User-Agent tertentu → satu sesi baru. */
async function masukApi(
  email: string,
  sandi: string,
  ua: string,
): Promise<SesiApi> {
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
  const sid = String(klaim(json.data.access_token).sid ?? "");
  expect(sid, "access token membawa klaim sid").not.toBe("");
  return {
    access: json.data.access_token,
    refresh: json.data.refresh_token,
    sid,
  };
}

const me = (access: string) =>
  goApi("GET", "auth/me", access).then((r) => r.status);

interface HasilRefresh {
  status: number;
  access?: string;
  refresh?: string;
}

async function refreshApi(refresh: string): Promise<HasilRefresh> {
  const r = await goApi("POST", "auth/refresh", undefined, {
    refresh_token: refresh,
  });
  return {
    status: r.status,
    access: r.body?.data?.access_token,
    refresh: r.body?.data?.refresh_token || undefined,
  };
}

const tidur = (ms: number) => new Promise((r) => setTimeout(r, ms));

// ── Pengguna uji ─────────────────────────────────────────────────────────

const dibuat: PenggunaUji[] = [];

test.afterAll(async () => {
  const gagal: string[] = [];
  for (const pengguna of dibuat) {
    await hapusPenggunaUji(pengguna).catch((err: Error) =>
      gagal.push(err.message),
    );
  }
  expect(gagal, "pengguna uji yang gagal dihapus").toEqual([]);
});

/**
 * Pengguna uji yang sudah menyelesaikan kewajiban ganti kata sandi (akun buatan admin diwajibkan, dan
 * selama itu API menolak endpoint lain) dan tidak punya sesi tersisa, sehingga hitungan sesi pasti.
 */
async function penggunaSiap(): Promise<PenggunaUji> {
  const pengguna = await buatPenggunaUji();
  dibuat.push(pengguna);
  const awal = await masukApi(pengguna.email, pengguna.sandi, UA_MAC);
  const baru = sandiAcak();
  const ganti = await goApi("POST", "auth/change-password", awal.access, {
    old_password: pengguna.sandi,
    new_password: baru,
  });
  expect(ganti.status, "ganti kata sandi wajib").toBe(200);
  pengguna.sandi = baru;
  const keluar = await goApi(
    "POST",
    "auth/logout",
    ganti.body.data.access_token,
  );
  expect(keluar.status).toBe(200);
  return pengguna;
}

/** Menanam cookie sesi di browser seperti yang dipasang BFF di produksi. */
async function tanamCookie(
  page: Page,
  sesi: { access?: string; refresh?: string },
  { csrf = true }: { csrf?: boolean } = {},
): Promise<void> {
  const { hostname, protocol } = new URL(BASE_URL);
  const dasar = { domain: hostname, secure: protocol === "https:" };
  const cookies = [];
  if (csrf)
    cookies.push({
      ...dasar,
      name: "garda_csrf",
      value: randomBytes(32).toString("hex"),
      path: "/",
      httpOnly: false,
      sameSite: "Lax" as const,
    });
  if (sesi.access)
    cookies.push({
      ...dasar,
      name: "garda_access",
      value: sesi.access,
      path: "/",
      httpOnly: true,
      sameSite: "Lax" as const,
    });
  if (sesi.refresh)
    cookies.push({
      ...dasar,
      name: "garda_refresh",
      value: sesi.refresh,
      path: "/api/auth",
      httpOnly: true,
      sameSite: "Strict" as const,
    });
  await page.context().clearCookies();
  await page.context().addCookies(cookies);
}

const cookieBrowser = async (page: Page, nama: string) =>
  (await page.context().cookies()).find((c) => c.name === nama);

/** Respons 401 sementara yang memang diharapkan sebelum klien memulihkan sesi, beserta baris console bawaan browser. */
function izinkan401(galat: { izinkan(p: RegExp): void }): void {
  galat.izinkan(/^401 /);
  galat.izinkan(/Failed to load resource.*status of 401/);
}

// ── Halaman Keamanan akun ────────────────────────────────────────────────

test("Keamanan akun mendaftar sesi aktif dan menandai sesi ini; Cabut mematikan sesi lain seketika", async ({
  page,
}) => {
  const u = await penggunaSiap();
  const ini = await masukApi(u.email, u.sandi, UA_MAC);
  const lain = await masukApi(u.email, u.sandi, UA_ANDROID);
  expect(await me(lain.access)).toBe(200);

  await tanamCookie(page, ini);
  await buka(page, "/keamanan");
  await expect(
    page.getByRole("heading", { name: "Keamanan akun" }),
  ).toBeVisible();

  const butir = page
    .getByRole("list", { name: "Sesi aktif" })
    .getByRole("listitem");
  await expect(butir).toHaveCount(2);

  const sesiIni = butir.filter({ hasText: "Perangkat ini" });
  await expect(sesiIni).toHaveCount(1);
  await expect(sesiIni).toContainText("Chrome di macOS");
  await expect(sesiIni).toContainText("IP ");
  await expect(
    sesiIni.getByRole("button", { name: /Cabut/ }),
    "sesi yang sedang dipakai tidak bisa dicabut dari daftar",
  ).toHaveCount(0);

  const sesiLain = butir.filter({ hasText: "Chrome di Android" });
  await expect(sesiLain).toHaveCount(1);
  await sesiLain
    .getByRole("button", { name: /^Cabut sesi Chrome di Android/ })
    .click();
  await expect(page.getByText("Sesi dicabut", { exact: true })).toBeVisible();
  await expect(butir).toHaveCount(1);

  expect(
    await me(lain.access),
    "access token sesi yang dicabut langsung 401",
  ).toBe(401);
  expect(
    (await refreshApi(lain.refresh)).status,
    "refresh token-nya gagal",
  ).toBe(401);
  expect(await me(ini.access), "sesi browser tetap hidup").toBe(200);
});

test("Keluar dari perangkat lain mencabut semua sesi lain dan menyisakan sesi ini", async ({
  page,
}) => {
  const u = await penggunaSiap();
  const ini = await masukApi(u.email, u.sandi, UA_MAC);
  const a = await masukApi(u.email, u.sandi, UA_ANDROID);
  const b = await masukApi(u.email, u.sandi, UA_IPHONE);

  await tanamCookie(page, ini);
  await buka(page, "/keamanan");
  const butir = page
    .getByRole("list", { name: "Sesi aktif" })
    .getByRole("listitem");
  await expect(butir).toHaveCount(3);

  await page
    .getByRole("button", { name: "Keluar dari perangkat lain" })
    .click();
  await expect(
    page.getByText("Perangkat lain dikeluarkan", { exact: true }),
  ).toBeVisible();
  await expect(butir).toHaveCount(1);
  await expect(butir.first()).toContainText("Perangkat ini");
  await expect(
    page.getByRole("button", { name: "Keluar dari perangkat lain" }),
    "tidak ada perangkat lain lagi",
  ).toBeDisabled();

  for (const s of [a, b]) {
    expect(await me(s.access)).toBe(401);
    expect((await refreshApi(s.refresh)).status).toBe(401);
  }
  expect(await me(ini.access)).toBe(200);
});

test("Keluar dari semua perangkat mengakhiri semua sesi, termasuk yang ini", async ({
  page,
}) => {
  const u = await penggunaSiap();
  const ini = await masukApi(u.email, u.sandi, UA_MAC);
  const lain = await masukApi(u.email, u.sandi, UA_ANDROID);

  await tanamCookie(page, ini);
  await buka(page, "/keamanan");
  await page
    .getByRole("button", { name: "Keluar dari semua perangkat" })
    .click();
  await page
    .getByRole("alertdialog")
    .getByRole("button", { name: "Ya, keluar semua" })
    .click();

  await expect(page).toHaveURL(/\/masuk$/);
  expect(await cookieBrowser(page, "garda_access")).toBeUndefined();
  expect(await cookieBrowser(page, "garda_refresh")).toBeUndefined();
  for (const s of [ini, lain]) {
    expect(await me(s.access)).toBe(401);
    expect((await refreshApi(s.refresh)).status).toBe(401);
  }
});

// ── Keluar ───────────────────────────────────────────────────────────────

test("Keluar hanya mengakhiri sesi ini: access token-nya ditolak, perangkat lain tetap masuk", async ({
  page,
}) => {
  const u = await penggunaSiap();
  const ini = await masukApi(u.email, u.sandi, UA_MAC);
  const lain = await masukApi(u.email, u.sandi, UA_ANDROID);

  await tanamCookie(page, ini);
  await buka(page, "/cold-storage");
  await tungguTenang(page);
  expect(await me(ini.access)).toBe(200);

  await page.getByRole("button", { name: "Menu akun" }).click();
  await page.getByRole("menuitem", { name: "Keluar" }).click();
  await expect(page).toHaveURL(/\/masuk$/);
  expect(await cookieBrowser(page, "garda_access")).toBeUndefined();

  expect(
    await me(ini.access),
    "access token yang sudah keluar tidak berlaku lagi",
  ).toBe(401);
  expect((await refreshApi(ini.refresh)).status).toBe(401);
  expect(await me(lain.access), "perangkat lain tetap masuk").toBe(200);
  expect((await refreshApi(lain.refresh)).status).toBe(200);
});

test("Keluar saat access token sudah habis tetap mencabut sesi di server", async ({
  page,
  galat,
}) => {
  izinkan401(galat);
  const u = await penggunaSiap();
  const ini = await masukApi(u.email, u.sandi, UA_MAC);
  const lain = await masukApi(u.email, u.sandi, UA_ANDROID);

  await tanamCookie(page, ini);
  await buka(page, "/cold-storage");
  await tungguTenang(page);
  await page.context().clearCookies({ name: "garda_access" });

  await page.getByRole("button", { name: "Menu akun" }).click();
  await page.getByRole("menuitem", { name: "Keluar" }).click();
  await expect(page).toHaveURL(/\/masuk$/);
  expect(await cookieBrowser(page, "garda_refresh")).toBeUndefined();

  expect(
    (await refreshApi(ini.refresh)).status,
    "sesi benar-benar berakhir di server",
  ).toBe(401);
  expect(await me(lain.access)).toBe(200);
});

// ── Pemulihan sesi senyap di web ─────────────────────────────────────────

test("browser yang hanya memegang garda_refresh membuka halaman terlindungi dan mendarat di sana, bukan /masuk", async ({
  page,
}) => {
  const u = await penggunaSiap();
  const ini = await masukApi(u.email, u.sandi, UA_MAC);
  await tanamCookie(page, { refresh: ini.refresh }, { csrf: false });

  await page.goto("/cold-storage", { waitUntil: "domcontentloaded" });
  await expect(page).toHaveURL((url) => url.pathname === "/cold-storage");
  await expect(page.getByRole("button", { name: "Menu akun" })).toBeVisible();
  await tungguTenang(page);

  expect(
    await cookieBrowser(page, "garda_access"),
    "access token baru terpasang",
  ).toBeDefined();
  expect(
    await cookieBrowser(page, "garda_csrf"),
    "token CSRF dibuat ulang",
  ).toBeDefined();
  const refresh = await cookieBrowser(page, "garda_refresh");
  expect(refresh?.path).toBe("/api/auth");
  expect(refresh?.value, "refresh token diputar").not.toBe(ini.refresh);
});

test("pemulihan saat navigasi membawa path dan query semula, dan menolak tujuan di luar situs", async ({
  page,
}) => {
  const u = await penggunaSiap();
  const ini = await masukApi(u.email, u.sandi, UA_MAC);
  await tanamCookie(page, { refresh: ini.refresh }, { csrf: false });

  await page.goto("/cold-storage?uji=1", { waitUntil: "domcontentloaded" });
  await expect(page).toHaveURL((url) => url.pathname === "/cold-storage");
  expect(new URL(page.url()).searchParams.get("uji")).toBe("1");

  // lanjut berisi URL eksternal: tetap di situs ini (beranda peran), tidak pernah ke luar.
  const sesi2 = await masukApi(u.email, u.sandi, UA_MAC);
  await tanamCookie(page, { refresh: sesi2.refresh }, { csrf: false });
  await page.goto("/api/auth/refresh?lanjut=//evil.example/x", {
    waitUntil: "domcontentloaded",
  });
  expect(new URL(page.url()).origin).toBe(new URL(BASE_URL).origin);
  expect(page.url()).not.toContain("evil.example");
});

test("refresh token yang sudah dicabut tidak memulihkan sesi: /masuk dengan tujuan semula dan cookie dibersihkan", async ({
  page,
}) => {
  const u = await penggunaSiap();
  const ini = await masukApi(u.email, u.sandi, UA_MAC);
  expect((await goApi("POST", "auth/logout", ini.access)).status).toBe(200);
  await tanamCookie(page, { refresh: ini.refresh }, { csrf: false });

  await page.goto("/cold-storage");

  await expect(page).toHaveURL(/\/masuk\?lanjut=%2Fcold-storage/);
  expect(await cookieBrowser(page, "garda_refresh")).toBeUndefined();
  expect(await cookieBrowser(page, "garda_access")).toBeUndefined();
});

test("garda_access hilang di tengah sesi: aksi data berikutnya pulih senyap tanpa keluar", async ({
  page,
  galat,
}) => {
  izinkan401(galat);
  const u = await penggunaSiap();
  const ini = await masukApi(u.email, u.sandi, UA_MAC);
  const lain = await masukApi(u.email, u.sandi, UA_ANDROID);

  await tanamCookie(page, ini);
  await buka(page, "/keamanan");
  const butir = page
    .getByRole("list", { name: "Sesi aktif" })
    .getByRole("listitem");
  await expect(butir).toHaveCount(2);

  const awalRefresh = (await cookieBrowser(page, "garda_refresh"))?.value;
  // Ditunggu sejak sebelum cookie dihapus. Sesi bisa pulih lewat dua jalur yang sama sahnya: klik memanggil
  // BFF → 401 → POST /api/auth/refresh, atau prefetch tautan halaman (mis. "Kembali ke beranda") lebih dulu
  // melewati edge → GET /api/auth/refresh?lanjut=… yang memutar token lalu mengalihkan. Yang diuji adalah
  // pemulihan senyapnya, bukan jalur mana yang menang.
  const refreshBerhasil = page.waitForResponse((r) => {
    const { pathname } = new URL(r.url());
    if (pathname !== "/api/auth/refresh") return false;
    return r.request().method() === "POST" ? r.status() === 200 : r.status() >= 300 && r.status() < 400;
  });
  await page.context().clearCookies({ name: "garda_access" });
  expect(await cookieBrowser(page, "garda_access")).toBeUndefined();

  // Aksi mutasi lewat BFF: 401 → POST /api/auth/refresh → diulang dengan cookie baru.
  await page
    .getByRole("button", { name: "Keluar dari perangkat lain" })
    .click();
  await refreshBerhasil;

  await expect(
    page.getByText("Perangkat lain dikeluarkan", { exact: true }),
  ).toBeVisible();
  await expect(page).toHaveURL(/\/keamanan$/);
  expect(
    await cookieBrowser(page, "garda_access"),
    "access token baru terpasang",
  ).toBeDefined();
  expect((await cookieBrowser(page, "garda_refresh"))?.value).not.toBe(
    awalRefresh,
  );
  expect(
    await me(lain.access),
    "aksi yang diulang benar-benar dijalankan",
  ).toBe(401);
});

test("401 dari API Go langsung (apiFetch) dipulihkan sekali walau banyak permintaan gagal bersamaan", async ({
  page,
  galat,
}) => {
  izinkan401(galat);
  const u = await penggunaSiap();
  const ini = await masukApi(u.email, u.sandi, UA_MAC);
  await tanamCookie(page, ini);

  // Permintaan data pertama ke API Go "kehilangan" access token: cookie dihapus lalu dijawab 401, seperti
  // token yang kedaluwarsa tepat saat halaman memuat banyak data sekaligus.
  let dicegat = false;
  // Aliran SSE sengaja tidak dicegat: hanya permintaan JSON biasa yang dibelokkan.
  await page.route(
    (url) =>
      url.pathname.startsWith("/api/v1/") &&
      !url.pathname.includes("/ews/stream"),
    async (route) => {
      if (!dicegat && route.request().method() === "GET") {
        dicegat = true;
        await page.context().clearCookies({ name: "garda_access" });
        await route.fulfill({
          status: 401,
          contentType: "application/json",
          body: JSON.stringify({ code: "401", message: "Unauthorized" }),
        });
        return;
      }
      await route.continue();
    },
  );
  let jumlahRefresh = 0;
  page.on("request", (r) => {
    if (r.url().endsWith("/api/auth/refresh") && r.method() === "POST")
      jumlahRefresh += 1;
  });

  await page.goto("/operasional", { waitUntil: "domcontentloaded" });
  await tungguTenang(page);

  expect(dicegat, "ada permintaan ke API Go yang digagalkan").toBe(true);
  expect(
    jumlahRefresh,
    "satu refresh bersama untuk semua permintaan yang gagal",
  ).toBe(1);
  await expect(page).toHaveURL((url) => url.pathname === "/operasional");
  await expect(page.getByRole("button", { name: "Menu akun" })).toBeVisible();
  expect(await cookieBrowser(page, "garda_access")).toBeDefined();
  expect(await me(ini.access)).toBe(200);
});

test("dua tab menyegarkan sesi bersamaan dengan cookie yang sama: keduanya berhasil dan tidak saling mengeluarkan", async ({
  page,
}) => {
  const u = await penggunaSiap();
  const ini = await masukApi(u.email, u.sandi, UA_MAC);
  await tanamCookie(page, ini);
  const tab2 = await page.context().newPage();
  try {
    await buka(page, "/keamanan");
    await tab2.goto("/keamanan", { waitUntil: "domcontentloaded" });
    await page.context().clearCookies({ name: "garda_access" });

    const segarkan = (p: Page) =>
      p.evaluate(async () => {
        const res = await fetch("/api/auth/refresh", { method: "POST" });
        return {
          status: res.status,
          body: (await res.json()) as { diputar?: boolean },
        };
      });
    // Dua evaluate di dua tab dikirim Playwright berurutan, jadi tab kedua bisa saja sudah membawa cookie
    // baru. Agar kedua permintaan pasti membawa refresh token yang SAMA (seperti dua tab yang menyegarkan
    // pada detik yang sama), keduanya dilepas pada tick yang sama dari satu halaman; jar cookie dipakai
    // bersama oleh semua tab di konteks ini.
    const [a, b] = await page.evaluate(async () => {
      const satu = async () => {
        const res = await fetch("/api/auth/refresh", { method: "POST" });
        return {
          status: res.status,
          body: (await res.json()) as { diputar?: boolean },
        };
      };
      return Promise.all([satu(), satu()]);
    });

    expect([a.status, b.status]).toEqual([200, 200]);
    expect(
      [a.body.diputar, b.body.diputar].filter(Boolean),
      "tepat satu tab memutar refresh token; tab lain hanya menerima access token",
    ).toHaveLength(1);

    // Sesi tetap utuh untuk kedua tab: cookie bersama sah, data dapat diambil, dan refresh berikutnya berhasil.
    for (const p of [page, tab2]) {
      const daftar = await p.evaluate(async () => {
        const res = await fetch("/api/auth/sessions");
        return {
          status: res.status,
          jumlah: ((await res.json()) as unknown[]).length,
        };
      });
      expect(daftar).toEqual({ status: 200, jumlah: 1 });
    }
    const lagi = await segarkan(tab2);
    expect(lagi).toEqual({
      status: 200,
      body: { success: true, diputar: true },
    });
  } finally {
    await tab2.close();
  }
});

// ── Rotasi refresh token (API) ───────────────────────────────────────────

test("dua refresh paralel dengan token yang sama: keduanya sukses dan sesi tetap dapat dipakai", async () => {
  const u = await penggunaSiap();
  const s = await masukApi(u.email, u.sandi, UA_MAC);

  const hasil = await Promise.all(
    [1, 2, 3, 4].map(() => refreshApi(s.refresh)),
  );

  expect(hasil.map((h) => h.status)).toEqual([200, 200, 200, 200]);
  const pemenang = hasil.filter((h) => h.refresh);
  expect(pemenang, "tepat satu menerima refresh token pengganti").toHaveLength(
    1,
  );
  for (const h of hasil) expect(await me(h.access!)).toBe(200);
  expect(
    (await refreshApi(pemenang[0].refresh!)).status,
    "token pemenang sah",
  ).toBe(200);
});

test("pemakaian ulang refresh token lama di luar jendela toleransi mencabut seluruh sesi", async () => {
  const u = await penggunaSiap();
  const s = await masukApi(u.email, u.sandi, UA_MAC);
  const lain = await masukApi(u.email, u.sandi, UA_ANDROID);

  const r1 = await refreshApi(s.refresh);
  expect(r1.status).toBe(200);
  expect(await me(r1.access!)).toBe(200);

  await tidur(LEWAT_TOLERANSI_MS);
  expect((await refreshApi(s.refresh)).status, "token lama dipakai ulang").toBe(
    401,
  );

  expect(
    (await refreshApi(r1.refresh!)).status,
    "token pengganti ikut mati",
  ).toBe(401);
  expect(await me(r1.access!), "access token sesi itu mati seketika").toBe(401);
  expect(await me(s.access)).toBe(401);
  expect(await me(lain.access), "sesi perangkat lain tidak terpengaruh").toBe(
    200,
  );
});

test("ganti kata sandi mempertahankan sesi ini dan mencabut sesi lain", async () => {
  const u = await penggunaSiap();
  const ini = await masukApi(u.email, u.sandi, UA_MAC);
  const lain = await masukApi(u.email, u.sandi, UA_ANDROID);

  const baru = sandiAcak();
  const ganti = await goApi("POST", "auth/change-password", ini.access, {
    old_password: u.sandi,
    new_password: baru,
  });
  expect(ganti.status).toBe(200);
  u.sandi = baru;
  const { access_token: aksesBaru, refresh_token: refreshBaru } = ganti.body
    .data as {
    access_token: string;
    refresh_token: string;
  };

  expect(klaim(aksesBaru).sid, "sesi yang sama").toBe(ini.sid);
  expect(await me(aksesBaru)).toBe(200);
  expect(await me(lain.access), "sesi lain dicabut").toBe(401);
  expect((await refreshApi(lain.refresh)).status).toBe(401);
  expect((await refreshApi(refreshBaru)).status).toBe(200);

  const daftar = await goApi("GET", "auth/sessions", aksesBaru);
  expect(daftar.body.data).toHaveLength(1);
  expect(daftar.body.data[0].saat_ini).toBe(true);
});

// ── Admin: paksa keluar ──────────────────────────────────────────────────

test("admin Paksa keluar di /pengguna mengakhiri semua sesi pengguna itu", async ({
  page,
}) => {
  const target = await penggunaSiap();
  const s1 = await masukApi(target.email, target.sandi, UA_ANDROID);
  const s2 = await masukApi(target.email, target.sandi, UA_IPHONE);
  const nama = `Pengguna Uji ${target.email.slice(4, target.email.indexOf("@"))}`;

  await masukSebagai(page, "superadmin");
  await buka(page, "/pengguna");
  await page
    .getByPlaceholder("Cari nama, email, peran, atau lembaga")
    .fill(target.email);
  await page.getByRole("button", { name: `Aksi untuk ${nama}` }).click();
  await page.getByRole("menuitem", { name: "Paksa keluar" }).click();

  await expect(
    page.getByText("Pengguna dipaksa keluar", { exact: true }),
  ).toBeVisible();
  for (const s of [s1, s2]) {
    expect(await me(s.access), "access token target langsung ditolak").toBe(
      401,
    );
    expect((await refreshApi(s.refresh)).status).toBe(401);
  }
  // Target dapat masuk lagi.
  expect(
    await me((await masukApi(target.email, target.sandi, UA_MAC)).access),
  ).toBe(200);
});
