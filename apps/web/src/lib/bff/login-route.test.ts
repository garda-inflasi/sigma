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

  it("kiriman form native (bukan JSON) ditolak 415 tanpa alihan dan tidak dihitung batas laju", async () => {
    const klien = ipBaru();
    const res = await POST(
      permintaan(klien, {
        form: true,
        body: { email: "a@b.id", password: "x" },
      }),
    );

    assert.equal(res.status, 415);
    assert.equal(res.headers.get("location"), null);
    assert.equal((await res.json()).error, true);
    // Lima permintaan JSON berikutnya masih lolos batas laju (gagal validasi 400, bukan 429).
    await habiskan(klien);
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

  it("sukses: JSON membawa tujuan sesuai redirect dan cookie sesi terpasang", async () => {
    goMenjawab(200, {
      code: "200",
      message: "ok",
      data: {
        access_token: "akses",
        refresh_token: "segar",
        user: { id: "u1", peran: "MANAJER_DATA", wajib_ganti_password: false },
        mfa_setup_diwajibkan: false,
      },
    });

    const res = await POST(
      permintaan(ipBaru(), {
        body: { email: "a@b.id", password: "Sandi#12345", redirect: "/ews?id=7" },
      }),
    );

    assert.equal(res.status, 200);
    assert.equal(res.headers.get("location"), null);
    const body = await res.json();
    assert.equal(body.tujuan, "/ews?id=7");
    assert.equal(body.mfa_required, undefined);
    const cookie = res.headers.getSetCookie().join("\n");
    assert.match(cookie, /garda_access=akses/);
  });

  it("akun ber-MFA: tujuan langkah OTP, tanpa cookie sesi", async () => {
    goMenjawab(200, {
      code: "200",
      message: "ok",
      data: { mfa_required: true, mfa_token: "tantangan" },
    });

    const res = await POST(
      permintaan(ipBaru(), {
        body: { email: "a@b.id", password: "Sandi#12345", redirect: "/kpi" },
      }),
    );

    assert.equal(res.status, 200);
    const body = await res.json();
    assert.deepEqual(body, { mfa_required: true, tujuan: "/masuk?mfa=1&lanjut=%2Fkpi" });
    assert.doesNotMatch(res.headers.getSetCookie().join("\n"), /garda_access=/);
  });

  it("redirect di luar aplikasi diabaikan", async () => {
    goMenjawab(200, {
      code: "200",
      message: "ok",
      data: { mfa_required: true, mfa_token: "tantangan" },
    });

    const res = await POST(
      permintaan(ipBaru(), {
        body: { email: "a@b.id", password: "Sandi#12345", redirect: "//evil.example" },
      }),
    );

    assert.equal((await res.json()).tujuan, "/masuk?mfa=1&lanjut=%2F");
  });

  it("401 kredensial salah: pesan tabel, tanpa hitung mundur", async () => {
    goMenjawab(401, {
      code: "INVALID_CREDENTIALS",
      message: "Email atau password salah",
    });

    const res = await kirim(ipBaru(), false);

    assert.equal(res.status, 401);
    const body = await res.json();
    assert.equal(body.message, "Email atau password salah.");
    assert.equal(body.kode, "INVALID_CREDENTIALS");
    assert.equal(body.sisa_detik, undefined);
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
