// Harus sebelum modul Next lain: memasang AsyncLocalStorage global yang dibutuhkan penyimpanan permintaan Next.
import { denganCookie } from "./request-scope-testing";
import assert from "node:assert/strict";
import { afterEach, before, beforeEach, describe, it } from "node:test";
import { NextRequest } from "next/server";
import { ACCESS_COOKIE, CSRF_COOKIE } from "./cookie-policy";

type Handler = (
  req: NextRequest,
  ctx: { params: Promise<{ id: string }> },
) => Promise<Response>;

let undang: Handler;
let undangUlang: Handler;
let impor: Handler;
let eksporGet: Handler;

before(async () => {
  process.env.GO_API_BASE_URL = "http://go.test/v1";
  undang = (await import("@/app/api/pengguna/undang/route")).POST as Handler;
  undangUlang = (await import("@/app/api/pengguna/[id]/undang-ulang/route"))
    .POST as Handler;
  impor = (await import("@/app/api/pengguna/impor/route")).POST as Handler;
  eksporGet = (await import("@/app/api/pengguna/ekspor/route")).GET as Handler;
});

const ENV_ASLI = {
  APP_ALLOWED_ORIGINS: process.env.APP_ALLOWED_ORIGINS,
  NEXT_PUBLIC_APP_URL: process.env.NEXT_PUBLIC_APP_URL,
};
beforeEach(() => {
  delete process.env.APP_ALLOWED_ORIGINS;
  delete process.env.NEXT_PUBLIC_APP_URL;
});

const fetchAsli = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = fetchAsli;
  for (const [k, v] of Object.entries(ENV_ASLI)) {
    if (v === undefined) delete process.env[k];
    else process.env[k] = v;
  }
});

const COOKIE_SAH = `${ACCESS_COOKIE}=jwt-akses; ${CSRF_COOKIE}=csrf-1`;
const KONTEKS = { params: Promise.resolve({ id: "USR/1" }) };

interface PanggilanGo {
  url: string;
  metode: string;
  auth: string | null;
  body: unknown;
}

/** Memasang jawaban API Go dan merekam panggilan yang sampai ke sana. */
function goMenjawab(
  status: number,
  body: unknown | (() => Response),
  headers: Record<string, string> = {},
): PanggilanGo[] {
  const panggilan: PanggilanGo[] = [];
  globalThis.fetch = (async (url: RequestInfo | URL, init?: RequestInit) => {
    const h = new Headers(init?.headers);
    panggilan.push({
      url: String(url),
      metode: init?.method ?? "GET",
      auth: h.get("authorization"),
      body: typeof init?.body === "string" ? JSON.parse(init.body) : undefined,
    });
    if (typeof body === "function") return body();
    return new Response(
      typeof body === "string" ? body : JSON.stringify(body),
      {
        status,
        headers: { "content-type": "application/json", ...headers },
      },
    );
  }) as typeof fetch;
  return panggilan;
}

function mutasi(
  path: string,
  body: unknown,
  ubah: Record<string, string | null> = {},
) {
  const headers: Record<string, string> = {
    origin: "https://garda.example",
    "x-forwarded-host": "garda.example",
    "x-csrf-token": "csrf-1",
    "content-type": "application/json",
  };
  for (const [k, v] of Object.entries(ubah)) {
    if (v === null) delete headers[k];
    else headers[k] = v;
  }
  return new NextRequest(`http://web:3000${path}`, {
    method: "POST",
    headers,
    body: JSON.stringify(body),
  });
}

const sah = {
  nama: "Budi Santoso",
  email: "budi@garda.test",
  peran: "OPS_MAHASISWA",
  lembaga: "FMIPA USK",
};
const dataUndangan = {
  code: "201",
  message: "Created",
  data: {
    user: {
      id: "ef45e253-50b5-3808-9710-90ed1484f293",
      nama: "Budi Santoso",
      email: "budi@garda.test",
      peran: "OPS_MAHASISWA",
      lapis: "OPERASIONAL",
      lembaga: "FMIPA USK",
      mfa_aktif: false,
      status: "MENUNGGU",
      dibuat_pada: "2026-11-06T00:00:00Z",
      wajib_ganti_password: false,
    },
    berlaku_sampai: "2026-11-09T00:00:00Z",
    email_terkirim: true,
  },
};

