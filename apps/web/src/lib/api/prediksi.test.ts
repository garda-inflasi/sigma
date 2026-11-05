import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  toBarisModel,
  toBarisProyeksi,
  toProyeksiXai,
  type ProyeksiResponse,
} from "./prediksi";

const respons: ProyeksiResponse = {
  komoditas_kode: "CBM-KRT",
  komoditas_nama: "Cabai merah keriting",
  kabupaten_kode: "1171",
  kabupaten_nama: "KOTA BANDA ACEH",
  horizon_hari: 14,
  harga_kini: 50000,
  perubahan_pct: 14.2,
  bawah_pct: -2.1,
  atas_pct: 30.5,
  seri: [
    { tanggal: "2026-11-04", aktual: 49000 },
    {
      tanggal: "2026-11-05",
      aktual: 50000,
      proyeksi: 50000,
      bawah: 50000,
      atas: 50000,
    },
    { tanggal: "2026-11-19", proyeksi: 57100, bawah: 48950, atas: 65250 },
  ],
  faktor: [{ nama: "Curah hujan", kontribusi: 6.4, detail: "2,1× normal" }],
  narasi: "Cabai naik.",
  model: {
    nama: "Ensemble",
    versi: "v2.3.1",
    mape: 6.8,
    dilatih: "2026-10-29",
  },
  rekomendasi: ["Siapkan operasi pasar"],
  sumber: "DEMO",
};

describe("toProyeksiXai", () => {
  it("memetakan respons ke bentuk panel XAI", () => {
    const x = toProyeksiXai(respons);

    assert.equal(x.komoditasKode, "CBM-KRT");
    assert.equal(x.nama, "Cabai merah keriting");
    assert.equal(x.horizonHari, 14);
    assert.equal(x.perubahanPct, 14.2);
    assert.deepEqual(x.interval, [-2.1, 30.5]);
    assert.equal(x.hargaKini, 50000);
    assert.deepEqual(x.model, respons.model);
    assert.equal(x.demo, true);
  });

  it("seri: riwayat tanpa rentang, titik sambung dan proyeksi dengan rentang", () => {
    const { seri } = toProyeksiXai(respons);

    assert.deepEqual(seri[0], { tanggal: "2026-11-04", aktual: 49000 });
    assert.deepEqual(seri[1], {
      tanggal: "2026-11-05",
      aktual: 50000,
      proyeksi: 50000,
      rentang: [50000, 50000],
    });
    assert.deepEqual(seri[2], {
      tanggal: "2026-11-19",
      proyeksi: 57100,
      rentang: [48950, 65250],
    });
    assert.ok(
      !("proyeksi" in seri[0]),
      "titik riwayat tidak membawa kunci proyeksi",
    );
  });

  it("sumber MODEL bukan demo", () => {
    assert.equal(toProyeksiXai({ ...respons, sumber: "MODEL" }).demo, false);
  });
});

describe("toBarisProyeksi", () => {
  it("memetakan kolom ringkasan ke baris tabel", () => {
    const baris = toBarisProyeksi({
      komoditas_kode: "BWM",
      komoditas_nama: "Bawang merah",
      kabupaten_kode: "1171",
      horizon_hari: 14,
      harga_kini: 41000,
      proyeksi: 43900,
      perubahan_pct: 7.1,
      bawah_pct: -2.9,
      atas_pct: 17.1,
      model_versi: "v2.3.1",
      mape: 5.3,
      sumber: "DEMO",
    });

    assert.deepEqual(baris, {
      id: "BWM",
      nama: "Bawang merah",
      kini: 41000,
      proyeksi: 43900,
      pct: 7.1,
      lo: -2.9,
      hi: 17.1,
      mape: 5.3,
      demo: true,
    });
  });
});

describe("toBarisModel", () => {
  const model = {
    id: "MDL-X",
    komoditas_kode: "X",
    komoditas_nama: "Beras medium",
    nama: "n",
    versi: "v2.3.1",
    mape: 2.2,
    drift: 0.03,
    dilatih_pada: "2026-10-29T00:00:00Z",
    sumber: "DEMO",
  };

  it("memetakan registri", () => {
    assert.deepEqual(toBarisModel({ ...model, status: "KANDIDAT" }), {
      id: "MDL-X",
      komoditas: "Beras medium",
      versi: "v2.3.1",
      mape: 2.2,
      drift: 0.03,
      status: "KANDIDAT",
      demo: true,
    });
  });

  it("status tak dikenal dianggap ARSIP, bukan produksi", () => {
    assert.equal(toBarisModel({ ...model, status: "AKTIF" }).status, "ARSIP");
  });
});
