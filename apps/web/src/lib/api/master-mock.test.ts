import assert from "node:assert/strict";
import { beforeEach, describe, it } from "node:test";
import { ApiError } from "./client";
import {
  mockAjukan,
  mockBatalkan,
  mockBulkKomoditas,
  mockHapus,
  mockKecamatanDetail,
  mockKecamatanList,
  mockKomoditasDetail,
  mockKomoditasList,
  mockPasarList,
  mockPedagangList,
  mockPerubahanDetail,
  mockPerubahanList,
  mockSatuanList,
  mockSetujui,
  mockTolak,
  resetMasterMock,
} from "./master-mock";

const pengaju = { id: "u-pengaju", nama: "Pengaju" };
const penyetuju = { id: "u-penyetuju", nama: "Penyetuju" };

/** Galat yang dilempar fungsi (ApiError) untuk dicek status dan kode domainnya. */
function galatDari(fn: () => unknown): {
  status: number;
  code: string | undefined;
  message: string;
} {
  try {
    fn();
  } catch (err) {
    assert.ok(err instanceof ApiError, "ApiError diharapkan");
    return {
      status: err.status,
      code: (err.payload as { code?: string }).code,
      message: err.message,
    };
  }
  assert.fail("galat diharapkan");
}

const ajukanHarga = (kode: string, harga: number) =>
  mockAjukan(
    {
      entitas: "komoditas",
      jenis: "UBAH",
      entitas_id: kode,
      perubahan: { harga_acuan: harga },
      alasan: "Penyesuaian harga",
    },
    pengaju,
  );

beforeEach(() => resetMasterMock());

describe("data awal", () => {
  it("antrean menunggu terlama dahulu, riwayat terbaru dahulu, dan total tercatat", () => {
    const menunggu = mockPerubahanList({ status: "MENUNGGU" });
    assert.ok(menunggu.total >= 3);
    const waktu = menunggu.rows.map((r) => r.dibuat_pada);
    assert.deepEqual([...waktu].sort(), waktu);

    const riwayat = mockPerubahanList({ status: "DISETUJUI" });
    assert.ok(riwayat.rows.length >= 1);
    assert.ok(riwayat.rows.every((r) => r.status === "DISETUJUI"));
  });

  it("komoditas yang menunggu tinjauan punya usulan TINJAU dengan diff kosong", () => {
    const menunggu = mockKomoditasList().find(
      (k) => k.status === "MENUNGGU_PERSETUJUAN",
    )!;
    const detail = mockKomoditasDetail(menunggu.kode);

    assert.equal(detail.perubahan_menunggu?.jenis, "TINJAU");
    assert.deepEqual(detail.perubahan_menunggu?.diff, []);
  });

  it("riwayat versi terbaru dahulu, satu versi berlaku, jumlahnya sama dengan versi", () => {
    const k = mockKomoditasList().find((x) => x.versi === 3)!;
    const { riwayat_versi } = mockKomoditasDetail(k.kode);

    assert.deepEqual(
      riwayat_versi?.map((v) => v.versi),
      [3, 2, 1],
    );
    assert.equal(
      riwayat_versi?.filter((v) => v.berlaku_sampai === null).length,
      1,
    );
    assert.equal(riwayat_versi?.[0].berlaku_sampai, null);
  });

  it("filter antrean menurut entitas dan entitas_id", () => {
    assert.ok(
      mockPerubahanList({ entitas: "pasar" }).rows.every(
        (r) => r.entitas === "pasar",
      ),
    );
    assert.deepEqual(
      mockPerubahanList({ entitas: "komoditas", entitasId: "BWM" }).rows.map(
        (r) => r.entitas_id,
      ),
      ["BWM"],
    );
  });
});

