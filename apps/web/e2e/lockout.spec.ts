/**
 * Lockout akun dan pesan galat login (01-F): akun terkunci setelah 5 kata sandi salah, pesan dengan hitung
 * mundur di form, kata sandi benar tetap ditolak selama terkunci, admin membuka kunci dari /pengguna, email
 * tak terdaftar berjawaban sama, dan pesan 429 batas laju BFF.
 *
 * Aturan suite ini:
 *  - Kegagalan penyebab kunci dikirim LANGSUNG ke API Go (`/api/v1/auth/login`, tanpa batas laju BFF),
 *    bukan lewat form. Hanya satu kiriman form untuk melihat pesan kunci dan satu untuk masuk setelah dibuka.
 *  - Pengguna uji sementara (`buatPenggunaUji`) dihapus di afterAll; akun seed tidak pernah dikunci atau diubah.
 *    Hitungan lockout adalah per email, jadi akun seed yang sama dengan spec lain tidak tersentuh.
 *  - Anggaran laju BFF (5/menit/IP, dipakai bersama login dan OTP): setiap kiriman form lewat
 *    `anggaranBff.pakai("login")`. Tes 429 sengaja menghabiskan jendela dengan permintaan langsung ke
 *    `/api/auth/login` (ditolak validasi 400, tidak menyentuh API Go) dan mencatat tiap yang diterima server
 *    dengan `anggaranBff.catat()`; kiriman form terakhir yang memang ditolak 429 tidak dicatat karena server
 *    pun tidak mencatatnya. Tes itu menunggu hitung mundur selesai (maks. ~60 dtk) sehingga jendela longgar lagi.
 *    Pemakaian suite: 2 kiriman form (kunci, masuk), 1 kiriman form ditolak 429, dan sampai 5 permintaan langsung.
 *  - Butuh stack yang dibangun dari kode ini (jalur 423 dan hitung mundur ada di API Go dan BFF baru).
 */
import { createHash } from "node:crypto";
import type { Page } from "@playwright/test";
import { ROLES, type Role } from "../src/lib/rbac/roles";
import {
  anggaranBff,
  buatPenggunaUji,
  hapusPenggunaUji,
  sandiAcak,
  type PenggunaUji,
} from "./auth-helpers";
import {
  apiSebagai,
  BASE_URL,
  buka,
  expect,
  jalankanSql,
  masukSebagai,
  sesiApi,
  test,
  tungguTenang,
  type PantauGalat,
} from "./fixtures";
import { cariPengguna } from "./pengguna-helpers";

// Tes yang menunggu jendela laju BFF atau hitung mundur bisa berjalan lebih dari semenit.
test.describe.configure({ timeout: 180_000 });

const alertForm = (page: Page) => page.locator('form [role="alert"]');
const tombolMasuk = (page: Page) =>
  page.getByRole("button", { name: "Masuk", exact: true });

// ── Pembantu API langsung ────────────────────────────────────────────────

interface JawabanLogin {
  status: number;
  code: string;
  message: string;
  sisaDetik?: number;
  terkunciSampai?: string;
  retryAfter?: number;
}