describe("POST /api/pengguna/undang", () => {
  it("tanpa sesi 401 dan tanpa CSRF 403, tanpa menyentuh API Go", async () => {
    const go = goMenjawab(201, dataUndangan);
    const tanpaSesi = await denganCookie(`${CSRF_COOKIE}=csrf-1`, () =>
      undang(mutasi("/api/pengguna/undang", sah), KONTEKS),
    );
    assert.equal(tanpaSesi.status, 401);
    const tanpaCsrf = await denganCookie(COOKIE_SAH, () =>
      undang(
        mutasi("/api/pengguna/undang", sah, { "x-csrf-token": "lain" }),
        KONTEKS,
      ),
    );
    assert.equal(tanpaCsrf.status, 403);
    assert.equal(go.length, 0);
  });

  it("isian wajib kosong: 400 tanpa menyentuh API Go", async () => {
    const go = goMenjawab(201, dataUndangan);
    for (const body of [
      {},
      { ...sah, nama: " " },
      { ...sah, email: undefined },
      { ...sah, peran: 3 },
      { ...sah, lembaga: "" },
      { ...sah, kabupaten_kode: 1171 },
    ]) {
      const res = await denganCookie(COOKIE_SAH, () =>
        undang(mutasi("/api/pengguna/undang", body), KONTEKS),
      );
      assert.equal(res.status, 400, JSON.stringify(body));
    }
    assert.equal(go.length, 0);
  });

  it("meneruskan hanya isian yang dikenal dengan token pengguna, jawaban 201", async () => {
    const go = goMenjawab(201, dataUndangan);

    const res = await denganCookie(COOKIE_SAH, () =>
      undang(
        mutasi("/api/pengguna/undang", {
          ...sah,
          kabupaten_kode: "1171",
          status: "AKTIF",
          password: "Rahasia#1",
          id: "2721eb4a-2e83-314e-89f0-a5bf74f2083b",
        }),
        KONTEKS,
      ),
    );

    assert.equal(res.status, 201);
    assert.deepEqual(await res.json(), dataUndangan.data);
    assert.equal(go.length, 1);
    assert.equal(go[0].url, "http://go.test/v1/users/undang");
    assert.equal(go[0].metode, "POST");
    assert.equal(go[0].auth, "Bearer jwt-akses");
    assert.deepEqual(
      go[0].body,
      { ...sah, kabupaten_kode: "1171" },
      "status, password, dan id dari klien tidak diteruskan",
    );
  });

  it("kabupaten kosong tidak diteruskan", async () => {
    const go = goMenjawab(201, dataUndangan);
    await denganCookie(COOKIE_SAH, () =>
      undang(
        mutasi("/api/pengguna/undang", { ...sah, kabupaten_kode: "" }),
        KONTEKS,
      ),
    );
    assert.deepEqual(go[0].body, sah);
  });

  it("galat API Go diteruskan dengan status dan pesannya (409 email terdaftar, 400 validasi)", async () => {
    goMenjawab(409, { code: "409", message: "Email sudah terdaftar" });
    const dup = await denganCookie(COOKIE_SAH, () =>
      undang(mutasi("/api/pengguna/undang", sah), KONTEKS),
    );
    assert.equal(dup.status, 409);
    assert.equal((await dup.json()).message, "Email sudah terdaftar");

    goMenjawab(400, { code: "400", message: "Peran tidak dikenal: DEWA." });
    const salah = await denganCookie(COOKIE_SAH, () =>
      undang(
        mutasi("/api/pengguna/undang", { ...sah, peran: "DEWA" }),
        KONTEKS,
      ),
    );
    assert.equal(salah.status, 400);
    assert.match((await salah.json()).message, /Peran tidak dikenal/);
  });
});