describe("ajukan", () => {
  it("usulan ubah masuk antrean dengan diff hanya bidang yang berbeda dan versi tujuan", () => {
    const sebelum = mockKomoditasList().find((k) => k.kode === "GLA")!;
    const p = mockAjukan(
      {
        entitas: "komoditas",
        jenis: "UBAH",
        entitas_id: "GLA",
        perubahan: { harga_acuan: 19000, nama: sebelum.nama },
        alasan: "  Harga naik  ",
      },
      pengaju,
    );

    assert.equal(p.status, "MENUNGGU");
    assert.equal(p.versi, sebelum.versi + 1);
    assert.equal(p.alasan, "Harga naik");
    assert.deepEqual(p.diff, [
      { bidang: "harga_acuan", sebelum: sebelum.harga_acuan, sesudah: 19000 },
    ]);
    assert.deepEqual(p.diajukan_oleh, pengaju);
    assert.ok(
      mockPerubahanList({ status: "MENUNGGU" }).rows.some((r) => r.id === p.id),
    );
    assert.equal(mockPerubahanDetail(p.id).id, p.id);
    assert.equal(mockKomoditasDetail("GLA").perubahan_menunggu?.id, p.id);
  });

  it("tidak mengubah baris sebelum disetujui", () => {
    const sebelum = mockKomoditasList().find((k) => k.kode === "GLA")!;
    ajukanHarga("GLA", 19000);
    assert.equal(
      mockKomoditasList().find((k) => k.kode === "GLA")!.harga_acuan,
      sebelum.harga_acuan,
    );
  });

  it("usulan kedua untuk data yang sama: 409 USULAN_MENUNGGU", () => {
    ajukanHarga("GLA", 19000);
    const g = galatDari(() => ajukanHarga("GLA", 19500));
    assert.deepEqual([g.status, g.code], [409, "USULAN_MENUNGGU"]);
  });

  it("komoditas yang menunggu tinjauan juga menolak usulan baru", () => {
    const menunggu = mockKomoditasList().find(
      (k) => k.status === "MENUNGGU_PERSETUJUAN",
    )!;
    assert.equal(
      galatDari(() => ajukanHarga(menunggu.kode, 1000)).code,
      "USULAN_MENUNGGU",
    );
  });

  it("tanpa beda dari data sekarang: 400 TANPA_PERUBAHAN", () => {
    const g = mockKomoditasList().find((k) => k.kode === "GLA")!;
    const e = galatDari(() => ajukanHarga("GLA", g.harga_acuan));
    assert.deepEqual([e.status, e.code], [400, "TANPA_PERUBAHAN"]);
  });

  it("masukan tidak sah: 400; data tidak ada: 404", () => {
    const ubah = (perubahan: Record<string, unknown>, alasan = "alasan") =>
      mockAjukan(
        {
          entitas: "komoditas",
          jenis: "UBAH",
          entitas_id: "GLA",
          perubahan,
          alasan,
        },
        pengaju,
      );
    assert.equal(galatDari(() => ubah({ harga_acuan: 0 })).status, 400);
    assert.equal(galatDari(() => ubah({ harga_acuan: 10.5 })).status, 400);
    assert.equal(galatDari(() => ubah({ shelf_life_hari: 3651 })).status, 400);
    assert.equal(galatDari(() => ubah({ satuan: "ton" })).status, 400);
    assert.equal(galatDari(() => ubah({ kode: "X" })).status, 400);
    assert.equal(
      galatDari(() => ubah({ harga_acuan: 1000 }, "ab")).status,
      400,
    );
    assert.equal(galatDari(() => ajukanHarga("TIDAK-ADA", 1000)).status, 404);
  });

  it("komoditas diarsipkan tidak dapat diubah: 409 STATUS_TIDAK_SESUAI", () => {
    mockBulkKomoditas("arsipkan", ["GLA"], pengaju);
    assert.equal(
      galatDari(() => ajukanHarga("GLA", 19000)).code,
      "STATUS_TIDAK_SESUAI",
    );
  });

  it("tambah komoditas membutuhkan keenam bidang dan kode baru", () => {
    const dasar = {
      nama: "Kedelai",
      kategori: "Pangan pokok",
      satuan: "kg",
      harga_acuan: 14000,
      shelf_life_hari: 180,
      pilot: false,
    };
    const tambah = (id: string, perubahan: Record<string, unknown>) =>
      mockAjukan(
        {
          entitas: "komoditas",
          jenis: "TAMBAH",
          entitas_id: id,
          perubahan,
          alasan: "Komoditas baru",
        },
        pengaju,
      );

    const { pilot: _p, ...kurang } = dasar;
    void _p;
    assert.equal(galatDari(() => tambah("KDL-HJU", kurang)).status, 400);
    assert.equal(galatDari(() => tambah("kdl hju", dasar)).status, 400);
    assert.equal(galatDari(() => tambah("GLA", dasar)).code, "KODE_SUDAH_ADA");

    const p = tambah("KDL-HJU", dasar);
    assert.equal(p.versi, 1);
    assert.equal(p.entitas_nama, "Kedelai");
    assert.equal(p.diff?.length, 6);
    assert.ok(p.diff?.every((d) => d.sebelum === null));
    assert.equal(
      galatDari(() => tambah("KDL-HJU", dasar)).code,
      "USULAN_MENUNGGU",
    );
  });

  it("pasar: kecamatan harus milik kabupaten pasar; null mengosongkan", () => {
    const ubah = (perubahan: Record<string, unknown>) =>
      mockAjukan(
        {
          entitas: "pasar",
          jenis: "UBAH",
          entitas_id: "PSR-1171-01",
          perubahan,
          alasan: "alasan",
        },
        pengaju,
      );

    assert.equal(
      galatDari(() => ubah({ kecamatan_kode: "1108010" })).status,
      400,
    );
    assert.equal(galatDari(() => ubah({ lat: 91 })).status, 400);
    assert.equal(galatDari(() => ubah({ radius_meter: 10001 })).status, 400);

    const p = ubah({ kecamatan_kode: null });
    assert.deepEqual(p.diff, [
      { bidang: "kecamatan_kode", sebelum: "1171060", sesudah: null },
    ]);
  });
});

