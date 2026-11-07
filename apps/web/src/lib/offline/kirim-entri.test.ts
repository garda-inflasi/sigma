import assert from "node:assert/strict";
import { afterEach, before, describe, it } from "node:test";
import type { EntriLokal } from "@/lib/stores/field-queue-store";

process.env.NEXT_PUBLIC_API_BASE_URL = "/api";

const entri: EntriLokal = {
  localId: "uuid-1",
  komoditasKode: "CBM-KRT",
  pasarId: "PSR-1171-01",
  tipe: "KONSUMEN",
  harga: 52000,
  satuan: "kg",
  kualitas: "MEDIUM",
  lat: 5.56,
  lng: 95.32,
  dicatatPada: "2026-10-16T02:00:00.000Z",
  status: "ANTRE",
  percobaan: 0,
};

const asliFetch = globalThis.fetch;
let permintaan: { url: string; init: RequestInit }[] = [];

function jawab(status: number, body?: unknown) {
  globalThis.fetch = (async (url: string, init: RequestInit) => {
    permintaan.push({ url, init });
    return new Response(body === undefined ? null : JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    });
  }) as typeof fetch;
}

describe("kirimEntri", () => {
  let kirimEntri: typeof import("./kirim-entri").kirimEntri;
  before(async () => {
    ({ kirimEntri } = await import("./kirim-entri"));
  });
  afterEach(() => {
    globalThis.fetch = asliFetch;
    permintaan = [];
  });

  it("mengirim ke endpoint lapangan dengan Idempotency-Key = localId", async () => {
    jawab(201, {
      code: "201",
      message: "Created",
      data: { id: "e1", status: "DIAJUKAN" },
    });

    assert.deepEqual(await kirimEntri(entri), { ok: true });

    assert.equal(permintaan.length, 1);
    assert.equal(permintaan[0].url, "/api/v1/lapangan/harga");
    const headers = permintaan[0].init.headers as Record<string, string>;
    assert.equal(headers["Idempotency-Key"], "uuid-1");
    assert.deepEqual(JSON.parse(permintaan[0].init.body as string), {
      komoditas_kode: "CBM-KRT",
      pasar_id: "PSR-1171-01",
      tipe: "KONSUMEN",
      harga: 52000,
      kualitas: "MEDIUM",
      dicatat_pada: "2026-10-16T02:00:00.000Z",
      lat: 5.56,
      lng: 95.32,
    });
  });

  it("antrean tersimpan sebelum migrasi 000111 tetap terkirim: hanya localId dan kolom entri, tanpa ID tugas", async () => {
    // Isi antrean di IndexedDB hanya memuat localId (Idempotency-Key) dan kolom entri. Tugas ditautkan server menurut
    // pasar, tipe, dan komoditas, jadi ID tugas lama (TGS-...) maupun ID BIGINT baru tidak pernah ikut dikirim.
    const tersimpan = JSON.parse(JSON.stringify(entri)) as EntriLokal;
    jawab(201, { code: "201", message: "Created", data: { id: "32290", status: "DIAJUKAN" } });

    assert.deepEqual(await kirimEntri(tersimpan), { ok: true });

    const body = JSON.parse(permintaan[0].init.body as string) as Record<string, unknown>;
    for (const kunci of ["id", "tugas", "tugas_id", "entri_id"]) assert.equal(kunci in body, false, kunci);
    assert.equal((permintaan[0].init.headers as Record<string, string>)["Idempotency-Key"], "uuid-1");
  });

  it("kiriman ulang yang dijawab 200 tetap sukses", async () => {
    jawab(200, {
      code: "200",
      message: "OK",
      data: { id: "e1", status: "DIAJUKAN" },
    });
    assert.deepEqual(await kirimEntri(entri), { ok: true });
  });

  it("409 menjadi konflik beserta versi server", async () => {
    jawab(409, {
      code: "409",
      message: "harga bentrok dengan entri petugas lain",
      data: {
        versi_server: {
          id: "e0",
          harga: 55000,
          dicatat_pada: "2026-10-16T01:40:00Z",
          petugas: "Cut Nanda",
        },
      },
    });

    assert.deepEqual(await kirimEntri(entri), {
      konflik: true,
      versiServer: {
        harga: 55000,
        dicatatPada: "2026-10-16T01:40:00Z",
        petugas: "Cut Nanda",
      },
    });
  });

  it("409 tanpa versi server dianggap galat, bukan konflik", async () => {
    jawab(409, { code: "409", message: "bentrok" });
    await assert.rejects(() => kirimEntri(entri));
  });

  it("pertahankan ikut terkirim setelah petugas memilih versinya", async () => {
    jawab(201, {
      code: "201",
      message: "Created",
      data: { id: "e2", status: "DIAJUKAN" },
    });
    await kirimEntri({ ...entri, pertahankan: true });
    assert.equal(
      JSON.parse(permintaan[0].init.body as string).pertahankan,
      true,
    );
  });

  it("galat server dilempar agar mesin sinkron mencatat GAGAL", async () => {
    jawab(503, { code: "503", message: "layanan sibuk" });
    await assert.rejects(() => kirimEntri(entri), /layanan sibuk/);
  });

  it("galat jaringan dilempar", async () => {
    globalThis.fetch = (async () => {
      throw new TypeError("Failed to fetch");
    }) as typeof fetch;
    await assert.rejects(() => kirimEntri(entri), /Failed to fetch/);
  });
});
