import assert from "node:assert/strict";
import { afterEach, beforeEach, describe, it } from "node:test";
import { NextResponse } from "next/server";
import { ACCESS_COOKIE, CSRF_COOKIE, REFRESH_COOKIE } from "./cookie-policy";
import { adalahSesi, attachSession, tujuanSesi } from "./session-response";

const SECURE_ASLI = process.env.COOKIE_SECURE;
beforeEach(() => {
  process.env.COOKIE_SECURE = "true";
});
afterEach(() => {
  if (SECURE_ASLI === undefined) delete process.env.COOKIE_SECURE;
  else process.env.COOKIE_SECURE = SECURE_ASLI;
});

describe("attachSession", () => {
  it("memasang access, refresh, dan CSRF baru dengan atribut masing-masing", async () => {
    const res = NextResponse.json({});
    await attachSession(res, { access_token: "at", refresh_token: "rt" });

    const akses = res.cookies.get(ACCESS_COOKIE);
    assert.equal(akses?.value, "at");
    assert.equal(akses?.httpOnly, true);
    assert.equal(akses?.path, "/");

    const segar = res.cookies.get(REFRESH_COOKIE);
    assert.equal(segar?.value, "rt");
    assert.equal(segar?.httpOnly, true);
    assert.equal(segar?.sameSite, "strict");
    assert.equal(segar?.path, "/api/auth");

    const csrf = res.cookies.get(CSRF_COOKIE);
    assert.match(csrf?.value ?? "", /^[0-9a-f]{64}$/);
    assert.equal(csrf?.httpOnly, false, "double-submit: harus terbaca skrip");
  });

  it("login baru selalu menerbitkan CSRF baru (tidak memakai ulang token sesi sebelumnya)", async () => {
    const a = NextResponse.json({});
    const b = NextResponse.json({});
    await attachSession(a, { access_token: "at", refresh_token: "rt" });
    await attachSession(b, { access_token: "at", refresh_token: "rt" });
    assert.notEqual(
      a.cookies.get(CSRF_COOKIE)?.value,
      b.cookies.get(CSRF_COOKIE)?.value,
    );
  });

  it("token tidak bocor ke badan atau header selain Set-Cookie", async () => {
    const res = NextResponse.json({ ok: true });
    await attachSession(res, {
      access_token: "RAHASIA-AT",
      refresh_token: "RAHASIA-RT",
    });
    for (const [nama, nilai] of res.headers) {
      // x-middleware-set-cookie adalah cermin internal Next yang dibuang sebelum respons keluar dari server.
      if (["set-cookie", "x-middleware-set-cookie"].includes(nama.toLowerCase()))
        continue;
      assert.doesNotMatch(nilai, /RAHASIA/, nama);
    }
    assert.doesNotMatch(await res.text(), /RAHASIA/);
  });
});

describe("tujuanSesi", () => {
  const biasa = { user: { wajib_ganti_password: false } };

  it("akun biasa langsung ke tujuan", () => {
    assert.equal(tujuanSesi(biasa as never, "/harga"), "/harga");
  });

  it("wajib ganti kata sandi: ke /ganti-password membawa tujuan semula (di-encode)", () => {
    assert.equal(
      tujuanSesi(
        { user: { wajib_ganti_password: true } } as never,
        "/harga?tanggal=2026-01-01",
      ),
      "/ganti-password?lanjut=%2Fharga%3Ftanggal%3D2026-01-01",
    );
  });

  it("wajib setup MFA: ke /mfa membawa tujuan semula", () => {
    assert.equal(
      tujuanSesi({ ...biasa, mfa_setup_diwajibkan: true } as never, "/kpi"),
      "/mfa?lanjut=%2Fkpi",
    );
  });

  it("keduanya wajib: ganti kata sandi dulu, lalu setup MFA, lalu tujuan", () => {
    const hasil = tujuanSesi(
      {
        user: { wajib_ganti_password: true },
        mfa_setup_diwajibkan: true,
      } as never,
      "/kpi",
    );
    assert.equal(
      hasil,
      `/ganti-password?lanjut=${encodeURIComponent("/mfa?lanjut=%2Fkpi")}`,
    );
  });

  it("tujuan berisi karakter pemisah query tidak bisa menyelundupkan parameter", () => {
    const hasil = tujuanSesi(
      { user: { wajib_ganti_password: true } } as never,
      "/a?x=1&lanjut=//evil.com",
    );
    const lanjut = new URL(hasil, "http://localhost").searchParams.get(
      "lanjut",
    );
    assert.equal(
      lanjut,
      "/a?x=1&lanjut=//evil.com",
      "diterima utuh sebagai satu nilai",
    );
  });
});

describe("adalahSesi", () => {
  it("benar hanya bila access token, refresh token, dan user semuanya ada", () => {
    assert.equal(
      adalahSesi({ access_token: "a", refresh_token: "r", user: {} }),
      true,
    );
  });

  it("tantangan MFA (tanpa token) bukan sesi", () => {
    assert.equal(adalahSesi({}), false);
    assert.equal(adalahSesi({ user: {} }), false);
  });

  it("salah satu bagian kosong bukan sesi", () => {
    assert.equal(adalahSesi({ access_token: "a", user: {} }), false);
    assert.equal(adalahSesi({ refresh_token: "r", user: {} }), false);
    assert.equal(
      adalahSesi({ access_token: "", refresh_token: "r", user: {} }),
      false,
    );
    assert.equal(adalahSesi({ access_token: "a", refresh_token: "r" }), false);
  });
});
