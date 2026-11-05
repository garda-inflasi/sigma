/**
 * Halaman autentikasi lewat form sungguhan: masuk, keluar, MFA, lupa/atur ulang kata sandi (email
 * ditangkap Mailpit), dan ganti kata sandi, ditambah pemeriksaan API publik kata sandi.
 *
 * Aturan suite ini:
 *  - Tes yang mengubah kata sandi memakai pengguna sementara `e2e.<timestamp>.<n>@gardainflasi.id`
 *    (POST /v1/users sebagai superadmin) yang dihapus di afterAll; kata sandi akun seed tidak disentuh.
 *  - Login lewat form BFF dibatasi 5 per menit per IP, dan forgot-password memakai jendela yang sama
 *    dengan batas 3 (lib/bff/rate-limit.ts). Setiap aksi yang mengenai route itu melewati
 *    `anggaranBff.pakai(...)` yang menunggu deterministik bila jendela penuh; tidak ada retry acak.
 *    Pemakaian suite ini: forgot 1, login 1 (sukses) + 1 (salah) + 1 (MFA) + 1 (OTP, +1 bila kode
 *    ditolak karena replay) + 1 (login kata sandi baru setelah reset) + 2 (alur ganti kata sandi)
 *    = 8 sampai 9; sisanya menunggu jendela longgar (kira-kira satu menit total).
 *  - Alur atur ulang membutuhkan Mailpit (E2E_MAILPIT_URL, bawaan http://localhost:8025).
 *  - Tes lain masuk lewat API (`masukSebagai`), yang tidak berbatas laju.
 */
import { ROLES, type Role } from "../src/lib/rbac/roles";
import {
  anggaranBff,
  apiPublik,
  buatPenggunaUji,
  hapusPenggunaUji,
  kodeTotp,
  msSampaiLangkahBerikut,
  pastikanMailpit,
  RAHASIA_MFA_DEMO,
  sandiAcak,
  tautanReset,
  tungguEmail,
  type PenggunaUji,
} from "./auth-helpers";
import {
  AKUN,
  apiSebagai,
  buka,
  expect,
  masukSebagai,
  sesiApi,
  SANDI_DEMO,
  test,
  tungguTenang,
  type PantauGalat,
} from "./fixtures";
import type { Page } from "@playwright/test";

// Tes yang menunggu jendela laju BFF bisa berjalan lebih dari semenit.
test.describe.configure({ timeout: 180_000 });

const alertForm = (page: Page) => page.locator('form [role="alert"]');

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

/** Mengisi form /masuk, lalu mengirimnya (satu pemakaian anggaran laju login). */
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
  await page.getByRole("button", { name: "Masuk", exact: true }).click();
}

/** Menunggu sampai halaman berada di beranda peran (alih-alih URL harfiah, supaya mengikuti ROLES). */
async function harapBeranda(page: Page, role: Role): Promise<void> {
  await expect(page).toHaveURL((url) => url.pathname === ROLES[role].beranda, {
    timeout: 20_000,
  });
  await tungguTenang(page);
}

// ── Pengguna uji sementara ───────────────────────────────────────────────

const dibuat: PenggunaUji[] = [];
let uReset: PenggunaUji;
let uSalah: PenggunaUji;
let uGanti: PenggunaUji;
let uApi: PenggunaUji;

test.beforeAll(async () => {
  for (const nama of ["reset", "salah", "ganti", "api"] as const) {
    const pengguna = await buatPenggunaUji();
    dibuat.push(pengguna);
    if (nama === "reset") uReset = pengguna;
    if (nama === "salah") uSalah = pengguna;
    if (nama === "ganti") uGanti = pengguna;
    if (nama === "api") uApi = pengguna;
  }
});

test.afterAll(async () => {
  const gagal: string[] = [];
  for (const pengguna of dibuat) {
    await hapusPenggunaUji(pengguna).catch((err: Error) =>
      gagal.push(err.message),
    );
  }
  expect(gagal, "pengguna uji yang gagal dihapus").toEqual([]);
});

// ── Lupa dan atur ulang kata sandi (dijalankan pertama: butuh jendela laju longgar) ────

