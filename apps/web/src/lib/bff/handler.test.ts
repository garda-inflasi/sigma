import assert from "node:assert/strict";
import { afterEach, beforeEach, describe, it } from "node:test";
// Harus sebelum modul Next lain: memasang AsyncLocalStorage global yang dibutuhkan penyimpanan permintaan Next.
import { denganCookie } from "./request-scope-testing";
import { NextRequest } from "next/server";
import { ACCESS_COOKIE, CSRF_COOKIE } from "./cookie-policy";
import { handleRouteError, requireSession } from "./handler";

const ENV_ASLI = {
  APP_ALLOWED_ORIGINS: process.env.APP_ALLOWED_ORIGINS,
  NEXT_PUBLIC_APP_URL: process.env.NEXT_PUBLIC_APP_URL,
};

beforeEach(() => {
  delete process.env.APP_ALLOWED_ORIGINS;
  delete process.env.NEXT_PUBLIC_APP_URL;
});

afterEach(() => {
  for (const [k, v] of Object.entries(ENV_ASLI)) {
    if (v === undefined) delete process.env[k];
    else process.env[k] = v;
  }
});

/** Permintaan mutasi dari halaman sendiri; `ubah` menimpa header bawaan. */
function mutasi(ubah: Record<string, string | null> = {}) {
  const headers: Record<string, string> = {
    origin: "https://garda.example",
    "x-forwarded-host": "garda.example",
    "x-csrf-token": "csrf-1",
  };
  for (const [k, v] of Object.entries(ubah)) {
    if (v === null) delete headers[k];
    else headers[k] = v;
  }
  return new NextRequest("http://web:3000/api/pengguna/u1", {
    method: "PATCH",
    headers,
  });
}

const COOKIE_SAH = `${ACCESS_COOKIE}=jwt-akses; ${CSRF_COOKIE}=csrf-1`;

async function badan(res: Response): Promise<Record<string, unknown>> {
  return (await res.json()) as Record<string, unknown>;
}

describe("requireSession tanpa pemeriksaan CSRF (GET)", () => {
  it("mengembalikan access token dari cookie", async () => {
    const hasil = await denganCookie(COOKIE_SAH, () =>
      requireSession(mutasi()),
    );
    assert.equal(hasil.accessToken, "jwt-akses");
    assert.equal(hasil.error, undefined);
  });

  it("tanpa cookie access: 401 'Tidak terautentikasi' dengan header keamanan", async () => {
    const hasil = await denganCookie(`${CSRF_COOKIE}=csrf-1`, () =>
      requireSession(mutasi()),
    );
    assert.equal(hasil.accessToken, undefined);
    assert.equal(hasil.error?.status, 401);
    assert.deepEqual(await badan(hasil.error!), {
      error: true,
      message: "Tidak terautentikasi.",
    });
    assert.equal(hasil.error?.headers.get("x-content-type-options"), "nosniff");
  });

  it("tidak memeriksa CSRF atau Origin (permintaan baca)", async () => {
    const hasil = await denganCookie(COOKIE_SAH, () =>
      requireSession(
        mutasi({ "x-csrf-token": "salah", origin: "https://evil.example" }),
      ),
    );
    assert.equal(hasil.accessToken, "jwt-akses");
  });
});

