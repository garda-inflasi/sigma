import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { NextRequest } from "next/server";
import {
  ACCESS_COOKIE,
  REFRESH_COOKIE,
  RESUME_COOKIE,
} from "@/lib/bff/cookie-policy";
import { ROLE_PERMISSIONS } from "@/lib/rbac/permissions";
import { ROLES, type Role } from "@/lib/rbac/roles";
import { config, middleware } from "./middleware";

// Keputusan akses diuji murni di lib/rbac/edge-guard.test.ts. Berkas ini menguji terjemahannya menjadi respons
// Next: status dan Location, cookie yang dihapus, JSON galat, serta header keamanan pada SEMUA jawaban.

const BASE = "http://localhost:3000";

function jwt(payload: object): string {
  const b64 = (o: object) =>
    Buffer.from(JSON.stringify(o)).toString("base64url");
  return `${b64({ alg: "HS256" })}.${b64(payload)}.tanda-tangan`;
}

function token(
  role: Role,
  extra: object = {},
  exp = Math.floor(Date.now() / 1000) + 600,
) {
  return jwt({ role, permissions: ROLE_PERMISSIONS[role], exp, ...extra });
}

function minta(
  path: string,
  cookies: Record<string, string> = {},
): NextRequest {
  const cookie = Object.entries(cookies)
    .map(([k, v]) => `${k}=${v}`)
    .join("; ");
  return new NextRequest(`${BASE}${path}`, {
    headers: cookie ? { cookie } : {},
  });
}

const lokasi = (res: Response) => res.headers.get("location");

function harapHeaderKeamanan(res: Response) {
  assert.equal(res.headers.get("X-Content-Type-Options"), "nosniff");
  assert.equal(res.headers.get("X-Frame-Options"), "DENY");
  assert.equal(res.headers.get("X-XSS-Protection"), "0");
  assert.equal(
    res.headers.get("Referrer-Policy"),
    "strict-origin-when-cross-origin",
  );
  assert.equal(
    res.headers.get("Strict-Transport-Security"),
    null,
    "HSTS milik Caddy; dari edge ikut ke HTTP dan merusak http://localhost",
  );
}

/** Baris Set-Cookie yang menghapus `nama` (nilai kosong dan sudah kedaluwarsa). */
function cookieDihapus(res: Response, nama: string): string | undefined {
  return res.headers.getSetCookie().find((c) => {
    if (!c.startsWith(`${nama}=`)) return false;
    const kosong = c.startsWith(`${nama}=;`);
    const kedaluwarsa = /Max-Age=0|Expires=Thu, 01 Jan 1970/i.test(c);
    return kosong && kedaluwarsa;
  });
}

describe("middleware: lolos", () => {
  it("pengguna sah ke halaman yang diizinkan perannya: lanjut, dengan header keamanan", () => {
    const res = middleware(
      minta("/pengguna", { [ACCESS_COOKIE]: token("ADMIN_SISTEM") }),
    );
    assert.equal(res.headers.get("x-middleware-next"), "1");
    assert.equal(res.status, 200);
    harapHeaderKeamanan(res);
  });

  it("rute publik tanpa token: lanjut, tetap berheader keamanan", () => {
    for (const path of [
      "/masuk",
      "/lupa-password",
      "/api/auth/login",
      "/healthz",
    ]) {
      const res = middleware(minta(path));
      assert.equal(res.headers.get("x-middleware-next"), "1", path);
      harapHeaderKeamanan(res);
    }
  });

  it("/api/auth/logout dan /api/auth/refresh terjangkau walau access token sudah habis", () => {
    const kedaluwarsa = token("ADMIN_SISTEM", {}, 1_000);
    for (const path of ["/api/auth/logout", "/api/auth/refresh"]) {
      const res = middleware(minta(path, { [ACCESS_COOKIE]: kedaluwarsa }));
      assert.equal(res.headers.get("x-middleware-next"), "1", path);
      assert.equal(cookieDihapus(res, ACCESS_COOKIE), undefined, path);
    }
  });
});

