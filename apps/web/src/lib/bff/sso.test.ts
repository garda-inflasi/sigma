import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";

import { NextResponse } from "next/server";

import {
  bacaParameterCallback,
  clearSsoFlowCookie,
  decodeFlow,
  encodeFlow,
  galatSsoDariApi,
  galatSsoDariPenyedia,
  lanjutAman,
  setSsoFlowCookie,
  SSO_FLOW_MAX_AGE,
  ssoFlowCookie,
  tujuanPenyedia,
  urlLangkahOtp,
  urlMasukGalat,
} from "./sso";

const BINDING = "q3Zk9vT2mXb7LpR4nYc1Hd5FgJ8sWe0uAaB6CiDkEoM"; // 43 karakter base64url

describe("lanjutAman", () => {
  it("meneruskan path internal biasa, lengkap dengan query", () => {
    assert.equal(lanjutAman("/harga"), "/harga");
    assert.equal(
      lanjutAman("/harga?wilayah=1171&x=1"),
      "/harga?wilayah=1171&x=1",
    );
    assert.equal(lanjutAman("/"), "/");
  });

  it("kosong atau tidak ada -> /", () => {
    assert.equal(lanjutAman(""), "/");
    assert.equal(lanjutAman(null), "/");
    assert.equal(lanjutAman(undefined), "/");
  });

  it("menolak open redirect: URL absolut, skema ganda, backslash, dan karakter kontrol", () => {
    for (const jahat of [
      "https://evil.example/phish",
      "http://evil.example",
      "//evil.example",
      "//evil.example/path",
      "/\\evil.example",
      "javascript:alert(1)",
      "data:text/html,<script>1</script>",
      "/ok\r\nSet-Cookie: a=b",
      "/ok\u0000",
      "evil.example",
    ]) {
      assert.equal(lanjutAman(jahat), "/", jahat);
    }
  });

  it("menolak rute /api/ (bukan halaman) dan nilai terlalu panjang", () => {
    assert.equal(lanjutAman("/api/auth/sso/google"), "/");
    assert.equal(lanjutAman("/api/v1/pengguna"), "/");
    assert.equal(lanjutAman("/api"), "/");
    assert.equal(lanjutAman(`/${"a".repeat(600)}`), "/");
    // Awalan mirip tetapi bukan /api/ tetap diterima.
    assert.equal(lanjutAman("/apiary"), "/apiary");
  });
});

describe("urlMasukGalat dan urlLangkahOtp", () => {
  it("galat tanpa tujuan khusus", () => {
    assert.equal(urlMasukGalat("sso_gagal"), "/masuk?error=sso_gagal");
    assert.equal(urlMasukGalat("sso_gagal", "/"), "/masuk?error=sso_gagal");
  });

  it("galat membawa tujuan semula (aman) agar bisa dicoba lagi", () => {
    assert.equal(
      urlMasukGalat("sso_dibatalkan", "/harga?x=1"),
      `/masuk?error=sso_dibatalkan&lanjut=${encodeURIComponent("/harga?x=1")}`,
    );
  });

  it("tujuan berbahaya dibuang dari URL galat", () => {
    assert.equal(
      urlMasukGalat("sso_gagal", "//evil.example"),
      "/masuk?error=sso_gagal",
    );
  });

  it("langkah OTP sama bentuknya dengan login kata sandi", () => {
    assert.equal(urlLangkahOtp("/"), "/masuk?mfa=1&lanjut=%2F");
    assert.equal(urlLangkahOtp("/harga"), "/masuk?mfa=1&lanjut=%2Fharga");
    assert.equal(
      urlLangkahOtp("https://evil.example"),
      "/masuk?mfa=1&lanjut=%2F",
    );
  });
});

