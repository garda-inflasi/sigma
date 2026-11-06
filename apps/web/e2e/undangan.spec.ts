/**
 * Undang pengguna lewat email, impor CSV, dan ekspor CSV pengguna (02-D).
 *
 *  1. Superadmin mengundang pengguna dari /pengguna → email undangan masuk Mailpit → tautan /aktivasi menetapkan
 *     kata sandi → masuk lewat form mendarat di beranda; tautan bekas ditolak; akun menunggu belum bisa masuk.
 *  2. "Kirim ulang undangan" mengirim email baru dan tautan lama berhenti berlaku.
 *  3. Impor CSV: satu baris sah dan dua baris salah → pratinjau menampilkan galat per baris → konfirmasi → akun
 *     muncul sebagai "Menunggu aktivasi" dan emailnya terkirim; templat CSV bisa diunduh.
 *  4. Ekspor CSV dari server (semua dan hanya yang dipilih) memuat akun seed, tanpa rahasia, dan tercatat di audit.
 *
 * Aturan suite ini:
 *  - Semua akun uji berformat `e2e.<timestamp>…@gardainflasi.id` dan dihapus lewat API di afterAll, beserta email
 *    Mailpit dan baris kunci_login-nya; akun seed tidak diubah.
 *  - Anggaran laju BFF (5/menit/IP, dipakai bersama login): setiap kiriman form aktivasi dan masuk lewat
 *    `anggaranBff.pakai(...)`. Pemakaian suite: aktivasi 3 (sukses, tautan bekas, tautan baru) + masuk 1.
 *    Penolakan tautan lama sesudah kirim ulang diperiksa lewat API Go langsung (tidak memakai anggaran BFF).
 *  - Butuh Mailpit (E2E_MAILPIT_URL, bawaan http://localhost:8025) dan stack yang dibangun dari kode ini
 *    (migration 000108 dan SMTP API Go yang mengarah ke Mailpit: `make docker/demo/up`).
 */
import { readFileSync } from "node:fs";
import { createHash } from "node:crypto";
import type { Page } from "@playwright/test";
import { ROLES } from "../src/lib/rbac/roles";
import {
  anggaranBff,
  apiPublik,
  hapusEmail,
  pastikanMailpit,
  sandiAcak,
  tautanAktivasi,
  tungguEmail,
} from "./auth-helpers";
import {
  apiSebagai,
  buka,
  expect,
  jalankanSql,
  masukSebagai,
  sesiApi,
  test,
  tungguTenang,
  type PantauGalat,
} from "./fixtures";

// Tes yang menunggu jendela laju BFF bisa berjalan lebih dari semenit.
test.describe.configure({ mode: "serial", timeout: 180_000 });

const stempel = Date.now();
const emailUndang = `e2e.${stempel}@gardainflasi.id`;
const emailUlang = `e2e.${stempel}.ulang@gardainflasi.id`;
const emailImpor = `e2e.${stempel}.impor@gardainflasi.id`;
const semuaEmail = [emailUndang, emailUlang, emailImpor];
const namaUndang = `Pengguna Undangan ${stempel}`;
const namaUlang = `Pengguna Ulang ${stempel}`;
const namaImpor = `Pengguna Impor ${stempel}`;

const alertForm = (page: Page) => page.locator('form [role="alert"]');
const kotakCari = (page: Page) => page.getByPlaceholder(/^Cari nama, email/);

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

/** Membuka /pengguna sebagai superadmin dan menunggu daftar siap. */
async function bukaPengguna(page: Page): Promise<void> {
  await masukSebagai(page, "superadmin");
  await buka(page, "/pengguna");
  await tungguTenang(page);
}

/** Baris tabel pengguna untuk satu email (mencari dulu supaya baris ada di halaman tabel). */
async function barisPengguna(page: Page, email: string) {
  await kotakCari(page).fill(email);
  const baris = page.getByRole("row").filter({ hasText: email });
  await expect(baris).toHaveCount(1);
  return baris;
}

