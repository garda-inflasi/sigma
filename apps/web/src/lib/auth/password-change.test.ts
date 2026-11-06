import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  isPasswordChangeExempt,
  postLoginDestination,
} from "./password-change";
import { evaluatePassword, isPasswordValid } from "./password-policy";



describe("isPasswordChangeExempt", () => {
  it("membuka halaman ganti password dan BFF auth", () => {
    assert.equal(isPasswordChangeExempt("/ganti-password"), true);
    assert.equal(isPasswordChangeExempt("/api/auth/change-password"), true);
    assert.equal(isPasswordChangeExempt("/api/auth/logout"), true);
  });

  it("menutup rute lain", () => {
    assert.equal(isPasswordChangeExempt("/pengguna"), false);
    assert.equal(isPasswordChangeExempt("/api/pengguna"), false);
    assert.equal(isPasswordChangeExempt("/api/v1/harga"), false);
  });

  it("mencocokkan menurut batas segmen: anak path terbuka, awalan tanpa garis miring tidak", () => {
    assert.equal(isPasswordChangeExempt("/ganti-password/"), true);
    assert.equal(isPasswordChangeExempt("/ganti-password/lanjut"), true);
    assert.equal(isPasswordChangeExempt("/ganti-passwordX"), false);
    assert.equal(isPasswordChangeExempt("/ganti-password-apa-saja"), false);
    assert.equal(isPasswordChangeExempt("/ganti-passwords/x"), false);
    assert.equal(isPasswordChangeExempt("/api/authX"), false);
    assert.equal(isPasswordChangeExempt("/api/auth-lain/x"), false);
    assert.equal(isPasswordChangeExempt("/api/authentik"), false);
  });
});

describe("postLoginDestination", () => {
  it("mengarahkan ke tujuan semula bila tidak wajib ganti", () => {
    assert.equal(postLoginDestination(false, "/harga"), "/harga");
  });

  it("mengarahkan ke ganti password sambil membawa tujuan semula", () => {
    assert.equal(
      postLoginDestination(true, "/harga?x=1"),
      "/ganti-password?lanjut=%2Fharga%3Fx%3D1",
    );
  });
});

describe("kebijakan kata sandi", () => {
  it("menerima sandi yang memenuhi semua aturan", () => {
    assert.equal(isPasswordValid("SandiBaru#2026"), true);
  });

  it("melaporkan aturan yang belum terpenuhi", () => {
    const belum = (p: string) =>
      evaluatePassword(p)
        .filter((r) => !r.terpenuhi)
        .map((r) => r.id);
    assert.deepEqual(belum("abc"), ["panjang", "kapital", "angka", "khusus"]);
    assert.deepEqual(belum("abcdefgh1!"), ["kapital"]);
    assert.deepEqual(belum("Admin123"), ["khusus", "umum"]);
  });

  it("menolak sandi terlalu umum meski lolos aturan lain", () => {
    assert.equal(
      evaluatePassword("Password").find((r) => r.id === "umum")?.terpenuhi,
      false,
    );
    assert.equal(
      evaluatePassword("").find((r) => r.id === "umum")?.terpenuhi,
      false,
    );
  });
});
