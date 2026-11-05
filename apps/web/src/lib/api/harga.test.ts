import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { EntriHarga } from "@/lib/data/types";
import {
  filterMockEntri,
  HARGA_PAGE_LIMIT,
  ringkasanDariEntri,
  toEntriHarga,
  toHargaQuery,
  type EntriHargaResponse,
} from "./harga";

const respons: EntriHargaResponse = {
  id: "HRG-D00-00-04-K",
  komoditas_kode: "CBM-KRT",
  komoditas_nama: "Cabai merah keriting",
  pasar_id: "PSR-1108-01",
  pasar_nama: "Pasar Induk Lambaro",
  kabupaten_kode: "1108",
  wilayah: "KABUPATEN ACEH BESAR",
  tipe: "KONSUMEN",
  harga: 61500,
  satuan: "kg",
  kualitas: "MEDIUM",
  petugas_id: "mahasiswa_2",
  petugas_nama: "Cut Nanda",
  dicatat_pada: "2026-10-16T05:12:00Z",
  status: "DIAJUKAN",
  deviasi_pct: 3.2,
  skor_anomali: 0.1,
  ada_foto: true,
  dalam_geofence: false,
};

describe("toEntriHarga", () => {
  it("memetakan kolom API ke baris UI dengan label wilayah ringkas", () => {
    assert.deepEqual(toEntriHarga(respons), {
      id: "HRG-D00-00-04-K",
      komoditasKode: "CBM-KRT",
      komoditasNama: "Cabai merah keriting",
      pasarId: "PSR-1108-01",
      pasarNama: "Pasar Induk Lambaro",
      kabupatenKode: "1108",
      wilayah: "Kab. Aceh Besar",
      tipe: "KONSUMEN",
      harga: 61500,
      satuan: "kg",
      kualitas: "MEDIUM",
      petugasId: "mahasiswa_2",
      petugasNama: "Cut Nanda",
      dicatatPada: "2026-10-16T05:12:00Z",
      status: "DIAJUKAN",
      deviasiPct: 3.2,
      skorAnomali: 0.1,
      adaFoto: true,
      dalamGeofence: false,
    });
  });

  it("petugas yang akunnya hilang menjadi string kosong", () => {
    const row = toEntriHarga({
      ...respons,
      petugas_id: null,
      petugas_nama: null,
    });
    assert.equal(row.petugasId, "");
    assert.equal(row.petugasNama, "");
  });
});

describe("toHargaQuery", () => {
  it("selalu menyertakan batas halaman", () => {
    assert.equal(toHargaQuery(), `?limit=${HARGA_PAGE_LIMIT}`);
  });

  it("memakai nama parameter kontrak API dan membuang nilai kosong", () => {
    const query = toHargaQuery({
      status: "DIAJUKAN",
      wilayah: "1171",
      komoditas: "BWM",
      dari: "2026-10-01",
      sampai: "",
      page: 2,
      limit: 50,
    });
    const qs = new URLSearchParams(query.slice(1));
    assert.deepEqual(Object.fromEntries(qs), {
      status: "DIAJUKAN",
      wilayah: "1171",
      komoditas: "BWM",
      dari: "2026-10-01",
      page: "2",
      limit: "50",
    });
  });
});

const baris = (
  id: string,
  status: EntriHarga["status"],
  deviasiPct: number,
  kabupatenKode = "1171",
): EntriHarga => ({
  ...toEntriHarga(respons),
  id,
  status,
  deviasiPct,
  kabupatenKode,
});

describe("data mock", () => {
  const rows = [
    baris("a", "DIAJUKAN", 2),
    baris("b", "DIAJUKAN", -30, "1108"),
    baris("c", "DIVALIDASI", 26),
    baris("d", "DITOLAK", 0),
  ];

  it("filterMockEntri menyaring menurut status dan wilayah serta menghormati limit", () => {
    assert.deepEqual(
      filterMockEntri(rows, { status: "DIAJUKAN" }).map((e) => e.id),
      ["a", "b"],
    );
    assert.deepEqual(
      filterMockEntri(rows, { wilayah: "1108" }).map((e) => e.id),
      ["b"],
    );
    assert.equal(filterMockEntri(rows, { limit: 1 }).length, 1);
  });

  it("ringkasanDariEntri menghitung per status dan anomali di atas 25%", () => {
    assert.deepEqual(ringkasanDariEntri(rows), {
      draf: 0,
      diajukan: 2,
      divalidasi: 1,
      dipublikasi: 0,
      ditolak: 1,
      anomali: 2,
    });
  });
});
