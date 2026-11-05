import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  capaianPersen,
  formatKpi,
  kpiMock,
  toKpiIndikator,
  type KpiResponse,
} from "./kpi";

const dasar: KpiResponse = {
  kode: "LOG-UTILISASI-CS",
  kelompok: "LOGISTIK",
  nama: "Utilisasi cold storage",
  deskripsi: "d",
  rumus: "r",
  satuan: "%",
  arah_baik: "NAIK",
  frekuensi: "BULANAN",
  otomatis: true,
  target: {
    nilai: 75,
    ambang_waspada: 67.5,
    baseline: null,
    periode_mulai: "2025-01-01",
  },
  nilai: {
    periode: "2026-10-01",
    nilai: 82.1,
    versi: 1,
    dihitung_pada: "2026-11-05T01:00:00Z",
  },
  tren: [
    {
      periode: "2026-09-01",
      nilai: 75,
      versi: 1,
      dihitung_pada: "2026-11-05T01:00:00Z",
    },
    {
      periode: "2026-10-01",
      nilai: 82.1,
      versi: 1,
      dihitung_pada: "2026-11-05T01:00:00Z",
    },
  ],
  status: "TERCAPAI",
};

describe("toKpiIndikator", () => {
  it("memetakan KPI bernilai", () => {
    const k = toKpiIndikator(dasar);

    assert.equal(k.kelompok, "LOGISTIK");
    assert.equal(k.naikBaik, true);
    assert.equal(k.target, 75);
    assert.equal(k.nilai, 82.1);
    assert.equal(k.periode, "2026-10-01T12:00:00");
    assert.deepEqual(k.tren, [75, 82.1]);
    assert.equal(k.status, "TERCAPAI");
  });

  it("KPI menunggu data tidak punya nilai, periode, atau tren", () => {
    const k = toKpiIndikator({
      ...dasar,
      arah_baik: "TURUN",
      otomatis: false,
      nilai: null,
      tren: [],
      status: "MENUNGGU_DATA",
    });

    assert.equal(k.naikBaik, false);
    assert.equal(k.nilai, null);
    assert.equal(k.periode, null);
    assert.deepEqual(k.tren, []);
    assert.equal(k.status, "MENUNGGU_DATA");
  });

  it("tanpa target: target null", () => {
    assert.equal(
      toKpiIndikator({ ...dasar, target: null, status: "TANPA_TARGET" }).target,
      null,
    );
  });

  it("status tak dikenal jatuh ke menunggu data, bukan capaian palsu", () => {
    assert.equal(
      toKpiIndikator({ ...dasar, status: "ANEH" }).status,
      "MENUNGGU_DATA",
    );
  });
});

describe("capaianPersen", () => {
  it("naik lebih baik: nilai / target, dibatasi 100", () => {
    assert.equal(capaianPersen({ nilai: 50, target: 100, naikBaik: true }), 50);
    assert.equal(
      capaianPersen({ nilai: 130, target: 100, naikBaik: true }),
      100,
    );
  });

  it("turun lebih baik: target / nilai", () => {
    assert.equal(capaianPersen({ nilai: 10, target: 5, naikBaik: false }), 50);
    assert.equal(capaianPersen({ nilai: 4, target: 5, naikBaik: false }), 100);
    assert.equal(capaianPersen({ nilai: 0, target: 5, naikBaik: false }), 100);
  });

  it("null bila nilai atau target tidak ada", () => {
    assert.equal(
      capaianPersen({ nilai: null, target: 5, naikBaik: true }),
      null,
    );
    assert.equal(
      capaianPersen({ nilai: 5, target: null, naikBaik: true }),
      null,
    );
  });
});

describe("formatKpi", () => {
  it("menempelkan satuan sesuai jenisnya", () => {
    assert.equal(formatKpi(82.1, "%"), "82,1%");
    assert.equal(formatKpi(4.2, "/5"), "4,2/5");
    assert.equal(formatKpi(1, "hari"), "1 hari");
    assert.equal(formatKpi(0.88, ""), "0,88");
    assert.equal(formatKpi(10000, "ton"), "10.000 ton");
  });
});

describe("kpiMock", () => {
  it("memuat KPI bernilai dan KPI menunggu data", () => {
    const mock = kpiMock();

    assert.ok(mock.some((k) => k.nilai !== null));
    assert.ok(
      mock.some((k) => k.status === "MENUNGGU_DATA" && k.nilai === null),
    );
  });
});
