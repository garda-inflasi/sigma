import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { MasterKomoditasRow, MasterPasarRow } from "@/lib/data/types";
import { formatRupiah } from "@/lib/format";
import { ApiError } from "./client";
import {
  PESAN_KODE_GALAT,
  adalahDibatalkan,
  bodyUsulan,
  bolehBatalkan,
  formatNilaiBidang,
  isianKomoditasDari,
  isianKomoditasKosong,
  isianPasarDari,
  isianPasarKosong,
  kodeGalatMaster,
  labelBidang,
  parseBulat,
  parseDesimal,
  pengajuSendiri,
  perubahanAntarVersi,
  perubahanKomoditas,
  perubahanPasar,
  pesanGalatMaster,
  queryString,
  ringkasDiff,
  statusTampil,
  toPerubahanMaster,
  toPerubahanQuery,
  toRiwayatVersi,
  validasiAlasan,
  validasiKomoditas,
  validasiPasar,
  type PerubahanMasterResponse,
} from "./master-perubahan";

const respons: PerubahanMasterResponse = {
  id: "31",
  kode: "PM-0031",
  entitas: "komoditas",
  entitas_id: "BWM",
  entitas_nama: "Bawang merah",
  jenis: "UBAH",
  versi: 3,
  status: "MENUNGGU",
  alasan: "Naik mengikuti BPS",
  alasan_penolakan: null,
  perubahan: { harga_acuan: 43500, pilot: false },
  sebelum: { harga_acuan: 41000, pilot: true },
  diff: [
    { bidang: "harga_acuan", sebelum: 41000, sesudah: 43500 },
    { bidang: "pilot", sebelum: true, sesudah: false },
  ],
  diajukan_oleh: { id: "u-1", nama: "Fajar Ramadhan" },
  dibuat_pada: "2026-10-01T03:00:00Z",
  disetujui_oleh: null,
  disetujui_pada: null,
  ditolak_oleh: null,
  ditolak_pada: null,
};

describe("toPerubahanMaster", () => {
  it("memetakan semua bidang termasuk diff dan pengaju", () => {
    const p = toPerubahanMaster(respons);

    assert.equal(p.id, "31");
    assert.equal(p.kode, "PM-0031");
    assert.equal(p.entitas, "komoditas");
    assert.equal(p.entitasId, "BWM");
    assert.equal(p.entitasNama, "Bawang merah");
    assert.equal(p.jenis, "UBAH");
    assert.equal(p.versi, 3);
    assert.equal(p.status, "MENUNGGU");
    assert.equal(p.alasan, "Naik mengikuti BPS");
    assert.deepEqual(p.diff, [
      { bidang: "harga_acuan", sebelum: 41000, sesudah: 43500 },
      { bidang: "pilot", sebelum: true, sesudah: false },
    ]);
    assert.deepEqual(p.diajukanOleh, { id: "u-1", nama: "Fajar Ramadhan" });
    assert.equal(p.disetujuiOleh, null);
    assert.equal(p.ditolakPada, null);
  });

  it("nilai kosong: diff null jadi daftar kosong, objek null jadi {}, pelaku null tetap null", () => {
    const p = toPerubahanMaster({
      id: "40",
      entitas: "pasar",
      entitas_id: "PSR-1171-05",
      jenis: "TINJAU",
      versi: 1,
      status: "DISETUJUI",
      perubahan: null,
      sebelum: null,
      diff: null,
      diajukan_oleh: null,
      dibuat_pada: "2026-10-01T03:00:00Z",
    });

    assert.deepEqual(p.diff, []);
    assert.deepEqual(p.perubahan, {});
    assert.deepEqual(p.sebelum, {});
    assert.equal(p.diajukanOleh, null);
    assert.equal(p.entitasNama, null);
    assert.equal(p.kode, null);
    assert.equal(p.alasan, null);
    assert.equal(p.entitas, "pasar");
  });

  it("sebelum yang tidak ada (usulan tambah) menjadi null, sesudah dipertahankan", () => {
    const p = toPerubahanMaster({
      ...respons,
      jenis: "TAMBAH",
      diff: [
        { bidang: "nama", sesudah: "Kedelai" },
        { bidang: "kecamatan_kode", sebelum: null, sesudah: null },
      ],
    });

    assert.deepEqual(p.diff[0], {
      bidang: "nama",
      sebelum: null,
      sesudah: "Kedelai",
    });
    assert.equal(p.diff[1].sesudah, null);
  });

  it("jenis dan status tak dikenal jatuh ke bawaan; ID numerik jadi string", () => {
    const p = toPerubahanMaster({
      ...respons,
      id: 31 as unknown as string,
      jenis: "HAPUS",
      status: "ENTAH",
    });

    assert.equal(p.id, "31");
    assert.equal(p.jenis, "UBAH");
    assert.equal(p.status, "MENUNGGU");
  });

  it("pelaku tanpa id dianggap tidak ada", () => {
    const p = toPerubahanMaster({
      ...respons,
      diajukan_oleh: { id: "", nama: "x" },
    });
    assert.equal(p.diajukanOleh, null);
  });
});

