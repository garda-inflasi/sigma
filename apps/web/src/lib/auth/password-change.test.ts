import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  isPasswordChangeExempt,
  postLoginDestination,
  requiresPasswordChange,
} from "./password-change";
import { evaluatePassword, isPasswordValid } from "./password-policy";

function jwtWith(payload: object): string {
  const b64 = (o: object) =>
    Buffer.from(JSON.stringify(o)).toString("base64url");
  return `${b64({ alg: "HS256" })}.${b64(payload)}.tanda-tangan`;
}

describe("requiresPasswordChange", () => {
  it("true bila klaim wajib_ganti_password aktif", () => {
    assert.equal(
      requiresPasswordChange(jwtWith({ wajib_ganti_password: true })),
      true,
    );
  });

  it("false bila klaim tidak ada atau bukan true", () => {
    assert.equal(
      requiresPasswordChange(jwtWith({ role: "ADMIN_SISTEM" })),
      false,
    );
    assert.equal(
      requiresPasswordChange(jwtWith({ wajib_ganti_password: "true" })),
      false,
    );
  });

  it("false untuk token kosong atau rusak", () => {
    assert.equal(requiresPasswordChange(undefined), false);
    assert.equal(requiresPasswordChange("bukan-jwt"), false);
    assert.equal(requiresPasswordChange("a.%%%.c"), false);
  });
});

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
