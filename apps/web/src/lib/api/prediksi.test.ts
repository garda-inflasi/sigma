import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  modelUsang,
  toBarisModel,
  toBarisProyeksi,
  toHasilSimulasi,
  toProyeksiXai,
  type ProyeksiResponse,
  type SimulasiResponse,
} from "./prediksi";
import { mockSimulasi } from "./prediksi-mock";

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
    catatan: "Data contoh seeder.",
    baseline_mape: null,
    mape_realisasi: null,
    n_evaluasi: 0,
  };

  it("memetakan registri", () => {
    assert.deepEqual(toBarisModel({ ...model, status: "KANDIDAT" }), {
      id: "MDL-X",
      komoditasKode: "X",
      komoditas: "Beras medium",
      versi: "v2.3.1",
      mape: 2.2,
      baselineMape: null,
      mapeRealisasi: null,
      nEvaluasi: 0,
      drift: 0.03,
      status: "KANDIDAT",
      dilatihPada: "2026-10-29T00:00:00Z",
      catatan: "Data contoh seeder.",
      demo: true,
    });
  });

  it("model hasil pelatihan membawa baseline dan MAPE realisasi", () => {
    const baris = toBarisModel({ ...model, status: "PRODUKSI", sumber: "MODEL", baseline_mape: 3.1, mape_realisasi: 2.8, n_evaluasi: 12 });

    assert.equal(baris.demo, false);
    assert.equal(baris.baselineMape, 3.1);
    assert.equal(baris.mapeRealisasi, 2.8);
    assert.equal(baris.nEvaluasi, 12);
  });

  it("status tak dikenal dianggap ARSIP, bukan produksi", () => {
    assert.equal(toBarisModel({ ...model, status: "AKTIF" }).status, "ARSIP");
  });
});

describe("modelUsang", () => {
  const sekarang = new Date("2026-11-05T00:00:00Z");

  it("model sungguhan lebih tua dari 30 hari dinyatakan usang", () => {
    assert.equal(modelUsang({ demo: false, dilatihPada: "2026-10-01T00:00:00Z" }, sekarang), true);
    assert.equal(modelUsang({ demo: false, dilatihPada: "2026-10-10T00:00:00Z" }, sekarang), false);
  });

  it("model demo tidak pernah ditandai usang", () => {
    assert.equal(modelUsang({ demo: true, dilatihPada: "2025-01-01T00:00:00Z" }, sekarang), false);
  });
});

describe("toHasilSimulasi", () => {
  const respons: SimulasiResponse = {
    id: "sim-1",
    komoditas_kode: "CBM-KRT",
    komoditas_nama: "Cabai merah keriting",
    kabupaten_kode: "1171",
    horizon_hari: 14,
    harga_kini: 50000,
    proyeksi: 54450,
    dasar_pct: 14.2,
    hasil_pct: 8.9,
    bawah_pct: -1.1,
    atas_pct: 18.9,
    efek: [
      { faktor: "Tambahan pasokan antardaerah", parameter: 10, koefisien: -0.35, efek_pct: -3.5, sumber: "ASUMSI" },
    ],
    model_versi: "v2.3.1",
    sumber_model: "DEMO",
    terbatas: true,
    catatan: "Terbatas: koefisien asumsi.",
  };

  it("memetakan hasil server termasuk label terbatas dan sumber koefisien", () => {
    assert.deepEqual(toHasilSimulasi(respons), {
      id: "sim-1",
      komoditas: "Cabai merah keriting",
      horizonHari: 14,
      hargaKini: 50000,
      proyeksi: 54450,
      dasarPct: 14.2,
      hasilPct: 8.9,
      rentang: [-1.1, 18.9],
      efek: [{ faktor: "Tambahan pasokan antardaerah", efekPct: -3.5, koefisien: -0.35, sumber: "ASUMSI" }],
      terbatas: true,
      catatan: "Terbatas: koefisien asumsi.",
      demo: true,
    });
  });

  it("proyeksi dasar dari model sungguhan bukan demo", () => {
    assert.equal(toHasilSimulasi({ ...respons, sumber_model: "MODEL" }).demo, false);
  });
});

describe("mockSimulasi", () => {
  it("skenario netral sama dengan proyeksi dasar", () => {
    const h = mockSimulasi({ komoditasKode: "CBM-KRT", hujanPct: 100, pasokanPct: 0, subsidiPct: 0 });

    assert.equal(h.hasilPct, h.dasarPct);
    assert.equal(h.terbatas, true);
  });

  it("tambahan pasokan menurunkan proyeksi", () => {
    const netral = mockSimulasi({ komoditasKode: "CBM-KRT", hujanPct: 100, pasokanPct: 0, subsidiPct: 0 });
    const pasok = mockSimulasi({ komoditasKode: "CBM-KRT", hujanPct: 100, pasokanPct: 20, subsidiPct: 0 });

    assert.ok(pasok.hasilPct < netral.hasilPct);
  });
});
