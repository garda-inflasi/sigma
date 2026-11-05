import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  kpiRiwayatMock,
  statusTerhadapTarget,
  toKpiRiwayat,
  type KpiRiwayatResponse,
} from "./kpi";

const nilai = (periode: string, n: number, versi = 1) => ({
  periode,
  nilai: n,
  versi,
  dihitung_pada: "2026-11-05T01:00:00Z",
});

const respons = (
  arah: string,
  riwayat: KpiRiwayatResponse["riwayat"],
  target: { nilai: number; ambang_waspada: number | null } | null,
): KpiRiwayatResponse => ({
  kpi: {
    kode: "LOG-UTILISASI-CS",
    kelompok: "LOGISTIK",
    nama: "Utilisasi cold storage",
    deskripsi: "",
    rumus: "",
    satuan: "%",
    arah_baik: arah,
    frekuensi: "BULANAN",
    otomatis: true,
    target: target && {
      ...target,
      baseline: null,
      periode_mulai: "2025-01-01",
    },
    nilai: riwayat.length ? riwayat[riwayat.length - 1] : null,
    tren: riwayat,
    status: "TERCAPAI",
  },
  riwayat,
});

describe("statusTerhadapTarget", () => {
  it("naik lebih baik: tercapai, waspada, belum tercapai", () => {
    assert.equal(statusTerhadapTarget(true, 80, 75, 67.5), "TERCAPAI");
    assert.equal(statusTerhadapTarget(true, 75, 75, 67.5), "TERCAPAI");
    assert.equal(statusTerhadapTarget(true, 70, 75, 67.5), "WASPADA");
    assert.equal(statusTerhadapTarget(true, 60, 75, 67.5), "TIDAK_TERCAPAI");
  });

  it("turun lebih baik membalik arah perbandingan", () => {
    assert.equal(statusTerhadapTarget(false, 5, 5.4, 6), "TERCAPAI");
    assert.equal(statusTerhadapTarget(false, 5.8, 5.4, 6), "WASPADA");
    assert.equal(statusTerhadapTarget(false, 7, 5.4, 6), "TIDAK_TERCAPAI");
  });

  it("tanpa ambang waspada tidak ada status waspada; tanpa target tak dinilai", () => {
    assert.equal(statusTerhadapTarget(true, 70, 75, null), "TIDAK_TERCAPAI");
    assert.equal(statusTerhadapTarget(true, 70, null, null), "TANPA_TARGET");
  });
});

describe("toKpiRiwayat", () => {
  it("memetakan periode, versi, dan status tiap baris terhadap target saat ini", () => {
    const r = toKpiRiwayat(
      respons(
        "NAIK",
        [
          nilai("2026-08-01", 60),
          nilai("2026-09-01", 70, 2),
          nilai("2026-10-01", 80),
        ],
        { nilai: 75, ambang_waspada: 67.5 },
      ),
    );

    assert.equal(r.kpi.nama, "Utilisasi cold storage");
    assert.equal(r.ambangWaspada, 67.5);
    assert.deepEqual(
      r.baris.map((b) => [b.periode, b.nilai, b.versi, b.status]),
      [
        ["2026-08-01T12:00:00", 60, 1, "TIDAK_TERCAPAI"],
        ["2026-09-01T12:00:00", 70, 2, "WASPADA"],
        ["2026-10-01T12:00:00", 80, 1, "TERCAPAI"],
      ],
    );
  });

  it("riwayat kosong menghasilkan daftar kosong, bukan galat", () => {
    const r = toKpiRiwayat(respons("NAIK", [], null));
    assert.deepEqual(r.baris, []);
    assert.equal(r.ambangWaspada, null);
  });

  it("tanpa target semua baris berstatus tanpa target", () => {
    const r = toKpiRiwayat(respons("TURUN", [nilai("2026-10-01", 3)], null));
    assert.equal(r.baris[0].status, "TANPA_TARGET");
  });
});

describe("kpiRiwayatMock", () => {
  it("memakai tren indikator mock dengan periode bulanan berurutan", () => {
    const r = kpiRiwayatMock("LOG-UTILISASI-CS");
    assert.equal(r.baris.length, 6);
    assert.equal(r.baris[5].periode, "2026-10-01T12:00:00");
    assert.equal(r.baris[0].periode, "2026-05-01T12:00:00");
  });

  it("menolak kode yang tidak dikenal", () => {
    assert.throws(() => kpiRiwayatMock("TAK-ADA"), /tidak ditemukan/);
  });
});
