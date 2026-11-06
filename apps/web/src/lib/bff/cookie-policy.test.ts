import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";

import {
  ACCESS_COOKIE,
  CSRF_COOKIE,
  REFRESH_COOKIE,
  REFRESH_COOKIE_PATH,
  RESUME_COOKIE,
  RESUME_COOKIE_MAX_AGE,
  cookieSecure,
} from "./cookie-policy";

describe("cookieSecure", () => {
  it("aktif di produksi tanpa konfigurasi", () => {
    assert.equal(cookieSecure({ NODE_ENV: "production" }), true);
  });

  it("nonaktif di pengembangan dan tes", () => {
    assert.equal(cookieSecure({ NODE_ENV: "development" }), false);
    assert.equal(cookieSecure({ NODE_ENV: "test" }), false);
    assert.equal(cookieSecure({}), false);
  });

  it("COOKIE_SECURE=false mematikan Secure di produksi (akses http://)", () => {
    assert.equal(
      cookieSecure({ NODE_ENV: "production", COOKIE_SECURE: "false" }),
      false,
    );
  });

  it("COOKIE_SECURE=true menyalakan Secure di luar produksi", () => {
    assert.equal(
      cookieSecure({ NODE_ENV: "development", COOKIE_SECURE: " TRUE " }),
      true,
    );
  });

  it("nilai COOKIE_SECURE yang tidak dikenal jatuh ke aturan lingkungan", () => {
    assert.equal(
      cookieSecure({ NODE_ENV: "production", COOKIE_SECURE: "" }),
      true,
    );
    assert.equal(
      cookieSecure({ NODE_ENV: "development", COOKIE_SECURE: "ya" }),
      false,
    );
  });
});

describe("cookie token BFF", () => {
  const original = { ...process.env };

  afterEach(() => {
    process.env.COOKIE_SECURE = original.COOKIE_SECURE;
    if (original.COOKIE_SECURE === undefined) delete process.env.COOKIE_SECURE;
  });

  async function setCookieHeaders(): Promise<string[]> {
    const { NextResponse } = await import("next/server");
    const { setAccessTokenCookie, setRefreshTokenCookie } =
      await import("./cookies");
    const { setCSRFCookie } = await import("./csrf");
    const response = NextResponse.json({});
    setAccessTokenCookie(response, "akses");
    setRefreshTokenCookie(response, "segar");
    setCSRFCookie(response, "csrf");
    return response.headers.getSetCookie();
  }

  it("menandai ketiga cookie Secure bila COOKIE_SECURE=true", async () => {
    process.env.COOKIE_SECURE = "true";
    const headers = await setCookieHeaders();
    assert.equal(headers.length, 3);
    for (const header of headers) assert.match(header, /;\s*Secure/i, header);
  });

  it("tidak menambahkan Secure bila COOKIE_SECURE=false", async () => {
    process.env.COOKIE_SECURE = "false";
    const headers = await setCookieHeaders();
    for (const header of headers)
      assert.doesNotMatch(header, /;\s*Secure/i, header);
  });
});

describe("nama dan path cookie sesi", () => {
  // Nama dibaca juga oleh API Go (pkg/middleware/auth.go dan csrf.go) dan klien (garda_csrf): mengubahnya di
  // satu sisi saja mematikan autentikasi cookie dan pemeriksaan CSRF diam-diam.
  it("nama tetap dan dipakai bersama dengan backend Go", () => {
    assert.equal(ACCESS_COOKIE, "garda_access");
    assert.equal(REFRESH_COOKIE, "garda_refresh");
    assert.equal(CSRF_COOKIE, "garda_csrf");
    assert.equal(RESUME_COOKIE, "garda_pulih");
  });

  it("cookie refresh dibatasi ke /api/auth dan penanda pemulihan singkat", () => {
    assert.equal(REFRESH_COOKIE_PATH, "/api/auth");
    assert.ok(RESUME_COOKIE_MAX_AGE > 0 && RESUME_COOKIE_MAX_AGE <= 60);
  });
});

describe("cookieSecure membaca process.env secara bawaan", () => {
  const asli = {
    COOKIE_SECURE: process.env.COOKIE_SECURE,
    NODE_ENV: process.env.NODE_ENV,
  };
  afterEach(() => {
    for (const [k, v] of Object.entries(asli)) {
      if (v === undefined) delete process.env[k];
      else process.env[k] = v;
    }
  });

  it("mengikuti COOKIE_SECURE yang sedang berlaku", () => {
    process.env.COOKIE_SECURE = "true";
    assert.equal(cookieSecure(), true);
    process.env.COOKIE_SECURE = "false";
    assert.equal(cookieSecure(), false);
  });
});