describe("label dan tampilan bidang", () => {
  it("label Indonesia untuk bidang yang dikenal", () => {
    assert.equal(labelBidang("harga_acuan"), "Harga acuan");
    assert.equal(labelBidang("shelf_life_hari"), "Umur simpan (hari)");
    assert.equal(labelBidang("radius_meter"), "Radius geofence (m)");
    assert.equal(labelBidang("kecamatan_kode"), "Kecamatan");
  });

  it("bidang tak dikenal dihumanisasi", () => {
    assert.equal(labelBidang("tipe_pasar"), "Tipe pasar");
  });

  it("memformat nilai menurut bidang", () => {
    assert.equal(formatNilaiBidang("harga_acuan", 52000), formatRupiah(52000));
    assert.equal(
      formatNilaiBidang("harga_acuan", "52000"),
      formatRupiah(52000),
    );
    assert.equal(formatNilaiBidang("pilot", true), "Ya");
    assert.equal(formatNilaiBidang("aktif", false), "Tidak");
    assert.match(formatNilaiBidang("radius_meter", 450), /^450 m$/);
    assert.match(formatNilaiBidang("shelf_life_hari", 21), /^21 hari$/);
    assert.equal(formatNilaiBidang("lat", 5.5603), "5.5603");
    assert.equal(
      formatNilaiBidang("nama", "Pasar Peunayong"),
      "Pasar Peunayong",
    );
  });

  it("nilai kosong tampil sebagai tanda hubung", () => {
    assert.equal(formatNilaiBidang("kecamatan_kode", null), "—");
    assert.equal(formatNilaiBidang("kecamatan_kode", undefined), "—");
    assert.equal(formatNilaiBidang("nama", ""), "—");
  });

  it("ringkasan diff: sedikit, banyak, tinjau, tambah", () => {
    const diff = (...b: string[]) =>
      b.map((bidang) => ({ bidang, sebelum: 1, sesudah: 2 }));
    assert.equal(
      ringkasDiff({ jenis: "UBAH", diff: diff("harga_acuan") }),
      "Harga acuan",
    );
    assert.equal(
      ringkasDiff({ jenis: "UBAH", diff: diff("harga_acuan", "pilot") }),
      "Harga acuan, Komoditas pilot",
    );
    assert.equal(
      ringkasDiff({
        jenis: "UBAH",
        diff: diff("nama", "kategori", "satuan", "pilot"),
      }),
      "Nama, Kategori dan 2 lainnya",
    );
    assert.equal(ringkasDiff({ jenis: "TINJAU", diff: [] }), "Tinjauan status");
    assert.equal(
      ringkasDiff({ jenis: "TAMBAH", diff: diff("nama") }),
      "Data baru",
    );
    assert.equal(
      ringkasDiff({ jenis: "UBAH", diff: [] }),
      "Tanpa perubahan isi",
    );
  });
});

