import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  toDetailFasilitas,
  toStok,
  toTransaksiBody,
  type DetailFasilitasWire,
  type StokWire,
} from "./cold-storage";

const stok: StokWire = {
  id: "STK-1",
  cold_storage_id: "CS-01",
  cold_storage_nama: "Cold Storage Lambaro",
  komoditas_kode: "CBM-KRT",
  komoditas_nama: "CABAI MERAH KERITING",
  volume_ton: 10,
  masuk_pada: "2026-10-01T00:00:00Z",
  kedaluwarsa_pada: "2026-10-22T00:00:00Z",
  sisa_masa_simpan_hari: 6,
  risiko_food_loss: 0.4,
  pemilik: "KUD",
};

describe("toStok", () => {
  it("status lot dari backend; data lama dianggap AKTIF", () => {
    assert.equal(
      toStok({ ...stok, status: "PRIORITAS_JUAL" }).status,
      "PRIORITAS_JUAL",
    );
    assert.equal(toStok(stok).status, "AKTIF");
  });
});

describe("toTransaksiBody", () => {
  it("MASUK memakai nama kolom kontrak", () => {
    assert.deepEqual(
      toTransaksiBody({
        jenis: "MASUK",
        fasilitasId: "CS-01",
        komoditasKode: "TLR",
        pemilik: "KUD",
        volumeTon: 5,
      }),
      {
        jenis: "MASUK",
        fasilitas_id: "CS-01",
        komoditas_kode: "TLR",
        pemilik: "KUD",
        volume_ton: 5,
      },
    );
  });

  it("KELUAR menyertakan alasan dan flag semua", () => {
    assert.deepEqual(
      toTransaksiBody({
        jenis: "KELUAR",
        lotId: "STK-1",
        semua: true,
        alasan: "Dijual",
      }),
      {
        jenis: "KELUAR",
        lot_id: "STK-1",
        volume_ton: undefined,
        semua: true,
        alasan: "Dijual",
      },
    );
  });

  it("PINDAH menyertakan tujuan", () => {
    const body = toTransaksiBody({
      jenis: "PINDAH",
      lotId: "STK-1",
      fasilitasTujuanId: "CS-03",
      volumeTon: 2,
    });
    assert.equal(body.fasilitas_tujuan_id, "CS-03");
    assert.equal(body.volume_ton, 2);
  });
});

describe("toDetailFasilitas", () => {
  const wire: DetailFasilitasWire = {
    fasilitas: {
      id: "CS-01",
      nama: "Cold Storage Lambaro",
      kabupaten_kode: "1106",
      kabupaten_nama: "KABUPATEN ACEH BESAR",
      lat: 5.5,
      lng: 95.3,
      kapasitas_ton: 400,
      terisi_ton: 347,
      utilisasi_persen: 86.8,
      suhu_target_c: 4,
      suhu_aktual_c: 7.5,
      status: "OPERASIONAL",
      batas_min_c: 2,
      batas_maks_c: 6,
      alarm_suhu: true,
      telemetri_pada: "2026-11-05T07:00:00Z",
    },
    telemetri: [
      {
        waktu: "2026-11-05T07:00:00Z",
        suhu_c: 7.5,
        kelembapan_pct: null,
        di_luar_rentang: true,
      },
    ],
    stok: [stok],
    transaksi: [
      {
        id: "TRX-1",
        lot_id: "STK-1",
        lot_baru_id: null,
        jenis: "MASUK",
        volume_ton: 10,
        komoditas_nama: "CABAI MERAH KERITING",
        fasilitas_asal_id: null,
        fasilitas_tujuan_id: "CS-01",
        oleh_nama: null,
        waktu: "2026-10-01T00:00:00Z",
        alasan: "",
      },
    ],
  };

  it("memetakan alarm, riwayat suhu, lot, dan transaksi", () => {
    const d = toDetailFasilitas(wire);

    assert.equal(d.fasilitas.alarmSuhu, true);
    assert.equal(d.fasilitas.batasMaksC, 6);
    assert.deepEqual(d.telemetri[0], {
      waktu: "2026-11-05T07:00:00Z",
      suhuC: 7.5,
      kelembapanPct: null,
      diLuarRentang: true,
    });
    assert.equal(d.stok[0].komoditasNama, "Cabai merah keriting");
    assert.equal(d.transaksi[0].komoditasNama, "Cabai merah keriting");
    assert.equal(d.transaksi[0].fasilitasTujuanId, "CS-01");
  });
});