test("lupa kata sandi → email Mailpit → atur ulang → masuk dengan sandi baru; tautan bekas ditolak", async ({
  page,
  galat,
}) => {
  await pastikanMailpit();
  const sandiLama = uReset.sandi;
  const sandiBaru = sandiAcak();
  const sandiKedua = sandiAcak();

  await test.step("minta tautan; jawabannya generik", async () => {
    await page.goto("/lupa-password", { waitUntil: "domcontentloaded" });
    await tungguTenang(page);
    await page.getByLabel("Email institusi").fill(uReset.email);
    await anggaranBff.pakai("lupa");
    await page.getByRole("button", { name: /Kirim tautan atur ulang/ }).click();
    await expect(
      page.getByText(/tautan atur ulang kata sandi sudah dikirim/),
    ).toBeVisible();
  });

  let path = "";
  await test.step("ambil tautan dari kotak masuk Mailpit", async () => {
    const email = await tungguEmail(uReset.email);
    expect(email.subjek).toContain("Reset Password");
    ({ path } = tautanReset(email.isi));
  });

  await test.step("atur kata sandi baru lewat tautan", async () => {
    await page.goto(path, { waitUntil: "domcontentloaded" });
    await tungguTenang(page);
    await expect(
      page.getByRole("heading", { name: "Buat kata sandi baru" }),
    ).toBeVisible();
    await page.getByLabel("Kata sandi baru", { exact: true }).fill(sandiBaru);
    await page.getByLabel("Ulangi kata sandi baru").fill(sandiBaru);
    await page.getByRole("button", { name: /Atur ulang kata sandi/ }).click();
    await expect(page.getByText(/berhasil diatur ulang/)).toBeVisible();
    await expect(page.getByRole("link", { name: "Masuk" })).toBeVisible();
    uReset.sandi = sandiBaru;
  });

  await test.step("kata sandi lama ditolak, kata sandi baru berhasil", async () => {
    await expect(sesiApi(uReset.email, sandiLama)).rejects.toThrow(/401/);
    await masukLewatForm(page, uReset.email, sandiBaru);
    // Reset menghapus kewajiban ganti kata sandi, jadi langsung ke beranda.
    await harapBeranda(page, "OPS_AGREGATOR");
  });

  await test.step("tautan yang sudah dipakai ditolak", async () => {
    izinkanHttp(galat, 401, "POST", "/api/auth/reset-password");
    await page.goto(path, { waitUntil: "domcontentloaded" });
    await tungguTenang(page);
    await page.getByLabel("Kata sandi baru", { exact: true }).fill(sandiKedua);
    await page.getByLabel("Ulangi kata sandi baru").fill(sandiKedua);
    await page.getByRole("button", { name: /Atur ulang kata sandi/ }).click();
    await expect(alertForm(page)).toContainText(
      /tidak valid atau sudah dipakai/,
    );
    await expect(
      page.getByRole("link", { name: "Minta tautan baru" }),
    ).toBeVisible();
    // Kata sandi tidak berubah oleh percobaan kedua.
    await expect(sesiApi(uReset.email, sandiKedua)).rejects.toThrow(/401/);
    await sesiApi(uReset.email, sandiBaru);
  });
});

test("/reset-password tanpa token menawarkan tautan baru", async ({ page }) => {
  await page.goto("/reset-password", { waitUntil: "domcontentloaded" });
  await expect(page.getByText(/Tautan tidak lengkap/)).toBeVisible();
  await page.getByRole("link", { name: "Minta tautan baru" }).click();
  await expect(page).toHaveURL(/\/lupa-password$/);
});

// ── Masuk dan keluar ─────────────────────────────────────────────────────

test("halaman masuk tidak lagi menawarkan SSO Google dan menaut ke lupa kata sandi", async ({
  page,
}) => {
  await page.goto("/masuk", { waitUntil: "domcontentloaded" });
  await expect(
    page.getByRole("heading", { name: "Masuk ke Garda Inflasi" }),
  ).toBeVisible();
  await expect(page.getByText(/Google/)).toHaveCount(0);
  await expect(page.getByText("atau", { exact: true })).toHaveCount(0);
  await page.getByRole("link", { name: "Lupa kata sandi?" }).click();
  await expect(page).toHaveURL(/\/lupa-password$/);
});

test("masuk berhasil mendarat di beranda peran", async ({ page }) => {
  await masukLewatForm(page, AKUN.superadmin, SANDI_DEMO);
  await harapBeranda(page, "ADMIN_SISTEM");
  await expect(page.getByRole("button", { name: "Menu akun" })).toBeVisible();
});

test("kata sandi salah menampilkan galat dan tetap di halaman masuk", async ({
  page,
}) => {
  await masukLewatForm(page, uSalah.email, "Salah#Sandi1x");
  await expect(page).toHaveURL(/\/masuk\?error=/);
  await expect(page.getByText("Email atau password salah")).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Masuk ke Garda Inflasi" }),
  ).toBeVisible();
  await expect(page.getByLabel("Kata sandi", { exact: true })).toHaveValue("");
});

test("keluar mencabut sesi dan kembali ke halaman masuk", async ({ page }) => {
  await masukSebagai(page, "superadmin");
  await buka(page, "/pengguna");
  await tungguTenang(page);
  const cookieAkses = async () =>
    (await page.context().cookies()).find((c) => c.name === "garda_access");
  expect(await cookieAkses(), "cookie sesi sebelum keluar").toBeDefined();

  await page.getByRole("button", { name: "Menu akun" }).click();
  await page.getByRole("menuitem", { name: "Keluar" }).click();

  await expect(page).toHaveURL(/\/masuk$/);
  // Cookie httpOnly hanya bisa dihapus server: bukti BFF logout benar-benar dipanggil.
  expect(await cookieAkses(), "cookie sesi setelah keluar").toBeUndefined();
  await page.goto("/pengguna");
  await expect(page).toHaveURL(/\/masuk\?lanjut=%2Fpengguna/);
});

// ── MFA ──────────────────────────────────────────────────────────────────

