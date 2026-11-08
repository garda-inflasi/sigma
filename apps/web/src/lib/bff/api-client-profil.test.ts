import assert from "node:assert/strict";
import { afterEach, before, describe, it } from "node:test";

let authApi: (typeof import("./api-client"))["authApi"];
let BffApiError: (typeof import("./api-client"))["BffApiError"];

before(async () => {
  process.env.GO_API_BASE_URL = "http://go.test/v1";
  ({ authApi, BffApiError } = await import("./api-client"));
});

const realFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = realFetch;
});

function stub(...responses: Response[]) {
  const calls: { url: string; init: RequestInit }[] = [];
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push({ url: String(input), init: init ?? {} });
    const next = responses.shift();
    assert.ok(next, "fetch dipanggil lebih banyak dari respons yang disiapkan");
    return next;
  }) as typeof fetch;
  return calls;
}

const envelope = (data: unknown) =>
  Response.json({ code: "200", message: "OK", data });
const header = (c: { init: RequestInit }, name: string) =>
  new Headers(c.init.headers).get(name);

const PROFIL = {
  id: "u-1",
  nama: "Budi Baru",
  email: "budi@garda.test",
  peran: "OPS_KOORDINATOR",
  lapis: "OPERASIONAL",
  lembaga: "FMIPA USK",
  mfa_aktif: true,
  status: "AKTIF",
  dibuat_pada: "2026-10-01T08:00:00Z",
  wajib_ganti_password: false,
  mfa_setup_diwajibkan: false,
  mfa_wajib: true,
  kode_cadangan_tersisa: 8,
  telepon: "+6281234567890",
};

describe("authApi.updateMe", () => {
  it("memanggil PATCH /v1/auth/me dengan bearer token dan badan JSON", async () => {
    const calls = stub(envelope(PROFIL));

    await authApi.updateMe("at", {
      nama: "Budi Baru",
      telepon: "+6281234567890",
    });

    assert.equal(calls.length, 1);
    assert.equal(calls[0].url, "http://go.test/v1/auth/me");
    assert.equal(calls[0].init.method, "PATCH");
    assert.equal(header(calls[0], "authorization"), "Bearer at");
    assert.equal(header(calls[0], "content-type"), "application/json");
    assert.deepEqual(JSON.parse(calls[0].init.body as string), {
      nama: "Budi Baru",
      telepon: "+6281234567890",
    });
  });

  it("hanya field yang diisi yang dikirim; telepon kosong tetap dikirim (hapus nomor)", async () => {
    const calls = stub(envelope(PROFIL), envelope(PROFIL));

    await authApi.updateMe("at", { nama: "Budi Baru" });
    await authApi.updateMe("at", { telepon: "" });

    assert.deepEqual(JSON.parse(calls[0].init.body as string), {
      nama: "Budi Baru",
    });
    assert.deepEqual(JSON.parse(calls[1].init.body as string), { telepon: "" });
  });

  it("membuka envelope dan mengembalikan profil beserta keadaan MFA dan telepon", async () => {
    stub(envelope(PROFIL));

    const hasil = await authApi.updateMe("at", { nama: "Budi Baru" });

    assert.deepEqual(hasil, PROFIL);
    assert.equal(hasil.telepon, "+6281234567890");
    assert.equal(hasil.mfa_wajib, true);
    assert.equal(hasil.kode_cadangan_tersisa, 8);
  });

  it("galat 400 dari API menjadi BffApiError dengan status dan pesannya", async () => {
    stub(
      Response.json(
        { code: "400", message: "Nomor telepon tidak valid" },
        { status: 400 },
      ),
    );

    await assert.rejects(
      () => authApi.updateMe("at", { telepon: "123" }),
      (err: unknown) => {
        assert.ok(err instanceof BffApiError);
        assert.equal(err.status, 400);
        assert.equal(err.message, "Nomor telepon tidak valid");
        return true;
      },
    );
  });
});

describe("authApi.me", () => {
  it("memanggil GET /v1/auth/me dan mengembalikan profil dengan telepon", async () => {
    const calls = stub(envelope(PROFIL));

    const hasil = await authApi.me("at");

    assert.equal(calls[0].url, "http://go.test/v1/auth/me");
    assert.equal(calls[0].init.method, "GET");
    assert.equal(header(calls[0], "authorization"), "Bearer at");
    assert.equal(hasil.telepon, "+6281234567890");
  });
});
