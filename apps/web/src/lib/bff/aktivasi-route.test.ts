import assert from "node:assert/strict";
import { afterEach, before, describe, it } from "node:test";
import { NextRequest } from "next/server";

let POST: (typeof import("@/app/api/auth/aktivasi/route"))["POST"];

before(async () => {
  process.env.GO_API_BASE_URL = "http://go.test/v1";
  ({ POST } = await import("@/app/api/auth/aktivasi/route"));
});

const realFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = realFetch;
});

let ip = 0;
const ipBaru = () => `198.51.100.${++ip}`;

function permintaan(klien: string, body: unknown) {
  return new NextRequest("http://localhost/api/auth/aktivasi", {
    method: "POST",
    headers: {
      "x-forwarded-for": klien,
      host: "localhost",
      "content-type": "application/json",
    },
    body: typeof body === "string" ? body : JSON.stringify(body),
  });
}

function goMenjawab(status: number, body: unknown) {
  const panggilan: { url: string; body: unknown; auth: string | null }[] = [];
  globalThis.fetch = (async (url: RequestInfo | URL, init?: RequestInit) => {
    panggilan.push({
      url: String(url),
      body: typeof init?.body === "string" ? JSON.parse(init.body) : undefined,
      auth: new Headers(init?.headers).get("authorization"),
    });
    return new Response(JSON.stringify(body), {
      status,
      headers: { "content-type": "application/json" },
    });
  }) as typeof fetch;
  return panggilan;
}

const SANDI = "Rahasia#2026Kuat";

describe("POST /api/auth/aktivasi", () => {
  it("meneruskan token dan kata sandi ke API Go tanpa sesi; jawabannya tidak memuat token", async () => {
    const go = goMenjawab(200, { code: "200", message: "OK" });

    const res = await POST(
      permintaan(ipBaru(), {
        token: "tok-abc",
        password: SANDI,
        ekstra: "diabaikan",
      }),
    );

    assert.equal(res.status, 200);
    assert.match((await res.json()).message, /Akun diaktifkan/);
    assert.equal(go.length, 1);
    assert.equal(go[0].url, "http://go.test/v1/auth/aktivasi");
    assert.deepEqual(go[0].body, { token: "tok-abc", password: SANDI });
    assert.equal(go[0].auth, null, "endpoint publik: tidak ada Authorization");
    assert.equal(
      res.headers.get("set-cookie"),
      null,
      "aktivasi tidak menerbitkan sesi",
    );
  });

  it("token atau kata sandi kosong atau bukan teks: 400 tanpa menyentuh API Go", async () => {
    const go = goMenjawab(200, { code: "200", message: "OK" });
    // Batas laju dihitung sebelum validasi (5/menit/IP): tiap kasus dari IP berbeda.
    for (const body of [
      {},
      { token: "t" },
      { password: SANDI },
      { token: "", password: SANDI },
      { token: "t", password: "" },
      { token: 5, password: SANDI },
      { token: "t", password: ["x"] },
    ]) {
      const res = await POST(permintaan(ipBaru(), body));
      assert.equal(res.status, 400, JSON.stringify(body));
    }
    assert.equal(go.length, 0);
  });

  it("tautan tidak sah, terpakai, atau kedaluwarsa (401): pesan server diteruskan tanpa kode palsu", async () => {
    for (const pesan of [
      "Tautan undangan tidak valid atau sudah dipakai. Minta administrator mengirim undangan baru.",
      "Tautan undangan sudah kedaluwarsa. Minta administrator mengirim undangan baru.",
    ]) {
      goMenjawab(401, { code: "401", message: pesan });
      const res = await POST(
        permintaan(ipBaru(), { token: "t", password: SANDI }),
      );
      assert.equal(res.status, 401);
      assert.equal((await res.json()).message, pesan);
    }
  });

  it("kata sandi melanggar kebijakan (400): alasannya sampai ke pengguna", async () => {
    goMenjawab(400, {
      code: "400",
      message: "kata sandi terlalu lemah: harus mengandung huruf kapital",
    });
    const res = await POST(
      permintaan(ipBaru(), { token: "t", password: "semuakecil1!" }),
    );
    assert.equal(res.status, 400);
    assert.match((await res.json()).message, /huruf kapital/);
  });

  it("429 dari API Go (batas laju per IP di sana) diteruskan dengan kode dan Retry-After", async () => {
    globalThis.fetch = (async () =>
      new Response(
        JSON.stringify({
          code: "RATE_LIMITED",
          message: "x",
          data: { sisa_detik: 30 },
        }),
        {
          status: 429,
          headers: { "content-type": "application/json", "retry-after": "30" },
        },
      )) as typeof fetch;

    const res = await POST(
      permintaan(ipBaru(), { token: "t", password: SANDI }),
    );

    assert.equal(res.status, 429);
    assert.equal(res.headers.get("retry-after"), "30");
    const body = await res.json();
    assert.equal(body.kode, "RATE_LIMITED");
    assert.equal(body.sisa_detik, 30);
  });

  it("batas laju BFF: permintaan ke-6 dalam semenit per IP dijawab 429 RATE_LIMITED, IP lain tidak terpengaruh", async () => {
    const go = goMenjawab(401, {
      code: "401",
      message: "Tautan undangan tidak valid atau sudah dipakai.",
    });
    const klien = ipBaru();
    for (let i = 1; i <= 5; i++) {
      const res = await POST(
        permintaan(klien, { token: "tebakan", password: SANDI }),
      );
      assert.equal(res.status, 401, `percobaan ${i}`);
    }

    const ditolak = await POST(
      permintaan(klien, { token: "tebakan", password: SANDI }),
    );

    assert.equal(ditolak.status, 429);
    const detik = Number(ditolak.headers.get("Retry-After"));
    assert.ok(detik >= 1 && detik <= 60, `Retry-After ${detik}`);
    const body = await ditolak.json();
    assert.equal(body.kode, "RATE_LIMITED");
    assert.equal(body.sisa_detik, detik);
    assert.equal(
      go.length,
      5,
      "permintaan yang ditolak tidak sampai ke API Go",
    );

    const lain = await POST(
      permintaan(ipBaru(), { token: "t", password: SANDI }),
    );
    assert.equal(lain.status, 401);
  });

  it("badan bukan JSON: galat, bukan 500 yang membocorkan detail", async () => {
    goMenjawab(200, { code: "200", message: "OK" });
    const res = await POST(permintaan(ipBaru(), "bukan json"));
    assert.ok(res.status >= 400 && res.status < 600);
    assert.equal((await res.json()).error, true);
  });
});
