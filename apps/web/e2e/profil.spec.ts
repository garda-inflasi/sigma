/**
 * Halaman profil terpadu (/profil, slice 20): navigasi menu akun dan tab, edit profil (nama, telepon), ganti kata
 * sandi dengan meter kekuatan, aktivasi MFA dengan kode cadangan, serta perangkat dan sesi aktif. Rute lama
 * /keamanan dialihkan ke /profil?tab=keamanan; /ganti-password dan /mfa tetap ada sebagai tujuan alur wajib.
 *
 * Aturan suite ini:
 *  - Semua yang mengubah profil, kata sandi, MFA, atau sesi memakai pengguna sementara `e2e.<timestamp>.<n>@gardainflasi.id`
 *    (OPS_AGREGATOR, bukan wajib MFA) yang dibuat lewat API, dibersihkan kewajiban ganti kata sandinya, dan dihapus di
 *    afterAll. Akun seed hanya dibaca (superadmin, sesi baru milik tes): nama "Super Admin" dan kata sandi demo dipakai
 *    spec lain, jadi tidak boleh berubah.
 *  - Masuk lewat API Go langsung, cookie ditanam menyerupai produksi (profil-helpers.ts `tanamSesi`); tidak berbatas
 *    laju BFF. Satu-satunya kiriman form /masuk ada di langkah terakhir tes ganti kata sandi (satu pemakaian
 *    `anggaranBff.pakai("login")`).
 *  - Validasi form dijawab klien (skema) atau server (400); tes yang memicunya mengizinkan 400 PATCH /api/auth/me dan
 *    hanya mensyaratkan pesannya tampil inline.
 *  - Tidak tercakup: akun wajib MFA yang belum setup diarahkan ke /mfa, dan tombol nonaktifkan MFA yang tersembunyi bagi
 *    akun wajib MFA. Keduanya butuh MFA_ENFORCE=true, sedangkan demo (dan seluruh suite ini) berjalan tanpa penegakan MFA;
 *    keduanya dikunci tes unit dan komponen. Alur wajib ganti kata sandi tercakup di tes terakhir dan di auth.spec.ts.
 *  - Teks dan nama aksesibel mengikuti kontrak UI 20 (docs/todos/20-profile.md); tulisan tombol yang tidak ada di kontrak
 *    dicari lewat peran dan tipe (mis. tombol kirim di dalam panel), bukan tulisannya.
 */
import { readFileSync } from "node:fs";
import type { Page } from "@playwright/test";
import { inisialNama } from "../src/lib/nama";
import { ROLES } from "../src/lib/rbac/roles";
import { kodeTotp, sandiAcak, type PenggunaUji } from "./auth-helpers";
import {
  apiSebagai,
  BASE_URL,
  buka,
  expect,
  masukSebagai,
  sesiApi,
  test,
  tungguTenang,
} from "./fixtures";
import {
  bacaKekuatanSandi,
  bacaProfilBff,
  bersihkanPengguna,
  buatPengguna,
  cookieBrowser,
  goApi,
  harapBeranda,
  izinkan401,
  izinkanHttp,
  klaimToken,
  LABEL_TAB,
  masukApi,
  masukLewatForm,
  namaAwalUji,
  panelProfil,
  penggunaSiap,
  statusMe,
  statusRefresh,
  tabProfil,
  tabTerpilih,
  tanamSesi,
  tombolKirimPanel,
  tombolMenuAkun,
  UA_ANDROID,
  UA_IPHONE,
  UA_MAC,
  URUTAN_TAB,
  type ProfilBff,
} from "./profil-helpers";

// Tes yang menunggu jendela laju BFF (login lewat form) bisa berjalan lebih dari semenit.
test.describe.configure({ timeout: 180_000 });

const dibuat: PenggunaUji[] = [];

test.afterAll(async () => {
  await bersihkanPengguna(dibuat);
});

/** Empat tautan profil di menu akun: nama, href, URL tujuan, dan tab yang harus terpilih. */
const TAUTAN_MENU = [
  {
    nama: "Profil Saya",
    href: "/profil",
    url: /\/profil$/,
    tab: LABEL_TAB.profil,
  },
  {
    nama: "Ganti Kata Sandi",
    href: "/profil?tab=ganti-password",
    url: /\/profil\?tab=ganti-password$/,
    tab: LABEL_TAB.gantiSandi,
  },
  {
    nama: "Verifikasi 2 Langkah",
    href: "/profil?tab=mfa",
    url: /\/profil\?tab=mfa$/,
    tab: LABEL_TAB.mfa,
  },
  {
    nama: "Perangkat & Sesi Aktif",
    href: "/profil?tab=keamanan",
    url: /\/profil\?tab=keamanan$/,
    tab: LABEL_TAB.sesi,
  },
] as const;

const daftarSesi = (page: Page) =>
  page.getByRole("list", { name: "Sesi aktif" }).getByRole("listitem");

// ── Navigasi: menu akun, tab, deep link ──────────────────────────────────