describe("status dan hak aksi", () => {
  const dibatalkan = {
    status: "DITOLAK" as const,
    alasanPenolakan: "Dibatalkan pengaju: salah ketik",
  };

  it("pembatalan pengaju dikenali dari awalan alasan penolakan", () => {
    assert.equal(adalahDibatalkan(dibatalkan), true);
    assert.equal(
      adalahDibatalkan({
        status: "DITOLAK",
        alasanPenolakan: "Tidak didukung data",
      }),
      false,
    );
    assert.equal(
      adalahDibatalkan({ status: "DITOLAK", alasanPenolakan: null }),
      false,
    );
    assert.equal(
      adalahDibatalkan({
        status: "MENUNGGU",
        alasanPenolakan: "Dibatalkan pengaju",
      }),
      false,
    );
  });

  it("badge status", () => {
    assert.deepEqual(
      statusTampil({ status: "MENUNGGU", alasanPenolakan: null }),
      { tone: "waspada", label: "Menunggu" },
    );
    assert.deepEqual(
      statusTampil({ status: "DISETUJUI", alasanPenolakan: null }),
      { tone: "normal", label: "Disetujui" },
    );
    assert.deepEqual(
      statusTampil({ status: "DITOLAK", alasanPenolakan: "x" }),
      { tone: "defisit", label: "Ditolak" },
    );
    assert.deepEqual(statusTampil(dibatalkan), {
      tone: "netral",
      label: "Dibatalkan",
    });
  });

  it("hanya pengaju yang boleh membatalkan usulan yang menunggu", () => {
    const usulan = {
      status: "MENUNGGU" as const,
      diajukanOleh: { id: "u-1", nama: "A" },
    };
    assert.equal(bolehBatalkan(usulan, "u-1"), true);
    assert.equal(bolehBatalkan(usulan, "u-2"), false);
    assert.equal(bolehBatalkan(usulan, undefined), false);
    assert.equal(
      bolehBatalkan({ ...usulan, status: "DISETUJUI" }, "u-1"),
      false,
    );
    assert.equal(
      bolehBatalkan({ status: "MENUNGGU", diajukanOleh: null }, "u-1"),
      false,
    );
  });

  it("pengaju sendiri terdeteksi untuk pemisahan tugas", () => {
    assert.equal(
      pengajuSendiri({ diajukanOleh: { id: "u-1", nama: "A" } }, "u-1"),
      true,
    );
    assert.equal(
      pengajuSendiri({ diajukanOleh: { id: "u-1", nama: "A" } }, "u-2"),
      false,
    );
    assert.equal(pengajuSendiri({ diajukanOleh: null }, "u-1"), false);
  });
});

describe("kueri", () => {
  it("queryString membuang nilai kosong tetapi mempertahankan false dan 0", () => {
    assert.equal(
      queryString({ a: "x", b: undefined, c: null, d: "", e: false, f: 0 }),
      "?a=x&e=false&f=0",
    );
    assert.equal(queryString({}), "");
  });

  it("kueri antrean", () => {
    assert.equal(toPerubahanQuery(), "");
    assert.equal(
      toPerubahanQuery({
        status: "MENUNGGU",
        entitas: "pasar",
        entitasId: "PSR-1171-01",
        page: 2,
        limit: 25,
      }),
      "?status=MENUNGGU&entitas=pasar&entitas_id=PSR-1171-01&page=2&limit=25",
    );
  });
});

