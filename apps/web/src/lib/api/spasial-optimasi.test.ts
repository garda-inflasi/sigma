import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  toOptimasiBody,
  toRencanaOptimasi,
  type OptimasiResponse,
} from "./spasial";

describe("toOptimasiBody", () => {
  it("hanya memuat kolom yang diisi", () => {
    assert.deepEqual(toOptimasiBody({ komoditasKode: "CBM-KRT" }), {
      komoditas_kode: "CBM-KRT",
      penyesuaian: undefined,
      maks_jam_tempuh: undefined,
      simpan: undefined,
    });
  });

  it("menerjemahkan skenario, batas waktu, dan simpan ke nama kontrak", () => {
    assert.deepEqual(
      toOptimasiBody({
        komoditasKode: "BWM",
        maksJamTempuh: 6,
        simpan: true,
        penyesuaian: [{ kabupatenKode: "1106", ton: -50 }],
      }),
      {
        komoditas_kode: "BWM",
        penyesuaian: [{ kabupaten_kode: "1106", ton: -50 }],
        maks_jam_tempuh: 6,
        simpan: true,
      },
    );
  });

  it("penyesuaian kosong dan batas waktu 0 dibuang", () => {
    const body = toOptimasiBody({
      komoditasKode: "X",
      penyesuaian: [],
      maksJamTempuh: 0,
    });

    assert.equal(body.penyesuaian, undefined);
    assert.equal(body.maks_jam_tempuh, undefined);
  });
});

describe("toRencanaOptimasi", () => {
  const respons: OptimasiResponse = {
    komoditas_kode: "CBM-KRT",
    komoditas_nama: "Cabai merah keriting",
    periode: "2026-11-01",
    rute: [
      {
        asal_kode: "1106",
        asal: "KABUPATEN ACEH TENGAH",
        tujuan_kode: "1171",
        tujuan: "KOTA BANDA ACEH",
        volume_ton: 20,
        jarak_km: 410.5,
        jam_tempuh: 9.1,
        biaya: 34_482_000,
      },
    ],
    total_volume_ton: 20,
    total_biaya: 34_482_000,
    belum_terpenuhi: [{ kode: "1108", nama: "KABUPATEN ACEH BESAR", ton: 5.5 }],
    surplus_sisa: [],
    biaya_pembanding: 40_000_000,
    penghematan_persen: 13.8,
    optimasi_id: null,
  };

  it("memformat nama wilayah dan memetakan rute", () => {
    const r = toRencanaOptimasi(respons);

    assert.equal(r.rute[0].asal, "Kab. Aceh Tengah");
    assert.equal(r.rute[0].tujuan, "Kota Banda Aceh");
    assert.equal(r.rute[0].jarakKm, 410.5);
    assert.equal(r.rute[0].komoditas, "Cabai merah keriting");
    assert.deepEqual(r.belumTerpenuhi, [
      { kode: "1108", nama: "Kab. Aceh Besar", ton: 5.5 },
    ]);
    assert.equal(r.optimasiId, null);
    assert.equal(r.penghematanPersen, 13.8);
  });

  it("meneruskan id rencana yang disimpan", () => {
    assert.equal(
      toRencanaOptimasi({ ...respons, optimasi_id: "OPT-1" }).optimasiId,
      "OPT-1",
    );
  });
});