test("navigasi: menu akun memuat nama, peran, lembaga, lalu empat tautan profil berurutan sebelum mode tema dan Keluar", async ({
  page,
}) => {
  await masukSebagai(page, "superadmin");
  await buka(page, "/pengguna");
  await tungguTenang(page);
  const { lembaga } = await bacaProfilBff(page);

  await tombolMenuAkun(page).click();
  const menu = page.getByRole("menu");
  await expect(menu).toBeVisible();
  await expect(menu, "kepala menu: nama, peran, lembaga").toContainText(
    "Super Admin",
  );
  await expect(menu).toContainText(ROLES.ADMIN_SISTEM.label);
  await expect(menu).toContainText(lembaga);

  await expect(menu.getByRole("menuitem")).toHaveText([
    "Profil Saya",
    "Ganti Kata Sandi",
    "Verifikasi 2 Langkah",
    "Perangkat & Sesi Aktif",
    /^Mode (gelap|terang)$/,
    "Keluar",
  ]);
  for (const { nama, href } of TAUTAN_MENU) {
    await expect(
      menu.getByRole("menuitem", { name: nama, exact: true }),
      `tautan ${nama}`,
    ).toHaveAttribute("href", href);
  }
});

test("navigasi: tiap tautan menu akun membuka /profil dengan tab yang benar", async ({
  page,
}) => {
  await masukSebagai(page, "superadmin");
  await buka(page, "/pengguna");
  await tungguTenang(page);

  // Dari halaman lain ke /profil, lalu berpindah tab lewat menu, lalu kembali ke Profil Saya.
  for (const { nama, url, tab } of [...TAUTAN_MENU, TAUTAN_MENU[0]]) {
    await tombolMenuAkun(page).click();
    await page.getByRole("menuitem", { name: nama, exact: true }).click();
    await expect(page, `URL sesudah ${nama}`).toHaveURL(url);
    await expect(tabTerpilih(page), `tab sesudah ${nama}`).toHaveText(tab);
    await expect(panelProfil(page)).toHaveCount(1);
  }
});

test("navigasi: deep link ?tab= membuka tab yang benar; nilai kosong atau tak dikenal jatuh ke Edit Profil", async ({
  page,
}) => {
  await masukSebagai(page, "superadmin");

  await buka(page, "/profil");
  await tungguTenang(page);
  await expect(
    page.getByRole("heading", { name: "Profil saya", level: 1 }),
  ).toBeVisible();
  const tablist = page.getByRole("tablist", { name: "Bagian profil" });
  await expect(tablist.getByRole("tab")).toHaveText([...URUTAN_TAB]);

  const dalamTautan: [string, string][] = [
    ["/profil", LABEL_TAB.profil],
    ["/profil?tab=profil", LABEL_TAB.profil],
    ["/profil?tab=ganti-password", LABEL_TAB.gantiSandi],
    ["/profil?tab=mfa", LABEL_TAB.mfa],
    ["/profil?tab=keamanan", LABEL_TAB.sesi],
    ["/profil?tab=tidak-dikenal", LABEL_TAB.profil],
    ["/profil?tab=", LABEL_TAB.profil],
  ];
  for (const [path, label] of dalamTautan) {
    await buka(page, path);
    await tungguTenang(page);
    await expect(tabTerpilih(page), `tab untuk ${path}`).toHaveText(label);
    // Tepat satu panel, dan namanya adalah tab yang terpilih (aria-labelledby).
    await expect(panelProfil(page)).toHaveCount(1);
    await expect(panelProfil(page), `panel untuk ${path}`).toHaveAccessibleName(
      label,
    );
  }
});

test("navigasi: klik tab mengganti URL lewat replaceState tanpa menambah riwayat; panah, Home, dan End memindahkan tab dan fokus", async ({
  page,
}) => {
  await masukSebagai(page, "superadmin");
  await buka(page, "/profil");
  await tungguTenang(page);

  const panjangRiwayat = () => page.evaluate(() => window.history.length);
  const awal = await panjangRiwayat();

  await test.step("klik tab: URL ikut, tab terpilih ikut", async () => {
    for (const { nama, url, tab } of TAUTAN_MENU.slice(1)) {
      await tabProfil(page, tab).click();
      await expect(page, `URL sesudah klik ${nama}`).toHaveURL(url);
      await expect(tabTerpilih(page)).toHaveText(tab);
    }
    // Tab bawaan tanpa query.
    await tabProfil(page, LABEL_TAB.profil).click();
    await expect(page).toHaveURL(
      (u) => u.pathname === "/profil" && u.search === "",
    );
    await expect(tabTerpilih(page)).toHaveText(LABEL_TAB.profil);
    expect(
      await panjangRiwayat(),
      "replaceState: riwayat tidak bertambah",
    ).toBe(awal);
  });

  await test.step("keyboard: panah, End, dan Home", async () => {
    await tabProfil(page, LABEL_TAB.profil).focus();
    const langkah: [string, string][] = [
      ["ArrowDown", LABEL_TAB.gantiSandi],
      ["ArrowDown", LABEL_TAB.mfa],
      ["End", LABEL_TAB.sesi],
      ["Home", LABEL_TAB.profil],
      ["ArrowRight", LABEL_TAB.gantiSandi],
      ["ArrowLeft", LABEL_TAB.profil],
    ];
    for (const [tombol, tab] of langkah) {
      await page.keyboard.press(tombol);
      await expect(tabTerpilih(page), `sesudah ${tombol}`).toHaveText(tab);
      await expect(
        tabProfil(page, tab),
        `fokus sesudah ${tombol}`,
      ).toBeFocused();
    }
    expect(await panjangRiwayat(), "keyboard pun memakai replaceState").toBe(
      awal,
    );
  });
});

