import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { cariAcuan, toTugasLapangan, toVersiServer, type DataAcuanResponse } from "./lapangan";

describe("toTugasLapangan", () => {
  it("memetakan kolom API ke tugas UI", () => {
    const tugas = toTugasLapangan({
      id: "TGS-mahasiswa_1-1",
      judul: "Survei harga konsumen pagi",
      pasar_id: "PSR-1171-01",
      pasar_nama: "Pasar Peunayong",
      komoditas: ["CBM-KRT", "BWM"],
      tenggat: "2026-10-16T09:30:00Z",
      status: "BERJALAN",
      target: 6,
      terisi: 4,
      tipe: "KONSUMEN",
    });

    assert.deepEqual(tugas, {
      id: "TGS-mahasiswa_1-1",
      judul: "Survei harga konsumen pagi",
      pasarId: "PSR-1171-01",
      pasarNama: "Pasar Peunayong",
      komoditas: ["CBM-KRT", "BWM"],
      tenggat: "2026-10-16T09:30:00Z",
      status: "BERJALAN",
      target: 6,
      terisi: 4,
      tipe: "KONSUMEN",
    });
  });
});

describe("cariAcuan", () => {
  const data: DataAcuanResponse = {
    versi: "v1",
    toleransi: 0.25,
    komoditas: [{ kode: "CBM-KRT", nama: "Cabai merah keriting", satuan: "kg", harga_acuan: 50000 }],
    pasar: [],
    harga_acuan: [{ komoditas_kode: "CBM-KRT", kabupaten_kode: "1171", tipe: "KONSUMEN", median: 52000, bawah: 39000, atas: 65000, sampel: 8 }],
  };

  it("memakai median resmi kabupaten bila ada", () => {
    assert.deepEqual(cariAcuan(data, "CBM-KRT", "1171", "KONSUMEN"), { median: 52000, bawah: 39000, atas: 65000 });
  });

  it("jatuh ke harga acuan master, produsen 72%, dengan toleransi server", () => {
    assert.deepEqual(cariAcuan(data, "CBM-KRT", "1108", "KONSUMEN"), { median: 50000, bawah: 37500, atas: 62500 });
    assert.deepEqual(cariAcuan(data, "CBM-KRT", "1171", "PRODUSEN"), { median: 36000, bawah: 27000, atas: 45000 });
  });

  it("komoditas tak dikenal tidak punya acuan", () => {
    assert.equal(cariAcuan(data, "XXX", "1171", "KONSUMEN"), undefined);
  });
});

describe("toVersiServer", () => {
  it("membaca versi server dari isi 409", () => {
    const payload = { code: "409", data: { versi_server: { id: "e0", harga: 55000, dicatat_pada: "2026-10-16T01:40:00Z", petugas: "Cut Nanda" } } };
    assert.deepEqual(toVersiServer(payload), { harga: 55000, dicatatPada: "2026-10-16T01:40:00Z", petugas: "Cut Nanda" });
  });

  it("tanpa versi server hasilnya undefined", () => {
    assert.equal(toVersiServer({ code: "409" }), undefined);
    assert.equal(toVersiServer(undefined), undefined);
  });
});
