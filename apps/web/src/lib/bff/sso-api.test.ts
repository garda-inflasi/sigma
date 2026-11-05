import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";

import { ssoApi, ssoGoogleAktif } from "./sso-api";
import { amplopGalat, amplopSukses, fetchPalsu } from "./sso-testing";

let pulihkan: (() => void) | undefined;

afterEach(() => {
  pulihkan?.();
  pulihkan = undefined;
});

function stub(penanggap: Parameters<typeof fetchPalsu>[0]) {
  const f = fetchPalsu(penanggap);
  pulihkan = f.pulihkan;
  return f.panggilan;
}

describe("ssoGoogleAktif (keputusan di server, bukan flag build)", () => {
  it("true hanya bila API melaporkan google: true", async () => {
    const panggilan = stub(() => amplopSukses({ google: true }));
    assert.equal(await ssoGoogleAktif(), true);
    assert.equal(panggilan.length, 1);
    assert.match(panggilan[0].url, /\/auth\/sso\/status$/);
    assert.equal(panggilan[0].method, "GET");
  });

  it("false bila API melaporkan google: false", async () => {
    stub(() => amplopSukses({ google: false }));
    assert.equal(await ssoGoogleAktif(), false);
  });

  it("false (tombol disembunyikan) pada setiap galat, tanpa melempar", async () => {
    stub(() => amplopGalat(500, "500"));
    assert.equal(await ssoGoogleAktif(), false);
    pulihkan?.();

    stub(() => {
      throw new TypeError("fetch failed");
    });
    assert.equal(await ssoGoogleAktif(), false);
    pulihkan?.();

    stub(() => new Response("<html>bukan json</html>", { status: 200 }));
    assert.equal(await ssoGoogleAktif(), false);
    pulihkan?.();

    stub(() => amplopSukses({ google: "true" }));
    assert.equal(
      await ssoGoogleAktif(),
      false,
      "hanya boolean true yang dihitung",
    );
  });
});

describe("ssoApi", () => {
  it("authorize: POST tanpa kredensial, meneruskan IP klien, mengembalikan url dan binding", async () => {
    const panggilan = stub(() =>
      amplopSukses({ authorize_url: "https://idp/auth", binding: "b" }),
    );
    const hasil = await ssoApi.authorize("203.0.113.5, 10.0.0.1");
    assert.deepEqual(hasil, {
      authorize_url: "https://idp/auth",
      binding: "b",
    });
    assert.equal(panggilan[0].method, "POST");
    assert.equal(
      panggilan[0].headers["x-forwarded-for"],
      "203.0.113.5, 10.0.0.1",
    );
    assert.equal(panggilan[0].headers.authorization, undefined);
  });

  it("tanpa IP klien tidak mengirim X-Forwarded-For", async () => {
    const panggilan = stub(() =>
      amplopSukses({ authorize_url: "https://idp/auth", binding: "b" }),
    );
    await ssoApi.authorize(null);
    assert.equal(panggilan[0].headers["x-forwarded-for"], undefined);
  });

  it("galat API membawa status dan kode amplop (dipetakan ke ?error= oleh BFF)", async () => {
    stub(() => amplopGalat(403, "SSO_ACCOUNT_NOT_FOUND", "Tidak ada akun"));
    await assert.rejects(
      ssoApi.callback({ code: "c", state: "s", binding: "b" }),
      (e: { status?: number; code?: string; message?: string }) =>
        e.status === 403 &&
        e.code === "SSO_ACCOUNT_NOT_FOUND" &&
        e.message === "Tidak ada akun",
    );
  });

  it("jawaban 200 tanpa data dianggap galat, bukan hasil kosong", async () => {
    stub(() => Response.json({ code: "200", message: "OK" }));
    await assert.rejects(
      ssoApi.callback({ code: "c", state: "s", binding: "b" }),
    );
  });
});
