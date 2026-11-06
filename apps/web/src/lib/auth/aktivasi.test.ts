import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  HALAMAN_AKTIVASI,
  ambilTokenAktivasi,
  galatAktivasi,
  skemaAktivasi,
} from "./aktivasi";
import { PESAN_KONFIRMASI_BEDA, PESAN_SANDI_LEMAH } from "./skema-sandi";

describe("ambilTokenAktivasi", () => {
  it("mengambil token dari query, membuang spasi", () => {
    assert.equal(ambilTokenAktivasi(" AbC-123_xyz "), "AbC-123_xyz");
    assert.equal(ambilTokenAktivasi(["pertama", "kedua"]), "pertama");
  });

  it("token 43 karakter (base64url 256-bit) diterima; kosong, hilang, atau tak masuk akal ditolak", () => {
    assert.equal(ambilTokenAktivasi("a".repeat(43)), "a".repeat(43));
    assert.equal(ambilTokenAktivasi(undefined), null);
    assert.equal(ambilTokenAktivasi("   "), null);
    assert.equal(ambilTokenAktivasi([]), null);
    assert.equal(ambilTokenAktivasi("x".repeat(500)), null);
  });
});

describe("halaman aktivasi", () => {
  it("path sama dengan tautan di email (email.AktivasiPath di API Go)", () => {
    assert.equal(HALAMAN_AKTIVASI, "/aktivasi");
  });
});

describe("skemaAktivasi", () => {
  const sah = "Rahasia#2026Kuat";

  it("kata sandi yang memenuhi kebijakan dan konfirmasi sama lolos", () => {
    const hasil = skemaAktivasi.safeParse({ baru: sah, konfirmasi: sah });
    assert.equal(hasil.success, true);
  });

  it("kata sandi yang belum memenuhi aturan ditolak dengan pesan kebijakan", () => {
    for (const lemah of [
      "",
      "pendek1!",
      "semuakecil1!",
      "TANPAANGKA!!",
      "Tanpa1Khusus",
    ]) {
      const hasil = skemaAktivasi.safeParse({ baru: lemah, konfirmasi: lemah });
      assert.equal(hasil.success, false, lemah);
      assert.equal(hasil.error?.issues[0].message, PESAN_SANDI_LEMAH, lemah);
      assert.deepEqual(hasil.error?.issues[0].path, ["baru"]);
    }
  });

  it("konfirmasi berbeda ditolak pada field konfirmasi", () => {
    const hasil = skemaAktivasi.safeParse({ baru: sah, konfirmasi: `${sah}x` });
    assert.equal(hasil.success, false);
    assert.equal(hasil.error?.issues[0].message, PESAN_KONFIRMASI_BEDA);
    assert.deepEqual(hasil.error?.issues[0].path, ["konfirmasi"]);
  });
});

describe("galatAktivasi", () => {
  it("401 berarti tautan tak bisa dipakai lagi: pesan server dipertahankan dan tautanBaru true", () => {
    const hasil = galatAktivasi(
      401,
      "Tautan undangan sudah kedaluwarsa. Minta administrator mengirim undangan baru.",
    );
    assert.equal(
      hasil.pesan,
      "Tautan undangan sudah kedaluwarsa. Minta administrator mengirim undangan baru.",
    );
    assert.equal(hasil.tautanBaru, true);
    assert.equal(galatAktivasi(401, undefined).tautanBaru, true);
    assert.match(
      galatAktivasi(401, undefined).pesan,
      /tidak valid atau sudah tidak berlaku/,
    );
  });

  it("400 (kebijakan kata sandi) menampilkan pesan server tanpa menandai tautan hangus", () => {
    const hasil = galatAktivasi(
      400,
      "kata sandi terlalu lemah: harus mengandung angka",
    );
    assert.equal(
      hasil.pesan,
      "kata sandi terlalu lemah: harus mengandung angka",
    );
    assert.equal(hasil.tautanBaru, false);
  });

  it("429 membawa sisa detik untuk hitung mundur; galat server dijelaskan ramah", () => {
    const hasil = galatAktivasi(429, "x", { kode: "RATE_LIMITED", detik: 30 });
    assert.equal(hasil.kode, "RATE_LIMITED");
    assert.ok(hasil.sampaiMs !== undefined && hasil.sampaiMs > Date.now());
    assert.equal(hasil.tautanBaru, false);
    assert.match(galatAktivasi(500, "x").pesan, /Tidak dapat terhubung/);
    assert.match(
      galatAktivasi(undefined, undefined).pesan,
      /Tidak dapat terhubung/,
    );
  });
});
