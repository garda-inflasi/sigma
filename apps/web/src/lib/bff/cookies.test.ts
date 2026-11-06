import assert from "node:assert/strict";
import { afterEach, beforeEach, describe, it } from "node:test";
// Harus sebelum modul Next lain: memasang AsyncLocalStorage global yang dibutuhkan penyimpanan permintaan Next.
import { denganCookie } from "./request-scope-testing";
import { NextResponse } from "next/server";
import { MFA_CHALLENGE_COOKIE, MFA_CHALLENGE_MAX_AGE } from "@/lib/auth/mfa";
import { ACCESS_COOKIE, REFRESH_COOKIE } from "./cookie-policy";
import {
  clearMfaChallengeCookie,
  clearTokenCookies,
  getAccessToken,
  getMfaChallenge,
  getRefreshToken,
  setAccessTokenCookie,
  setMfaChallengeCookie,
  setRefreshTokenCookie,
} from "./cookies";
const SECURE_ASLI = process.env.COOKIE_SECURE;

beforeEach(() => {
  process.env.COOKIE_SECURE = "true";
});

afterEach(() => {
  if (SECURE_ASLI === undefined) delete process.env.COOKIE_SECURE;
  else process.env.COOKIE_SECURE = SECURE_ASLI;
});

const baru = () => NextResponse.json({});

describe("cookie access token", () => {
  it("httpOnly, Secure, SameSite=Lax, path /, umur 15 menit bawaan", () => {
    const res = baru();
    setAccessTokenCookie(res, "jwt-akses");
    const c = res.cookies.get(ACCESS_COOKIE);
    assert.equal(c?.value, "jwt-akses");
    assert.equal(c?.httpOnly, true, "tidak terbaca skrip halaman");
    assert.equal(c?.secure, true);
    assert.equal(c?.sameSite, "lax");
    assert.equal(c?.path, "/");
    assert.equal(c?.maxAge, 15 * 60);
  });

  it("umur dapat ditentukan pemanggil", () => {
    const res = baru();
    setAccessTokenCookie(res, "jwt", 60);
    assert.equal(res.cookies.get(ACCESS_COOKIE)?.maxAge, 60);
  });
});

describe("cookie refresh token", () => {
  // Path sempit membuat edge tidak pernah melihat refresh token dan mengurangi permukaan pengiriman otomatis;
  // Strict membuat navigasi lintas situs tidak membawanya (pemulihan sesi berjalan dari dalam situs).
  it("httpOnly, Secure, SameSite=Strict, hanya untuk /api/auth, umur 7 hari bawaan", () => {
    const res = baru();
    setRefreshTokenCookie(res, "rt-opaque");
    const c = res.cookies.get(REFRESH_COOKIE);
    assert.equal(c?.value, "rt-opaque");
    assert.equal(c?.httpOnly, true);
    assert.equal(c?.secure, true);
    assert.equal(c?.sameSite, "strict");
    assert.equal(c?.path, "/api/auth");
    assert.equal(c?.maxAge, 7 * 24 * 60 * 60);
  });
});

describe("cookie tantangan MFA", () => {
  it("lima menit, Strict, hanya untuk /api/auth, tidak terbaca skrip", () => {
    const res = baru();
    setMfaChallengeCookie(res, "mfa.jwt");
    const c = res.cookies.get(MFA_CHALLENGE_COOKIE);
    assert.equal(c?.value, "mfa.jwt");
    assert.equal(c?.httpOnly, true);
    assert.equal(c?.secure, true);
    assert.equal(c?.sameSite, "strict");
    assert.equal(c?.path, "/api/auth");
    assert.equal(c?.maxAge, MFA_CHALLENGE_MAX_AGE);
    assert.equal(MFA_CHALLENGE_MAX_AGE, 300);
  });

  it("clearMfaChallengeCookie menghapus dengan atribut yang sama (path harus cocok agar peramban menghapusnya)", () => {
    const res = baru();
    clearMfaChallengeCookie(res);
    const c = res.cookies.get(MFA_CHALLENGE_COOKIE);
    assert.equal(c?.value, "");
    assert.equal(c?.maxAge, 0);
    assert.equal(c?.path, "/api/auth");
    assert.equal(c?.httpOnly, true);
    assert.equal(c?.sameSite, "strict");
  });
});

