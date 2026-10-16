import assert from "node:assert/strict";
import { afterEach, before, beforeEach, describe, it } from "node:test";

let BffApiError: (typeof import("./api-client"))["BffApiError"];
let coldStorageApi: (typeof import("./api-client"))["coldStorageApi"];

// API_BASE dibaca saat modul dimuat, jadi env harus diatur sebelum impor.
before(async () => {
  process.env.GO_API_BASE_URL = "http://go.test/v1";
  ({ BffApiError, coldStorageApi } = await import("./api-client"));
});

interface Call {
  url: string;
  init: RequestInit;
}

const realFetch = globalThis.fetch;
let calls: Call[] = [];

/** Mengganti fetch dengan respons berurutan; mencatat setiap panggilan. */
function stubFetch(...responses: Response[]) {
  calls = [];
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push({ url: String(input), init: init ?? {} });
    const next = responses.shift();
    assert.ok(next, "fetch dipanggil lebih banyak dari respons yang disiapkan");
    return next;
  }) as typeof fetch;
}

const envelope = (data: unknown, code = "200") => Response.json({ code, message: "OK", data });
const headerOf = (c: Call, name: string) => new Headers(c.init.headers).get(name);

beforeEach(() => {
  calls = [];
});
afterEach(() => {
  globalThis.fetch = realFetch;
});

describe("coldStorageApi.list", () => {
  it("memanggil GET /v1/cold-storage dengan bearer token dan membuka envelope", async () => {
    stubFetch(envelope({ fasilitas: [], stok: [] }));

    const data = await coldStorageApi.list("tok-123");

    assert.deepEqual(data, { fasilitas: [], stok: [] });
    assert.equal(calls.length, 1);
    assert.equal(calls[0].url, "http://go.test/v1/cold-storage");
    assert.equal(calls[0].init.method, "GET");
    assert.equal(headerOf(calls[0], "authorization"), "Bearer tok-123");
    assert.equal(headerOf(calls[0], "accept"), "application/json");
  });

  it("meneruskan kriteria sebagai query string kontrak API", async () => {
    stubFetch(envelope({}));

    await coldStorageApi.list("t", { fasilitasId: "CS-03", q: "tomat", sort: "volume", desc: true, page: 2, limit: 25 });

    const url = new URL(calls[0].url);
    assert.equal(url.pathname, "/v1/cold-storage");
    assert.deepEqual(Object.fromEntries(url.searchParams), {
      cold_storage_id: "CS-03",
      q: "tomat",
      sort: "volume",
      order: "desc",
      page: "2",
      limit: "25",
    });
  });

  it("kata kunci berisi karakter khusus tidak menyisipkan parameter", async () => {
    stubFetch(envelope({}));

    await coldStorageApi.list("t", { q: "x&order=desc#frag" });

    const url = new URL(calls[0].url);
    assert.equal(url.searchParams.get("q"), "x&order=desc#frag");
    assert.equal(url.searchParams.get("order"), null);
  });

  it("memetakan galat backend ke BffApiError beserta status, pesan, dan kode", async () => {
    stubFetch(Response.json({ code: "403", message: "Akses ditolak" }, { status: 403 }));

    await assert.rejects(
      () => coldStorageApi.list("t"),
      (err: unknown) => {
        assert.ok(err instanceof BffApiError);
        assert.equal(err.status, 403);
        assert.equal(err.message, "Akses ditolak");
        assert.equal(err.code, "403");
        return true;
      },
    );
  });

  it("galat tanpa badan JSON memakai pesan bawaan berisi status", async () => {
    stubFetch(new Response("Bad Gateway", { status: 502 }));

    await assert.rejects(() => coldStorageApi.list("t"), (err: unknown) => err instanceof BffApiError && err.status === 502 && /502/.test(err.message));
  });

  it("envelope dengan kode bukan 200/201 diperlakukan sebagai galat", async () => {
    stubFetch(Response.json({ code: "500", message: "Gagal mengambil data cold storage" }));

    await assert.rejects(() => coldStorageApi.list("t"), (err: unknown) => err instanceof BffApiError && err.code === "500");
  });
});

describe("coldStorageApi.bulk", () => {
  it("memanggil POST /v1/cold-storage/bulk dengan badan JSON dan bearer token", async () => {
    stubFetch(envelope({ berhasil: 2, gagal: 0, volume_ton: 65.9 }));

    const res = await coldStorageApi.bulk("tok", { action: "rencanakan_distribusi", ids: ["STK-1", "STK-2"] });

    assert.deepEqual(res, { berhasil: 2, gagal: 0, volume_ton: 65.9 });
    assert.equal(calls[0].url, "http://go.test/v1/cold-storage/bulk");
    assert.equal(calls[0].init.method, "POST");
    assert.equal(headerOf(calls[0], "authorization"), "Bearer tok");
    assert.equal(headerOf(calls[0], "content-type"), "application/json");
    assert.deepEqual(JSON.parse(String(calls[0].init.body)), { action: "rencanakan_distribusi", ids: ["STK-1", "STK-2"] });
  });

  it("izin ditolak backend sampai ke pemanggil sebagai 403", async () => {
    stubFetch(Response.json({ code: "403", message: "Akses ditolak" }, { status: 403 }));

    await assert.rejects(
      () => coldStorageApi.bulk("t", { action: "rekomendasi_olahan", ids: ["S1"] }),
      (err: unknown) => err instanceof BffApiError && err.status === 403,
    );
  });
});
