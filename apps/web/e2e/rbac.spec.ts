/**
 * RBAC berbasis basis data (02-C): izin langsung per pengguna dan matriks peran × izin yang dapat diedit.
 *
 * Aturan suite ini:
 *  - Pengguna yang izinnya diubah adalah pengguna sementara `e2e.<timestamp>.<n>@gardainflasi.id` (OPS_AGREGATOR,
 *    tanpa `kpi.lihat` secara bawaan) yang dibuat lewat API dan dihapus di afterAll. Superadmin hanya bertindak
 *    sebagai admin (sesi baru milik tes) dan tidak pernah diubah izinnya.
 *  - Matriks demo tidak boleh tersisa berubah: setiap tes yang menyentuhnya memulihkan matriks bawaan di
 *    `finally`, dan afterAll memulihkannya sekali lagi (`pulihkanMatriks` membandingkan dengan `bawaan` dari API).
 *  - Izin yang berubah mengubah salinan izin di token; API menjawab 401 PERMISSIONS_CHANGED untuk token basi
 *    dan klien menyegarkannya lewat refresh senyap. Tes API memakai refresh itu secara eksplisit; tes browser
 *    memastikan UI menyusul sendiri (sidebar, halaman) tanpa pengguna masuk ulang.
 *  - Cache izin API berumur 15 detik untuk perubahan di luar proses (SQL langsung), tetapi perubahan lewat API
 *    berlaku pada request berikutnya dan izin yang kedaluwarsa berhenti tepat waktunya.
 *  - Tes yang sengaja memakai token basi mengizinkan 401 sementara sebelum klien memulihkan sesi.
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
  jalankanSql,
  masukSebagai,
  PETUNJUK_BATAS_LAJU,
  sesiApi,
  test,
  tungguTenang,
} from "./fixtures";

test.describe.configure({ mode: "serial", timeout: 120_000 });

/** Izin yang tidak dimiliki OPS_AGREGATOR (peran pengguna uji) secara bawaan; membuka halaman /kpi. */
const IZIN_UJI = "kpi.lihat";
const LABEL_PERAN_UJI = "Unit Agregator";
const ID_PERAN_UJI = "OPS_AGREGATOR";

// ── Klien API Go ─────────────────────────────────────────────────────────

