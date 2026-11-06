/**
 * Daftar pengguna yang dijalankan server (/pengguna): pencarian, filter, urutan, dan paginasi dikerjakan
 * `GET /v1/users` (lewat BFF `/api/pengguna`), keadaannya hidup di URL, "pilih semua" hanya untuk halaman yang
 * tampil, reset MFA oleh admin, dan riwayat peran per pengguna di dialog "Ubah pengguna".
 *
 * Aturan suite ini:
 *  - Urutan dan isi baris yang tampil dibandingkan dengan jawaban API untuk kueri yang sama, bukan dengan
 *    urutan yang dihitung ulang di tes, supaya hasilnya tidak bergantung pada collation basis data.
 *  - Dua pengguna uji sementara (`buatPenggunaUji`, OPS_AGREGATOR) dibuat di beforeAll dan dihapus di afterAll:
 *    satu dengan MFA aktif (diaktifkan lewat API memakai `kodeTotp`) untuk reset MFA, satu untuk riwayat peran.
 *    Akun seed hanya dibaca; tidak ada yang diubah selain pengguna sementara itu.
 *  - Keadaan tabel hanya ada di URL, jadi tidak ada yang perlu dipulihkan antar tes.
 *  - Kueri yang sama dalam 20 detik dijawab dari cache React Query tanpa permintaan baru: pernyataan tentang
 *    permintaan hanya dibuat untuk kueri pertama di halaman yang baru dimuat.
 */
import { request, type APIRequestContext, type Page } from "@playwright/test";
import { LAPIS_META, ROLES, type Role } from "../src/lib/rbac/roles";
import {
  buatPenggunaUji,
  hapusPenggunaUji,
  kodeTotp,
  sandiAcak,
  type PenggunaUji,
} from "./auth-helpers";
import {
  apiSebagai,
  BASE_URL,
  buka,
  expect,
  masukSebagai,
  test,
  tungguTenang,
} from "./fixtures";
import { cariPengguna, kotakCariPengguna } from "./pengguna-helpers";

test.describe.configure({ mode: "serial", timeout: 120_000 });

interface BarisApi {
  id: string;
  nama: string;
  peran: Role;
  mfa_aktif: boolean;
}

let admin: APIRequestContext;
const dibuat: PenggunaUji[] = [];
let uPeran: PenggunaUji;
let uMfa: PenggunaUji;
/** Access token sesi uMfa yang dibuat saat MFA diaktifkan: harus mati seketika setelah reset. */
let aksesMfa: string;

/** Pengguna uji yang MFA-nya aktif (kewajiban ganti kata sandi diselesaikan lebih dulu, lalu setup + verifikasi). */
async function penggunaBerMfa(): Promise<{
  pengguna: PenggunaUji;
  akses: string;
}> {
  const pengguna = await buatPenggunaUji();
  dibuat.push(pengguna);
  const masuk = await apiSebagai(pengguna.email, pengguna.sandi);
  let tokenBaru: string;
  try {
    const baru = sandiAcak();
    const ganti = await masuk.post("auth/change-password", {
      data: { old_password: pengguna.sandi, new_password: baru },
    });
    expect(ganti.status(), "ganti kata sandi wajib").toBe(200);
    tokenBaru = ((await ganti.json()) as { data: { access_token: string } })
      .data.access_token;
    pengguna.sandi = baru;
  } finally {
    await masuk.dispose();
  }

  const sesi = await request.newContext({
    baseURL: `${BASE_URL}/api/v1/`,
    extraHTTPHeaders: {
      Authorization: `Bearer ${tokenBaru}`,
      Origin: BASE_URL,
    },
  });
  try {
    const setup = await sesi.post("auth/mfa/setup");
    expect(setup.status(), "setup MFA").toBe(200);
    const { secret } = ((await setup.json()) as { data: { secret: string } })
      .data;
    const verifikasi = await sesi.post("auth/mfa/verify", {
      data: { code: kodeTotp(secret) },
    });
    expect(verifikasi.status(), "verifikasi MFA").toBe(200);
    const { access_token } = (
      (await verifikasi.json()) as { data: { access_token: string } }
    ).data;
    return { pengguna, akses: access_token };
  } finally {
    await sesi.dispose();
  }
}

