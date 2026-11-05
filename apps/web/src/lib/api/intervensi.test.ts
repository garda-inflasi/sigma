import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, it } from "node:test";
import { SIKLUS_TAHAP } from "@/lib/data/siklus";
import { bisaMemutus, ringkasHasilKartu, toKartu, toPersetujuan, type KartuResponse, type PersetujuanResponse } from "./intervensi";

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
  versi: 4,
  dieskalasi_pada: null,
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
      versi: 4,
      dieskalasi: false,
    });
  });

  it("menandai kartu yang dieskalasi", () => {
    assert.equal(toKartu({ ...kartu, dieskalasi_pada: "2026-11-05T08:00:00Z" }).dieskalasi, true);
  });

  it("siklus tak dikenal jatuh ke normal", () => {
    assert.equal(toKartu({ ...kartu, siklus: "ACAK" }).siklus, "normal");
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

describe("ringkasHasilKartu", () => {
  it("semua berhasil", () => {
    assert.deepEqual(ringkasHasilKartu("Ditugaskan", { berhasil: 2, gagal: 0, hasil: [{ id: "a", berhasil: true }, { id: "b", berhasil: true }] }), {
      judul: "Ditugaskan: 2 kartu",
      deskripsi: "Tercatat di audit trail.",
    });
  });
  it("sebagian dilewati dengan alasan unik", () => {
    const r = ringkasHasilKartu("Dieskalasi", {
      berhasil: 1,
      gagal: 2,
      hasil: [{ id: "a", berhasil: true }, { id: "b", berhasil: false, alasan: "Kartu sudah selesai" }, { id: "c", berhasil: false, alasan: "Kartu sudah selesai" }],
    });
    assert.equal(r.deskripsi, "2 kartu dilewati (Kartu sudah selesai).");
  });
});

const permintaan: PersetujuanResponse = {
  id: 7,
  intervensi_id: "INT-304",
  kartu_judul: "Kirim cabai Takengon → Banda Aceh",
  kartu_tahap: "t-logistik",
  jenis: "PINDAH_TAHAP",
  tahap_tujuan: "t-fiskal",
  status: "MENUNGGU",
  langkah_kini: 2,
  izin_langkah_kini: "intervensi.setujui_fiskal",
  diajukan_oleh_id: "manajer_program",
  diajukan_oleh_nama: "Rina Safitri, S.T., M.T.",
  diajukan_pada: "2026-11-05T05:00:00Z",
  catatan: "",
  langkah: [
    { urutan: 1, label: "Persetujuan Dewan Pengarah", izin_dibutuhkan: "intervensi.setujui", keputusan: "SETUJU", oleh_id: "strategis_bi", oleh_nama: "Dr. Muhammad Rizal", catatan: "" },
    { urutan: 2, label: "Persetujuan Kanwil DJPb", izin_dibutuhkan: "intervensi.setujui_fiskal", keputusan: null, oleh_id: null, oleh_nama: null, catatan: "" },
  ],
};

describe("toPersetujuan", () => {
  it("memetakan rantai langkah dan mengosongkan nilai null", () => {
    const p = toPersetujuan(permintaan);
    assert.equal(p.kartuId, "INT-304");
    assert.equal(p.tahapTujuan, "t-fiskal");
    assert.equal(p.langkah[0].keputusan, "SETUJU");
    assert.equal(p.langkah[0].olehNama, "Dr. Muhammad Rizal");
    assert.equal(p.langkah[1].keputusan, undefined);
    assert.equal(toPersetujuan({ ...permintaan, tahap_tujuan: null, diajukan_oleh_id: null, diajukan_oleh_nama: null }).diajukanOlehId, undefined);
  });
});

describe("bisaMemutus", () => {
  const p = toPersetujuan(permintaan);
  it("pemegang izin langkah yang belum terlibat boleh memutus", () => {
    assert.equal(bisaMemutus(p, "strategis_djpb", true), true);
  });
  it("tanpa izin langkah tidak boleh", () => {
    assert.equal(bisaMemutus(p, "strategis_bi", false), false);
  });
  it("pengaju tidak menyetujui sendiri, dan pemutus langkah lain tidak memutus lagi", () => {
    assert.equal(bisaMemutus(p, "manajer_program", true), false);
    assert.equal(bisaMemutus(p, "strategis_bi", true), false);
  });
  it("permintaan yang sudah diputuskan tidak dapat diputus", () => {
    assert.equal(bisaMemutus({ ...p, status: "DISETUJUI" }, "strategis_djpb", true), false);
  });
  it("permintaan buatan sistem dapat diputus siapa pun yang berizin", () => {
    assert.equal(bisaMemutus({ ...p, diajukanOlehId: undefined }, "strategis_djpb", true), true);
  });
});
