import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { EntriHarga } from "@/lib/data/types";
import {
  filterMockEntri,
  HARGA_PAGE_LIMIT,
  ringkasanDariEntri,
  ringkasSebab,
  toEntriHarga,
  toDetailHarga,
  toRiwayatStatus,
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

describe("ringkasSebab", () => {
  it("menjumlahkan entri yang dilewati menurut sebabnya", () => {
    assert.equal(
      ringkasSebab([
        { id: "a", sebab: "status_tidak_sesuai" },
        { id: "b", sebab: "pemisahan_tugas" },
        { id: "c", sebab: "status_tidak_sesuai" },
      ]),
      "2 status tidak sesuai, 1 dicatat oleh Anda sendiri",
    );
  });

  it("kosong bila tidak ada yang dilewati", () => {
    assert.equal(ringkasSebab(undefined), "");
    assert.equal(ringkasSebab([]), "");
  });
});

describe("toRiwayatStatus", () => {
  it("memetakan perpindahan status dan membuang null", () => {
    assert.deepEqual(
      toRiwayatStatus({ dari: null, ke: "DIAJUKAN", oleh: "mahasiswa_1", oleh_nama: "Aulia Rahman", waktu: "2026-10-16T02:00:00Z", alasan: null }),
      { dari: undefined, ke: "DIAJUKAN", olehNama: "Aulia Rahman", waktu: "2026-10-16T02:00:00Z", alasan: undefined },
    );
    assert.equal(toRiwayatStatus({ dari: "DIAJUKAN", ke: "DITOLAK", oleh: null, oleh_nama: null, waktu: "w", alasan: "bukti kurang" }).alasan, "bukti kurang");
  });
});

describe("toEntriHarga alasan penolakan", () => {
  it("membawa alasan penolakan bila ada", () => {
    const base = { id: "e1", komoditas_kode: "BWM", komoditas_nama: "Bawang merah", pasar_id: "P", pasar_nama: "Pasar", kabupaten_kode: "1171", wilayah: "KOTA BANDA ACEH", tipe: "KONSUMEN", harga: 1, satuan: "kg", kualitas: "MEDIUM", petugas_id: "u", petugas_nama: "U", dicatat_pada: "2026-10-16T02:00:00Z", status: "DITOLAK", deviasi_pct: 0, skor_anomali: 0, ada_foto: false, dalam_geofence: true } as const;
    assert.equal(toEntriHarga({ ...base, alasan_penolakan: "satuan salah" }).alasanPenolakan, "satuan salah");
    assert.equal(toEntriHarga({ ...base, alasan_penolakan: null }).alasanPenolakan, undefined);
  });
});

describe("toDetailHarga", () => {
  const dasar = { ...respons, lat: 5.56, lng: 95.32, foto_url: null };
  const bersih = { di_luar_rentang: false, outlier: false, duplikat: false, luar_geofence: false, tanpa_lokasi: false, bentrok: false };

  it("mengurutkan bendera yang menyala dari yang paling berat", () => {
    const detail = toDetailHarga({ ...dasar, flag_validasi: { ...bersih, luar_geofence: true, di_luar_rentang: true, outlier: true, median: 52000, jarak_meter: 2200 } });

    assert.deepEqual(detail.bendera.map((b) => b.kode), ["di_luar_rentang", "outlier", "luar_geofence"]);
    assert.equal(detail.median, 52000);
    assert.equal(detail.jarakMeter, 2200);
    assert.equal(detail.lat, 5.56);
    assert.equal(detail.fotoUrl, undefined);
  });

  it("entri bersih tidak punya bendera", () => {
    assert.deepEqual(toDetailHarga({ ...dasar, lat: null, lng: null, flag_validasi: bersih }), { lat: undefined, lng: undefined, fotoUrl: undefined, median: undefined, jarakMeter: undefined, bendera: [] });
  });
});

describe("urut skor", () => {
  it("diteruskan ke query dan diterapkan pada data mock", () => {
    assert.equal(new URLSearchParams(toHargaQuery({ urut: "skor" }).slice(1)).get("urut"), "skor");
    const rows = [0.1, 0.9, 0.4].map((skorAnomali, i) => ({ ...toEntriHarga(respons), id: `e${i}`, skorAnomali }));
    assert.deepEqual(filterMockEntri(rows, { urut: "skor" }).map((e) => e.id), ["e1", "e2", "e0"]);
  });
});