test.beforeAll(async () => {
  admin = await apiSebagai("superadmin");
  uPeran = await buatPenggunaUji();
  dibuat.push(uPeran);
  const mfa = await penggunaBerMfa();
  uMfa = mfa.pengguna;
  aksesMfa = mfa.akses;
});

test.afterAll(async () => {
  const gagal: string[] = [];
  for (const pengguna of dibuat) {
    await hapusPenggunaUji(pengguna).catch((err: Error) =>
      gagal.push(`${pengguna.email}: ${err.message}`),
    );
  }
  await admin?.dispose();
  expect(gagal, "pengguna uji yang gagal dihapus").toEqual([]);
});

// ── Pembantu ─────────────────────────────────────────────────────────────

async function daftarApi(
  query: string,
): Promise<{ items: BarisApi[]; total: number }> {
  const res = await admin.get(`users?${query}`);
  expect(res.status(), `GET users?${query}`).toBe(200);
  return ((await res.json()) as { data: { items: BarisApi[]; total: number } })
    .data;
}

const barisPengguna = (page: Page, email: string) =>
  page.getByRole("row").filter({ hasText: email });
/** Nama pada kolom pertama tiap baris data (urutan tampil). */
const namaTampil = (page: Page) =>
  page.locator("tbody tr td:nth-child(2) p:first-child").allInnerTexts();
const peranTampil = (page: Page) =>
  page.locator("tbody tr td:nth-child(3) p:first-child").allInnerTexts();

/** Jumlah seluruh baris dari ringkasan "1–25 dari 27 pengguna". */
async function totalTampil(page: Page): Promise<number> {
  const teks = await page.getByText(/dari [\d.]+ pengguna/).innerText();
  return Number(/dari ([\d.]+) pengguna/.exec(teks)![1].replace(/\./g, ""));
}

const paramUrl = (page: Page) =>
  Object.fromEntries(new URL(page.url()).searchParams);

async function pilihFilter(page: Page, nama: string, opsi: string) {
  await page.getByRole("combobox", { name: nama }).click();
  await page.getByRole("option", { name: opsi, exact: true }).click();
}

const jawabanDaftar = (page: Page, cocok: (u: URL) => boolean) =>
  page.waitForResponse((r) => {
    const u = new URL(r.url());
    return (
      r.request().method() === "GET" &&
      u.pathname === "/api/pengguna" &&
      cocok(u)
    );
  });

async function statusMe(access: string): Promise<number> {
  const res = await fetch(`${BASE_URL}/api/v1/auth/me`, {
    headers: { Authorization: `Bearer ${access}`, Origin: BASE_URL },
  });
  return res.status;
}

async function bukaTabel(page: Page, path = "/pengguna") {
  await masukSebagai(page, "superadmin");
  await buka(page, path);
  await tungguTenang(page);
}

// ── Pencarian ────────────────────────────────────────────────────────────

test("pencarian dijalankan server: satu permintaan sesudah jeda ketik, hasil menyempit, URL ikut", async ({
  page,
}) => {
  await bukaTabel(page);
  const awal = await totalTampil(page);
  expect(awal).toBeGreaterThan(2);

  const dikirim: string[] = [];
  page.on("request", (r) => {
    const u = new URL(r.url());
    if (
      r.method() === "GET" &&
      u.pathname === "/api/pengguna" &&
      u.searchParams.has("search")
    )
      dikirim.push(u.searchParams.get("search")!);
  });
  const jawab = jawabanDaftar(
    page,
    (u) => u.searchParams.get("search") === uPeran.email,
  );
  await kotakCariPengguna(page).pressSequentially(uPeran.email, { delay: 25 });
  const res = await jawab;

  expect(res.status()).toBe(200);
  const kueri = new URL(res.url()).searchParams;
  expect(kueri.get("page")).toBe("1");
  expect(kueri.get("limit")).toBe("25");
  await expect(barisPengguna(page, uPeran.email)).toHaveCount(1);
  await expect(page.getByText(/dari 1 pengguna/)).toBeVisible();
  expect(await namaTampil(page)).toHaveLength(1);
  expect(paramUrl(page).search).toBe(uPeran.email);
  expect(
    dikirim,
    "jeda ketik: hanya kata kunci utuh yang dikirim, bukan tiap huruf",
  ).toEqual([uPeran.email]);

  // Mengosongkan kotak mengembalikan seluruh daftar dan membuang parameter dari URL.
  await cariPengguna(page, "");
  await expect.poll(() => totalTampil(page)).toBe(awal);
  expect(paramUrl(page).search).toBeUndefined();

  // Tanpa kecocokan: keadaan kosong dan total 0.
  await cariPengguna(page, "zzz-tidak-ada-orang-ini");
  await expect(page.getByText("Tidak ada pengguna yang cocok")).toBeVisible();
  await expect(page.getByText(/0–0 dari 0 pengguna/)).toBeVisible();
});