/** Id dan status pengguna menurut API (null bila tidak ada). */
async function cariPengguna(
  email: string,
): Promise<{ id: string; status: string } | null> {
  const api = await apiSebagai("superadmin");
  try {
    const res = await api.get(
      `users?search=${encodeURIComponent(email)}&limit=100`,
    );
    expect(res.status()).toBe(200);
    const { data } = (await res.json()) as {
      data: { items: { id: string; email: string; status: string }[] };
    };
    return (
      data.items.find((u) => u.email.toLowerCase() === email.toLowerCase()) ??
      null
    );
  } finally {
    await api.dispose();
  }
}

/** Menunggu email undangan terbaru yang tokennya berbeda dari `tokenLama` (kirim ulang menambah email). */
async function tungguTokenBaru(
  penerima: string,
  tokenLama: string,
): Promise<string> {
  const akhir = Date.now() + 20_000;
  for (;;) {
    const email = await tungguEmail(penerima);
    const { token } = tautanAktivasi(email.isi);
    if (token !== tokenLama) return token;
    if (Date.now() > akhir)
      throw new Error(
        `Email undangan baru untuk ${penerima} tidak datang dalam 20 dtk`,
      );
    await new Promise((r) => setTimeout(r, 500));
  }
}

/** Mengisi form /aktivasi lalu mengirimnya (satu pemakaian anggaran laju BFF). */
async function kirimAktivasi(
  page: Page,
  path: string,
  sandi: string,
): Promise<void> {
  await page.goto(path, { waitUntil: "domcontentloaded" });
  await tungguTenang(page); // tunggu hidrasi supaya isian tidak tertimpa
  await expect(
    page.getByRole("heading", { name: "Aktifkan akun Anda" }),
  ).toBeVisible();
  await page.getByLabel("Kata sandi baru", { exact: true }).fill(sandi);
  await page.getByLabel("Ulangi kata sandi baru").fill(sandi);
  await anggaranBff.pakai("aktivasi");
  await page.getByRole("button", { name: /Aktifkan akun/ }).click();
}

/** Mengisi form /masuk lalu mengirimnya (satu pemakaian anggaran laju login). */
async function masukLewatForm(
  page: Page,
  email: string,
  sandi: string,
): Promise<void> {
  await page.goto("/masuk", { waitUntil: "domcontentloaded" });
  await tungguTenang(page);
  await page.getByLabel("Email institusi").fill(email);
  await page.getByLabel("Kata sandi", { exact: true }).fill(sandi);
  await expect(page.getByLabel("Email institusi")).toHaveValue(email);
  await anggaranBff.pakai("login");
  await page.getByRole("button", { name: "Masuk", exact: true }).click();
}

/** Berkas yang diunduh peramban sebagai teks UTF-8 (BOM tetap ada di awal). */
async function bacaUnduhan(unduhan: {
  path(): Promise<string | null>;
}): Promise<string> {
  const lokasi = await unduhan.path();
  expect(lokasi, "berkas unduhan tersimpan").toBeTruthy();
  return readFileSync(lokasi as string).toString("utf8");
}

async function jumlahAuditEkspor(): Promise<number> {
  const api = await apiSebagai("superadmin");
  try {
    const res = await api.get(
      "audit?aksi=EKSPOR_DATA&entitas=pengguna&limit=200",
    );
    expect(res.status()).toBe(200);
    const { data } = (await res.json()) as { data: { items: unknown[] } };
    return data.items.length;
  } finally {
    await api.dispose();
  }
}