describe("galat per kode domain", () => {
  const galat = (status: number, message: string, code?: string) =>
    new ApiError(
      status,
      message,
      code ? { code, message, data: null } : { message },
    );

  it("kodeGalatMaster membaca kode domain dan mengabaikan kode status HTTP", () => {
    assert.equal(
      kodeGalatMaster(galat(409, "x", "USULAN_MENUNGGU")),
      "USULAN_MENUNGGU",
    );
    assert.equal(kodeGalatMaster(galat(400, "x", "400")), undefined);
    assert.equal(kodeGalatMaster(galat(500, "x")), undefined);
    assert.equal(kodeGalatMaster(new Error("x")), undefined);
    assert.equal(kodeGalatMaster(null), undefined);
  });

  it("pesan dari server didahulukan", () => {
    assert.equal(
      pesanGalatMaster(
        galat(
          409,
          "Komoditas BWM masih dirujuk; arsipkan saja.",
          "MASTER_DIRUJUK",
        ),
      ),
      "Komoditas BWM masih dirujuk; arsipkan saja.",
    );
  });

  it("pesan generik 'Permintaan gagal (…)' diganti pesan menurut kode", () => {
    for (const kode of Object.keys(PESAN_KODE_GALAT)) {
      assert.equal(
        pesanGalatMaster(galat(409, "Permintaan gagal (409)", kode)),
        PESAN_KODE_GALAT[kode],
        kode,
      );
    }
  });

  it("tanpa kode: menurut status HTTP", () => {
    assert.match(
      pesanGalatMaster(galat(403, "Permintaan gagal (403)")),
      /izin/,
    );
    assert.match(
      pesanGalatMaster(galat(404, "Permintaan gagal (404)")),
      /tidak ditemukan/,
    );
    assert.match(pesanGalatMaster(galat(503, "boom")), /Server/);
  });

  it("bukan Error memakai pesan umum", () => {
    assert.match(pesanGalatMaster("aneh"), /Permintaan gagal/);
  });

  it("pesan bawaan menyebut arsip/nonaktifkan untuk MASTER_DIRUJUK", () => {
    assert.match(PESAN_KODE_GALAT.MASTER_DIRUJUK, /Arsipkan/);
    assert.match(PESAN_KODE_GALAT.MASTER_DIRUJUK, /nonaktifkan/);
  });
});

describe("parseBulat dan parseDesimal", () => {
  it("bilangan bulat: titik ribuan dan spasi diabaikan, desimal ditolak", () => {
    assert.equal(parseBulat("52.500"), 52500);
    assert.equal(parseBulat(" 52 500 "), 52500);
    assert.equal(parseBulat("52,5"), null);
    assert.equal(parseBulat("abc"), null);
    assert.equal(parseBulat(""), null);
  });

  it("desimal: koma atau titik", () => {
    assert.equal(parseDesimal("5,56"), 5.56);
    assert.equal(parseDesimal("-95.3237"), -95.3237);
    assert.equal(parseDesimal("5.5.5"), null);
    assert.equal(parseDesimal(""), null);
  });
});

const komoditas: MasterKomoditasRow = {
  id: "BWM",
  kode: "BWM",
  nama: "Bawang merah",
  kategori: "Hortikultura",
  satuan: "kg",
  hargaAcuan: 41000,
  pilot: true,
  shelfLifeHari: 60,
  versi: 2,
  status: "AKTIF",
};

const pasar: MasterPasarRow = {
  id: "PSR-1171-01",
  nama: "Pasar Peunayong",
  kabupaten: "Kota Banda Aceh",
  kabupatenKode: "1171",
  lat: 5.5603,
  lng: 95.3237,
  radiusMeter: 400,
  aktif: true,
  kecamatanKode: "1171060",
  kecamatanNama: "Kuta Alam",
  versi: 1,
};

describe("validasiAlasan", () => {
  it("3 sampai 1000 karakter setelah dipangkas", () => {
    assert.ok(validasiAlasan("  ab  "));
    assert.equal(validasiAlasan("abc"), undefined);
    assert.equal(validasiAlasan("x".repeat(1000)), undefined);
    assert.ok(validasiAlasan("x".repeat(1001)));
  });
});

