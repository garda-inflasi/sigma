import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { paginationItems } from "./pagination-items";

describe("paginationItems", () => {
  it("tanpa halaman, tanpa tombol", () => {
    assert.deepEqual(paginationItems(1, 0), []);
  });

  it("halaman sedikit ditampilkan seluruhnya", () => {
    assert.deepEqual(paginationItems(1, 1), [1]);
    assert.deepEqual(paginationItems(2, 3), [1, 2, 3]);
    assert.deepEqual(paginationItems(4, 7), [1, 2, 3, 4, 5, 6, 7]);
  });

  it("di awal: elipsis hanya di kanan", () => {
    assert.deepEqual(paginationItems(1, 10), [1, 2, 3, 4, 5, "…", 10]);
    assert.deepEqual(paginationItems(3, 10), [1, 2, 3, 4, 5, "…", 10]);
  });

  it("di tengah: elipsis di kedua sisi", () => {
    assert.deepEqual(paginationItems(5, 10), [1, "…", 4, 5, 6, "…", 10]);
  });

  it("di akhir: elipsis hanya di kiri", () => {
    assert.deepEqual(paginationItems(10, 10), [1, "…", 6, 7, 8, 9, 10]);
    assert.deepEqual(paginationItems(8, 10), [1, "…", 6, 7, 8, 9, 10]);
  });

  it("jumlah tombol tetap agar paginasi tidak melompat saat pindah halaman", () => {
    for (let p = 1; p <= 20; p++) assert.equal(paginationItems(p, 20).length, 7, `halaman ${p}`);
  });

  it("selalu memuat halaman aktif, pertama, dan terakhir", () => {
    for (let p = 1; p <= 20; p++) {
      const items = paginationItems(p, 20);
      assert.ok(items.includes(p), `halaman aktif ${p}`);
      assert.equal(items[0], 1);
      assert.equal(items.at(-1), 20);
    }
  });

  it("urutan naik dan elipsis hanya menggantikan celah", () => {
    for (let p = 1; p <= 20; p++) {
      const nums = paginationItems(p, 20).filter((i): i is number => i !== "…");
      assert.deepEqual(nums, [...nums].sort((a, b) => a - b));
      assert.equal(new Set(nums).size, nums.length, "tanpa duplikat");
    }
  });

  it("halaman aktif di luar jangkauan dijepit", () => {
    assert.deepEqual(paginationItems(99, 10), paginationItems(10, 10));
    assert.deepEqual(paginationItems(-3, 10), paginationItems(1, 10));
  });

  it("jendela sibling dapat diperlebar", () => {
    assert.deepEqual(paginationItems(10, 30, 2), [1, "…", 8, 9, 10, 11, 12, "…", 30]);
  });
});
