// Harus sebelum modul Next lain: memasang AsyncLocalStorage global yang dibutuhkan penyimpanan permintaan Next.
import { denganCookie } from "./request-scope-testing";
import assert from "node:assert/strict";
import { afterEach, before, beforeEach, describe, it } from "node:test";
import { NextRequest } from "next/server";
import { ACCESS_COOKIE, CSRF_COOKIE, REFRESH_COOKIE } from "./cookie-policy";

type Handler = (req: NextRequest) => Promise<Response>;

let GET: Handler;
let PATCH: Handler;
let middleware: (typeof import("@/middleware"))["middleware"];

before(async () => {
  process.env.GO_API_BASE_URL = "http://go.test/v1";
  const rute = await import("@/app/api/auth/me/route");
  GET = rute.GET as Handler;
  PATCH = rute.PATCH as Handler;
  ({ middleware } = await import("@/middleware"));
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

// ── Bahan ────────────────────────────────────────────────────────────

/** JWT tanpa tanda tangan sah: route hanya membaca klaim `name` (API Go yang memverifikasi). */
function jwt(payload: object): string {
  const b64 = (o: object) =>
    Buffer.from(JSON.stringify(o)).toString("base64url");
  return `${b64({ alg: "HS256" })}.${b64(payload)}.tanda-tangan`;
}

const DEPAN = () => Math.floor(Date.now() / 1000) + 600;
const tokenAkses = (name: string | undefined, ekstra: object = {}) =>
  jwt({ role: "OPS_KOORDINATOR", name, exp: DEPAN(), ...ekstra });

const cookieSesi = (token: string, refresh: string | null = "rt-lama") =>
  [
    `${ACCESS_COOKIE}=${token}`,
    `${CSRF_COOKIE}=csrf-1`,
    ...(refresh ? [`${REFRESH_COOKIE}=${refresh}`] : []),
  ].join("; ");

const USER = {
  id: "u-1",
  nama: "Budi Baru",
  email: "budi@garda.test",
  peran: "OPS_KOORDINATOR",
  lapis: "OPERASIONAL",
  lembaga: "FMIPA USK",
  kabupaten_kode: "1106",
  mfa_aktif: false,
  status: "AKTIF",
  dibuat_pada: "2026-10-01T08:00:00Z",
  wajib_ganti_password: false,
  mfa_setup_diwajibkan: false,
  mfa_wajib: false,
  telepon: "+6281234567890",
};

const envelope = (data: unknown, status = 200) =>
  Response.json({ code: String(status), message: "OK", data }, { status });
const galatGo = (status: number, message: string) =>
  Response.json({ code: String(status), message }, { status });

interface PanggilanGo {
  path: string;
  metode: string;
  auth: string | null;
  body: unknown;
}

/** Memasang API Go palsu per "METODE /path" dan merekam semua panggilan yang sampai. */
function goMenjawab(rute: Record<string, () => Response>): PanggilanGo[] {
  const panggilan: PanggilanGo[] = [];
  globalThis.fetch = (async (url: RequestInfo | URL, init?: RequestInit) => {
    const u = new URL(String(url));
    const metode = init?.method ?? "GET";
    panggilan.push({
      path: u.pathname,
      metode,
      auth: new Headers(init?.headers).get("authorization"),
      body: typeof init?.body === "string" ? JSON.parse(init.body) : undefined,
    });
    const penangan = rute[`${metode} ${u.pathname}`];
    return penangan
      ? penangan()
      : galatGo(500, `Go palsu: ${metode} ${u.pathname} tidak diatur`);
  }) as typeof fetch;
  return panggilan;
}

/** PATCH dari halaman sendiri; `ubah` menimpa header bawaan (null menghapusnya). */
function patch(
  body: unknown,
  cookie: string,
  ubah: Record<string, string | null> = {},
) {
  const headers: Record<string, string> = {
    origin: "https://garda.example",
    "x-forwarded-host": "garda.example",
    "x-csrf-token": "csrf-1",
    "content-type": "application/json",
    cookie,
  };
  for (const [k, v] of Object.entries(ubah)) {
    if (v === null) delete headers[k];
    else headers[k] = v;
  }
  return new NextRequest("http://web:3000/api/auth/me", {
    method: "PATCH",
    headers,
    body: typeof body === "string" ? body : JSON.stringify(body),
  });
}

const jalankan = (cookie: string, req: NextRequest, h: Handler = PATCH) =>
  denganCookie(cookie, () => h(req));

const setCookie = (res: Response) => res.headers.getSetCookie();
const cookieDipasang = (res: Response, nama: string) =>
  setCookie(res).find((c) => c.startsWith(`${nama}=`));

// ── PATCH ────────────────────────────────────────────────────────────

describe("PATCH /api/auth/me: perlindungan", () => {
  it("token CSRF salah, tanpa cookie CSRF, atau Origin asing: 403 tanpa menyentuh API Go", async () => {
    const go = goMenjawab({ "PATCH /v1/auth/me": () => envelope(USER) });
    const cookie = cookieSesi(tokenAkses("Budi Lama"));

    const csrfSalah = await jalankan(
      cookie,
      patch({ nama: "Budi Baru" }, cookie, { "x-csrf-token": "lain" }),
    );
    const tanpaCookieCsrf = await jalankan(
      `${ACCESS_COOKIE}=${tokenAkses("Budi Lama")}`,
      patch({ nama: "Budi Baru" }, cookie),
    );
    const asal = await jalankan(
      cookie,
      patch({ nama: "Budi Baru" }, cookie, { origin: "https://evil.example" }),
    );

    for (const res of [csrfSalah, tanpaCookieCsrf, asal]) {
      assert.equal(res.status, 403);
      assert.equal((await res.json()).message, "CSRF token tidak valid.");
    }
    assert.equal(go.length, 0);
  });

  it("tanpa cookie access: 401 tanpa menyentuh API Go", async () => {
    const go = goMenjawab({ "PATCH /v1/auth/me": () => envelope(USER) });
    const cookie = `${CSRF_COOKIE}=csrf-1`;

    const res = await jalankan(cookie, patch({ nama: "Budi Baru" }, cookie));

    assert.equal(res.status, 401);
    assert.equal((await res.json()).message, "Tidak terautentikasi.");
    assert.equal(go.length, 0);
  });
});

describe("PATCH /api/auth/me: isian yang diteruskan", () => {
  it("hanya nama dan telepon yang sampai ke API Go, dengan token pengguna; jawabannya { user }", async () => {
    const go = goMenjawab({ "PATCH /v1/auth/me": () => envelope(USER) });
    const token = tokenAkses("Budi Baru");
    const cookie = cookieSesi(token, null);

    const res = await jalankan(
      cookie,
      patch(
        {
          nama: "Budi Baru",
          telepon: "+6281234567890",
          peran: "ADMIN_SISTEM",
          email: "lain@garda.test",
          status: "NONAKTIF",
          lembaga: "Lembaga Lain",
          lapis: "SISTEM",
          kabupaten_kode: "1101",
          id: "u-2",
          wajib_ganti_password: true,
        },
        cookie,
      ),
    );

    assert.equal(res.status, 200);
    assert.deepEqual(await res.json(), { user: USER });
    assert.equal(go.length, 1);
    assert.equal(go[0].path, "/v1/auth/me");
    assert.equal(go[0].metode, "PATCH");
    assert.equal(go[0].auth, `Bearer ${token}`, "token dari cookie");
    assert.deepEqual(go[0].body, {
      nama: "Budi Baru",
      telepon: "+6281234567890",
    });
  });

  it("telepon kosong (hapus nomor) diteruskan; null dan field yang tidak ada tidak diteruskan", async () => {
    const go = goMenjawab({ "PATCH /v1/auth/me": () => envelope(USER) });
    const cookie = cookieSesi(tokenAkses("Budi Baru"), null);

    await jalankan(cookie, patch({ nama: null, telepon: "" }, cookie));
    await jalankan(cookie, patch({ nama: "Budi Baru", telepon: null }, cookie));
    await jalankan(cookie, patch({ peran: "ADMIN_SISTEM" }, cookie));

    assert.deepEqual(
      go.map((p) => p.body),
      [{ telepon: "" }, { nama: "Budi Baru" }, {}],
    );
  });

  it("jawaban memuat Cache-Control private dan header keamanan (data pribadi)", async () => {
    goMenjawab({ "PATCH /v1/auth/me": () => envelope(USER) });
    const cookie = cookieSesi(tokenAkses("Budi Baru"), null);

    const res = await jalankan(cookie, patch({ nama: "Budi Baru" }, cookie));

    assert.equal(res.headers.get("cache-control"), "private, no-store");
    assert.equal(res.headers.get("x-content-type-options"), "nosniff");
  });

  it("badan bukan JSON, bukan objek, atau isian bukan teks: 400 tanpa menyentuh API Go", async () => {
    const go = goMenjawab({ "PATCH /v1/auth/me": () => envelope(USER) });
    const cookie = cookieSesi(tokenAkses("Budi Lama"));

    for (const body of [
      "bukan json",
      "",
      "null",
      "42",
      '"teks"',
      "[]",
      '[{"nama":"Budi"}]',
      { nama: 5 },
      { nama: { a: 1 } },
      { telepon: 628123 },
      { telepon: ["+6281234567890"] },
      { nama: "Budi Baru", telepon: false },
    ]) {
      const res = await jalankan(cookie, patch(body, cookie));
      assert.equal(res.status, 400, JSON.stringify(body));
      assert.equal((await res.json()).error, true);
    }
    assert.equal(go.length, 0);
  });
});

describe("PATCH /api/auth/me: cookie sesi saat nama berubah", () => {
  it("klaim name berbeda dari nama baru: refresh token ditukar dan cookie access serta refresh baru dipasang", async () => {
    const go = goMenjawab({
      "PATCH /v1/auth/me": () => envelope(USER),
      "POST /v1/auth/refresh": () =>
        envelope({ access_token: "at-baru", refresh_token: "rt-baru" }),
    });
    const cookie = cookieSesi(tokenAkses("Budi Lama"));

    const res = await jalankan(cookie, patch({ nama: "Budi Baru" }, cookie));

    assert.equal(res.status, 200);
    assert.deepEqual(await res.json(), { user: USER });
    assert.deepEqual(
      go.map((p) => `${p.metode} ${p.path}`),
      ["PATCH /v1/auth/me", "POST /v1/auth/refresh"],
    );
    assert.deepEqual(go[1].body, { refresh_token: "rt-lama" });

    const akses = cookieDipasang(res, ACCESS_COOKIE);
    const refresh = cookieDipasang(res, REFRESH_COOKIE);
    assert.ok(akses?.startsWith(`${ACCESS_COOKIE}=at-baru;`), akses);
    assert.match(akses!, /HttpOnly/i);
    assert.match(akses!, /Path=\/(;|$)/);
    assert.ok(refresh?.startsWith(`${REFRESH_COOKIE}=rt-baru;`), refresh);
    assert.match(refresh!, /HttpOnly/i);
    assert.match(refresh!, /Path=\/api\/auth/);
    assert.match(refresh!, /SameSite=strict/i);
    assert.equal(
      cookieDipasang(res, CSRF_COOKIE),
      undefined,
      "token CSRF yang ada tidak diganti",
    );
  });

  it("token tanpa klaim name juga disegarkan (sesi tanpa nama dipulihkan)", async () => {
    const go = goMenjawab({
      "PATCH /v1/auth/me": () => envelope(USER),
      "POST /v1/auth/refresh": () =>
        envelope({ access_token: "at-baru", refresh_token: "rt-baru" }),
    });
    const cookie = cookieSesi(tokenAkses(undefined));

    const res = await jalankan(cookie, patch({ telepon: "0812" }, cookie));

    assert.equal(go.length, 2);
    assert.ok(cookieDipasang(res, ACCESS_COOKIE)?.includes("at-baru"));
  });

  it("nama tidak berubah (klaim sudah sama): tidak ada penukaran token dan tidak ada cookie token baru", async () => {
    const go = goMenjawab({
      "PATCH /v1/auth/me": () => envelope(USER),
      "POST /v1/auth/refresh": () =>
        envelope({ access_token: "at-baru", refresh_token: "rt-baru" }),
    });
    const cookie = cookieSesi(tokenAkses("Budi Baru"));

    const res = await jalankan(
      cookie,
      patch({ telepon: "081234567890" }, cookie),
    );

    assert.equal(res.status, 200);
    assert.deepEqual(
      go.map((p) => `${p.metode} ${p.path}`),
      ["PATCH /v1/auth/me"],
    );
    assert.deepEqual(setCookie(res), []);
  });

  it("pemutaran ulang dalam jendela toleransi (tanpa refresh token baru): hanya cookie access yang diganti", async () => {
    goMenjawab({
      "PATCH /v1/auth/me": () => envelope(USER),
      "POST /v1/auth/refresh": () => envelope({ access_token: "at-baru" }),
    });
    const cookie = cookieSesi(tokenAkses("Budi Lama"));

    const res = await jalankan(cookie, patch({ nama: "Budi Baru" }, cookie));

    assert.ok(cookieDipasang(res, ACCESS_COOKIE)?.includes("at-baru"));
    assert.equal(cookieDipasang(res, REFRESH_COOKIE), undefined);
  });

  it("tanpa cookie refresh: tidak ada penukaran, jawaban tetap 200", async () => {
    const go = goMenjawab({
      "PATCH /v1/auth/me": () => envelope(USER),
      "POST /v1/auth/refresh": () =>
        envelope({ access_token: "at-baru", refresh_token: "rt-baru" }),
    });
    const cookie = cookieSesi(tokenAkses("Budi Lama"), null);

    const res = await jalankan(cookie, patch({ nama: "Budi Baru" }, cookie));

    assert.equal(res.status, 200);
    assert.equal(go.length, 1);
    assert.deepEqual(setCookie(res), []);
  });

  it("penukaran ditolak (401) atau gagal (500, jaringan): tetap 200 dan sesi tidak dihapus", async () => {
    const cookie = cookieSesi(tokenAkses("Budi Lama"));
    const kegagalan: (() => Response)[] = [
      () => galatGo(401, "Token tidak valid"),
      () => galatGo(500, "Gagal"),
      () => envelope({ access_token: "" }),
    ];
    for (const gagal of kegagalan) {
      goMenjawab({
        "PATCH /v1/auth/me": () => envelope(USER),
        "POST /v1/auth/refresh": gagal,
      });
      const res = await jalankan(cookie, patch({ nama: "Budi Baru" }, cookie));

      assert.equal(res.status, 200);
      assert.deepEqual(await res.json(), { user: USER });
      assert.deepEqual(
        setCookie(res),
        [],
        "tidak ada cookie yang dipasang atau dihapus",
      );
    }

    globalThis.fetch = (async (url: RequestInfo | URL, init?: RequestInit) => {
      if (String(url).endsWith("/auth/refresh"))
        throw new TypeError("fetch failed");
      void init;
      return envelope(USER);
    }) as typeof fetch;
    const res = await jalankan(cookie, patch({ nama: "Budi Baru" }, cookie));
    assert.equal(res.status, 200);
    assert.deepEqual(setCookie(res), []);
  });
});

describe("PATCH /api/auth/me: galat dari API Go", () => {
  it("400 validasi diteruskan dengan status dan pesannya", async () => {
    const cookie = cookieSesi(tokenAkses("Budi Lama"));
    for (const pesan of [
      "Nama minimal 3 karakter",
      "Nama maksimal 100 karakter",
      "Nomor telepon tidak valid",
      "Tidak ada perubahan",
    ]) {
      goMenjawab({ "PATCH /v1/auth/me": () => galatGo(400, pesan) });
      const res = await jalankan(cookie, patch({ nama: "Bu" }, cookie));

      assert.equal(res.status, 400, pesan);
      assert.deepEqual(await res.json(), { error: true, message: pesan });
    }
  });

  it("galat 400 tidak memicu penukaran token", async () => {
    const go = goMenjawab({
      "PATCH /v1/auth/me": () => galatGo(400, "Nama minimal 3 karakter"),
      "POST /v1/auth/refresh": () => envelope({ access_token: "at-baru" }),
    });
    const cookie = cookieSesi(tokenAkses("Budi Lama"));

    const res = await jalankan(cookie, patch({ nama: "Bu" }, cookie));

    assert.equal(res.status, 400);
    assert.equal(go.length, 1);
    assert.deepEqual(setCookie(res), []);
  });

  it("401, 404, 429, dan 500 diteruskan apa adanya; kode mesin stabil ikut diteruskan", async () => {
    const cookie = cookieSesi(tokenAkses("Budi Lama"));
    const kasus: [number, string, string | undefined][] = [
      [401, "Token tidak valid", undefined],
      [404, "Pengguna tidak ditemukan", undefined],
      [
        403,
        "Anda harus mengganti kata sandi sebelum melanjutkan.",
        "PASSWORD_CHANGE_REQUIRED",
      ],
      [500, "Gagal memperbarui profil", undefined],
    ];
    for (const [status, message, code] of kasus) {
      goMenjawab({
        "PATCH /v1/auth/me": () =>
          Response.json({ code: code ?? String(status), message }, { status }),
      });
      const res = await jalankan(cookie, patch({ nama: "Budi Baru" }, cookie));

      assert.equal(res.status, status);
      const badan = await res.json();
      assert.equal(badan.message, message);
      assert.equal(badan.kode, code);
    }
  });

  it("API Go tidak terjangkau: 500 tanpa membocorkan detail", async () => {
    globalThis.fetch = (async () => {
      throw new TypeError("fetch failed: ECONNREFUSED 10.0.0.5:8080");
    }) as typeof fetch;
    const cookie = cookieSesi(tokenAkses("Budi Lama"));

    const res = await jalankan(cookie, patch({ nama: "Budi Baru" }, cookie));

    assert.equal(res.status, 500);
    assert.equal((await res.json()).message, "Kesalahan server internal.");
  });
});

// ── GET (tidak berubah) ──────────────────────────────────────────────

describe("GET /api/auth/me", () => {
  const minta = (cookie: string) =>
    new NextRequest("http://web:3000/api/auth/me", { headers: { cookie } });

  it("tanpa sesi 401; dengan sesi meneruskan profil sebagai { user }", async () => {
    const go = goMenjawab({ "GET /v1/auth/me": () => envelope(USER) });

    const tanpa = await jalankan("", minta(""), GET);
    assert.equal(tanpa.status, 401);

    const token = tokenAkses("Budi Baru");
    const cookie = cookieSesi(token);
    const res = await jalankan(cookie, minta(cookie), GET);
    assert.equal(res.status, 200);
    assert.deepEqual(await res.json(), { user: USER });
    assert.equal(go.length, 1);
    assert.equal(go[0].metode, "GET");
    assert.equal(go[0].auth, `Bearer ${token}`);
  });
});

// ── Edge ─────────────────────────────────────────────────────────────

describe("middleware: PATCH /api/auth/me", () => {
  const minta = (metode: string, token?: string) =>
    new NextRequest("http://localhost:3000/api/auth/me", {
      method: metode,
      headers: token ? { cookie: `${ACCESS_COOKIE}=${token}` } : {},
    });
  const lolos = (res: Response) => res.headers.get("x-middleware-next") === "1";

  it("pengguna yang masuk lolos untuk GET dan PATCH, apa pun perannya", () => {
    for (const metode of ["GET", "PATCH"]) {
      assert.ok(lolos(middleware(minta(metode, tokenAkses("Budi")))), metode);
      assert.ok(
        lolos(
          middleware(
            minta(
              metode,
              jwt({ role: "OPS_MAHASISWA", permissions: [], exp: DEPAN() }),
            ),
          ),
        ),
        `${metode} tanpa izin apa pun`,
      );
    }
  });

  it("akun wajib ganti kata sandi atau wajib setup MFA tetap bisa membaca dan mengubah profil", () => {
    for (const ekstra of [
      { wajib_ganti_password: true },
      { wajib_setup_mfa: true },
    ]) {
      for (const metode of ["GET", "PATCH"]) {
        assert.ok(
          lolos(middleware(minta(metode, tokenAkses("Budi", ekstra)))),
          `${metode} ${JSON.stringify(ekstra)}`,
        );
      }
    }
  });

  it("tanpa sesi: 401 JSON, bukan lolos", async () => {
    const res = middleware(minta("PATCH"));
    assert.equal(res.status, 401);
    assert.equal(lolos(res), false);
  });
});
