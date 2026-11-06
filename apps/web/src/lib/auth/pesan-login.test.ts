import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  adalahKodeDikenal,
  bagianPesan,
  formatSisaSingkat,
  formatSisaWaktu,
  galatAuth,
  KODE_GALAT,
  masihMenunggu,
  masukanDariError,
  PESAN_GALAT,
  PESAN_JARINGAN,
  PESAN_TUNGGU_SELESAI,
  pesanTampil,
  sisaDetik,
} from "./pesan-login";

const SEKARANG = Date.UTC(2026, 10, 5, 10, 0, 0);

describe("formatSisaWaktu", () => {
  it("detik, menit, dan jam dalam bahasa Indonesia", () => {
    assert.equal(formatSisaWaktu(0), "0 detik");
    assert.equal(formatSisaWaktu(1), "1 detik");
    assert.equal(formatSisaWaktu(59), "59 detik");
    assert.equal(formatSisaWaktu(60), "1 menit");
    assert.equal(formatSisaWaktu(61), "1 menit 1 detik");
    assert.equal(formatSisaWaktu(14 * 60 + 52), "14 menit 52 detik");
    assert.equal(formatSisaWaktu(15 * 60), "15 menit");
    assert.equal(formatSisaWaktu(3600), "1 jam");
    assert.equal(formatSisaWaktu(3600 + 5 * 60 + 7), "1 jam 5 menit");
  });

  it("membulatkan pecahan ke atas dan menolak nilai negatif", () => {
    assert.equal(formatSisaWaktu(0.2), "1 detik");
    assert.equal(formatSisaWaktu(-5), "0 detik");
  });
});

describe("formatSisaSingkat", () => {
  it("ringkas untuk lencana tabel", () => {
    assert.equal(formatSisaSingkat(45), "45 dtk");
    assert.equal(formatSisaSingkat(60), "1 mnt");
    assert.equal(formatSisaSingkat(61), "2 mnt");
    assert.equal(formatSisaSingkat(14 * 60 + 52), "15 mnt");
    assert.equal(formatSisaSingkat(3600), "1 jam");
    assert.equal(formatSisaSingkat(3600 + 5 * 60), "1 jam 5 mnt");
    assert.equal(formatSisaSingkat(3600 + 59 * 60 + 30), "2 jam");
  });
});

describe("sisaDetik", () => {
  it("dibulatkan ke atas dan tidak pernah negatif", () => {
    assert.equal(sisaDetik(SEKARANG + 1500, SEKARANG), 2);
    assert.equal(sisaDetik(SEKARANG + 1000, SEKARANG), 1);
    assert.equal(sisaDetik(SEKARANG, SEKARANG), 0);
    assert.equal(sisaDetik(SEKARANG - 5000, SEKARANG), 0);
  });
});