describe("validasiKomoditas", () => {
  const satuan = ["kg", "liter"];

  it("ubah tanpa perubahan: galat tingkat form", () => {
    const hasil = validasiKomoditas(
      { ...isianKomoditasDari(komoditas), alasan: "Tanpa beda" },
      { awal: komoditas, satuan },
    );
    assert.equal(typeof hasil, "string");
    assert.match(hasil as string, /Tidak ada perubahan/);
  });

  it("ubah dengan satu perubahan dan alasan: lolos", () => {
    const i = {
      ...isianKomoditasDari(komoditas),
      harga_acuan: "43.500",
      alasan: "Naik mengikuti BPS",
    };
    assert.equal(validasiKomoditas(i, { awal: komoditas, satuan }), null);
  });

  it("alasan wajib", () => {
    const i = {
      ...isianKomoditasDari(komoditas),
      harga_acuan: "43500",
      alasan: "",
    };
    const hasil = validasiKomoditas(i, { awal: komoditas, satuan }) as Record<
      string,
      string
    >;
    assert.ok(hasil.alasan);
  });

  it("aturan bidang: harga > 0 bulat, umur 1..3650, nama dan kategori wajib, satuan harus dari daftar", () => {
    const i = {
      ...isianKomoditasDari(komoditas),
      nama: " ",
      kategori: "",
      satuan: "ton",
      harga_acuan: "0",
      shelf_life_hari: "3651",
      alasan: "ok!",
    };
    const hasil = validasiKomoditas(i, { awal: komoditas, satuan }) as Record<
      string,
      string
    >;

    assert.ok(hasil.nama);
    assert.ok(hasil.kategori);
    assert.match(hasil.satuan, /daftar satuan/);
    assert.match(hasil.harga_acuan, /lebih dari 0/);
    assert.match(hasil.shelf_life_hari, /1–3650/);
    assert.equal(hasil.alasan, undefined);
  });

  it("harga desimal ditolak", () => {
    const i = {
      ...isianKomoditasDari(komoditas),
      harga_acuan: "100,5",
      alasan: "alasan",
    };
    assert.ok(
      (
        validasiKomoditas(i, { awal: komoditas, satuan }) as Record<
          string,
          string
        >
      ).harga_acuan,
    );
  });

  it("daftar satuan belum termuat: hanya wajib terisi", () => {
    const i = {
      ...isianKomoditasDari(komoditas),
      satuan: "kg",
      harga_acuan: "43500",
      alasan: "alasan",
    };
    assert.equal(validasiKomoditas(i, { awal: komoditas }), null);
  });

  it("tambah: kode huruf kapital/angka/hubung 2..30 dan semua bidang wajib", () => {
    const kosong = validasiKomoditas(isianKomoditasKosong(), {}) as Record<
      string,
      string
    >;
    for (const k of [
      "kode",
      "nama",
      "kategori",
      "satuan",
      "harga_acuan",
      "shelf_life_hari",
      "alasan",
    ])
      assert.ok(kosong[k], k);

    const lengkap = {
      kode: "KDL-HJU",
      nama: "Kedelai hijau",
      kategori: "Pangan pokok",
      satuan: "kg",
      harga_acuan: "14000",
      shelf_life_hari: "180",
      pilot: false,
      alasan: "Komoditas baru",
    };
    assert.equal(validasiKomoditas(lengkap, { satuan }), null);
    for (const kode of [
      "a",
      "kdl hju",
      "kdl_hju",
      "KDL-HJU-".padEnd(31, "X"),
    ]) {
      assert.ok(
        (
          validasiKomoditas({ ...lengkap, kode }, { satuan }) as Record<
            string,
            string
          >
        ).kode,
        kode,
      );
    }
  });
});

