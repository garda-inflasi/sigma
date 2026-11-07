import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  BATAS_SKEMA,
  keteranganLokasi,
  pesanGalatGeolokasi,
  ringkasNeraca,
  statusNeraca,
  titikDariSkema,
  toLokasi,
  toLokasiQuery,
  toNeraca,
  toNeracaQuery,
  toRiwayatOptimasi,
  type NeracaResponse,
} from "./spasial";
import { mockLokasi, mockNeraca, mockRiwayatOptimasi } from "./spasial-mock";

const neraca = (ton: number, sumber = "DEMO"): NeracaResponse => ({
  komoditas_kode: "CBM-KRT",
  komoditas_nama: "Cabai merah keriting",
  kabupaten_kode: "1171",
  wilayah: "KOTA BANDA ACEH",
  periode: "2026-11-01",
  surplus_defisit_ton: ton,
  delta_harga_pct: -2.5,
  sumber,
});

describe("statusNeraca", () => {
  it("positif surplus, negatif defisit, nol seimbang", () => {
    assert.equal(statusNeraca(12.5), "surplus");
    assert.equal(statusNeraca(-0.1), "defisit");
    assert.equal(statusNeraca(0), "seimbang");
  });
});

describe("toNeraca", () => {
  it("memetakan baris API, meringkas nama wilayah, dan menandai data demo", () => {
    assert.deepEqual(toNeraca(neraca(-31.2)), {
      id: "CBM-KRT-1171",
      komoditasKode: "CBM-KRT",
      komoditas: "Cabai merah keriting",
      kabupatenKode: "1171",
      wilayah: "Kota Banda Aceh",
      periode: "2026-11-01",
      surplusDefisitTon: -31.2,
      deltaHargaPct: -2.5,
      sumber: "DEMO",
      demo: true,
      status: "defisit",
    });
  });

  it("sumber resmi tidak ditandai demo", () => {
    const n = toNeraca(neraca(10, "BPS"));
    assert.equal(n.demo, false);
    assert.equal(n.status, "surplus");
  });
});

describe("toNeracaQuery", () => {
  it("membuang parameter kosong", () => {
    assert.equal(toNeracaQuery(), "");
    assert.equal(toNeracaQuery(""), "");
    assert.equal(toNeracaQuery("CBM-KRT"), "?komoditas=CBM-KRT");
    assert.equal(
      toNeracaQuery("BWM", "2026-10-01"),
      "?komoditas=BWM&periode=2026-10-01",
    );
  });
});

describe("ringkasNeraca", () => {
  it("menjumlahkan surplus dan defisit terpisah", () => {
    const r = ringkasNeraca([
      toNeraca(neraca(10)),
      toNeraca(neraca(5.5)),
      toNeraca(neraca(-4)),
      toNeraca(neraca(0)),
    ]);
    assert.deepEqual(r, {
      surplusTon: 15.5,
      defisitTon: 4,
      jumlahSurplus: 2,
      jumlahDefisit: 1,
    });
  });

  it("daftar kosong menghasilkan nol", () => {
    assert.deepEqual(ringkasNeraca([]), {
      surplusTon: 0,
      defisitTon: 0,
      jumlahSurplus: 0,
      jumlahDefisit: 0,
    });
  });
});

describe("lokasi", () => {
  it("memetakan respons dan meringkas nama", () => {
    assert.deepEqual(
      toLokasi({
        kode: "1171",
        nama: "KOTA BANDA ACEH",
        di_dalam: true,
        jarak_pusat_km: 1.24,
      }),
      {
        kode: "1171",
        nama: "Kota Banda Aceh",
        diDalam: true,
        jarakPusatKm: 1.24,
      },
    );
  });

  it("query memakai titik desimal dan tanda minus apa adanya", () => {
    assert.equal(toLokasiQuery(5.5483, 95.3238), "?lat=5.5483&lng=95.3238");
    assert.equal(toLokasiQuery(-6.2, 106.8), "?lat=-6.2&lng=106.8");
  });

  it("keterangan membedakan di dalam batas dan terdekat", () => {
    const dalam = keteranganLokasi({
      kode: "1171",
      nama: "Kota Banda Aceh",
      diDalam: true,
      jarakPusatKm: 1.24,
    });
    assert.match(dalam, /^di dalam batas \(perkiraan\)/);
    const luar = keteranganLokasi({
      kode: "1171",
      nama: "Kota Banda Aceh",
      diDalam: false,
      jarakPusatKm: 12.34,
    });
    assert.match(luar, /^terdekat, 12,3 km dari pusat$/);
  });

  it("pesan galat geolokasi menurut kode", () => {
    assert.match(pesanGalatGeolokasi(1), /Izin lokasi ditolak/);
    assert.match(pesanGalatGeolokasi(2), /tidak dapat ditentukan/);
    assert.match(pesanGalatGeolokasi(3), /batas waktu/);
    assert.match(pesanGalatGeolokasi(99), /Gagal menentukan lokasi/);
  });
});