describe("POST /api/pengguna/{id}/undang-ulang", () => {
  it("butuh sesi dan CSRF", async () => {
    const go = goMenjawab(200, dataUndangan);
    const a = await denganCookie(`${CSRF_COOKIE}=csrf-1`, () =>
      undangUlang(mutasi("/api/pengguna/USR%2F1/undang-ulang", {}), KONTEKS),
    );
    const b = await denganCookie(COOKIE_SAH, () =>
      undangUlang(
        mutasi(
          "/api/pengguna/USR%2F1/undang-ulang",
          {},
          { origin: "https://evil.example" },
        ),
        KONTEKS,
      ),
    );
    assert.equal(a.status, 401);
    assert.equal(b.status, 403);
    assert.equal(go.length, 0);
  });

  it("memanggil API Go dengan id ter-encode", async () => {
    const go = goMenjawab(200, { ...dataUndangan, code: "200" });

    const res = await denganCookie(COOKIE_SAH, () =>
      undangUlang(mutasi("/api/pengguna/USR%2F1/undang-ulang", {}), KONTEKS),
    );

    assert.equal(res.status, 200);
    assert.equal(go[0].url, "http://go.test/v1/users/USR%2F1/undang-ulang");
    assert.equal(go[0].auth, "Bearer jwt-akses");
  });

  it("409 (bukan menunggu) dan 404 diteruskan", async () => {
    goMenjawab(409, {
      code: "409",
      message: "Akun ini tidak sedang menunggu aktivasi",
    });
    assert.equal(
      (
        await denganCookie(COOKIE_SAH, () =>
          undangUlang(mutasi("/x", {}), KONTEKS),
        )
      ).status,
      409,
    );
    goMenjawab(404, { code: "404", message: "Pengguna tidak ditemukan" });
    assert.equal(
      (
        await denganCookie(COOKIE_SAH, () =>
          undangUlang(mutasi("/x", {}), KONTEKS),
        )
      ).status,
      404,
    );
  });
});

describe("POST /api/pengguna/impor", () => {
  const ringkasan = {
    code: "200",
    message: "OK",
    data: {
      dry_run: true,
      total: 1,
      valid: 1,
      galat: 0,
      dibuat: 0,
      email_aktif: true,
      baris: [],
    },
  };

  it("butuh sesi dan CSRF", async () => {
    const go = goMenjawab(200, ringkasan);
    const body = { csv: "nama,email\n", dry_run: true };
    assert.equal(
      (
        await denganCookie(`${CSRF_COOKIE}=csrf-1`, () =>
          impor(mutasi("/api/pengguna/impor", body), KONTEKS),
        )
      ).status,
      401,
    );
    assert.equal(
      (
        await denganCookie(COOKIE_SAH, () =>
          impor(
            mutasi("/api/pengguna/impor", body, { "x-csrf-token": null }),
            KONTEKS,
          ),
        )
      ).status,
      403,
    );
    assert.equal(go.length, 0);
  });

  it("csv dan dry_run wajib dan bertipe benar: lupa dry_run tidak boleh berarti komit", async () => {
    const go = goMenjawab(200, ringkasan);
    for (const body of [
      {},
      { csv: "x" },
      { dry_run: true },
      { csv: "  ", dry_run: true },
      { csv: 5, dry_run: true },
      { csv: "x", dry_run: "false" },
      { csv: "x", dry_run: 0 },
    ]) {
      const res = await denganCookie(COOKIE_SAH, () =>
        impor(mutasi("/api/pengguna/impor", body), KONTEKS),
      );
      assert.equal(res.status, 400, JSON.stringify(body));
    }
    assert.equal(go.length, 0);
  });

  it("badan terlalu besar ditolak 413 sebelum diteruskan", async () => {
    const go = goMenjawab(200, ringkasan);
    const besar = mutasi("/api/pengguna/impor", {
      csv: "x".repeat(1_200_000),
      dry_run: true,
    });
    const res = await denganCookie(COOKIE_SAH, () => impor(besar, KONTEKS));
    assert.equal(res.status, 413);
    assert.equal(go.length, 0);
  });

  it("meneruskan csv apa adanya beserta dry_run (periksa dan komit)", async () => {
    const go = goMenjawab(200, ringkasan);
    const csv =
      "nama;email;peran;lembaga;kabupaten_kode\r\nBudi;budi@garda.test;OPS_MAHASISWA;FMIPA;1171\r\n";

    for (const dry_run of [true, false]) {
      const res = await denganCookie(COOKIE_SAH, () =>
        impor(
          mutasi("/api/pengguna/impor", { csv, dry_run, ekstra: "diabaikan" }),
          KONTEKS,
        ),
      );
      assert.equal(res.status, 200);
      assert.deepEqual(await res.json(), ringkasan.data);
    }
    assert.equal(go.length, 2);
    assert.equal(go[0].url, "http://go.test/v1/users/impor");
    assert.equal(go[0].auth, "Bearer jwt-akses");
    assert.deepEqual(go[0].body, { csv, dry_run: true });
    assert.deepEqual(go[1].body, { csv, dry_run: false });
  });

  it("galat berkas dari API Go diteruskan (400 header salah, 413 terlalu besar, 409 bentrok)", async () => {
    for (const [status, pesan] of [
      [400, "Kolom wajib tidak ada di header: peran."],
      [413, "Berkas terlalu besar."],
      [409, "Ada email yang baru saja didaftarkan pihak lain."],
    ] as const) {
      goMenjawab(status, { code: String(status), message: pesan });
      const res = await denganCookie(COOKIE_SAH, () =>
        impor(
          mutasi("/api/pengguna/impor", { csv: "x", dry_run: false }),
          KONTEKS,
        ),
      );
      assert.equal(res.status, status);
      assert.equal((await res.json()).message, pesan);
    }
  });
});