describe("galatAuth: kode stabil dari server", () => {
  it("setiap kode dikenal punya pesan tunggal dari tabel", () => {
    for (const kode of Object.values(KODE_GALAT)) {
      assert.equal(adalahKodeDikenal(kode), true);
      assert.equal(
        galatAuth({ kode }, "masuk", SEKARANG).pesan,
        PESAN_GALAT[kode],
      );
    }
    assert.equal(adalahKodeDikenal("401"), false);
    assert.equal(adalahKodeDikenal(undefined), false);
  });

  it("kredensial salah: pesan generik tanpa membedakan penyebab", () => {
    const g = galatAuth(
      { status: 401, kode: "INVALID_CREDENTIALS", pesan: "apa saja" },
      "masuk",
      SEKARANG,
    );
    assert.equal(g.pesan, "Email atau password salah.");
    assert.equal(g.sampaiMs, undefined);
    assert.equal(pesanTampil(g, SEKARANG), "Email atau password salah.");
  });

  it("akun terkunci memuat akhir kunci dan teks sisa waktu", () => {
    const g = galatAuth(
      { status: 423, kode: "ACCOUNT_LOCKED", detik: 892 },
      "masuk",
      SEKARANG,
    );
    assert.equal(g.sampaiMs, SEKARANG + 892_000);
    assert.equal(
      pesanTampil(g, SEKARANG),
      "Akun terkunci karena terlalu banyak percobaan gagal. Coba lagi dalam 14 menit 52 detik.",
    );
    assert.equal(
      pesanTampil(g, SEKARANG + 100_000),
      "Akun terkunci karena terlalu banyak percobaan gagal. Coba lagi dalam 13 menit 12 detik.",
    );
    assert.equal(masihMenunggu(g, SEKARANG), true);
  });

  it("hitung mundur selesai mengganti pesan dan membebaskan form", () => {
    const g = galatAuth(
      { kode: "ACCOUNT_LOCKED", detik: 30 },
      "masuk",
      SEKARANG,
    );
    assert.equal(pesanTampil(g, SEKARANG + 30_000), PESAN_TUNGGU_SELESAI);
    assert.equal(masihMenunggu(g, SEKARANG + 30_000), false);
    assert.equal(masihMenunggu(null, SEKARANG), false);
  });

  it("batas laju BFF memuat detik dari Retry-After", () => {
    const g = galatAuth(
      { status: 429, kode: "RATE_LIMITED", detik: 42 },
      "masuk",
      SEKARANG,
    );
    assert.equal(g.kode, "RATE_LIMITED");
    assert.equal(
      pesanTampil(g, SEKARANG),
      "Terlalu banyak percobaan dari perangkat ini. Coba lagi dalam 42 detik.",
    );
  });

  it("tanpa detik, pesan tunggu tetap jelas", () => {
    const g = galatAuth({ status: 429 }, "masuk", SEKARANG);
    assert.equal(g.kode, "RATE_LIMITED");
    assert.equal(g.sampaiMs, undefined);
    assert.equal(
      pesanTampil(g, SEKARANG),
      "Terlalu banyak percobaan dari perangkat ini. Coba lagi nanti.",
    );
  });

  it("detik nol atau negatif tidak membuat hitung mundur", () => {
    assert.equal(
      galatAuth({ kode: "ACCOUNT_LOCKED", detik: 0 }, "masuk", SEKARANG)
        .sampaiMs,
      undefined,
    );
    assert.equal(
      galatAuth({ kode: "ACCOUNT_LOCKED", detik: -3 }, "masuk", SEKARANG)
        .sampaiMs,
      undefined,
    );
  });

  it("akun nonaktif, MFA, dan wajib ganti kata sandi", () => {
    assert.equal(
      galatAuth({ kode: "ACCOUNT_INACTIVE" }, "masuk").pesan,
      "Akun Anda tidak aktif. Hubungi administrator.",
    );
    const sesi = galatAuth({ kode: "MFA_CHALLENGE_EXPIRED" }, "mfa");
    assert.equal(sesi.ulangMasuk, true);
    assert.match(sesi.pesan, /Masuk ulang/);
    const kode = galatAuth({ kode: "MFA_CODE_INVALID" }, "mfa");
    assert.equal(kode.ulangMasuk, false);
    assert.match(kode.pesan, /Kode verifikasi tidak valid/);
    assert.match(
      galatAuth({ kode: "PASSWORD_CHANGE_REQUIRED", status: 403 }, "ganti")
        .pesan,
      /mengganti kata sandi/,
    );
  });

  it("kode stabil mengalahkan status dan konteks", () => {
    assert.equal(
      galatAuth({ status: 401, kode: "ACCOUNT_LOCKED" }, "masuk").kode,
      "ACCOUNT_LOCKED",
    );
    assert.equal(
      galatAuth({ status: 401, kode: "MFA_CHALLENGE_EXPIRED" }, "masuk")
        .ulangMasuk,
      true,
    );
  });
});

