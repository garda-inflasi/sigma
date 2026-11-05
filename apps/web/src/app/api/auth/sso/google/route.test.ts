import assert from "node:assert/strict";
import { afterEach, before, describe, it } from "node:test";

import { NextRequest } from "next/server";

import { decodeFlow } from "@/lib/bff/sso";
import {
  amplopGalat,
  amplopSukses,
  cookieDiset,
  fetchPalsu,
  impor,
  nilaiCookie,
} from "@/lib/bff/sso-testing";

const BINDING = "q3Zk9vT2mXb7LpR4nYc1Hd5FgJ8sWe0uAaB6CiDkEoM";
const URL_GOOGLE =
  "https://accounts.google.com/o/oauth2/v2/auth?client_id=x&state=s";

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

function mulai(query = "", ip = `10.2.0.${++urutanIp}`): NextRequest {
  return new NextRequest(`http://localhost:8090/api/auth/sso/google${query}`, {
    headers: { "x-forwarded-for": ip },
  });
}

function stub(penanggap: Parameters<typeof fetchPalsu>[0]) {
  const f = fetchPalsu(penanggap);
  pulihkan = f.pulihkan;
  return f.panggilan;
}

const lokasi = (r: Response) => r.headers.get("location") ?? "";

describe("GET /api/auth/sso/google", () => {
  it("mengambil URL otorisasi dari API, menyimpan binding di cookie httpOnly Lax, lalu mengalihkan ke Google", async () => {
    const panggilan = stub(() =>
      amplopSukses({ authorize_url: URL_GOOGLE, binding: BINDING }),
    );

    const response = await GET(mulai("", "203.0.113.9"));

    assert.equal(panggilan.length, 1);
    assert.match(panggilan[0].url, /\/auth\/sso\/google\/authorize$/);
    assert.equal(panggilan[0].method, "POST");
    assert.equal(panggilan[0].headers["x-forwarded-for"], "203.0.113.9");

    assert.equal(response.status, 302);
    assert.equal(lokasi(response), URL_GOOGLE);
    assert.equal(response.headers.get("cache-control"), "no-store");

    const cookie = cookieDiset(response, "garda_sso");
    assert.ok(cookie, "cookie alur terpasang");
    assert.match(cookie, /HttpOnly/i);
    assert.match(cookie, /SameSite=lax/i);
    assert.match(cookie, /Max-Age=600/i);
    assert.match(cookie, /Path=\/api\/auth\/sso\/google/i);
    assert.deepEqual(decodeFlow(nilaiCookie(cookie)), {
      binding: BINDING,
      lanjut: "/",
    });
    assert.ok(
      !lokasi(response).includes(BINDING),
      "binding tidak pernah ada di URL",
    );
  });

  it("menyimpan tujuan semula (lanjut) yang aman di cookie", async () => {
    stub(() => amplopSukses({ authorize_url: URL_GOOGLE, binding: BINDING }));
    const response = await GET(
      mulai(`?lanjut=${encodeURIComponent("/harga?wilayah=1171")}`),
    );
    assert.deepEqual(
      decodeFlow(nilaiCookie(cookieDiset(response, "garda_sso"))),
      {
        binding: BINDING,
        lanjut: "/harga?wilayah=1171",
      },
    );
  });

  it("tujuan berbahaya diganti /", async () => {
    stub(() => amplopSukses({ authorize_url: URL_GOOGLE, binding: BINDING }));
    for (const jahat of [
      "//evil.example",
      "https://evil.example",
      "/\\evil.example",
      "/api/auth/logout",
    ]) {
      const response = await GET(mulai(`?lanjut=${encodeURIComponent(jahat)}`));
      assert.equal(
        decodeFlow(nilaiCookie(cookieDiset(response, "garda_sso")))?.lanjut,
        "/",
        jahat,
      );
    }
  });

  it("SSO tidak dikonfigurasi di API -> kembali ke /masuk dengan pesan yang jelas, tanpa cookie", async () => {
    stub(() => amplopGalat(503, "SSO_NOT_CONFIGURED"));
    const response = await GET(mulai());
    assert.equal(response.status, 302);
    assert.equal(lokasi(response), "/masuk?error=sso_tidak_aktif");
    assert.equal(cookieDiset(response, "garda_sso"), undefined);
  });

  it("penerbit tidak terjangkau -> sso_tidak_tersedia", async () => {
    stub(() => amplopGalat(503, "SSO_UNAVAILABLE"));
    assert.equal(lokasi(await GET(mulai())), "/masuk?error=sso_tidak_tersedia");
  });

  it("galat membawa tujuan semula agar bisa dicoba lagi", async () => {
    stub(() => amplopGalat(503, "SSO_UNAVAILABLE"));
    const response = await GET(
      mulai(`?lanjut=${encodeURIComponent("/harga")}`),
    );
    assert.equal(
      lokasi(response),
      `/masuk?error=sso_tidak_tersedia&lanjut=${encodeURIComponent("/harga")}`,
    );
  });

  it("API tidak terjangkau -> sso_tidak_tersedia", async () => {
    stub(() => {
      throw new TypeError("fetch failed");
    });
    assert.equal(lokasi(await GET(mulai())), "/masuk?error=sso_tidak_tersedia");
  });

  it("URL otorisasi bukan http(s) atau binding kosong tidak diikuti", async () => {
    stub(() =>
      amplopSukses({ authorize_url: "javascript:alert(1)", binding: BINDING }),
    );
    assert.equal(lokasi(await GET(mulai())), "/masuk?error=sso_gagal");
    pulihkan?.();
    stub(() => amplopSukses({ authorize_url: URL_GOOGLE, binding: "" }));
    assert.equal(lokasi(await GET(mulai())), "/masuk?error=sso_gagal");
  });

  it("dibatasi 10 permulaan per menit per IP", async () => {
    const panggilan = stub(() =>
      amplopSukses({ authorize_url: URL_GOOGLE, binding: BINDING }),
    );
    const ip = "198.51.100.88";
    for (let i = 0; i < 10; i++)
      assert.equal(lokasi(await GET(mulai("", ip))), URL_GOOGLE);

    const response = await GET(mulai("", ip));
    assert.equal(lokasi(response), "/masuk?error=sso_terlalu_banyak");
    assert.equal(panggilan.length, 10);
    assert.equal(cookieDiset(response, "garda_sso"), undefined);
  });
});
