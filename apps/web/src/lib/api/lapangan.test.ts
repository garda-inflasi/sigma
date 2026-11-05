import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { toTugasLapangan } from "./lapangan";

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
