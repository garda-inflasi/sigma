import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  tanggalLokal,
  toRingkasanEksekutif,
  type RingkasanEksekutifResponse,
} from "./eksekutif";

const dasar: RingkasanEksekutifResponse = {
  demo: true,
  inflasi: {
    periode: "2026-10-01",
    sumber: "DEMO",
    diimpor_pada: "2026-11-05T01:00:00Z",
    ihk: 110.5,
    yoy: 2.84,
    mtm: 0.41,
    ytd: 2.13,
    sasaran: { tahun: 2026, bawah: 1.5, atas: 3.5 },
    seri: [
      { periode: "2026-09-01", yoy: 2.63 },
      { periode: "2026-10-01", yoy: 2.84 },
    ],
    penyumbang: [
      { kode: "CBM-KRT", nama: "Cabai merah keriting", andil: 0.18 },
    ],
  },
  neraca: {
    periode: "2026-10-01",
    sumber: "DEMO",
    items: [
      {
        komoditas_kode: "BRS-MED",
        nama: "Beras medium",
        produksi_ton: 146200,
        konsumsi_ton: 52400,
        stok_ton: null,
      },
    ],
  },
  fiskal: {
    sumber: "DEMO",
    per_tanggal: "2026-10-31",
    pagu: 186e9,
    realisasi: 54.2e9,
    skor: 71,
    status: "MEMADAI",
    instrumen: [{ nama: "BTT", pagu: 120e9, realisasi: 31.2e9 }],
  },
  ews: { KRITIS: 2, SIAGA: 1 },
  intervensi: { tekanan: { aktif: 3, selesai: 1 } },
  siklus_dominan: "tekanan",
};

describe("tanggalLokal", () => {
  it("menetapkan siang hari lokal agar tanggal tidak bergeser oleh zona waktu", () => {
    assert.equal(tanggalLokal("2026-10-01"), "2026-10-01T12:00:00");
    assert.equal(tanggalLokal("2026-10-01T00:00:00Z"), "2026-10-01T12:00:00");
    assert.equal(new Date(tanggalLokal("2026-10-01")).getDate(), 1);
  });
});

describe("toRingkasanEksekutif", () => {
  it("memetakan inflasi, neraca, dan fiskal ke model UI", () => {
    const r = toRingkasanEksekutif(dasar);

    assert.equal(r.demo, true);
    assert.deepEqual(r.inflasi, {
      periode: "2026-10-01T12:00:00",
      sumber: "DEMO",
      diimporPada: "2026-11-05T01:00:00Z",
      yoy: 2.84,
      mtm: 0.41,
      ytd: 2.13,
      sasaran: { bawah: 1.5, atas: 3.5 },
      seri: [
        { bulan: "Sep", yoy: 2.63 },
        { bulan: "Okt", yoy: 2.84 },
      ],
      penyumbang: [{ nama: "Cabai merah keriting", andil: 0.18 }],
    });
    assert.deepEqual(r.neraca?.items, [
      {
        komoditasKode: "BRS-MED",
        nama: "Beras medium",
        produksi: 146200,
        kebutuhan: 52400,
      },
    ]);
    assert.equal(r.fiskal?.perTanggal, "2026-10-31T12:00:00");
    assert.equal(r.fiskal?.status, "MEMADAI");
    assert.deepEqual(r.fiskal?.instrumen, [
      { nama: "BTT", pagu: 120e9, realisasi: 31.2e9 },
    ]);
  });

  it("melengkapi severity dan siklus yang tidak dikirim dengan nol", () => {
    const r = toRingkasanEksekutif(dasar);

    assert.deepEqual(r.ews, { INFO: 0, WASPADA: 0, SIAGA: 1, KRITIS: 2 });
    assert.deepEqual(r.intervensi, {
      normal: { aktif: 0, selesai: 0 },
      tekanan: { aktif: 3, selesai: 1 },
      surplus: { aktif: 0, selesai: 0 },
    });
    assert.equal(r.siklusDominan, "tekanan");
  });

  it("mempertahankan blok yang kosong sebagai null", () => {
    const r = toRingkasanEksekutif({
      ...dasar,
      demo: false,
      inflasi: null,
      neraca: null,
      fiskal: null,
    });

    assert.equal(r.demo, false);
    assert.equal(r.inflasi, null);
    assert.equal(r.neraca, null);
    assert.equal(r.fiskal, null);
  });

  it("sumber tak dikenal diberi label DEMO, bukan tampil resmi", () => {
    const r = toRingkasanEksekutif({
      ...dasar,
      inflasi: { ...dasar.inflasi!, sumber: "ENTAH" },
    });

    assert.equal(r.inflasi?.sumber, "DEMO");
  });

  it("siklus dominan dan status fiskal tak dikenal jatuh ke nilai aman", () => {
    const r = toRingkasanEksekutif({
      ...dasar,
      siklus_dominan: "aneh",
      fiskal: { ...dasar.fiskal!, status: "ANEH" },
    });

    assert.equal(r.siklusDominan, "normal");
    assert.equal(r.fiskal?.status, "KRITIS");
  });
});
