import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  bacaKebijakan,
  bacaMulai,
  bacaParamRiwayat,
  tentukanTujuan,
} from "./impersonasi-masukan";

const U1 = "11111111-1111-4111-8111-111111111111";
const U2 = "22222222-2222-4222-8222-222222222222";

describe("bacaMulai", () => {
  it("akun sasaran: meneruskan id dan alasan yang dipangkas, dan membawa halaman semula terpisah", () => {
    const hasil = bacaMulai({
      target_pengguna_id: U1,
      alasan: "  uji  ",
      dari: "/eksekutif",
      sembarang: "dibuang",
    });
    assert.deepEqual(hasil, {
      ok: true,
      nilai: { masukan: { target_pengguna_id: U1, alasan: "uji" }, dari: "/eksekutif" },
    });
  });

  it("peran sasaran saja sah; alasan kosong dibuang", () => {
    assert.deepEqual(bacaMulai({ target_peran: "OPS_KOORDINATOR", alasan: "  " }), {
      ok: true,
      nilai: { masukan: { target_peran: "OPS_KOORDINATOR" }, dari: undefined },
    });
  });

  it("menolak badan bukan objek, tanpa sasaran, id bukan UUID, peran asing, dan ADMIN_SISTEM", () => {
    for (const badan of [
      null,
      "x",
      [],
      {},
      { alasan: "a" },
      { target_pengguna_id: "USR/1" },
      { target_pengguna_id: 5 },
      { target_pengguna_id: `${U1}x` },
      { target_peran: "SUPERADMIN" },
      { target_peran: 7 },
      { target_peran: "ADMIN_SISTEM" },
    ]) {
      const hasil = bacaMulai(badan);
      assert.equal(hasil.ok, false, JSON.stringify(badan));
    }
  });

  it("alasan bukan teks atau lebih dari 500 karakter ditolak (titik kode, bukan unit UTF-16)", () => {
    assert.equal(bacaMulai({ target_peran: "OPS_MAHASISWA", alasan: 5 }).ok, false);
    assert.equal(
      bacaMulai({ target_peran: "OPS_MAHASISWA", alasan: "a".repeat(501) }).ok,
      false,
    );
    assert.equal(
      bacaMulai({ target_peran: "OPS_MAHASISWA", alasan: "a".repeat(500) }).ok,
      true,
    );
    // 500 emoji = 1000 unit UTF-16 tetapi 500 karakter: sah.
    assert.equal(
      bacaMulai({ target_peran: "OPS_MAHASISWA", alasan: "😀".repeat(500) }).ok,
      true,
    );
    assert.equal(
      bacaMulai({ target_peran: "OPS_MAHASISWA", alasan: "😀".repeat(501) }).ok,
      false,
    );
  });

  it("halaman semula bukan teks ditolak", () => {
    assert.equal(bacaMulai({ target_peran: "OPS_MAHASISWA", dari: 5 }).ok, false);
  });
});

