import assert from "node:assert/strict";
import { afterEach, before, describe, it } from "node:test";
import { NextRequest } from "next/server";

let POST: (typeof import("@/app/api/auth/login/route"))["POST"];

before(async () => {
  process.env.GO_API_BASE_URL = "http://go.test/v1";
  ({ POST } = await import("@/app/api/auth/login/route"));
});

const realFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = realFetch;
});

let ip = 0;
const ipBaru = () => `203.0.113.${++ip}`;

function permintaan(
  klien: string,
  opsi: { form?: boolean; body?: unknown } = {},
) {
  const form = opsi.form ?? false;
  return new NextRequest("http://localhost/api/auth/login", {
    method: "POST",
    headers: {
      "x-forwarded-for": klien,
      host: "localhost",
      "content-type": form
        ? "application/x-www-form-urlencoded"
        : "application/json",
    },
    body: form
      ? new URLSearchParams(opsi.body as Record<string, string>).toString()
      : JSON.stringify(opsi.body ?? {}),
  });
}

/** Menghabiskan jendela 5/menit dengan permintaan yang gagal validasi (tidak menyentuh API Go). */
async function habiskan(klien: string) {
  for (let i = 0; i < 5; i++) {
    const res = await POST(permintaan(klien));
    assert.equal(
      res.status,
      400,
      `permintaan ke-${i + 1} lolos batas laju dan gagal validasi`,
    );
  }
}

describe("POST /api/auth/login: batas laju BFF", () => {
  it("permintaan ke-6 dalam semenit dijawab 429 dengan body, Retry-After, dan kode RATE_LIMITED", async () => {
    const klien = ipBaru();
    await habiskan(klien);

    const res = await POST(permintaan(klien));

    assert.equal(res.status, 429);
    const detik = Number(res.headers.get("Retry-After"));
    assert.ok(detik >= 1 && detik <= 60, `Retry-After ${detik}`);
    const body = await res.json();
    assert.equal(body.error, true);
    assert.equal(body.kode, "RATE_LIMITED");
    assert.equal(body.sisa_detik, detik);
    assert.match(
      body.message,
      /^Terlalu banyak percobaan dari perangkat ini\. Coba lagi dalam /,
    );
  });

  it("kiriman form dialihkan ke /masuk dengan kode dan akhir tunggu (bukan JSON mentah)", async () => {
    const klien = ipBaru();
    await habiskan(klien);

    const res = await POST(
      permintaan(klien, {
        form: true,
        body: { email: "a@b.id", password: "x" },
      }),
    );

    assert.ok(res.status >= 300 && res.status < 400, `status ${res.status}`);
    const lokasi = new URL(res.headers.get("location") ?? "");
    assert.equal(lokasi.pathname, "/masuk");
    assert.equal(lokasi.searchParams.get("kode"), "RATE_LIMITED");
    const sampai = Number(lokasi.searchParams.get("sampai"));
    const sisa = sampai - Date.now() / 1000;
    assert.ok(sisa > 0 && sisa <= 61, `sampai ${sisa} detik dari sekarang`);
    assert.match(
      lokasi.searchParams.get("error") ?? "",
      /^Terlalu banyak percobaan dari perangkat ini\./,
    );
  });

  it("IP lain tidak terpengaruh", async () => {
    await habiskan(ipBaru());
    const res = await POST(permintaan(ipBaru()));
    assert.equal(res.status, 400);
  });
});

describe("POST /api/auth/login: galat dari API Go", () => {
  function goMenjawab(
    status: number,
    body: unknown,
    headers: Record<string, string> = {},
  ) {
    globalThis.fetch = (async () =>
      new Response(JSON.stringify(body), {
        status,
        headers: { "content-type": "application/json", ...headers },
      })) as typeof fetch;
  }
  const kirim = (klien: string, form: boolean) =>
    POST(
      permintaan(klien, {
        form,
        body: { email: "a@b.id", password: "Sandi#12345" },
      }),
    );

  it("423 ACCOUNT_LOCKED: JSON meneruskan kode, sisa detik, dan Retry-After", async () => {
    goMenjawab(
      423,
      {
        code: "ACCOUNT_LOCKED",
        message: "Akun terkunci",
        data: { terkunci_sampai: "2026-11-05T10:15:00Z", sisa_detik: 840 },
      },
      { "Retry-After": "840" },
    );

    const res = await kirim(ipBaru(), false);

    assert.equal(res.status, 423);
    assert.equal(res.headers.get("Retry-After"), "840");
    const body = await res.json();
    assert.equal(body.kode, "ACCOUNT_LOCKED");
    assert.equal(body.sisa_detik, 840);
    assert.match(
      body.message,
      /Akun terkunci karena terlalu banyak percobaan gagal\. Coba lagi dalam 14 menit\./,
    );
  });

  it("423 pada kiriman form: alihan /masuk membawa kode dan sampai", async () => {
    goMenjawab(423, {
      code: "ACCOUNT_LOCKED",
      message: "x",
      data: { sisa_detik: 840 },
    });

    const res = await kirim(ipBaru(), true);

    const lokasi = new URL(res.headers.get("location") ?? "");
    assert.equal(lokasi.pathname, "/masuk");
    assert.equal(lokasi.searchParams.get("kode"), "ACCOUNT_LOCKED");
    const sisa = Number(lokasi.searchParams.get("sampai")) - Date.now() / 1000;
    assert.ok(sisa > 835 && sisa <= 841, `sisa ${sisa}`);
    assert.match(
      lokasi.searchParams.get("error") ?? "",
      /^Akun terkunci karena terlalu banyak percobaan gagal\. Coba lagi dalam /,
    );
  });

  it("401 kredensial salah: pesan tabel, tanpa hitung mundur", async () => {
    goMenjawab(401, {
      code: "INVALID_CREDENTIALS",
      message: "Email atau password salah",
    });

    const res = await kirim(ipBaru(), true);

    const lokasi = new URL(res.headers.get("location") ?? "");
    assert.equal(
      lokasi.searchParams.get("error"),
      "Email atau password salah.",
    );
    assert.equal(lokasi.searchParams.get("kode"), "INVALID_CREDENTIALS");
    assert.equal(lokasi.searchParams.has("sampai"), false);
  });

  it("403 akun nonaktif", async () => {
    goMenjawab(403, { code: "ACCOUNT_INACTIVE", message: "Akun tidak aktif" });

    const res = await kirim(ipBaru(), false);

    assert.equal(res.status, 403);
    const body = await res.json();
    assert.equal(body.kode, "ACCOUNT_INACTIVE");
    assert.equal(body.message, "Akun Anda tidak aktif. Hubungi administrator.");
  });
});
