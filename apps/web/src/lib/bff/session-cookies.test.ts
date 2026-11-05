import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { NextResponse } from "next/server";
import {
  ACCESS_COOKIE,
  CSRF_COOKIE,
  REFRESH_COOKIE,
  REFRESH_COOKIE_PATH,
  RESUME_COOKIE,
  RESUME_COOKIE_MAX_AGE,
} from "./cookie-policy";
import {
  csrfSudahAda,
  pasangSesiBaru,
  setResumeCookie,
} from "./session-cookies";

describe("pasangSesiBaru", () => {
  it("rotasi normal: access, refresh, dan CSRF (bila belum ada) dipasang", async () => {
    const res = NextResponse.json({});
    await pasangSesiBaru(res, { accessToken: "at", refreshToken: "rt" }, false);
    assert.equal(res.cookies.get(ACCESS_COOKIE)?.value, "at");
    assert.equal(res.cookies.get(REFRESH_COOKIE)?.value, "rt");
    assert.ok(res.cookies.get(CSRF_COOKIE)?.value);
  });

  it("refresh token hanya dipasang di path /api/auth, httpOnly dan SameSite=Strict", async () => {
    const res = NextResponse.json({});
    await pasangSesiBaru(res, { accessToken: "at", refreshToken: "rt" }, true);
    const refresh = res.cookies.get(REFRESH_COOKIE);
    assert.equal(refresh?.path, REFRESH_COOKIE_PATH);
    assert.equal(refresh?.httpOnly, true);
    assert.equal(refresh?.sameSite, "strict");
    assert.equal(res.cookies.get(ACCESS_COOKIE)?.path, "/");
  });

  it("pemutaran ulang (tanpa refresh token baru): cookie refresh pemenang tidak ditimpa", async () => {
    const res = NextResponse.json({});
    await pasangSesiBaru(res, { accessToken: "at" }, true);
    assert.equal(res.cookies.get(ACCESS_COOKIE)?.value, "at");
    assert.equal(res.cookies.get(REFRESH_COOKIE), undefined);
  });

  it("CSRF yang sudah ada tidak diganti (tab lain mungkin sedang memakainya)", async () => {
    const res = NextResponse.json({});
    await pasangSesiBaru(res, { accessToken: "at", refreshToken: "rt" }, true);
    assert.equal(res.cookies.get(CSRF_COOKIE), undefined);
  });
});

describe("csrfSudahAda", () => {
  const dari = (nilai?: string) => ({
    cookies: {
      get: () => (nilai === undefined ? undefined : { value: nilai }),
    },
  });
  it("benar hanya bila cookie CSRF ada dan tidak kosong", () => {
    assert.equal(csrfSudahAda(dari("abc")), true);
    assert.equal(csrfSudahAda(dari("")), false);
    assert.equal(csrfSudahAda(dari()), false);
  });
});

describe("setResumeCookie", () => {
  it("penanda singkat httpOnly di path /", () => {
    const res = NextResponse.json({});
    setResumeCookie(res);
    const c = res.cookies.get(RESUME_COOKIE);
    assert.equal(c?.value, "1");
    assert.equal(c?.path, "/");
    assert.equal(c?.httpOnly, true);
    assert.equal(c?.maxAge, RESUME_COOKIE_MAX_AGE);
    assert.ok(RESUME_COOKIE_MAX_AGE <= 60);
  });
});
