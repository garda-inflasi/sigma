import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { namaBerkasProyeksi, unduhProyeksiCsv } from "./prediksi-ekspor";

describe("namaBerkasProyeksi", () => {
  it("memakai kode komoditas", () => {
    assert.equal(namaBerkasProyeksi("CBM-KRT"), "proyeksi_CBM-KRT.csv");
  });
});

describe("unduhProyeksiCsv", () => {
  it("menolak dengan pesan jelas saat tanpa backend (mode mock)", async () => {
    await assert.rejects(unduhProyeksiCsv("CBM-KRT"), /backend/);
  });
});
