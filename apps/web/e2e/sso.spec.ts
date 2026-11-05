/**
 * Masuk dengan Google (OIDC) terhadap mock OIDC demo (mock-oauth2-server, profil compose `sso-mock`,
 * dinyalakan `make docker/demo/up`). Penerbit `http://sso.localhost:8089/default` identik bagi browser
 * (Chromium memetakan *.localhost ke loopback) dan container API (alias jaringan).
 *
 * Aturan suite ini:
 *  - Tes menautkan akun seed ke identitas mock (`sso_provider`/`sso_subject`). `beforeEach` dan `afterAll`
 *    melepas tautan itu lewat `jalankanSql` (container `E2E_PG_CONTAINER`), sehingga spec dapat diulang dan
 *    tidak meninggalkan akun seed bertaut.
 *  - Rute BFF SSO dibatasi 10 permulaan dan 10 callback per menit per IP; `anggaranSso.pakai` menunggu
 *    deterministik bila jendela penuh (sso-helpers.ts), tanpa retry acak.
 *  - Langkah OTP akun ber-MFA memakai route BFF berbatas laju bersama login: `anggaranBff.pakai("mfa")`.
 *  - Halaman login mock bukan buatan kita: `masukDiMock` mengisi username (menjadi klaim sub) dan klaim JSON.
 */
import type { Page } from "@playwright/test";
import { pesanGalatSso } from "../src/lib/auth/sso";
import { ROLES } from "../src/lib/rbac/roles";
import {
  anggaranBff,
  kodeTotp,
  msSampaiLangkahBerikut,
  RAHASIA_MFA_DEMO,
} from "./auth-helpers";
import {
  AKUN,
  BASE_URL,
  expect,
  jalankanSql,
  test,
  tungguTenang,
  type PantauGalat,
} from "./fixtures";
import {
  anggaranSso,
  masukDiMock,
  POLA_URL_MOCK,
  sqlLepasTautan,
  stubFontMock,
} from "./sso-helpers";

test.describe.configure({ timeout: 180_000 });

const TOMBOL_GOOGLE = { name: "Masuk dengan Google" };
const alertForm = (page: Page) => page.locator('form [role="alert"]');

const sql = (teks: string) => jalankanSql(teks);
const idAkun = (email: string) =>
  sql(`SELECT id FROM pengguna WHERE email = '${email}';`);
const subjekTertaut = (email: string) =>
  sql(
    `SELECT COALESCE(sso_subject, '') FROM pengguna WHERE email = '${email}';`,
  );
const hitung = (kueri: string) => Number(sql(kueri));
// audit_log append-only dan bertahan antar-jalan: perubahan dihitung relatif terhadap keadaan awal tes.
const jumlahLink = (id: string) =>
  hitung(
    `SELECT count(*) FROM audit_log WHERE aksi = 'SSO_LINK' AND aktor_id = '${id}'`,
  );
const jumlahLoginSso = (id: string) =>
  hitung(
    `SELECT count(*) FROM audit_log WHERE aksi = 'LOGIN' AND hasil = 'BERHASIL' AND aktor_id = '${id}' AND metadata->>'metode' = 'sso_google'`,
  );
const jumlahDitolak = (id: string, alasan: string, subjek: string) =>
  hitung(
    `SELECT count(*) FROM audit_log WHERE aksi = 'LOGIN' AND hasil = 'DITOLAK' AND aktor_id = '${id}' AND metadata->>'alasan' = '${alasan}' AND metadata->>'sso_subject' = '${subjek}'`,
  );

test.beforeEach(async ({ page }) => {
  sql(sqlLepasTautan());
  await stubFontMock(page);
});

test.afterAll(() => {
  sql(sqlLepasTautan());
});

