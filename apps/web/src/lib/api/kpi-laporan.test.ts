import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { namaBerkasLaporan, unduhLaporanKpi } from "./kpi-laporan";

describe("namaBerkasLaporan", () => {
  it("memuat tahun dan bulan", () => {
    assert.equal(
      namaBerkasLaporan(new Date("2026-11-05T09:00:00Z")),
      "laporan-kpi-2026-11.csv",
    );
  });
});

describe("unduhLaporanKpi", () => {
  it("menolak dengan pesan jelas saat tanpa backend (mode mock)", async () => {
    await assert.rejects(unduhLaporanKpi(), /backend/);
  });
});
