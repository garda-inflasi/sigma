import assert from "node:assert/strict";
import { afterEach, before, beforeEach, describe, it } from "node:test";

// API_BASE dibaca saat modul dimuat, jadi harus diisi sebelum endpoints diimpor.
process.env.NEXT_PUBLIC_API_BASE_URL = "http://api.test";
let api: typeof import("./endpoints").api;
before(async () => {
  ({ api } = await import("./endpoints"));
});

type Panggilan = { url: string; method: string; body: unknown };

describe("api.ews.batalPush", () => {
  const asli = globalThis.fetch;
  let panggilan: Panggilan[];

  const jawab = (status: number, body: unknown) => {
    globalThis.fetch = (async (url: string | URL | Request, init?: RequestInit) => {
      panggilan.push({ url: String(url), method: init?.method ?? "GET", body: JSON.parse(String(init?.body ?? "null")) });
      return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
    }) as typeof fetch;
  };

  beforeEach(() => {
    panggilan = [];
  });
  afterEach(() => {
    globalThis.fetch = asli;
  });

  it("mengirim DELETE dengan endpoint langganan", async () => {
    jawab(200, { code: "200", message: "OK", data: null });
    await api.ews.batalPush("https://push.example/abc");
    assert.deepEqual(panggilan, [
      { url: "http://api.test/v1/ews/notifikasi/langganan-push", method: "DELETE", body: { endpoint: "https://push.example/abc" } },
    ]);
  });

  it("404 (server sudah tidak menyimpan langganan) dianggap berhasil", async () => {
    jawab(404, { code: "404", message: "Langganan push tidak ditemukan" });
    await assert.doesNotReject(api.ews.batalPush("https://push.example/abc"));
  });

  it("galat lain diteruskan agar langganan browser tidak dihapus sepihak", async () => {
    jawab(500, { code: "500", message: "Gagal membatalkan langganan push" });
    await assert.rejects(api.ews.batalPush("https://push.example/abc"), /Gagal membatalkan langganan push/);
  });
});