// ── Urutan ───────────────────────────────────────────────────────────────

test("kepala kolom mengurutkan di server: naik, turun, lalu kembali ke urutan bawaan", async ({
  page,
}) => {
  await bukaTabel(page, "/pengguna?limit=100");
  const namaApi = async (query: string) =>
    (await daftarApi(`${query}&limit=100`)).items.map((i) => i.nama);

  const bawaan = await namaApi("page=1");
  await expect.poll(() => namaTampil(page)).toEqual(bawaan);

  const kepala = page.getByRole("columnheader", { name: "Nama", exact: true });
  await expect(kepala).toHaveAttribute("aria-sort", "none");

  // Klik 1: naik.
  const naik = page.waitForRequest((r) => {
    const q = new URL(r.url()).searchParams;
    return (
      new URL(r.url()).pathname === "/api/pengguna" &&
      q.get("sort") === "nama" &&
      q.get("arah") === "asc"
    );
  });
  await kepala.getByRole("button").click();
  await naik;
  await expect(kepala).toHaveAttribute("aria-sort", "ascending");
  expect(paramUrl(page)).toMatchObject({ sort: "nama", arah: "asc" });
  const namaNaik = await namaApi("sort=nama&arah=asc");
  await expect.poll(() => namaTampil(page)).toEqual(namaNaik);
  expect(namaNaik, "urutan berubah dari bawaan").not.toEqual(bawaan);

  // Klik 2: turun. Urutan total (id sebagai tiebreaker) membuat turun = kebalikan persis dari naik.
  await kepala.getByRole("button").click();
  await expect(kepala).toHaveAttribute("aria-sort", "descending");
  expect(paramUrl(page)).toMatchObject({ sort: "nama", arah: "desc" });
  const turun = await daftarApi("sort=nama&arah=desc&limit=100");
  await expect
    .poll(() => namaTampil(page))
    .toEqual(turun.items.map((i) => i.nama));
  if (turun.total <= 100) {
    const naikIds = (await daftarApi("sort=nama&arah=asc&limit=100")).items.map(
      (i) => i.id,
    );
    expect(turun.items.map((i) => i.id)).toEqual([...naikIds].reverse());
  }

  // Klik 3: kembali ke bawaan (terbaru dibuat dulu), tanpa parameter urut di URL.
  await kepala.getByRole("button").click();
  await expect(kepala).toHaveAttribute("aria-sort", "none");
  expect(paramUrl(page).sort).toBeUndefined();
  await expect.poll(() => namaTampil(page)).toEqual(bawaan);

  // Kolom lain: urutan peran mengikuti enum, bukan abjad label.
  const kepalaPeran = page.getByRole("columnheader", {
    name: "Peran",
    exact: true,
  });
  await kepalaPeran.getByRole("button").click();
  await expect(kepalaPeran).toHaveAttribute("aria-sort", "ascending");
  const peranNaik = (
    await daftarApi("sort=peran&arah=asc&limit=100")
  ).items.map((i) => ROLES[i.peran].label);
  await expect.poll(() => peranTampil(page)).toEqual(peranNaik);
});

