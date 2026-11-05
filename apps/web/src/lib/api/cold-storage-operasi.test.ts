import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  angkaIsian,
  dariDatetimeLocal,
  galatPembacaan,
  galatUbahFasilitas,
  keDatetimeLocal,
  toDetailFasilitas,
  toHasilTelemetri,
  toStok,
  toTelemetriBody,
  toTransaksiBody,
  toUbahFasilitasBody,
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
        sumber: "MANUAL",
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
      sumber: "MANUAL",
      diLuarRentang: true,
    });
    assert.equal(d.stok[0].komoditasNama, "Cabai merah keriting");
    assert.equal(d.transaksi[0].komoditasNama, "Cabai merah keriting");
    assert.equal(d.transaksi[0].fasilitasTujuanId, "CS-01");
  });
});

describe("toUbahFasilitasBody", () => {
  it("hanya mengirim kolom yang diisi, dengan nama kolom kontrak", () => {
    assert.deepEqual(toUbahFasilitasBody({ suhuMaksC: 7.5 }), {
      suhu_maks_c: 7.5,
    });
    assert.deepEqual(
      toUbahFasilitasBody({
        kapasitasTon: 500,
        suhuMinC: 1,
        suhuMaksC: 6,
        status: "PERAWATAN",
      }),
      {
        kapasitas_ton: 500,
        suhu_min_c: 1,
        suhu_maks_c: 6,
        status: "PERAWATAN",
      },
    );
  });

  it("nilai 0 tetap dikirim (bukan dianggap kosong)", () => {
    assert.deepEqual(toUbahFasilitasBody({ suhuMinC: 0 }), { suhu_min_c: 0 });
  });
});

describe("galatUbahFasilitas", () => {
  it("menolak min >= maks, termasuk saat hanya satu sisi diubah", () => {
    assert.match(
      galatUbahFasilitas({ suhuMinC: 6, suhuMaksC: 6 }) ?? "",
      /minimum harus lebih kecil/,
    );
    assert.match(
      galatUbahFasilitas({ suhuMinC: 8, suhuMaksC: 6 }) ?? "",
      /minimum harus lebih kecil/,
    );
  });

  it("menolak kapasitas bukan bilangan bulat positif dan suhu di luar rentang", () => {
    assert.match(
      galatUbahFasilitas({ kapasitasTon: 0, suhuMinC: 1, suhuMaksC: 6 }) ?? "",
      /Kapasitas/,
    );
    assert.match(
      galatUbahFasilitas({ kapasitasTon: 10.5, suhuMinC: 1, suhuMaksC: 6 }) ??
        "",
      /Kapasitas/,
    );
    assert.match(
      galatUbahFasilitas({ suhuMinC: -90, suhuMaksC: 6 }) ?? "",
      /antara -80 dan 60/,
    );
    assert.match(
      galatUbahFasilitas({ suhuMinC: 1, suhuMaksC: Number.NaN }) ?? "",
      /antara -80 dan 60/,
    );
  });

  it("isian sah lolos", () => {
    assert.equal(
      galatUbahFasilitas({ kapasitasTon: 400, suhuMinC: -22, suhuMaksC: -18 }),
      null,
    );
  });
});

describe("toTelemetriBody", () => {
  it("membungkus satu pembacaan bersumber MANUAL", () => {
    assert.deepEqual(
      toTelemetriBody({
        fasilitasId: "CS-01",
        waktu: "2026-11-05T01:02:03.000Z",
        suhuC: 4.3,
        kelembapanPct: 80,
      }),
      {
        pembacaan: [
          {
            fasilitas_id: "CS-01",
            waktu: "2026-11-05T01:02:03.000Z",
            suhu_c: 4.3,
            kelembapan_pct: 80,
            sumber: "MANUAL",
          },
        ],
      },
    );
  });

  it("kelembapan opsional tidak dikirim bila kosong", () => {
    const body = toTelemetriBody({
      fasilitasId: "CS-01",
      waktu: "2026-11-05T01:02:03.000Z",
      suhuC: 4,
    });
    assert.equal("kelembapan_pct" in (body.pembacaan as object[])[0], false);
  });
});

describe("toHasilTelemetri", () => {
  it("memetakan jumlah dan alarm di luar rentang", () => {
    assert.deepEqual(
      toHasilTelemetri({
        diterima: 1,
        duplikat: 0,
        di_luar_rentang: [
          {
            fasilitas_id: "CS-01",
            waktu: "2026-11-05T01:02:03Z",
            suhu_c: 9,
            batas_min_c: 2,
            batas_maks_c: 6,
          },
        ],
      }),
      {
        diterima: 1,
        duplikat: 0,
        diLuarRentang: [
          {
            waktu: "2026-11-05T01:02:03Z",
            suhuC: 9,
            batasMinC: 2,
            batasMaksC: 6,
          },
        ],
      },
    );
  });

  it("alarm null dari backend dianggap kosong", () => {
    const hasil = toHasilTelemetri({
      diterima: 0,
      duplikat: 1,
      di_luar_rentang: null as never,
    });
    assert.deepEqual(hasil.diLuarRentang, []);
    assert.equal(hasil.duplikat, 1);
  });
});

describe("galatPembacaan", () => {
  const sekarang = new Date("2026-11-05T10:00:00Z");

  it("menolak suhu dan kelembapan di luar rentang", () => {
    assert.match(
      galatPembacaan({ suhuC: 70, waktu: sekarang }, sekarang) ?? "",
      /Suhu/,
    );
    assert.match(
      galatPembacaan({ suhuC: Number.NaN, waktu: sekarang }, sekarang) ?? "",
      /Suhu/,
    );
    assert.match(
      galatPembacaan(
        { suhuC: 4, kelembapanPct: 101, waktu: sekarang },
        sekarang,
      ) ?? "",
      /Kelembapan/,
    );
  });

  it("menolak waktu kosong atau lebih dari 5 menit di masa depan", () => {
    assert.match(
      galatPembacaan({ suhuC: 4, waktu: null }, sekarang) ?? "",
      /wajib/,
    );
    assert.match(
      galatPembacaan(
        { suhuC: 4, waktu: new Date("2026-11-05T10:06:00Z") },
        sekarang,
      ) ?? "",
      /masa depan/,
    );
    assert.equal(
      galatPembacaan(
        { suhuC: 4, waktu: new Date("2026-11-05T10:04:00Z") },
        sekarang,
      ),
      null,
    );
  });
});

describe("datetime-local", () => {
  it("bolak-balik menjaga detik dalam waktu setempat", () => {
    const d = new Date(2026, 10, 5, 8, 9, 37);
    const teks = keDatetimeLocal(d);
    assert.equal(teks, "2026-11-05T08:09:37");
    assert.equal(dariDatetimeLocal(teks)?.getTime(), d.getTime());
  });

  it("kosong atau tidak valid menjadi null", () => {
    assert.equal(dariDatetimeLocal(""), null);
    assert.equal(dariDatetimeLocal("bukan tanggal"), null);
  });
});

describe("angkaIsian", () => {
  it("menerima koma desimal; kosong menjadi NaN", () => {
    assert.equal(angkaIsian("4,5"), 4.5);
    assert.equal(angkaIsian(" -20 "), -20);
    assert.ok(Number.isNaN(angkaIsian("")));
    assert.ok(Number.isNaN(angkaIsian("abc")));
  });
});
