import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { toPenggunaDetail, type UserResponse } from "./pengguna";

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
