import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  COLD_STORAGE_BULK_ACTIONS,
  COLD_STORAGE_MAX_LIMIT,
  COLD_STORAGE_MAX_SEARCH,
  COLD_STORAGE_SORTS,
  parseColdStorageQuery,
  toQueryString,
} from "./cold-storage-query";

const parse = (qs: string) => parseColdStorageQuery(new URLSearchParams(qs));

describe("toQueryString", () => {
  it("memakai nama parameter kontrak API", () => {
    const qs = toQueryString({ fasilitasId: "CS-03", q: "tomat", sort: "volume", desc: true, page: 2, limit: 25 });
    assert.deepEqual(Object.fromEntries(new URLSearchParams(qs)), {
      cold_storage_id: "CS-03",
      q: "tomat",
      sort: "volume",
      order: "desc",
      page: "2",
      limit: "25",
    });
  });

  it("membuang nilai kosong", () => {
    assert.equal(toQueryString({}), "");
    assert.equal(toQueryString({ q: "   ", fasilitasId: "" }), "");
  });

  it("memangkas spasi kata kunci", () => {
    assert.equal(new URLSearchParams(toQueryString({ q: "  cabai  " })).get("q"), "cabai");
  });

  it("menyandi karakter khusus agar tidak menyisipkan parameter lain", () => {
    const qs = toQueryString({ q: "a&sort=password#" });
    const sp = new URLSearchParams(qs);
    assert.equal(sp.get("q"), "a&sort=password#");
    assert.equal(sp.get("sort"), null);
  });

  it("order hanya dikirim bila arah ditentukan", () => {
    assert.equal(new URLSearchParams(toQueryString({ desc: false })).get("order"), "asc");
    assert.equal(new URLSearchParams(toQueryString({})).get("order"), null);
  });
});

describe("parseColdStorageQuery", () => {
  it("menerima kombinasi sah dan memetakannya ke parameter UI", () => {
    const r = parse("cold_storage_id=CS-01&q=cabai&sort=risiko&order=desc&page=3&limit=50");
    assert.deepEqual(r, { ok: true, params: { fasilitasId: "CS-01", q: "cabai", sort: "risiko", desc: true, page: 3, limit: 50 } });
  });

  it("kosong berarti bawaan", () => {
    assert.deepEqual(parse(""), { ok: true, params: {} });
  });

  it("mengabaikan parameter yang tidak dikenal agar tidak diteruskan ke backend", () => {
    const r = parse("admin=true&sort=volume&role=ADMIN_SISTEM");
    assert.deepEqual(r, { ok: true, params: { sort: "volume" } });
  });

  it("setiap kolom urut yang didukung diterima", () => {
    for (const sort of COLD_STORAGE_SORTS) assert.equal(parse(`sort=${sort}`).ok, true, sort);
  });

  for (const [nama, qs] of Object.entries({
    "sort tidak dikenal": "sort=password",
    "sort berisi SQL": "sort=volume;DROP TABLE x",
    "order tidak dikenal": "order=sideways",
    "halaman nol": "page=0",
    "halaman negatif": "page=-1",
    "halaman desimal": "page=1.5",
    "halaman bukan angka": "page=abc",
    "limit nol": "limit=0",
    [`limit melebihi ${COLD_STORAGE_MAX_LIMIT}`]: `limit=${COLD_STORAGE_MAX_LIMIT + 1}`,
    "limit bukan angka": "limit=banyak",
    "kata kunci terlalu panjang": `q=${"a".repeat(COLD_STORAGE_MAX_SEARCH + 1)}`,
    "id fasilitas terlalu panjang": `cold_storage_id=${"x".repeat(256)}`,
  })) {
    it(`menolak ${nama}`, () => {
      const r = parse(qs);
      assert.equal(r.ok, false);
      if (!r.ok) assert.ok(r.message.length > 0);
    });
  }

  it("batas atas yang sah diterima", () => {
    assert.equal(parse(`limit=${COLD_STORAGE_MAX_LIMIT}`).ok, true);
    assert.equal(parse(`q=${"a".repeat(COLD_STORAGE_MAX_SEARCH)}`).ok, true);
  });

  it("hasil toQueryString dapat dibaca kembali tanpa kehilangan nilai", () => {
    const params = { fasilitasId: "CS-02", q: "ikan", sort: "masuk" as const, desc: true, page: 4, limit: 25 };
    assert.deepEqual(parse(toQueryString(params)), { ok: true, params });
  });
});

describe("aksi massal", () => {
  it("sama dengan aksi yang diterima backend", () => {
    assert.deepEqual([...COLD_STORAGE_BULK_ACTIONS], ["rencanakan_distribusi", "rekomendasi_olahan"]);
  });
});