test("navigasi: tanpa sesi, deep link /profil?tab=mfa diarahkan ke /masuk dengan tujuan semula", async ({
  page,
}) => {
  await page.goto("/profil?tab=mfa", { waitUntil: "domcontentloaded" });
  await expect(page).toHaveURL(
    (url) =>
      url.pathname === "/masuk" &&
      url.searchParams.get("lanjut") === "/profil?tab=mfa",
  );
});

test("navigasi: /keamanan dialihkan ke /profil?tab=keamanan; /ganti-password dan /mfa tetap membuka halaman auth", async ({
  page,
}) => {
  await masukSebagai(page, "superadmin");

  await page.goto("/keamanan", { waitUntil: "domcontentloaded" });
  await expect(page).toHaveURL(/\/profil\?tab=keamanan$/);
  await expect(tabTerpilih(page)).toHaveText(LABEL_TAB.sesi);
  await expect(page.getByRole("list", { name: "Sesi aktif" })).toBeVisible();

  // Dua halaman ini tujuan pengalihan wajib (edge-guard): tetap berdiri sendiri, tidak dialihkan ke /profil.
  await buka(page, "/ganti-password");
  await expect(page).toHaveURL(/\/ganti-password$/);
  await expect(
    page.getByRole("heading", { name: "Ganti kata sandi" }),
  ).toBeVisible();

  await buka(page, "/mfa");
  await expect(page).toHaveURL(/\/mfa$/);
  await expect(
    page.getByRole("heading", { name: "Verifikasi dua langkah" }),
  ).toBeVisible();
});

test("navigasi: di layar ponsel keempat tab dapat dijangkau dengan target sentuh minimal 44 px", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await masukSebagai(page, "superadmin");
  await buka(page, "/profil");
  await tungguTenang(page);

  for (const label of URUTAN_TAB) {
    const tab = tabProfil(page, label);
    await tab.scrollIntoViewIfNeeded();
    await expect(tab).toBeVisible();
    const kotak = await tab.boundingBox();
    expect(kotak, `kotak tab ${label}`).not.toBeNull();
    expect(
      kotak!.height,
      `tinggi tab ${label} (target sentuh)`,
    ).toBeGreaterThanOrEqual(44);
  }

  await tabProfil(page, LABEL_TAB.sesi).click();
  await expect(page).toHaveURL(/\/profil\?tab=keamanan$/);
  await expect(tabTerpilih(page)).toHaveText(LABEL_TAB.sesi);
});

// ── Edit Profil ──────────────────────────────────────────────────────────

