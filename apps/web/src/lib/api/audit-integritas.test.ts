import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  labelMasalahAudit,
  mockAuditIntegritas,
  penjelasanIntegritas,
  rentangPartisiAudit,
  toAuditIntegritas,
  type AuditIntegritasResponse,
  type AuditVerifikasiResponse,
} from "./audit";

const HASH = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08";

const verifikasi: AuditVerifikasiResponse = {
  id: "12",
  mulai: "2026-11-07T03:00:00Z",
  selesai: "2026-11-07T03:00:04Z",
  pemicu: "JADWAL",
  status: "VALID",
  jumlah_baris: 12345,
  jumlah_masalah: 0,
  id_awal: "1",
  id_akhir: "12345",
  hash_akhir: HASH,
};

const respons: AuditIntegritasResponse = {
  verifikasi_terakhir: verifikasi,
  partisi: [
    {
      nama: "audit_log_p202612",
      dari: "2026-12-01T00:00:00Z",
      sampai: "2027-01-01T00:00:00Z",
      perkiraan_baris: null,
    },
    {
      nama: "audit_log_p202611",
      dari: "2026-11-01T00:00:00Z",
      sampai: "2026-12-01T00:00:00Z",
      perkiraan_baris: 1234,
    },
  ],
  arsip: [
    {
      id: "1",
      partisi: "audit_log_p202101",
      dari: "2021-01-01T00:00:00Z",
      sampai: "2021-02-01T00:00:00Z",
      jumlah_baris: 100,
      id_pertama: "1",
      id_terakhir: "100",
      berkas: "audit_log_p202101.jsonl.gz",
      sha256: HASH,
      diarsipkan_pada: "2026-02-01T02:00:00Z",
    },
  ],
};

describe("toAuditIntegritas", () => {
  it("memetakan verifikasi, partisi, dan arsip ke camelCase", () => {
    const hasil = toAuditIntegritas(respons);

    assert.deepEqual(hasil.verifikasiTerakhir, {
      id: "12",
      mulai: "2026-11-07T03:00:00Z",
      selesai: "2026-11-07T03:00:04Z",
      pemicu: "JADWAL",
      status: "VALID",
      jumlahBaris: 12345,
      jumlahMasalah: 0,
      idAwal: "1",
      idAkhir: "12345",
      hashAkhir: HASH,
      masalahPertama: undefined,
      keterangan: undefined,
    });
    assert.equal(hasil.partisi.length, 2);
    assert.equal(hasil.partisi[0]?.nama, "audit_log_p202612");
    assert.equal(hasil.partisi[1]?.perkiraanBaris, 1234);
    assert.deepEqual(hasil.arsip[0], {
      id: "1",
      partisi: "audit_log_p202101",
      dari: "2021-01-01T00:00:00Z",
      sampai: "2021-02-01T00:00:00Z",
      jumlahBaris: 100,
      idPertama: "1",
      idTerakhir: "100",
      berkas: "audit_log_p202101.jsonl.gz",
      sha256: HASH,
      diarsipkanPada: "2026-02-01T02:00:00Z",
    });
  });

  it("verifikasi_terakhir null berarti belum pernah diverifikasi", () => {
    const hasil = toAuditIntegritas({
      verifikasi_terakhir: null,
      partisi: [],
      arsip: [],
    });

    assert.equal(hasil.verifikasiTerakhir, null);
    assert.deepEqual(hasil.partisi, []);
    assert.deepEqual(hasil.arsip, []);
  });

  it("partisi dan arsip yang hilang atau null menjadi daftar kosong", () => {
    assert.deepEqual(toAuditIntegritas({}).partisi, []);
    assert.deepEqual(
      toAuditIntegritas({ partisi: null, arsip: null }).arsip,
      [],
    );
  });

  it("perkiraan_baris null menjadi undefined, nol tetap nol", () => {
    const hasil = toAuditIntegritas({
      partisi: [
        { nama: "a", dari: "d", sampai: "s", perkiraan_baris: null },
        { nama: "b", dari: "d", sampai: "s", perkiraan_baris: 0 },
        { nama: "c", dari: "d", sampai: "s" },
      ],
    });

    assert.equal(hasil.partisi[0]?.perkiraanBaris, undefined);
    assert.equal(hasil.partisi[1]?.perkiraanBaris, 0);
    assert.equal(hasil.partisi[2]?.perkiraanBaris, undefined);
  });

  it("bidang opsional null atau kosong menjadi undefined", () => {
    const v = toAuditIntegritas({
      verifikasi_terakhir: {
        ...verifikasi,
        selesai: null,
        id_awal: null,
        id_akhir: "",
        hash_akhir: null,
        masalah_pertama: null,
        keterangan: null,
      },
    }).verifikasiTerakhir;

    assert.equal(v?.selesai, undefined);
    assert.equal(v?.idAwal, undefined);
    assert.equal(v?.idAkhir, undefined);
    assert.equal(v?.hashAkhir, undefined);
    assert.equal(v?.masalahPertama, undefined);
    assert.equal(v?.keterangan, undefined);
  });

  it("RUSAK membawa baris masalah pertama", () => {
    const v = toAuditIntegritas({
      verifikasi_terakhir: {
        ...verifikasi,
        status: "RUSAK",
        pemicu: "MANUAL",
        jumlah_masalah: 3,
        masalah_pertama: {
          id: "4301",
          jenis: "ISI_BERUBAH",
          keterangan: "hash tersimpan tidak sama dengan isi baris",
        },
      },
    }).verifikasiTerakhir;

    assert.equal(v?.status, "RUSAK");
    assert.equal(v?.pemicu, "MANUAL");
    assert.equal(v?.jumlahMasalah, 3);
    assert.deepEqual(v?.masalahPertama, {
      id: "4301",
      jenis: "ISI_BERUBAH",
      keterangan: "hash tersimpan tidak sama dengan isi baris",
    });
  });

  it("GAGAL membawa keterangan galat", () => {
    const v = toAuditIntegritas({
      verifikasi_terakhir: {
        ...verifikasi,
        status: "GAGAL",
        keterangan: "context deadline exceeded",
      },
    }).verifikasiTerakhir;

    assert.equal(v?.status, "GAGAL");
    assert.equal(v?.keterangan, "context deadline exceeded");
  });

  it("status tak dikenal dianggap GAGAL, pemicu tak dikenal dianggap JADWAL", () => {
    const v = toAuditIntegritas({
      verifikasi_terakhir: { ...verifikasi, status: "ANEH", pemicu: "ANEH" },
    }).verifikasiTerakhir;

    assert.equal(v?.status, "GAGAL");
    assert.equal(v?.pemicu, "JADWAL");
  });
});

