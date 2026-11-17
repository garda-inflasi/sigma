// Harus sebelum modul Next lain: memasang AsyncLocalStorage global yang dibutuhkan penyimpanan permintaan Next.
import { denganCookie } from "./request-scope-testing";
import assert from "node:assert/strict";
import { afterEach, before, beforeEach, describe, it } from "node:test";
import { NextRequest } from "next/server";
import { ACCESS_COOKIE, CSRF_COOKIE, REFRESH_COOKIE } from "./cookie-policy";

type Handler = (
  req: NextRequest,
  ctx: { params: Promise<{ id: string }> },
) => Promise<Response>;

let start: Handler;
let stop: Handler;
let targets: Handler;
let policyGet: Handler;
let policyPut: Handler;
let riwayat: Handler;
let akhiri: Handler;

before(async () => {
  process.env.GO_API_BASE_URL = "http://go.test/v1";
  start = (await import("@/app/api/impersonation/start/route")).POST as Handler;
  stop = (await import("@/app/api/impersonation/stop/route")).POST as Handler;
  targets = (await import("@/app/api/impersonation/targets/route"))
    .GET as Handler;
  const policy = await import("@/app/api/impersonation/policy/route");
  policyGet = policy.GET as Handler;
  policyPut = policy.PUT as Handler;
  riwayat = (await import("@/app/api/impersonation/riwayat/route"))
    .GET as Handler;
  akhiri = (await import("@/app/api/impersonation/riwayat/[id]/akhiri/route"))
    .POST as Handler;
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

const U1 = "11111111-1111-4111-8111-111111111111";
const U2 = "22222222-2222-4222-8222-222222222222";
const KONTEKS = { params: Promise.resolve({ id: U1 }) };

const COOKIE_SAH = `${ACCESS_COOKIE}=jwt-akses-admin; ${CSRF_COOKIE}=csrf-1; ${REFRESH_COOKIE}=rt-lama`;

function jwt(payload: object): string {
  const b64 = (o: object) =>
    Buffer.from(JSON.stringify(o)).toString("base64url");
  return `${b64({ alg: "HS256" })}.${b64(payload)}.tanda-tangan`;
}
const DEPAN = () => Math.floor(Date.now() / 1000) + 600;

/** Token sasaran yang diterbitkan Go: peran OPS_KOORDINATOR dengan izin tertentu. */
const TOKEN_SASARAN = jwt({
  sub: U1,
  role: "OPS_KOORDINATOR",
  permissions: ["kpi.lihat"],
  exp: DEPAN(),
  imp: {
    id: U2,
    by: { user_id: U2, name: "A", email: "a@x.id", role: "ADMIN_SISTEM" },
    started_at: 1_763_370_000,
  },
});
/** Token pelaku asli (ADMIN_SISTEM) yang diterbitkan Go saat berhenti. */
const TOKEN_ASAL = jwt({
  sub: U2,
  role: "ADMIN_SISTEM",
  permissions: ["pengguna.kelola", "impersonasi.kelola"],
  exp: DEPAN(),
});

const USER_SASARAN = {
  id: U1,
  nama: "Zulkifli",
  peran: "OPS_KOORDINATOR",
  email: "z@x.id",
};
const META = {
  aktif: true,
  sesi_id: U2,
  pelaku: {
    id: U2,
    nama: "Super Admin",
    email: "a@x.id",
    peran: "ADMIN_SISTEM",
  },
  sasaran: { id: U1, nama: "Zulkifli", peran: "OPS_KOORDINATOR" },
  mulai_pada: "2026-11-17T10:00:00Z",
  kedaluwarsa_pada: null,
  alasan: "uji",
};

const envelope = (data: unknown, status = 200) =>
  Response.json({ code: String(status), message: "OK", data }, { status });
const galatGo = (status: number, message: string, code = String(status)) =>
  Response.json({ code, message }, { status });

interface PanggilanGo {
  path: string;
  query: string;
  metode: string;
  auth: string | null;
  body: unknown;
}

function goMenjawab(rute: Record<string, () => Response>): PanggilanGo[] {
  const panggilan: PanggilanGo[] = [];
  globalThis.fetch = (async (url: RequestInfo | URL, init?: RequestInit) => {
    const u = new URL(String(url));
    const metode = init?.method ?? "GET";
    panggilan.push({
      path: u.pathname,
      query: u.search,
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

function permintaan(
  metode: "GET" | "POST" | "PUT",
  path: string,
  body?: unknown,
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
    method: metode,
    headers,
    ...(body === undefined
      ? {}
      : { body: typeof body === "string" ? body : JSON.stringify(body) }),
  });
}

const jalankan = (cookie: string, h: Handler, req: NextRequest) =>
  denganCookie(cookie, () => h(req, KONTEKS));

const cookieDipasang = (res: Response, nama: string) =>
  res.headers.getSetCookie().find((c) => c.startsWith(`${nama}=`));

// ── start ────────────────────────────────────────────────────────────

describe("POST /api/impersonation/start", () => {
  const sasaran = { target_pengguna_id: U1, alasan: "uji dasbor" };
  const jawabanGo = () =>
    envelope({
      access_token: TOKEN_SASARAN,
      refresh_token: "rt-baru-sasaran",
      user: USER_SASARAN,
      impersonasi: META,
    });

  it("tanpa sesi 401; CSRF salah, tanpa CSRF, atau Origin asing 403; semuanya tanpa menyentuh API Go", async () => {
    const go = goMenjawab({ "POST /v1/auth/impersonation/start": jawabanGo });

    const tanpaSesi = await jalankan(
      `${CSRF_COOKIE}=csrf-1`,
      start,
      permintaan("POST", "/api/impersonation/start", sasaran),
    );
    const csrfSalah = await jalankan(
      COOKIE_SAH,
      start,
      permintaan("POST", "/api/impersonation/start", sasaran, {
        "x-csrf-token": "lain",
      }),
    );
    const tanpaCsrf = await jalankan(
      COOKIE_SAH,
      start,
      permintaan("POST", "/api/impersonation/start", sasaran, {
        "x-csrf-token": null,
      }),
    );
    const asal = await jalankan(
      COOKIE_SAH,
      start,
      permintaan("POST", "/api/impersonation/start", sasaran, {
        origin: "https://evil.example",
      }),
    );

    assert.equal(tanpaSesi.status, 401);
    assert.equal(csrfSalah.status, 403);
    assert.equal(tanpaCsrf.status, 403);
    assert.equal(asal.status, 403);
    assert.equal(go.length, 0);
  });

  it("masukan cacat: 400 tanpa menyentuh API Go", async () => {
    const go = goMenjawab({ "POST /v1/auth/impersonation/start": jawabanGo });
    for (const body of [
      "{rusak",
      {},
      { target_pengguna_id: "USR/1" },
      { target_peran: "ADMIN_SISTEM" },
      { target_peran: "SUPERADMIN" },
      { target_peran: "OPS_MAHASISWA", alasan: "a".repeat(501) },
    ]) {
      const res = await jalankan(
        COOKIE_SAH,
        start,
        permintaan("POST", "/api/impersonation/start", body),
      );
      assert.equal(res.status, 400, JSON.stringify(body));
    }
    assert.equal(go.length, 0);
  });

  it("sukses: sesi baru jadi cookie, TIDAK ada token di badan, no-store, hanya isian dikenal ke Go", async () => {
    const go = goMenjawab({ "POST /v1/auth/impersonation/start": jawabanGo });

    const res = await jalankan(
      COOKIE_SAH,
      start,
      permintaan("POST", "/api/impersonation/start", {
        ...sasaran,
        dari: "/kpi",
        peran_palsu: "ADMIN_SISTEM",
      }),
    );

    assert.equal(res.status, 200);
    assert.equal(res.headers.get("cache-control"), "private, no-store");
    const teks = await res.clone().text();
    assert.doesNotMatch(teks, /access_token|refresh_token|rt-baru-sasaran/);
    assert.ok(!teks.includes(TOKEN_SASARAN), "JWT tidak boleh tampil di badan");
    assert.deepEqual(await res.json(), {
      user: USER_SASARAN,
      impersonasi: META,
      // OPS_KOORDINATOR di token baru punya kpi.lihat: halaman semula dipertahankan.
      redirectTo: "/kpi",
    });

    const akses = cookieDipasang(res, ACCESS_COOKIE);
    const refresh = cookieDipasang(res, REFRESH_COOKIE);
    assert.ok(akses?.includes(TOKEN_SASARAN), "cookie access = token sasaran");
    assert.match(akses ?? "", /HttpOnly/i);
    assert.ok(refresh?.includes("rt-baru-sasaran"), "cookie refresh diganti");
    assert.match(refresh ?? "", /HttpOnly/i);
    assert.match(refresh ?? "", /SameSite=strict/i);
    assert.ok(cookieDipasang(res, CSRF_COOKIE), "token CSRF dibuat ulang");

    assert.equal(go.length, 1);
    assert.equal(go[0].path, "/v1/auth/impersonation/start");
    assert.equal(go[0].auth, "Bearer jwt-akses-admin");
    assert.deepEqual(
      go[0].body,
      sasaran,
      "dari dan field asing tidak diteruskan",
    );
  });

  it("halaman semula yang tak boleh dibuka peran baru, atau tak aman, menjadi /", async () => {
    goMenjawab({ "POST /v1/auth/impersonation/start": jawabanGo });
    for (const dari of [
      "/pengguna",
      "/impersonasi",
      "//evil.example",
      "/api/auth/me",
    ]) {
      const res = await jalankan(
        COOKIE_SAH,
        start,
        permintaan("POST", "/api/impersonation/start", { ...sasaran, dari }),
      );
      assert.equal((await res.json()).redirectTo, "/", dari);
    }
  });

  it("kode galat Go diteruskan tanpa mengubah cookie sesi", async () => {
    for (const [status, kode] of [
      [403, "IMPERSONASI_DITOLAK"],
      [403, "IMPERSONASI_NONAKTIF"],
      [400, "ALASAN_WAJIB"],
      [409, "TARGET_AMBIGU"],
      [404, "TARGET_TIDAK_ADA"],
    ] as const) {
      goMenjawab({
        "POST /v1/auth/impersonation/start": () =>
          galatGo(status, "ditolak", kode),
      });
      const res = await jalankan(
        COOKIE_SAH,
        start,
        permintaan("POST", "/api/impersonation/start", sasaran),
      );
      assert.equal(res.status, status);
      assert.equal((await res.json()).kode, kode);
      assert.equal(cookieDipasang(res, ACCESS_COOKIE), undefined);
      assert.equal(cookieDipasang(res, REFRESH_COOKIE), undefined);
    }
  });

  it("jawaban Go tanpa token: 502, tidak ada cookie", async () => {
    goMenjawab({
      "POST /v1/auth/impersonation/start": () =>
        envelope({ user: USER_SASARAN }),
    });
    const res = await jalankan(
      COOKIE_SAH,
      start,
      permintaan("POST", "/api/impersonation/start", sasaran),
    );
    assert.equal(res.status, 502);
    assert.equal(cookieDipasang(res, ACCESS_COOKIE), undefined);
  });
});

// ── stop ─────────────────────────────────────────────────────────────

describe("POST /api/impersonation/stop", () => {
  const jawabanGo = () =>
    envelope({
      access_token: TOKEN_ASAL,
      refresh_token: "rt-baru-asal",
      user: { id: U2, nama: "Super Admin", peran: "ADMIN_SISTEM" },
    });

  it("CSRF wajib; tanpa sesi 401", async () => {
    const go = goMenjawab({ "POST /v1/auth/impersonation/stop": jawabanGo });
    const tanpaSesi = await jalankan(
      `${CSRF_COOKIE}=csrf-1`,
      stop,
      permintaan("POST", "/api/impersonation/stop", {}),
    );
    const csrfSalah = await jalankan(
      COOKIE_SAH,
      stop,
      permintaan(
        "POST",
        "/api/impersonation/stop",
        {},
        { "x-csrf-token": "lain" },
      ),
    );
    assert.equal(tanpaSesi.status, 401);
    assert.equal(csrfSalah.status, 403);
    assert.equal(go.length, 0);
  });

  it("sukses: pelaku kembali, token hanya di cookie, tidak ada badan ke Go (akun asli ditentukan server)", async () => {
    const go = goMenjawab({ "POST /v1/auth/impersonation/stop": jawabanGo });

    const res = await jalankan(
      COOKIE_SAH,
      stop,
      permintaan("POST", "/api/impersonation/stop", {
        dari: "/pengguna",
        user_id: U1,
        impersonated_by: "penyerang",
      }),
    );

    assert.equal(res.status, 200);
    assert.equal(res.headers.get("cache-control"), "private, no-store");
    const teks = await res.clone().text();
    assert.ok(!teks.includes(TOKEN_ASAL) && !teks.includes("rt-baru-asal"));
    assert.deepEqual(await res.json(), {
      user: { id: U2, nama: "Super Admin", peran: "ADMIN_SISTEM" },
      redirectTo: "/pengguna",
    });
    assert.ok(cookieDipasang(res, ACCESS_COOKIE)?.includes(TOKEN_ASAL));
    assert.ok(cookieDipasang(res, REFRESH_COOKIE)?.includes("rt-baru-asal"));
    assert.equal(go.length, 1);
    assert.equal(
      go[0].body,
      undefined,
      "tidak ada badan dari klien yang sampai ke API",
    );
    assert.equal(go[0].auth, "Bearer jwt-akses-admin");
  });

  it("badan kosong sah; badan bukan objek atau dari bukan teks 400", async () => {
    const go = goMenjawab({ "POST /v1/auth/impersonation/stop": jawabanGo });
    const kosong = await jalankan(
      COOKIE_SAH,
      stop,
      permintaan("POST", "/api/impersonation/stop"),
    );
    assert.equal(kosong.status, 200);
    assert.equal((await kosong.json()).redirectTo, "/");
    for (const body of ["[]", "5", "{rusak", { dari: 7 }]) {
      const res = await jalankan(
        COOKIE_SAH,
        stop,
        permintaan("POST", "/api/impersonation/stop", body),
      );
      assert.equal(res.status, 400, JSON.stringify(body));
    }
    assert.equal(go.length, 1);
  });

  it("400 TIDAK_ADA_IMPERSONASI dari Go diteruskan", async () => {
    goMenjawab({
      "POST /v1/auth/impersonation/stop": () =>
        galatGo(400, "Tidak sedang impersonasi", "TIDAK_ADA_IMPERSONASI"),
    });
    const res = await jalankan(
      COOKIE_SAH,
      stop,
      permintaan("POST", "/api/impersonation/stop", {}),
    );
    assert.equal(res.status, 400);
    assert.equal((await res.json()).kode, "TIDAK_ADA_IMPERSONASI");
    assert.equal(cookieDipasang(res, ACCESS_COOKIE), undefined);
  });
});

// ── targets, policy, riwayat, akhiri ─────────────────────────────────

describe("GET /api/impersonation/targets", () => {
  it("tanpa sesi 401; dengan sesi meneruskan jawaban no-store", async () => {
    const data = {
      boleh: true,
      aktif: true,
      wajib_alasan: true,
      durasi_maks_menit: 60,
      peran: [],
    };
    const go = goMenjawab({
      "GET /v1/auth/impersonation/targets": () => envelope(data),
    });

    const tanpa = await jalankan(
      `${CSRF_COOKIE}=csrf-1`,
      targets,
      permintaan("GET", "/api/impersonation/targets"),
    );
    assert.equal(tanpa.status, 401);

    const res = await jalankan(
      COOKIE_SAH,
      targets,
      permintaan("GET", "/api/impersonation/targets"),
    );
    assert.equal(res.status, 200);
    assert.equal(res.headers.get("cache-control"), "private, no-store");
    assert.deepEqual(await res.json(), data);
    assert.equal(go[0].auth, "Bearer jwt-akses-admin");
  });
});

describe("/api/impersonation/policy", () => {
  const kebijakan = {
    aktif: true,
    wajib_alasan: true,
    durasi_maks_menit: 60,
    versi: 3,
    diperbarui_pada: null,
    pelaku: [],
  };

  it("GET meneruskan kebijakan (no-store); 403 dari Go untuk non-superadmin diteruskan", async () => {
    goMenjawab({
      "GET /v1/admin/impersonation/policy": () => envelope(kebijakan),
    });
    const ok = await jalankan(
      COOKIE_SAH,
      policyGet,
      permintaan("GET", "/api/impersonation/policy"),
    );
    assert.equal(ok.status, 200);
    assert.equal(ok.headers.get("cache-control"), "private, no-store");
    assert.deepEqual(await ok.json(), kebijakan);

    goMenjawab({
      "GET /v1/admin/impersonation/policy": () => galatGo(403, "Akses ditolak"),
    });
    const tolak = await jalankan(
      COOKIE_SAH,
      policyGet,
      permintaan("GET", "/api/impersonation/policy"),
    );
    assert.equal(tolak.status, 403);
  });

  it("PUT: CSRF wajib, masukan divalidasi, hanya field dikenal diteruskan", async () => {
    const go = goMenjawab({
      "PUT /v1/admin/impersonation/policy": () => envelope(kebijakan),
    });
    const badan = {
      versi: 3,
      aktif: true,
      wajib_alasan: true,
      durasi_maks_menit: 60,
      pelaku: [
        {
          tipe: "PERAN",
          peran: "MANAJER_MUTU",
          aktif: true,
          pengguna_nama: "dibuang",
          target: [{ tipe: "PERAN", peran: "OPS_MAHASISWA" }],
        },
      ],
    };

    const csrf = await jalankan(
      COOKIE_SAH,
      policyPut,
      permintaan("PUT", "/api/impersonation/policy", badan, {
        "x-csrf-token": "lain",
      }),
    );
    assert.equal(csrf.status, 403);
    const cacat = await jalankan(
      COOKIE_SAH,
      policyPut,
      permintaan("PUT", "/api/impersonation/policy", {
        ...badan,
        durasi_maks_menit: 2,
      }),
    );
    assert.equal(cacat.status, 400);
    const admin = await jalankan(
      COOKIE_SAH,
      policyPut,
      permintaan("PUT", "/api/impersonation/policy", {
        ...badan,
        pelaku: [{ ...badan.pelaku[0], peran: "ADMIN_SISTEM" }],
      }),
    );
    assert.equal(admin.status, 400);
    assert.equal(go.length, 0);

    const ok = await jalankan(
      COOKIE_SAH,
      policyPut,
      permintaan("PUT", "/api/impersonation/policy", badan),
    );
    assert.equal(ok.status, 200);
    assert.equal(go.length, 1);
    assert.deepEqual((go[0].body as { pelaku: unknown[] }).pelaku, [
      {
        tipe: "PERAN",
        peran: "MANAJER_MUTU",
        aktif: true,
        target: [{ tipe: "PERAN", peran: "OPS_MAHASISWA" }],
      },
    ]);
  });

  it("PUT: 409 VERSI_KEBIJAKAN_BERUBAH diteruskan dengan kodenya", async () => {
    goMenjawab({
      "PUT /v1/admin/impersonation/policy": () =>
        galatGo(409, "Kebijakan sudah diubah", "VERSI_KEBIJAKAN_BERUBAH"),
    });
    const res = await jalankan(
      COOKIE_SAH,
      policyPut,
      permintaan("PUT", "/api/impersonation/policy", {
        versi: 1,
        aktif: true,
        wajib_alasan: true,
        durasi_maks_menit: null,
        pelaku: [],
      }),
    );
    assert.equal(res.status, 409);
    assert.equal((await res.json()).kode, "VERSI_KEBIJAKAN_BERUBAH");
  });
});

describe("/api/impersonation/riwayat", () => {
  it("GET meneruskan hanya query yang sah; query cacat 400 tanpa menyentuh Go", async () => {
    const go = goMenjawab({
      "GET /v1/admin/impersonation/riwayat": () => envelope({ items: [] }),
    });
    const bad = await jalankan(
      COOKIE_SAH,
      riwayat,
      permintaan("GET", "/api/impersonation/riwayat?status=lainnya"),
    );
    assert.equal(bad.status, 400);
    assert.equal(go.length, 0);

    const ok = await jalankan(
      COOKIE_SAH,
      riwayat,
      permintaan(
        "GET",
        "/api/impersonation/riwayat?status=aktif&limit=20&cursor=abc&x=1",
      ),
    );
    assert.equal(ok.status, 200);
    assert.equal(ok.headers.get("cache-control"), "private, no-store");
    assert.equal(go[0].query, "?status=aktif&limit=20&cursor=abc");
  });

  it("POST /riwayat/:id/akhiri: CSRF wajib, id harus UUID, 404 Go diteruskan", async () => {
    const go = goMenjawab({
      [`POST /v1/admin/impersonation/riwayat/${U1}/akhiri`]: () =>
        envelope({ berakhir: true }),
    });
    const csrf = await jalankan(
      COOKIE_SAH,
      akhiri,
      permintaan("POST", `/api/impersonation/riwayat/${U1}/akhiri`, undefined, {
        "x-csrf-token": "lain",
      }),
    );
    assert.equal(csrf.status, 403);
    const idBuruk = await denganCookie(COOKIE_SAH, () =>
      akhiri(permintaan("POST", "/api/impersonation/riwayat/x/akhiri"), {
        params: Promise.resolve({ id: "../../auth/logout" }),
      }),
    );
    assert.equal(idBuruk.status, 400);
    assert.equal(go.length, 0);

    const ok = await jalankan(
      COOKIE_SAH,
      akhiri,
      permintaan("POST", `/api/impersonation/riwayat/${U1}/akhiri`),
    );
    assert.equal(ok.status, 200);
    assert.deepEqual(await ok.json(), { berakhir: true });
    assert.equal(go.length, 1);

    goMenjawab({
      [`POST /v1/admin/impersonation/riwayat/${U1}/akhiri`]: () =>
        galatGo(404, "Tidak ada"),
    });
    const hilang = await jalankan(
      COOKIE_SAH,
      akhiri,
      permintaan("POST", `/api/impersonation/riwayat/${U1}/akhiri`),
    );
    assert.equal(hilang.status, 404);
  });
});