describe("setujui", () => {
  it("pengaju tidak boleh menyetujui usulannya sendiri: 403 PEMISAHAN_TUGAS", () => {
    const p = ajukanHarga("GLA", 19000);
    const g = galatDari(() => mockSetujui(p.id, pengaju));
    assert.deepEqual([g.status, g.code], [403, "PEMISAHAN_TUGAS"]);
    assert.equal(mockPerubahanDetail(p.id).status, "MENUNGGU");
  });

  it("menerapkan usulan: nilai baru, versi naik, versi lama ditutup, versi baru berlaku", () => {
    const sebelum = mockKomoditasList().find((k) => k.kode === "GLA")!;
    const p = ajukanHarga("GLA", 19000);

    const hasil = mockSetujui(p.id, penyetuju);

    assert.equal(hasil.status, "DISETUJUI");
    assert.deepEqual(hasil.disetujui_oleh, penyetuju);
    assert.ok(hasil.disetujui_pada);
    const sesudah = mockKomoditasList().find((k) => k.kode === "GLA")!;
    assert.equal(sesudah.harga_acuan, 19000);
    assert.equal(sesudah.versi, sebelum.versi + 1);

    const { riwayat_versi, perubahan_menunggu } = mockKomoditasDetail("GLA");
    assert.equal(perubahan_menunggu, null);
    assert.equal(riwayat_versi?.length, sebelum.versi + 1);
    assert.equal(riwayat_versi?.[0].harga_acuan, 19000);
    assert.equal(riwayat_versi?.[0].berlaku_sampai, null);
    assert.equal(riwayat_versi?.[0].perubahan_id, p.id);
    assert.notEqual(
      riwayat_versi?.[1].berlaku_sampai,
      null,
      "versi sebelumnya ditutup",
    );
  });

  it("usulan yang sudah diputuskan: 409 BUKAN_MENUNGGU; ID tak dikenal: 404", () => {
    const p = ajukanHarga("GLA", 19000);
    mockSetujui(p.id, penyetuju);
    assert.equal(
      galatDari(() => mockSetujui(p.id, penyetuju)).code,
      "BUKAN_MENUNGGU",
    );
    assert.equal(galatDari(() => mockSetujui("99999", penyetuju)).status, 404);
  });

  it("usulan tambah komoditas membuat baris v1 aktif dengan riwayat", () => {
    const p = mockAjukan(
      {
        entitas: "komoditas",
        jenis: "TAMBAH",
        entitas_id: "KDL-HJU",
        perubahan: {
          nama: "Kedelai",
          kategori: "Pangan pokok",
          satuan: "kg",
          harga_acuan: 14000,
          shelf_life_hari: 180,
          pilot: false,
        },
        alasan: "Komoditas baru",
      },
      pengaju,
    );
    mockSetujui(p.id, penyetuju);

    const baru = mockKomoditasList().find((k) => k.kode === "KDL-HJU")!;
    assert.equal(baru.versi, 1);
    assert.equal(baru.status, "AKTIF");
    assert.equal(mockKomoditasDetail("KDL-HJU").riwayat_versi?.length, 1);
    assert.equal(
      mockSatuanList().find((s) => s.kode === "kg")!.jumlah_komoditas,
      mockKomoditasList().filter((k) => k.satuan === "kg").length,
    );
  });

  it("usulan tambah pasar: aktif, radius bawaan 400, nama kecamatan terisi", () => {
    const p = mockAjukan(
      {
        entitas: "pasar",
        jenis: "TAMBAH",
        entitas_id: "PSR-1171-05",
        perubahan: {
          nama: "Pasar Baru",
          kabupaten_kode: "1171",
          lat: 5.55,
          lng: 95.32,
          kecamatan_kode: "1171040",
        },
        alasan: "Pasar baru",
      },
      pengaju,
    );
    assert.equal(p.perubahan?.radius_meter, 400);
    mockSetujui(p.id, penyetuju);

    const baru = mockPasarList().find((x) => x.id === "PSR-1171-05")!;
    assert.equal(baru.aktif, true);
    assert.equal(baru.radius_meter, 400);
    assert.equal(baru.kecamatan_nama, "Baiturrahman");
    assert.equal(baru.versi, 1);
  });

  it("usulan ubah pasar menerapkan kecamatan dan menaikkan versi", () => {
    const p = mockAjukan(
      {
        entitas: "pasar",
        jenis: "UBAH",
        entitas_id: "PSR-1108-01",
        perubahan: { kecamatan_kode: "1108010" },
        alasan: "alasan",
      },
      pengaju,
    );
    mockSetujui(p.id, penyetuju);

    const pasar = mockPasarList().find((x) => x.id === "PSR-1108-01")!;
    assert.equal(pasar.kecamatan_nama, "Kecamatan Aceh Besar 1");
    assert.equal(pasar.versi, 2);
  });

  it("usulan tinjauan yang disetujui menaikkan versi dan mengaktifkan kembali komoditas", () => {
    const k = mockKomoditasList().find(
      (x) => x.status === "MENUNGGU_PERSETUJUAN",
    )!;
    const p = mockKomoditasDetail(k.kode).perubahan_menunggu!;
    mockSetujui(p.id, penyetuju);

    const sesudah = mockKomoditasList().find((x) => x.kode === k.kode)!;
    assert.equal(sesudah.status, "AKTIF");
    assert.equal(sesudah.versi, k.versi + 1);
  });
});

