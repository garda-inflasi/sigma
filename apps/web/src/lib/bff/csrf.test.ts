import assert from "node:assert/strict";
import { afterEach, beforeEach, describe, it } from "node:test";
// Harus sebelum modul Next lain: memasang AsyncLocalStorage global yang dibutuhkan penyimpanan permintaan Next.
import { denganCookie } from "./request-scope-testing";
import { NextResponse } from "next/server";
import { CSRF_COOKIE } from "./cookie-policy";
import {
  generateCSRFToken,
  setCSRFCookie,
  validateCSRF,
  validateOrigin,
} from "./csrf";

const ENV_ASLI = {
  APP_ALLOWED_ORIGINS: process.env.APP_ALLOWED_ORIGINS,
  NEXT_PUBLIC_APP_URL: process.env.NEXT_PUBLIC_APP_URL,
  COOKIE_SECURE: process.env.COOKIE_SECURE,
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

describe("generateCSRFToken", () => {
  it("32 byte acak dalam heksadesimal huruf kecil (64 karakter) dan tidak berulang", async () => {
    const a = await generateCSRFToken();
    const b = await generateCSRFToken();
    assert.match(a, /^[0-9a-f]{64}$/);
    assert.match(b, /^[0-9a-f]{64}$/);
    assert.notEqual(a, b);
  });
});

describe("setCSRFCookie", () => {
  it("terbaca skrip (double-submit), SameSite=Lax, path /, 24 jam", () => {
    const res = NextResponse.json({});
    setCSRFCookie(res, "token-csrf");
    const c = res.cookies.get(CSRF_COOKIE);
    assert.equal(c?.value, "token-csrf");
    assert.equal(
      c?.httpOnly,
      false,
      "klien harus bisa membacanya untuk header",
    );
    assert.equal(c?.sameSite, "lax");
    assert.equal(c?.path, "/");
    assert.equal(c?.maxAge, 60 * 60 * 24);
  });

  it("Secure mengikuti COOKIE_SECURE", () => {
    process.env.COOKIE_SECURE = "true";
    const aman = NextResponse.json({});
    setCSRFCookie(aman, "t");
    assert.equal(aman.cookies.get(CSRF_COOKIE)?.secure, true);

    process.env.COOKIE_SECURE = "false";
    const polos = NextResponse.json({});
    setCSRFCookie(polos, "t");
    assert.notEqual(polos.cookies.get(CSRF_COOKIE)?.secure, true);
  });
});

describe("validateCSRF (double-submit)", () => {
  const permintaan = (header?: string) =>
    new Request("http://localhost/api/x", {
      method: "POST",
      headers: header === undefined ? {} : { "x-csrf-token": header },
    });
  const valid = (cookie: string, header?: string) =>
    denganCookie(cookie, () => validateCSRF(permintaan(header)));

  it("lolos bila header sama persis dengan cookie", async () => {
    assert.equal(await valid(`${CSRF_COOKIE}=abc123`, "abc123"), true);
  });

  it("ditolak bila header berbeda, walau panjangnya sama", async () => {
    assert.equal(await valid(`${CSRF_COOKIE}=abc123`, "abc124"), false);
  });

  it("ditolak bila panjang berbeda (awalan yang sama tidak cukup)", async () => {
    assert.equal(await valid(`${CSRF_COOKIE}=abc123`, "abc"), false);
    assert.equal(await valid(`${CSRF_COOKIE}=abc`, "abc123"), false);
  });

  it("ditolak bila header hilang atau kosong", async () => {
    assert.equal(await valid(`${CSRF_COOKIE}=abc123`), false);
    assert.equal(await valid(`${CSRF_COOKIE}=abc123`, ""), false);
  });

  it("ditolak bila cookie hilang, juga ketika header kosong (kosong tidak sama dengan kosong)", async () => {
    assert.equal(await valid("lain=x", "abc123"), false);
    assert.equal(await valid("lain=x", ""), false);
    assert.equal(await valid("", undefined), false);
  });

  it("cookie dengan nama mirip tidak dihitung", async () => {
    assert.equal(await valid("xgarda_csrf=abc123", "abc123"), false);
  });
});

describe("validateOrigin", () => {
  const dari = (headers: Record<string, string>) =>
    new Request("http://web:3000/api/auth/logout", {
      method: "POST",
      headers,
    });

  describe("origin yang sama dengan host yang dituju peramban", () => {
    it("Origin sama dengan X-Forwarded-Host (di belakang Caddy)", () => {
      assert.equal(
        validateOrigin(
          dari({
            origin: "https://garda.example",
            "x-forwarded-host": "garda.example",
            "x-forwarded-proto": "https",
          }),
        ),
        true,
      );
    });

    it("Host dipakai bila tidak ada X-Forwarded-Host", () => {
      assert.equal(
        validateOrigin(
          dari({ origin: "http://garda.example", host: "garda.example" }),
        ),
        true,
      );
    });

    it("skema http atau https untuk host yang sama sama-sama diterima (TLS diakhiri proxy)", () => {
      const host = { "x-forwarded-host": "garda.example" };
      assert.equal(
        validateOrigin(dari({ ...host, origin: "http://garda.example" })),
        true,
      );
      assert.equal(
        validateOrigin(dari({ ...host, origin: "https://garda.example" })),
        true,
      );
    });

    it("port harus sama", () => {
      assert.equal(
        validateOrigin(
          dari({
            origin: "https://garda.example:8443",
            "x-forwarded-host": "garda.example",
          }),
        ),
        false,
      );
      assert.equal(
        validateOrigin(
          dari({
            origin: "http://localhost:3000",
            "x-forwarded-host": "localhost:3000",
          }),
        ),
        true,
      );
    });
  });

  describe("situs lain ditolak", () => {
    const sah = { "x-forwarded-host": "garda.example" };

    it("origin asing", () => {
      assert.equal(
        validateOrigin(dari({ ...sah, origin: "https://evil.example" })),
        false,
      );
    });

    it("host yang hanya menyerupai (akhiran atau awalan)", () => {
      for (const origin of [
        "https://evil-garda.example",
        "https://garda.example.evil.com",
        "https://garda.example@evil.com",
        "https://sub.garda.example",
      ]) {
        assert.equal(validateOrigin(dari({ ...sah, origin })), false, origin);
      }
    });

    it("Origin 'null' (dokumen sandbox, redirect lintas situs) dan isi sampah", () => {
      assert.equal(validateOrigin(dari({ ...sah, origin: "null" })), false);
      assert.equal(
        validateOrigin(dari({ ...sah, origin: "bukan url" })),
        false,
      );
    });

    it("tanpa Origin dan tanpa Referer ditolak (gagal tertutup)", () => {
      assert.equal(validateOrigin(dari(sah)), false);
    });
  });

  describe("Referer sebagai cadangan", () => {
    const sah = { "x-forwarded-host": "garda.example" };

    it("dipakai bila Origin tidak ada; hanya origin-nya yang dibandingkan, path diabaikan", () => {
      assert.equal(
        validateOrigin(
          dari({ ...sah, referer: "https://garda.example/keamanan?x=1" }),
        ),
        true,
      );
      assert.equal(
        validateOrigin(
          dari({
            ...sah,
            referer: "https://evil.example/https://garda.example/",
          }),
        ),
        false,
      );
    });

    it("Origin didahulukan: Origin asing tidak bisa ditolong Referer yang sah", () => {
      assert.equal(
        validateOrigin(
          dari({
            ...sah,
            origin: "https://evil.example",
            referer: "https://garda.example/",
          }),
        ),
        false,
      );
    });
  });

  describe("daftar izin dari konfigurasi", () => {
    const asing = {
      origin: "https://publik.example",
      "x-forwarded-host": "internal.svc",
    };

    it("APP_ALLOWED_ORIGINS (dipisah koma, spasi diabaikan) menambah origin yang diterima", () => {
      process.env.APP_ALLOWED_ORIGINS =
        " https://publik.example , https://lain.example ";
      assert.equal(validateOrigin(dari(asing)), true);
      assert.equal(
        validateOrigin(dari({ ...asing, origin: "https://lain.example" })),
        true,
      );
      assert.equal(
        validateOrigin(dari({ ...asing, origin: "https://ketiga.example" })),
        false,
      );
    });

    it("entri rusak dilewati, bukan membuka semuanya", () => {
      process.env.APP_ALLOWED_ORIGINS = "bukan url,,*, https://publik.example";
      assert.equal(validateOrigin(dari(asing)), true);
      assert.equal(
        validateOrigin(dari({ ...asing, origin: "https://evil.example" })),
        false,
        "'*' bukan wildcard",
      );
    });

    it("NEXT_PUBLIC_APP_URL dipakai bila APP_ALLOWED_ORIGINS tidak diisi", () => {
      process.env.NEXT_PUBLIC_APP_URL = "https://publik.example/";
      assert.equal(validateOrigin(dari(asing)), true);
    });

    it("APP_ALLOWED_ORIGINS mengalahkan NEXT_PUBLIC_APP_URL", () => {
      process.env.APP_ALLOWED_ORIGINS = "https://lain.example";
      process.env.NEXT_PUBLIC_APP_URL = "https://publik.example";
      assert.equal(validateOrigin(dari(asing)), false);
    });

    it("tanpa konfigurasi apa pun, bawaannya http://localhost:3000", () => {
      assert.equal(
        validateOrigin(
          dari({
            origin: "http://localhost:3000",
            "x-forwarded-host": "x.svc",
          }),
        ),
        true,
      );
      assert.equal(validateOrigin(dari(asing)), false);
    });
  });
});