describe("titikDariSkema", () => {
  it("sudut kotak menjadi batas skema", () => {
    assert.deepEqual(titikDariSkema(0, 0), {
      lat: BATAS_SKEMA.maxLat,
      lng: BATAS_SKEMA.minLng,
    });
    const sudut = titikDariSkema(1, 1);
    assert.ok(Math.abs(sudut.lat - BATAS_SKEMA.minLat) < 1e-9);
    assert.ok(Math.abs(sudut.lng - BATAS_SKEMA.maxLng) < 1e-9);
  });

  it("kebalikan dari proyeksi persen yang dipakai peta", () => {
    const lat = 5.5483;
    const lng = 95.3238;
    const x =
      (lng - BATAS_SKEMA.minLng) / (BATAS_SKEMA.maxLng - BATAS_SKEMA.minLng);
    const y =
      (BATAS_SKEMA.maxLat - lat) / (BATAS_SKEMA.maxLat - BATAS_SKEMA.minLat);
    const t = titikDariSkema(x, y);
    assert.ok(Math.abs(t.lat - lat) < 1e-9);
    assert.ok(Math.abs(t.lng - lng) < 1e-9);
  });
});

describe("toRiwayatOptimasi", () => {
  it("memetakan ringkasan rencana, termasuk pembuat yang sudah terhapus", () => {
    const base = {
      id: "7",
      kode: "OPT-000007",
      komoditas_kode: "BWM",
      komoditas_nama: "Bawang merah",
      periode: "2026-11-01",
      total_volume_ton: 54,
      total_biaya: 61_200_000,
      belum_terpenuhi_ton: 3.5,
      dibuat_pada: "2026-11-02T14:40:00Z",
    };
    assert.deepEqual(
      toRiwayatOptimasi({ ...base, dibuat_oleh: "Manajer Data" }),
      {
        id: "7",
        kode: "OPT-000007",
        komoditasKode: "BWM",
        komoditas: "Bawang merah",
        periode: "2026-11-01",
        totalVolumeTon: 54,
        totalBiaya: 61_200_000,
        belumTerpenuhiTon: 3.5,
        dibuatOleh: "Manajer Data",
        dibuatPada: "2026-11-02T14:40:00Z",
      },
    );
    assert.equal(
      toRiwayatOptimasi({ ...base, dibuat_oleh: null }).dibuatOleh,
      null,
    );
  });
});

describe("mock spasial", () => {
  it("neraca mock mengikuti filter komoditas dan berlabel demo", () => {
    const semua = mockNeraca();
    const satu = mockNeraca("CBM-KRT");
    assert.ok(satu.length > 0 && satu.length < semua.length);
    assert.ok(satu.every((n) => n.komoditasKode === "CBM-KRT" && n.demo));
    assert.equal(new Set(semua.map((n) => n.id)).size, semua.length);
    assert.deepEqual(mockNeraca("TIDAK-ADA"), []);
  });

  it("lokasi mock memilih pusat wilayah terdekat", () => {
    const l = mockLokasi(5.5483, 95.3238);
    assert.equal(l.kode, "1171");
    assert.ok(l.jarakPusatKm < 10);
  });

  it("riwayat mock berisi rencana yang valid", () => {
    const r = mockRiwayatOptimasi();
    assert.ok(r.length > 0 && r.every((x) => x.totalVolumeTon >= 0 && x.id));
  });
});
