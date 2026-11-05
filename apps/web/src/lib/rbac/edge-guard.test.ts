import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { ROLE_PERMISSIONS } from "./permissions";
import { ROLE_IDS, ROLES, type Role } from "./roles";
import { ROUTE_ACCESS, canAccessRoute } from "./routes";
import { decideEdge, isPublicRoute } from "./edge-guard";
import { decodeJwtPayload, readEdgeClaims } from "@/lib/auth/edge-claims";

const NOW = 1_800_000_000;

function jwt(payload: object): string {
  const b64 = (o: object) =>
    Buffer.from(JSON.stringify(o)).toString("base64url");
  return `${b64({ alg: "HS256" })}.${b64(payload)}.tanda-tangan`;
}

function tokenFor(role: Role, extra: object = {}): string {
  return jwt({
    role,
    permissions: ROLE_PERMISSIONS[role],
    exp: NOW + 600,
    ...extra,
  });
}

const decide = (
  pathname: string,
  accessToken?: string,
  refreshToken?: string,
) => decideEdge({ pathname, accessToken, refreshToken, nowSeconds: NOW });

describe("klaim edge", () => {
  it("membaca role, permissions, exp, wajib_ganti_password", () => {
    const c = readEdgeClaims(
      jwt({
        role: "ADMIN_SISTEM",
        permissions: ["audit.lihat"],
        exp: 5,
        wajib_ganti_password: true,
      }),
    );
    assert.deepEqual(c, {
      role: "ADMIN_SISTEM",
      permissions: ["audit.lihat"],
      exp: 5,
      wajibGantiPassword: true,
    });
  });

  it("menolak token rusak atau peran tidak dikenal", () => {
    assert.equal(readEdgeClaims("bukan-jwt"), null);
    assert.equal(readEdgeClaims("a.%%%.c"), null);
    assert.equal(readEdgeClaims(jwt({ role: "TUKANG" })), null);
    assert.equal(readEdgeClaims(undefined), null);
    assert.equal(
      decodeJwtPayload(`x.${Buffer.from("[1]").toString("base64url")}.z`),
      null,
    );
  });

  it("izin yang bukan array string dianggap tidak ada", () => {
    assert.equal(
      readEdgeClaims(jwt({ role: "ADMIN_SISTEM", permissions: [1, 2] }))
        ?.permissions,
      null,
    );
  });

  it("mendekode muatan UTF-8", () => {
    assert.equal(
      decodeJwtPayload(jwt({ name: "Café Ünïcode ✓" }))?.name,
      "Café Ünïcode ✓",
    );
  });
});

describe("rute publik dan alur masuk tidak terblokir", () => {
  for (const path of [
    "/masuk",
    "/lupa-password",
    "/reset-password",
    "/akses-ditolak",
    "/offline",
    "/healthz",
    "/api/auth/login",
    "/api/auth/refresh",
    "/_next/static/chunk.js",
  ]) {
    it(`${path} lolos tanpa token`, () => {
      assert.deepEqual(decide(path), { kind: "next" });
    });
  }

  it("batas prefiks: /masuk-palsu bukan rute publik", () => {
    assert.equal(isPublicRoute("/masuk-palsu"), false);
  });

  it("/ganti-password terbuka bagi peran apa pun yang sudah masuk, termasuk yang wajib ganti sandi", () => {
    for (const role of ROLE_IDS) {
      assert.deepEqual(decide("/ganti-password", tokenFor(role)), {
        kind: "next",
      });
      assert.deepEqual(
        decide(
          "/ganti-password",
          tokenFor(role, { wajib_ganti_password: true }),
        ),
        { kind: "next" },
      );
    }
  });

  it("/api/auth/me dan /api/auth/change-password tetap terbuka bagi akun wajib ganti sandi", () => {
    const t = tokenFor("ADMIN_SISTEM", { wajib_ganti_password: true });
    assert.deepEqual(decide("/api/auth/change-password", t), { kind: "next" });
    assert.deepEqual(decide("/api/auth/me", t), { kind: "next" });
  });
});

describe("sesi", () => {
  it("tanpa token: halaman dialihkan ke /masuk dengan tujuan semula", () => {
    assert.deepEqual(decide("/kpi"), {
      kind: "redirect",
      path: "/masuk",
      query: { lanjut: "/kpi" },
      clearAccessCookie: false,
    });
  });

  it("tanpa token: API menjawab 401 Unauthorized", () => {
    const d = decide("/api/pengguna");
    assert.equal(d.kind, "json");
    assert.equal(d.kind === "json" && d.status, 401);
    assert.equal(d.kind === "json" && d.body.message, "Unauthorized");
  });

  it("hanya refresh token: halaman ke /masuk, API 401 Token expired", () => {
    assert.equal(decide("/kpi", undefined, "r").kind, "redirect");
    const d = decide("/api/pengguna", undefined, "r");
    assert.equal(d.kind === "json" && d.body.message, "Token expired");
  });

  it("token kedaluwarsa diperlakukan seperti tidak ada dan cookie dihapus", () => {
    const expired = tokenFor("MANAJER_PROGRAM", { exp: NOW - 1 });
    assert.deepEqual(decide("/kpi", expired, "r"), {
      kind: "redirect",
      path: "/masuk",
      query: { lanjut: "/kpi" },
      clearAccessCookie: true,
    });
    const api = decide("/api/x", expired);
    assert.equal(api.kind === "json" && api.status, 401);
    assert.equal(api.kind === "json" && api.clearAccessCookie, true);
  });

  it("token rusak: halaman ke /masuk dan cookie dihapus (tanpa loop, /masuk publik)", () => {
    assert.deepEqual(decide("/kpi", "sampah"), {
      kind: "redirect",
      path: "/masuk",
      query: { lanjut: "/kpi" },
      clearAccessCookie: true,
    });
    assert.deepEqual(decide("/masuk", "sampah"), { kind: "next" });
  });

  it("token tanpa exp tidak dianggap kedaluwarsa (API yang memutuskan)", () => {
    assert.deepEqual(
      decide(
        "/kpi",
        jwt({ role: "MANAJER_PROGRAM", permissions: ["kpi.lihat"] }),
      ),
      { kind: "next" },
    );
  });
});