describe("perubahanKomoditas", () => {
  it("ubah: hanya bidang yang berbeda, bertipe benar", () => {
    const i = {
      ...isianKomoditasDari(komoditas),
      nama: "  Bawang merah super ",
      harga_acuan: "43.500",
      pilot: false,
    };
    assert.deepEqual(perubahanKomoditas(i, komoditas), {
      nama: "Bawang merah super",
      harga_acuan: 43500,
      pilot: false,
    });
  });

  it("ubah tanpa beda: kosong", () => {
    assert.deepEqual(
      perubahanKomoditas(isianKomoditasDari(komoditas), komoditas),
      {},
    );
  });

  it("tambah: keenam bidang, tanpa kode (kode jadi entitas_id)", () => {
    const i = {
      kode: "KDL-HJU",
      nama: "Kedelai",
      kategori: "Pangan pokok",
      satuan: "kg",
      harga_acuan: "14000",
      shelf_life_hari: "180",
      pilot: true,
      alasan: "x",
    };
    assert.deepEqual(perubahanKomoditas(i), {
      nama: "Kedelai",
      kategori: "Pangan pokok",
      satuan: "kg",
      harga_acuan: 14000,
      shelf_life_hari: 180,
      pilot: true,
    });
  });
});

describe("validasiPasar dan perubahanPasar", () => {
  it("ubah tanpa perubahan: galat tingkat form", () => {
    const hasil = validasiPasar(
      { ...isianPasarDari(pasar), alasan: "Tanpa beda" },
      { awal: pasar },
    );
    assert.match(hasil as string, /Tidak ada perubahan/);
  });

  it("koordinat, radius, dan kode kecamatan divalidasi", () => {
    const i = {
      ...isianPasarDari(pasar),
      lat: "91",
      lng: "-181",
      radius_meter: "10001",
      kecamatan_kode: "11710",
      alasan: "alasan",
    };
    const hasil = validasiPasar(i, { awal: pasar }) as Record<string, string>;

    assert.match(hasil.lat, /-90 dan 90/);
    assert.match(hasil.lng, /-180 dan 180/);
    assert.match(hasil.radius_meter, /1–10000/);
    assert.match(hasil.kecamatan_kode, /7 digit/);
  });

  it("batas koordinat dan radius yang masih sah lolos", () => {
    const i = {
      ...isianPasarDari(pasar),
      lat: "-90",
      lng: "180",
      radius_meter: "10000",
      alasan: "alasan",
    };
    assert.equal(validasiPasar(i, { awal: pasar }), null);
  });

  it("ubah: kecamatan dikosongkan dikirim sebagai null, hanya bidang yang berbeda", () => {
    const i = {
      ...isianPasarDari(pasar),
      kecamatan_kode: "",
      radius_meter: "450",
      aktif: false,
    };
    assert.deepEqual(perubahanPasar(i, pasar), {
      radius_meter: 450,
      aktif: false,
      kecamatan_kode: null,
    });
  });

  it("ubah: pasar tanpa kecamatan yang tetap kosong tidak dianggap berubah", () => {
    const tanpa = { ...pasar, kecamatanKode: null, kecamatanNama: null };
    assert.deepEqual(perubahanPasar(isianPasarDari(tanpa), tanpa), {});
  });

  it("ubah: lat/lng desimal dengan koma dibaca benar", () => {
    const i = { ...isianPasarDari(pasar), lat: "5,57" };
    assert.deepEqual(perubahanPasar(i, pasar), { lat: 5.57 });
  });

  it("tambah: ID, kabupaten, dan koordinat wajib; kecamatan opsional", () => {
    const kosong = validasiPasar(isianPasarKosong(), {}) as Record<
      string,
      string
    >;
    for (const k of ["id", "kabupaten_kode", "nama", "lat", "lng", "alasan"])
      assert.ok(kosong[k], k);
    assert.equal(
      kosong.radius_meter,
      undefined,
      "radius bawaan 400 sudah terisi",
    );

    const lengkap = {
      id: "PSR-1171-05",
      nama: "Pasar Baru",
      kabupaten_kode: "1171",
      lat: "5.55",
      lng: "95.32",
      radius_meter: "400",
      aktif: true,
      kecamatan_kode: "",
      alasan: "Pasar baru",
    };
    assert.equal(validasiPasar(lengkap, {}), null);
    assert.deepEqual(perubahanPasar(lengkap), {
      nama: "Pasar Baru",
      kabupaten_kode: "1171",
      lat: 5.55,
      lng: 95.32,
      radius_meter: 400,
    });
    assert.deepEqual(
      perubahanPasar({ ...lengkap, kecamatan_kode: "1171060" }),
      {
        nama: "Pasar Baru",
        kabupaten_kode: "1171",
        lat: 5.55,
        lng: 95.32,
        radius_meter: 400,
        kecamatan_kode: "1171060",
      },
    );
  });
});

