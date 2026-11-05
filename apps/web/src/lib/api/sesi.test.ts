import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import type { SesiWire } from "@/lib/bff/api-client";
import {
  jumlahSesiLain,
  keamananApi,
  rincianSesi,
  toSesi,
  urutkanSesi,
  type Sesi,
} from "./sesi";

const wire = (o: Partial<SesiWire> = {}): SesiWire => ({
  id: "3f1c8a52-0a57-4b9a-8f0e-0d1f6f3c9a11",
  perangkat: "Chrome di macOS",
  user_agent: "Mozilla/5.0 Chrome/120",
  ip: "203.0.113.7",
  dibuat_pada: "2026-11-01T08:00:00Z",
  terakhir_dipakai_pada: "2026-11-05T09:30:00Z",
  kedaluwarsa_pada: "2026-11-12T09:30:00Z",
  saat_ini: false,
  ...o,
});

describe("toSesi", () => {
  it("memetakan semua kolom ke model UI", () => {
    assert.deepEqual(toSesi(wire({ saat_ini: true })), {
      id: "3f1c8a52-0a57-4b9a-8f0e-0d1f6f3c9a11",
      perangkat: "Chrome di macOS",
      userAgent: "Mozilla/5.0 Chrome/120",
      ip: "203.0.113.7",
      dibuatPada: "2026-11-01T08:00:00Z",
      terakhirDipakai: "2026-11-05T09:30:00Z",
      kedaluwarsaPada: "2026-11-12T09:30:00Z",
      saatIni: true,
    });
  });

  it("kolom opsional kosong menjadi string kosong dan perangkat kosong mendapat nama bawaan", () => {
    const s = toSesi(
      wire({ user_agent: undefined, ip: undefined, perangkat: "  " }),
    );
    assert.equal(s.userAgent, "");
    assert.equal(s.ip, "");
    assert.equal(s.perangkat, "Perangkat tidak dikenal");
  });

  it("saat_ini hanya true bila benar-benar true", () => {
    assert.equal(
      toSesi(wire({ saat_ini: undefined as unknown as boolean })).saatIni,
      false,
    );
  });
});

describe("urutkanSesi", () => {
  const s = (id: string, terakhir: string, saatIni = false): Sesi =>
    toSesi(wire({ id, terakhir_dipakai_pada: terakhir, saat_ini: saatIni }));

  it("sesi ini paling atas, lalu yang terakhir dipakai paling awal", () => {
    const daftar = [
      s("lama", "2026-11-01T00:00:00Z"),
      s("ini", "2026-10-01T00:00:00Z", true),
      s("baru", "2026-11-05T00:00:00Z"),
    ];
    assert.deepEqual(
      urutkanSesi(daftar).map((x) => x.id),
      ["ini", "baru", "lama"],
    );
  });

  it("tidak mengubah masukan", () => {
    const daftar = [
      s("a", "2026-11-01T00:00:00Z"),
      s("b", "2026-11-05T00:00:00Z"),
    ];
    urutkanSesi(daftar);
    assert.deepEqual(
      daftar.map((x) => x.id),
      ["a", "b"],
    );
  });

  it("tanggal rusak dianggap paling lama, tidak melempar", () => {
    const daftar = [
      s("rusak", "bukan-tanggal"),
      s("baik", "2026-11-05T00:00:00Z"),
    ];
    assert.deepEqual(
      urutkanSesi(daftar).map((x) => x.id),
      ["baik", "rusak"],
    );
  });
});

describe("jumlahSesiLain dan rincianSesi", () => {
  it("menghitung sesi selain yang sedang dipakai", () => {
    const daftar = [
      toSesi(wire({ saat_ini: true })),
      toSesi(wire()),
      toSesi(wire()),
    ];
    assert.equal(jumlahSesiLain(daftar), 2);
    assert.equal(jumlahSesiLain([]), 0);
    assert.equal(jumlahSesiLain([daftar[0]]), 0);
  });

  it("rincian memuat IP atau menyatakan tidak diketahui", () => {
    assert.equal(rincianSesi(toSesi(wire())), "IP 203.0.113.7");
    assert.equal(
      rincianSesi(toSesi(wire({ ip: undefined }))),
      "IP tidak diketahui",
    );
  });
});

describe("keamananApi", () => {
  const realFetch = globalThis.fetch;
  afterEach(() => {
    globalThis.fetch = realFetch;
  });

  function stub(...responses: Response[]) {
    const calls: { url: string; init: RequestInit }[] = [];
    globalThis.fetch = (async (
      input: RequestInfo | URL,
      init?: RequestInit,
    ) => {
      calls.push({ url: String(input), init: init ?? {} });
      const next = responses.shift();
      assert.ok(
        next,
        "fetch dipanggil lebih banyak dari respons yang disiapkan",
      );
      return next;
    }) as typeof fetch;
    return calls;
  }

  it("daftar memetakan dan mengurutkan hasil BFF", async () => {
    stub(
      Response.json([
        wire({ id: "lain", terakhir_dipakai_pada: "2026-11-05T10:00:00Z" }),
        wire({ id: "ini", saat_ini: true }),
      ]),
    );
    const daftar = await keamananApi.daftar();
    assert.deepEqual(
      daftar.map((x) => x.id),
      ["ini", "lain"],
    );
    assert.equal(daftar[0].saatIni, true);
  });

  it("cabut memanggil DELETE dengan id yang di-encode", async () => {
    const calls = stub(Response.json({ id: "a b" }));
    await keamananApi.cabut("a b");
    assert.equal(calls[0].url, "/api/auth/sessions/a%20b");
    assert.equal(calls[0].init.method, "DELETE");
  });

  it("cabutLain dan keluarSemua memakai POST ke rute BFF masing-masing", async () => {
    const calls = stub(
      Response.json({ dicabut: 2 }),
      Response.json({ dicabut: 3 }),
    );
    assert.deepEqual(await keamananApi.cabutLain(), { dicabut: 2 });
    assert.deepEqual(await keamananApi.keluarSemua(), { dicabut: 3 });
    assert.deepEqual(
      calls.map((c) => [c.url, c.init.method]),
      [
        ["/api/auth/sessions/revoke-others", "POST"],
        ["/api/auth/logout-all", "POST"],
      ],
    );
  });
});
