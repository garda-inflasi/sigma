import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { SignJWT } from "jose";
import { can } from "@/lib/rbac/permissions";
import { izinDariKlaim, resolveSession, SESSION_COOKIE } from "./resolve";

const env = process.env as Record<string, string | undefined>;
const original = {
  NODE_ENV: env.NODE_ENV,
  GARDA_JWT_SECRET: env.GARDA_JWT_SECRET,
};
const RAHASIA = "r".repeat(40);

afterEach(() => {
  for (const [k, v] of Object.entries(original)) {
    if (v === undefined) delete env[k];
    else env[k] = v;
  }
});

async function tokenDengan(klaim: Record<string, unknown>): Promise<string> {
  return new SignJWT(klaim)
    .setProtectedHeader({ alg: "HS256" })
    .setIssuer("garda-auth")
    .setAudience("garda-api")
    .setSubject("u-1")
    .setExpirationTime("10m")
    .sign(new TextEncoder().encode(RAHASIA));
}

const cookie = (token: string) => (nama: string) =>
  nama === SESSION_COOKIE ? token : undefined;

describe("izinDariKlaim", () => {
  it("larik string diterima apa adanya, termasuk larik kosong", () => {
    assert.deepEqual(izinDariKlaim(["a.b", "c.d"]), ["a.b", "c.d"]);
    assert.deepEqual(izinDariKlaim([]), []);
  });

  it("bukan larik string berarti tidak ada klaim", () => {
    for (const nilai of [
      undefined,
      null,
      "a.b",
      5,
      { a: 1 },
      ["a.b", 3],
      [null],
    ]) {
      assert.equal(izinDariKlaim(nilai), undefined, JSON.stringify(nilai));
    }
  });
});

describe("resolveSession membawa izin efektif dari token", () => {
  it("klaim permissions masuk ke SessionUser dan menentukan can()", async () => {
    env.NODE_ENV = "production";
    env.GARDA_JWT_SECRET = RAHASIA;
    const token = await tokenDengan({
      role: "OPS_MAHASISWA",
      name: "Uji",
      email: "uji@x.id",
      org: "FMIPA",
      permissions: ["harga.input", "kpi.lihat"],
    });

    const user = await resolveSession(cookie(token));

    assert.ok(user);
    assert.equal(user.role, "OPS_MAHASISWA");
    assert.deepEqual(user.permissions, ["harga.input", "kpi.lihat"]);
    assert.equal(can(user, "kpi.lihat"), true, "izin langsung dari token");
    assert.equal(
      can(user, "tugas.lihat_sendiri"),
      false,
      "dicabut dari matriks peran",
    );
  });

  it("token tanpa klaim permissions: permissions undefined dan can() memakai matriks bawaan", async () => {
    env.NODE_ENV = "production";
    env.GARDA_JWT_SECRET = RAHASIA;
    const token = await tokenDengan({ role: "ADMIN_SISTEM" });

    const user = await resolveSession(cookie(token));

    assert.ok(user);
    assert.equal(user.permissions, undefined);
    assert.equal(can(user, "pengguna.kelola"), true);
  });

  it("klaim permissions kosong (peran dicabut semua izinnya) tidak jatuh ke matriks bawaan", async () => {
    env.NODE_ENV = "production";
    env.GARDA_JWT_SECRET = RAHASIA;
    const token = await tokenDengan({ role: "ADMIN_SISTEM", permissions: [] });

    const user = await resolveSession(cookie(token));

    assert.ok(user);
    assert.deepEqual(user.permissions, []);
    assert.equal(can(user, "pengguna.kelola"), false);
  });

  it("klaim permissions yang rusak diperlakukan sebagai tidak ada, bukan melempar", async () => {
    env.NODE_ENV = "production";
    env.GARDA_JWT_SECRET = RAHASIA;
    const token = await tokenDengan({
      role: "OPS_MAHASISWA",
      permissions: "harga.input",
    });

    const user = await resolveSession(cookie(token));

    assert.ok(user);
    assert.equal(user.permissions, undefined);
  });

  it("sesi demo (cookie peran dev) tidak membawa klaim: matriks bawaan", async () => {
    env.NODE_ENV = "development";
    delete env.GARDA_JWT_SECRET;

    const user = await resolveSession((n) =>
      n === "garda_dev_role" ? "MANAJER_DATA" : undefined,
    );

    assert.ok(user);
    assert.equal(user.permissions, undefined);
    assert.equal(can(user, "harga.publikasi"), true);
  });
});