test("akun ber-MFA: kata sandi → kode OTP → beranda", async ({
  page,
  galat,
}) => {
  // Kode yang langkah waktunya sudah pernah dipakai ditolak server (anti-replay): 401 yang wajar.
  izinkanHttp(galat, 401, "POST", "/api/auth/mfa/validate");

  await masukLewatForm(page, AKUN.mfa, SANDI_DEMO);
  await expect(page).toHaveURL(/\/masuk\?mfa=1/);
  await expect(
    page.getByRole("heading", { name: "Verifikasi dua langkah" }),
  ).toBeVisible();
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
    // Langkah waktu ini sudah dipakai (mis. proses sebelumnya): tunggu langkah berikutnya, coba sekali lagi.
    await page.waitForTimeout(msSampaiLangkahBerikut() + 1_500);
    hasil = await kirimKode();
  }
  expect(
    hasil,
    "kode OTP diterima (setelah menunggu langkah baru bila replay)",
  ).toBe("diterima");
  await harapBeranda(page, "OPS_KOORDINATOR");
});

// ── Ganti kata sandi ─────────────────────────────────────────────────────

test("pengguna baru wajib ganti kata sandi, lalu masuk dengan kata sandi baru", async ({
  page,
  galat,
}) => {
  const sandiAwal = uGanti.sandi;
  const sandiBaru = sandiAcak();

  await test.step("login pertama dialihkan ke /ganti-password", async () => {
    await masukLewatForm(page, uGanti.email, sandiAwal);
    await expect(page).toHaveURL(/\/ganti-password(\?|$)/);
    await expect(
      page.getByRole("heading", { name: "Ganti kata sandi" }),
    ).toBeVisible();
    // Halaman lain tertutup selama kewajiban belum dipenuhi.
    await page.goto("/cold-storage", { waitUntil: "domcontentloaded" });
    await expect(page).toHaveURL(/\/ganti-password(\?|$)/);
  });

  await test.step("kata sandi saat ini yang salah ditolak server", async () => {
    izinkanHttp(galat, 401, "POST", "/api/auth/change-password");
    await tungguTenang(page);
    await page.getByLabel("Kata sandi saat ini").fill("Salah#Lama1x");
    await page.getByLabel("Kata sandi baru", { exact: true }).fill(sandiBaru);
    await page.getByLabel("Ulangi kata sandi baru").fill(sandiBaru);
    await expect(
      page.getByRole("list", { name: "Aturan kata sandi" }),
    ).toContainText("Minimal 8 karakter");
    await page.getByRole("button", { name: /Simpan dan lanjutkan/ }).click();
    await expect(alertForm(page)).toContainText("Password lama salah");
    await expect(page).toHaveURL(/\/ganti-password(\?|$)/);
  });

  await test.step("ganti kata sandi → beranda", async () => {
    await page.getByLabel("Kata sandi saat ini").fill(sandiAwal);
    await page.getByRole("button", { name: /Simpan dan lanjutkan/ }).click();
    await harapBeranda(page, "OPS_AGREGATOR");
    uGanti.sandi = sandiBaru;
  });

  await test.step("masuk ulang dengan kata sandi baru; kata sandi awal ditolak", async () => {
    await page.context().clearCookies();
    await masukLewatForm(page, uGanti.email, sandiBaru);
    // Kewajiban ganti kata sandi sudah hilang: tidak dialihkan lagi.
    await harapBeranda(page, "OPS_AGREGATOR");
    await expect(sesiApi(uGanti.email, sandiAwal)).rejects.toThrow(/401/);
  });
});

// ── API publik kata sandi ────────────────────────────────────────────────

test.describe("API kata sandi", () => {
  test("forgot-password menjawab sama untuk email terdaftar dan tak dikenal", async () => {
    const api = await apiPublik();
    try {
      const dikenal = await api.post("auth/forgot-password", {
        data: { email: uApi.email },
      });
      const takDikenal = await api.post("auth/forgot-password", {
        data: { email: `tidak.ada.${Date.now()}@gardainflasi.id` },
      });
      expect(dikenal.status()).toBe(200);
      expect(takDikenal.status()).toBe(200);
      expect(await takDikenal.json()).toEqual(await dikenal.json());
    } finally {
      await api.dispose();
    }
  });

  test("reset-password dengan token tak dikenal ditolak", async () => {
    const api = await apiPublik();
    try {
      const res = await api.post("auth/reset-password", {
        data: {
          token: "00000000-0000-4000-8000-000000000000",
          password: sandiAcak(),
        },
      });
      expect([400, 401]).toContain(res.status());
      expect((await res.json()).code).not.toBe("200");
    } finally {
      await api.dispose();
    }
  });

  test("change-password dengan kata sandi saat ini yang salah ditolak", async () => {
    const api = await apiSebagai(uApi.email, uApi.sandi);
    try {
      const res = await api.post("auth/change-password", {
        data: { old_password: "Salah#Lama1x", new_password: sandiAcak() },
      });
      expect(res.status()).toBe(401);
      // Kata sandi tetap yang lama.
      await sesiApi(uApi.email, uApi.sandi);
    } finally {
      await api.dispose();
    }
  });
});
