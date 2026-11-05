import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { EwsAlert } from "@/lib/data/types";
import { layakDiumumkan, mergeAlert, parseAlertEvent } from "./ews-stream";

const payload = {
  id: "EWS-2000",
  severity: "SIAGA",
  jenis: "LONJAKAN_HARGA",
  judul: "Lonjakan harga bawang merah",
  pesan: "Harga naik.",
  komoditas_kode: "BWM",
  komoditas_nama: "Bawang merah",
  kabupaten_kode: "1107",
  wilayah: "KABUPATEN ACEH BARAT",
  nilai: 13,
  satuan_nilai: "%",
  skor_dampak: 63,
  status: "BARU",
  sumber: "Rule engine EWS",
  dibuat_pada: "2026-11-05T08:00:00Z",
};

function alert(over: Partial<EwsAlert> = {}): EwsAlert {
  return {
    id: "EWS-1",
    severity: "WASPADA",
    jenis: "LONJAKAN_HARGA",
    judul: "j",
    kabupatenKode: "1171",
    wilayah: "Kota Banda Aceh",
    nilai: 9,
    satuanNilai: "%",
    pesan: "p",
    waktu: "2026-11-05T08:00:00Z",
    status: "BARU",
    skorDampak: 50,
    sumber: "s",
    ...over,
  };
}

describe("parseAlertEvent", () => {
  it("memetakan payload API (snake_case) lewat toEwsAlert, bukan mem-parse mentah", () => {
    const a = parseAlertEvent(JSON.stringify(payload));
    assert.ok(a);
    assert.equal(a.kabupatenKode, "1107");
    assert.equal(a.komoditasNama, "Bawang merah");
    assert.equal(a.wilayah, "Kab. Aceh Barat");
    assert.equal(a.waktu, "2026-11-05T08:00:00Z");
    assert.equal(a.skorDampak, 63);
    assert.equal("kabupaten_kode" in a, false);
  });

  it("membuang event rusak atau tidak lengkap", () => {
    for (const bad of [
      "",
      "bukan json",
      "null",
      "42",
      "[]",
      JSON.stringify({ id: "x" }),
      JSON.stringify({ ...payload, severity: "AMAN" }),
      JSON.stringify({ ...payload, id: 7 }),
    ]) {
      assert.equal(parseAlertEvent(bad), null, bad);
    }
  });
});

describe("mergeAlert", () => {
  it("menaruh alert baru di depan", () => {
    const lama = alert({ id: "EWS-1" });
    assert.deepEqual(
      mergeAlert([lama], alert({ id: "EWS-2" }))?.map((a) => a.id),
      ["EWS-2", "EWS-1"],
    );
  });

  it("mengganti alert yang sudah ada di posisinya (tanpa duplikat)", () => {
    const hasil = mergeAlert(
      [alert({ id: "EWS-2" }), alert({ id: "EWS-1" })],
      alert({ id: "EWS-1", severity: "KRITIS" }),
    );
    assert.deepEqual(
      hasil?.map((a) => a.id),
      ["EWS-2", "EWS-1"],
    );
    assert.equal(hasil?.[1].severity, "KRITIS");
  });

  it("daftar yang belum dimuat dibiarkan", () => {
    assert.equal(mergeAlert(undefined, alert()), undefined);
  });
});

describe("layakDiumumkan", () => {
  it("alert baru atau yang naik tingkat diumumkan; yang sama atau turun tidak", () => {
    const daftar = [alert({ id: "EWS-1", severity: "SIAGA" })];
    assert.equal(layakDiumumkan(daftar, alert({ id: "EWS-9" })), true);
    assert.equal(layakDiumumkan(undefined, alert({ id: "EWS-9" })), true);
    assert.equal(
      layakDiumumkan(daftar, alert({ id: "EWS-1", severity: "KRITIS" })),
      true,
    );
    assert.equal(
      layakDiumumkan(daftar, alert({ id: "EWS-1", severity: "SIAGA" })),
      false,
    );
    assert.equal(
      layakDiumumkan(daftar, alert({ id: "EWS-1", severity: "WASPADA" })),
      false,
    );
  });
});