// ── Filter ───────────────────────────────────────────────────────────────

test("filter peran dan MFA dijalankan server dan bisa direset", async ({
  page,
}) => {
  await bukaTabel(page);
  const semua = await totalTampil(page);

  const dengan = jawabanDaftar(
    page,
    (u) => u.searchParams.get("peran") === "OPS_AGREGATOR",
  );
  await pilihFilter(page, "Filter peran", ROLES.OPS_AGREGATOR.label);
  const kueri = new URL((await dengan).url()).searchParams;
  expect(kueri.get("page")).toBe("1");
  expect(paramUrl(page).peran).toBe("OPS_AGREGATOR");

  const agregator = await daftarApi("peran=OPS_AGREGATOR&limit=100");
  await expect.poll(() => totalTampil(page)).toBe(agregator.total);
  await expect
    .poll(() => namaTampil(page))
    .toEqual(agregator.items.map((i) => i.nama));
  expect(new Set(await peranTampil(page))).toEqual(
    new Set([ROLES.OPS_AGREGATOR.label]),
  );

  // Gabungan dengan MFA aktif: pengguna uji ber-MFA ada, yang tanpa MFA tidak.
  await pilihFilter(page, "Filter mfa", "MFA aktif");
  await expect.poll(() => paramUrl(page).mfa).toBe("true");
  await expect(barisPengguna(page, uMfa.email)).toHaveCount(1);
  await expect(barisPengguna(page, uPeran.email)).toHaveCount(0);
  const gabungan = await daftarApi("peran=OPS_AGREGATOR&mfa=true&limit=100");
  await expect.poll(() => totalTampil(page)).toBe(gabungan.total);
  await expect(page.getByLabel("MFA aktif")).toHaveCount(gabungan.items.length);

  // Reset membuang kedua filter, kata kunci dan urutan tidak terkena.
  await page.getByRole("button", { name: /^Reset filter/ }).click();
  await expect.poll(() => paramUrl(page).peran).toBeUndefined();
  expect(paramUrl(page).mfa).toBeUndefined();
  await expect.poll(() => totalTampil(page)).toBe(semua);
  await expect(page.getByRole("button", { name: /^Reset filter/ })).toHaveCount(
    0,
  );
});

// ── Paginasi dan pilih semua ─────────────────────────────────────────────

test("paginasi: halaman berikutnya dan sebelumnya dengan total benar, halaman terlewat dikoreksi", async ({
  page,
}) => {
  await bukaTabel(page, "/pengguna?limit=10");
  const total = await totalTampil(page);
  expect(total, "butuh lebih dari 10 pengguna di data demo").toBeGreaterThan(
    10,
  );
  const halaman = Math.ceil(total / 10);
  const sebut = (n: number) => n.toLocaleString("id-ID");

  await expect(
    page.getByText(`1–10 dari ${sebut(total)} pengguna`),
  ).toBeVisible();
  await expect(page.getByText(`1 / ${halaman}`, { exact: true })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Halaman sebelumnya" }),
  ).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "Halaman berikutnya" }),
  ).toBeEnabled();
  const halaman1 = await namaTampil(page);
  expect(halaman1).toHaveLength(10);
  expect(halaman1).toEqual(
    (await daftarApi("page=1&limit=10")).items.map((i) => i.nama),
  );

  // Berikutnya: permintaan membawa page=2, URL ikut, baris berganti.
  const dua = jawabanDaftar(
    page,
    (u) =>
      u.searchParams.get("page") === "2" &&
      u.searchParams.get("limit") === "10",
  );
  await page.getByRole("button", { name: "Halaman berikutnya" }).click();
  expect((await dua).status()).toBe(200);
  expect(paramUrl(page)).toMatchObject({ page: "2", limit: "10" });
  const halaman2 = (await daftarApi("page=2&limit=10")).items.map(
    (i) => i.nama,
  );
  await expect.poll(() => namaTampil(page)).toEqual(halaman2);
  expect(halaman2).not.toEqual(halaman1);
  await expect(
    page.getByText(`11–${Math.min(20, total)} dari ${sebut(total)} pengguna`),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Halaman sebelumnya" }),
  ).toBeEnabled();

  // Sebelumnya: kembali ke halaman 1, parameter page hilang dari URL.
  await page.getByRole("button", { name: "Halaman sebelumnya" }).click();
  await expect.poll(() => namaTampil(page)).toEqual(halaman1);
  expect(paramUrl(page).page).toBeUndefined();

  // Halaman terakhir: tombol berikutnya mati, rentang baris benar.
  await buka(page, `/pengguna?limit=10&page=${halaman}`);
  await tungguTenang(page);
  await expect(
    page.getByRole("button", { name: "Halaman berikutnya" }),
  ).toBeDisabled();
  await expect(
    page.getByText(
      `${(halaman - 1) * 10 + 1}–${total} dari ${sebut(total)} pengguna`,
    ),
  ).toBeVisible();

  // Halaman di luar jangkauan (URL lama): dikoreksi ke halaman terakhir tanpa galat.
  await buka(page, `/pengguna?limit=10&page=${halaman + 5}`);
  await expect.poll(() => paramUrl(page).page).toBe(String(halaman));
  await expect
    .poll(() => namaTampil(page))
    .toEqual(
      (await daftarApi(`page=${halaman}&limit=10`)).items.map((i) => i.nama),
    );
});