describe("cookie alur", () => {
  it("memakai awalan __Host- (path /) saat Secure dan nama sempit saat http", () => {
    assert.deepEqual(ssoFlowCookie(true), {
      name: "__Host-garda_sso",
      path: "/",
    });
    assert.deepEqual(ssoFlowCookie(false), {
      name: "garda_sso",
      path: "/api/auth/sso/google",
    });
  });

  it("encode lalu decode mengembalikan binding dan tujuan", () => {
    const nilai = encodeFlow(BINDING, "/harga?x=1&y=é");
    assert.ok(nilai.startsWith(`${BINDING}.`));
    assert.deepEqual(decodeFlow(nilai), {
      binding: BINDING,
      lanjut: "/harga?x=1&y=é",
    });
  });

  it("encode membersihkan tujuan berbahaya", () => {
    assert.deepEqual(decodeFlow(encodeFlow(BINDING, "https://evil.example")), {
      binding: BINDING,
      lanjut: "/",
    });
  });

  it("decode menyanitasi ulang tujuan karena cookie dapat dipalsukan pengguna", () => {
    const palsu = `${BINDING}.${Buffer.from("//evil.example", "utf8").toString("base64url")}`;
    assert.deepEqual(decodeFlow(palsu), { binding: BINDING, lanjut: "/" });
  });

  it("decode menolak bentuk tidak sah", () => {
    assert.equal(decodeFlow(undefined), null);
    assert.equal(decodeFlow(null), null);
    assert.equal(decodeFlow(""), null);
    assert.equal(decodeFlow(BINDING), null, "tanpa pemisah");
    assert.equal(
      decodeFlow(`pendek.${Buffer.from("/").toString("base64url")}`),
      null,
      "binding terlalu pendek",
    );
    assert.equal(
      decodeFlow(`${BINDING}!!.Lw`),
      null,
      "karakter binding tidak sah",
    );
    assert.equal(
      decodeFlow(`${"a".repeat(129)}.Lw`),
      null,
      "binding terlalu panjang",
    );
    assert.equal(
      decodeFlow(`${BINDING}.${"A".repeat(2000)}`),
      null,
      "cookie terlalu besar",
    );
  });

  it("decode tetap berhasil dengan tujuan bawaan bila bagian tujuan rusak", () => {
    assert.deepEqual(decodeFlow(`${BINDING}.%%%`), {
      binding: BINDING,
      lanjut: "/",
    });
    assert.deepEqual(decodeFlow(`${BINDING}.`), {
      binding: BINDING,
      lanjut: "/",
    });
  });
});

describe("setSsoFlowCookie dan clearSsoFlowCookie", () => {
  const semula = process.env.COOKIE_SECURE;
  afterEach(() => {
    if (semula === undefined) delete process.env.COOKIE_SECURE;
    else process.env.COOKIE_SECURE = semula;
  });

  it("httpOnly, SameSite=Lax (callback datang lintas situs), umur 10 menit, path sempit di http", () => {
    process.env.COOKIE_SECURE = "false";
    const response = new NextResponse(null);
    setSsoFlowCookie(response, BINDING, "/harga");

    const cookie = response.cookies.get("garda_sso");
    assert.ok(cookie, "cookie terpasang");
    assert.equal(cookie.httpOnly, true);
    assert.equal(
      cookie.sameSite,
      "lax",
      "Strict tidak ikut pada navigasi dari Google",
    );
    assert.equal(cookie.maxAge, SSO_FLOW_MAX_AGE);
    assert.equal(SSO_FLOW_MAX_AGE, 600);
    assert.equal(cookie.path, "/api/auth/sso/google");
    assert.notEqual(cookie.secure, true);
    assert.deepEqual(decodeFlow(cookie.value), {
      binding: BINDING,
      lanjut: "/harga",
    });
  });

  it("Secure memakai __Host-, path / dan tanpa Domain", () => {
    process.env.COOKIE_SECURE = "true";
    const response = new NextResponse(null);
    setSsoFlowCookie(response, BINDING, "/");
    const cookie = response.cookies.get("__Host-garda_sso");
    assert.ok(cookie);
    assert.equal(cookie.secure, true);
    assert.equal(cookie.path, "/");
    assert.equal(cookie.domain, undefined);
    assert.equal(cookie.httpOnly, true);
    assert.equal(cookie.sameSite, "lax");
  });

  it("clear menghapus dengan atribut yang sama (maxAge 0)", () => {
    process.env.COOKIE_SECURE = "false";
    const response = new NextResponse(null);
    clearSsoFlowCookie(response);
    const cookie = response.cookies.get("garda_sso");
    assert.ok(cookie);
    assert.equal(cookie.value, "");
    assert.equal(cookie.maxAge, 0);
    assert.equal(cookie.path, "/api/auth/sso/google");
    assert.equal(cookie.httpOnly, true);
  });
});

