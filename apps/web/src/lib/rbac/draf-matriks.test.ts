import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { buatStoreDraf } from "./draf-matriks";

describe("store draf matriks", () => {
  it("menyimpan draf per peran dan memberi tahu pendengar", () => {
    const store = buatStoreDraf();
    let dipanggil = 0;
    const lepas = store.langgan(() => dipanggil++);

    store.atur("OPS_MAHASISWA", { versi: 3, izin: new Set(["kpi.lihat"]) });

    assert.equal(dipanggil, 1);
    assert.deepEqual(
      [...(store.ambil().get("OPS_MAHASISWA")?.izin ?? [])],
      ["kpi.lihat"],
    );
    assert.equal(store.ambil().get("OPS_MAHASISWA")?.versi, 3);

    lepas();
    store.atur("MANAJER_DATA", { versi: 1, izin: new Set() });
    assert.equal(dipanggil, 1, "pendengar yang dilepas tidak dipanggil lagi");
  });

  it("setiap perubahan menghasilkan peta baru (aman untuk useSyncExternalStore)", () => {
    const store = buatStoreDraf();
    const sebelum = store.ambil();
    store.atur("OPS_MAHASISWA", { versi: 1, izin: new Set() });
    assert.notEqual(store.ambil(), sebelum);
    assert.equal(sebelum.size, 0, "peta lama tidak berubah");
  });

  it("hapus membuang satu draf; menghapus yang tidak ada tidak memberi tahu siapa pun", () => {
    const store = buatStoreDraf();
    let dipanggil = 0;
    store.langgan(() => dipanggil++);
    store.atur("OPS_MAHASISWA", { versi: 1, izin: new Set() });
    store.atur("MANAJER_DATA", { versi: 1, izin: new Set() });

    store.hapus("OPS_MAHASISWA");
    store.hapus("OPS_MAHASISWA");

    assert.equal(dipanggil, 3);
    assert.deepEqual([...store.ambil().keys()], ["MANAJER_DATA"]);
  });

  it("buangUsang membuang draf yang versi dasarnya sudah tertinggal dari server", () => {
    const store = buatStoreDraf();
    store.atur("OPS_MAHASISWA", { versi: 1, izin: new Set(["a.b"]) });
    store.atur("MANAJER_DATA", { versi: 4, izin: new Set(["c.d"]) });

    const dibuang = store.buangUsang(
      new Map([
        ["OPS_MAHASISWA", 2],
        ["MANAJER_DATA", 4],
      ]),
    );

    assert.deepEqual(dibuang, ["OPS_MAHASISWA"]);
    assert.deepEqual([...store.ambil().keys()], ["MANAJER_DATA"]);
  });

  it("buangUsang tanpa yang usang tidak mengubah apa pun dan mengabaikan peran yang tak dikenal server", () => {
    const store = buatStoreDraf();
    let dipanggil = 0;
    store.atur("OPS_MAHASISWA", { versi: 1, izin: new Set() });
    store.langgan(() => dipanggil++);

    assert.deepEqual(store.buangUsang(new Map([["OPS_MAHASISWA", 1]])), []);
    assert.deepEqual(store.buangUsang(new Map()), []);
    assert.equal(dipanggil, 0);
    assert.equal(store.ambil().size, 1);
  });

  it("bersihkan mengosongkan semua draf", () => {
    const store = buatStoreDraf();
    store.atur("OPS_MAHASISWA", { versi: 1, izin: new Set() });
    store.bersihkan();
    assert.equal(store.ambil().size, 0);
  });
});
