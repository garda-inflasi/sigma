import assert from "node:assert/strict";
import { beforeEach, describe, it } from "node:test";
import type { DataAcuanResponse } from "@/lib/api/lapangan";

process.env.NEXT_PUBLIC_API_BASE_URL = "/api";

const data: DataAcuanResponse = {
  versi: "abc123",
  toleransi: 0.25,
  komoditas: [
    {
      kode: "CBM-KRT",
      nama: "Cabai merah keriting",
      satuan: "kg",
      harga_acuan: 52000,
    },
  ],
  pasar: [],
  harga_acuan: [],
};

describe("unduhAcuan", () => {
  let unduhAcuan: typeof import("./acuan-server").unduhAcuan;
  let useAcuanServer: typeof import("./acuan-server").useAcuanServer;
  beforeEach(async () => {
    ({ unduhAcuan, useAcuanServer } = await import("./acuan-server"));
    useAcuanServer.setState({ data: undefined });
  });

  it("menyimpan data baru dari server", async () => {
    const ambil = (async (url: string) => {
      assert.equal(url, "/api/v1/lapangan/acuan");
      return new Response(
        JSON.stringify({ code: "200", message: "OK", data }),
        { status: 200, headers: { ETag: '"abc123"' } },
      );
    }) as unknown as typeof fetch;

    assert.equal(await unduhAcuan(ambil), true);
    assert.deepEqual(useAcuanServer.getState().data, data);
  });

  it("304 dan galat jaringan tidak mengubah data yang ada", async () => {
    useAcuanServer.setState({ data });

    assert.equal(
      await unduhAcuan(
        (async () =>
          new Response(null, { status: 304 })) as unknown as typeof fetch,
      ),
      false,
    );
    assert.equal(
      await unduhAcuan((async () => {
        throw new TypeError("offline");
      }) as unknown as typeof fetch),
      false,
    );
    assert.equal(
      await unduhAcuan(
        (async () =>
          new Response("{}", { status: 403 })) as unknown as typeof fetch,
      ),
      false,
    );
    assert.deepEqual(useAcuanServer.getState().data, data);
  });

  it("jawaban tanpa ETag ditolak", async () => {
    const ambil = (async () =>
      new Response(JSON.stringify({ data }), {
        status: 200,
      })) as unknown as typeof fetch;
    assert.equal(await unduhAcuan(ambil), false);
    assert.equal(useAcuanServer.getState().data, undefined);
  });
});
