import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, it } from "node:test";
import { SIKLUS_TAHAP } from "@/lib/data/siklus";
import { inisial, toKartu, type KartuResponse } from "./intervensi";

const kartu: KartuResponse = {
  id: "INT-301",
  siklus: "TEKANAN",
  tahap: "t-ews",
  judul: "Lonjakan cabai merah Banda Aceh",
  komoditas_kode: "CBM-KRT",
  kabupaten_kode: "1171",
  prioritas: "TINGGI",
  tenggat: "2026-10-17T09:00:00Z",
  rekomendasi_ai: true,
  progres: { selesai: 1, total: 5 },
  penanggung_jawab: [
    { id: "manajer_program", nama: "Rina Safitri, S.T., M.T." },
  ],
};

describe("toKartu", () => {
  it("memetakan kolom API ke model kanban", () => {
    assert.deepEqual(toKartu(kartu), {
      id: "INT-301",
      siklus: "tekanan",
      tahap: "t-ews",
      judul: "Lonjakan cabai merah Banda Aceh",
      komoditasKode: "CBM-KRT",
      kabupatenKode: "1171",
      prioritas: "TINGGI",
      penanggungJawab: [{ nama: "Rina Safitri, S.T., M.T.", inisial: "RS" }],
      tenggat: "2026-10-17T09:00:00Z",
      rekomendasiAi: true,
      progres: { selesai: 1, total: 5 },
    });
  });

  it("siklus tak dikenal jatuh ke normal", () => {
    assert.equal(toKartu({ ...kartu, siklus: "ACAK" }).siklus, "normal");
  });
});

describe("inisial", () => {
  it("melewati gelar dan tanda baca di depan kata", () => {
    assert.equal(inisial("Dr. Nurul Aini, S.E., M.E."), "DN");
    assert.equal(inisial("Zulkifli, S.P."), "ZS");
    assert.equal(inisial("Maya"), "M");
  });
});

/** Cermin seed intervensi.reference di API Go (internal/intervensi/seeder/seeder_test.go). */
const seed = JSON.parse(
  readFileSync(
    resolve(process.cwd(), "../../packages/contracts/intervensi-tahap.json"),
    "utf8",
  ),
) as {
  siklus: string;
  kode: string;
  urutan: number;
  label: string;
  petunjuk: string;
  izin_dibutuhkan: string;
}[];

describe("siklus.ts sama dengan seed intervensi.reference", () => {
  for (const [siklus, tahap] of Object.entries(SIKLUS_TAHAP)) {
    it(`tahap siklus ${siklus}: kode, urutan, label, petunjuk, dan izin masuk sama`, () => {
      const dariSeed = seed
        .filter((s) => s.siklus === siklus)
        .sort((a, b) => a.urutan - b.urutan);
      assert.deepEqual(
        tahap.map((t) => ({
          kode: t.id,
          label: t.judul,
          petunjuk: t.petunjuk,
          izin: t.izinMasuk ?? "intervensi.kelola",
        })),
        dariSeed.map((s) => ({
          kode: s.kode,
          label: s.label,
          petunjuk: s.petunjuk,
          izin: s.izin_dibutuhkan,
        })),
      );
    });
  }

  it("seed tidak punya siklus yang tidak dikenal web", () => {
    assert.deepEqual(
      [...new Set(seed.map((s) => s.siklus))].sort(),
      Object.keys(SIKLUS_TAHAP).sort(),
    );
  });
});