test.afterAll(async () => {
  const gagal: string[] = [];
  const api = await apiSebagai("superadmin");
  try {
    for (const email of semuaEmail) {
      try {
        const pengguna = await cariPengguna(email);
        if (pengguna) {
          const res = await api.delete(`users/${pengguna.id}`);
          if (res.status() !== 200 && res.status() !== 404)
            gagal.push(`hapus ${email}: ${res.status()} ${await res.text()}`);
        }
      } catch (err) {
        gagal.push(`${email}: ${(err as Error).message}`);
      }
      await hapusEmail(email);
    }
  } finally {
    await api.dispose();
  }
  // Baris kunci_login hanya berisi hash email (dibuat oleh percobaan masuk akun menunggu); dibersihkan agar demo kembali seperti semula.
  try {
    const hash = (e: string) =>
      createHash("sha256").update(e.toLowerCase()).digest("hex");
    jalankanSql(
      `DELETE FROM kunci_login WHERE email_hash IN (${semuaEmail.map((e) => `'${hash(e)}'`).join(",")});`,
    );
  } catch (err) {
    console.warn(
      `[undangan.spec] pembersihan kunci_login dilewati: ${(err as Error).message}`,
    );
  }
  expect(gagal, "akun uji yang gagal dihapus").toEqual([]);
});

// ── 1. Undang → email → aktivasi → masuk ─────────────────────────────────

test("superadmin mengundang pengguna → email Mailpit → atur kata sandi di /aktivasi → masuk ke beranda; tautan bekas ditolak", async ({
  page,
  galat,
}) => {
  await pastikanMailpit();
  const sandi = sandiAcak();
  let path = "";
  let token = "";

  await test.step("undang dari /pengguna: akun muncul sebagai Menunggu aktivasi", async () => {
    await bukaPengguna(page);
    await page.getByRole("button", { name: "Undang pengguna" }).click();
    const dialog = page.getByRole("dialog");
    await expect(
      dialog.getByRole("heading", { name: "Undang pengguna" }),
    ).toBeVisible();

    await dialog.getByLabel("Nama lengkap").fill(namaUndang);
    await dialog.getByLabel("Email", { exact: true }).fill(emailUndang);
    await dialog.getByLabel("Lembaga").fill("Mitra Agregator");
    await dialog.getByRole("combobox", { name: "Peran" }).click();
    await page
      .getByRole("option", { name: new RegExp(ROLES.OPS_AGREGATOR.label) })
      .click();
    // Peran dengan cakupan kabupaten menampilkan pilihan kabupaten/kota.
    await dialog.getByRole("combobox", { name: /Kabupaten\/kota/ }).click();
    await page.getByRole("option", { name: "Kota Banda Aceh" }).click();
    await dialog.getByRole("button", { name: "Kirim undangan" }).click();

    await expect(page.getByText("Undangan dikirim").first()).toBeVisible();
    await expect(dialog).toBeHidden();

    const baris = await barisPengguna(page, emailUndang);
    await expect(baris.getByText("Menunggu aktivasi")).toBeVisible();
    await expect(baris.getByText("Banda Aceh")).toBeVisible();
    await expect(
      baris.getByText(ROLES.OPS_AGREGATOR.label).first(),
    ).toBeVisible();
  });

  await test.step("email undangan berisi tautan aktivasi", async () => {
    const email = await tungguEmail(emailUndang);
    expect(email.subjek).toContain("Undangan Akun");
    expect(email.isi).toContain(namaUndang);
    ({ path, token } = tautanAktivasi(email.isi));
    expect(token.length).toBeGreaterThanOrEqual(43);
  });

  await test.step("akun yang menunggu belum bisa masuk", async () => {
    await expect(sesiApi(emailUndang, sandi)).rejects.toThrow(/401/);
    expect((await cariPengguna(emailUndang))?.status).toBe("MENUNGGU");
  });

  await test.step("tautan yang terpotong menawarkan jalan keluar, bukan formulir", async () => {
    await page.goto("about:blank");
    await page.context().clearCookies();
    await page.goto("/aktivasi", { waitUntil: "domcontentloaded" });
    await expect(page.getByText(/Tautan tidak lengkap/)).toBeVisible();
    await expect(
      page.getByRole("link", { name: "Ke halaman masuk" }),
    ).toBeVisible();
  });

  await test.step("kata sandi yang belum memenuhi aturan ditolak di form, tautan tetap berlaku", async () => {
    await page.goto(path, { waitUntil: "domcontentloaded" });
    await tungguTenang(page);
    await page.getByLabel("Kata sandi baru", { exact: true }).fill("lemah");
    await page.getByLabel("Ulangi kata sandi baru").fill("lemah");
    await page.getByRole("button", { name: /Aktifkan akun/ }).click();
    await expect(
      page.getByText("Kata sandi baru belum memenuhi semua aturan."),
    ).toBeVisible();
  });

  await test.step("atur kata sandi lewat tautan", async () => {
    await kirimAktivasi(page, path, sandi);
    await expect(page.getByText(/Akun Anda sudah aktif/)).toBeVisible();
    await expect(page.getByRole("link", { name: "Masuk" })).toBeVisible();
    expect((await cariPengguna(emailUndang))?.status).toBe("AKTIF");
  });

  await test.step("tautan yang sama tidak bisa dipakai lagi", async () => {
    izinkanHttp(galat, 401, "POST", "/api/auth/aktivasi");
    await kirimAktivasi(page, path, sandiAcak());
    await expect(alertForm(page)).toContainText(
      /tidak valid atau sudah dipakai/,
    );
    await expect(
      page.getByText(/Minta administrator mengirim ulang undangan/),
    ).toBeVisible();
    // Kata sandi pertama tidak tertimpa.
    await sesiApi(emailUndang, sandi);
  });

  await test.step("masuk lewat form mendarat di beranda peran", async () => {
    await masukLewatForm(page, emailUndang, sandi);
    await expect(page).toHaveURL(
      (url) => url.pathname === ROLES.OPS_AGREGATOR.beranda,
      { timeout: 20_000 },
    );
    await tungguTenang(page);
  });

  await test.step("tercatat di audit trail", async () => {
    const pengguna = await cariPengguna(emailUndang);
    expect(pengguna).not.toBeNull();
    const api = await apiSebagai("superadmin");
    try {
      for (const aksi of ["UNDANG_PENGGUNA", "AKTIVASI_AKUN"]) {
        const res = await api.get(
          `audit?aksi=${aksi}&entitas_id=${pengguna!.id}`,
        );
        const { data } = (await res.json()) as {
          data: { items: { aktor_id?: string }[] };
        };
        expect(data.items, aksi).toHaveLength(1);
        if (aksi === "AKTIVASI_AKUN")
          expect(data.items[0].aktor_id).toBe(pengguna!.id);
      }
    } finally {
      await api.dispose();
    }
  });
});

