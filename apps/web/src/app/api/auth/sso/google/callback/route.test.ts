import assert from "node:assert/strict";
import { afterEach, before, describe, it } from "node:test";

import { NextRequest } from "next/server";

import { encodeFlow } from "@/lib/bff/sso";
import {
  amplopGalat,
  amplopSukses,
  cookieDiset,
  fetchPalsu,
  impor,
  nilaiCookie,
  setCookies,
} from "@/lib/bff/sso-testing";

const BINDING = "q3Zk9vT2mXb7LpR4nYc1Hd5FgJ8sWe0uAaB6CiDkEoM";
const PENGGUNA = {
  id: "u1",
  nama: "Ani",
  email: "ani@kemenkeu.go.id",
  peran: "ADMIN_SISTEM",
  wajib_ganti_password: false,
};
const SESI = {
  access_token: "akses.jwt.token",
  refresh_token: "segar-token",
  user: PENGGUNA,
};

let GET: (request: NextRequest) => Promise<Response>;
let pulihkan: (() => void) | undefined;
let urutanIp = 0;

before(async () => {
  ({ GET } = await impor(() => import("./route")));
});

afterEach(() => {
  pulihkan?.();
  pulihkan = undefined;
});

/** Permintaan callback dengan cookie alur (kecuali `tanpaCookie`) dan IP unik agar batas laju tidak saling mengganggu. */
function callback(
  query: string,
  opsi: { cookie?: string | null; ip?: string } = {},
): NextRequest {
  const headers: Record<string, string> = {
    "x-forwarded-for": opsi.ip ?? `10.1.0.${++urutanIp}`,
  };
  const cookie =
    opsi.cookie === undefined
      ? `garda_sso=${encodeFlow(BINDING, "/")}`
      : opsi.cookie;
  if (cookie) headers.cookie = cookie;
  return new NextRequest(
    `http://localhost:8090/api/auth/sso/google/callback?${query}`,
    { headers },
  );
}

function stub(penanggap: Parameters<typeof fetchPalsu>[0]) {
  const f = fetchPalsu(penanggap);
  pulihkan = f.pulihkan;
  return f.panggilan;
}

function lokasi(response: Response): string {
  return response.headers.get("location") ?? "";
}

/** Pengalihan kembali ke /masuk dengan kode galat; tidak ada cookie sesi/tantangan, cookie alur dihapus. */
function harapGalat(response: Response, kode: string) {
  assert.equal(response.status, 302);
  assert.equal(lokasi(response), `/masuk?error=${kode}`);
  assert.equal(
    cookieDiset(response, "garda_access"),
    undefined,
    "tanpa cookie akses",
  );
  assert.equal(cookieDiset(response, "garda_refresh"), undefined);
  assert.equal(cookieDiset(response, "garda_mfa"), undefined);
  const flow = cookieDiset(response, "garda_sso");
  assert.ok(flow, "cookie alur dihapus");
  assert.match(flow, /Max-Age=0/i);
}

