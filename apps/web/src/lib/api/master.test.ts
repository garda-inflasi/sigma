import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  ringkasDitolak,
  toMasterBulkHasil,
  toMasterKomoditasRow,
  toMasterPasarRow,
  toMasterWilayahRow,
} from "./master";

describe("toMasterKomoditasRow", () => {
  it("memetakan kolom API ke baris UI", () => {
    const row = toMasterKomoditasRow({
      kode: "CBM-KRT",
      nama: "Cabai merah keriting",
      kategori: "Hortikultura",
      satuan: "kg",
      harga_acuan: 52000,
      pilot: true,
      shelf_life_hari: 21,
      versi: 2,
      status: "MENUNGGU_PERSETUJUAN",
    });

    assert.deepEqual(row, {
      id: "CBM-KRT",
      kode: "CBM-KRT",
      nama: "Cabai merah keriting",
      kategori: "Hortikultura",
      satuan: "kg",
      hargaAcuan: 52000,
      pilot: true,
      versi: 2,
      status: "MENUNGGU_PERSETUJUAN",
    });
  });

  it("status tak dikenal dianggap AKTIF", () => {
    const row = toMasterKomoditasRow({
      kode: "X",
      nama: "X",
      kategori: "k",
      satuan: "kg",
      harga_acuan: 1,
      pilot: false,
      shelf_life_hari: 1,
      versi: 1,
      status: "ARSIP",
    });

    assert.equal(row.status, "AKTIF");
  });
});

describe("toMasterWilayahRow", () => {
  it("nama resmi menjadi label ringkas", () => {
    const kab = toMasterWilayahRow({
      kode: "1108",
      nama: "KABUPATEN ACEH BESAR",
      jenis: "Kabupaten",
      ibukota: "Kota Jantho",
      lat: 5.3,
      lng: 95.63,
      provinsi_kode: "11",
    });
    const kota = toMasterWilayahRow({
      kode: "1173",
      nama: "KOTA LANGSA",
      jenis: "Kota",
      ibukota: "Langsa",
      lat: 4.47,
      lng: 97.97,
      provinsi_kode: "11",
    });

    assert.equal(kab.nama, "Kab. Aceh Besar");
    assert.equal(kota.nama, "Kota Langsa");
    assert.equal(kab.ibukota, "Kota Jantho");
  });
});

describe("toMasterPasarRow", () => {
  const pasar = {
    id: "PSR-1171-01",
    nama: "Pasar Peunayong",
    kabupaten_kode: "1171",
    lat: 5.56,
    lng: 95.32,
    radius_meter: 400,
    aktif: true,
  };

  it("memakai nama kabupaten dari peta", () => {
    const row = toMasterPasarRow(pasar, new Map([["1171", "Kota Banda Aceh"]]));

    assert.equal(row.kabupaten, "Kota Banda Aceh");
    assert.equal(row.radiusMeter, 400);
    assert.equal(row.aktif, true);
  });

  it("kabupaten tak dikenal jatuh ke kode", () => {
    assert.equal(toMasterPasarRow(pasar, new Map()).kabupaten, "1171");
  });
});

describe("toMasterBulkHasil", () => {
  it("memisahkan kode yang ditolak beserta alasannya", () => {
    const hasil = toMasterBulkHasil({
      berhasil: 1,
      gagal: 2,
      rincian: [
        { kode: "A", berhasil: true },
        { kode: "B", berhasil: false, pesan: "Komoditas tidak ditemukan" },
        { kode: "C", berhasil: false },
      ],
    });

    assert.equal(hasil.berhasil, 1);
    assert.equal(hasil.gagal, 2);
    assert.deepEqual(hasil.ditolak, [
      { kode: "B", pesan: "Komoditas tidak ditemukan" },
      { kode: "C", pesan: "Ditolak" },
    ]);
  });

  it("rincian kosong tidak menimbulkan galat", () => {
    assert.deepEqual(toMasterBulkHasil({ berhasil: 0, gagal: 0, rincian: undefined as never }).ditolak, []);
  });
});

describe("ringkasDitolak", () => {
  it("meringkas lebih dari batas", () => {
    const ditolak = ["A", "B", "C", "D", "E"].map((kode) => ({ kode, pesan: "x" }));
    assert.equal(ringkasDitolak(ditolak), "A: x; B: x; C: x; dan 2 lainnya");
  });

  it("kosong menghasilkan teks kosong", () => {
    assert.equal(ringkasDitolak([]), "");
  });
});
