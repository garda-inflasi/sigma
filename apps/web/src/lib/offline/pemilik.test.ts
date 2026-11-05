import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { pastikanPemilik, type DepsPemilik } from "./pemilik";

function deps(
  awal: { pemilik?: string; belumTerkirim?: number },
  ubah: Partial<DepsPemilik> = {},
) {
  const catatan = {
    ditulis: [] as string[],
    dibersihkan: 0,
    pemilik: awal.pemilik,
  };
  const d: DepsPemilik = {
    bacaPemilik: async () => catatan.pemilik,
    tulisPemilik: async (id) => {
      catatan.ditulis.push(id);
      catatan.pemilik = id;
    },
    hitungBelumTerkirim: async () => awal.belumTerkirim ?? 0,
    bersihkan: async () => {
      catatan.dibersihkan++;
      catatan.pemilik = undefined; // IndexedDB dikosongkan, penanda ikut hilang
      return { galat: [] };
    },
    ...ubah,
  };
  return { d, catatan };
}

describe("pastikanPemilik", () => {
  it("pemilik sama: tidak menyentuh data", async () => {
    const { d, catatan } = deps({ pemilik: "u1", belumTerkirim: 3 });
    const h = await pastikanPemilik("u1", d);
    assert.deepEqual(h, { status: "sama", aman: true, entriDibuang: 0 });
    assert.equal(catatan.dibersihkan, 0);
    assert.deepEqual(catatan.ditulis, []);
  });

  it("belum ada pemilik (perangkat baru, sesudah Keluar, atau data lama): pengguna ini menjadi pemilik tanpa menghapus", async () => {
    const { d, catatan } = deps({ belumTerkirim: 2 });
    const h = await pastikanPemilik("u1", d);
    assert.deepEqual(h, { status: "baru", aman: true, entriDibuang: 0 });
    assert.equal(catatan.dibersihkan, 0);
    assert.deepEqual(catatan.ditulis, ["u1"]);
  });

  it("pemilik berbeda: data dibersihkan lalu pengguna baru menjadi pemilik, jumlah entri yang dibuang dilaporkan", async () => {
    const { d, catatan } = deps({ pemilik: "u1", belumTerkirim: 4 });
    const h = await pastikanPemilik("u2", d);
    assert.deepEqual(h, { status: "dialihkan", aman: true, entriDibuang: 4 });
    assert.equal(catatan.dibersihkan, 1);
    assert.deepEqual(catatan.ditulis, ["u2"]);
    assert.equal(catatan.pemilik, "u2");
  });

  it("pemilik berbeda tanpa entri tertahan: dialihkan dengan nol entri dibuang", async () => {
    const { d } = deps({ pemilik: "u1", belumTerkirim: 0 });
    assert.deepEqual(await pastikanPemilik("u2", d), {
      status: "dialihkan",
      aman: true,
      entriDibuang: 0,
    });
  });

  it("pembersihan gagal: antrean tidak aman dipulihkan dan pemilik baru TIDAK dicatat", async () => {
    const { d, catatan } = deps(
      { pemilik: "u1", belumTerkirim: 1 },
      { bersihkan: async () => ({ galat: ["indexeddb: terkunci"] }) },
    );
    const h = await pastikanPemilik("u2", d);
    assert.deepEqual(h, {
      status: "gagal-bersihkan",
      aman: false,
      entriDibuang: 0,
    });
    assert.deepEqual(catatan.ditulis, []);
    assert.equal(
      catatan.pemilik,
      "u1",
      "percobaan berikutnya masih melihat pemilik lama dan mengulang pembersihan",
    );
  });

  it("IndexedDB tidak terbaca: berjalan tanpa persistensi, tidak membersihkan apa pun", async () => {
    const { d, catatan } = deps(
      {},
      {
        bacaPemilik: async () => {
          throw new Error("SecurityError");
        },
      },
    );
    const h = await pastikanPemilik("u1", d);
    assert.deepEqual(h, { status: "tak-terbaca", aman: true, entriDibuang: 0 });
    assert.equal(catatan.dibersihkan, 0);
    assert.deepEqual(catatan.ditulis, []);
  });

  it("gagal menulis penanda tidak menggagalkan: dicoba lagi pada pemuatan berikutnya", async () => {
    const tulisGagal = async () => {
      throw new Error("quota");
    };
    const baru = await pastikanPemilik(
      "u1",
      deps({}, { tulisPemilik: tulisGagal }).d,
    );
    assert.equal(baru.status, "baru");
    assert.equal(baru.aman, true);

    const alih = await pastikanPemilik(
      "u2",
      deps({ pemilik: "u1", belumTerkirim: 1 }, { tulisPemilik: tulisGagal }).d,
    );
    assert.equal(alih.status, "dialihkan");
    assert.equal(alih.aman, true);
  });

  it("gagal menghitung entri tertahan tidak menghalangi pembersihan", async () => {
    const { d, catatan } = deps(
      { pemilik: "u1" },
      {
        hitungBelumTerkirim: async () => {
          throw new Error("rusak");
        },
      },
    );
    const h = await pastikanPemilik("u2", d);
    assert.deepEqual(h, { status: "dialihkan", aman: true, entriDibuang: 0 });
    assert.equal(catatan.dibersihkan, 1);
  });

  it("urutan A → B → A: setiap peralihan membersihkan, kembalinya A tidak memulihkan data lama", async () => {
    const { d, catatan } = deps({ pemilik: "A", belumTerkirim: 2 });
    assert.equal((await pastikanPemilik("B", d)).status, "dialihkan");
    assert.equal((await pastikanPemilik("B", d)).status, "sama");
    assert.equal((await pastikanPemilik("A", d)).status, "dialihkan");
    assert.equal(catatan.dibersihkan, 2);
    assert.equal(catatan.pemilik, "A");
  });
});