describe("GET /api/pengguna/ekspor", () => {
  const permintaan = (query = "") =>
    new NextRequest(`http://web:3000/api/pengguna/ekspor${query}`, {
      method: "GET",
    });
  const csv = "\ufeffid;nama\r\nef45e253-50b5-3808-9710-90ed1484f293;Budi\r\n";

  it("tanpa sesi 401", async () => {
    const go = goMenjawab(200, csv);
    const res = await denganCookie(`${CSRF_COOKIE}=csrf-1`, () =>
      eksporGet(permintaan(), KONTEKS),
    );
    assert.equal(res.status, 401);
    assert.equal(go.length, 0);
  });

  it("meneruskan hanya search dan ids; berkas, header unduhan, dan penanda ekspor diteruskan apa adanya", async () => {
    const go = goMenjawab(200, csv, {
      "content-type": "text/csv; charset=utf-8",
      "content-disposition": 'attachment; filename="pengguna-2026-11-06.csv"',
      "x-export-rows": "1",
      "x-export-truncated": "true",
    });

    const res = await denganCookie(COOKIE_SAH, () =>
      eksporGet(permintaan("?search=ani&ids=a,b&foo=1&limit=9999"), KONTEKS),
    );

    assert.equal(res.status, 200);
    assert.equal(go.length, 1);
    assert.equal(
      go[0].url,
      "http://go.test/v1/users/ekspor?search=ani&ids=a%2Cb",
    );
    assert.equal(go[0].metode, "GET");
    assert.equal(go[0].auth, "Bearer jwt-akses");
    // Dibandingkan sebagai byte: Response.text() membuang BOM, padahal BOM harus sampai ke Excel.
    assert.deepEqual(Buffer.from(await res.arrayBuffer()), Buffer.from(csv));
    assert.equal(res.headers.get("content-type"), "text/csv; charset=utf-8");
    assert.equal(
      res.headers.get("content-disposition"),
      'attachment; filename="pengguna-2026-11-06.csv"',
    );
    assert.equal(res.headers.get("x-export-rows"), "1");
    assert.equal(res.headers.get("x-export-truncated"), "true");
    assert.equal(
      res.headers.get("cache-control"),
      "private, no-store",
      "data pribadi tidak boleh di-cache",
    );
    assert.equal(res.headers.get("x-content-type-options"), "nosniff");
  });

  it("tanpa filter: tanpa query string", async () => {
    const go = goMenjawab(200, csv);
    await denganCookie(COOKIE_SAH, () => eksporGet(permintaan(), KONTEKS));
    assert.equal(go[0].url, "http://go.test/v1/users/ekspor");
  });

  it("galat API Go (403 tanpa izin, 400 id salah, 500 audit gagal) menjadi JSON galat, bukan berkas", async () => {
    for (const [status, pesan] of [
      [403, "Tidak memiliki izin"],
      [400, "Daftar id pengguna tidak valid."],
      [500, "Gagal mengekspor pengguna"],
    ] as const) {
      goMenjawab(status, { code: String(status), message: pesan });
      const res = await denganCookie(COOKIE_SAH, () =>
        eksporGet(permintaan("?ids=,"), KONTEKS),
      );
      assert.equal(res.status, status);
      assert.match(res.headers.get("content-type") ?? "", /application\/json/);
      assert.equal(res.headers.get("content-disposition"), null);
      assert.equal((await res.json()).message, pesan);
    }
  });

  it("badan galat yang bukan JSON tetap menjadi galat dengan status aslinya", async () => {
    goMenjawab(502, "<html>bad gateway</html>", {
      "content-type": "text/html",
    });
    const res = await denganCookie(COOKIE_SAH, () =>
      eksporGet(permintaan(), KONTEKS),
    );
    assert.equal(res.status, 502);
    assert.equal((await res.json()).error, true);
  });
});
