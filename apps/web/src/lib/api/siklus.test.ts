import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { keteranganSiklus, toSiklusAktif, type PemicuSiklusResponse, type SiklusAktifResponse } from "./siklus";

const pemicu: PemicuSiklusResponse = {
  aturan: "SKL-TKN-LONJAKAN-HARGA",
  siklus: "tekanan",
  indikator: "ALERT_EWS",
  nilai: 6,
  ambang: 1,
  satuan: "alert",
  prioritas: 10,
  rujukan: ["EWS-1000", "EWS-1001"],
  rincian: "",
};

describe("toSiklusAktif", () => {
  it("siklus dari seed: otomatis false dan alasan null tetap null (bukan objek kosong)", () => {
    assert.deepEqual(toSiklusAktif({ siklus: "tekanan", sejak: "2026-11-04", otomatis: false, alasan: null }), {
      siklus: "tekanan",
      sejak: "2026-11-04",
      otomatis: false,
      alasan: null,
    });
  });

  it("siklus otomatis: alasan dipetakan lengkap", () => {
    const s = toSiklusAktif({
      siklus: "tekanan",
      sejak: "2026-11-10",
      otomatis: true,
      alasan: { ringkasan: "Siklus awal TEKANAN: 6 alert.", pemicu: [pemicu], ditekan: [] },
    });
    assert.equal(s.otomatis, true);
    assert.deepEqual(s.alasan, { ringkasan: "Siklus awal TEKANAN: 6 alert.", pemicu: [pemicu], ditekan: [] });
  });

  it("server lama tanpa otomatis dan alasan: dianggap tidak otomatis tanpa alasan (tampilan lama tetap benar)", () => {
    const lama = { siklus: "surplus", sejak: "2026-09-27" } as SiklusAktifResponse;
    assert.deepEqual(toSiklusAktif(lama), { siklus: "surplus", sejak: "2026-09-27", otomatis: false, alasan: null });
  });

  it("alasan tanpa larik pemicu atau ditekan dan pemicu tanpa rujukan tidak menghasilkan undefined", () => {
    const s = toSiklusAktif({
      siklus: "normal",
      sejak: "2026-11-10",
      otomatis: true,
      alasan: { ringkasan: "Siklus awal NORMAL: tidak ada aturan yang memicu." } as never,
    });
    assert.deepEqual(s.alasan?.pemicu, []);
    assert.deepEqual(s.alasan?.ditekan, []);
    const tanpaRujukan = toSiklusAktif({ siklus: "tekanan", sejak: "2026-11-10", otomatis: true, alasan: { ringkasan: "r", pemicu: [{ ...pemicu, rujukan: undefined, rincian: undefined } as never], ditekan: [] } });
    assert.deepEqual(tanpaRujukan.alasan?.pemicu[0].rujukan, []);
    assert.equal(tanpaRujukan.alasan?.pemicu[0].rincian, "");
  });
});

describe("keteranganSiklus", () => {
  const dasar = { siklus: "tekanan", sejak: "2026-11-04", otomatis: false, alasan: null } as const;

  it("siklus dari seed atau manual hanya menyebut tanggal mulainya", () => {
    assert.equal(keteranganSiklus(dasar), "Sejak 4 Nov 2026");
  });

  it("siklus otomatis menyebut mesin yang menentukan dan ringkasan alasannya", () => {
    assert.equal(
      keteranganSiklus({ ...dasar, otomatis: true, alasan: { ringkasan: "Berpindah dari NORMAL ke TEKANAN: 3 alert LONJAKAN_HARGA ≥ SIAGA.", pemicu: [], ditekan: [] } }),
      "Ditentukan otomatis sejak 4 Nov 2026. Berpindah dari NORMAL ke TEKANAN: 3 alert LONJAKAN_HARGA ≥ SIAGA.",
    );
  });

  it("otomatis tanpa ringkasan tetap menyebut penentuan otomatis", () => {
    assert.equal(keteranganSiklus({ ...dasar, otomatis: true }), "Ditentukan otomatis sejak 4 Nov 2026");
    assert.equal(keteranganSiklus({ ...dasar, otomatis: true, alasan: { ringkasan: "", pemicu: [], ditekan: [] } }), "Ditentukan otomatis sejak 4 Nov 2026");
  });

  it("tanggal tak terbaca tidak melempar galat (tooltip tidak boleh merusak tajuk halaman)", () => {
    assert.equal(keteranganSiklus({ ...dasar, sejak: "bukan tanggal" }), "");
    assert.equal(keteranganSiklus({ ...dasar, sejak: "bukan tanggal", otomatis: true, alasan: { ringkasan: "R.", pemicu: [], ditekan: [] } }), "Ditentukan otomatis. R.");
  });

  it("hari tidak bergeser oleh zona waktu (tanggal dibaca sebagai tanggal setempat)", () => {
    assert.equal(keteranganSiklus({ ...dasar, sejak: "2026-01-01" }), "Sejak 1 Jan 2026");
    assert.equal(keteranganSiklus({ ...dasar, sejak: "2026-12-31" }), "Sejak 31 Des 2026");
  });
});