/** Menekan tombol Google di /masuk sampai halaman login mock terbuka (1 permulaan + 1 callback dari anggaran). */
async function mulaiGoogle(page: Page, path = "/masuk"): Promise<void> {
  await page.goto(path, { waitUntil: "domcontentloaded" });
  const tombol = page.getByRole("link", TOMBOL_GOOGLE);
  await expect(tombol).toBeVisible();
  await anggaranSso.pakai("mulai");
  await anggaranSso.pakai("callback");
  await Promise.all([
    page.waitForURL(POLA_URL_MOCK, { timeout: 20_000 }),
    tombol.click(),
  ]);
}

/** Alur penuh sampai pengalihan balik ke aplikasi, dengan identitas mock tertentu. */
async function masukLewatGoogle(
  page: Page,
  sub: string,
  email: string,
  klaim: Record<string, unknown> = {},
  path = "/masuk",
): Promise<void> {
  await mulaiGoogle(page, path);
  await masukDiMock(page, sub, { email, ...klaim });
}

/**
 * Menunggu halaman /masuk dengan kode galat SSO tertentu dan memeriksa pesan terbaca di form.
 * `sesiSudahAda` true bila browser memang sudah punya sesi dari login sebelumnya (callback yang diputar ulang).
 */
async function harapGalatSso(
  page: Page,
  kode: string,
  sesiSudahAda = false,
): Promise<void> {
  await expect(page).toHaveURL(new RegExp(`/masuk\\?error=${kode}(&|$)`), {
    timeout: 20_000,
  });
  const pesan = pesanGalatSso(kode);
  expect(pesan, `pesan untuk ${kode}`).toBeTruthy();
  await expect(alertForm(page)).toHaveText(pesan as string);
  const cookies = await page.context().cookies();
  if (!sesiSudahAda) {
    expect(
      cookies.find((c) => c.name === "garda_access"),
      "galat SSO tidak boleh menerbitkan sesi",
    ).toBeUndefined();
  }
  expect(
    cookies.find((c) => c.name === "garda_sso"),
    "cookie alur dihapus setelah callback",
  ).toBeUndefined();
}

async function harapBeranda(page: Page, path: string): Promise<void> {
  await expect(page).toHaveURL((url) => url.pathname === path, {
    timeout: 20_000,
  });
  await tungguTenang(page);
}

// ── Tombol ───────────────────────────────────────────────────────────────

test("tombol Masuk dengan Google tampil rapi di bawah form, dengan pemisah atau", async ({
  page,
}) => {
  await page.goto("/masuk", { waitUntil: "domcontentloaded" });
  const tombol = page.getByRole("link", TOMBOL_GOOGLE);
  await expect(tombol).toBeVisible();
  await expect(tombol).toHaveAttribute("href", "/api/auth/sso/google");
  await expect(
    page.getByTestId("sso-google").getByText("atau", { exact: true }),
  ).toBeVisible();

  // Tata letak: tombol Google di bawah tombol Masuk, selebar tombol Masuk, dan di dalam kartu.
  const masuk = await page
    .getByRole("button", { name: "Masuk", exact: true })
    .boundingBox();
  const google = await tombol.boundingBox();
  const kartu = await page.locator("main > div").first().boundingBox();
  expect(masuk && google && kartu, "ketiga elemen terukur").toBeTruthy();
  expect(google!.y).toBeGreaterThan(masuk!.y + masuk!.height);
  expect(Math.abs(google!.width - masuk!.width)).toBeLessThanOrEqual(1);
  expect(google!.x).toBeGreaterThanOrEqual(kartu!.x);
  expect(google!.x + google!.width).toBeLessThanOrEqual(
    kartu!.x + kartu!.width,
  );
});

test("tujuan semula (lanjut) dibawa ke tautan Google; langkah OTP tidak menampilkannya", async ({
  page,
}) => {
  await page.goto("/masuk?lanjut=%2Faudit", { waitUntil: "domcontentloaded" });
  await expect(page.getByRole("link", TOMBOL_GOOGLE)).toHaveAttribute(
    "href",
    "/api/auth/sso/google?lanjut=%2Faudit",
  );

  await page.goto("/masuk?mfa=1", { waitUntil: "domcontentloaded" });
  await expect(
    page.getByRole("heading", { name: "Verifikasi dua langkah" }),
  ).toBeVisible();
  await expect(page.getByRole("link", TOMBOL_GOOGLE)).toHaveCount(0);
});

