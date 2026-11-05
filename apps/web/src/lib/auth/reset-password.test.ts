import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { ambilToken, galatReset, validasiFormReset } from "./reset-password";

describe("ambilToken", () => {
  it("mengambil token dari query, membuang spasi", () => {
    assert.equal(ambilToken(" 3f1c-aa "), "3f1c-aa");
    assert.equal(ambilToken(["pertama", "kedua"]), "pertama");
  });

  it("menolak token kosong, hilang, atau tak masuk akal", () => {
    assert.equal(ambilToken(undefined), null);
    assert.equal(ambilToken("   "), null);
    assert.equal(ambilToken([]), null);
    assert.equal(ambilToken("x".repeat(500)), null);
  });
});

describe("validasiFormReset", () => {
  it("menolak kata sandi yang melanggar kebijakan", () => {
    assert.match(validasiFormReset("lemah", "lemah") ?? "", /aturan/);
  });

  it("menolak konfirmasi yang berbeda", () => {
    assert.match(
      validasiFormReset("SandiBaru#2026", "SandiBaru#2027") ?? "",
      /Konfirmasi/,
    );
  });

  it("menerima kata sandi sah dengan konfirmasi sama", () => {
    assert.equal(validasiFormReset("SandiBaru#2026", "SandiBaru#2026"), null);
  });
});

describe("galatReset", () => {
  it("401 berarti tautan tak bisa dipakai lagi dan menawarkan tautan baru", () => {
    assert.deepEqual(
      galatReset(
        401,
        "Tautan atur ulang sudah kedaluwarsa. Minta tautan baru.",
      ),
      {
        pesan: "Tautan atur ulang sudah kedaluwarsa. Minta tautan baru.",
        tautanBaru: true,
      },
    );
    assert.equal(galatReset(401, undefined).tautanBaru, true);
  });

  it("galat isian (400) menampilkan pesan server tanpa menawarkan tautan baru", () => {
    assert.deepEqual(
      galatReset(400, "kata sandi terlalu lemah: harus mengandung angka"),
      {
        pesan: "kata sandi terlalu lemah: harus mengandung angka",
        tautanBaru: false,
      },
    );
  });

  it("429 dan galat server dijelaskan ramah", () => {
    assert.match(galatReset(429, "x").pesan, /Terlalu banyak/);
    assert.match(galatReset(500, "x").pesan, /Tidak dapat terhubung/);
    assert.match(
      galatReset(undefined, undefined).pesan,
      /Tidak dapat terhubung/,
    );
  });
});
