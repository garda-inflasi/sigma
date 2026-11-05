import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  masukanBatasLaju,
  masukanDariGalatApi,
  responsBatasLaju,
  responsGalat,
  susunJawabanGalat,
  tanggapiGalatApi,
} from "./galat-auth";
import { handleRouteError } from "./handler";

const SEKARANG = Date.UTC(2026, 10, 5, 10, 0, 0);

/** Galat seperti BffApiError dari lib/bff/api-client (tanpa mengimpor modul server-only). */
function galatApi(
  status: number,
  message: string,
  code?: string,
  detik?: number,
) {
  return Object.assign(new Error(message), { status, code, detik });
}

describe("susunJawabanGalat", () => {
  it("akun terkunci: status 423, kode diteruskan, sisa detik, dan Retry-After", () => {
    const j = susunJawabanGalat(
      { status: 423, kode: "ACCOUNT_LOCKED", detik: 892, pesan: "teks Go" },
      "masuk",
      SEKARANG,
    );
    assert.equal(j.status, 423);
    assert.equal(j.retryAfter, 892);
    assert.deepEqual(j.body, {
      error: true,
      message:
        "Akun terkunci karena terlalu banyak percobaan gagal. Coba lagi dalam 14 menit 52 detik.",
      kode: "ACCOUNT_LOCKED",
      sisa_detik: 892,
    });
  });

  it("kredensial salah: 401 tanpa hitung mundur", () => {
    const j = susunJawabanGalat(
      { status: 401, kode: "INVALID_CREDENTIALS" },
      "masuk",
      SEKARANG,
    );
    assert.deepEqual(j.body, {
      error: true,
      message: "Email atau password salah.",
      kode: "INVALID_CREDENTIALS",
    });
    assert.equal(j.retryAfter, undefined);
  });

  it("kode numerik amplop Go tidak diteruskan sebagai kode; kode string tak dikenal diteruskan apa adanya", () => {
    const numerik = susunJawabanGalat(
      { status: 400, kode: "400", pesan: "Validasi gagal" },
      "masuk",
      SEKARANG,
    );
    assert.equal("kode" in numerik.body, false);
    const asing = susunJawabanGalat(
      { status: 403, kode: "MFA_SETUP_REQUIRED", pesan: "Aktifkan MFA" },
      "ganti",
      SEKARANG,
    );
    assert.equal(asing.body.kode, "MFA_SETUP_REQUIRED");
    assert.equal(asing.body.message, "Aktifkan MFA");
  });

  it("status tidak diketahui menjadi 500 dengan pesan umum", () => {
    const j = susunJawabanGalat({}, "masuk", SEKARANG);
    assert.equal(j.status, 500);
    assert.match(j.body.message, /Tidak dapat terhubung/);
  });

  it("sesi OTP berakhir: ulangMasuk", () => {
    const j = susunJawabanGalat(
      { status: 401, kode: "MFA_CHALLENGE_EXPIRED" },
      "mfa",
      SEKARANG,
    );
    assert.equal(j.galat.ulangMasuk, true);
    assert.equal(j.body.kode, "MFA_CHALLENGE_EXPIRED");
  });
});

describe("responsGalat dan batas laju BFF", () => {
  it("429 memuat body, header Retry-After, dan header keamanan", async () => {
    const res = responsBatasLaju(37, "masuk");
    assert.equal(res.status, 429);
    assert.equal(res.headers.get("Retry-After"), "37");
    assert.equal(res.headers.get("X-Content-Type-Options"), "nosniff");
    const body = await res.json();
    assert.equal(body.error, true);
    assert.equal(body.kode, "RATE_LIMITED");
    assert.equal(body.sisa_detik, 37);
    assert.match(
      body.message,
      /Terlalu banyak percobaan dari perangkat ini\. Coba lagi dalam 37 detik\./,
    );
  });

  it("423 dari API Go diteruskan lengkap", async () => {
    const res = tanggapiGalatApi(
      galatApi(423, "Akun terkunci", "ACCOUNT_LOCKED", 120),
      "masuk",
    );
    assert.equal(res.status, 423);
    assert.equal(res.headers.get("Retry-After"), "120");
    const body = await res.json();
    assert.equal(body.kode, "ACCOUNT_LOCKED");
    assert.equal(body.sisa_detik, 120);
    assert.match(body.message, /Coba lagi dalam 2 menit\./);
  });

  it("galat tanpa waktu tunggu tidak mengirim Retry-After", async () => {
    const res = responsGalat(
      susunJawabanGalat({ status: 401, kode: "INVALID_CREDENTIALS" }, "masuk"),
    );
    assert.equal(res.headers.get("Retry-After"), null);
    assert.equal("sisa_detik" in (await res.json()), false);
  });

  it("masukanBatasLaju memberi 429 RATE_LIMITED", () => {
    assert.deepEqual(masukanBatasLaju(5), {
      status: 429,
      kode: "RATE_LIMITED",
      detik: 5,
    });
  });

  it("exception bukan dari API menjadi 500", async () => {
    assert.deepEqual(masukanDariGalatApi(new Error("boom")), {
      status: 500,
      pesan: "Kesalahan server internal.",
    });
    const res = tanggapiGalatApi("aneh", "masuk");
    assert.equal(res.status, 500);
  });
});

describe("handleRouteError (rute BFF lain)", () => {
  it("meneruskan kode mesin dan sisa waktu dari API Go", async () => {
    const res = handleRouteError(
      galatApi(
        403,
        "Anda harus mengganti kata sandi",
        "PASSWORD_CHANGE_REQUIRED",
      ),
    );
    assert.equal(res.status, 403);
    assert.equal((await res.json()).kode, "PASSWORD_CHANGE_REQUIRED");

    const terkunci = handleRouteError(
      galatApi(423, "Akun terkunci", "ACCOUNT_LOCKED", 60),
    );
    assert.equal(terkunci.headers.get("Retry-After"), "60");
  });

  it("kode numerik tidak ikut", async () => {
    const res = handleRouteError(
      galatApi(404, "Pengguna tidak ditemukan", "404"),
    );
    assert.equal("kode" in (await res.json()), false);
  });
});