describe("bacaParameterCallback", () => {
  const q = (s: string) => new URLSearchParams(s);

  it("mengambil code dan state", () => {
    assert.deepEqual(
      bacaParameterCallback(q("code=abc&state=xyz&scope=openid")),
      {
        code: "abc",
        state: "xyz",
      },
    );
  });

  it("menolak yang hilang, kosong, atau tidak wajar panjangnya", () => {
    assert.equal(bacaParameterCallback(q("")), null);
    assert.equal(bacaParameterCallback(q("code=abc")), null);
    assert.equal(bacaParameterCallback(q("state=xyz")), null);
    assert.equal(bacaParameterCallback(q("code=&state=xyz")), null);
    assert.equal(
      bacaParameterCallback(q(`code=${"c".repeat(2049)}&state=x`)),
      null,
    );
    assert.equal(
      bacaParameterCallback(q(`code=c&state=${"s".repeat(129)}`)),
      null,
    );
  });
});

describe("pemetaan galat", () => {
  it("galat dari penyedia: penolakan izin = dibatalkan, selain itu gagal", () => {
    assert.equal(galatSsoDariPenyedia("access_denied"), "sso_dibatalkan");
    assert.equal(galatSsoDariPenyedia("server_error"), "sso_gagal");
    assert.equal(galatSsoDariPenyedia("temporarily_unavailable"), "sso_gagal");
    assert.equal(galatSsoDariPenyedia(null), "sso_gagal");
  });

  it("kode amplop API dipetakan satu-satu", () => {
    const peta: Record<string, string> = {
      SSO_NOT_CONFIGURED: "sso_tidak_aktif",
      SSO_UNAVAILABLE: "sso_tidak_tersedia",
      SSO_STATE_INVALID: "sso_sesi_tidak_valid",
      SSO_EXCHANGE_FAILED: "sso_verifikasi_gagal",
      SSO_EMAIL_UNVERIFIED: "sso_email_belum_terverifikasi",
      SSO_DOMAIN_NOT_ALLOWED: "sso_domain_ditolak",
      SSO_ACCOUNT_NOT_FOUND: "sso_akun_tidak_ditemukan",
      SSO_ACCOUNT_CONFLICT: "sso_akun_bentrok",
      SSO_ACCOUNT_INACTIVE: "sso_akun_nonaktif",
      SSO_ACCOUNT_LOCKED: "sso_akun_terkunci",
    };
    for (const [kode, harap] of Object.entries(peta)) {
      assert.equal(galatSsoDariApi({ status: 403, code: kode }), harap, kode);
    }
  });

  it("tanpa kode: 429 terlalu banyak, 5xx dan galat jaringan tidak tersedia, 4xx lain gagal", () => {
    assert.equal(galatSsoDariApi({ status: 429 }), "sso_terlalu_banyak");
    assert.equal(galatSsoDariApi({ status: 500 }), "sso_tidak_tersedia");
    assert.equal(galatSsoDariApi({ status: 502 }), "sso_tidak_tersedia");
    assert.equal(galatSsoDariApi({ status: 400, code: "400" }), "sso_gagal");
    assert.equal(
      galatSsoDariApi(new TypeError("fetch failed")),
      "sso_tidak_tersedia",
    );
    assert.equal(
      galatSsoDariApi(new DOMException("timeout", "TimeoutError")),
      "sso_tidak_tersedia",
    );
    assert.equal(galatSsoDariApi(null), "sso_tidak_tersedia");
  });

  it("kode amplop tak dikenal (termasuk nama prototipe) tidak dipetakan", () => {
    assert.equal(
      galatSsoDariApi({ status: 400, code: "constructor" }),
      "sso_gagal",
    );
    assert.equal(
      galatSsoDariApi({ status: 400, code: "__proto__" }),
      "sso_gagal",
    );
    assert.equal(
      galatSsoDariApi({ status: 400, code: "SSO_KARANGAN" }),
      "sso_gagal",
    );
  });
});

describe("tujuanPenyedia", () => {
  it("hanya mengikuti http(s)", () => {
    assert.equal(
      tujuanPenyedia(
        "https://accounts.google.com/o/oauth2/v2/auth?client_id=x",
      ),
      "https://accounts.google.com/o/oauth2/v2/auth?client_id=x",
    );
    assert.ok(
      tujuanPenyedia("http://sso.localhost:8089/default/authorize?state=1"),
    );
    assert.equal(tujuanPenyedia("javascript:alert(1)"), null);
    assert.equal(tujuanPenyedia("data:text/html,x"), null);
    assert.equal(tujuanPenyedia("/relatif"), null);
    assert.equal(tujuanPenyedia(""), null);
  });
});
