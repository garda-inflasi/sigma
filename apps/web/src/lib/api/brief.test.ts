import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { periodeBulanIni, toPolicyBrief, toPolicyBriefDetail, type PolicyBriefDetailResponse } from "./brief";

const respons = {
  id: "PB-2026-10",
  periode: "Oktober 2026",
  judul: "Meredam tekanan harga cabai",
  status: "DRAF",
  ringkasan: "Ringkasan",
  disusun_oleh: "Dr. Nurul Aini, S.E., M.E.",
  jumlah_rekomendasi: 5,
  halaman: 8,
  sumber_narasi: "LLM",
  label_draf: "Draf AI — wajib ditelaah manusia",
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
      sumberNarasi: "LLM",
      labelDraf: "Draf AI — wajib ditelaah manusia",
    });
  });

  it("menerima semua status dari API, termasuk status penyusunan asinkron", () => {
    for (const status of [
      "DIPROSES",
      "GAGAL",
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

  it("asal narasi tak dikenal tidak diisi", () => {
    assert.equal(toPolicyBrief({ ...respons, sumber_narasi: "LAIN" }).sumberNarasi, undefined);
    assert.equal(toPolicyBrief({ ...respons, sumber_narasi: "TEMPLAT" }).sumberNarasi, "TEMPLAT");
  });
});

describe("toPolicyBriefDetail", () => {
  const detail: PolicyBriefDetailResponse = {
    ...respons,
    konten: {
      label: "Draf AI — wajib ditelaah manusia",
      catatan: "Catatan",
      bagian: [{ kode: "ringkasan", judul: "Ringkasan", butir: ["Inflasi tahunan 2,84%."] }],
      lampiran: [{ id: "INFLASI_YOY", label: "Inflasi tahunan", nilai: "2,84%", sumber: "inflasi_bulanan (BPS)" }],
    },
    model_llm: "claude-sonnet-5-5",
    prompt_versi: "brief-v1",
    prompt_hash: "abc",
    galat: null,
  };

  it("memetakan isi, sitasi, dan jejak model", () => {
    const d = toPolicyBriefDetail(detail);

    assert.equal(d.bagian[0].butir[0], "Inflasi tahunan 2,84%.");
    assert.equal(d.sitasi[0].sumber, "inflasi_bulanan (BPS)");
    assert.equal(d.catatan, "Catatan");
    assert.equal(d.model, "claude-sonnet-5-5");
    assert.equal(d.promptVersi, "brief-v1");
    assert.equal(d.promptHash, "abc");
    assert.equal(d.galat, undefined);
    assert.equal(d.labelDraf, "Draf AI — wajib ditelaah manusia");
  });

  it("brief yang belum selesai atau gagal punya isi kosong dan membawa galat", () => {
    const d = toPolicyBriefDetail({ ...detail, status: "GAGAL", konten: {}, model_llm: null, prompt_versi: null, prompt_hash: null, galat: "kumpulkan data: db down" });

    assert.deepEqual(d.bagian, []);
    assert.deepEqual(d.sitasi, []);
    assert.equal(d.model, undefined);
    assert.equal(d.galat, "kumpulkan data: db down");
    assert.equal(d.status, "GAGAL");
  });
});

describe("periodeBulanIni", () => {
  it("memakai format YYYY-MM dengan bulan dua digit", () => {
    assert.equal(periodeBulanIni(new Date(2026, 10, 5)), "2026-11");
    assert.equal(periodeBulanIni(new Date(2027, 0, 31)), "2027-01");
  });
});
