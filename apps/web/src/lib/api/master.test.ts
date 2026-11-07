import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  ringkasDitolak,
  toHalaman,
  toKecamatanQuery,
  toKecamatanRow,
  toKomoditasDetail,
  toMasterBulkHasil,
  toMasterKomoditasRow,
  toMasterPasarRow,
  toMasterWilayahRow,
  toPasarDetail,
  toPedagangQuery,
  toPedagangRow,
  toSatuanRow,
  toWilayahDetail,
} from "./master";

describe("toMasterKomoditasRow", () => {
  it("memetakan kolom API ke baris UI", () => {
    const row = toMasterKomoditasRow({
      kode: "CBM-KRT",
      nama: "Cabai merah keriting",
      kategori: "Hortikultura",
      satuan: "kg",
      harga_acuan: 52000,
      pilot: true,
      shelf_life_hari: 21,
      versi: 2,
      status: "MENUNGGU_PERSETUJUAN",
    });

    assert.deepEqual(row, {
      id: "CBM-KRT",
      kode: "CBM-KRT",
      nama: "Cabai merah keriting",
      kategori: "Hortikultura",
      satuan: "kg",
      hargaAcuan: 52000,
      pilot: true,
      shelfLifeHari: 21,
      versi: 2,
      status: "MENUNGGU_PERSETUJUAN",
    });
  });

  it("status tak dikenal dianggap AKTIF", () => {
    const row = toMasterKomoditasRow({
      kode: "X",
      nama: "X",
      kategori: "k",
      satuan: "kg",
      harga_acuan: 1,
      pilot: false,
      shelf_life_hari: 1,
      versi: 1,
      status: "ARSIP",
    });

    assert.equal(row.status, "AKTIF");
  });
});

describe("toMasterWilayahRow", () => {
  it("nama resmi menjadi label ringkas", () => {
    const kab = toMasterWilayahRow({
      kode: "1108",
      nama: "KABUPATEN ACEH BESAR",
      jenis: "Kabupaten",
      ibukota: "Kota Jantho",
      lat: 5.3,
      lng: 95.63,
      provinsi_kode: "11",
    });
    const kota = toMasterWilayahRow({
      kode: "1173",
      nama: "KOTA LANGSA",
      jenis: "Kota",
      ibukota: "Langsa",
      lat: 4.47,
      lng: 97.97,
      provinsi_kode: "11",
    });

    assert.equal(kab.nama, "Kab. Aceh Besar");
    assert.equal(kota.nama, "Kota Langsa");
    assert.equal(kab.ibukota, "Kota Jantho");
  });
});

describe("toMasterPasarRow", () => {
  const pasar = {
    id: "PSR-1171-01",
    nama: "Pasar Peunayong",
    kabupaten_kode: "1171",
    lat: 5.56,
    lng: 95.32,
    radius_meter: 400,
    aktif: true,
  };

  it("memakai nama kabupaten dari peta", () => {
    const row = toMasterPasarRow(pasar, new Map([["1171", "Kota Banda Aceh"]]));

    assert.equal(row.kabupaten, "Kota Banda Aceh");
    assert.equal(row.radiusMeter, 400);
    assert.equal(row.aktif, true);
  });

  it("kabupaten tak dikenal jatuh ke kode", () => {
    assert.equal(toMasterPasarRow(pasar, new Map()).kabupaten, "1171");
  });

  it("server lama tanpa kecamatan dan versi: kecamatan kosong, versi 1", () => {
    const row = toMasterPasarRow(pasar, new Map());

    assert.equal(row.kecamatanKode, null);
    assert.equal(row.kecamatanNama, null);
    assert.equal(row.versi, 1);
    assert.equal(row.kabupatenKode, "1171");
    assert.equal(row.lat, 5.56);
  });

  it("memetakan kecamatan dan versi dari server baru", () => {
    const row = toMasterPasarRow(
      {
        ...pasar,
        kecamatan_kode: "1171060",
        kecamatan_nama: "Kuta Alam",
        versi: 3,
      },
      new Map(),
    );

    assert.equal(row.kecamatanKode, "1171060");
    assert.equal(row.kecamatanNama, "Kuta Alam");
    assert.equal(row.versi, 3);
  });
});

