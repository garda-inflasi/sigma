import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  createGangguanJalan,
  hitungJarakSpasial,
  listGangguanJalan,
  updateGangguanJalan,
} from "./spasial-jarak";

describe("spasial-jarak road routing API", () => {
  it("mengembalikan jalur utama + alternatif dalam mode simulasi dan mengalihkan otomatis saat jalur terblokir", async () => {
    const asal = { lat: 4.627, lng: 96.846 }; // Takengon
    const tujuan = { lat: 5.5577, lng: 95.3222 }; // Banda Aceh

    const normal = await hitungJarakSpasial({
      asalLat: asal.lat,
      asalLng: asal.lng,
      tujuanLat: tujuan.lat,
      tujuanLng: tujuan.lng,
    });
    assert.equal(normal.jalur_utama_diblokir, false);
    assert.ok((normal.rute?.length ?? 0) >= 2);
    assert.equal(normal.rute?.[0].tipe, "Jalur Utama");
    assert.equal(normal.rute?.[1].tipe, "Jalur Alternatif");

    const coords = normal.rute![0].geometry.coordinates;
    const mid = coords[1];
    const dibuat = await createGangguanJalan({
      judul: "Longsor Uji Koridor",
      jenis: "LONGSOR",
      tingkat: "TUTUP_TOTAL",
      ruas_jalan: "Ruas Uji",
      lat: mid[1],
      lng: mid[0],
      radius_m: 1500,
    });

    const aktif = await listGangguanJalan(true);
    assert.equal(
      aktif.some((g) => g.id === dibuat.id),
      true,
    );

    const teralihkan = await hitungJarakSpasial({
      asalLat: asal.lat,
      asalLng: asal.lng,
      tujuanLat: tujuan.lat,
      tujuanLng: tujuan.lng,
    });
    assert.equal(teralihkan.jalur_utama_diblokir, true);
    assert.equal(teralihkan.rute_diblokir?.tipe, "Jalur Terdampak");
    assert.equal(teralihkan.rute?.[0].tipe, "Jalur Pengalihan");
    assert.equal(
      teralihkan.gangguan_terkait?.some((g) => g.id === dibuat.id),
      true,
    );

    await updateGangguanJalan(dibuat.id, { aktif: false });
    const setelahSelesai = await hitungJarakSpasial({
      asalLat: asal.lat,
      asalLng: asal.lng,
      tujuanLat: tujuan.lat,
      tujuanLng: tujuan.lng,
    });
    assert.equal(setelahSelesai.jalur_utama_diblokir, false);
  });
});
