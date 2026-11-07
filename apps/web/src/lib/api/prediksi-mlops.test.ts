import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { api } from "./endpoints";
import {
  alasanTakBisaSegarkan,
  ringkasEvaluasi,
  ringkasSegarkan,
  type BarisModel,
} from "./prediksi";

const model = (patch: Partial<BarisModel>): BarisModel => ({
  id: "1",
  kode: "MDL-CBM-KRT-ml-20261105-090000",
  komoditasKode: "CBM-KRT",
  komoditas: "Cabai merah keriting",
  versi: "ml-20261105-090000",
  mape: 3,
  baselineMape: 4,
  mapeRealisasi: null,
  nEvaluasi: 0,
  drift: 0,
  status: "PRODUKSI",
  dilatihPada: "2026-11-05T09:00:00Z",
  catatan: "",
  demo: false,
  ...patch,
});

describe("ringkasSegarkan", () => {
  it("menyebut jumlah proyeksi dan komoditas", () => {
    assert.match(
      ringkasSegarkan({ proyeksi: 21 }, "Cabai merah keriting"),
      /^21 proyeksi ditulis ulang untuk Cabai merah keriting/,
    );
  });
});

describe("ringkasEvaluasi", () => {
  it("nol dijelaskan sebagai keadaan wajar, bukan galat", () => {
    assert.match(
      ringkasEvaluasi({ dievaluasi: 0 }),
      /Tidak ada proyeksi baru yang bisa dievaluasi/,
    );
  });

  it("menyebut jumlah yang dievaluasi", () => {
    assert.match(
      ringkasEvaluasi({ dievaluasi: 1234 }),
      /^1\.234 proyeksi jatuh tempo dievaluasi/,
    );
  });
});

describe("alasanTakBisaSegarkan", () => {
  it("null bila model produksi hasil pelatihan dan layanan ML siap", () => {
    assert.equal(alasanTakBisaSegarkan([model({})], "CBM-KRT", true), null);
  });

  it("menolak bila layanan ML tidak tersedia", () => {
    assert.match(
      alasanTakBisaSegarkan([model({})], "CBM-KRT", false) ?? "",
      /Layanan ML/,
    );
  });

  it("menolak model produksi demo dan komoditas tanpa model produksi", () => {
    assert.match(
      alasanTakBisaSegarkan([model({ demo: true })], "CBM-KRT", true) ?? "",
      /data demo/,
    );
    assert.match(
      alasanTakBisaSegarkan([model({ status: "KANDIDAT" })], "CBM-KRT", true) ??
        "",
      /belum punya model produksi/,
    );
    assert.match(
      alasanTakBisaSegarkan(undefined, "CBM-KRT", true) ?? "",
      /belum punya model produksi/,
    );
  });
});

describe("api.prediksi tanpa backend (mode mock)", () => {
  it("segarkan dan evaluasi ditolak dengan pesan jelas, bukan berpura-pura berhasil", async () => {
    await assert.rejects(api.prediksi.segarkan("CBM-KRT"), /backend/);
    await assert.rejects(api.prediksi.evaluasi(), /backend/);
  });
});