test("pilih semua hanya untuk halaman yang tampil dan pilihan dibersihkan saat halaman berganti", async ({
  page,
}) => {
  await bukaTabel(page, "/pengguna?limit=10");
  const total = await totalTampil(page);
  expect(total).toBeGreaterThan(10);

  await page
    .getByRole("checkbox", { name: "Pilih semua di halaman ini" })
    .click();
  await expect(
    page.getByText(
      `Seluruh 10 pengguna di halaman ini terpilih. ${total - 10} pengguna di halaman lain tidak ikut terpilih.`,
    ),
  ).toBeVisible();
  await expect(page.getByText("10 terpilih")).toBeVisible();
  await expect(page.getByRole("toolbar")).toBeVisible();

  await page.getByRole("button", { name: "Halaman berikutnya" }).click();
  await expect.poll(() => paramUrl(page).page).toBe("2");
  await expect(page.getByText(/di halaman ini terpilih/)).toHaveCount(0);
  await expect(page.getByText(/\d+ terpilih/)).toHaveCount(0);
  await expect(page.getByRole("toolbar")).toHaveCount(0);
  for (const kotak of await page
    .getByRole("checkbox", { name: "Pilih baris" })
    .all())
    await expect(kotak).not.toBeChecked();

  // Mengubah filter atau pencarian juga membersihkan pilihan.
  await page.getByRole("checkbox", { name: "Pilih baris" }).first().click();
  await expect(page.getByText("1 terpilih")).toBeVisible();
  await cariPengguna(page, "gardainflasi");
  await expect(page.getByText(/\d+ terpilih/)).toHaveCount(0);
});

// ── Keadaan di URL ───────────────────────────────────────────────────────