// ── 2. Kirim ulang undangan ──────────────────────────────────────────────

test("Kirim ulang undangan mengirim email baru dan tautan lama berhenti berlaku", async ({
  page,
}) => {
  await pastikanMailpit();
  const sandi = sandiAcak();

  // Akun menunggu disiapkan lewat API (alur dialog sudah diuji di atas).
  const admin = await apiSebagai("superadmin");
  try {
    const res = await admin.post("users/undang", {
      data: {
        nama: namaUlang,
        email: emailUlang,
        peran: "OPS_AGREGATOR",
        lembaga: "Mitra Agregator",
      },
    });
    expect(res.status(), await res.text()).toBe(201);
  } finally {
    await admin.dispose();
  }
  const { token: tokenLama } = tautanAktivasi(
    (await tungguEmail(emailUlang)).isi,
  );

  await test.step("aksi baris hanya ada untuk akun yang menunggu", async () => {
    await bukaPengguna(page);
    const aktif = await barisPengguna(page, "superadmin@gardainflasi.id");
    await aktif.getByRole("button", { name: /^Aksi untuk / }).click();
    await expect(
      page.getByRole("menuitem", { name: "Reset kata sandi" }),
    ).toBeVisible();
    await expect(
      page.getByRole("menuitem", { name: "Kirim ulang undangan" }),
    ).toHaveCount(0);
    await page.keyboard.press("Escape");

    const menunggu = await barisPengguna(page, emailUlang);
    await expect(menunggu.getByText("Menunggu aktivasi")).toBeVisible();
    await menunggu
      .getByRole("button", { name: `Aksi untuk ${namaUlang}` })
      .click();
    await expect(
      page.getByRole("menuitem", { name: "Reset kata sandi" }),
    ).toHaveCount(0);
    await page.getByRole("menuitem", { name: "Kirim ulang undangan" }).click();
    await expect(
      page.getByText("Undangan dikirim ulang").first(),
    ).toBeVisible();
  });

  let tokenBaru = "";
  await test.step("email baru datang dengan token berbeda", async () => {
    tokenBaru = await tungguTokenBaru(emailUlang, tokenLama);
    expect(tokenBaru).not.toBe(tokenLama);
  });

  await test.step("tautan lama ditolak API dan akun tetap menunggu", async () => {
    const publik = await apiPublik();
    try {
      const res = await publik.post("auth/aktivasi", {
        data: { token: tokenLama, password: sandi },
      });
      expect(res.status()).toBe(401);
      expect(((await res.json()) as { message: string }).message).toMatch(
        /tidak valid atau sudah dipakai/,
      );
    } finally {
      await publik.dispose();
    }
    expect((await cariPengguna(emailUlang))?.status).toBe("MENUNGGU");
  });

  await test.step("tautan baru mengaktifkan akun", async () => {
    await page.goto("about:blank");
    await page.context().clearCookies();
    await kirimAktivasi(
      page,
      `/aktivasi?token=${encodeURIComponent(tokenBaru)}`,
      sandi,
    );
    await expect(page.getByText(/Akun Anda sudah aktif/)).toBeVisible();
    await sesiApi(emailUlang, sandi);
    expect((await cariPengguna(emailUlang))?.status).toBe("AKTIF");
  });

  await test.step("akun yang sudah aktif tidak bisa diundang ulang (API)", async () => {
    const pengguna = await cariPengguna(emailUlang);
    const api = await apiSebagai("superadmin");
    try {
      const res = await api.post(`users/${pengguna!.id}/undang-ulang`);
      expect(res.status()).toBe(409);
    } finally {
      await api.dispose();
    }
  });
});