describe("galatAuth: server lama tanpa kode", () => {
  it("menyimpulkan dari status dan konteks", () => {
    assert.equal(
      galatAuth({ status: 401 }, "masuk").kode,
      "INVALID_CREDENTIALS",
    );
    assert.equal(galatAuth({ status: 403 }, "masuk").kode, "ACCOUNT_INACTIVE");
    assert.equal(galatAuth({ status: 423 }, "masuk").kode, "ACCOUNT_LOCKED");
    assert.equal(galatAuth({ status: 429 }, "mfa").kode, "RATE_LIMITED");
    assert.equal(
      galatAuth({ status: 401, pesan: "Kode tidak valid" }, "mfa").kode,
      "MFA_CODE_INVALID",
    );
    assert.equal(
      galatAuth(
        {
          status: 401,
          pesan:
            "sesi verifikasi MFA tidak valid atau sudah kedaluwarsa, masuk ulang",
        },
        "mfa",
      ).kode,
      "MFA_CHALLENGE_EXPIRED",
    );
  });

  it("kode numerik dari amplop Go diabaikan, bukan dianggap kode stabil", () => {
    const g = galatAuth(
      { status: 401, kode: "401", pesan: "Password lama salah" },
      "ganti",
    );
    assert.equal(g.kode, "401");
    assert.equal(g.pesan, "Password lama salah");
  });
});

describe("galatAuth: atur ulang dan galat umum", () => {
  it("tautan reset yang tak berlaku menawarkan tautan baru dan memakai pesan server", () => {
    const g = galatAuth(
      {
        status: 401,
        pesan: "Tautan atur ulang sudah kedaluwarsa. Minta tautan baru.",
      },
      "reset",
    );
    assert.equal(g.tautanBaru, true);
    assert.equal(
      g.pesan,
      "Tautan atur ulang sudah kedaluwarsa. Minta tautan baru.",
    );
    assert.equal(galatAuth({ status: 401 }, "reset").tautanBaru, true);
  });

  it("galat klien lain memakai pesan server, 5xx dan jaringan pesan umum", () => {
    assert.equal(
      galatAuth(
        {
          status: 400,
          pesan: "kata sandi terlalu lemah: harus mengandung angka",
        },
        "reset",
      ).pesan,
      "kata sandi terlalu lemah: harus mengandung angka",
    );
    assert.equal(
      galatAuth({ status: 400 }, "reset").pesan,
      "Permintaan tidak dapat diproses.",
    );
    assert.equal(
      galatAuth({ status: 500, pesan: "x" }, "masuk").pesan,
      PESAN_JARINGAN,
    );
    assert.equal(galatAuth({}, "masuk").pesan, PESAN_JARINGAN);
  });

  it("429 saat atur ulang memakai batas laju yang sama", () => {
    const g = galatAuth(
      { status: 429, kode: "RATE_LIMITED", detik: 10 },
      "reset",
      SEKARANG,
    );
    assert.match(pesanTampil(g, SEKARANG), /Coba lagi dalam 10 detik/);
    assert.equal(g.tautanBaru, false);
  });
});

describe("masukanDariError", () => {
  it("membaca status, kode, dan sisa detik dari payload BFF", () => {
    const err = Object.assign(new Error("Terlalu banyak"), {
      status: 429,
      payload: { error: true, kode: "RATE_LIMITED", sisa_detik: 33 },
    });
    assert.deepEqual(masukanDariError(err), {
      status: 429,
      pesan: "Terlalu banyak",
      kode: "RATE_LIMITED",
      detik: 33,
    });
  });

  it("toleran terhadap galat tanpa payload dan bukan Error", () => {
    assert.deepEqual(masukanDariError(new Error("putus")), {
      status: undefined,
      pesan: "putus",
      kode: undefined,
      detik: undefined,
    });
    assert.deepEqual(masukanDariError("aneh"), {});
  });
});

describe("bagianPesan", () => {
  it("sebelum waktu diketahui (render server) hitung mundur ditunda", () => {
    const g = galatAuth({ kode: "ACCOUNT_LOCKED", detik: 90 }, "masuk", SEKARANG);
    assert.deepEqual(bagianPesan(g, null), {
      teks: "Akun terkunci karena terlalu banyak percobaan gagal.",
      sisa: null,
    });
    assert.deepEqual(bagianPesan(g, SEKARANG), {
      teks: "Akun terkunci karena terlalu banyak percobaan gagal.",
      sisa: "1 menit 30 detik",
    });
    assert.equal(pesanTampil(g, null), "Akun terkunci karena terlalu banyak percobaan gagal.");
  });

  it("galat biasa tidak pernah punya hitung mundur", () => {
    const g = galatAuth({ kode: "INVALID_CREDENTIALS" }, "masuk");
    assert.deepEqual(bagianPesan(g, null), { teks: "Email atau password salah.", sisa: null });
  });
});
