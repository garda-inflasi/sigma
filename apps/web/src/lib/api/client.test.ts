import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import {
  ApiError,
  apiFetch,
  apiFetchHalaman,
  bacaTotal,
  mockDelay,
} from "./client";

const fetchAsli = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = fetchAsli;
});

function jawab(body: unknown, init: ResponseInit = {}) {
  globalThis.fetch = (async () =>
    new Response(body === undefined ? null : JSON.stringify(body), {
      status: 200,
      headers: {
        "Content-Type": "application/json",
        ...(init.headers as Record<string, string>),
      },
      ...init,
    })) as typeof fetch;
}

describe("bacaTotal", () => {
  const dari = (nilai: string | null) => ({
    get: (nama: string) =>
      nama.toLowerCase() === "x-total-count" ? nilai : null,
  });

  it("membaca bilangan bulat tak negatif", () => {
    assert.equal(bacaTotal(dari("290")), 290);
    assert.equal(bacaTotal(dari(" 0 ")), 0);
  });

  it("header tidak ada atau tidak sah menjadi null", () => {
    assert.equal(bacaTotal(dari(null)), null);
    assert.equal(bacaTotal(dari("")), null);
    assert.equal(bacaTotal(dari("-1")), null);
    assert.equal(bacaTotal(dari("12.5")), null);
    assert.equal(bacaTotal(dari("banyak")), null);
  });
});

describe("apiFetchHalaman", () => {
  it("membuka amplop dan membaca X-Total-Count (tanpa membedakan huruf besar kecil)", async () => {
    jawab(
      {
        code: "200",
        message: "OK",
        data: [{ kode: "1171010" }, { kode: "1171020" }],
      },
      { headers: { "x-total-count": "290" } },
    );

    const hasil = await apiFetchHalaman<{ kode: string }>(
      "/v1/master/kecamatan?page=1&limit=2",
    );

    assert.deepEqual(hasil.data, [{ kode: "1171010" }, { kode: "1171020" }]);
    assert.equal(hasil.total, 290);
  });

  it("tanpa header: total null; data null menjadi daftar kosong", async () => {
    jawab({ code: "200", message: "OK", data: null });

    const hasil = await apiFetchHalaman("/v1/master/kecamatan");

    assert.deepEqual(hasil.data, []);
    assert.equal(hasil.total, null);
  });

  it("galat tetap ApiError dengan status, pesan, dan payload", async () => {
    jawab(
      { code: "VALIDASI", message: "limit terlalu besar" },
      { status: 400 },
    );

    await assert.rejects(
      apiFetchHalaman("/v1/master/kecamatan?limit=9999"),
      (err: unknown) => {
        assert.ok(err instanceof ApiError);
        assert.equal(err.status, 400);
        assert.equal(err.message, "limit terlalu besar");
        assert.equal((err.payload as { code: string }).code, "VALIDASI");
        return true;
      },
    );
  });
});

describe("apiFetch", () => {
  it("204 mengembalikan undefined (hapus)", async () => {
    globalThis.fetch = (async () =>
      new Response(null, { status: 204 })) as typeof fetch;
    assert.equal(
      await apiFetch("/v1/master/pasar/PSR-1", { method: "DELETE" }),
      undefined,
    );
  });

  it("membuka amplop { code, message, data }", async () => {
    jawab({ code: "200", message: "OK", data: { id: "5" } });
    assert.deepEqual(await apiFetch("/v1/master/perubahan/5"), { id: "5" });
  });
});

describe("mockDelay", () => {
  it("galat yang dilempar fungsi menolak promise (bukan galat tak tertangkap di timer)", async () => {
    await assert.rejects(
      mockDelay(
        () => {
          throw new ApiError(404, "tidak ada");
        },
        0,
        1,
      ),
      (err: unknown) => err instanceof ApiError && err.status === 404,
    );
  });

  it("nilai atau fungsi menghasilkan nilainya", async () => {
    assert.equal(await mockDelay(7, 0, 1), 7);
    assert.equal(await mockDelay(() => 8, 0, 1), 8);
  });
});
