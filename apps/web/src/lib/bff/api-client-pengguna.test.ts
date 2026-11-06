import assert from "node:assert/strict";
import { afterEach, before, beforeEach, describe, it } from "node:test";

let userApi: (typeof import("./api-client"))["userApi"];
let PARAM_DAFTAR_PENGGUNA: (typeof import("./api-client"))["PARAM_DAFTAR_PENGGUNA"];

// API_BASE dibaca saat modul dimuat, jadi env harus diatur sebelum impor.
before(async () => {
  process.env.GO_API_BASE_URL = "http://go.test/v1";
  ({ userApi, PARAM_DAFTAR_PENGGUNA } = await import("./api-client"));
});

interface Call {
  url: string;
  init: RequestInit;
}

const realFetch = globalThis.fetch;
let calls: Call[] = [];

function stubFetch(...responses: Response[]) {
  calls = [];
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push({ url: String(input), init: init ?? {} });
    const next = responses.shift();
    assert.ok(next, "fetch dipanggil lebih banyak dari respons yang disiapkan");
    return next;
  }) as typeof fetch;
}

const envelope = (data: unknown) =>
  Response.json({ code: "200", message: "OK", data });

beforeEach(() => {
  calls = [];
});
afterEach(() => {
  globalThis.fetch = realFetch;
});

describe("userApi.list", () => {
  it("tanpa parameter memanggil /users polos", async () => {
    stubFetch(envelope({ items: [], total: 0, page: 1, limit: 20 }));
    await userApi.list("tok");
    assert.equal(calls[0].url, "http://go.test/v1/users");
    assert.equal(calls[0].init.method, "GET");
  });

  it("meneruskan filter, urutan, dan halaman ke API Go", async () => {
    stubFetch(envelope({ items: [], total: 0, page: 2, limit: 25 }));

    await userApi.list("tok", {
      page: 2,
      limit: 25,
      search: "ani aceh",
      peran: "ADMIN_SISTEM",
      status: "AKTIF",
      lapis: "SISTEM",
      mfa: "true",
      terkunci: "false",
      sort: "nama",
      arah: "desc",
    });

    const url = new URL(calls[0].url);
    assert.equal(url.pathname, "/v1/users");
    assert.deepEqual(Object.fromEntries(url.searchParams), {
      page: "2",
      limit: "25",
      search: "ani aceh",
      peran: "ADMIN_SISTEM",
      status: "AKTIF",
      lapis: "SISTEM",
      mfa: "true",
      terkunci: "false",
      sort: "nama",
      arah: "desc",
    });
  });

  it("parameter kosong tidak dikirim", async () => {
    stubFetch(envelope({ items: [], total: 0, page: 1, limit: 20 }));
    await userApi.list("tok", {
      search: "",
      peran: "",
      mfa: undefined,
      page: 1,
    });
    assert.equal(new URL(calls[0].url).search, "?page=1");
  });

  it("daftar parameter yang diteruskan sama dengan kontrak API", () => {
    assert.deepEqual(
      [...PARAM_DAFTAR_PENGGUNA],
      ["search", "peran", "status", "lapis", "mfa", "terkunci", "sort", "arah"],
    );
  });
});

describe("userApi.resetMfa dan riwayatPeran", () => {
  it("resetMfa memanggil POST /users/{id}/reset-mfa dengan id ter-encode dan membuka envelope", async () => {
    stubFetch(
      envelope({ id: "a/b", mfa_aktif_sebelumnya: true, sesi_dicabut: 2 }),
    );

    const hasil = await userApi.resetMfa("tok", "a/b");

    assert.deepEqual(hasil, {
      id: "a/b",
      mfa_aktif_sebelumnya: true,
      sesi_dicabut: 2,
    });
    assert.equal(calls[0].url, "http://go.test/v1/users/a%2Fb/reset-mfa");
    assert.equal(calls[0].init.method, "POST");
    assert.equal(
      new Headers(calls[0].init.headers).get("authorization"),
      "Bearer tok",
    );
  });

  it("riwayatPeran memanggil GET /users/{id}/riwayat-peran", async () => {
    stubFetch(envelope({ items: [], terpotong: false }));

    const hasil = await userApi.riwayatPeran("tok", "u 1");

    assert.deepEqual(hasil, { items: [], terpotong: false });
    assert.equal(calls[0].url, "http://go.test/v1/users/u%201/riwayat-peran");
    assert.equal(calls[0].init.method, "GET");
  });
});
