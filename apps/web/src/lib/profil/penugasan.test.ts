import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { ROLE_IDS } from "@/lib/rbac/roles";
import {
  cakupanWilayah,
  infoPeran,
  labelLapis,
  labelStatusAkun,
} from "./penugasan";

describe("labelStatusAkun", () => {
  it("status dikenal memakai label dan nada tetap", () => {
    assert.deepEqual(labelStatusAkun("AKTIF"), {
      label: "Aktif",
      tone: "normal",
    });
    assert.deepEqual(labelStatusAkun("MENUNGGU"), {
      label: "Menunggu aktivasi",
      tone: "waspada",
    });
    assert.deepEqual(labelStatusAkun("NONAKTIF"), {
      label: "Nonaktif",
      tone: "netral",
    });
  });

  it("status asing tampil sebagai kata biasa dengan nada netral", () => {
    assert.deepEqual(labelStatusAkun("DIBEKUKAN"), {
      label: "Dibekukan",
      tone: "netral",
    });
    assert.deepEqual(labelStatusAkun(""), {
      label: "Tidak diketahui",
      tone: "netral",
    });
  });

  it("nama bawaan Object tidak dianggap status dikenal", () => {
    assert.deepEqual(labelStatusAkun("constructor"), {
      label: "Constructor",
      tone: "netral",
    });
  });
});

describe("infoPeran", () => {
  it("peran dikenal: label dan uraian tugas", () => {
    const info = infoPeran("OPS_KOORDINATOR");
    assert.equal(info.label, "Koordinator Lapangan");
    assert.ok(info.tugas);
  });

  it("setiap peran dikenal punya label", () => {
    for (const peran of ROLE_IDS) assert.ok(infoPeran(peran).label, peran);
  });

  it("peran asing: kodenya sebagai label tanpa uraian", () => {
    assert.deepEqual(infoPeran("PERAN_BARU"), { label: "PERAN_BARU" });
    assert.deepEqual(infoPeran("toString"), { label: "toString" });
  });
});

describe("labelLapis", () => {
  it("lapis dikenal memakai label katalog", () => {
    assert.equal(labelLapis("OPERASIONAL"), "Lapis operasional wilayah");
    assert.equal(labelLapis("SISTEM"), "Sistem");
  });

  it("lapis asing apa adanya", () => {
    assert.equal(labelLapis("LAIN"), "LAIN");
  });
});

describe("cakupanWilayah", () => {
  it("tanpa kode kabupaten: cakupan provinsi", () => {
    for (const kosong of [undefined, ""]) {
      assert.deepEqual(cakupanWilayah(kosong, undefined, "STRATEGIS_FMIPA"), {
        wilayah: "Seluruh Aceh",
        tingkat: "Cakupan provinsi",
        batasPeran: "Melihat data seluruh wilayah",
      });
    }
  });

  it("dengan kode kabupaten: nama kabupaten dan cakupan kabupaten/kota", () => {
    assert.deepEqual(
      cakupanWilayah("1106", "Kab. Aceh Tengah", "OPS_KOORDINATOR"),
      {
        wilayah: "Kab. Aceh Tengah",
        tingkat: "Cakupan kabupaten/kota",
        batasPeran: "Data dibatasi pada kabupaten/kota penugasan",
      },
    );
  });

  it("nama belum diketahui: kode kabupaten yang tampil", () => {
    assert.equal(
      cakupanWilayah("1106", undefined, "OPS_KOORDINATOR").wilayah,
      "1106",
    );
    assert.equal(cakupanWilayah("1106", "", "OPS_KOORDINATOR").wilayah, "1106");
  });

  it("batas data menurut cakupan peran; peran asing tanpa keterangan", () => {
    assert.equal(
      cakupanWilayah("1106", "Kab. X", "OPS_MAHASISWA").batasPeran,
      "Hanya data yang Anda input sendiri",
    );
    assert.equal(
      cakupanWilayah(undefined, undefined, "PERAN_BARU").batasPeran,
      undefined,
    );
  });
});