// ── Berhasil ─────────────────────────────────────────────────────────────

test("superadmin: tombol → login mock → beranda; akun tertaut, sesi httpOnly, audit metode; masuk lagi tanpa menaut ulang", async ({
  page,
  context,
}) => {
  const email = AKUN.superadmin;
  const id = idAkun(email);
  const linkAwal = jumlahLink(id);
  const loginAwal = jumlahLoginSso(id);
  let urlCallback = "";
  page.on("request", (req) => {
    if (req.url().includes("/api/auth/sso/google/callback"))
      urlCallback = req.url();
  });

  await mulaiGoogle(page);
  // Cookie alur terpasang selama di Google: httpOnly, SameSite=Lax (callback datang lintas situs).
  const alur = (await context.cookies()).find((c) => c.name === "garda_sso");
  expect(alur, "cookie alur").toBeTruthy();
  expect(alur).toMatchObject({ httpOnly: true, sameSite: "Lax" });
  await masukDiMock(page, "sub-superadmin", { email });

  await harapBeranda(page, ROLES.ADMIN_SISTEM.beranda);

  const cookies = await context.cookies();
  const akses = cookies.find((c) => c.name === "garda_access");
  expect(akses, "cookie akses").toBeTruthy();
  expect(akses).toMatchObject({ httpOnly: true, sameSite: "Lax" });
  expect(
    cookies.find((c) => c.name === "garda_refresh"),
    "cookie refresh juga terpasang pada pengalihan lintas situs",
  ).toMatchObject({ httpOnly: true, sameSite: "Strict" });
  expect(cookies.find((c) => c.name === "garda_csrf")).toBeTruthy();
  expect(
    cookies.find((c) => c.name === "garda_sso"),
    "cookie alur dihapus setelah dipakai",
  ).toBeUndefined();

  // Sesi benar-benar bekerja di API.
  const saya = await page.request.get("/api/auth/me");
  expect(saya.ok()).toBeTruthy();
  expect(JSON.stringify(await saya.json())).toContain(email);

  // Akun tertaut ke identitas mock dan LOGIN dicatat dengan metodenya.
  expect(await subjekTertaut(email)).toBe("sub-superadmin");
  expect(jumlahLink(id) - linkAwal, "SSO_LINK pada masuk pertama").toBe(1);
  expect(jumlahLoginSso(id) - loginAwal, "LOGIN bermetode sso_google").toBe(1);

  // Pengalihan callback yang sama tidak dapat diputar ulang di browser yang sama (cookie alur sudah hilang).
  expect(urlCallback, "URL callback tertangkap").toContain("code=");
  await anggaranSso.pakai("callback");
  await page.goto(urlCallback);
  await harapGalatSso(page, "sso_sesi_tidak_valid", true);

  // Masuk lagi dengan identitas yang sama: cocok menurut subjek, tidak ditautkan ulang.
  await context.clearCookies();
  await masukLewatGoogle(page, "sub-superadmin", email);
  await harapBeranda(page, ROLES.ADMIN_SISTEM.beranda);
  expect(jumlahLink(id) - linkAwal, "tidak ditautkan ulang").toBe(1);
  expect(jumlahLoginSso(id) - loginAwal).toBe(2);
});

test("tujuan semula (lanjut) dipertahankan setelah masuk", async ({ page }) => {
  await masukLewatGoogle(
    page,
    "sub-superadmin",
    AKUN.superadmin,
    {},
    "/masuk?lanjut=%2Faudit",
  );
  await harapBeranda(page, "/audit");
});

// ── Ditolak: pemetaan akun ───────────────────────────────────────────────

