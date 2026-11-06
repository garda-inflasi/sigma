import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { galatFormBawaan, jalankanKiriman } from "./kiriman";
import { skemaDariValidasi } from "./skema";

describe("skemaDariValidasi", () => {
  it("peta galat menjadi galat per field", async () => {
    const skema = skemaDariValidasi<{ judul: string }>((n) => (n.judul ? {} : { judul: "Judul wajib diisi." }));
    const hasil = await jalankanKiriman({ nilai: { judul: "" }, skema, kirim: async () => 1, petaGalat: galatFormBawaan });
    assert.deepEqual(hasil, { jenis: "tidak-valid", galatField: { judul: "Judul wajib diisi." }, pesanForm: undefined });
  });

  it("string menjadi galat tingkat form", async () => {
    const skema = skemaDariValidasi<{ kanal: string[] }>((n) => (n.kanal.length ? null : "Pilih sedikitnya satu kanal."));
    const hasil = await jalankanKiriman({ nilai: { kanal: [] }, skema, kirim: async () => 1, petaGalat: galatFormBawaan });
    assert.deepEqual(hasil, { jenis: "tidak-valid", galatField: {}, pesanForm: "Pilih sedikitnya satu kanal." });
  });

  it("valid: nilai yang sama diteruskan apa adanya", async () => {
    const nilai = { a: 1 };
    let diterima: unknown;
    const skema = skemaDariValidasi<typeof nilai>(() => null);
    await jalankanKiriman({ nilai, skema, kirim: async (n) => void (diterima = n), petaGalat: galatFormBawaan });
    assert.deepEqual(diterima, nilai);
  });
});