describe("penjelasanIntegritas", () => {
  const ui = (patch: Partial<AuditVerifikasiResponse>) =>
    toAuditIntegritas({ verifikasi_terakhir: { ...verifikasi, ...patch } })
      .verifikasiTerakhir;

  it("belum diverifikasi", () => {
    assert.match(penjelasanIntegritas(null), /belum pernah dijalankan/);
  });

  it("VALID menyebut jumlah baris", () => {
    assert.equal(
      penjelasanIntegritas(ui({})),
      "Rantai hash utuh: 12.345 baris diverifikasi.",
    );
  });

  it("RUSAK menyebut baris dan jenis masalah, serta masalah lainnya", () => {
    const masalah = { id: "4301", jenis: "RANTAI_PUTUS", keterangan: "x" };
    assert.equal(
      penjelasanIntegritas(
        ui({ status: "RUSAK", jumlah_masalah: 1, masalah_pertama: masalah }),
      ),
      "Terdeteksi perubahan pada baris #4301 (rantai putus).",
    );
    assert.equal(
      penjelasanIntegritas(
        ui({ status: "RUSAK", jumlah_masalah: 3, masalah_pertama: masalah }),
      ),
      "Terdeteksi perubahan pada baris #4301 (rantai putus); 2 masalah lain.",
    );
  });

  it("RUSAK tanpa baris masalah tetap menyebut jumlah masalah", () => {
    assert.equal(
      penjelasanIntegritas(ui({ status: "RUSAK", jumlah_masalah: 2 })),
      "Terdeteksi 2 masalah pada rantai hash.",
    );
  });

  it("GAGAL menyertakan keterangan bila ada", () => {
    assert.equal(
      penjelasanIntegritas(ui({ status: "GAGAL", keterangan: "timeout" })),
      "Verifikasi gagal dijalankan: timeout",
    );
    assert.equal(
      penjelasanIntegritas(ui({ status: "GAGAL" })),
      "Verifikasi gagal dijalankan.",
    );
  });
});

describe("labelMasalahAudit", () => {
  it("menerjemahkan jenis yang dikenal dan meneruskan yang tidak", () => {
    assert.equal(labelMasalahAudit("ISI_BERUBAH"), "isi baris berubah");
    assert.equal(labelMasalahAudit("RANTAI_PUTUS"), "rantai putus");
    assert.equal(
      labelMasalahAudit("ARSIP_TIDAK_COCOK"),
      "tidak cocok dengan arsip",
    );
    assert.equal(labelMasalahAudit("BARU"), "BARU");
  });
});

describe("rentangPartisiAudit", () => {
  it("mengambil bulan tertua dan terbaru tanpa bergantung urutan", () => {
    const { partisi } = toAuditIntegritas(respons);

    assert.deepEqual(rentangPartisiAudit(partisi), {
      jumlah: 2,
      terlama: "2026-11-01T00:00:00Z",
      terbaru: "2026-12-01T00:00:00Z",
    });
    assert.deepEqual(rentangPartisiAudit([...partisi].reverse()), {
      jumlah: 2,
      terlama: "2026-11-01T00:00:00Z",
      terbaru: "2026-12-01T00:00:00Z",
    });
  });

  it("tanpa partisi", () => {
    assert.deepEqual(rentangPartisiAudit([]), {
      jumlah: 0,
      terlama: undefined,
      terbaru: undefined,
    });
  });
});

describe("mockAuditIntegritas", () => {
  it("VALID dengan tiga partisi di sekitar bulan berjalan (terbaru dulu) dan tanpa arsip", () => {
    const mock = mockAuditIntegritas(new Date("2026-11-07T10:00:00Z"));

    assert.equal(mock.verifikasiTerakhir?.status, "VALID");
    assert.deepEqual(
      mock.partisi.map((p) => p.nama),
      ["audit_log_p202612", "audit_log_p202611", "audit_log_p202610"],
    );
    assert.equal(mock.partisi[1]?.dari, "2026-11-01T00:00:00.000Z");
    assert.equal(mock.partisi[1]?.sampai, "2026-12-01T00:00:00.000Z");
    assert.deepEqual(mock.arsip, []);
  });

  it("melewati pergantian tahun", () => {
    const mock = mockAuditIntegritas(new Date("2026-12-31T23:00:00Z"));

    assert.deepEqual(
      mock.partisi.map((p) => p.nama),
      ["audit_log_p202701", "audit_log_p202612", "audit_log_p202611"],
    );
  });
});