describe("middleware: tanpa sesi yang sah", () => {
  it("halaman tanpa cookie: ke pemulihan sesi membawa tujuan semula (path dan query)", () => {
    const res = middleware(minta("/pengguna?cari=ani&_rsc=x1"));
    assert.equal(res.status, 307);
    const tujuan = new URL(lokasi(res)!);
    assert.equal(tujuan.origin, BASE);
    assert.equal(tujuan.pathname, "/api/auth/refresh");
    assert.equal(tujuan.searchParams.get("lanjut"), "/pengguna?cari=ani");
    harapHeaderKeamanan(res);
  });

  it("API tanpa cookie: 401 Unauthorized berformat JSON, bukan pengalihan", async () => {
    const res = middleware(minta("/api/pengguna"));
    assert.equal(res.status, 401);
    assert.equal(lokasi(res), null);
    assert.deepEqual(await res.json(), {
      error: true,
      message: "Unauthorized",
    });
    harapHeaderKeamanan(res);
  });

  it("API dengan refresh token saja: 'Token expired' agar klien memulihkan sesi", async () => {
    const res = middleware(minta("/api/pengguna", { [REFRESH_COOKIE]: "rt" }));
    assert.equal(res.status, 401);
    assert.deepEqual(await res.json(), {
      error: true,
      message: "Token expired",
    });
  });

  it("access token kedaluwarsa: cookie-nya dihapus pada pengalihan maupun JSON", async () => {
    const kedaluwarsa = token("ADMIN_SISTEM", {}, 1_000);

    const halaman = middleware(
      minta("/pengguna", { [ACCESS_COOKIE]: kedaluwarsa }),
    );
    assert.equal(halaman.status, 307);
    assert.ok(
      cookieDihapus(halaman, ACCESS_COOKIE),
      "cookie access dihapus di pengalihan",
    );

    const api = middleware(
      minta("/api/pengguna", { [ACCESS_COOKIE]: kedaluwarsa }),
    );
    assert.equal(api.status, 401);
    assert.ok(
      cookieDihapus(api, ACCESS_COOKIE),
      "cookie access dihapus di JSON 401",
    );
    assert.equal((await api.json()).message, "Token expired");
  });

  it("token rusak diperlakukan seperti tidak ada dan cookie-nya dibuang", () => {
    const res = middleware(
      minta("/pengguna", { [ACCESS_COOKIE]: "bukan.jwt" }),
    );
    assert.equal(res.status, 307);
    assert.ok(cookieDihapus(res, ACCESS_COOKIE));
  });

  it("pengaman loop: sesi baru dipulihkan tetapi token tetap tak terpakai → /masuk dan penanda dihapus", () => {
    const res = middleware(minta("/pengguna", { [RESUME_COOKIE]: "1" }));
    assert.equal(res.status, 307);
    const tujuan = new URL(lokasi(res)!);
    assert.equal(tujuan.pathname, "/masuk");
    assert.equal(tujuan.searchParams.get("lanjut"), "/pengguna");
    assert.ok(cookieDihapus(res, RESUME_COOKIE), "garda_pulih dihapus");
  });

  it("penanda pemulihan tidak dihapus bila sesi sah (tidak ada yang perlu diputus)", () => {
    const res = middleware(
      minta("/pengguna", {
        [ACCESS_COOKIE]: token("ADMIN_SISTEM"),
        [RESUME_COOKIE]: "1",
      }),
    );
    assert.equal(res.headers.get("x-middleware-next"), "1");
  });
});