describe("toKecamatanRow dan kueri", () => {
  it("memetakan baris dan meringkas nama kabupaten resmi", () => {
    assert.deepEqual(
      toKecamatanRow({
        kode: "1171060",
        nama: "Kuta Alam",
        kabupaten_kode: "1171",
        kabupaten_nama: "KOTA BANDA ACEH",
        kode_dagri: "11.71.06",
      }),
      {
        id: "1171060",
        kode: "1171060",
        nama: "Kuta Alam",
        kabupatenKode: "1171",
        kabupatenNama: "Kota Banda Aceh",
        kodeDagri: "11.71.06",
      },
    );
  });

  it("kode_dagri yang tidak ada menjadi null", () => {
    const row = toKecamatanRow({
      kode: "1171010",
      nama: "Meuraxa",
      kabupaten_kode: "1171",
      kabupaten_nama: "KOTA BANDA ACEH",
    });
    assert.equal(row.kodeDagri, null);
  });

  it("kueri hanya memuat parameter yang terisi dan memangkas kata kunci", () => {
    assert.equal(toKecamatanQuery({}), "");
    assert.equal(
      toKecamatanQuery({
        kabupatenKode: "1171",
        q: "  kuta ",
        page: 2,
        limit: 25,
      }),
      "?kabupaten_kode=1171&q=kuta&page=2&limit=25",
    );
    assert.equal(toKecamatanQuery({ q: "   " }), "");
  });
});

describe("toSatuanRow dan toPedagangRow", () => {
  it("memetakan satuan", () => {
    assert.deepEqual(
      toSatuanRow({ kode: "kg", nama: "Kilogram", jumlah_komoditas: 9 }),
      { id: "kg", kode: "kg", nama: "Kilogram", jumlahKomoditas: 9 },
    );
  });

  it("pedagang: id BIGINT string, nama kosong menjadi null", () => {
    const dasar = {
      id: 12,
      pasar_id: "PSR-1171-01",
      pasar_nama: "Pasar Peunayong",
      kode: "PDG-1",
      aktif: true,
      jumlah_entri: 40,
    };

    assert.deepEqual(toPedagangRow({ ...dasar, nama: "Bu Aminah" }), {
      id: "12",
      pasarId: "PSR-1171-01",
      pasarNama: "Pasar Peunayong",
      kode: "PDG-1",
      nama: "Bu Aminah",
      aktif: true,
      jumlahEntri: 40,
    });
    assert.equal(toPedagangRow({ ...dasar, nama: "  " }).nama, null);
    assert.equal(toPedagangRow({ ...dasar }).nama, null);
  });

  it("kueri pedagang: pasar, kata kunci, dan status aktif (false tetap dikirim)", () => {
    assert.equal(
      toPedagangQuery({
        pasarId: "PSR-1171-01",
        q: " ami ",
        aktif: false,
        page: 1,
        limit: 50,
      }),
      "?pasar_id=PSR-1171-01&q=ami&aktif=false&page=1&limit=50",
    );
    assert.equal(toPedagangQuery({}), "");
  });
});

describe("toHalaman", () => {
  it("memakai total dari header", () => {
    const h = toHalaman([1, 2, 3], 290, 2, 25, (n) => n * 2);
    assert.deepEqual(h, { rows: [2, 4, 6], total: 290 });
  });

  it("tanpa header: halaman penuh menandakan masih ada halaman berikutnya", () => {
    assert.equal(toHalaman([1, 2], null, 1, 2, (n) => n).total, 3);
    assert.equal(toHalaman([1], null, 3, 2, (n) => n).total, 5);
    assert.equal(toHalaman([], null, 1, 25, (n) => n).total, 0);
  });
});

