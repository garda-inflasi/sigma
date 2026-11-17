import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { SignJWT } from "jose";
import { impersonasiDariKlaim } from "./impersonasi";
import { resolveSession, SESSION_COOKIE } from "./resolve";

const SASARAN = "11111111-1111-4111-8111-111111111111";
const PELAKU = "22222222-2222-4222-8222-222222222222";
const MULAI = 1_763_370_000;

function klaim(sisip: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    sub: SASARAN,
    role: "OPS_KOORDINATOR",
    name: "Zulkifli, S.P.",
    imp: {
      id: "33333333-3333-4333-8333-333333333333",
      by: {
        user_id: PELAKU,
        name: "Super Admin",
        email: "superadmin@gardainflasi.id",
        role: "ADMIN_SISTEM",
      },
      started_at: MULAI,
    },
    ...sisip,
  };
}

describe("impersonasiDariKlaim", () => {
  it("klaim sah menjadi metadata: pelaku asli, sasaran dari sub/name/role, waktu mulai ISO", () => {
    assert.deepEqual(impersonasiDariKlaim(klaim()), {
      active: true,
      originalUser: {
        id: PELAKU,
        name: "Super Admin",
        email: "superadmin@gardainflasi.id",
        role: "ADMIN_SISTEM",
      },
      targetUser: {
        id: SASARAN,
        name: "Zulkifli, S.P.",
        role: "OPS_KOORDINATOR",
      },
      startedAt: new Date(MULAI * 1000).toISOString(),
    });
  });

  it("tanpa klaim imp: bukan impersonasi", () => {
    const { imp, ...tanpa } = klaim();
    void imp;
    assert.equal(impersonasiDariKlaim(tanpa), undefined);
  });

  it("klaim cacat dalam bentuk apa pun menghasilkan undefined, bukan galat", () => {
    const sah = klaim().imp as Record<string, unknown>;
    const by = sah.by as Record<string, unknown>;
    const cacat: unknown[] = [
      null,
      "imp",
      5,
      [],
      {},
      { ...sah, id: "" },
      { ...sah, id: 7 },
      { ...sah, by: null },
      { ...sah, by: "admin" },
      { ...sah, by: { ...by, user_id: "" } },
      { ...sah, by: { ...by, name: " " } },
      { ...sah, by: { ...by, email: undefined } },
      { ...sah, by: { ...by, role: "SUPERADMIN" } },
      { ...sah, by: { ...by, role: 3 } },
      { ...sah, started_at: "1763370000" },
      { ...sah, started_at: 0 },
      { ...sah, started_at: -5 },
      { ...sah, started_at: Number.NaN },
      { ...sah, started_at: 1e20 },
    ];
    for (const imp of cacat) {
      assert.equal(
        impersonasiDariKlaim(klaim({ imp })),
        undefined,
        JSON.stringify(imp),
      );
    }
  });

  it("token tanpa sub atau dengan peran tak dikenal bukan sesi impersonasi yang dapat dipakai", () => {
    assert.equal(impersonasiDariKlaim(klaim({ sub: undefined })), undefined);
    assert.equal(impersonasiDariKlaim(klaim({ sub: "" })), undefined);
    assert.equal(
      impersonasiDariKlaim(klaim({ role: "SUPERADMIN" })),
      undefined,
    );
  });

  it("nama sasaran yang tidak ada menjadi teks kosong, bukan 'undefined'", () => {
    assert.equal(
      impersonasiDariKlaim(klaim({ name: undefined }))?.targetUser.name,
      "",
    );
  });
});

describe("resolveSession membawa impersonasi dari token bertanda tangan", () => {
  const env = process.env as Record<string, string | undefined>;
  const asli = {
    NODE_ENV: env.NODE_ENV,
    GARDA_JWT_SECRET: env.GARDA_JWT_SECRET,
  };
  const RAHASIA = "r".repeat(40);
  afterEach(() => {
    for (const [k, v] of Object.entries(asli)) {
      if (v === undefined) delete env[k];
      else env[k] = v;
    }
  });

  async function token(isi: Record<string, unknown>) {
    const { sub, ...sisa } = isi;
    return new SignJWT(sisa)
      .setProtectedHeader({ alg: "HS256" })
      .setIssuer("garda-auth")
      .setAudience("garda-api")
      .setSubject(String(sub))
      .setExpirationTime("10m")
      .sign(new TextEncoder().encode(RAHASIA));
  }
  const jar = (t: string) => (n: string) =>
    n === SESSION_COOKIE ? t : undefined;

  it("identitas sesi = sasaran dan impersonation menunjuk pelaku asli", async () => {
    env.NODE_ENV = "production";
    env.GARDA_JWT_SECRET = RAHASIA;
    const user = await resolveSession(
      jar(
        await token(
          klaim({
            email: "zul@x.id",
            org: "FMIPA",
            permissions: ["harga.input"],
          }),
        ),
      ),
    );

    assert.ok(user);
    assert.equal(user.id, SASARAN);
    assert.equal(user.role, "OPS_KOORDINATOR");
    assert.equal(user.impersonation?.active, true);
    assert.equal(user.impersonation?.originalUser.id, PELAKU);
    assert.equal(user.impersonation?.originalUser.role, "ADMIN_SISTEM");
    assert.equal(user.impersonation?.targetUser.id, SASARAN);
  });

  it("sesi biasa tidak punya impersonation", async () => {
    env.NODE_ENV = "production";
    env.GARDA_JWT_SECRET = RAHASIA;
    const { imp, ...biasa } = klaim();
    void imp;
    const user = await resolveSession(jar(await token(biasa)));
    assert.ok(user);
    assert.equal(user.impersonation, undefined);
  });

  it("klaim imp cacat tidak merusak sesi: tetap masuk sebagai sasaran tanpa banner", async () => {
    env.NODE_ENV = "production";
    env.GARDA_JWT_SECRET = RAHASIA;
    const user = await resolveSession(
      jar(await token(klaim({ imp: { id: 1 } }))),
    );
    assert.ok(user);
    assert.equal(user.role, "OPS_KOORDINATOR");
    assert.equal(user.impersonation, undefined);
  });

  it("mode dev (garda_dev_role) tidak punya impersonasi", async () => {
    env.NODE_ENV = "development";
    delete env.GARDA_JWT_SECRET;
    const user = await resolveSession((n) =>
      n === "garda_dev_role" ? "OPS_KOORDINATOR" : undefined,
    );
    assert.ok(user);
    assert.equal(user.impersonation, undefined);
  });
});