describe("middleware: kewajiban sesudah masuk", () => {
  it("wajib ganti kata sandi: halaman ke /ganti-password, API 403 dengan kode", async () => {
    const wajib = token("ADMIN_SISTEM", { wajib_ganti_password: true });

    const halaman = middleware(minta("/pengguna", { [ACCESS_COOKIE]: wajib }));
    assert.equal(halaman.status, 307);
    const tujuan = new URL(lokasi(halaman)!);
    assert.equal(tujuan.pathname, "/ganti-password");
    assert.equal(tujuan.searchParams.get("lanjut"), "/pengguna");

    const api = middleware(minta("/api/pengguna", { [ACCESS_COOKIE]: wajib }));
    assert.equal(api.status, 403);
    assert.equal((await api.json()).code, "PASSWORD_CHANGE_REQUIRED");
    harapHeaderKeamanan(api);

    const ganti = middleware(
      minta("/ganti-password", { [ACCESS_COOKIE]: wajib }),
    );
    assert.equal(ganti.headers.get("x-middleware-next"), "1");
  });

  it("wajib setup MFA: halaman ke /mfa, API 403 MFA_SETUP_REQUIRED", async () => {
    const wajib = token("ADMIN_SISTEM", { wajib_setup_mfa: true });

    const halaman = middleware(minta("/pengguna", { [ACCESS_COOKIE]: wajib }));
    assert.equal(new URL(lokasi(halaman)!).pathname, "/mfa");

    const api = middleware(minta("/api/pengguna", { [ACCESS_COOKIE]: wajib }));
    assert.equal(api.status, 403);
    assert.equal((await api.json()).code, "MFA_SETUP_REQUIRED");
  });
});

describe("middleware: izin rute", () => {
  it("penolakan pertama menyegarkan token dulu (izin bisa saja baru diberikan), lalu ke /akses-ditolak membawa asal rute", () => {
    const pertama = middleware(
      minta("/pengguna", { [ACCESS_COOKIE]: token("OPS_MAHASISWA") }),
    );
    assert.equal(pertama.status, 307);
    const pemulihan = new URL(lokasi(pertama)!);
    assert.equal(pemulihan.pathname, "/api/auth/refresh");
    assert.equal(pemulihan.searchParams.get("lanjut"), "/pengguna");

    const kedua = middleware(
      minta("/pengguna", { [ACCESS_COOKIE]: token("OPS_MAHASISWA"), [RESUME_COOKIE]: "1" }),
    );
    assert.equal(kedua.status, 307);
    const tujuan = new URL(lokasi(kedua)!);
    assert.equal(tujuan.pathname, "/akses-ditolak");
    assert.equal(tujuan.searchParams.get("dari"), "/pengguna");
  });

  it("endpoint BFF /api/* tidak terkena izin halaman (Go yang memutuskan)", () => {
    const res = middleware(
      minta("/api/auth/me", { [ACCESS_COOKIE]: token("OPS_MAHASISWA") }),
    );
    assert.equal(res.headers.get("x-middleware-next"), "1");
  });

  it("akar situs ke beranda sesuai peran", () => {
    for (const role of [
      "ADMIN_SISTEM",
      "OPS_MAHASISWA",
      "STRATEGIS_BI",
    ] as Role[]) {
      const res = middleware(minta("/", { [ACCESS_COOKIE]: token(role) }));
      assert.equal(res.status, 307, role);
      assert.equal(new URL(lokasi(res)!).pathname, ROLES[role].beranda, role);
    }
  });

  it("alamat pengalihan dibangun dari origin permintaan, bukan nilai tetap", () => {
    const res = middleware(
      new NextRequest("https://garda.example/pengguna", {
        headers: { cookie: `${ACCESS_COOKIE}=${token("OPS_MAHASISWA")}` },
      }),
    );
    assert.equal(new URL(lokasi(res)!).origin, "https://garda.example");
  });
});

describe("config.matcher", () => {
  // Pola Next (path-to-regexp) yang dipakai di sini juga sah sebagai RegExp JavaScript bila dijangkarkan.
  const [pola] = config.matcher;
  const cocok = (path: string) => new RegExp(`^${pola}$`).test(path);

  it("aset statis dan berkas publik tidak melewati middleware", () => {
    for (const path of [
      "/_next/static/chunks/app.js",
      "/_next/image",
      "/favicon.ico",
      "/sw.js",
      "/manifest.webmanifest",
      "/icons/icon-192.png",
    ]) {
      assert.equal(cocok(path), false, path);
    }
  });

  it("halaman, API BFF, dan rute lain melewatinya (termasuk yang mirip aset)", () => {
    for (const path of [
      "/",
      "/pengguna",
      "/masuk",
      "/api/auth/login",
      "/api/pengguna/u1",
      "/lapangan/input",
      "/_next-palsu/x",
      "/icons-palsu",
    ]) {
      assert.equal(cocok(path), true, path);
    }
  });
});