describe("tolak dan batalkan", () => {
  it("tolak butuh alasan, tidak boleh oleh pengaju, dan menyimpan alasan penolakan", () => {
    const awal = mockKomoditasList().find((k) => k.kode === "GLA")!;
    const p = ajukanHarga("GLA", 19000);
    assert.equal(galatDari(() => mockTolak(p.id, "ab", penyetuju)).status, 400);
    assert.equal(
      galatDari(() => mockTolak(p.id, "tidak sesuai", pengaju)).code,
      "PEMISAHAN_TUGAS",
    );

    const hasil = mockTolak(p.id, "  Belum didukung data  ", penyetuju);
    assert.equal(hasil.status, "DITOLAK");
    assert.equal(hasil.alasan_penolakan, "Belum didukung data");
    assert.deepEqual(hasil.ditolak_oleh, penyetuju);
    const sesudah = mockKomoditasList().find((k) => k.kode === "GLA")!;
    assert.equal(sesudah.versi, awal.versi);
    assert.equal(sesudah.harga_acuan, awal.harga_acuan);
    assert.equal(
      galatDari(() => mockTolak(p.id, "sekali lagi", penyetuju)).code,
      "BUKAN_MENUNGGU",
    );
  });

  it("menolak usulan tinjauan mengembalikan komoditas ke AKTIF tanpa menaikkan versi", () => {
    const k = mockKomoditasList().find(
      (x) => x.status === "MENUNGGU_PERSETUJUAN",
    )!;
    const p = mockKomoditasDetail(k.kode).perubahan_menunggu!;
    mockTolak(p.id, "belum perlu", penyetuju);

    const sesudah = mockKomoditasList().find((x) => x.kode === k.kode)!;
    assert.equal(sesudah.status, "AKTIF");
    assert.equal(sesudah.versi, k.versi);
  });

  it("hanya pengaju yang dapat membatalkan; alasan penolakan berawalan 'Dibatalkan pengaju'", () => {
    const p = ajukanHarga("GLA", 19000);
    assert.equal(
      galatDari(() => mockBatalkan(p.id, undefined, penyetuju)).status,
      403,
    );

    const tanpaAlasan = mockBatalkan(p.id, undefined, pengaju);
    assert.equal(tanpaAlasan.status, "DITOLAK");
    assert.equal(tanpaAlasan.alasan_penolakan, "Dibatalkan pengaju");

    const p2 = ajukanHarga("GLA", 19500);
    assert.equal(
      mockBatalkan(p2.id, "salah angka", pengaju).alasan_penolakan,
      "Dibatalkan pengaju: salah angka",
    );
    assert.equal(
      galatDari(() => mockBatalkan(p2.id, undefined, pengaju)).code,
      "BUKAN_MENUNGGU",
    );
  });

  it("setelah dibatalkan data dapat diusulkan lagi", () => {
    const p = ajukanHarga("GLA", 19000);
    mockBatalkan(p.id, undefined, pengaju);
    assert.equal(ajukanHarga("GLA", 19500).status, "MENUNGGU");
  });
});