describe("GET /api/auth/sso/google/callback: berhasil", () => {
  it("menyerahkan code, state, dan binding cookie ke API lalu memasang sesi seperti login kata sandi", async () => {
    const panggilan = stub(() => amplopSukses(SESI));

    const response = await GET(
      callback("code=kode-1&state=state-1", { ip: "203.0.113.7" }),
    );

    assert.equal(panggilan.length, 1);
    assert.match(panggilan[0].url, /\/auth\/sso\/google\/callback$/);
    assert.equal(panggilan[0].method, "POST");
    assert.deepEqual(panggilan[0].body, {
      code: "kode-1",
      state: "state-1",
      binding: BINDING,
    });
    assert.equal(
      panggilan[0].headers["x-forwarded-for"],
      "203.0.113.7",
      "IP klien diteruskan untuk audit",
    );

    assert.equal(response.status, 302);
    assert.equal(
      lokasi(response),
      "/",
      "tujuan bawaan; halaman / memilih beranda peran",
    );

    const akses = cookieDiset(response, "garda_access");
    assert.ok(
      akses && /HttpOnly/i.test(akses) && /SameSite=lax/i.test(akses),
      akses,
    );
    assert.equal(nilaiCookie(akses), "akses.jwt.token");
    const segar = cookieDiset(response, "garda_refresh");
    assert.ok(
      segar &&
        /HttpOnly/i.test(segar) &&
        /SameSite=strict/i.test(segar) &&
        /Path=\/api\/auth/i.test(segar),
      segar,
    );
    assert.ok(cookieDiset(response, "garda_csrf"), "cookie CSRF dipasang");

    const flow = cookieDiset(response, "garda_sso");
    assert.ok(
      flow && /Max-Age=0/i.test(flow),
      "cookie alur dihapus setelah dipakai",
    );
    assert.equal(response.headers.get("cache-control"), "no-store");
    assert.equal(response.headers.get("referrer-policy"), "no-referrer");
  });

  it("mengembalikan ke tujuan semula (lanjut) yang tersimpan di cookie alur", async () => {
    stub(() => amplopSukses(SESI));
    const response = await GET(
      callback("code=c&state=s", {
        cookie: `garda_sso=${encodeFlow(BINDING, "/harga?wilayah=1171")}`,
      }),
    );
    assert.equal(lokasi(response), "/harga?wilayah=1171");
  });

  it("tujuan di cookie yang dipalsukan tetap disanitasi", async () => {
    stub(() => amplopSukses(SESI));
    const palsu = `${BINDING}.${Buffer.from("//evil.example/x", "utf8").toString("base64url")}`;
    const response = await GET(
      callback("code=c&state=s", { cookie: `garda_sso=${palsu}` }),
    );
    assert.equal(lokasi(response), "/");
  });

  it("akun wajib ganti kata sandi diarahkan ke halaman ganti kata sandi dengan tujuan semula", async () => {
    stub(() =>
      amplopSukses({
        ...SESI,
        user: { ...PENGGUNA, wajib_ganti_password: true },
      }),
    );
    const response = await GET(callback("code=c&state=s"));
    assert.equal(
      lokasi(response),
      `/ganti-password?lanjut=${encodeURIComponent("/")}`,
    );
    assert.ok(cookieDiset(response, "garda_access"), "sesi tetap terbit");
  });

  it("akun wajib MFA yang belum setup diarahkan ke halaman setup MFA", async () => {
    stub(() => amplopSukses({ ...SESI, mfa_setup_diwajibkan: true }));
    const response = await GET(callback("code=c&state=s"));
    assert.equal(lokasi(response), `/mfa?lanjut=${encodeURIComponent("/")}`);
  });
});

describe("GET /api/auth/sso/google/callback: akun ber-MFA", () => {
  it("memasang cookie tantangan dan melanjutkan ke langkah OTP tanpa sesi", async () => {
    stub(() =>
      amplopSukses({ mfa_required: true, mfa_token: "tantangan.jwt" }),
    );
    const response = await GET(
      callback("code=c&state=s", {
        cookie: `garda_sso=${encodeFlow(BINDING, "/harga")}`,
      }),
    );

    assert.equal(response.status, 302);
    assert.equal(
      lokasi(response),
      `/masuk?mfa=1&lanjut=${encodeURIComponent("/harga")}`,
    );

    const tantangan = cookieDiset(response, "garda_mfa");
    assert.ok(
      tantangan &&
        /HttpOnly/i.test(tantangan) &&
        /Path=\/api\/auth/i.test(tantangan),
      tantangan,
    );
    assert.equal(nilaiCookie(tantangan), "tantangan.jwt");
    assert.equal(
      cookieDiset(response, "garda_access"),
      undefined,
      "belum ada sesi sebelum OTP",
    );
    assert.equal(cookieDiset(response, "garda_refresh"), undefined);
    assert.match(cookieDiset(response, "garda_sso") ?? "", /Max-Age=0/i);
  });
});

describe("GET /api/auth/sso/google/callback: ditolak sebelum menghubungi API", () => {
  it("tanpa cookie alur (login CSRF atau cookie kedaluwarsa)", async () => {
    const panggilan = stub(() => amplopSukses(SESI));
    const response = await GET(callback("code=c&state=s", { cookie: null }));
    harapGalat(response, "sso_sesi_tidak_valid");
    assert.equal(panggilan.length, 0, "API tidak dipanggil");
  });

  it("cookie alur rusak atau dipalsukan", async () => {
    const panggilan = stub(() => amplopSukses(SESI));
    for (const cookie of [
      "garda_sso=bukan-cookie-sah",
      "garda_sso=pendek.Lw",
      `garda_sso=${BINDING}`,
    ]) {
      harapGalat(
        await GET(callback("code=c&state=s", { cookie })),
        "sso_sesi_tidak_valid",
      );
    }
    assert.equal(panggilan.length, 0);
  });

  it("code atau state hilang atau terlalu panjang", async () => {
    const panggilan = stub(() => amplopSukses(SESI));
    for (const query of [
      "",
      "code=c",
      "state=s",
      `code=${"c".repeat(2049)}&state=s`,
    ]) {
      harapGalat(await GET(callback(query)), "sso_sesi_tidak_valid");
    }
    assert.equal(panggilan.length, 0);
  });

  it("penyedia mengembalikan error (pengguna menolak izin)", async () => {
    const panggilan = stub(() => amplopSukses(SESI));
    harapGalat(
      await GET(callback("error=access_denied&state=s")),
      "sso_dibatalkan",
    );
    harapGalat(await GET(callback("error=server_error&state=s")), "sso_gagal");
    assert.equal(panggilan.length, 0);
  });

  it("pengalihan galat membawa tujuan semula agar bisa dicoba lagi", async () => {
    stub(() => amplopSukses(SESI));
    const response = await GET(
      callback("error=access_denied", {
        cookie: `garda_sso=${encodeFlow(BINDING, "/harga")}`,
      }),
    );
    assert.equal(
      lokasi(response),
      `/masuk?error=sso_dibatalkan&lanjut=${encodeURIComponent("/harga")}`,
    );
  });
});