test("keadaan tabel hidup di URL: bertahan saat dimuat ulang dan dipulihkan tombol kembali", async ({
  page,
}) => {
  await bukaTabel(page);
  const lapisOperasional = LAPIS_META.OPERASIONAL.label.split(" — ")[0];

  await cariPengguna(page, "gardainflasi");
  await pilihFilter(page, "Filter lapis", lapisOperasional);
  await expect.poll(() => paramUrl(page).lapis).toBe("OPERASIONAL");
  await page
    .getByRole("columnheader", { name: "Nama", exact: true })
    .getByRole("button")
    .click();
  await expect.poll(() => paramUrl(page).sort).toBe("nama");
  await page.getByRole("combobox").filter({ hasText: "/ halaman" }).click();
  await page.getByRole("option", { name: "10 / halaman", exact: true }).click();
  await expect.poll(() => paramUrl(page).limit).toBe("10");
  await page.getByRole("button", { name: "Halaman berikutnya" }).click();
  await expect.poll(() => paramUrl(page).page).toBe("2");

  const diharapkan = {
    search: "gardainflasi",
    lapis: "OPERASIONAL",
    sort: "nama",
    arah: "asc",
    page: "2",
    limit: "10",
  };
  expect(paramUrl(page)).toEqual(diharapkan);
  const halaman2 = (
    await daftarApi(
      "search=gardainflasi&lapis=OPERASIONAL&sort=nama&arah=asc&page=2&limit=10",
    )
  ).items.map((i) => i.nama);
  await expect.poll(() => namaTampil(page)).toEqual(halaman2);

  // Muat ulang: semua kontrol dan baris pulih dari URL.
  await page.reload({ waitUntil: "domcontentloaded" });
  await tungguTenang(page);
  expect(paramUrl(page)).toEqual(diharapkan);
  await expect(kotakCariPengguna(page)).toHaveValue("gardainflasi");
  await expect(
    page.getByRole("combobox", { name: "Filter lapis" }),
  ).toContainText(lapisOperasional);
  await expect(
    page.getByRole("columnheader", { name: "Nama", exact: true }),
  ).toHaveAttribute("aria-sort", "ascending");
  await expect(
    page.getByRole("combobox").filter({ hasText: "/ halaman" }),
  ).toContainText("10 / halaman");
  await expect.poll(() => namaTampil(page)).toEqual(halaman2);

  // Kembali: halaman 1 dengan keadaan lain utuh.
  await page.goBack();
  await expect.poll(() => paramUrl(page).page).toBeUndefined();
  expect(paramUrl(page)).toMatchObject({
    search: "gardainflasi",
    lapis: "OPERASIONAL",
    sort: "nama",
    arah: "asc",
    limit: "10",
  });
  const halaman1 = (
    await daftarApi(
      "search=gardainflasi&lapis=OPERASIONAL&sort=nama&arah=asc&page=1&limit=10",
    )
  ).items.map((i) => i.nama);
  await expect.poll(() => namaTampil(page)).toEqual(halaman1);
});

// ── Reset MFA ────────────────────────────────────────────────────────────

