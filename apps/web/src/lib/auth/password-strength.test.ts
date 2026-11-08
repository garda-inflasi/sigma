import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { evaluatePassword, isPasswordValid } from "./password-policy";
import {
  LABEL_KEKUATAN,
  PANJANG_SANDI_KUAT,
  SKOR_KEKUATAN,
  kekuatanSandi,
} from "./password-strength";

describe("kekuatanSandi", () => {
  it("kosong tidak punya kekuatan (meter disembunyikan)", () => {
    assert.equal(kekuatanSandi(""), null);
  });

  it("belum memenuhi semua aturan kebijakan: lemah, berapa pun panjangnya", () => {
    for (const sandi of [
      "a",
      "abc",
      // Panjang cukup tetapi tanpa kapital, angka, dan karakter khusus.
      "abcdefghijklmnopqrstuvwxyz",
      // Tanpa karakter khusus.
      "Abcdefg1",
      "Abcdefghijkl1",
      // Tanpa angka.
      "Abcdefgh!",
      // Tanpa kapital.
      "abcdefg1!",
      // Satu karakter kurang dari batas kebijakan (8).
      "Abcde1!",
    ]) {
      assert.equal(kekuatanSandi(sandi), "lemah", sandi);
    }
  });

  it("kata sandi umum: lemah (aturan 'bukan kata sandi umum' tidak terpenuhi)", () => {
    for (const sandi of [
      "password",
      "Password",
      "12345678",
      "admin123",
      "ACEHKUKAYA",
    ]) {
      // Daftar tolak tidak membedakan huruf besar-kecil.
      assert.equal(
        evaluatePassword(sandi).find((a) => a.id === "umum")?.terpenuhi,
        false,
        `aturan umum untuk ${sandi}`,
      );
      assert.equal(kekuatanSandi(sandi), "lemah", sandi);
    }
  });

  it("memenuhi semua aturan dan kurang dari 12 karakter: sedang", () => {
    // Tepat 8 (batas kebijakan) sampai 11 karakter.
    for (const sandi of [
      "Abcdef1!",
      "Abcdefg1!",
      "Abcdefgh1!",
      "Abcdefghi1!",
    ]) {
      assert.equal(isPasswordValid(sandi), true, sandi);
      assert.ok(sandi.length < PANJANG_SANDI_KUAT, sandi);
      assert.equal(kekuatanSandi(sandi), "sedang", sandi);
    }
  });

  it("memenuhi semua aturan dan sekurangnya 12 karakter: kuat", () => {
    for (const sandi of [
      "Abcdefghij1!",
      "Abcdefghijk1!",
      "Garda-Inflasi-2026",
    ]) {
      assert.equal(isPasswordValid(sandi), true, sandi);
      assert.ok(sandi.length >= PANJANG_SANDI_KUAT, sandi);
      assert.equal(kekuatanSandi(sandi), "kuat", sandi);
    }
  });

  it("batas tepat di 12 karakter: 11 sedang, 12 kuat", () => {
    const sebelas = "Abcdefghi1!";
    const duaBelas = "Abcdefghij1!";
    assert.equal(sebelas.length, PANJANG_SANDI_KUAT - 1);
    assert.equal(duaBelas.length, PANJANG_SANDI_KUAT);
    assert.equal(kekuatanSandi(sebelas), "sedang");
    assert.equal(kekuatanSandi(duaBelas), "kuat");
  });

  it("sandi panjang tanpa angka tetap lemah: panjang tidak menutupi aturan yang gagal", () => {
    assert.equal(kekuatanSandi("Kata-Sandi-Yang-Sangat-Panjang!"), "lemah");
  });

  it("selaras dengan kebijakan: lemah tepat bila isPasswordValid false (tidak ada kata sandi tertolak yang sedang/kuat)", () => {
    const contoh = [
      "x",
      "password",
      "Password1",
      "Password1!",
      "P@ss1",
      "Sandi#2026",
      "sandi#2026",
      "SANDI#2026",
      "Sandi 2026 baru",
      "Ünïcödé#12345",
      "        ",
      "12345678!",
      "Abcdefgh1!xyz",
    ];
    for (const sandi of contoh) {
      assert.equal(
        kekuatanSandi(sandi) === "lemah",
        !isPasswordValid(sandi),
        sandi,
      );
    }
  });

  it("skor dan label tiap tingkat: 1/2/3 dan Lemah/Sedang/Kuat", () => {
    assert.deepEqual(SKOR_KEKUATAN, { lemah: 1, sedang: 2, kuat: 3 });
    assert.deepEqual(LABEL_KEKUATAN, {
      lemah: "Lemah",
      sedang: "Sedang",
      kuat: "Kuat",
    });
  });
});
