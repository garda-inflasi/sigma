import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { DEV_ROLE_COOKIE, devAuthEnabled, resolveSession } from "./resolve";

const env = process.env as Record<string, string | undefined>;
const original = {
  NODE_ENV: env.NODE_ENV,
  GARDA_JWT_SECRET: env.GARDA_JWT_SECRET,
};

afterEach(() => {
  for (const [k, v] of Object.entries(original)) {
    if (v === undefined) delete env[k];
    else env[k] = v;
  }
});

describe("devAuthEnabled", () => {
  it("aktif di development/test, mati di production", () => {
    assert.equal(devAuthEnabled({ NODE_ENV: "development" }), true);
    assert.equal(devAuthEnabled({ NODE_ENV: "test" }), true);
    assert.equal(devAuthEnabled({}), true);
    assert.equal(devAuthEnabled({ NODE_ENV: "production" }), false);
  });
});

describe("resolveSession di production", () => {
  it("cookie peran dev diabaikan: tanpa JWT sah sesi null", async () => {
    env.NODE_ENV = "production";
    delete env.GARDA_JWT_SECRET;
    const jar = (n: string) =>
      n === DEV_ROLE_COOKIE ? "ADMIN_SISTEM" : undefined;
    assert.equal(await resolveSession(jar), null);
  });

  it("token palsu dengan rahasia terpasang ditolak", async () => {
    env.NODE_ENV = "production";
    env.GARDA_JWT_SECRET = "x".repeat(40);
    const jar = (n: string) => (n === "garda_access" ? "a.b.c" : undefined);
    assert.equal(await resolveSession(jar), null);
  });

  it("di development cookie peran dev masih memberi sesi demo", async () => {
    env.NODE_ENV = "development";
    delete env.GARDA_JWT_SECRET;
    const user = await resolveSession((n) =>
      n === DEV_ROLE_COOKIE ? "ADMIN_SISTEM" : undefined,
    );
    assert.equal(user?.role, "ADMIN_SISTEM");
  });
});