test("reset MFA: dialog menjelaskan akibatnya, MFA mati, sesi lama ditolak, tercatat di audit", async ({
  page,
}) => {
  expect(await statusMe(aksesMfa), "sesi uji hidup sebelum reset").toBe(200);
  await bukaTabel(page);
  await cariPengguna(page, uMfa.email);
  const baris = barisPengguna(page, uMfa.email);
  await expect(baris).toHaveCount(1);
  await expect(baris.getByLabel("MFA aktif")).toBeVisible();

  // Pengguna tanpa MFA tidak ditawari Reset MFA.
  await cariPengguna(page, uPeran.email);
  await barisPengguna(page, uPeran.email)
    .getByRole("button", { name: /^Aksi untuk Pengguna Uji/ })
    .click();
  await expect(page.getByRole("menuitem", { name: "Ubah data" })).toBeVisible();
  await expect(page.getByRole("menuitem", { name: "Reset MFA" })).toHaveCount(
    0,
  );
  await page.keyboard.press("Escape");
  await cariPengguna(page, uMfa.email);

  // Batal: tidak ada yang berubah.
  await baris.getByRole("button", { name: /^Aksi untuk Pengguna Uji/ }).click();
  await page.getByRole("menuitem", { name: "Reset MFA" }).click();
  const dialog = page.getByRole("dialog");
  await expect(
    dialog.getByRole("heading", { name: "Reset MFA" }),
  ).toBeVisible();
  await expect(dialog).toContainText("Seluruh sesi aktif");
  await expect(dialog).toContainText("kode cadangan dihapus");
  await expect(dialog).toContainText("audit trail");
  await dialog.getByRole("button", { name: "Batal" }).click();
  await expect(dialog).toBeHidden();
  expect(await statusMe(aksesMfa), "batal tidak mencabut sesi").toBe(200);
  await expect(baris.getByLabel("MFA aktif")).toBeVisible();

  // Jalankan.
  await baris.getByRole("button", { name: /^Aksi untuk Pengguna Uji/ }).click();
  await page.getByRole("menuitem", { name: "Reset MFA" }).click();
  const reset = page.waitForResponse(
    (r) =>
      r.request().method() === "POST" &&
      new URL(r.url()).pathname === `/api/pengguna/${uMfa.id}/reset-mfa`,
  );
  await dialog.getByRole("button", { name: "Reset MFA", exact: true }).click();
  const res = await reset;
  expect(res.status()).toBe(200);
  const hasilReset = (await res.json()) as {
    id: string;
    mfa_aktif_sebelumnya: boolean;
    sesi_dicabut: number;
  };
  expect(hasilReset).toMatchObject({ id: uMfa.id, mfa_aktif_sebelumnya: true });
  expect(
    hasilReset.sesi_dicabut,
    "sesi aktif pengguna dicabut",
  ).toBeGreaterThanOrEqual(1);

  await expect(dialog).toBeHidden();
  await expect(page.getByText("MFA direset", { exact: true })).toBeVisible();
  // Status pada tabel diperbarui sesudah daftar dimuat ulang.
  await expect(baris.getByLabel("MFA aktif")).toHaveCount(0);
  await expect(baris.getByText("Belum", { exact: true })).toBeVisible();
  await baris.getByRole("button", { name: /^Aksi untuk Pengguna Uji/ }).click();
  await expect(page.getByRole("menuitem", { name: "Ubah data" })).toBeVisible();
  await expect(page.getByRole("menuitem", { name: "Reset MFA" })).toHaveCount(
    0,
  );
  await page.keyboard.press("Escape");

  // Sesi lama pengguna ditolak seketika; API dan audit mencatatnya.
  expect(await statusMe(aksesMfa), "access token lama ditolak seketika").toBe(
    401,
  );
  const detail = (await (await admin.get(`users/${uMfa.id}`)).json()) as {
    data: { mfa_aktif: boolean };
  };
  expect(detail.data.mfa_aktif).toBe(false);
  const audit = await admin.get(`audit?aksi=RESET_MFA&entitas_id=${uMfa.id}`);
  expect(audit.status()).toBe(200);
  const entri = (
    (await audit.json()) as {
      data: { items: { aksi: string; hasil: string; aktor_nama?: string }[] };
    }
  ).data.items;
  expect(entri).toHaveLength(1);
  expect(entri[0]).toMatchObject({
    aksi: "RESET_MFA",
    hasil: "BERHASIL",
    aktor_nama: "Super Admin",
  });

  // Mengulang lewat API itu sendiri idempoten.
  const ulang = await admin.post(`users/${uMfa.id}/reset-mfa`);
  expect(ulang.status()).toBe(200);
  expect(
    ((await ulang.json()) as { data: { mfa_aktif_sebelumnya: boolean } }).data
      .mfa_aktif_sebelumnya,
  ).toBe(false);
});

// ── Riwayat peran ────────────────────────────────────────────────────────