describe("bodyUsulan", () => {
  it("memangkas ID dan alasan dan memakai nama bidang API", () => {
    assert.deepEqual(
      bodyUsulan({
        entitas: "komoditas",
        jenis: "UBAH",
        entitasId: " BWM ",
        perubahan: { harga_acuan: 43500 },
        alasan: "  Naik  ",
      }),
      {
        entitas: "komoditas",
        jenis: "UBAH",
        entitas_id: "BWM",
        perubahan: { harga_acuan: 43500 },
        alasan: "Naik",
      },
    );
  });
});

describe("riwayat versi", () => {
  const v = (
    versi: number,
    harga: number,
    over: Record<string, unknown> = {},
  ) =>
    toRiwayatVersi({
      id: versi,
      versi,
      nama: "Bawang merah",
      kategori: "Hortikultura",
      satuan: "kg",
      harga_acuan: harga,
      shelf_life_hari: 60,
      pilot: true,
      berlaku_dari: `2026-0${versi}-01`,
      berlaku_sampai: null,
      ...over,
    });

  it("memetakan satu versi", () => {
    const r = v(2, 41000, { perubahan_id: 31, berlaku_sampai: "2026-03-01" });
    assert.equal(r.id, "2");
    assert.equal(r.hargaAcuan, 41000);
    assert.equal(r.perubahanId, "31");
    assert.equal(r.berlakuSampai, "2026-03-01");
    assert.equal(v(1, 1).perubahanId, null);
  });

  it("beda tiap versi terhadap versi sebelumnya, urutan masukan dipertahankan", () => {
    const terbaruDulu = [
      v(3, 43500, { nama: "Bawang merah super", pilot: false }),
      v(2, 41000),
      v(1, 39000),
    ];
    const hasil = perubahanAntarVersi(terbaruDulu);

    assert.deepEqual(
      hasil.map((h) => h.versi.versi),
      [3, 2, 1],
    );
    assert.deepEqual(
      hasil[0].perubahan.map((p) => p.bidang),
      ["nama", "harga_acuan", "pilot"],
    );
    assert.deepEqual(
      hasil[0].perubahan.find((p) => p.bidang === "harga_acuan"),
      { bidang: "harga_acuan", sebelum: 41000, sesudah: 43500 },
    );
    assert.deepEqual(hasil[1].perubahan, [
      { bidang: "harga_acuan", sebelum: 39000, sesudah: 41000 },
    ]);
    assert.deepEqual(
      hasil[2].perubahan,
      [],
      "versi pertama tidak punya pembanding",
    );
  });

  it("versi tanpa beda isi (hanya tinjauan status) tidak punya perubahan", () => {
    const hasil = perubahanAntarVersi([v(2, 41000), v(1, 41000)]);
    assert.deepEqual(hasil[0].perubahan, []);
  });

  it("riwayat kosong", () => {
    assert.deepEqual(perubahanAntarVersi([]), []);
  });
});
