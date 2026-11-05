/**
 * Pembantu e2e masuk dengan Google (dipakai sso.spec.ts):
 *  - anggaran laju rute BFF SSO (terpisah dari anggaran login di auth-helpers.ts),
 *  - pengisian halaman login interaktif mock OIDC (mock-oauth2-server, profil compose `sso-mock`),
 *  - SQL pemulih keadaan akun seed yang ditautkan oleh tes.
 */
import { readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, type Page } from "@playwright/test";

// ── Anggaran laju ──────────────────────────────────────────────────────────
//
// Rute BFF membatasi per IP dalam jendela geser 60 detik (lib/bff/rate-limit.ts):
//   GET /api/auth/sso/google           → kunci `…:sso-mulai`    maksimal 10
//   GET /api/auth/sso/google/callback  → kunci `…:sso-callback` maksimal 10
// Setiap permintaan yang lolos menambah satu entri. Satu alur penuh memakai 1 mulai + 1 callback; `pakai`
// mencatat tiap pemakaian dan menunggu deterministik sampai jendela cukup longgar (tanpa retry acak).
// Catatan disimpan di berkas sementara agar dua kali jalan berurutan (< 1 menit) saling mengetahui.

export type JenisSso = "mulai" | "callback";

const BERKAS = join(tmpdir(), "garda-e2e-sso-laju.json");
const JENDELA_MS = 60_000;
const BATAS = 10;
/** Cadangan atas selisih jam dan latensi antara pencatatan di sini dan di server. */
const MARGIN_MS = 3_000;

type Catatan = Record<JenisSso, number[]>;

function muat(): Catatan {
  try {
    const nilai = JSON.parse(readFileSync(BERKAS, "utf8")) as Partial<Catatan>;
    const angka = (a: unknown) =>
      Array.isArray(a)
        ? a.filter((t): t is number => typeof t === "number")
        : [];
    return { mulai: angka(nilai.mulai), callback: angka(nilai.callback) };
  } catch {
    return { mulai: [], callback: [] };
  }
}

export const anggaranSso = {
  /** Dipanggil tepat sebelum aksi yang mengenai rute SSO berbatas laju; menunggu bila perlu. */
  async pakai(jenis: JenisSso): Promise<void> {
    for (;;) {
      const sekarang = Date.now();
      const catatan = muat();
      const aktif = catatan[jenis]
        .filter((t) => sekarang - t < JENDELA_MS + MARGIN_MS)
        .sort((a, b) => a - b);
      if (aktif.length < BATAS) {
        writeFileSync(
          BERKAS,
          JSON.stringify({ ...catatan, [jenis]: [...aktif, sekarang] }),
        );
        return;
      }
      const tunggu =
        aktif[aktif.length - BATAS] + JENDELA_MS + MARGIN_MS - sekarang + 50;
      console.log(
        `[anggaran-sso] ${jenis}: jendela laju penuh (${aktif.length}/${BATAS}), menunggu ${Math.ceil(tunggu / 1000)} dtk`,
      );
      await new Promise((r) => setTimeout(r, tunggu));
    }
  },
};

// ── Mock OIDC ──────────────────────────────────────────────────────────────

/** Halaman otorisasi mock: penerbit http://sso.localhost:<port>/default (identik bagi browser dan API). */
export const POLA_URL_MOCK = /^http:\/\/sso\.localhost:\d+\/default\/authorize/;

/**
 * Halaman login mock memuat font dari fonts.googleapis.com; di lingkungan tanpa internet itu menjadi
 * console.error yang menggagalkan fixture `galat`. Jawab dengan CSS kosong.
 */
export async function stubFontMock(page: Page): Promise<void> {
  await page.route(/fonts\.(googleapis|gstatic)\.com/, (route) =>
    route.fulfill({ status: 200, contentType: "text/css", body: "" }),
  );
}

/**
 * Mengisi dan mengirim halaman login mock. `sub` menjadi klaim sub (identitas Google), `klaim` ditambahkan
 * ke ID token; email_verified bawaan true.
 */
export async function masukDiMock(
  page: Page,
  sub: string,
  klaim: Record<string, unknown>,
): Promise<void> {
  await expect(page).toHaveURL(POLA_URL_MOCK);
  await page.locator('input[name="username"]').fill(sub);
  await page
    .locator('textarea[name="claims"]')
    .fill(JSON.stringify({ email_verified: true, ...klaim }));
  await page.getByRole("button", { name: "Sign-in" }).click();
}

// ── Keadaan akun seed ──────────────────────────────────────────────────────

/** Email akun seed yang dipakai spec ini; tautan SSO-nya dilepas sebelum dan sesudah tiap tes. */
export const EMAIL_UJI_SSO = [
  "superadmin@gardainflasi.id",
  "mfa@gardainflasi.id",
  "nonaktif@gardainflasi.id",
  "mahasiswa.1@gardainflasi.id",
];

/** SQL yang mengembalikan akun seed ke keadaan belum bertaut (UNIQUE sso_provider+sso_subject bebas lagi). */
export function sqlLepasTautan(): string {
  const daftar = EMAIL_UJI_SSO.map((e) => `'${e}'`).join(", ");
  return `UPDATE pengguna SET sso_provider = NULL, sso_subject = NULL, sso_terhubung_pada = NULL WHERE email IN (${daftar});`;
}
