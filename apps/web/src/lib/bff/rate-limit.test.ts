import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  checkLoginRateLimit,
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

describe("getRateLimitKey", () => {
  const dari = (xff?: string) =>
    new Request("http://localhost/api/auth/login", {
      headers: xff === undefined ? {} : { "x-forwarded-for": xff },
    });

  it("memakai entri paling kiri X-Forwarded-For dan membuang spasi", () => {
    assert.equal(getRateLimitKey(dari("  203.0.113.4 , 10.0.0.1")), "bff:203.0.113.4");
  });

  it("tanpa header (akses langsung ke Next) jatuh ke satu ember bersama 127.0.0.1", () => {
    assert.equal(getRateLimitKey(dari()), "bff:127.0.0.1");
    assert.equal(getRateLimitKey(dari("")), "bff:127.0.0.1");
    assert.equal(getRateLimitKey(dari(" , 10.0.0.1")), "bff:127.0.0.1");
  });
});

describe("checkLoginRateLimit", () => {
  const dari = (ip: string) =>
    new Request("http://localhost/api/auth/login", {
      headers: { "x-forwarded-for": ip },
    });

  it("boolean: lima percobaan pertama lolos, keenam ditolak, IP lain tidak terpengaruh", () => {
    for (let i = 0; i < 5; i++)
      assert.equal(checkLoginRateLimit(dari("198.51.100.201")), true, `ke-${i + 1}`);
    assert.equal(checkLoginRateLimit(dari("198.51.100.201")), false);
    assert.equal(checkLoginRateLimit(dari("198.51.100.202")), true);
  });

  it("login dan validasi OTP berbagi ember yang sama (ukurBatasLogin)", () => {
    const ip = "198.51.100.203";
    for (let i = 0; i < 3; i++) checkLoginRateLimit(dari(ip));
    for (let i = 0; i < 2; i++) assert.equal(ukurBatasLogin(dari(ip)).diizinkan, true);
    assert.equal(
      ukurBatasLogin(dari(ip)).diizinkan,
      false,
      "5 percobaan gabungan sudah habis",
    );
  });
});

describe("ukurBatasLaju: batas ekstrem", () => {
  it("limit 1 menolak segera dan menunggu tepat sampai entri itu habis", () => {
    const kunci = kunciBaru();
    assert.equal(ukurBatasLaju(kunci, 1, 10_000, T0).diizinkan, true);
    const ditolak = ukurBatasLaju(kunci, 1, 10_000, T0 + 2_500);
    assert.deepEqual(ditolak, { diizinkan: false, tungguDetik: 8 });
    assert.equal(ukurBatasLaju(kunci, 1, 10_000, T0 + 10_000).diizinkan, true);
  });

  it("entri tepat di batas jendela sudah dianggap keluar (< bukan <=)", () => {
    const kunci = kunciBaru();
    ukurBatasLaju(kunci, 1, 1_000, T0);
    assert.equal(ukurBatasLaju(kunci, 1, 1_000, T0 + 999).diizinkan, false);
    assert.equal(ukurBatasLaju(kunci, 1, 1_000, T0 + 1_000).diizinkan, true);
  });
});
