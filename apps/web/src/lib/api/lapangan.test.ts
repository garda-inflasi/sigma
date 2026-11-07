import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { cariAcuan, hrefIsiTugas, toTugasDetail, toTugasLapangan, toVersiServer, type DataAcuanResponse } from "./lapangan";

describe("toTugasLapangan", () => {
  it("memetakan kolom API ke tugas UI", () => {
    const tugas = toTugasLapangan({
      id: "42",
      kode: "TGS-mahasiswa_1-1",
      judul: "Survei harga konsumen pagi",
      pasar_id: "PSR-1171-01",
      pasar_nama: "Pasar Peunayong",
      komoditas: ["CBM-KRT", "BWM"],
      tenggat: "2026-10-16T09:30:00Z",
      status: "BERJALAN",
      target: 6,
      terisi: 4,
      tipe: "KONSUMEN",
    });

    assert.deepEqual(tugas, {
      id: "42",
      kode: "TGS-mahasiswa_1-1",
      judul: "Survei harga konsumen pagi",
      pasarId: "PSR-1171-01",
      pasarNama: "Pasar Peunayong",
      komoditas: ["CBM-KRT", "BWM"],
      tenggat: "2026-10-16T09:30:00Z",
      status: "BERJALAN",
      target: 6,
      terisi: 4,
      tipe: "KONSUMEN",
    });
  });
});

describe("toTugasLapangan: tugas baru tanpa kode", () => {
  const dasar = {
    id: "77", judul: "Tugas baru", pasar_id: "PSR-1171-01", pasar_nama: "Pasar Peunayong", komoditas: ["BWM"],
    tenggat: "2026-10-16T09:30:00Z", status: "BELUM" as const, target: 1, terisi: 0, tipe: "KONSUMEN" as const,
  };

  it("kode null atau kosong tidak dibawa ke model UI", () => {
    for (const kode of [null, undefined, ""]) {
      const tugas = toTugasLapangan({ ...dasar, kode });
      assert.equal(tugas.id, "77");
      assert.equal("kode" in tugas, false);
      assert.equal(tugas.kode ?? tugas.id, "77", "teks untuk manusia memakai kode ?? id");
    }
  });
});

describe("cariAcuan", () => {
  const data: DataAcuanResponse = {
    versi: "v1",
    toleransi: 0.25,
    komoditas: [{ kode: "CBM-KRT", nama: "Cabai merah keriting", satuan: "kg", harga_acuan: 50000 }],
    pasar: [],
    harga_acuan: [{ komoditas_kode: "CBM-KRT", kabupaten_kode: "1171", tipe: "KONSUMEN", median: 52000, bawah: 39000, atas: 65000, sampel: 8 }],
  };

  it("memakai median resmi kabupaten bila ada", () => {
    assert.deepEqual(cariAcuan(data, "CBM-KRT", "1171", "KONSUMEN"), { median: 52000, bawah: 39000, atas: 65000 });
  });

  it("jatuh ke harga acuan master, produsen 72%, dengan toleransi server", () => {
    assert.deepEqual(cariAcuan(data, "CBM-KRT", "1108", "KONSUMEN"), { median: 50000, bawah: 37500, atas: 62500 });
    assert.deepEqual(cariAcuan(data, "CBM-KRT", "1171", "PRODUSEN"), { median: 36000, bawah: 27000, atas: 45000 });
  });

  it("komoditas tak dikenal tidak punya acuan", () => {
    assert.equal(cariAcuan(data, "XXX", "1171", "KONSUMEN"), undefined);
  });
});

describe("toVersiServer", () => {
  it("membaca versi server dari isi 409", () => {
    const payload = { code: "409", data: { versi_server: { id: "e0", harga: 55000, dicatat_pada: "2026-10-16T01:40:00Z", petugas: "Cut Nanda" } } };
    assert.deepEqual(toVersiServer(payload), { harga: 55000, dicatatPada: "2026-10-16T01:40:00Z", petugas: "Cut Nanda" });
  });

  it("tanpa versi server hasilnya undefined", () => {
    assert.equal(toVersiServer({ code: "409" }), undefined);
    assert.equal(toVersiServer(undefined), undefined);
  });
});

describe("detail tugas", () => {
  const tugas = {
    id: "TGS-mahasiswa_1-2",
    judul: "Verifikasi ulang harga cabai",
    pasar_id: "PSR-1171-01",
    pasar_nama: "Pasar Peunayong",
    komoditas: ["CBM-KRT", "CBR-RWT"],
    tenggat: "2026-10-16T09:30:00Z",
    status: "BERJALAN" as const,
    target: 2,
    terisi: 1,
    tipe: "KONSUMEN" as const,
  };

  it("toTugasDetail menambahkan rincian komoditas terisi dan belum", () => {
    const d = toTugasDetail({
      ...tugas,
      rincian_komoditas: [
        { kode: "CBM-KRT", nama: "Cabai merah keriting", terisi: true },
        { kode: "CBR-RWT", nama: "Cabai rawit", terisi: false },
      ],
    });
    assert.equal(d.pasarNama, "Pasar Peunayong");
    assert.deepEqual(d.rincian, [
      { kode: "CBM-KRT", nama: "Cabai merah keriting", terisi: true },
      { kode: "CBR-RWT", nama: "Cabai rawit", terisi: false },
    ]);
  });

  it("hrefIsiTugas memakai parameter yang dibaca form input", () => {
    assert.equal(
      hrefIsiTugas({ id: "TGS-1", pasarId: "PSR-1171-01", komoditas: ["CBM-KRT", "CBR-RWT"], tipe: "PRODUSEN" }),
      "/lapangan/input?pasar=PSR-1171-01&komoditas=CBM-KRT,CBR-RWT&tipe=PRODUSEN&tugas=TGS-1",
    );
  });

  it("hrefIsiTugas melabeli tugas dengan kode tampilan bila ada, selain itu id BIGINT", () => {
    const dasar = { pasarId: "PSR-1171-01", komoditas: ["BWM"], tipe: "KONSUMEN" as const };
    assert.match(hrefIsiTugas({ ...dasar, id: "42", kode: "TGS-mahasiswa_1-1" }), /&tugas=TGS-mahasiswa_1-1$/);
    assert.match(hrefIsiTugas({ ...dasar, id: "42" }), /&tugas=42$/);
    assert.match(hrefIsiTugas({ ...dasar, id: "42", kode: "TGS x&y" }), /&tugas=TGS%20x%26y$/, "label di-encode agar tidak memecah query");
  });
});
