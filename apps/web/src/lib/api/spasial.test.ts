import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { toKandidat, toMarker, toRute } from "./spasial";

describe("toMarker", () => {
  it("memetakan marker API dan meringkas nama wilayah", () => {
    assert.deepEqual(
      toMarker({
        kode: "1171",
        nama: "KOTA BANDA ACEH",
        lat: 5.55,
        lng: 95.32,
        delta: 21.4,
        komoditas: "Cabai merah keriting",
        status: "defisit",
        punya_pasar: true,
      }),
      {
        kode: "1171",
        nama: "Kota Banda Aceh",
        lat: 5.55,
        lng: 95.32,
        delta: 21.4,
        komoditas: "Cabai merah keriting",
        status: "defisit",
        punyaPasar: true,
      },
    );
  });

  it("nama kabupaten diringkas menjadi Kab.", () => {
    const m = toMarker({
      kode: "1106",
      nama: "KABUPATEN ACEH TENGAH",
      lat: 4.62,
      lng: 96.85,
      delta: -18,
      komoditas: "Kentang",
      status: "surplus",
      punya_pasar: true,
    });
    assert.equal(m.nama, "Kab. Aceh Tengah");
  });
});

describe("toRute", () => {
  it("memetakan rute dengan nama asal dan tujuan ringkas", () => {
    assert.deepEqual(
      toRute({
        id: "RT-1106-1171",
        asal: "KABUPATEN ACEH TENGAH",
        tujuan: "KOTA BANDA ACEH",
        komoditas_nama: "Cabai merah keriting",
        volume_ton: 42,
        jarak_km: 330,
        jam_tempuh: 7.3,
        biaya: 58000000,
      }),
      {
        id: "RT-1106-1171",
        asal: "Kab. Aceh Tengah",
        tujuan: "Kota Banda Aceh",
        komoditas: "Cabai merah keriting",
        volumeTon: 42,
        jarakKm: 330,
        jamTempuh: 7.3,
        biaya: 58000000,
      },
    );
  });
});

describe("toKandidat", () => {
  it("status RENCANA menjadi rencana, lainnya usulan baru", () => {
    const dasar = { id: "KC-1", lokasi: "Kota Langsa", skor: 82, alasan: "x" };
    assert.equal(toKandidat({ ...dasar, status: "RENCANA" }).rencana, true);
    assert.equal(
      toKandidat({ ...dasar, status: "USULAN_BARU" }).rencana,
      false,
    );
  });
});
