import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  KOLOM_IMPOR,
  LABEL_STATUS_BARIS,
  MAKS_BARIS_IMPOR,
  MAKS_BYTE_IMPOR,
  NAMA_BERKAS_TEMPLATE,
  periksaBerkasImpor,
  ringkasHasil,
  ringkasPratinjau,
  templateImporCsv,
  toHasilImpor,
  type ImporWire,
} from "./impor";

describe("templateImporCsv", () => {
  it("hanya header persis seperti yang diminta server, dengan BOM dan titik koma", () => {
    const teks = templateImporCsv();
    assert.equal(
      teks.charCodeAt(0),
      0xfeff,
      "BOM UTF-8 agar Excel membaca huruf non-ASCII dengan benar",
    );
    assert.equal(teks.slice(1), "nama;email;peran;lembaga;kabupaten_kode\r\n");
    assert.deepEqual(teks.slice(1).trim().split(";"), [...KOLOM_IMPOR]);
  });

  it("tanpa baris contoh: templat yang diunggah tanpa diisi tidak membuat akun", () => {
    assert.equal(templateImporCsv().trim().split(/\r?\n/).length, 1);
  });

  it("kolom dan batas sama dengan server (header canonical, 500 baris, 512 KB)", () => {
    assert.deepEqual(
      [...KOLOM_IMPOR],
      ["nama", "email", "peran", "lembaga", "kabupaten_kode"],
    );
    assert.equal(MAKS_BARIS_IMPOR, 500);
    assert.equal(MAKS_BYTE_IMPOR, 512 * 1024);
    assert.equal(NAMA_BERKAS_TEMPLATE, "template-impor-pengguna.csv");
  });
});

describe("periksaBerkasImpor", () => {
  it("berkas .csv berukuran wajar layak (huruf besar ekstensi juga)", () => {
    assert.equal(
      periksaBerkasImpor({ name: "pengguna.csv", size: 1200 }),
      null,
    );
    assert.equal(periksaBerkasImpor({ name: "PENGGUNA.CSV", size: 1 }), null);
    assert.equal(
      periksaBerkasImpor({ name: "a.csv", size: MAKS_BYTE_IMPOR }),
      null,
      "tepat di batas masih layak",
    );
  });

  it("bukan .csv, kosong, atau terlalu besar ditolak dengan pesan yang bisa ditindaklanjuti", () => {
    assert.match(
      periksaBerkasImpor({ name: "pengguna.xlsx", size: 100 }) ?? "",
      /\.csv/,
    );
    assert.match(
      periksaBerkasImpor({ name: "pengguna.csv.exe", size: 100 }) ?? "",
      /\.csv/,
    );
    assert.match(
      periksaBerkasImpor({ name: "pengguna", size: 100 }) ?? "",
      /\.csv/,
    );
    assert.equal(
      periksaBerkasImpor({ name: "a.csv", size: 0 }),
      "Berkas kosong.",
    );
    assert.equal(
      periksaBerkasImpor({ name: "a.csv", size: MAKS_BYTE_IMPOR + 1 }),
      "Berkas terlalu besar (maksimal 512 KB).",
    );
  });
});

const wire: ImporWire = {
  dry_run: true,
  total: 3,
  valid: 1,
  galat: 2,
  dibuat: 0,
  email_aktif: true,
  baris: [
    {
      baris: 2,
      nama: "Budi",
      email: "budi@garda.test",
      peran: "OPS_MAHASISWA",
      lembaga: "FMIPA",
      kabupaten_kode: "1171",
      status: "valid",
      galat: [],
    },
    {
      baris: 3,
      nama: "",
      email: "salah",
      peran: "DEWA",
      lembaga: "",
      kabupaten_kode: "",
      status: "galat",
      galat: ["Nama wajib diisi.", "Email tidak valid."],
    },
    {
      baris: 4,
      nama: "Siti",
      email: "siti@garda.test",
      peran: "OPS_AGREGATOR",
      lembaga: "BPS",
      kabupaten_kode: "",
      status: "dibuat",
      galat: null,
      id: "ef45e253-50b5-3808-9710-90ed1484f293",
    },
  ],
};

describe("toHasilImpor", () => {
  it("memetakan ringkasan dan baris ke model UI", () => {
    const h = toHasilImpor(wire);
    assert.equal(h.dryRun, true);
    assert.equal(h.total, 3);
    assert.equal(h.valid, 1);
    assert.equal(h.galat, 2);
    assert.equal(h.dibuat, 0);
    assert.equal(h.emailAktif, true);
    assert.equal(h.baris.length, 3);
    assert.deepEqual(h.baris[0], {
      baris: 2,
      nama: "Budi",
      email: "budi@garda.test",
      peran: "OPS_MAHASISWA",
      lembaga: "FMIPA",
      kabupatenKode: "1171",
      status: "valid",
      galat: [],
      id: undefined,
    });
    assert.deepEqual(h.baris[1].galat, [
      "Nama wajib diisi.",
      "Email tidak valid.",
    ]);
    assert.equal(h.baris[1].status, "galat");
  });

  it("galat null atau hilang menjadi larik kosong; id akun dibawa untuk baris yang dibuat", () => {
    const h = toHasilImpor(wire);
    assert.deepEqual(h.baris[2].galat, []);
    assert.equal(h.baris[2].id, "ef45e253-50b5-3808-9710-90ed1484f293");
    assert.deepEqual(toHasilImpor({ ...wire, baris: null }).baris, []);
    assert.deepEqual(toHasilImpor({ ...wire, baris: undefined }).baris, []);
  });

  it("status tak dikenal dianggap galat: tidak pernah menampilkan baris yang tak jelas sebagai sukses", () => {
    const h = toHasilImpor({
      ...wire,
      baris: [{ ...wire.baris![0], status: "aneh" }],
    });
    assert.equal(h.baris[0].status, "galat");
  });

  it("label status berbahasa Indonesia", () => {
    assert.deepEqual(LABEL_STATUS_BARIS, {
      valid: "Valid",
      galat: "Ditolak",
      dibuat: "Dibuat",
    });
  });
});

describe("ringkasan", () => {
  it("pratinjau menyebut jumlah valid dan ditolak", () => {
    assert.equal(
      ringkasPratinjau({ total: 3, valid: 1, galat: 2 }),
      "3 baris diperiksa: 1 valid, 2 ditolak dan tidak akan diimpor.",
    );
    assert.equal(
      ringkasPratinjau({ total: 2, valid: 2, galat: 0 }),
      "2 baris diperiksa, semuanya valid.",
    );
    assert.equal(
      ringkasPratinjau({ total: 2, valid: 0, galat: 2 }),
      "2 baris diperiksa, tidak ada yang valid.",
    );
  });

  it("angka besar memakai pemisah ribuan Indonesia", () => {
    assert.equal(
      ringkasPratinjau({ total: 1500, valid: 1500, galat: 0 }),
      "1.500 baris diperiksa, semuanya valid.",
    );
  });

  it("hasil komit menyebut akun dibuat, baris ditolak, dan peringatan SMTP bila email tidak aktif", () => {
    assert.equal(
      ringkasHasil({ dibuat: 2, galat: 1, emailAktif: true }),
      "2 akun dibuat sebagai undangan, 1 baris ditolak.",
    );
    assert.equal(
      ringkasHasil({ dibuat: 2, galat: 0, emailAktif: true }),
      "2 akun dibuat sebagai undangan.",
    );
    assert.match(
      ringkasHasil({ dibuat: 2, galat: 0, emailAktif: false }),
      /SMTP belum dikonfigurasi, jadi email undangan tidak dikirim\.$/,
    );
  });
});
