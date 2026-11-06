import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { ROLE_IDS, ROLES } from "@/lib/rbac/roles";
import {
  KABUPATEN_SEMUA,
  isianUndangAwal,
  keInputUndang,
  peranButuhKabupaten,
  skemaUndang,
} from "./undang";

const sah = {
  nama: "Budi Santoso",
  email: "budi@gardainflasi.id",
  peran: "OPS_MAHASISWA",
  lembaga: "FMIPA USK",
  kabupaten: KABUPATEN_SEMUA,
};

describe("skemaUndang", () => {
  it("isian sah lolos dan dirapikan (spasi tepi dibuang)", () => {
    const hasil = skemaUndang.parse({
      ...sah,
      nama: "  Budi Santoso ",
      email: " budi@gardainflasi.id ",
      lembaga: " FMIPA USK ",
    });
    assert.equal(hasil.nama, "Budi Santoso");
    assert.equal(hasil.email, "budi@gardainflasi.id");
    assert.equal(hasil.lembaga, "FMIPA USK");
  });

  it("isian wajib kosong ditolak per field dengan pesan Indonesia", () => {
    const hasil = skemaUndang.safeParse({
      ...sah,
      nama: "  ",
      email: "",
      lembaga: "",
    });
    assert.equal(hasil.success, false);
    const pesan = Object.fromEntries(
      hasil.error!.issues.map((i) => [String(i.path[0]), i.message]),
    );
    assert.equal(pesan.nama, "Nama wajib diisi.");
    assert.equal(pesan.email, "Email wajib diisi.");
    assert.equal(pesan.lembaga, "Lembaga wajib diisi.");
  });

  it("email tidak valid ditolak", () => {
    for (const email of ["bukan-email", "a@", "@b.id", "a b@c.id"]) {
      const hasil = skemaUndang.safeParse({ ...sah, email });
      assert.equal(hasil.success, false, email);
      assert.equal(hasil.error?.issues[0].message, "Email tidak valid.", email);
    }
  });

  it("peran harus salah satu dari 14 peran; batas panjang nama dan lembaga mengikuti server", () => {
    assert.equal(
      skemaUndang.safeParse({ ...sah, peran: "DEWA" }).success,
      false,
    );
    for (const peran of ROLE_IDS)
      assert.equal(
        skemaUndang.safeParse({ ...sah, peran }).success,
        true,
        peran,
      );
    assert.equal(
      skemaUndang.safeParse({ ...sah, nama: "a".repeat(255) }).success,
      true,
    );
    assert.equal(
      skemaUndang.safeParse({ ...sah, nama: "a".repeat(256) }).success,
      false,
    );
    assert.equal(
      skemaUndang.safeParse({ ...sah, lembaga: "a".repeat(256) }).success,
      false,
    );
  });
});

describe("isianUndangAwal", () => {
  it("peran bawaan mahasiswa tanpa kabupaten, lolos skema begitu nama, email, dan lembaga diisi", () => {
    const awal = isianUndangAwal();
    assert.equal(awal.peran, "OPS_MAHASISWA");
    assert.equal(awal.kabupaten, KABUPATEN_SEMUA);
    assert.equal(skemaUndang.safeParse(awal).success, false);
    assert.equal(
      skemaUndang.safeParse({
        ...awal,
        nama: "A",
        email: "a@b.id",
        lembaga: "X",
      }).success,
      true,
    );
  });
});

describe("peranButuhKabupaten", () => {
  it("hanya peran yang cakupan datanya bukan seluruh Aceh", () => {
    for (const peran of ROLE_IDS)
      assert.equal(
        peranButuhKabupaten(peran),
        ROLES[peran].cakupan !== "SELURUH",
        peran,
      );
    assert.equal(peranButuhKabupaten("OPS_KOORDINATOR"), true);
    assert.equal(peranButuhKabupaten("OPS_MAHASISWA"), true);
    assert.equal(peranButuhKabupaten("ADMIN_SISTEM"), false);
    assert.equal(peranButuhKabupaten("STRATEGIS_BI"), false);
  });
});

describe("keInputUndang", () => {
  it("kabupaten dikirim hanya untuk peran yang membutuhkannya dan bila dipilih", () => {
    const nilai = (peran: string, kabupaten: string) =>
      skemaUndang.parse({ ...sah, peran, kabupaten });

    assert.deepEqual(keInputUndang(nilai("OPS_KOORDINATOR", "1171")), {
      nama: "Budi Santoso",
      email: "budi@gardainflasi.id",
      peran: "OPS_KOORDINATOR",
      lembaga: "FMIPA USK",
      kabupaten_kode: "1171",
    });
    assert.equal(
      "kabupaten_kode" in
        keInputUndang(nilai("OPS_KOORDINATOR", KABUPATEN_SEMUA)),
      false,
      "seluruh Aceh = tanpa kabupaten",
    );
    assert.equal(
      "kabupaten_kode" in keInputUndang(nilai("OPS_KOORDINATOR", "")),
      false,
    );
    assert.equal(
      "kabupaten_kode" in keInputUndang(nilai("ADMIN_SISTEM", "1171")),
      false,
      "peran tanpa cakupan wilayah mengabaikan kabupaten sisa",
    );
  });

  it("email dikirim huruf kecil", () => {
    assert.equal(
      keInputUndang(
        skemaUndang.parse({ ...sah, email: "Budi.Santoso@GardaInflasi.ID" }),
      ).email,
      "budi.santoso@gardainflasi.id",
    );
  });
});