test("email tak dikenal: pesan terbaca di /masuk, tidak ada akun dibuat", async ({
  page,
}) => {
  const sebelum = hitung("SELECT count(*) FROM pengguna");

  await masukLewatGoogle(page, "sub-orang-asing", "orang.asing@example.com");

  await harapGalatSso(page, "sso_akun_tidak_ditemukan");
  await expect(alertForm(page)).toContainText("Tidak ada akun Garda Inflasi");
  await expect(page.getByRole("link", TOMBOL_GOOGLE)).toBeVisible();
  expect(
    hitung("SELECT count(*) FROM pengguna"),
    "tidak ada pendaftaran mandiri",
  ).toBe(sebelum);
});

test("email Google belum terverifikasi ditolak dan tidak menautkan akun", async ({
  page,
}) => {
  await masukLewatGoogle(page, "sub-belum", AKUN.superadmin, {
    email_verified: false,
  });

  await harapGalatSso(page, "sso_email_belum_terverifikasi");
  expect(await subjekTertaut(AKUN.superadmin)).toBe("");
});

test("akun nonaktif ditolak seperti login kata sandi dan tidak ditautkan", async ({
  page,
}) => {
  await masukLewatGoogle(page, "sub-nonaktif", "nonaktif@gardainflasi.id");

  await harapGalatSso(page, "sso_akun_nonaktif");
  expect(await subjekTertaut("nonaktif@gardainflasi.id")).toBe("");
});

test("akun yang sudah tertaut ke identitas Google lain ditolak dan diaudit; pemilik asli tetap bisa masuk", async ({
  page,
  context,
}) => {
  const email = AKUN.superadmin;
  const id = idAkun(email);
  sql(
    `UPDATE pengguna SET sso_provider = 'google', sso_subject = 'sub-pemilik-asli', sso_terhubung_pada = now() WHERE email = '${email}';`,
  );

  const ditolakAwal = jumlahDitolak(id, "sso_subjek_berbeda", "sub-penyerang");

  await masukLewatGoogle(page, "sub-penyerang", email);

  await harapGalatSso(page, "sso_akun_bentrok");
  expect(await subjekTertaut(email), "tautan lama tidak tergeser").toBe(
    "sub-pemilik-asli",
  );
  expect(
    jumlahDitolak(id, "sso_subjek_berbeda", "sub-penyerang") - ditolakAwal,
    "penolakan diaudit",
  ).toBe(1);

  await context.clearCookies();
  await masukLewatGoogle(page, "sub-pemilik-asli", email);
  await harapBeranda(page, ROLES.ADMIN_SISTEM.beranda);
});

// ── MFA ──────────────────────────────────────────────────────────────────

test("akun ber-MFA: Google → langkah OTP yang sama dengan login kata sandi → beranda", async ({
  page,
  galat,
}) => {
  // Kode yang langkah waktunya sudah pernah dipakai ditolak server (anti-replay): 401 yang wajar.
  izinkanHttp(galat, 401, "POST", "/api/auth/mfa/validate");

  await masukLewatGoogle(page, "sub-mfa", AKUN.mfa);

  await expect(page).toHaveURL(/\/masuk\?mfa=1/, { timeout: 20_000 });
  await expect(
    page.getByRole("heading", { name: "Verifikasi dua langkah" }),
  ).toBeVisible();
  expect(
    (await page.context().cookies()).find((c) => c.name === "garda_access"),
    "belum ada sesi sebelum OTP",
  ).toBeUndefined();
  await tungguTenang(page);

  const kirimKode = async (): Promise<"diterima" | "ditolak"> => {
    await anggaranBff.pakai("mfa");
    await page.getByLabel("Kode verifikasi").fill(kodeTotp(RAHASIA_MFA_DEMO));
    await page.getByRole("button", { name: "Verifikasi", exact: true }).click();
    let hasil: "diterima" | "ditolak" | "menunggu" = "menunggu";
    await expect
      .poll(
        async () => {
          if (new URL(page.url()).pathname !== "/masuk") hasil = "diterima";
          else if ((await alertForm(page).count()) > 0) hasil = "ditolak";
          return hasil;
        },
        { timeout: 20_000 },
      )
      .not.toBe("menunggu");
    return hasil as "diterima" | "ditolak";
  };

  let hasil = await kirimKode();
  if (hasil === "ditolak") {
    // Langkah waktu ini sudah dipakai (mis. spec lain): tunggu langkah berikutnya, coba sekali lagi.
    await page.waitForTimeout(msSampaiLangkahBerikut() + 1_500);
    hasil = await kirimKode();
  }
  expect(
    hasil,
    "kode OTP diterima (setelah menunggu langkah baru bila replay)",
  ).toBe("diterima");

  await harapBeranda(page, ROLES.OPS_KOORDINATOR.beranda);
  expect(await subjekTertaut(AKUN.mfa)).toBe("sub-mfa");
});

