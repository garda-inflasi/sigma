import assert from "node:assert/strict";
import { afterEach, before, describe, it } from "node:test";

let authApi: (typeof import("./api-client"))["authApi"];
let userApi: (typeof import("./api-client"))["userApi"];
let headerKlien: (typeof import("./api-client"))["headerKlien"];

before(async () => {
  process.env.GO_API_BASE_URL = "http://go.test/v1";
  ({ authApi, userApi, headerKlien } = await import("./api-client"));
});

const realFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = realFetch;
});

function stub(...responses: Response[]) {
  const calls: { url: string; init: RequestInit }[] = [];
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push({ url: String(input), init: init ?? {} });
    const next = responses.shift();
    assert.ok(next, "fetch dipanggil lebih banyak dari respons yang disiapkan");
    return next;
  }) as typeof fetch;
  return calls;
}

const envelope = (data: unknown) =>
  Response.json({ code: "200", message: "OK", data });
const header = (c: { init: RequestInit }, name: string) =>
  new Headers(c.init.headers).get(name);

describe("klien sesi BFF → Go", () => {
  it("logout meneruskan refresh token cookie di badan", async () => {
    const calls = stub(envelope(null));
    await authApi.logout("at", "rt");
    assert.equal(calls[0].url, "http://go.test/v1/auth/logout");
    assert.equal(calls[0].init.method, "POST");
    assert.equal(header(calls[0], "authorization"), "Bearer at");
    assert.deepEqual(JSON.parse(calls[0].init.body as string), {
      refresh_token: "rt",
    });
  });

  it("logout tanpa refresh token mengirim badan kosong, bukan token kosong", async () => {
    const calls = stub(envelope(null));
    await authApi.logout("at");
    assert.deepEqual(JSON.parse(calls[0].init.body as string), {});
  });

  it("logoutAll, sessions, revokeSession, revokeOtherSessions memakai rute Go yang benar", async () => {
    const calls = stub(
      envelope({ dicabut: 2 }),
      envelope([]),
      envelope({ id: "a b" }),
      envelope({ dicabut: 1 }),
    );
    await authApi.logoutAll("at");
    await authApi.sessions("at");
    await authApi.revokeSession("at", "a b");
    await authApi.revokeOtherSessions("at");
    assert.deepEqual(
      calls.map((c) => [c.init.method, c.url.replace("http://go.test/v1", "")]),
      [
        ["POST", "/auth/logout-all"],
        ["GET", "/auth/sessions"],
        ["DELETE", "/auth/sessions/a%20b"],
        ["POST", "/auth/sessions/revoke-others"],
      ],
    );
    for (const c of calls)
      assert.equal(header(c, "authorization"), "Bearer at");
  });

  it("forceLogout memanggil POST /users/:id/revoke-sessions", async () => {
    const calls = stub(envelope({ id: "u 1", dicabut: 2 }));
    const hasil = await userApi.forceLogout("at", "u 1");
    assert.deepEqual(hasil, { id: "u 1", dicabut: 2 });
    assert.equal(calls[0].url, "http://go.test/v1/users/u%201/revoke-sessions");
    assert.equal(calls[0].init.method, "POST");
  });

  it("refresh pemutaran ulang: jawaban tanpa refresh_token tetap diterima", async () => {
    stub(envelope({ access_token: "at" }));
    const hasil = await authApi.refresh({ refresh_token: "rt" });
    assert.equal(hasil.access_token, "at");
    assert.equal(hasil.refresh_token, undefined);
  });
});

describe("headerKlien", () => {
  it("di luar lingkup request tidak meneruskan apa pun dan tidak melempar", async () => {
    assert.deepEqual(await headerKlien(), {});
  });
});
