import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  checkRateLimit,
  getRateLimitKey,
  ukurBatasLaju,
  ukurBatasLogin,
} from "./rate-limit";

const T0 = 1_700_000_000_000;
let urutan = 0;
const kunciBaru = () => `uji:${++urutan}`;

describe("ukurBatasLaju", () => {
  it("mengizinkan sampai batas lalu menolak dengan lama tunggu", () => {
    const kunci = kunciBaru();
    for (let i = 0; i < 5; i++) {
      assert.deepEqual(ukurBatasLaju(kunci, 5, 60_000, T0 + i * 1000), {
        diizinkan: true,
        tungguDetik: 0,
      });
    }
    // Entri tertua (T0) habis pada T0+60s; permintaan di T0+10s menunggu 50 detik.
    assert.deepEqual(ukurBatasLaju(kunci, 5, 60_000, T0 + 10_000), {
      diizinkan: false,
      tungguDetik: 50,
    });
  });

  it("permintaan yang ditolak tidak dicatat dan tidak memperpanjang tunggu", () => {
    const kunci = kunciBaru();
    for (let i = 0; i < 5; i++) ukurBatasLaju(kunci, 5, 60_000, T0);
    const pertama = ukurBatasLaju(kunci, 5, 60_000, T0 + 1000);
    const kedua = ukurBatasLaju(kunci, 5, 60_000, T0 + 1000);
    assert.equal(pertama.tungguDetik, 59);
    assert.equal(kedua.tungguDetik, 59);
    assert.equal(ukurBatasLaju(kunci, 5, 60_000, T0 + 60_000).diizinkan, true);
  });

  it("membulatkan lama tunggu ke atas dan minimal 1 detik", () => {
    const kunci = kunciBaru();
    for (let i = 0; i < 3; i++) ukurBatasLaju(kunci, 3, 60_000, T0);
    assert.equal(ukurBatasLaju(kunci, 3, 60_000, T0 + 59_500).tungguDetik, 1);
    assert.equal(ukurBatasLaju(kunci, 3, 60_000, T0 + 59_999).tungguDetik, 1);
  });

  it("jendela yang dipakai bersama dengan batas lebih kecil menunggu entri yang tepat", () => {
    // Login (5) dan lupa kata sandi (3) memakai kunci yang sama: 4 entri tercatat, batas 3 harus menunggu 2 entri habis.
    const kunci = kunciBaru();
    for (let i = 0; i < 4; i++)
      ukurBatasLaju(kunci, 5, 60_000, T0 + i * 10_000);
    const hasil = ukurBatasLaju(kunci, 3, 60_000, T0 + 40_000);
    assert.equal(hasil.diizinkan, false);
    // Entri ke-(4-3)=indeks 1 (T0+10s) harus habis: 10s + 60s - 40s = 30 detik.
    assert.equal(hasil.tungguDetik, 30);
  });

  it("kunci berbeda tidak saling mempengaruhi", () => {
    const a = kunciBaru();
    const b = kunciBaru();
    for (let i = 0; i < 5; i++) ukurBatasLaju(a, 5, 60_000, T0);
    assert.equal(ukurBatasLaju(a, 5, 60_000, T0).diizinkan, false);
    assert.equal(ukurBatasLaju(b, 5, 60_000, T0).diizinkan, true);
  });

  it("checkRateLimit tetap mengembalikan boolean", () => {
    const kunci = kunciBaru();
    assert.equal(checkRateLimit(kunci, 1, 60_000), true);
    assert.equal(checkRateLimit(kunci, 1, 60_000), false);
  });
});

describe("ukurBatasLogin", () => {
  it("5 per menit per IP dari X-Forwarded-For", () => {
    const req = () =>
      new Request("http://localhost/api/auth/login", {
        method: "POST",
        headers: { "x-forwarded-for": "198.51.100.77, 10.0.0.1" },
      });
    assert.equal(getRateLimitKey(req()), "bff:198.51.100.77");
    for (let i = 0; i < 5; i++)
      assert.equal(ukurBatasLogin(req()).diizinkan, true);
    const ditolak = ukurBatasLogin(req());
    assert.equal(ditolak.diizinkan, false);
    assert.ok(ditolak.tungguDetik >= 1 && ditolak.tungguDetik <= 60);
  });
});