describe("bacaKebijakan", () => {
  const dasar = () => ({
    versi: 2,
    aktif: true,
    wajib_alasan: false,
    durasi_maks_menit: null,
    pelaku: [
      {
        tipe: "PERAN",
        peran: "MANAJER_MUTU",
        aktif: true,
        // field tampilan dari respons dibuang
        pengguna_nama: "x",
        target: [
          { tipe: "PERAN", peran: "OPS_MAHASISWA", pengguna_nama: "y" },
          { tipe: "PENGGUNA", pengguna_id: U1, pengguna_nama: "Zul" },
        ],
      },
      { tipe: "PENGGUNA", pengguna_id: U2, aktif: false, target: [] },
    ],
  });

  it("badan sah diteruskan hanya dengan field yang dikenal", () => {
    assert.deepEqual(bacaKebijakan(dasar()), {
      ok: true,
      nilai: {
        versi: 2,
        aktif: true,
        wajib_alasan: false,
        durasi_maks_menit: null,
        pelaku: [
          {
            tipe: "PERAN",
            peran: "MANAJER_MUTU",
            aktif: true,
            target: [
              { tipe: "PERAN", peran: "OPS_MAHASISWA" },
              { tipe: "PENGGUNA", pengguna_id: U1 },
            ],
          },
          { tipe: "PENGGUNA", pengguna_id: U2, aktif: false, target: [] },
        ],
      },
    });
  });

  it("menolak setiap bagian yang cacat", () => {
    const ubah = (f: (b: ReturnType<typeof dasar>) => void) => {
      const b = dasar();
      f(b);
      return bacaKebijakan(b).ok;
    };
    assert.equal(bacaKebijakan(null).ok, false);
    assert.equal(bacaKebijakan([]).ok, false);
    assert.equal(ubah((b) => ((b as { versi: unknown }).versi = "2")), false);
    assert.equal(ubah((b) => (b.versi = -1)), false);
    assert.equal(ubah((b) => (b.versi = 1.5)), false);
    assert.equal(ubah((b) => ((b as { aktif: unknown }).aktif = "ya")), false);
    assert.equal(ubah((b) => ((b as { wajib_alasan: unknown }).wajib_alasan = 1)), false);
    for (const durasi of [4, 1441, 2.5, "60", undefined]) {
      assert.equal(ubah((b) => ((b as { durasi_maks_menit: unknown }).durasi_maks_menit = durasi)), false, String(durasi));
    }
    assert.equal(ubah((b) => ((b as { durasi_maks_menit: unknown }).durasi_maks_menit = 5)), true);
    assert.equal(ubah((b) => ((b as { pelaku: unknown }).pelaku = {})), false);
    assert.equal(ubah((b) => (b.pelaku[0].aktif = "ya" as never)), false);
    assert.equal(ubah((b) => ((b.pelaku[0] as { target: unknown }).target = null)), false);
    assert.equal(ubah((b) => (b.pelaku[0].peran = "SUPERADMIN")), false);
    assert.equal(ubah((b) => (b.pelaku[0].peran = "ADMIN_SISTEM")), false);
    assert.equal(ubah((b) => (b.pelaku[0].tipe = "KELOMPOK")), false);
    assert.equal(ubah((b) => (b.pelaku[1].pengguna_id = "bukan-uuid")), false);
    assert.equal(ubah((b) => (b.pelaku[0].target[0].peran = "ADMIN_SISTEM")), false);
    assert.equal(ubah((b) => ((b.pelaku[0].target[1] as { pengguna_id: unknown }).pengguna_id = 9)), false);
    assert.equal(ubah((b) => ((b.pelaku as unknown[])[0] = "x")), false);
  });

  it("batas jumlah pelaku dan target", () => {
    const b = dasar();
    b.pelaku = Array.from({ length: 101 }, () => ({
      tipe: "PENGGUNA",
      pengguna_id: U1,
      aktif: false,
      target: [],
    })) as never;
    assert.equal(bacaKebijakan(b).ok, false);
    const c = dasar();
    c.pelaku[0].target = Array.from({ length: 201 }, () => ({ tipe: "PERAN", peran: "OPS_MAHASISWA" })) as never;
    assert.equal(bacaKebijakan(c).ok, false);
  });
});

describe("bacaParamRiwayat", () => {
  const baca = (q: string) => bacaParamRiwayat(new URLSearchParams(q));

  it("meneruskan status, limit, dan cursor yang sah; parameter lain dibuang", () => {
    assert.deepEqual(baca("status=aktif&limit=20&cursor=abc_-9&x=1"), {
      ok: true,
      nilai: "status=aktif&limit=20&cursor=abc_-9",
    });
    assert.deepEqual(baca(""), { ok: true, nilai: "" });
  });

  it("menolak status, limit, dan cursor yang tidak sah", () => {
    for (const q of [
      "status=lainnya",
      "limit=0",
      "limit=101",
      "limit=abc",
      "limit=-1",
      "cursor=a b",
      "cursor=%2F%2E%2E",
      `cursor=${"a".repeat(513)}`,
    ]) {
      assert.equal(baca(q).ok, false, q);
    }
  });
});

describe("tentukanTujuan", () => {
  const izinKpi = ["kpi.lihat"];

  it("halaman semula dipertahankan (beserta query) bila peran baru boleh membukanya", () => {
    assert.equal(tentukanTujuan("/kpi?bulan=2026-11", "OPS_KOORDINATOR", izinKpi), "/kpi?bulan=2026-11");
    assert.equal(tentukanTujuan("/alur-sistem", "OPS_MAHASISWA", []), "/alur-sistem");
  });

  it("halaman semula yang tak boleh dibuka peran baru menjadi /", () => {
    assert.equal(tentukanTujuan("/pengguna", "OPS_MAHASISWA", []), "/");
    assert.equal(tentukanTujuan("/impersonasi", "OPS_KOORDINATOR", izinKpi), "/");
    assert.equal(tentukanTujuan("/kpi", "OPS_MAHASISWA", []), "/");
  });

  it("tanpa klaim izin memakai matriks bawaan peran", () => {
    assert.equal(tentukanTujuan("/pengguna", "ADMIN_SISTEM", null), "/pengguna");
    assert.equal(tentukanTujuan("/pengguna", "OPS_MAHASISWA", null), "/");
  });

  it("tujuan tak aman atau tak bermakna menjadi /", () => {
    for (const dari of [
      undefined,
      "",
      "//evil.example",
      "/\\evil",
      "https://evil.example/x",
      "javascript:alert(1)",
      "/api/auth/me",
      "/masuk",
      "/masuk?lanjut=/kpi",
      "/",
    ]) {
      assert.equal(tentukanTujuan(dari, "ADMIN_SISTEM", null), "/", String(dari));
    }
  });
});