test("Edit Profil: ubah nama dan telepon → toast, menu akun berubah tanpa masuk ulang, telepon ternormalisasi, bertahan, tercatat di audit", async ({
  page,
  galat,
  browser,
}) => {
  // Validasi bisa dijawab klien (skema) atau server (400); keduanya sah selama pesannya tampil inline.
  izinkanHttp(galat, 400, "PATCH", "/api/auth/me");

  const u = await penggunaSiap(dibuat);
  const namaAwal = namaAwalUji(u);
  const namaBaru = "Cut Mutia Ramadhani";
  const teleponKetik = "0812-3456-7890";
  const teleponE164 = "+6281234567890";
  const pesanNama = "Nama minimal 3 karakter";
  const pesanTelepon = "Nomor telepon tidak valid";

  await tanamSesi(page.context(), await masukApi(u.email, u.sandi, UA_MAC));
  await buka(page, "/profil");
  await tungguTenang(page);

  const nama = page.getByLabel("Nama lengkap");
  const telepon = page.getByLabel("Nomor telepon / WhatsApp");
  const email = page.getByLabel("Email");
  const simpan = page.getByRole("button", { name: "Simpan perubahan" });
  const batalkan = page.getByRole("button", { name: "Batalkan perubahan" });
  const toastDiperbarui = page.getByText("Profil diperbarui", { exact: true });

  await test.step("keadaan awal: data akun, email read-only, simpan nonaktif", async () => {
    await expect(tabTerpilih(page)).toHaveText(LABEL_TAB.profil);
    await expect(
      page.getByRole("heading", { name: "Data pribadi" }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "Penugasan & wilayah" }),
    ).toBeVisible();
    await expect(nama).toHaveValue(namaAwal);
    await expect(telepon).toHaveValue("");
    await expect(email).toHaveValue(u.email);
    await expect(email, "email hanya dibaca").not.toBeEditable();
    await expect(simpan, "tanpa perubahan").toBeDisabled();
    await expect(panelProfil(page)).toContainText(ROLES.OPS_AGREGATOR.label);
    await expect(panelProfil(page)).toContainText("FMIPA USK");
    await expect(tombolMenuAkun(page)).toHaveText(inisialNama(namaAwal));
  });

  await test.step("Batalkan perubahan mengembalikan isian dan menonaktifkan simpan", async () => {
    await nama.fill(namaBaru);
    await telepon.fill(teleponKetik);
    await expect(simpan).toBeEnabled();
    await batalkan.click();
    await expect(nama).toHaveValue(namaAwal);
    await expect(telepon).toHaveValue("");
    await expect(simpan).toBeDisabled();
  });

  await test.step("nama terlalu pendek dan telepon tidak valid tampil inline, tidak tersimpan", async () => {
    await nama.fill("Al");
    await telepon.fill("08123");
    // Pesan tampil selagi mengetik dan tombol simpan nonaktif selama ada isian yang tidak sah; bila validasinya
    // baru berjalan saat kirim, tombol aktif dan pesan tampil sesudah diklik. Keduanya sah.
    if (await simpan.isEnabled()) await simpan.click();
    await expect(page.getByText(pesanNama).first()).toBeVisible();
    await expect(page.getByText(pesanTelepon).first()).toBeVisible();
    await expect(page).toHaveURL(/\/profil$/);
    const server = await bacaProfilBff(page);
    expect(server.nama, "nama di server tidak berubah").toBe(namaAwal);
    expect(server.telepon, "telepon di server tidak berubah").toBeUndefined();
  });

  await test.step("simpan nama dan telepon yang benar", async () => {
    await nama.fill(namaBaru);
    await telepon.fill(teleponKetik);
    await simpan.click();
    await expect(toastDiperbarui).toBeVisible();
    await expect(
      simpan,
      "sesudah tersimpan tidak ada perubahan",
    ).toBeDisabled();
  });

  await test.step("menu akun memakai nama baru tanpa masuk ulang (cookie sesi diterbitkan ulang)", async () => {
    await expect(tombolMenuAkun(page)).toHaveText(inisialNama(namaBaru));
    await tombolMenuAkun(page).click();
    await expect(page.getByRole("menu")).toContainText(namaBaru);
    await page.keyboard.press("Escape");
    await expect(page.getByRole("menu")).toHaveCount(0);
    await expect
      .poll(
        async () => {
          const akses = await cookieBrowser(page.context(), "garda_access");
          return akses ? klaimToken(akses).name : undefined;
        },
        { message: "klaim nama pada cookie sesi baru" },
      )
      .toBe(namaBaru);
  });

  await test.step("telepon tersimpan dalam bentuk E.164; nilai sama di BFF dan di API Go", async () => {
    const lewatBff = await bacaProfilBff(page);
    expect(lewatBff.nama).toBe(namaBaru);
    expect(lewatBff.telepon).toBe(teleponE164);
    expect(lewatBff.email, "email tidak berubah").toBe(u.email);

    const sesi = await masukApi(u.email, u.sandi);
    const lewatApi = await goApi<ProfilBff>("GET", "auth/me", sesi.access);
    expect(lewatApi.status).toBe(200);
    expect(lewatApi.data.nama).toBe(namaBaru);
    expect(lewatApi.data.telepon).toBe(teleponE164);
  });

  await test.step("tercatat di audit sebagai UPDATE_PROFIL dengan keadaan sebelum dan sesudah", async () => {
    const admin = await apiSebagai("superadmin");
    try {
      let entri: {
        aksi: string;
        hasil: string;
        aktor_id?: string;
        sebelum?: unknown;
        sesudah?: unknown;
      }[] = [];
      await expect
        .poll(
          async () => {
            const res = await admin.get(
              `audit?aksi=UPDATE_PROFIL&entitas_id=${u.id}`,
            );
            expect(res.status(), "GET audit").toBe(200);
            entri = ((await res.json()) as { data: { items: typeof entri } })
              .data.items;
            return entri.length;
          },
          { message: "entri audit UPDATE_PROFIL" },
        )
        .toBe(1);
      expect(entri[0]).toMatchObject({
        aksi: "UPDATE_PROFIL",
        hasil: "BERHASIL",
        aktor_id: u.id,
      });
      const sebelum = JSON.stringify(entri[0].sebelum);
      const sesudah = JSON.stringify(entri[0].sesudah);
      expect(sebelum).toContain(namaAwal);
      expect(sesudah).toContain(namaBaru);
      expect(sesudah).toContain(teleponE164);
    } finally {
      await admin.dispose();
    }
  });

  await test.step("halaman /audit milik admin menampilkan baris UPDATE_PROFIL akun ini", async () => {
    const konteksAdmin = await browser.newContext({
      baseURL: BASE_URL,
      locale: "id-ID",
      timezoneId: "Asia/Jakarta",
      viewport: { width: 1440, height: 900 },
    });
    try {
      const halamanAdmin = await konteksAdmin.newPage();
      await masukSebagai(halamanAdmin, "superadmin");
      await buka(halamanAdmin, `/audit?entitas=pengguna&entitas_id=${u.id}`);
      await expect(halamanAdmin.getByText(`Objek ${u.id}`)).toBeVisible();
      await tungguTenang(halamanAdmin);
      await expect(
        halamanAdmin.locator("tbody tr").filter({ hasText: "UPDATE_PROFIL" }),
      ).toHaveCount(1);
    } finally {
      await konteksAdmin.close();
    }
  });

  await test.step("nilai bertahan setelah muat ulang, termasuk nama di menu akun", async () => {
    await page.reload({ waitUntil: "domcontentloaded" });
    await tungguTenang(page);
    await expect(nama).toHaveValue(namaBaru);
    await expect
      .poll(async () => (await telepon.inputValue()).replace(/\D/g, ""), {
        message: "telepon pada isian (hanya digit)",
      })
      .toBe(teleponE164.replace(/\D/g, ""));
    await expect(tombolMenuAkun(page)).toHaveText(inisialNama(namaBaru));
  });

  await test.step("mengosongkan telepon menghapus nomor; nama tidak berubah", async () => {
    await telepon.fill("");
    await simpan.click();
    await expect(toastDiperbarui).toBeVisible();
    const server = await bacaProfilBff(page);
    expect(server.telepon ?? "", "telepon dihapus").toBe("");
    expect(server.nama).toBe(namaBaru);
  });
});

