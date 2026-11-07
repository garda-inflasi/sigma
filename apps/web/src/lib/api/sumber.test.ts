import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { formatJadwal, toEksekusi, toPipeline, toSampel, toSumberData, toSumberDetail, type EksekusiResponse, type SumberDataResponse } from "./sumber";

describe("formatJadwal", () => {
  it("menerjemahkan pola cron katalog ke teks Indonesia", () => {
    assert.equal(formatJadwal("0 6 * * *"), "Harian 06.00");
    assert.equal(formatJadwal("0 11 * * *"), "Harian 11.00");
    assert.equal(formatJadwal("0 7 * * 1"), "Mingguan Senin 07.00");
    assert.equal(formatJadwal("0 */3 * * *"), "Tiap 3 jam");
    assert.equal(formatJadwal("0 * * * *"), "Tiap jam");
    assert.equal(formatJadwal("*/5 * * * *"), "Tiap 5 menit");
  });

  it("menampilkan cron apa adanya bila polanya tidak dikenal", () => {
    assert.equal(formatJadwal("0 6 1 * *"), "0 6 1 * *");
    assert.equal(formatJadwal("bukan cron"), "bukan cron");
  });
});

const dasar: SumberDataResponse = {
  id: "40745615-f152-32aa-b1e3-f54add179514",
  kode: "SRC-BI",
  nama: "PIHPS Nasional",
  penyedia: "Bank Indonesia",
  jenis: "API",
  cakupan: "Pasar tradisional",
  jadwal_cron: "0 10 * * *",
  status: "SEHAT",
  terakhir_jalan: "2026-09-30T08:00:00Z",
  keberhasilan_7h: 98.6,
  latensi_ms: 910,
  robots_txt_patuh: true,
  atribusi: "Sumber: BI",
  url_dasar: null,
  bisa_dijalankan: true,
};

describe("toSumberData", () => {
  it("memetakan kolom API ke model UI", () => {
    assert.deepEqual(toSumberData(dasar), {
      id: "40745615-f152-32aa-b1e3-f54add179514",
      kode: "SRC-BI",
      nama: "PIHPS Nasional",
      penyedia: "Bank Indonesia",
      jenis: "API",
      cakupan: "Pasar tradisional",
      jadwal: "Harian 10.00",
      terakhir: "2026-09-30T08:00:00Z",
      status: "SEHAT",
      keberhasilan7h: 98.6,
      latensiMs: 910,
      bisaDijalankan: true,
    });
  });

  it("mempertahankan null untuk sumber yang belum pernah berjalan", () => {
    const row = toSumberData({
      ...dasar,
      status: "BELUM_AKTIF",
      terakhir_jalan: null,
      keberhasilan_7h: null,
      latensi_ms: 0,
    });
    assert.equal(row.status, "BELUM_AKTIF");
    assert.equal(row.terakhir, null);
    assert.equal(row.keberhasilan7h, null);
  });

  it("status dan jenis tak dikenal jatuh ke nilai aman", () => {
    const row = toSumberData({ ...dasar, status: "ANEH", jenis: "LAIN" });
    assert.equal(row.status, "BELUM_AKTIF");
    assert.equal(row.jenis, "API");
  });
});

describe("toEksekusi", () => {
  const dasarEksekusi: EksekusiResponse = {
    id: "7",
    pemicu: "MANUAL",
    status: "SEBAGIAN",
    mulai: "2026-10-15T03:00:00Z",
    selesai: "2026-10-15T03:00:09Z",
    durasi_ms: 9000,
    latensi_ms: 850,
    dokumen: 2,
    baris_diambil: 14,
    baris_ditolak: 7,
    galat: "wilayah=102: timeout",
    ringkasan: {
      tak_terpetakan: { "KOMODITAS:ikan segar baru": 3, "KOMODITAS:apel": 3, "KOMODITAS:zat x": 5 },
      tidak_valid: { "harga kosong atau bukan angka": 3, "harga di luar batas wajar": 1 },
    },
  };

  it("memetakan kolom dan mengurutkan rincian penolakan menurut jumlah baris lalu abjad", () => {
    const e = toEksekusi(dasarEksekusi);
    assert.equal(e.status, "SEBAGIAN");
    assert.equal(e.durasiMs, 9000);
    assert.equal(e.barisDitolak, 7);
    assert.deepEqual(e.takTerpetakan, [
      { alias: "KOMODITAS:zat x", baris: 5 },
      { alias: "KOMODITAS:apel", baris: 3 },
      { alias: "KOMODITAS:ikan segar baru", baris: 3 },
    ]);
    assert.deepEqual(e.tidakValid[0], { alasan: "harga kosong atau bukan angka", baris: 3 });
  });

  it("menerima run berjalan tanpa ringkasan", () => {
    const e = toEksekusi({ ...dasarEksekusi, status: "BERJALAN", selesai: null, durasi_ms: null, latensi_ms: null, galat: null, ringkasan: {} });
    assert.equal(e.selesai, null);
    assert.deepEqual(e.takTerpetakan, []);
    assert.deepEqual(e.tidakValid, []);
  });
});

describe("toSampel", () => {
  it("memetakan cuplikan payload mentah", () => {
    assert.deepEqual(toSampel({ id: "3", kunci_permintaan: "wilayah=101", diambil_pada: "2026-10-15T03:00:00Z", ukuran: 5231, cuplikan: '{"data":[' }), {
      id: "3",
      kunci: "wilayah=101",
      diambilPada: "2026-10-15T03:00:00Z",
      ukuran: 5231,
      cuplikan: '{"data":[',
    });
  });
});

describe("toPipeline", () => {
  it("memetakan hitungan lapisan dan mutu, mempertahankan null", () => {
    assert.deepEqual(
      toPipeline({ bronze: 3, silver: 20, gold: 5, demo: true, mutu: { kelengkapan: 97.5, akurasi: null, ketepatan_waktu: 66.7, konsistensi: 88 } }),
      { bronze: 3, silver: 20, gold: 5, demo: true, mutu: { kelengkapan: 97.5, akurasi: null, ketepatanWaktu: 66.7, konsistensi: 88 } },
    );
  });
});

describe("toSumberDetail", () => {
  it("memuat kolom daftar ditambah cron mentah, robots.txt, atribusi, dan URL dasar", () => {
    const d = toSumberDetail({ ...dasar, url_dasar: "https://pihps.example", robots_txt_patuh: false });
    assert.equal(d.nama, toSumberData(dasar).nama);
    assert.equal(d.keberhasilan7h, 98.6);
    assert.equal(d.jadwalCron, "0 10 * * *");
    assert.equal(d.jadwal, "Harian 10.00");
    assert.equal(d.robotsTxtPatuh, false);
    assert.equal(d.atribusi, "Sumber: BI");
    assert.equal(d.urlDasar, "https://pihps.example");
  });

  it("URL dasar kosong tetap null", () => {
    assert.equal(toSumberDetail(dasar).urlDasar, null);
  });
});