// ── Keamanan alur ────────────────────────────────────────────────────────

test("callback tanpa cookie alur, galat dari Google, dan parameter hilang ditolak dengan pesan jelas", async ({
  page,
}) => {
  await page.context().clearCookies();

  await anggaranSso.pakai("callback");
  await page.goto(
    "/api/auth/sso/google/callback?code=kode-karangan&state=state-karangan",
  );
  await harapGalatSso(page, "sso_sesi_tidak_valid");

  await anggaranSso.pakai("callback");
  await page.goto("/api/auth/sso/google/callback?error=access_denied&state=x");
  await harapGalatSso(page, "sso_dibatalkan");

  await anggaranSso.pakai("callback");
  await page.goto("/api/auth/sso/google/callback");
  await harapGalatSso(page, "sso_sesi_tidak_valid");
});

test("login CSRF: callback penyerang di browser korban ditolak, korban tidak masuk sebagai penyerang", async ({
  page,
  browser,
}) => {
  // Penyerang menjalankan alur di browsernya sendiri dan berhenti sebelum membuka callback: pengiriman
  // form login mock dicegat, pengalihan 302-nya dibaca tanpa diikuti.
  let callbackPenyerang = "";
  await page.route(POLA_URL_MOCK, async (route) => {
    if (route.request().method() !== "POST") return route.continue();
    const res = await route.fetch({ maxRedirects: 0 });
    callbackPenyerang = res.headers()["location"] ?? "";
    return route.fulfill({
      status: 200,
      contentType: "text/html",
      body: "<p>tertahan</p>",
    });
  });
  await masukLewatGoogle(page, "sub-superadmin", AKUN.superadmin);
  await expect
    .poll(() => callbackPenyerang, { timeout: 20_000 })
    .toContain("code=");

  const korban = await browser.newContext({
    baseURL: BASE_URL,
    locale: "id-ID",
  });
  try {
    const halaman = await korban.newPage();
    await stubFontMock(halaman);

    // Korban tanpa cookie alur.
    await anggaranSso.pakai("callback");
    await halaman.goto(callbackPenyerang);
    await harapGalatSso(halaman, "sso_sesi_tidak_valid");

    // Korban sendiri sedang di tengah alur (punya binding sendiri): binding berbeda, state penyerang hangus.
    await halaman.goto("/masuk", { waitUntil: "domcontentloaded" });
    await anggaranSso.pakai("mulai");
    await Promise.all([
      halaman.waitForURL(POLA_URL_MOCK, { timeout: 20_000 }),
      halaman.getByRole("link", TOMBOL_GOOGLE).click(),
    ]);
    await anggaranSso.pakai("callback");
    await halaman.goto(callbackPenyerang);
    await harapGalatSso(halaman, "sso_sesi_tidak_valid");
  } finally {
    await korban.close();
  }

  // State penyerang sudah hangus: ia pun tidak dapat memakainya lagi.
  await anggaranSso.pakai("callback");
  await page.goto(callbackPenyerang);
  await harapGalatSso(page, "sso_sesi_tidak_valid");
  expect(
    await subjekTertaut(AKUN.superadmin),
    "tidak ada penautan dari alur yang ditolak",
  ).toBe("");
});

/** Mengizinkan satu respons HTTP gagal yang memang diharapkan, beserta baris console bawaan browser untuknya. */
function izinkanHttp(
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