test("API profil: PATCH /v1/auth/me hanya mengubah nama dan telepon; telepon dinormalisasi; masukan buruk ditolak", async () => {
  const u = await penggunaSiap(dibuat);
  const { access } = await masukApi(u.email, u.sandi, UA_MAC);
  const sebelum = (await goApi<ProfilBff>("GET", "auth/me", access)).data;
  const ubah = (body: unknown) =>
    goApi<ProfilBff>("PATCH", "auth/me", access, body);

  expect(
    (await goApi("PATCH", "auth/me", undefined, { nama: "Tanpa Token" }))
      .status,
    "tanpa token",
  ).toBe(401);
  const kosong = await ubah({});
  expect(kosong.status, "badan tanpa perubahan").toBe(400);
  expect(kosong.message).toBe("Tidak ada perubahan");

  await test.step("field di luar nama dan telepon diabaikan; nama dipangkas", async () => {
    const hasil = await ubah({
      nama: "  Nama Lewat API  ",
      telepon: "0812-3456-7890",
      email: "penyusup@gardainflasi.id",
      peran: "ADMIN_SISTEM",
      lapis: "SISTEM",
      lembaga: "Lembaga Lain",
      kabupaten_kode: "1101",
      status: "NONAKTIF",
    });
    expect(hasil.status).toBe(200);
    expect(hasil.data.nama).toBe("Nama Lewat API");
    expect(hasil.data.telepon).toBe("+6281234567890");
    const sesudah = (await goApi<ProfilBff>("GET", "auth/me", access)).data;
    expect(sesudah).toMatchObject({
      nama: "Nama Lewat API",
      telepon: "+6281234567890",
      email: sebelum.email,
      peran: sebelum.peran,
      lapis: sebelum.lapis,
      lembaga: sebelum.lembaga,
      status: sebelum.status,
    });
    expect(sesudah.kabupaten_kode, "kabupaten tidak berubah").toBe(
      sebelum.kabupaten_kode,
    );
  });

  await test.step("awalan 08, 62, dan +62 menjadi E.164 yang sama; string kosong menghapus", async () => {
    for (const mentah of [
      "081234567890",
      "62 812 3456 7890",
      "+62 (812) 3456-7890",
    ]) {
      const hapus = await ubah({ telepon: "" });
      expect(hapus.status, "hapus telepon").toBe(200);
      expect(hapus.data.telepon ?? "").toBe("");
      const hasil = await ubah({ telepon: mentah });
      expect(hasil.status, mentah).toBe(200);
      expect(hasil.data.telepon, mentah).toBe("+6281234567890");
    }
  });

  await test.step("nama dan telepon yang buruk ditolak dengan pesan dari kontrak", async () => {
    for (const buruk of [
      "12345",
      "08123",
      "abc",
      "+6501234567",
      "0800000000000000000",
    ]) {
      const hasil = await ubah({ telepon: buruk });
      expect(hasil.status, buruk).toBe(400);
      expect(hasil.message, buruk).toBe("Nomor telepon tidak valid");
    }
    const pendek = await ubah({ nama: "Al" });
    expect(pendek.status).toBe(400);
    expect(pendek.message).toBe("Nama minimal 3 karakter");
    const spasi = await ubah({ nama: "   " });
    expect(spasi.status, "nama hanya spasi").toBe(400);
    const panjang = await ubah({ nama: "N".repeat(101) });
    expect(panjang.status).toBe(400);
    expect(panjang.message).toBe("Nama maksimal 100 karakter");
    // Semua penolakan tidak mengubah apa pun.
    const akhir = (await goApi<ProfilBff>("GET", "auth/me", access)).data;
    expect(akhir.nama).toBe("Nama Lewat API");
    expect(akhir.telepon).toBe("+6281234567890");
  });
});

// ── Ganti Kata Sandi ─────────────────────────────────────────────────────