describe("wajib ganti kata sandi", () => {
  const t = tokenFor("MANAJER_PROGRAM", { wajib_ganti_password: true });
  it("halaman dialihkan ke /ganti-password", () => {
    assert.deepEqual(decide("/kpi", t), {
      kind: "redirect",
      path: "/ganti-password",
      query: { lanjut: "/kpi" },
    });
  });
  it("API menjawab 403 PASSWORD_CHANGE_REQUIRED", () => {
    const d = decide("/api/pengguna", t);
    assert.equal(d.kind === "json" && d.status, 403);
    assert.equal(d.kind === "json" && d.body.code, "PASSWORD_CHANGE_REQUIRED");
  });
});

describe("izin rute", () => {
  it("peran tanpa izin dialihkan ke /akses-ditolak membawa asal rute", () => {
    assert.deepEqual(decide("/pengguna", tokenFor("MANAJER_PROGRAM")), {
      kind: "redirect",
      path: "/akses-ditolak",
      query: { dari: "/pengguna" },
    });
  });

  it("sub-rute mewarisi izin prefiks", () => {
    assert.equal(
      decide("/master/komoditas", tokenFor("OPS_MAHASISWA")).kind,
      "redirect",
    );
    assert.deepEqual(decide("/master/komoditas", tokenFor("ADMIN_SISTEM")), {
      kind: "next",
    });
  });

  it("OPS_MAHASISWA hanya membuka aplikasi lapangan", () => {
    assert.deepEqual(decide("/lapangan/input", tokenFor("OPS_MAHASISWA")), {
      kind: "next",
    });
    assert.equal(
      decide("/operasional", tokenFor("OPS_MAHASISWA")).kind,
      "redirect",
    );
  });

  it("izin diambil dari klaim token, bukan matriks statis", () => {
    // Token menyempit: MANAJER_PROGRAM tanpa kpi.lihat -> ditolak walau matriks statis mengizinkan.
    assert.equal(
      decide(
        "/kpi",
        jwt({ role: "MANAJER_PROGRAM", permissions: [], exp: NOW + 60 }),
      ).kind,
      "redirect",
    );
    assert.deepEqual(
      decide(
        "/kpi",
        jwt({
          role: "OPS_MAHASISWA",
          permissions: ["kpi.lihat"],
          exp: NOW + 60,
        }),
      ),
      { kind: "next" },
    );
  });

  it("tanpa klaim permissions, jatuh ke matriks peran", () => {
    assert.deepEqual(
      decide("/kpi", jwt({ role: "MANAJER_PROGRAM", exp: NOW + 60 })),
      { kind: "next" },
    );
    assert.equal(
      decide("/kpi", jwt({ role: "ADMIN_SISTEM", exp: NOW + 60 })).kind,
      "redirect",
    );
  });

  it("rute tanpa izin terdaftar (/alur-sistem) terbuka bagi semua yang masuk", () => {
    for (const role of ROLE_IDS)
      assert.deepEqual(decide("/alur-sistem", tokenFor(role)), {
        kind: "next",
      });
  });

  it("endpoint BFF /api/* tidak terkena izin halaman", () => {
    assert.deepEqual(decide("/api/pengguna", tokenFor("OPS_MAHASISWA")), {
      kind: "next",
    });
  });

  it("keputusan edge sama dengan canAccessRoute untuk semua peran x rute", () => {
    for (const role of ROLE_IDS) {
      for (const { prefix } of ROUTE_ACCESS) {
        const path = `${prefix}/uji`;
        const allowed = decide(path, tokenFor(role)).kind === "next";
        assert.equal(allowed, canAccessRoute(role, path), `${role} ${path}`);
      }
    }
  });
});

describe("beranda peran", () => {
  it("/ mengalihkan ke beranda sesuai peran (bukan peran tetap)", () => {
    for (const role of ROLE_IDS) {
      assert.deepEqual(decide("/", tokenFor(role)), {
        kind: "redirect",
        path: ROLES[role].beranda,
      });
    }
  });

  it("beranda setiap peran dapat dibuka oleh perannya sendiri", () => {
    for (const role of ROLE_IDS) {
      assert.deepEqual(
        decide(ROLES[role].beranda, tokenFor(role)),
        { kind: "next" },
        `${role} -> ${ROLES[role].beranda}`,
      );
    }
  });
});