test("riwayat peran: dua perubahan peran tampil di dialog dengan pelaku, dan tautan membuka audit tersaring", async ({
  page,
}) => {
  await bukaTabel(page);
  await cariPengguna(page, uPeran.email);
  const baris = barisPengguna(page, uPeran.email);
  await expect(baris).toHaveCount(1);
  const menuAksi = () =>
    baris.getByRole("button", { name: /^Aksi untuk Pengguna Uji/ });
  const tukarPeran = () =>
    page.waitForResponse(
      (r) =>
        r.request().method() === "PUT" &&
        new URL(r.url()).pathname === `/api/pengguna/${uPeran.id}/roles`,
    );

  // Perubahan 1: OPS_AGREGATOR -> ADMIN_SISTEM.
  await menuAksi().click();
  const satu = tukarPeran();
  await page.getByRole("menuitem", { name: "Jadikan admin sistem" }).click();
  expect((await satu).status()).toBe(200);
  await expect(
    baris.getByText(ROLES.ADMIN_SISTEM.label, { exact: true }),
  ).toBeVisible();

  // Perubahan 2: ADMIN_SISTEM -> OPS_MAHASISWA.
  await menuAksi().click();
  const dua = tukarPeran();
  await page.getByRole("menuitem", { name: "Cabut admin sistem" }).click();
  expect((await dua).status()).toBe(200);
  await expect(
    baris.getByText(ROLES.OPS_MAHASISWA.label, { exact: true }),
  ).toBeVisible();

  // Dialog Ubah pengguna memuat riwayat dari server.
  await menuAksi().click();
  const riwayat = page.waitForResponse(
    (r) =>
      r.request().method() === "GET" &&
      new URL(r.url()).pathname === `/api/pengguna/${uPeran.id}/riwayat-peran`,
  );
  await page.getByRole("menuitem", { name: "Ubah data" }).click();
  expect((await riwayat).status()).toBe(200);

  const dialog = page.getByRole("dialog");
  await expect(
    dialog.getByRole("heading", { name: "Riwayat peran" }),
  ).toBeVisible();
  const daftar = dialog.getByRole("list", { name: "Riwayat peran" });
  const item = daftar.getByRole("listitem");
  await expect(item).toHaveCount(3);
  // Terbaru dulu, masing-masing dengan pelaku.
  await expect(item.nth(0)).toContainText("Peran diganti");
  await expect(item.nth(0)).toContainText(ROLES.ADMIN_SISTEM.label);
  await expect(item.nth(0)).toContainText(ROLES.OPS_MAHASISWA.label);
  await expect(item.nth(0)).toContainText("oleh Super Admin");
  await expect(item.nth(1)).toContainText("Peran diganti");
  await expect(item.nth(1)).toContainText(ROLES.OPS_AGREGATOR.label);
  await expect(item.nth(1)).toContainText(ROLES.ADMIN_SISTEM.label);
  await expect(item.nth(1)).toContainText("oleh Super Admin");
  await expect(item.nth(2)).toContainText("Akun dibuat");
  await expect(item.nth(2)).toContainText(ROLES.OPS_AGREGATOR.label);
  await expect(item.nth(2)).toContainText("oleh Super Admin");

  // Sama dengan yang dicatat server.
  const server = (await (
    await admin.get(`users/${uPeran.id}/riwayat-peran`)
  ).json()) as {
    data: {
      items: {
        aksi: string;
        dari: string | null;
        ke: string;
        aktor: { nama: string } | null;
      }[];
    };
  };
  expect(server.data.items.map((e) => [e.aksi, e.dari, e.ke])).toEqual([
    ["ASSIGN_ROLE", "ADMIN_SISTEM", "OPS_MAHASISWA"],
    ["ASSIGN_ROLE", "OPS_AGREGATOR", "ADMIN_SISTEM"],
    ["CREATE_USER", null, "OPS_AGREGATOR"],
  ]);
  expect(server.data.items.every((e) => e.aktor?.nama === "Super Admin")).toBe(
    true,
  );

  // Tautan ke audit trail yang sudah disaring pada pengguna ini.
  const tautan = dialog.getByRole("link", { name: "Lihat di audit trail" });
  await expect(tautan).toHaveAttribute(
    "href",
    `/audit?entitas=pengguna&entitas_id=${uPeran.id}`,
  );
  const permintaanAudit = page.waitForRequest((r) => {
    const u = new URL(r.url());
    return (
      u.pathname.endsWith("/v1/audit") &&
      u.searchParams.get("entitas_id") === uPeran.id
    );
  });
  await tautan.click();
  await expect(page).toHaveURL(/\/audit\?/);
  expect(
    new URL((await permintaanAudit).url()).searchParams.get("entitas"),
  ).toBe("pengguna");
  await expect(page.getByText(`Objek ${uPeran.id}`)).toBeVisible();
  await tungguTenang(page);
  const semuaBaris = page.locator("tbody tr");
  await expect(semuaBaris.filter({ hasText: "CREATE_USER" })).toHaveCount(1);
  await expect(semuaBaris.filter({ hasText: "ASSIGN_ROLE" })).toHaveCount(2);
  expect(
    await semuaBaris.count(),
    "audit tersaring: semua baris milik pengguna ini",
  ).toBe(await semuaBaris.filter({ hasText: uPeran.id }).count());
});