// ── 3. Impor CSV ─────────────────────────────────────────────────────────

test("Impor CSV: pratinjau menampilkan galat per baris, konfirmasi membuat akun yang sah sebagai undangan", async ({
  page,
}) => {
  await pastikanMailpit();
  const csv =
    "nama;email;peran;lembaga;kabupaten_kode\r\n" +
    `${namaImpor};${emailImpor};OPS_MAHASISWA;FMIPA USK;1171\r\n` +
    `Email Salah;bukan-email;OPS_MAHASISWA;FMIPA USK;\r\n` +
    `Sudah Terdaftar;SUPERADMIN@gardainflasi.id;OPS_MAHASISWA;FMIPA USK;\r\n`;

  await bukaPengguna(page);
  await page.getByRole("button", { name: "Impor CSV" }).click();
  const dialog = page.getByRole("dialog");
  await expect(
    dialog.getByRole("heading", { name: "Impor pengguna dari CSV" }),
  ).toBeVisible();

  await test.step("templat CSV bisa diunduh (header saja, BOM UTF-8)", async () => {
    const [unduhan] = await Promise.all([
      page.waitForEvent("download"),
      dialog.getByRole("button", { name: "Unduh templat CSV" }).click(),
    ]);
    expect(unduhan.suggestedFilename()).toBe("template-impor-pengguna.csv");
    const isi = await bacaUnduhan(unduhan);
    expect(isi.charCodeAt(0)).toBe(0xfeff);
    expect(isi.slice(1).trim()).toBe("nama;email;peran;lembaga;kabupaten_kode");
  });

  await test.step("berkas yang bukan .csv ditolak sebelum dikirim", async () => {
    await dialog
      .getByLabel("Berkas CSV")
      .setInputFiles({
        name: "pengguna.xlsx",
        mimeType: "application/vnd.ms-excel",
        buffer: Buffer.from("x"),
      });
    await expect(dialog.getByRole("alert")).toContainText(/berekstensi \.csv/);
  });

  await test.step("pratinjau: satu baris valid, dua ditolak dengan alasannya", async () => {
    await dialog
      .getByLabel("Berkas CSV")
      .setInputFiles({
        name: "pengguna.csv",
        mimeType: "text/csv",
        buffer: Buffer.from(csv, "utf8"),
      });
    await expect(
      dialog.getByRole("status").filter({ hasText: "3 baris diperiksa" }),
    ).toContainText("1 valid, 2 ditolak dan tidak akan diimpor.");

    const tabel = dialog.getByRole("table", { name: "Pratinjau impor" });
    const sah = tabel.getByRole("row").filter({ hasText: emailImpor });
    await expect(sah.getByText("Valid", { exact: true })).toBeVisible();
    const salah = tabel.getByRole("row").filter({ hasText: "bukan-email" });
    await expect(salah.getByText("Ditolak")).toBeVisible();
    await expect(salah.getByText("Email tidak valid.")).toBeVisible();
    const terdaftar = tabel
      .getByRole("row")
      .filter({ hasText: "superadmin@gardainflasi.id" });
    await expect(terdaftar.getByText("Email sudah terdaftar.")).toBeVisible();
    // Nomor baris mengikuti nomor di spreadsheet (header = 1).
    await expect(sah.getByRole("cell").first()).toHaveText("2");
    await expect(salah.getByRole("cell").first()).toHaveText("3");

    // Pratinjau tidak membuat apa pun.
    expect(await cariPengguna(emailImpor)).toBeNull();
  });

  await test.step("konfirmasi: hanya baris sah yang dibuat", async () => {
    await dialog
      .getByRole("button", { name: "Impor 1 pengguna valid" })
      .click();
    await expect(
      dialog.getByRole("status").filter({ hasText: "akun dibuat" }),
    ).toContainText("1 akun dibuat sebagai undangan, 2 baris ditolak.");
    const hasil = dialog.getByRole("table", { name: "Hasil impor" });
    await expect(
      hasil
        .getByRole("row")
        .filter({ hasText: emailImpor })
        .getByText("Dibuat"),
    ).toBeVisible();
    await dialog.getByRole("button", { name: "Selesai" }).click();
    await expect(dialog).toBeHidden();
  });

  await test.step("akun muncul di daftar sebagai Menunggu aktivasi", async () => {
    const baris = await barisPengguna(page, emailImpor);
    await expect(baris.getByText("Menunggu aktivasi")).toBeVisible();
    expect((await cariPengguna(emailImpor))?.status).toBe("MENUNGGU");
  });

  await test.step("email undangan terkirim ke akun yang diimpor dan tautannya berlaku", async () => {
    const email = await tungguEmail(emailImpor);
    const { token } = tautanAktivasi(email.isi);
    const publik = await apiPublik();
    try {
      // Kata sandi lemah ditolak tanpa menghabiskan tautan: cukup membuktikan token dikenal server.
      const res = await publik.post("auth/aktivasi", {
        data: { token, password: "lemah-sekali" },
      });
      expect(res.status()).toBe(400);
    } finally {
      await publik.dispose();
    }
  });

  await test.step("baris yang ditolak tidak membuat akun", async () => {
    expect(await cariPengguna("bukan-email")).toBeNull();
  });

  await test.step("tercatat di audit: satu entri per akun dan satu ringkasan", async () => {
    const pengguna = await cariPengguna(emailImpor);
    const api = await apiSebagai("superadmin");
    try {
      const res = await api.get(
        `audit?aksi=UNDANG_PENGGUNA&entitas_id=${pengguna!.id}`,
      );
      const { data } = (await res.json()) as {
        data: { items: { metadata?: { sumber?: string } }[] };
      };
      expect(data.items).toHaveLength(1);
      expect(data.items[0].metadata?.sumber).toBe("impor");
      const ringkasan = await api.get("audit?aksi=IMPOR_PENGGUNA&limit=5");
      const { data: r } = (await ringkasan.json()) as {
        data: { items: unknown[] };
      };
      expect(r.items.length).toBeGreaterThanOrEqual(1);
    } finally {
      await api.dispose();
    }
  });
});