/** Satu percobaan login langsung ke API Go (tanpa BFF, jadi tanpa batas laju BFF). */
async function loginApi(email: string, sandi: string): Promise<JawabanLogin> {
  const res = await fetch(`${BASE_URL}/api/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Origin: BASE_URL },
    body: JSON.stringify({ email, password: sandi }),
  });
  const body = (await res.json().catch(() => ({}))) as {
    code?: string;
    message?: string;
    data?: { sisa_detik?: number; terkunci_sampai?: string };
  };
  const retry = Number(res.headers.get("Retry-After"));
  return {
    status: res.status,
    code: body.code ?? "",
    message: body.message ?? "",
    sisaDetik: body.data?.sisa_detik,
    terkunciSampai: body.data?.terkunci_sampai,
    retryAfter: Number.isFinite(retry) && retry > 0 ? retry : undefined,
  };
}

/** Mengirim `n` kata sandi salah dan menegaskan semuanya dijawab 401 INVALID_CREDENTIALS. */
async function gagalkan(email: string, n: number): Promise<void> {
  for (let i = 1; i <= n; i++) {
    const j = await loginApi(email, "Salah#Sandi1x");
    expect(j.status, `kegagalan ke-${i}`).toBe(401);
    expect(j.code).toBe("INVALID_CREDENTIALS");
  }
}

/** Pengguna uji yang sudah lepas dari kewajiban ganti kata sandi, supaya masuk lewat form mendarat di beranda. */
async function siapkanPengguna(): Promise<PenggunaUji> {
  const pengguna = await buatPenggunaUji();
  const api = await apiSebagai(pengguna.email, pengguna.sandi);
  try {
    const baru = sandiAcak();
    const res = await api.post("auth/change-password", {
      data: { old_password: pengguna.sandi, new_password: baru },
    });
    expect(res.status(), "ganti kata sandi awal").toBe(200);
    pengguna.sandi = baru;
  } finally {
    await api.dispose();
  }
  return pengguna;
}

/** "14 menit 52 detik" → 892. */
function detikDariTeks(teks: string): number {
  const menit = Number(teks.match(/(\d+)\s*menit/)?.[1] ?? 0);
  const detik = Number(teks.match(/(\d+)\s*detik/)?.[1] ?? 0);
  return menit * 60 + detik;
}

/** Mengisi form /masuk lalu mengirimnya (satu pemakaian anggaran laju login). */
async function masukLewatForm(
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
  await tombolMasuk(page).click();
}

async function harapBeranda(page: Page, role: Role): Promise<void> {
  await expect(page).toHaveURL((url) => url.pathname === ROLES[role].beranda, {
    timeout: 20_000,
  });
  await tungguTenang(page);
}

// ── Pengguna uji sementara ───────────────────────────────────────────────

const dibuat: PenggunaUji[] = [];
const emailTakTerdaftar = `tidak.ada.${Date.now()}@gardainflasi.id`;
let uKunci: PenggunaUji;
let uBanding: PenggunaUji;

test.beforeAll(async () => {
  uKunci = await siapkanPengguna();
  dibuat.push(uKunci);
  uBanding = await siapkanPengguna();
  dibuat.push(uBanding);
});

test.afterAll(async () => {
  const gagal: string[] = [];
  for (const pengguna of dibuat) {
    await hapusPenggunaUji(pengguna).catch((err: Error) =>
      gagal.push(err.message),
    );
  }
  // Baris kunci_login hanya berisi hash email; dibersihkan supaya keadaan demo kembali seperti semula.
  try {
    const hash = (email: string) =>
      createHash("sha256").update(email.toLowerCase()).digest("hex");
    const daftar = [...dibuat.map((p) => p.email), emailTakTerdaftar]
      .map((e) => `'${hash(e)}'`)
      .join(",");
    jalankanSql(`DELETE FROM kunci_login WHERE email_hash IN (${daftar});`);
  } catch (err) {
    console.warn(
      `[lockout.spec] pembersihan kunci_login dilewati: ${(err as Error).message}`,
    );
  }
  expect(gagal, "pengguna uji yang gagal dihapus").toEqual([]);
});

/** Kiriman form masuk kini dijawab JSON (fetch), jadi penolakan yang memang diharapkan tes ini diizinkan. */
function izinkanPenolakanMasuk(galat: PantauGalat, status: number): void {
  galat.izinkan(new RegExp(`^${status} POST /api/auth/login$`));
  galat.izinkan(new RegExp(`Failed to load resource.*status of ${status}`));
}

// ── Akun terkunci → pesan di form → admin membuka kunci → masuk ──────────

test("lima kata sandi salah mengunci: form menampilkan sisa waktu, kata sandi benar ditolak, admin membuka kunci, masuk berhasil", async ({
  page,
  galat,
}) => {
  izinkanPenolakanMasuk(galat, 423);
  let sebelumMuatUlang = 0;

  await test.step("lima kegagalan lewat API langsung, lalu kata sandi benar tetap ditolak 423", async () => {
    await gagalkan(uKunci.email, 5);

    const j = await loginApi(uKunci.email, uKunci.sandi);
    expect(j.status).toBe(423);
    expect(j.code).toBe("ACCOUNT_LOCKED");
    expect(j.sisaDetik, "sisa kunci ~15 menit").toBeGreaterThan(14 * 60);
    expect(j.sisaDetik).toBeLessThanOrEqual(15 * 60);
    expect(j.retryAfter, "Retry-After sama dengan sisa detik").toBe(
      j.sisaDetik,
    );
    expect(j.terkunciSampai).toBeTruthy();

    // Percobaan selama terkunci tidak memperpanjang kunci.
    const lagi = await loginApi(uKunci.email, "Salah#Sandi1x");
    expect(lagi.status).toBe(423);
    expect(lagi.terkunciSampai).toBe(j.terkunciSampai);
  });

  await test.step("satu kiriman form: pesan kunci dengan sisa waktu yang berjalan, tombol tertahan", async () => {
    await masukLewatForm(page, uKunci.email, uKunci.sandi);

    await expect(page).toHaveURL(/\/masuk$/);
    await expect(alertForm(page)).toContainText(
      /Akun terkunci karena terlalu banyak percobaan gagal\./,
    );
    await expect(alertForm(page)).toContainText(/Coba lagi dalam 1[45] menit/);
    await expect(tombolMasuk(page)).toBeDisabled();
    // Bukan pesan "salah kata sandi": kata sandinya benar, akunnya yang terkunci.
    await expect(page.getByText("Email atau password salah")).toHaveCount(0);

    // Hitung mundur berjalan.
    const timer = alertForm(page).getByRole("timer");
    const awal = await timer.innerText();
    await expect
      .poll(() => timer.innerText(), { timeout: 5_000 })
      .not.toBe(awal);
    sebelumMuatUlang = detikDariTeks(await timer.innerText());
    expect(sebelumMuatUlang).toBeGreaterThan(14 * 60 - 40);
  });

  await test.step("muat ulang tanpa kiriman ulang; kiriman berikutnya memakai sisa waktu dari server", async () => {
    await page.waitForTimeout(2_000);
    // Muat ulang adalah GET biasa (form tidak pernah dikirim native): halaman bersih, tanpa galat lama.
    await page.reload({ waitUntil: "domcontentloaded" });
    await tungguTenang(page);
    await expect(alertForm(page)).toHaveCount(0);

    // Kiriman berikutnya tetap 423 dan hitung mundurnya melanjutkan akhir kunci di server, bukan mengulang.
    await masukLewatForm(page, uKunci.email, uKunci.sandi);
    const timer = alertForm(page).getByRole("timer");
    await expect(timer).toBeVisible();
    const sesudah = detikDariTeks(await timer.innerText());
    expect(sesudah, "sisa waktu setelah muat ulang").toBeLessThan(
      sebelumMuatUlang,
    );
  });

  await test.step("kata sandi benar tetap ditolak selama terkunci (API)", async () => {
    await expect(sesiApi(uKunci.email, uKunci.sandi)).rejects.toThrow(/423/);
  });

  await test.step("admin melihat status terkunci di API dan di /pengguna", async () => {
    const api = await apiSebagai("superadmin");
    try {
      const res = await api.get(`users/${uKunci.id}`);
      expect(res.status()).toBe(200);
      const { data } = (await res.json()) as {
        data: { terkunci_sampai?: string };
      };
      expect(
        Date.parse(data.terkunci_sampai ?? ""),
        "terkunci_sampai di masa depan",
      ).toBeGreaterThan(Date.now());
    } finally {
      await api.dispose();
    }

    await masukSebagai(page, "superadmin");
    await buka(page, "/pengguna");
    await tungguTenang(page);
    await cariPengguna(page, uKunci.email);
    const baris = page.getByRole("row").filter({ hasText: uKunci.email });
    await expect(baris).toHaveCount(1);
    await expect(baris.getByText("Terkunci")).toBeVisible();
    await expect(baris.getByRole("timer")).toContainText(/mnt|dtk/);
    // Pengguna lain tidak ditandai terkunci.
    await cariPengguna(page, uBanding.email);
    await expect(
      page.getByRole("row").filter({ hasText: uBanding.email }),
    ).toHaveCount(1);
    await expect(page.getByRole("timer")).toHaveCount(0);
    await cariPengguna(page, uKunci.email);
  });

  await test.step("admin membuka kunci dari menu aksi baris", async () => {
    const baris = page.getByRole("row").filter({ hasText: uKunci.email });
    await baris
      .getByRole("button", { name: /^Aksi untuk Pengguna Uji/ })
      .click();
    await page.getByRole("menuitem", { name: "Buka kunci" }).click();

    await expect(page.getByText("Kunci dibuka").first()).toBeVisible();
    await expect(baris.getByText("Terkunci")).toHaveCount(0);

    // Setelah dibuka, menu aksi tidak lagi menawarkan Buka kunci.
    await baris
      .getByRole("button", { name: /^Aksi untuk Pengguna Uji/ })
      .click();
    await expect(
      page.getByRole("menuitem", { name: "Ubah data" }),
    ).toBeVisible();
    await expect(
      page.getByRole("menuitem", { name: "Buka kunci" }),
    ).toHaveCount(0);
    await page.keyboard.press("Escape");
  });

  await test.step("masuk lewat form berhasil dan mendarat di beranda peran", async () => {
    // Tinggalkan halaman admin dulu: polling latar sesudah cookie dihapus akan tercatat sebagai 401.
    await page.goto("about:blank");
    await page.context().clearCookies();
    await masukLewatForm(page, uKunci.email, uKunci.sandi);
    await harapBeranda(page, "OPS_AGREGATOR");
  });
});

// ── Email tak terdaftar berjawaban sama (anti-enumerasi) ─────────────────

test("email tak terdaftar mendapat urutan jawaban yang sama dengan email terdaftar, termasuk kunci", async () => {
  const urutan = async (email: string) => {
    const hasil: JawabanLogin[] = [];
    for (let i = 0; i < 7; i++)
      hasil.push(await loginApi(email, "Salah#Sandi1x"));
    return hasil;
  };

  const terdaftar = await urutan(uBanding.email);
  const takTerdaftar = await urutan(emailTakTerdaftar);

  for (let i = 0; i < terdaftar.length; i++) {
    const a = terdaftar[i];
    const b = takTerdaftar[i];
    expect(b.status, `status percobaan ${i + 1}`).toBe(a.status);
    expect(b.code, `kode percobaan ${i + 1}`).toBe(a.code);
    expect(b.message, `pesan percobaan ${i + 1}`).toBe(a.message);
    if (a.sisaDetik !== undefined) {
      expect(b.sisaDetik).toBeDefined();
      expect(Math.abs((b.sisaDetik ?? 0) - a.sisaDetik)).toBeLessThanOrEqual(
        10,
      );
    } else {
      expect(b.sisaDetik).toBeUndefined();
    }
  }
  expect(takTerdaftar.slice(0, 5).map((j) => j.status)).toEqual([
    401, 401, 401, 401, 401,
  ]);
  expect(takTerdaftar.slice(5).map((j) => j.status)).toEqual([423, 423]);
  expect(takTerdaftar[5].code).toBe("ACCOUNT_LOCKED");

  // Kata sandi benar milik email terdaftar pun dijawab sama dengan email tak terdaftar: terkunci.
  const benar = await loginApi(uBanding.email, uBanding.sandi);
  expect(benar.status).toBe(423);
  expect(benar.code).toBe("ACCOUNT_LOCKED");
});

// ── Batas laju BFF: pesan 429 di form ────────────────────────────────────

test("batas laju BFF: kiriman form yang ditolak 429 menampilkan pesan dengan hitung mundur sampai selesai", async ({
  page,
  galat,
}) => {
  izinkanPenolakanMasuk(galat, 429);
  await test.step("menghabiskan jendela laju dengan permintaan langsung ke BFF, tercatat di anggaran", async () => {
    let ditolak: Response | null = null;
    for (let i = 0; i < 8 && !ditolak; i++) {
      // Body kosong: lolos batas laju lalu ditolak validasi 400 di BFF, tanpa menyentuh API Go.
      const res = await fetch(`${BASE_URL}/api/auth/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: "{}",
      });
      if (res.status === 429) ditolak = res;
      else {
        expect(res.status).toBe(400);
        anggaranBff.catat(1); // server mencatat permintaan yang diterima
      }
    }
    expect(
      ditolak,
      "jendela laju penuh setelah paling banyak 8 permintaan",
    ).not.toBeNull();

    // Kontrak 429 BFF: header Retry-After, kode, dan sisa detik yang sama.
    const retry = Number(ditolak!.headers.get("Retry-After"));
    expect(retry).toBeGreaterThanOrEqual(1);
    expect(retry).toBeLessThanOrEqual(60);
    const body = (await ditolak!.json()) as {
      error: boolean;
      kode: string;
      sisa_detik: number;
      message: string;
    };
    expect(body.error).toBe(true);
    expect(body.kode).toBe("RATE_LIMITED");
    expect(body.sisa_detik).toBe(retry);
    expect(body.message).toMatch(
      /^Terlalu banyak percobaan dari perangkat ini\./,
    );
  });

  await test.step("kiriman form dijawab 429: pesan dan hitung mundur tampil di form", async () => {
    await page.goto("/masuk", { waitUntil: "domcontentloaded" });
    await tungguTenang(page);
    await page.getByLabel("Email institusi").fill(uKunci.email);
    await page.getByLabel("Kata sandi", { exact: true }).fill("Salah#Sandi1x");
    // Sengaja tanpa anggaranBff.pakai: kiriman ini memang harus ditolak 429, dan server tidak mencatatnya.
    await tombolMasuk(page).click();

    await expect(page).toHaveURL(/\/masuk$/);
    await expect(alertForm(page)).toContainText(
      /Terlalu banyak percobaan dari perangkat ini\. Coba lagi dalam \d+ (menit|detik)/,
    );
    await expect(tombolMasuk(page)).toBeDisabled();
    await expect(alertForm(page).getByRole("timer")).toBeVisible();
  });

  await test.step("hitung mundur selesai: pesan berganti dan tombol aktif lagi (jendela laju longgar kembali)", async () => {
    await expect(alertForm(page)).toContainText("Waktu tunggu selesai", {
      timeout: 75_000,
    });
    await expect(tombolMasuk(page)).toBeEnabled();
  });
});