describe("hapus", () => {
  it("data yang masih dirujuk: 409 MASTER_DIRUJUK dengan saran arsip/nonaktifkan", () => {
    const k = galatDari(() => mockHapus("komoditas", "GLA"));
    assert.deepEqual([k.status, k.code], [409, "MASTER_DIRUJUK"]);
    assert.match(k.message, /Arsipkan/);

    const p = galatDari(() => mockHapus("pasar", "PSR-1171-01"));
    assert.equal(p.code, "MASTER_DIRUJUK");
    assert.match(p.message, /Nonaktifkan/);
  });

  it("ada usulan menunggu: 409 MASTER_ADA_USULAN; tak dikenal: 404", () => {
    ajukanHarga("GLA", 19000);
    assert.equal(
      galatDari(() => mockHapus("komoditas", "GLA")).code,
      "MASTER_ADA_USULAN",
    );
    assert.equal(
      galatDari(() => mockHapus("komoditas", "TIDAK-ADA")).status,
      404,
    );
    assert.equal(galatDari(() => mockHapus("pasar", "TIDAK-ADA")).status, 404);
  });

  it("data baru yang belum dirujuk dapat dihapus", () => {
    const p = mockAjukan(
      {
        entitas: "komoditas",
        jenis: "TAMBAH",
        entitas_id: "KDL-HJU",
        perubahan: {
          nama: "Kedelai",
          kategori: "Pangan pokok",
          satuan: "kg",
          harga_acuan: 14000,
          shelf_life_hari: 180,
          pilot: false,
        },
        alasan: "Komoditas baru",
      },
      pengaju,
    );
    mockSetujui(p.id, penyetuju);
    mockHapus("komoditas", "KDL-HJU");
    assert.equal(
      mockKomoditasList().some((k) => k.kode === "KDL-HJU"),
      false,
    );
  });
});