describe("detail master", () => {
  it("komoditas: riwayat versi dan usulan menunggu dipetakan; ketiadaannya aman", () => {
    const dasar = {
      kode: "BWM",
      nama: "Bawang merah",
      kategori: "Hortikultura",
      satuan: "kg",
      harga_acuan: 41000,
      pilot: true,
      shelf_life_hari: 60,
      versi: 2,
      status: "AKTIF",
    };
    const kosong = toKomoditasDetail(dasar);
    assert.deepEqual(kosong.riwayat, []);
    assert.equal(kosong.menunggu, null);
    assert.equal(kosong.komoditas.shelfLifeHari, 60);

    const detail = toKomoditasDetail({
      ...dasar,
      riwayat_versi: [
        {
          id: 7,
          versi: 2,
          nama: "Bawang merah",
          kategori: "Hortikultura",
          satuan: "kg",
          harga_acuan: 41000,
          shelf_life_hari: 60,
          pilot: true,
          berlaku_dari: "2026-05-01",
          berlaku_sampai: null,
          perubahan_id: 31,
        },
      ],
      perubahan_menunggu: {
        id: "31",
        entitas: "komoditas",
        entitas_id: "BWM",
        jenis: "UBAH",
        versi: 3,
        status: "MENUNGGU",
        dibuat_pada: "2026-10-01T00:00:00Z",
      },
    });
    assert.equal(detail.riwayat[0].id, "7");
    assert.equal(detail.riwayat[0].perubahanId, "31");
    assert.equal(detail.menunggu?.id, "31");
  });

  it("pasar: nama kabupaten dari respons detail dan jumlah pedagang", () => {
    const d = toPasarDetail({
      id: "PSR-1171-01",
      nama: "Pasar Peunayong",
      kabupaten_kode: "1171",
      lat: 5.56,
      lng: 95.32,
      radius_meter: 400,
      aktif: true,
      kecamatan_kode: "1171060",
      kecamatan_nama: "Kuta Alam",
      versi: 2,
      kabupaten_nama: "KOTA BANDA ACEH",
      jumlah_pedagang: 3,
    });
    assert.equal(d.pasar.kabupaten, "Kota Banda Aceh");
    assert.equal(d.pasar.kecamatanNama, "Kuta Alam");
    assert.equal(d.jumlahPedagang, 3);
  });

  it("wilayah: jumlah kecamatan dan pasar", () => {
    const d = toWilayahDetail({
      kode: "1171",
      nama: "KOTA BANDA ACEH",
      jenis: "Kota",
      ibukota: "Banda Aceh",
      lat: 5.55,
      lng: 95.32,
      provinsi_kode: "11",
      jumlah_kecamatan: 9,
      jumlah_pasar: 2,
    });
    assert.equal(d.wilayah.nama, "Kota Banda Aceh");
    assert.equal(d.jumlahKecamatan, 9);
    assert.equal(d.jumlahPasar, 2);
  });
});

describe("toMasterBulkHasil", () => {
  it("memisahkan kode yang ditolak beserta alasannya", () => {
    const hasil = toMasterBulkHasil({
      berhasil: 1,
      gagal: 2,
      rincian: [
        { kode: "A", berhasil: true },
        { kode: "B", berhasil: false, pesan: "Komoditas tidak ditemukan" },
        { kode: "C", berhasil: false },
      ],
    });

    assert.equal(hasil.berhasil, 1);
    assert.equal(hasil.gagal, 2);
    assert.deepEqual(hasil.ditolak, [
      { kode: "B", pesan: "Komoditas tidak ditemukan" },
      { kode: "C", pesan: "Ditolak" },
    ]);
  });

  it("rincian kosong tidak menimbulkan galat", () => {
    assert.deepEqual(
      toMasterBulkHasil({ berhasil: 0, gagal: 0, rincian: undefined as never })
        .ditolak,
      [],
    );
  });
});

describe("ringkasDitolak", () => {
  it("meringkas lebih dari batas", () => {
    const ditolak = ["A", "B", "C", "D", "E"].map((kode) => ({
      kode,
      pesan: "x",
    }));
    assert.equal(ringkasDitolak(ditolak), "A: x; B: x; C: x; dan 2 lainnya");
  });

  it("kosong menghasilkan teks kosong", () => {
    assert.equal(ringkasDitolak([]), "");
  });
});
