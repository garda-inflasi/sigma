import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { formatJadwal, toSumberData, type SumberDataResponse } from "./sumber";

describe("formatJadwal", () => {
  it("menerjemahkan pola cron katalog ke teks Indonesia", () => {
    assert.equal(formatJadwal("0 6 * * *"), "Harian 06.00");
    assert.equal(formatJadwal("0 11 * * *"), "Harian 11.00");
    assert.equal(formatJadwal("0 7 * * 1"), "Mingguan Senin 07.00");
    assert.equal(formatJadwal("0 */3 * * *"), "Tiap 3 jam");
    assert.equal(formatJadwal("0 * * * *"), "Tiap jam");
    assert.equal(formatJadwal("*/5 * * * *"), "Tiap 5 menit");
  });

  it("menampilkan cron apa adanya bila polanya tidak dikenal", () => {
    assert.equal(formatJadwal("0 6 1 * *"), "0 6 1 * *");
    assert.equal(formatJadwal("bukan cron"), "bukan cron");
  });
});

const dasar: SumberDataResponse = {
  id: "SRC-BI",
  nama: "PIHPS Nasional",
  penyedia: "Bank Indonesia",
  jenis: "API",
  cakupan: "Pasar tradisional",
  jadwal_cron: "0 10 * * *",
  status: "SEHAT",
  terakhir_jalan: "2026-09-30T08:00:00Z",
  keberhasilan_7h: 98.6,
  latensi_ms: 910,
  robots_txt_patuh: true,
  atribusi: "Sumber: BI",
  url_dasar: null,
};

describe("toSumberData", () => {
  it("memetakan kolom API ke model UI", () => {
    assert.deepEqual(toSumberData(dasar), {
      id: "SRC-BI",
      nama: "PIHPS Nasional",
      penyedia: "Bank Indonesia",
      jenis: "API",
      cakupan: "Pasar tradisional",
      jadwal: "Harian 10.00",
      terakhir: "2026-09-30T08:00:00Z",
      status: "SEHAT",
      keberhasilan7h: 98.6,
      latensiMs: 910,
    });
  });

  it("mempertahankan null untuk sumber yang belum pernah berjalan", () => {
    const row = toSumberData({
      ...dasar,
      status: "BELUM_AKTIF",
      terakhir_jalan: null,
      keberhasilan_7h: null,
      latensi_ms: 0,
    });
    assert.equal(row.status, "BELUM_AKTIF");
    assert.equal(row.terakhir, null);
    assert.equal(row.keberhasilan7h, null);
  });

  it("status dan jenis tak dikenal jatuh ke nilai aman", () => {
    const row = toSumberData({ ...dasar, status: "ANEH", jenis: "LAIN" });
    assert.equal(row.status, "BELUM_AKTIF");
    assert.equal(row.jenis, "API");
  });
});
