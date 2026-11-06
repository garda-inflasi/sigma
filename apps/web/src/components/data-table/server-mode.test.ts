import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  catatanPilihanHalaman,
  jumlahHalamanTabel,
  rentangBaris,
} from "./server-mode";

describe("jumlahHalamanTabel", () => {
  it("membulatkan ke atas dan minimal 1", () => {
    assert.equal(jumlahHalamanTabel(0, 25), 1);
    assert.equal(jumlahHalamanTabel(25, 25), 1);
    assert.equal(jumlahHalamanTabel(26, 25), 2);
    assert.equal(jumlahHalamanTabel(250, 10), 25);
    assert.equal(jumlahHalamanTabel(-1, 25), 1);
    assert.equal(jumlahHalamanTabel(5, 0), 5);
  });
});

describe("rentangBaris", () => {
  it("rentang halaman penuh, halaman terakhir sebagian, dan kosong", () => {
    assert.deepEqual(rentangBaris(90, 1, 25), { from: 1, to: 25 });
    assert.deepEqual(rentangBaris(90, 2, 25), { from: 26, to: 50 });
    assert.deepEqual(rentangBaris(90, 4, 25), { from: 76, to: 90 });
    assert.deepEqual(rentangBaris(0, 1, 25), { from: 0, to: 0 });
    assert.deepEqual(rentangBaris(1, 1, 25), { from: 1, to: 1 });
  });

  it("halaman di luar jangkauan tidak menghasilkan rentang terbalik", () => {
    const { from, to } = rentangBaris(10, 5, 25);
    assert.ok(from <= to, `${from} > ${to}`);
    assert.ok(to <= 10);
  });
});

describe("catatanPilihanHalaman", () => {
  it("menjelaskan bahwa pilihan hanya untuk halaman ini bila halaman lain ada", () => {
    assert.equal(
      catatanPilihanHalaman(25, 25, 90, "pengguna"),
      "Seluruh 25 pengguna di halaman ini terpilih. 65 pengguna di halaman lain tidak ikut terpilih.",
    );
  });

  it("diam bila halaman belum terpilih semua atau hanya ada satu halaman", () => {
    assert.equal(catatanPilihanHalaman(3, 25, 90, "pengguna"), null);
    assert.equal(
      catatanPilihanHalaman(25, 25, 25, "pengguna"),
      null,
      "satu halaman: tidak ada yang tertinggal",
    );
    assert.equal(catatanPilihanHalaman(0, 0, 0, "pengguna"), null);
  });

  it("halaman terakhir sebagian yang terpilih semua tetap menyebut halaman lain", () => {
    assert.equal(
      catatanPilihanHalaman(15, 15, 90, "pengguna"),
      "Seluruh 15 pengguna di halaman ini terpilih. 75 pengguna di halaman lain tidak ikut terpilih.",
    );
  });
});
