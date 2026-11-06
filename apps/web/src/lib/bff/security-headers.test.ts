import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { NextResponse } from "next/server";
import {
  applySecurityHeaders,
  errorResponse,
  jsonResponse,
  privateJsonResponse,
} from "./security-headers";

function harapHeaderKeamanan(res: Response) {
  assert.equal(res.headers.get("X-Content-Type-Options"), "nosniff");
  assert.equal(res.headers.get("X-Frame-Options"), "DENY");
  assert.equal(res.headers.get("X-XSS-Protection"), "0");
  assert.equal(
    res.headers.get("Referrer-Policy"),
    "strict-origin-when-cross-origin",
  );
  assert.equal(
    res.headers.get("Permissions-Policy"),
    "camera=(), microphone=(), geolocation=()",
  );
}

describe("applySecurityHeaders", () => {
  it("memasang header keamanan dan mengembalikan respons yang sama", () => {
    const res = NextResponse.json({});
    const hasil = applySecurityHeaders(res);
    assert.equal(hasil, res);
    harapHeaderKeamanan(res);
  });

  // HSTS dikirim Caddy hanya untuk permintaan HTTPS; dari sini akan ikut ke HTTP dan merusak http://localhost.
  it("tidak mengirim Strict-Transport-Security (milik Caddy)", () => {
    const res = applySecurityHeaders(NextResponse.json({}));
    assert.equal(res.headers.get("Strict-Transport-Security"), null);
  });

  it("tidak menimpa header lain yang sudah ada", () => {
    const res = NextResponse.json({});
    res.headers.set("Retry-After", "30");
    applySecurityHeaders(res);
    assert.equal(res.headers.get("Retry-After"), "30");
  });
});

describe("jsonResponse", () => {
  it("200 bawaan dengan badan JSON dan header keamanan", async () => {
    const res = jsonResponse({ a: 1 });
    assert.equal(res.status, 200);
    assert.deepEqual(await res.json(), { a: 1 });
    harapHeaderKeamanan(res);
  });

  it("status ditentukan pemanggil", () => {
    assert.equal(jsonResponse({}, 201).status, 201);
  });

  it("tidak melarang cache (itu tugas privateJsonResponse)", () => {
    assert.equal(jsonResponse({}).headers.get("Cache-Control"), null);
  });
});

describe("errorResponse", () => {
  it("bentuk dasar: error true dan pesan, 400 bawaan", async () => {
    const res = errorResponse("Tidak boleh");
    assert.equal(res.status, 400);
    assert.deepEqual(await res.json(), { error: true, message: "Tidak boleh" });
    harapHeaderKeamanan(res);
  });

  it("membawa kode mesin stabil bila ada", async () => {
    const res = errorResponse("Terkunci", 423, { kode: "ACCOUNT_LOCKED" });
    assert.equal(res.status, 423);
    assert.deepEqual(await res.json(), {
      error: true,
      message: "Terkunci",
      kode: "ACCOUNT_LOCKED",
    });
  });

  it("sisa waktu tunggu menjadi sisa_detik dan Retry-After", async () => {
    const res = errorResponse("Tunggu", 429, {
      kode: "RATE_LIMITED",
      sisaDetik: 42,
    });
    assert.equal(res.headers.get("Retry-After"), "42");
    assert.equal(((await res.json()) as { sisa_detik: number }).sisa_detik, 42);
  });

  it("tanpa sisa waktu (undefined atau 0) tidak ada sisa_detik maupun Retry-After", async () => {
    for (const sisaDetik of [undefined, 0]) {
      const res = errorResponse("x", 401, { sisaDetik });
      assert.equal(res.headers.get("Retry-After"), null);
      assert.equal("sisa_detik" in ((await res.json()) as object), false);
    }
  });

  it("kode kosong tidak ikut ditulis", async () => {
    const res = errorResponse("x", 400, { kode: "" });
    assert.equal("kode" in ((await res.json()) as object), false);
  });
});

describe("privateJsonResponse", () => {
  it("data per-pengguna tidak boleh disimpan cache bersama maupun peramban", async () => {
    const res = privateJsonResponse({ nama: "Ani" }, 200);
    assert.equal(res.headers.get("Cache-Control"), "private, no-store");
    assert.deepEqual(await res.json(), { nama: "Ani" });
    harapHeaderKeamanan(res);
  });
});