describe("GET /api/auth/sso/google/callback: galat dari API", () => {
  const kasus: Array<[string, number, string, string]> = [
    [
      "akun tidak ada",
      403,
      "SSO_ACCOUNT_NOT_FOUND",
      "sso_akun_tidak_ditemukan",
    ],
    ["subjek berbeda", 409, "SSO_ACCOUNT_CONFLICT", "sso_akun_bentrok"],
    ["akun tidak aktif", 403, "SSO_ACCOUNT_INACTIVE", "sso_akun_nonaktif"],
    ["akun terkunci", 423, "SSO_ACCOUNT_LOCKED", "sso_akun_terkunci"],
    [
      "email belum terverifikasi",
      403,
      "SSO_EMAIL_UNVERIFIED",
      "sso_email_belum_terverifikasi",
    ],
    ["domain ditolak", 403, "SSO_DOMAIN_NOT_ALLOWED", "sso_domain_ditolak"],
    ["state tidak valid", 400, "SSO_STATE_INVALID", "sso_sesi_tidak_valid"],
    ["verifikasi gagal", 401, "SSO_EXCHANGE_FAILED", "sso_verifikasi_gagal"],
    ["penyedia tidak tersedia", 503, "SSO_UNAVAILABLE", "sso_tidak_tersedia"],
    ["SSO tidak dikonfigurasi", 503, "SSO_NOT_CONFIGURED", "sso_tidak_aktif"],
    ["galat server", 500, "500", "sso_tidak_tersedia"],
  ];
  for (const [nama, status, kode, galat] of kasus) {
    it(`${nama}: ${status} ${kode} -> ?error=${galat}`, async () => {
      stub(() => amplopGalat(status, kode));
      harapGalat(await GET(callback("code=c&state=s")), galat);
    });
  }

  it("API tidak terjangkau (fetch gagal) -> tidak tersedia", async () => {
    stub(() => {
      throw new TypeError("fetch failed");
    });
    harapGalat(await GET(callback("code=c&state=s")), "sso_tidak_tersedia");
  });

  it("jawaban sukses tanpa sesi maupun tantangan -> gagal, bukan sesi kosong", async () => {
    stub(() => amplopSukses({ user: PENGGUNA }));
    harapGalat(await GET(callback("code=c&state=s")), "sso_gagal");
  });

  it("jawaban bukan JSON dari proxy -> tidak tersedia", async () => {
    stub(() => new Response("<html>Bad Gateway</html>", { status: 502 }));
    harapGalat(await GET(callback("code=c&state=s")), "sso_tidak_tersedia");
  });
});

describe("GET /api/auth/sso/google/callback: batas laju", () => {
  it("percobaan ke-11 dari IP yang sama dalam semenit ditolak tanpa menghubungi API", async () => {
    const panggilan = stub(() => amplopSukses(SESI));
    const ip = "198.51.100.77";
    for (let i = 0; i < 10; i++) {
      assert.equal((await GET(callback("code=c&state=s", { ip }))).status, 302);
    }
    assert.equal(panggilan.length, 10);

    const response = await GET(callback("code=c&state=s", { ip }));
    assert.equal(lokasi(response), "/masuk?error=sso_terlalu_banyak");
    assert.equal(panggilan.length, 10, "API tidak dipanggil lagi");
    assert.equal(cookieDiset(response, "garda_access"), undefined);
    assert.ok(setCookies(response).length >= 0);
  });
});