interface JawabanApi {
  status: number;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  body: any;
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

function klaim(
  access: string,
): { permissions?: string[] } & Record<string, unknown> {
  return JSON.parse(
    Buffer.from(access.split(".")[1], "base64url").toString("utf8"),
  ) as { permissions?: string[] } & Record<string, unknown>;
}

interface SesiApi {
  access: string;
  refresh: string;
}

async function masukApi(email: string, sandi: string): Promise<SesiApi> {
  const res = await fetch(`${BASE_URL}/api/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Origin: BASE_URL },
    body: JSON.stringify({ email, password: sandi }),
  });
  const json = (await res.json().catch(() => ({}))) as {
    data?: { access_token?: string; refresh_token?: string };
  };
  if (!res.ok || !json.data?.access_token || !json.data.refresh_token)
    throw new Error(
      `Login ${email} gagal (${res.status})${res.status === 429 ? ` — ${PETUNJUK_BATAS_LAJU}` : ""}`,
    );
  return { access: json.data.access_token, refresh: json.data.refresh_token };
}

/** Refresh senyap seperti klien web: access token baru (refresh token dirotasi bila diterbitkan). */
async function segarkan(sesi: SesiApi): Promise<SesiApi> {
  const r = await goApi("POST", "auth/refresh", undefined, {
    refresh_token: sesi.refresh,
  });
  expect(r.status, "refresh").toBe(200);
  return {
    access: r.body.data.access_token,
    refresh: r.body.data.refresh_token || sesi.refresh,
  };
}

const kpi = (access: string) => goApi("GET", "kpi", access);
const tidur = (ms: number) => new Promise((r) => setTimeout(r, ms));

async function tungguSampai(
  kondisi: () => Promise<boolean>,
  batasMs: number,
  pesan: string,
): Promise<number> {
  const mulai = Date.now();
  while (!(await kondisi())) {
    if (Date.now() - mulai > batasMs)
      throw new Error(`Melewati ${batasMs} ms: ${pesan}`);
    await tidur(500);
  }
  return Date.now() - mulai;
}

const tokenAdmin = async () => (await sesiApi("superadmin")).accessToken;

// ── Matriks ──────────────────────────────────────────────────────────────

interface PeranApi {
  id: string;
  label: string;
  permissions: string[];
  bawaan: string[];
  versi: number;
}

async function bacaPeran(admin: string): Promise<PeranApi[]> {
  const r = await goApi("GET", "roles", admin);
  expect(r.status).toBe(200);
  return r.body.data as PeranApi[];
}

const samaHimpunan = (a: string[], b: string[]) =>
  a.length === b.length && a.every((p) => b.includes(p));

/** Mengembalikan setiap peran yang menyimpang ke matriks bawaan (tidak pernah membiarkan matriks demo berubah). */
async function pulihkanMatriks(): Promise<void> {
  const admin = await tokenAdmin();
  for (const p of await bacaPeran(admin)) {
    if (samaHimpunan(p.permissions, p.bawaan)) continue;
    const r = await goApi("PUT", `roles/${p.id}/permissions`, admin, {
      permissions: p.bawaan,
      versi: p.versi,
    });
    expect(
      r.status,
      `pulihkan matriks ${p.id}: ${JSON.stringify(r.body)}`,
    ).toBe(200);
  }
}

// ── Pengguna uji ─────────────────────────────────────────────────────────

const dibuat: PenggunaUji[] = [];

test.afterAll(async () => {
  const gagal: string[] = [];
  for (const pengguna of dibuat) {
    await hapusPenggunaUji(pengguna).catch((err: Error) =>
      gagal.push(err.message),
    );
  }
  await pulihkanMatriks().catch((err: Error) => gagal.push(err.message));
  expect(gagal, "pembersihan yang gagal").toEqual([]);
});

/** Pengguna uji yang sudah menyelesaikan kewajiban ganti kata sandi (akun buatan admin diwajibkan). */
async function penggunaSiap(): Promise<PenggunaUji> {
  const pengguna = await buatPenggunaUji();
  dibuat.push(pengguna);
  const awal = await masukApi(pengguna.email, pengguna.sandi);
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
async function tanamCookie(page: Page, sesi: SesiApi): Promise<void> {
  const { hostname, protocol } = new URL(BASE_URL);
  const dasar = { domain: hostname, secure: protocol === "https:" };
  await page.context().clearCookies();
  await page.context().addCookies([
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

/** 401 sementara yang diharapkan sebelum klien memulihkan sesi, beserta baris console bawaan browser. */
function izinkan401(galat: { izinkan(p: RegExp): void }): void {
  galat.izinkan(/^401 /);
  galat.izinkan(/Failed to load resource.*status of 401/);
}

const hariKeDepan = (n: number) => {
  const d = new Date(Date.now() + n * 86_400_000);
  const p = (x: number) => String(x).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
};

// ── API: izin langsung ───────────────────────────────────────────────────

test("matriks di server sama dengan bawaan di kode (demo tidak tersisa berubah)", async () => {
  const peran = await bacaPeran(await tokenAdmin());
  expect(peran).toHaveLength(14);
  for (const p of peran) {
    expect(
      samaHimpunan(p.permissions, p.bawaan),
      `${p.id} menyimpang dari bawaan`,
    ).toBe(true);
  }
});

test("izin langsung: token basi ditolak 401 PERMISSIONS_CHANGED, token segar diterima; dicabut menutup lagi", async () => {
  const u = await penggunaSiap();
  let sesi = await masukApi(u.email, u.sandi);
  expect((await kpi(sesi.access)).status, "tanpa izin").toBe(403);
  const admin = await tokenAdmin();

  const beri = await goApi("POST", `users/${u.id}/permissions`, admin, {
    izin: IZIN_UJI,
    alasan: "e2e: izin langsung",
  });
  expect(beri.status, JSON.stringify(beri.body)).toBe(201);
  expect(beri.body.data).toMatchObject({
    izin: IZIN_UJI,
    aktif: true,
    berlaku_sampai: null,
  });

  const basi = await kpi(sesi.access);
  expect(basi.status, "token lama membawa izin sebelum pemberian").toBe(401);
  expect(basi.body.code).toBe("PERMISSIONS_CHANGED");
  sesi = await segarkan(sesi);
  expect(klaim(sesi.access).permissions).toContain(IZIN_UJI);
  expect(
    (await kpi(sesi.access)).status,
    "pengguna: API berikutnya berhasil",
  ).toBe(200);

  const cabut = await goApi(
    "DELETE",
    `users/${u.id}/permissions/${IZIN_UJI}`,
    admin,
  );
  expect(cabut.status).toBe(200);
  expect(
    (await kpi(sesi.access)).status,
    "pencabutan berlaku seketika, tanpa menunggu token 15 menit",
  ).toBe(401);
  sesi = await segarkan(sesi);
  expect(klaim(sesi.access).permissions).not.toContain(IZIN_UJI);
  expect((await kpi(sesi.access)).status).toBe(403);
});

test("izin langsung berkedaluwarsa berhenti berlaku tepat waktu (tanpa menunggu cache)", async () => {
  const u = await penggunaSiap();
  let sesi = await masukApi(u.email, u.sandi);
  const admin = await tokenAdmin();
  const sampai = new Date(Date.now() + 6_000).toISOString();

  const beri = await goApi("POST", `users/${u.id}/permissions`, admin, {
    izin: IZIN_UJI,
    alasan: "e2e: izin sementara",
    berlaku_sampai: sampai,
  });
  expect(beri.status, JSON.stringify(beri.body)).toBe(201);
  sesi = await segarkan(sesi);
  expect((await kpi(sesi.access)).status, "selama masih berlaku").toBe(200);

  await tidur(7_000);

  const lewat = await kpi(sesi.access);
  expect(
    lewat.status,
    "kedaluwarsa: tidak lagi 200 walau cache izin 15 detik belum habis",
  ).toBe(401);
  expect(lewat.body.code).toBe("PERMISSIONS_CHANGED");
  sesi = await segarkan(sesi);
  expect((await kpi(sesi.access)).status).toBe(403);

  // Riwayat tetap terbaca sebagai tidak aktif, dan bisa diberikan lagi.
  const daftar = await goApi("GET", `users/${u.id}/permissions`, admin);
  expect(daftar.body.data.langsung[0]).toMatchObject({
    izin: IZIN_UJI,
    aktif: false,
  });
  const lagi = await goApi("POST", `users/${u.id}/permissions`, admin, {
    izin: IZIN_UJI,
    alasan: "e2e: diberikan lagi",
  });
  expect(lagi.status).toBe(201);
});

test("izin langsung yang dikedaluwarsakan lewat SQL berhenti berlaku dalam batas cache API", async () => {
  const u = await penggunaSiap();
  let sesi = await masukApi(u.email, u.sandi);
  const admin = await tokenAdmin();
  const beri = await goApi("POST", `users/${u.id}/permissions`, admin, {
    izin: IZIN_UJI,
    alasan: "e2e: kedaluwarsa lewat SQL",
  });
  expect(beri.status).toBe(201);
  sesi = await segarkan(sesi);
  expect((await kpi(sesi.access)).status).toBe(200);

  jalankanSql(
    `UPDATE user_permissions SET berlaku_sampai = now() - interval '1 minute' WHERE pengguna_id = '${u.id}' AND permission_nama = '${IZIN_UJI}'`,
  );

  const lama = await tungguSampai(
    async () => (await kpi(sesi.access)).status === 401,
    45_000,
    "izin yang dikedaluwarsakan lewat SQL tetap berlaku",
  );
  // Batas terdokumentasi: umur cache izin (bawaan 15 detik) ditambah keterlambatan jaringan.
  expect(lama).toBeLessThan(30_000);
  sesi = await segarkan(sesi);
  expect((await kpi(sesi.access)).status).toBe(403);
});

test("endpoint matriks dan izin langsung menolak pengguna tanpa pengguna.kelola (403) dan tanpa token (401)", async () => {
  const u = await penggunaSiap();
  const sesi = await masukApi(u.email, u.sandi);
  const peran = [
    ["GET", "roles", undefined],
    [
      "PUT",
      `roles/${ID_PERAN_UJI}/permissions`,
      { permissions: [IZIN_UJI], versi: 1 },
    ],
    ["GET", `users/${u.id}/permissions`, undefined],
    [
      "POST",
      `users/${u.id}/permissions`,
      { izin: IZIN_UJI, alasan: "naik sendiri" },
    ],
    ["DELETE", `users/${u.id}/permissions/${IZIN_UJI}`, undefined],
  ] as const;
  for (const [method, path, body] of peran) {
    expect(
      (await goApi(method, path, sesi.access, body)).status,
      `${method} ${path}`,
    ).toBe(403);
    expect(
      (await goApi(method, path, undefined, body)).status,
      `${method} ${path} tanpa token`,
    ).toBe(401);
  }
  expect((await kpi(sesi.access)).status, "tidak ada efek samping").toBe(403);
});

// ── Browser: izin langsung ───────────────────────────────────────────────

test("pengguna: izin langsung diberi admin → menu dan halaman terbuka lewat refresh senyap; dicabut → tertutup lagi", async ({
  page,
  galat,
}) => {
  izinkan401(galat);
  const u = await penggunaSiap();
  await tanamCookie(page, await masukApi(u.email, u.sandi));
  const menu = page.getByRole("navigation", { name: "Navigasi utama" });
  const tautanKpi = menu.getByRole("link", { name: "Indikator kinerja" });

  await buka(page, "/cold-storage");
  await tungguTenang(page);
  await expect(tautanKpi, "belum punya izin").toHaveCount(0);

  const admin = await tokenAdmin();
  const beri = await goApi("POST", `users/${u.id}/permissions`, admin, {
    izin: IZIN_UJI,
    alasan: "e2e: buka halaman KPI",
  });
  expect(beri.status, JSON.stringify(beri.body)).toBe(201);

  // Panggilan API pengguna berikutnya (muat ulang halaman) dijawab 401 PERMISSIONS_CHANGED, klien menyegarkan
  // sesi, dan layout membaca ulang izin: menu "Indikator kinerja" muncul tanpa masuk ulang.
  await page.reload({ waitUntil: "domcontentloaded" });
  await expect(tautanKpi).toBeVisible({ timeout: 20_000 });
  await tautanKpi.click();
  await expect(page).toHaveURL(/\/kpi$/);
  await expect(
    page.getByRole("heading", { name: "Indikator kinerja" }),
  ).toBeVisible();

  const cabut = await goApi(
    "DELETE",
    `users/${u.id}/permissions/${IZIN_UJI}`,
    admin,
  );
  expect(cabut.status).toBe(200);

  await page.goto("/cold-storage", { waitUntil: "domcontentloaded" });
  await expect(
    tautanKpi,
    "izin dicabut: menu menyusul tanpa masuk ulang",
  ).toHaveCount(0, { timeout: 20_000 });
  await page.goto("/kpi");
  await expect(page).toHaveURL(/\/akses-ditolak/);
});

test("navigasi langsung ke halaman yang baru diizinkan memulihkan token dulu, bukan langsung ditolak", async ({
  page,
  galat,
}) => {
  izinkan401(galat);
  const u = await penggunaSiap();
  await tanamCookie(page, await masukApi(u.email, u.sandi));
  await buka(page, "/cold-storage");
  await tungguTenang(page);

  const beri = await goApi(
    "POST",
    `users/${u.id}/permissions`,
    await tokenAdmin(),
    { izin: IZIN_UJI, alasan: "e2e: navigasi langsung" },
  );
  expect(beri.status).toBe(201);

  // Token di cookie masih basi (tanpa kpi.lihat): edge menukar refresh token sekali, lalu halaman terbuka.
  await page.goto("/kpi", { waitUntil: "domcontentloaded" });
  await expect(page).not.toHaveURL(/\/(masuk|akses-ditolak)/);
  await expect(
    page.getByRole("heading", { name: "Indikator kinerja" }),
  ).toBeVisible();
});

// ── Browser: dialog "Izin langsung" ──────────────────────────────────────

test("admin: dialog Izin langsung memberi izin dengan masa berlaku dan alasan, menampilkannya, lalu mencabutnya", async ({
  page,
}) => {
  const u = await penggunaSiap();
  await masukSebagai(page, "superadmin");
  await buka(page, "/pengguna");
  await page
    .getByPlaceholder("Cari nama, email, peran, atau lembaga")
    .fill(u.email);
  const baris = page.getByRole("row").filter({ hasText: u.email });
  await baris.getByRole("button", { name: /^Aksi untuk/ }).click();
  await page.getByRole("menuitem", { name: "Ubah data" }).click();

  const dialog = page.getByRole("dialog");
  await dialog.getByRole("tab", { name: "Izin langsung" }).click();
  await expect(
    dialog.getByRole("heading", { name: /^Izin langsung/ }),
  ).toBeVisible();
  await expect(
    dialog.getByText("Belum ada izin langsung untuk pengguna ini."),
  ).toBeVisible();

  // Validasi klien: alasan wajib, izin wajib dipilih.
  await dialog.getByRole("button", { name: "Berikan izin" }).click();
  await expect(
    dialog.getByText("Pilih izin yang akan diberikan."),
  ).toBeVisible();
  await expect(dialog.getByText("Alasan wajib diisi.")).toBeVisible();

  // Izin yang sudah dimiliki lewat peran tidak ditawarkan.
  await dialog.getByLabel("Izin", { exact: true }).click();
  await expect(page.getByRole("option", { name: /^harga\.input/ })).toHaveCount(
    0,
  );
  await page
    .getByRole("option", {
      name: new RegExp(`^${IZIN_UJI.replace(".", "\\.")}`),
    })
    .click();

  await dialog.getByLabel("Berlaku sampai (opsional)").fill(hariKeDepan(30));
  await dialog.getByLabel("Alasan").fill("e2e: proyek indikator kinerja");
  await dialog.getByRole("button", { name: "Berikan izin" }).click();

  const butir = dialog
    .getByRole("list", { name: "Izin langsung" })
    .getByRole("listitem");
  await expect(butir).toHaveCount(1);
  await expect(butir).toContainText(IZIN_UJI);
  await expect(butir).toContainText("Langsung");
  await expect(butir).toContainText("Aktif");
  await expect(butir).toContainText("Berlaku sampai");
  await expect(butir).toContainText("e2e: proyek indikator kinerja");
  await expect(butir).toContainText("oleh Super Admin");

  // Sumber lain: izin dari peran terdaftar terpisah.
  await dialog.getByText(/^Izin dari peran \(/).click();
  await expect(
    dialog.getByRole("list", { name: "Izin dari peran" }),
  ).toContainText("harga.input");

  const api = await goApi(
    "GET",
    `users/${u.id}/permissions`,
    await tokenAdmin(),
  );
  expect(api.body.data.efektif).toContain(IZIN_UJI);

  await dialog.getByRole("button", { name: `Cabut izin ${IZIN_UJI}` }).click();
  await expect(
    dialog.getByText("Belum ada izin langsung untuk pengguna ini."),
  ).toBeVisible();
  const sesudah = await goApi(
    "GET",
    `users/${u.id}/permissions`,
    await tokenAdmin(),
  );
  expect(sesudah.body.data.efektif).not.toContain(IZIN_UJI);
});

// ── Browser: matriks akses ───────────────────────────────────────────────

async function bukaMatriks(page: Page) {
  await buka(page, "/pengguna");
  await page.getByRole("tab", { name: "Matriks akses" }).click();
  const tabel = page.getByRole("table", {
    name: "Matriks akses peran dan izin",
  });
  await expect(tabel).toBeVisible();
  return {
    tabel,
    sel: tabel.getByRole("button", {
      name: `${IZIN_UJI} untuk ${LABEL_PERAN_UJI}`,
    }),
  };
}

test("admin: sel matriks menjadi draf (ditandai, bisa dibatalkan), disimpan per peran, dan mengubah akses pengguna peran itu; bawaan dipulihkan", async ({
  page,
  galat,
}) => {
  izinkan401(galat);
  const u = await penggunaSiap();
  let sesiPengguna = await masukApi(u.email, u.sandi);
  try {
    await masukSebagai(page, "superadmin");
    const { tabel, sel } = await bukaMatriks(page);
    await expect(sel).toHaveAttribute("aria-pressed", "false");

    // Draf: ditandai belum disimpan dan belum berlaku; dapat dibatalkan.
    await sel.click();
    await expect(sel).toHaveAttribute("aria-pressed", "true");
    await expect(page.getByText("1 peran belum disimpan")).toBeVisible();
    await expect(
      tabel.getByRole("button", { name: `Simpan matriks ${LABEL_PERAN_UJI}` }),
    ).toBeVisible();
    expect((await kpi(sesiPengguna.access)).status, "draf belum berlaku").toBe(
      403,
    );
    await tabel
      .getByRole("button", { name: `Batalkan perubahan ${LABEL_PERAN_UJI}` })
      .click();
    await expect(sel).toHaveAttribute("aria-pressed", "false");
    await expect(page.getByText("peran belum disimpan")).toHaveCount(0);

    // Draf bertahan saat berpindah tab di halaman.
    await sel.click();
    await page.getByRole("tab", { name: "Daftar pengguna" }).click();
    await page.getByRole("tab", { name: "Matriks akses" }).click();
    await expect(sel).toHaveAttribute("aria-pressed", "true");
    await expect(page.getByText("1 peran belum disimpan")).toBeVisible();

    // Simpan: berlaku bagi pengguna peran itu (token basi → refresh → 200).
    await tabel
      .getByRole("button", { name: `Simpan matriks ${LABEL_PERAN_UJI}` })
      .click();
    await expect(
      page.getByText(`Matriks ${LABEL_PERAN_UJI} disimpan`),
    ).toBeVisible();
    await expect(page.getByText("peran belum disimpan")).toHaveCount(0);
    await expect(sel).toHaveAttribute("aria-pressed", "true");
    expect((await kpi(sesiPengguna.access)).status, "token lama basi").toBe(
      401,
    );
    sesiPengguna = await segarkan(sesiPengguna);
    expect(
      (await kpi(sesiPengguna.access)).status,
      "izin dari matriks berlaku",
    ).toBe(200);

    // Pulihkan bawaan: menjadi draf, lalu disimpan.
    const pulihkan = tabel.getByRole("button", {
      name: `Pulihkan bawaan ${LABEL_PERAN_UJI}`,
    });
    await pulihkan.click();
    await expect(sel).toHaveAttribute("aria-pressed", "false");
    await tabel
      .getByRole("button", { name: `Simpan matriks ${LABEL_PERAN_UJI}` })
      .click();
    await expect(
      page.getByText(`Matriks ${LABEL_PERAN_UJI} disimpan`).last(),
    ).toBeVisible();
    await expect(pulihkan, "sudah sama dengan bawaan").toHaveCount(0);
    expect((await kpi(sesiPengguna.access)).status).toBe(401);
    sesiPengguna = await segarkan(sesiPengguna);
    expect((await kpi(sesiPengguna.access)).status).toBe(403);
  } finally {
    await pulihkanMatriks();
  }
});

test("admin: menyimpan matriks yang sudah diubah admin lain ditolak (konflik versi), draf dibuang, data dimuat ulang", async ({
  page,
  galat,
}) => {
  galat.izinkan(/^409 PUT \/api\/rbac\/roles\//);
  galat.izinkan(/Failed to load resource.*status of 409/);
  try {
    await masukSebagai(page, "superadmin");
    const { tabel, sel } = await bukaMatriks(page);
    await sel.click();
    await expect(page.getByText("1 peran belum disimpan")).toBeVisible();

    // Admin lain menyimpan lebih dulu (versi naik) dengan izin berbeda.
    const admin = await tokenAdmin();
    const agregator = (await bacaPeran(admin)).find(
      (p) => p.id === ID_PERAN_UJI,
    )!;
    const lain = await goApi(
      "PUT",
      `roles/${ID_PERAN_UJI}/permissions`,
      admin,
      {
        permissions: [...agregator.permissions, "audit.lihat"],
        versi: agregator.versi,
      },
    );
    expect(lain.status, JSON.stringify(lain.body)).toBe(200);

    await tabel
      .getByRole("button", { name: `Simpan matriks ${LABEL_PERAN_UJI}` })
      .click();
    await expect(
      page.getByText(`Matriks ${LABEL_PERAN_UJI} sudah diubah admin lain`),
    ).toBeVisible();

    // Draf dibuang; yang tampil adalah keadaan terbaru (izin admin lain), bukan timpaan.
    await expect(page.getByText("peran belum disimpan")).toHaveCount(0);
    await expect(sel).toHaveAttribute("aria-pressed", "false");
    await expect(
      tabel.getByRole("button", {
        name: `audit.lihat untuk ${LABEL_PERAN_UJI}`,
      }),
    ).toHaveAttribute("aria-pressed", "true");
    const sesudah = (await bacaPeran(admin)).find(
      (p) => p.id === ID_PERAN_UJI,
    )!;
    expect(sesudah.permissions).not.toContain(IZIN_UJI);
    expect(sesudah.permissions).toContain("audit.lihat");
  } finally {
    await pulihkanMatriks();
  }
});