test("Ganti Kata Sandi: meter kekuatan, validasi, sandi baru mengeluarkan perangkat lain dan dipakai masuk", async ({
  page,
  galat,
  browser,
}) => {
  // Kata sandi saat ini yang salah dijawab 401 oleh API; BFF meneruskannya sebagai galat form.
  izinkanHttp(galat, 401, "POST", "/api/auth/change-password");

  const u = await penggunaSiap(dibuat);
  const lama = u.sandi;
  const baru = sandiAcak();
  const ini = await masukApi(u.email, lama, UA_MAC);
  const lain = await masukApi(u.email, lama, UA_ANDROID);

  const inputLama = page.getByLabel("Kata sandi saat ini");
  const inputBaru = page.getByLabel("Kata sandi baru", { exact: true });
  const inputUlang = page.getByLabel("Ulangi kata sandi baru");
  const kirim = tombolKirimPanel(page);

  await tanamSesi(page.context(), ini);
  await buka(page, "/profil?tab=ganti-password");
  await tungguTenang(page);

  // Konteks browser kedua = perangkat lain yang masih masuk dengan sandi lama.
  const konteks2 = await browser.newContext({
    baseURL: BASE_URL,
    locale: "id-ID",
    timezoneId: "Asia/Jakarta",
    viewport: { width: 1440, height: 900 },
  });
  try {
    await tanamSesi(konteks2, lain);
    const page2 = await konteks2.newPage();
    await page2.goto("/profil?tab=keamanan", { waitUntil: "domcontentloaded" });
    await expect(
      daftarSesi(page2),
      "perangkat lain masih masuk dan melihat dua sesi",
    ).toHaveCount(2);

    await test.step("kolom, aturan kata sandi, dan teks info: tanpa kotak centang, perangkat lain dikeluarkan", async () => {
      await expect(tabTerpilih(page)).toHaveText(LABEL_TAB.gantiSandi);
      await expect(inputLama).toBeVisible();
      await expect(inputBaru).toBeVisible();
      await expect(inputUlang).toBeVisible();
      await expect(
        page.getByRole("list", { name: "Aturan kata sandi" }),
      ).toContainText("Minimal 8 karakter");
      await expect(
        panelProfil(page).getByRole("checkbox"),
        "keputusan K1: tanpa pilihan, selalu mengeluarkan perangkat lain",
      ).toHaveCount(0);
      await expect(panelProfil(page)).toContainText(/perangkat lain/i);
    });

    await test.step("meter berubah Lemah → Sedang → Kuat menurut aturan dan panjang", async () => {
      const kekuatan = (nilai: string, harapan: string) =>
        test.step(`"${nilai}" → ${harapan}`, async () => {
          await inputBaru.fill(nilai);
          await expect
            .poll(() => bacaKekuatanSandi(page), {
              message: `kekuatan untuk "${nilai}"`,
            })
            .toBe(harapan);
        });
      await kekuatan("abc", "Lemah"); // aturan hampir semua belum terpenuhi
      await kekuatan("Abcdefg1", "Lemah"); // belum ada karakter khusus
      await kekuatan("Abcdefg1!xy", "Sedang"); // semua aturan, 11 karakter
      await kekuatan("Abcdefg1!xyz", "Kuat"); // semua aturan, 12 karakter
    });

    await test.step("kata sandi saat ini yang salah ditolak server dan sesi tetap hidup", async () => {
      await inputLama.fill("Salah#Lama1x");
      await inputBaru.fill(baru);
      await inputUlang.fill(baru);
      await kirim.click();
      await expect(page.getByText(/Password lama salah/).first()).toBeVisible();
      await expect(page).toHaveURL(/\/profil\?tab=ganti-password$/);
      expect(await statusMe(ini.access), "sesi ini tetap hidup").toBe(200);
    });

    await test.step("konfirmasi yang berbeda tampil inline", async () => {
      await inputLama.fill(lama);
      await inputBaru.fill(baru);
      await inputUlang.fill(`${baru}x`);
      await kirim.click();
      await expect(
        page.getByText("Konfirmasi kata sandi tidak sama.").first(),
      ).toBeVisible();
    });

    await test.step("sukses: toast, tetap di tab, form dikosongkan", async () => {
      await inputUlang.fill(baru);
      await kirim.click();
      await expect(
        page.getByText("Kata sandi diganti", { exact: true }),
      ).toBeVisible();
      await expect(page).toHaveURL(/\/profil\?tab=ganti-password$/);
      await expect(tabTerpilih(page)).toHaveText(LABEL_TAB.gantiSandi);
      await expect(inputLama).toHaveValue("");
      await expect(inputBaru).toHaveValue("");
      await expect(inputUlang).toHaveValue("");
    });

    await test.step("perangkat lain dikeluarkan: token dan browser keduanya; sesi ini tetap masuk", async () => {
      expect(await statusMe(lain.access), "access token perangkat lain").toBe(
        401,
      );
      expect(await statusRefresh(lain.refresh), "refresh token-nya").toBe(401);
      // Browser kedua kehilangan sesinya begitu halaman memuat data (401 lalu refresh ditolak) dan ke /masuk.
      await page2.reload({ waitUntil: "domcontentloaded" });
      await expect(page2).toHaveURL((url) => url.pathname === "/masuk", {
        timeout: 20_000,
      });

      // Browser ini tetap masuk dengan cookie baru dan hanya sesinya yang tersisa.
      await tabProfil(page, LABEL_TAB.sesi).click();
      await expect(daftarSesi(page)).toHaveCount(1);
      await expect(daftarSesi(page).first()).toContainText("Sesi Ini");
    });
  } finally {
    await konteks2.close();
  }

  await test.step("sandi baru berhasil dipakai masuk dan sandi lama ditolak", async () => {
    await sesiApi(u.email, baru);
    await expect(sesiApi(u.email, lama)).rejects.toThrow(/401/);
    u.sandi = baru;

    await page.context().clearCookies();
    await masukLewatForm(page, u.email, baru);
    await harapBeranda(page, "OPS_AGREGATOR");
  });
});

// ── Verifikasi 2 Langkah ─────────────────────────────────────────────────

