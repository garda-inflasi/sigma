import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  destinasiSetelahMasuk,
  galatLangkahOtp,
  isMfaSetupExempt,
  rapikanKode,
  requiresMfaSetup,
  teksKodeCadangan,
  validasiKode,
} from "./mfa";

function jwt(payload: object): string {
  const b64 = (o: object) =>
    Buffer.from(JSON.stringify(o)).toString("base64url");
  return `${b64({ alg: "HS256" })}.${b64(payload)}.tanda-tangan`;
}

describe("requiresMfaSetup", () => {
  it("true hanya bila klaim wajib_setup_mfa bernilai true", () => {
    assert.equal(requiresMfaSetup(jwt({ wajib_setup_mfa: true })), true);
    assert.equal(requiresMfaSetup(jwt({ wajib_setup_mfa: false })), false);
    assert.equal(requiresMfaSetup(jwt({ wajib_setup_mfa: "true" })), false);
    assert.equal(requiresMfaSetup(jwt({})), false);
  });

  it("token rusak atau kosong dianggap tidak wajib", () => {
    assert.equal(requiresMfaSetup(undefined), false);
    assert.equal(requiresMfaSetup("bukan-jwt"), false);
    assert.equal(requiresMfaSetup("a.%%%.c"), false);
  });
});

describe("isMfaSetupExempt", () => {
  it("membuka halaman setup, ganti kata sandi, dan API auth saja", () => {
    for (const path of [
      "/mfa",
      "/mfa/setup",
      "/ganti-password",
      "/api/auth/mfa/setup",
      "/api/auth/logout",
    ]) {
      assert.equal(isMfaSetupExempt(path), true, path);
    }
    for (const path of [
      "/operasional",
      "/data/harga",
      "/api/harga",
      "/lapangan/input",
    ]) {
      assert.equal(isMfaSetupExempt(path), false, path);
    }
  });
});

describe("destinasiSetelahMasuk", () => {
  it("tanpa kewajiban langsung ke tujuan", () => {
    assert.equal(
      destinasiSetelahMasuk(
        { wajib_ganti_password: false },
        false,
        "/operasional",
      ),
      "/operasional",
    );
  });

  it("wajib MFA mengalihkan ke /mfa membawa tujuan semula", () => {
    assert.equal(
      destinasiSetelahMasuk({}, true, "/operasional"),
      "/mfa?lanjut=%2Foperasional",
    );
  });

  it("ganti kata sandi didahulukan, lalu MFA", () => {
    const tujuan = destinasiSetelahMasuk(
      { wajib_ganti_password: true },
      true,
      "/operasional",
    );
    assert.equal(
      tujuan,
      `/ganti-password?lanjut=${encodeURIComponent("/mfa?lanjut=%2Foperasional")}`,
    );
  });
});

describe("rapikanKode dan validasiKode", () => {
  it("kode OTP dibersihkan dari spasi", () => {
    assert.equal(rapikanKode(" 123 456 "), "123456");
  });

  it("kode cadangan menjadi huruf besar", () => {
    assert.equal(rapikanKode("abcde-23456"), "ABCDE-23456");
    assert.equal(rapikanKode("backup01"), "BACKUP01");
  });

  it("menolak kosong dan terlalu panjang, selebihnya diserahkan ke server", () => {
    assert.equal(validasiKode("   "), "Kode wajib diisi.");
    assert.equal(validasiKode("x".repeat(40)), "Kode terlalu panjang.");
    assert.equal(validasiKode("123456"), null);
    assert.equal(validasiKode("abcde-23456"), null);
  });
});

describe("galatLangkahOtp", () => {
  it("sesi tantangan yang kedaluwarsa mengharuskan masuk ulang", () => {
    const hasil = galatLangkahOtp(
      401,
      "sesi verifikasi MFA tidak valid atau sudah kedaluwarsa, masuk ulang",
    );
    assert.equal(hasil.ulangMasuk, true);
  });

  it("kode salah tetap di langkah OTP", () => {
    const hasil = galatLangkahOtp(401, "Kode tidak valid");
    assert.equal(hasil.ulangMasuk, false);
    assert.match(hasil.pesan, /Kode tidak valid/);
  });

  it("429 dan galat jaringan dijelaskan ramah", () => {
    assert.match(galatLangkahOtp(429, "x").pesan, /terkunci/);
    assert.match(galatLangkahOtp(500, "x").pesan, /Tidak dapat terhubung/);
    assert.match(
      galatLangkahOtp(undefined, undefined).pesan,
      /Tidak dapat terhubung/,
    );
    assert.equal(
      galatLangkahOtp(400, "Kode tidak valid").pesan,
      "Kode tidak valid",
    );
  });
});

describe("teksKodeCadangan", () => {
  it("memuat satu kode per baris beserta catatan", () => {
    const teks = teksKodeCadangan(
      ["AAAAA-11111", "BBBBB-22222"],
      "a@gardainflasi.id",
    );
    assert.match(teks, /a@gardainflasi\.id/);
    assert.ok(teks.split("\n").includes("AAAAA-11111"));
    assert.ok(teks.split("\n").includes("BBBBB-22222"));
  });
});
