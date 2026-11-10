import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  jalurKunci,
  lacakBaca,
  nilaiTakWajar,
  periksaKontrak,
} from "./jejak-kunci";

describe("jalurKunci", () => {
  it("objek bersarang memakai titik dan item larik memakai [] (item sejenis digabung)", () => {
    assert.deepEqual(
      [
        ...jalurKunci({
          a: 1,
          b: { c: [{ d: 1 }, { d: 2, e: null }], f: [1, 2] },
          g: null,
        }),
      ].sort(),
      ["a", "b", "b.c", "b.c[].d", "b.c[].e", "b.f", "g"],
    );
  });

  it("larik di akar dan nilai primitif", () => {
    assert.deepEqual([...jalurKunci([{ x: 1 }, { y: 2 }])].sort(), [
      "[].x",
      "[].y",
    ]);
    assert.deepEqual([...jalurKunci(5)], []);
  });
});

describe("periksaKontrak: menangkap field yang tak terpetakan", () => {
  const fixture = {
    id: "1",
    nama: "A",
    baru: 7,
    anak: { kode: "k", baru: 1 },
    daftar: [{ x: 1, y: 2 }],
  };
  const petakan = (d: typeof fixture) => ({
    id: d.id,
    nama: d.nama,
    kode: d.anak.kode,
    x: d.daftar.map((i) => i.x),
  });

  it("kunci fixture yang tidak dibaca pemeta dilaporkan, termasuk di dalam objek dan item larik", () => {
    const r = periksaKontrak(fixture, petakan);
    assert.deepEqual(r.takTerpetakan, ["anak.baru", "baru", "daftar[].y"]);
    assert.deepEqual(r.takAda, []);
  });

  it("daftar abaikan menutup keputusan yang disengaja, dan entri basi dilaporkan", () => {
    const r = periksaKontrak(fixture, petakan, [
      "baru",
      "anak.baru",
      "daftar[].y",
      "nama",
      "tidak.ada",
    ]);
    assert.deepEqual(r.takTerpetakan, []);
    assert.deepEqual(
      r.abaikanBasi,
      ["nama", "tidak.ada"],
      "nama kini dibaca, tidak.ada bukan kunci fixture",
    );
  });

  it("objek yang diteruskan apa adanya ke hasil dihitung terbaca (tanpa disalin pemeta)", () => {
    const r = periksaKontrak(
      { daftar: [{ kode: "a", nama: "b" }], lain: 1 },
      (d) => ({ daftar: d.daftar }),
    );
    assert.deepEqual(r.takTerpetakan, ["lain"]);
  });

  it("pemeta yang membaca kunci yang tidak dikirim backend dilaporkan", () => {
    const r = periksaKontrak(
      { a: 1, b: { c: 1 } } as { a: number; b: { c: number; d?: number } },
      (d) => [d.a, d.b.c, d.b.d],
    );
    assert.deepEqual(r.takAda, ["b.d"]);
    assert.deepEqual(r.takTerpetakan, []);
  });

  it("tidak membaca apa pun dari objek bernilai null dan tidak menganggapnya hilang", () => {
    const r = periksaKontrak(
      { margin: null as null | { selisih: number } },
      (d) => d.margin && { s: d.margin.selisih },
    );
    assert.deepEqual(r.takTerpetakan, []);
    assert.deepEqual(r.takAda, []);
  });

  it("data asli tidak berubah dan serialisasi hasil tidak mencatat bacaan mesin (toJSON dan sejenisnya)", () => {
    const asli = { a: 1, b: [{ c: 2 }] };
    const jejak = lacakBaca(asli);
    JSON.stringify(jejak.data);
    assert.deepEqual([...jejak.tidakAda], []);
    assert.deepEqual(asli, { a: 1, b: [{ c: 2 }] });
    assert.deepEqual([...jejak.terbaca].sort(), ["a", "b", "b[].c"]);
  });
});

describe("nilaiTakWajar", () => {
  it("menunjuk jalur undefined, NaN, dan tak hingga; null dan nol wajar", () => {
    assert.deepEqual(
      nilaiTakWajar({
        a: undefined,
        b: [1, NaN, Infinity],
        c: { d: null, e: 0 },
      }),
      ["$.a", "$.b[1]=NaN", "$.b[2]=Infinity"],
    );
  });

  it("data bersih tidak menghasilkan apa pun", () => {
    assert.deepEqual(nilaiTakWajar({ a: 1, b: ["x"], c: null }), []);
  });
});
