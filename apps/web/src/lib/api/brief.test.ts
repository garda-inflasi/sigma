import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { periodeBulanIni, toPolicyBrief } from "./brief";

const respons = {
  id: "PB-2026-10",
  periode: "Oktober 2026",
  judul: "Meredam tekanan harga cabai",
  status: "DRAF",
  ringkasan: "Ringkasan",
  disusun_oleh: "Dr. Nurul Aini, S.E., M.E.",
  jumlah_rekomendasi: 5,
  halaman: 8,
  dibuat_pada: "2026-11-02T08:15:00Z",
  diperbarui: "2026-11-05T08:15:00Z",
};

describe("toPolicyBrief", () => {
  it("memetakan kolom API ke baris UI", () => {
    assert.deepEqual(toPolicyBrief(respons), {
      id: "PB-2026-10",
      periode: "Oktober 2026",
      judul: "Meredam tekanan harga cabai",
      status: "DRAF",
      ringkasan: "Ringkasan",
      disusunOleh: "Dr. Nurul Aini, S.E., M.E.",
      diperbarui: "2026-11-05T08:15:00Z",
      jumlahRekomendasi: 5,
      halaman: 8,
    });
  });

  it("menerima keempat status dari API", () => {
    for (const status of [
      "DRAF",
      "TELAAH",
      "DISETUJUI",
      "DIDISTRIBUSIKAN",
    ] as const) {
      assert.equal(toPolicyBrief({ ...respons, status }).status, status);
    }
  });

  it("status tak dikenal dianggap DRAF", () => {
    assert.equal(toPolicyBrief({ ...respons, status: "DRAFT" }).status, "DRAF");
  });
});

describe("periodeBulanIni", () => {
  it("memakai format YYYY-MM dengan bulan dua digit", () => {
    assert.equal(periodeBulanIni(new Date(2026, 10, 5)), "2026-11");
    assert.equal(periodeBulanIni(new Date(2027, 0, 31)), "2027-01");
  });
});