describe("aksi massal komoditas", () => {
  it("ajukan_perubahan membuat usulan tinjauan; setujui oleh pengaju gagal per item, oleh orang lain berhasil", () => {
    const sebelum = mockKomoditasList().find((k) => k.kode === "GLA")!;
    const ajukan = mockBulkKomoditas(
      "ajukan_perubahan",
      ["GLA", "TIDAK-ADA"],
      pengaju,
    );
    assert.equal(ajukan.berhasil, 1);
    assert.equal(ajukan.gagal, 1);
    assert.equal(
      ajukan.rincian.find((r) => r.kode === "TIDAK-ADA")?.pesan,
      "Komoditas tidak ditemukan",
    );
    assert.equal(
      mockKomoditasList().find((k) => k.kode === "GLA")!.status,
      "MENUNGGU_PERSETUJUAN",
    );

    const sendiri = mockBulkKomoditas("setujui", ["GLA"], pengaju);
    assert.equal(sendiri.gagal, 1);
    assert.match(sendiri.rincian[0].pesan ?? "", /pemisahan tugas/);

    const lain = mockBulkKomoditas("setujui", ["GLA"], penyetuju);
    assert.equal(lain.berhasil, 1);
    const sesudah = mockKomoditasList().find((k) => k.kode === "GLA")!;
    assert.equal(sesudah.status, "AKTIF");
    assert.equal(sesudah.versi, sebelum.versi + 1);
  });

  it("setujui tanpa usulan dan arsipkan dua kali gagal per item", () => {
    assert.equal(mockBulkKomoditas("setujui", ["GLA"], penyetuju).gagal, 1);
    assert.equal(mockBulkKomoditas("arsipkan", ["GLA"], pengaju).berhasil, 1);
    assert.equal(mockBulkKomoditas("arsipkan", ["GLA"], pengaju).gagal, 1);
  });
});

describe("referensi", () => {
  it("kecamatan: urut menurut kode, filter kabupaten, pencarian, paginasi, dan total", () => {
    const semua = mockKecamatanList({ limit: 1000 });
    assert.ok(semua.total > 100);
    assert.deepEqual(
      semua.rows.map((k) => k.kode),
      [...semua.rows.map((k) => k.kode)].sort(),
    );

    const banda = mockKecamatanList({ kabupatenKode: "1171", limit: 1000 });
    assert.equal(banda.total, 9);
    assert.ok(banda.rows.every((k) => k.kabupaten_kode === "1171"));

    const cari = mockKecamatanList({ q: "kuta" });
    assert.deepEqual(cari.rows.map((k) => k.nama).sort(), [
      "Kuta Alam",
      "Kuta Raja",
    ]);

    const hal2 = mockKecamatanList({
      kabupatenKode: "1171",
      page: 2,
      limit: 4,
    });
    assert.equal(hal2.rows.length, 4);
    assert.equal(hal2.total, 9);
    assert.equal(
      mockKecamatanList({ kabupatenKode: "1171", page: 3, limit: 4 }).rows
        .length,
      1,
    );
  });

  it("detail kecamatan: ada dan 404", () => {
    assert.equal(mockKecamatanDetail("1171060").nama, "Kuta Alam");
    assert.equal(galatDari(() => mockKecamatanDetail("0000000")).status, 404);
  });

  it("pedagang: filter pasar, status, pencarian, dan paginasi", () => {
    const semua = mockPedagangList({ limit: 1000 });
    const satu = mockPedagangList({ pasarId: "PSR-1171-01" });
    assert.equal(satu.total, 3);
    assert.ok(satu.rows.every((p) => p.pasar_id === "PSR-1171-01"));
    assert.ok(
      mockPedagangList({ aktif: false, limit: 1000 }).rows.every(
        (p) => !p.aktif,
      ),
    );
    assert.ok(mockPedagangList({ q: "PDG-1171-01" }).total >= 3);
    assert.equal(mockPedagangList({ limit: 5 }).rows.length, 5);
    assert.equal(mockPedagangList({ limit: 5 }).total, semua.total);
  });
});
