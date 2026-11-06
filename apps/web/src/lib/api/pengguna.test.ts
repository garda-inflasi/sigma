import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { HALAMAN_PENGGUNA, lanjutHalamanPengguna, masihTerkunci, penggunaApi, queryDaftarPengguna, toPenggunaDetail, toPenggunaRow, type UserResponse } from "./pengguna";

const dasar: UserResponse = {
  id: "USR-1",
  nama: "Siti Rahma",
  email: "siti@gardainflasi.id",
  peran: "MANAJER_DATA",
  lapis: "MANAJERIAL",
  lembaga: "BPS Aceh",
  kabupaten_kode: "1171",
  mfa_aktif: true,
  status: "AKTIF",
  terakhir_masuk: "2026-10-16T08:00:00Z",
  dibuat_pada: "2026-01-05T08:00:00Z",
};

describe("toPenggunaDetail", () => {
  it("memuat baris daftar ditambah kabupaten dan waktu dibuat", () => {
    const d = toPenggunaDetail(dasar);
    assert.equal(d.id, "USR-1");
    assert.equal(d.role, "MANAJER_DATA");
    assert.equal(d.mfa, true);
    assert.equal(d.terakhirMasuk, "2026-10-16T08:00:00Z");
    assert.equal(d.kabupatenKode, "1171");
    assert.equal(d.dibuatPada, "2026-01-05T08:00:00Z");
  });

  it("pengguna tanpa kabupaten berlingkup seluruh Aceh", () => {
    const d = toPenggunaDetail({ ...dasar, kabupaten_kode: undefined, terakhir_masuk: undefined });
    assert.equal(d.wilayah, "Seluruh Aceh");
    assert.equal(d.kabupatenKode, undefined);
    assert.equal(d.terakhirMasuk, "");
  });

  it("peran dan status tak dikenal jatuh ke nilai aman", () => {
    const d = toPenggunaDetail({ ...dasar, peran: "ASING", status: "ANEH" });
    assert.equal(d.role, "OPS_MAHASISWA");
    assert.equal(d.status, "MENUNGGU");
  });
});

describe("kunci login (lockout)", () => {
  const SEKARANG = Date.UTC(2026, 10, 5, 10, 0, 0);

  it("terkunci_sampai dari API menjadi terkunciSampai di baris dan detail", () => {
    const terkunci = { ...dasar, terkunci_sampai: "2026-11-05T10:15:00Z" };
    assert.equal(toPenggunaRow(terkunci).terkunciSampai, "2026-11-05T10:15:00Z");
    assert.equal(toPenggunaDetail(terkunci).terkunciSampai, "2026-11-05T10:15:00Z");
    assert.equal(toPenggunaRow(dasar).terkunciSampai, undefined);
  });

  it("masihTerkunci hanya benar selama kunci belum lewat", () => {
    assert.equal(masihTerkunci("2026-11-05T10:15:00Z", SEKARANG), true);
    assert.equal(masihTerkunci("2026-11-05T10:00:00Z", SEKARANG), false, "tepat berakhir bukan terkunci");
    assert.equal(masihTerkunci("2026-11-05T09:00:00Z", SEKARANG), false);
    assert.equal(masihTerkunci(undefined, SEKARANG), false);
    assert.equal(masihTerkunci("bukan tanggal", SEKARANG), false);
  });

  it("bukaKunci memanggil POST /api/pengguna/{id}/buka-kunci dengan id ter-encode", async () => {
    const panggilan: { url: string; init: RequestInit }[] = [];
    const asli = globalThis.fetch;
    globalThis.fetch = (async (url: RequestInfo | URL, init?: RequestInit) => {
      panggilan.push({ url: String(url), init: init ?? {} });
      return Response.json({ id: "a/b", terkunci_sebelumnya: true });
    }) as typeof fetch;
    try {
      const hasil = await penggunaApi.bukaKunci("a/b");
      assert.deepEqual(hasil, { id: "a/b", terkunci_sebelumnya: true });
      assert.equal(panggilan.length, 1);
      assert.equal(panggilan[0].url, "/api/pengguna/a%2Fb/buka-kunci");
      assert.equal(panggilan[0].init.method, "POST");
    } finally {
      globalThis.fetch = asli;
    }
  });
});

describe("daftar pengguna dimuat seluruh halamannya", () => {
  it("meminta halaman berukuran maksimum dan meneruskan pencarian", () => {
    assert.equal(queryDaftarPengguna(1), `page=1&limit=${HALAMAN_PENGGUNA}`);
    assert.equal(queryDaftarPengguna(3, "aulia rahman"), `page=3&limit=${HALAMAN_PENGGUNA}&search=aulia+rahman`);
  });

  it("berhenti tepat setelah halaman terakhir", () => {
    assert.equal(lanjutHalamanPengguna({ items: new Array(100), total: 250, page: 1, limit: 100 }), true);
    assert.equal(lanjutHalamanPengguna({ items: new Array(100), total: 250, page: 2, limit: 100 }), true);
    assert.equal(lanjutHalamanPengguna({ items: new Array(50), total: 250, page: 3, limit: 100 }), false);
    assert.equal(lanjutHalamanPengguna({ items: new Array(100), total: 100, page: 1, limit: 100 }), false);
  });

  it("halaman kosong menghentikan perulangan walau total tidak konsisten", () => {
    assert.equal(lanjutHalamanPengguna({ items: [], total: 500, page: 2, limit: 100 }), false);
  });
});