test("Verifikasi 2 Langkah: aktivasi dengan TOTP menampilkan QR dan 10 kode cadangan yang dapat disalin dan diunduh", async ({
  page,
  galat,
}) => {
  // Verifikasi MFA mencabut sesi lama dan memasang sesi baru; permintaan latar yang masih membawa token lama
  // dapat dijawab 401 sesaat sebelum cookie baru terpasang.
  izinkan401(galat);

  const u = await penggunaSiap(dibuat);
  await tanamSesi(page.context(), await masukApi(u.email, u.sandi, UA_MAC));
  await page.context().grantPermissions(["clipboard-read", "clipboard-write"], {
    origin: new URL(BASE_URL).origin,
  });
  await buka(page, "/profil?tab=mfa");
  await tungguTenang(page);
  await expect(tabTerpilih(page)).toHaveText(LABEL_TAB.mfa);

  let secret = "";
  let kode: string[] = [];

  await test.step("mulai setup: rahasia dari server dan QR SVG ter-render", async () => {
    const setup = page.waitForResponse(
      (r) =>
        r.request().method() === "POST" &&
        new URL(r.url()).pathname === "/api/auth/mfa/setup",
    );
    await page.getByRole("button", { name: /Mulai setup/i }).click();
    const res = await setup;
    expect(res.status(), "POST /api/auth/mfa/setup").toBe(200);
    ({ secret } = (await res.json()) as { secret: string });
    expect(secret, "rahasia TOTP base32").toMatch(/^[A-Z2-7]{16,}$/);

    const qr = page.getByRole("img", { name: /Kode QR/i });
    await expect(qr).toBeVisible();
    // SVG tanpa width/height (hanya viewBox) bisa punya naturalWidth 0, jadi keberhasilan dimuat diperiksa lewat
    // decode() (menolak bila gambar rusak) dan ukuran tampilnya.
    expect(
      await qr.evaluate(async (el) => {
        if (!(el instanceof HTMLImageElement))
          return el.tagName.toLowerCase() === "svg";
        await el.decode();
        return (
          el.src.startsWith("data:image/svg+xml") &&
          el.getBoundingClientRect().width > 0
        );
      }),
      "QR berupa SVG yang berhasil dimuat",
    ).toBe(true);
  });

  await test.step("kode TOTP 6 digit mengaktifkan MFA dan menampilkan 10 kode cadangan", async () => {
    const verifikasi = page.waitForResponse(
      (r) =>
        r.request().method() === "POST" &&
        new URL(r.url()).pathname === "/api/auth/mfa/verify",
    );
    await page.getByLabel(/Kode 6 digit/i).fill(kodeTotp(secret));
    await page.getByRole("button", { name: /^Aktifkan/i }).click();
    const res = await verifikasi;
    expect(res.status(), "POST /api/auth/mfa/verify").toBe(200);
    ({ kode_cadangan: kode } = (await res.json()) as {
      kode_cadangan: string[];
    });

    expect(kode, "tepat 10 kode cadangan").toHaveLength(10);
    expect(new Set(kode).size, "kode cadangan unik").toBe(10);
    for (const k of kode) expect(k).toMatch(/^[A-Z0-9]{5}-[A-Z0-9]{5}$/);
    for (const k of kode) await expect(panelProfil(page)).toContainText(k);
  });

  await test.step("Unduh menghasilkan berkas .txt berisi semua kode", async () => {
    const unduhan = page.waitForEvent("download");
    await page.getByRole("button", { name: /^Unduh/i }).click();
    const berkas = await unduhan;
    expect(berkas.suggestedFilename()).toMatch(/\.txt$/);
    const jalur = await berkas.path();
    expect(jalur, "berkas unduhan tersimpan").not.toBeNull();
    const isi = readFileSync(jalur!, "utf8");
    for (const k of kode) expect(isi).toContain(k);
    expect(isi, "berkas menyebut akunnya").toContain(u.email);
  });

  await test.step("Salin menaruh semua kode di papan klip", async () => {
    await page.bringToFront();
    await page.getByRole("button", { name: /^Salin/i }).click();
    await expect(page.getByText(/disalin/i).first()).toBeVisible();
    const papan = await page.evaluate(() => navigator.clipboard.readText());
    for (const k of kode) expect(papan).toContain(k);
  });

  await test.step("setelah kode disimpan: MFA aktif dengan 10 kode tersisa", async () => {
    await page.getByRole("checkbox", { name: /sudah menyimpan/i }).check();
    const selesai = page.getByRole("button", { name: /^(Selesai|Lanjutkan)/i });
    await expect(selesai).toBeEnabled();
    await selesai.click();
    await expect(panelProfil(page)).toContainText(
      /Verifikasi dua langkah aktif/i,
    );
    await expect(panelProfil(page)).toContainText(
      /10\s+kode cadangan tersisa/i,
    );

    const profil = await bacaProfilBff(page);
    expect(profil.mfa_aktif, "MFA aktif di server").toBe(true);
    expect(profil.kode_cadangan_tersisa, "sisa kode cadangan").toBe(10);
  });
});

// ── Perangkat & Sesi Aktif ───────────────────────────────────────────────