// ── 4. Ekspor CSV ────────────────────────────────────────────────────────

test("Ekspor CSV dari server: semua dan hanya yang dipilih, tanpa rahasia, tercatat di audit", async ({
  page,
}) => {
  const sebelum = await jumlahAuditEkspor();
  await bukaPengguna(page);

  await test.step("Ekspor semua (CSV) memuat akun seed dan akun uji, tanpa rahasia", async () => {
    const [unduhan] = await Promise.all([
      page.waitForEvent("download"),
      page.getByRole("button", { name: "Ekspor semua (CSV)" }).click(),
    ]);
    expect(unduhan.suggestedFilename()).toMatch(
      /^pengguna-\d{4}-\d{2}-\d{2}\.csv$/,
    );
    const isi = await bacaUnduhan(unduhan);
    expect(isi.charCodeAt(0)).toBe(0xfeff);
    const baris = isi.slice(1).trim().split(/\r?\n/);
    expect(baris[0]).toBe(
      "id;nama;email;peran;lapis;lembaga;kabupaten_kode;status;mfa_aktif;terakhir_masuk;dibuat_pada",
    );
    expect(isi).toContain("superadmin@gardainflasi.id");
    expect(isi).toContain(emailImpor);
    expect(isi).toMatch(new RegExp(`${emailImpor};.*;MENUNGGU;`));
    for (const terlarang of [
      "$2a$",
      "$2b$",
      "password",
      "mfa_secret",
      "JBSWY3DP",
    ])
      expect(isi).not.toContain(terlarang);
    await expect(
      page.getByText("Ekspor pengguna selesai").first(),
    ).toBeVisible();
  });

  await test.step("Ekspor CSV pada aksi massal hanya memuat baris terpilih", async () => {
    const baris = await barisPengguna(page, emailImpor);
    await baris.getByRole("checkbox", { name: "Pilih baris" }).click();
    const bilah = page.getByRole("toolbar", { name: /terpilih/ });
    await expect(bilah).toBeVisible();
    // Aksi kelima berada di menu "Aksi lainnya" (bilah menampilkan tiga aksi pertama).
    await bilah.getByRole("button", { name: "Aksi lainnya" }).click();
    const [unduhan] = await Promise.all([
      page.waitForEvent("download"),
      page.getByRole("menuitem", { name: "Ekspor CSV" }).click(),
    ]);
    const isi = await bacaUnduhan(unduhan);
    const rekaman = isi.slice(1).trim().split(/\r?\n/);
    expect(rekaman).toHaveLength(2);
    expect(rekaman[1]).toContain(emailImpor);
    expect(isi).not.toContain("superadmin@gardainflasi.id");
  });

  await test.step("setiap ekspor tercatat sebagai EKSPOR_DATA", async () => {
    expect(await jumlahAuditEkspor()).toBeGreaterThanOrEqual(sebelum + 2);
  });

  await test.step("peran tanpa pengguna.kelola tidak bisa mengekspor", async () => {
    const api = await apiSebagai("mahasiswa");
    try {
      const res = await api.get("users/ekspor");
      expect(res.status()).toBe(403);
      expect(await res.text()).not.toContain("@gardainflasi.id");
    } finally {
      await api.dispose();
    }
  });
});
