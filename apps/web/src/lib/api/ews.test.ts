import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  satuanAmbang,
  toEwsAlert,
  toEwsAturan,
  type EwsAlertResponse,
} from "./ews";

const alert: EwsAlertResponse = {
  id: "EWS-1000",
  severity: "KRITIS",
  jenis: "LONJAKAN_HARGA",
  judul: "Lonjakan harga cabai merah keriting",
  pesan: "Harga naik.",
  komoditas_kode: "CBM-KRT",
  komoditas_nama: "Cabai merah keriting",
  kabupaten_kode: "1171",
  wilayah: "KOTA BANDA ACEH",
  nilai: 21.4,
  satuan_nilai: "%",
  skor_dampak: 91,
  status: "BARU",
  sumber: "Model EWS v2.3",
  dibuat_pada: "2026-10-16T08:56:00Z",
};

describe("toEwsAlert", () => {
  it("memetakan kolom API ke model UI dengan nama wilayah ringkas", () => {
    assert.deepEqual(toEwsAlert(alert), {
      id: "EWS-1000",
      severity: "KRITIS",
      jenis: "LONJAKAN_HARGA",
      judul: "Lonjakan harga cabai merah keriting",
      komoditasKode: "CBM-KRT",
      komoditasNama: "Cabai merah keriting",
      kabupatenKode: "1171",
      wilayah: "Kota Banda Aceh",
      nilai: 21.4,
      satuanNilai: "%",
      pesan: "Harga naik.",
      waktu: "2026-10-16T08:56:00Z",
      status: "BARU",
      skorDampak: 91,
      sumber: "Model EWS v2.3",
    });
  });

  it("alert tanpa komoditas tidak membawa kunci komoditas", () => {
    const row = toEwsAlert({
      ...alert,
      komoditas_kode: null,
      komoditas_nama: null,
      satuan_nilai: "mm",
    });
    assert.equal(row.komoditasKode, undefined);
    assert.equal(row.komoditasNama, undefined);
    assert.equal(row.satuanNilai, "mm");
  });

  it("satuan tak dikenal jatuh ke persen", () => {
    assert.equal(toEwsAlert({ ...alert, satuan_nilai: "?" }).satuanNilai, "%");
  });
});

describe("toEwsAturan", () => {
  it("memetakan ambang dan kanal", () => {
    const row = toEwsAturan({
      id: "ATR-LH-CBM-KRT",
      komoditas_kode: "CBM-KRT",
      komoditas_nama: "Cabai merah keriting",
      jenis: "LONJAKAN_HARGA",
      ambang_waspada: 8,
      ambang_siaga: 12,
      ambang_kritis: 18,
      jendela_hari: 7,
      kanal: ["DASHBOARD", "PUSH"],
      aktif: true,
      keterangan: "Ambang awal",
    });
    assert.deepEqual([row.waspada, row.siaga, row.kritis], [8, 12, 18]);
    assert.deepEqual(row.kanal, ["DASHBOARD", "PUSH"]);
    assert.equal(row.komoditasNama, "Cabai merah keriting");
  });
});

describe("satuanAmbang", () => {
  it("cuaca dalam mm, lainnya persen", () => {
    assert.equal(satuanAmbang("ANOMALI_CUACA"), "mm");
    assert.equal(satuanAmbang("LONJAKAN_HARGA"), "%");
    assert.equal(satuanAmbang("KELANGKAAN_STOK"), "%");
  });
});