describe("clearTokenCookies (keluar)", () => {
  it("mengosongkan access dan refresh dengan path dan atribut yang sama dengan saat dipasang", () => {
    const res = baru();
    clearTokenCookies(res);

    const akses = res.cookies.get(ACCESS_COOKIE);
    assert.equal(akses?.value, "");
    assert.equal(akses?.maxAge, 0);
    assert.equal(akses?.path, "/");
    assert.equal(akses?.sameSite, "lax");

    const segar = res.cookies.get(REFRESH_COOKIE);
    assert.equal(segar?.value, "");
    assert.equal(segar?.maxAge, 0);
    assert.equal(
      segar?.path,
      "/api/auth",
      "path berbeda berarti peramban menyimpan cookie refresh lama",
    );
    assert.equal(segar?.sameSite, "strict");
    assert.equal(segar?.httpOnly, true);
  });

  it("header Set-Cookie sungguhan memuat Max-Age=0 dan Path untuk keduanya", () => {
    const res = baru();
    clearTokenCookies(res);
    const baris = res.headers.getSetCookie();
    assert.equal(baris.length, 2);
    assert.ok(
      baris.some(
        (b) => b.startsWith("garda_refresh=;") && /Path=\/api\/auth/.test(b),
      ),
    );
    for (const b of baris) assert.match(b, /Max-Age=0/i);
  });
});

describe("flag Secure mengikuti COOKIE_SECURE", () => {
  it("COOKIE_SECURE=false: semua cookie sesi tanpa Secure (akses http:// untuk demo lokal)", () => {
    process.env.COOKIE_SECURE = "false";
    const res = baru();
    setAccessTokenCookie(res, "a");
    setRefreshTokenCookie(res, "r");
    setMfaChallengeCookie(res, "m");
    clearTokenCookies(res);
    clearMfaChallengeCookie(res);
    for (const header of res.headers.getSetCookie())
      assert.doesNotMatch(header, /;\s*Secure/i, header);
  });

  it("COOKIE_SECURE=true: semua cookie, termasuk yang menghapus, memakai Secure", () => {
    const res = baru();
    setAccessTokenCookie(res, "a");
    setRefreshTokenCookie(res, "r");
    setMfaChallengeCookie(res, "m");
    clearTokenCookies(res);
    clearMfaChallengeCookie(res);
    for (const header of res.headers.getSetCookie())
      assert.match(header, /;\s*Secure/i, header);
  });
});

describe("pembaca cookie sisi server", () => {
  const cookie = `${ACCESS_COOKIE}=akses-1; ${REFRESH_COOKIE}=segar-1; ${MFA_CHALLENGE_COOKIE}=mfa-1; lain=x`;

  it("membaca access, refresh, dan tantangan MFA dari permintaan yang sedang dilayani", async () => {
    const hasil = await denganCookie(cookie, async () => ({
      akses: await getAccessToken(),
      segar: await getRefreshToken(),
      mfa: await getMfaChallenge(),
    }));
    assert.deepEqual(hasil, {
      akses: "akses-1",
      segar: "segar-1",
      mfa: "mfa-1",
    });
  });

  it("undefined bila cookie tidak ada (bukan string kosong)", async () => {
    const hasil = await denganCookie("lain=x", async () => ({
      akses: await getAccessToken(),
      segar: await getRefreshToken(),
      mfa: await getMfaChallenge(),
    }));
    assert.deepEqual(hasil, {
      akses: undefined,
      segar: undefined,
      mfa: undefined,
    });
  });

  it("tidak tertukar: hanya cookie bernama persis yang dibaca", async () => {
    const akses = await denganCookie(
      "garda_access_palsu=x; xgarda_access=y",
      () => getAccessToken(),
    );
    assert.equal(akses, undefined);
  });
});