test("Perangkat & Sesi Aktif: badge Sesi Ini, cabut satu sesi dengan konfirmasi inline, keluarkan semua perangkat lain lewat dialog", async ({
  page,
}) => {
  const u = await penggunaSiap(dibuat);
  const ini = await masukApi(u.email, u.sandi, UA_MAC);
  const android = await masukApi(u.email, u.sandi, UA_ANDROID);
  const iphone = await masukApi(u.email, u.sandi, UA_IPHONE);

  await tanamSesi(page.context(), ini);
  await buka(page, "/profil?tab=keamanan");
  await tungguTenang(page);
  const butir = daftarSesi(page);

  await test.step("tiga sesi; sesi ini punya badge Sesi Ini dan tanpa tombol cabut", async () => {
    await expect(tabTerpilih(page)).toHaveText(LABEL_TAB.sesi);
    await expect(butir).toHaveCount(3);
    const sesiIni = butir.filter({ hasText: "Sesi Ini" });
    await expect(sesiIni).toHaveCount(1);
    await expect(sesiIni.getByText("Sesi Ini", { exact: true })).toBeVisible();
    await expect(sesiIni).toContainText("Chrome di macOS");
    await expect(sesiIni).toContainText(/IP (\d|tidak diketahui)/);
    await expect(
      sesiIni.getByRole("button", { name: /Cabut/ }),
      "sesi yang sedang dipakai tidak bisa dicabut dari daftar",
    ).toHaveCount(0);
  });

  await test.step("Cabut meminta konfirmasi inline; Batal membatalkan; Konfirmasi cabut mencabut", async () => {
    const sesiAndroid = butir.filter({ hasText: "Chrome di Android" });
    await expect(sesiAndroid).toHaveCount(1);
    const cabut = sesiAndroid.getByRole("button", {
      name: /^Cabut sesi Chrome di Android/,
    });
    const konfirmasi = page.getByRole("button", { name: /Konfirmasi cabut/ });
    const batal = page.getByRole("button", { name: "Batal", exact: true });

    await cabut.click();
    await expect(konfirmasi).toBeVisible();
    await expect(batal).toBeVisible();
    await batal.click();
    await expect(konfirmasi).toHaveCount(0);
    await expect(butir).toHaveCount(3);
    expect(await statusMe(android.access), "batal tidak mencabut").toBe(200);

    await cabut.click();
    await konfirmasi.click();
    await expect(page.getByText("Sesi dicabut", { exact: true })).toBeVisible();
    await expect(butir).toHaveCount(2);
    expect(await statusMe(android.access), "access token tercabut").toBe(401);
    expect(await statusRefresh(android.refresh), "refresh token tercabut").toBe(
      401,
    );
    expect(await statusMe(iphone.access), "sesi lain tidak ikut").toBe(200);
  });

  await test.step("Keluarkan semua perangkat lain: dialog, Batal tidak mengubah, Ya mengeluarkan", async () => {
    const keluarkan = page.getByRole("button", {
      name: "Keluarkan semua perangkat lain",
    });
    const dialog = page.getByRole("alertdialog", {
      name: "Keluarkan semua perangkat lain?",
    });

    await keluarkan.click();
    await expect(dialog).toBeVisible();
    await dialog.getByRole("button", { name: "Batal" }).click();
    await expect(dialog).toHaveCount(0);
    expect(await statusMe(iphone.access), "batal tidak mengeluarkan").toBe(200);

    await keluarkan.click();
    await dialog.getByRole("button", { name: "Ya, keluarkan" }).click();
    await expect(
      page.getByText("Perangkat lain dikeluarkan", { exact: true }),
    ).toBeVisible();
    await expect(butir).toHaveCount(1);
    await expect(butir.first()).toContainText("Sesi Ini");
    expect(await statusMe(iphone.access), "perangkat lain keluar").toBe(401);
    expect(await statusRefresh(iphone.refresh)).toBe(401);
    expect(await statusMe(ini.access), "sesi ini tetap masuk").toBe(200);
  });
});

// ── Alur wajib ───────────────────────────────────────────────────────────

test("alur wajib: akun wajib ganti kata sandi dialihkan dari /profil ke /ganti-password, dan API menolak ubah profil", async ({
  page,
}) => {
  // Akun baru buatan admin masih wajib mengganti kata sandi; tanpa langkah lepasKewajibanGantiSandi.
  const u = await buatPengguna(dibuat);
  const sesi = await masukApi(u.email, u.sandi, UA_MAC);
  await tanamSesi(page.context(), sesi);

  for (const path of ["/profil", "/profil?tab=mfa", "/profil?tab=keamanan"]) {
    await page.goto(path, { waitUntil: "domcontentloaded" });
    await expect(page, `${path} dialihkan`).toHaveURL(/\/ganti-password(\?|$)/);
  }
  await expect(
    page.getByRole("heading", { name: "Ganti kata sandi" }),
  ).toBeVisible();

  await test.step("API: membaca profil sendiri tetap boleh, mengubahnya ditolak sampai kata sandi diganti", async () => {
    const baca = await goApi<ProfilBff>("GET", "auth/me", sesi.access);
    expect(baca.status).toBe(200);
    expect(baca.data.wajib_ganti_password).toBe(true);
    const ubah = await goApi("PATCH", "auth/me", sesi.access, {
      nama: "Nama Sebelum Ganti Sandi",
    });
    expect(ubah.status).toBe(403);
    expect(ubah.code).toBe("PASSWORD_CHANGE_REQUIRED");
  });
});