describe("requireSession dengan csrf: true (mutasi)", () => {
  const jalankan = (cookie: string, req: NextRequest) =>
    denganCookie(cookie, () => requireSession(req, { csrf: true }));

  it("lolos bila token CSRF cocok dan Origin sama", async () => {
    const hasil = await jalankan(COOKIE_SAH, mutasi());
    assert.equal(hasil.accessToken, "jwt-akses");
  });

  it("token CSRF salah: 403 dan access token tidak dikembalikan", async () => {
    const hasil = await jalankan(
      COOKIE_SAH,
      mutasi({ "x-csrf-token": "lain" }),
    );
    assert.equal(hasil.accessToken, undefined);
    assert.equal(hasil.error?.status, 403);
    assert.equal(
      (await badan(hasil.error!)).message,
      "CSRF token tidak valid.",
    );
  });

  it("header CSRF hilang: 403", async () => {
    const hasil = await jalankan(COOKIE_SAH, mutasi({ "x-csrf-token": null }));
    assert.equal(hasil.error?.status, 403);
  });

  it("cookie CSRF hilang: 403", async () => {
    const hasil = await jalankan(`${ACCESS_COOKIE}=jwt-akses`, mutasi());
    assert.equal(hasil.error?.status, 403);
  });

  it("Origin asing: 403 walau token CSRF cocok (lapis kedua)", async () => {
    const hasil = await jalankan(
      COOKIE_SAH,
      mutasi({ origin: "https://evil.example" }),
    );
    assert.equal(hasil.error?.status, 403);
  });

  it("tanpa Origin maupun Referer: 403", async () => {
    const hasil = await jalankan(COOKIE_SAH, mutasi({ origin: null }));
    assert.equal(hasil.error?.status, 403);
  });

  it("CSRF diperiksa sebelum sesi: tanpa access token tetapi CSRF salah tetap 403, bukan 401", async () => {
    const hasil = await jalankan(
      `${CSRF_COOKIE}=csrf-1`,
      mutasi({ "x-csrf-token": "lain" }),
    );
    assert.equal(hasil.error?.status, 403);
  });

  it("CSRF sah tetapi tanpa access token: 401", async () => {
    const hasil = await jalankan(`${CSRF_COOKIE}=csrf-1`, mutasi());
    assert.equal(hasil.error?.status, 401);
  });
});

describe("handleRouteError", () => {
  const galatApi = (
    status: number,
    message: string,
    extra: { code?: string; detik?: number } = {},
  ) => Object.assign(new Error(message), { status, ...extra });

  it("meneruskan status dan pesan dari API Go", async () => {
    const res = handleRouteError(galatApi(404, "Pengguna tidak ditemukan"));
    assert.equal(res.status, 404);
    assert.deepEqual(await badan(res), {
      error: true,
      message: "Pengguna tidak ditemukan",
    });
  });

  it("meneruskan kode mesin stabil (PASSWORD_CHANGE_REQUIRED) tanpa diubah", async () => {
    const res = handleRouteError(
      galatApi(403, "Ganti kata sandi dulu", {
        code: "PASSWORD_CHANGE_REQUIRED",
      }),
    );
    assert.equal(res.status, 403);
    assert.equal((await badan(res)).kode, "PASSWORD_CHANGE_REQUIRED");
  });

  it("kode yang hanya mengulang status ('403') tidak dijadikan kode mesin", async () => {
    const res = handleRouteError(
      galatApi(403, "Akses ditolak", { code: "403" }),
    );
    assert.equal("kode" in (await badan(res)), false);
  });

  it("sisa waktu tunggu menjadi sisa_detik dan header Retry-After (akun terkunci)", async () => {
    const res = handleRouteError(
      galatApi(423, "terkunci", { code: "ACCOUNT_LOCKED", detik: 840 }),
    );
    assert.equal(res.status, 423);
    assert.equal(res.headers.get("Retry-After"), "840");
    const isi = await badan(res);
    assert.equal(isi.sisa_detik, 840);
    assert.equal(isi.kode, "ACCOUNT_LOCKED");
  });

  it("galat tak terduga menjadi 500 generik tanpa membocorkan pesan aslinya", async () => {
    const res = handleRouteError(
      new Error("pq: password authentication failed for user garda"),
    );
    assert.equal(res.status, 500);
    const teks = JSON.stringify(await badan(res));
    assert.doesNotMatch(teks, /pq:|password authentication/);
    assert.match(teks, /Kesalahan server internal/);
  });

  it("nilai yang bukan Error (string, undefined) juga 500", async () => {
    assert.equal(handleRouteError("boom").status, 500);
    assert.equal(handleRouteError(undefined).status, 500);
  });

  it("objek biasa dengan status tetap 500 (hanya instance Error yang dipercaya)", async () => {
    assert.equal(handleRouteError({ status: 418, message: "x" }).status, 500);
  });
});
