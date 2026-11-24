import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, it } from "node:test";
import { PERMISSIONS, ROLE_PERMISSIONS, peranIzinTerkunci } from "./permissions";
import { ROLE_IDS } from "./roles";

/** Kontrak izin yang dihasilkan dari peta rolePermissions di API Go (lihat rbac_test.go). */
const contract = JSON.parse(
  readFileSync(
    resolve(process.cwd(), "../../packages/contracts/permissions.json"),
    "utf8",
  ),
) as { permissions: string[]; roles: Record<string, string[]> };

const sorted = (values: readonly string[]) => [...values].sort();

describe("izin web sama dengan API Go", () => {
  it("daftar izin sama", () => {
    assert.deepEqual(sorted(PERMISSIONS), sorted(contract.permissions));
  });

  it("memuat peran yang sama", () => {
    assert.deepEqual(sorted(ROLE_IDS), sorted(Object.keys(contract.roles)));
  });

  for (const role of ROLE_IDS) {
    it(`izin ${role} sama`, () => {
      assert.deepEqual(
        sorted(ROLE_PERMISSIONS[role]),
        sorted(contract.roles[role]),
      );
    });
  }

  it("setiap izin dalam daftar dimiliki minimal satu peran", () => {
    const dimiliki = new Set(
      ROLE_IDS.flatMap((role) => ROLE_PERMISSIONS[role]),
    );
    assert.deepEqual(sorted(PERMISSIONS.filter((p) => !dimiliki.has(p))), []);
  });
});

describe("ADMIN_SISTEM", () => {
  it("memegang seluruh katalog izin", () => {
    assert.deepEqual(sorted(ROLE_PERMISSIONS.ADMIN_SISTEM), sorted(PERMISSIONS));
    assert.deepEqual(sorted(contract.roles.ADMIN_SISTEM), sorted(contract.permissions));
  });

  it("satu-satunya peran yang kolom matriksnya terkunci", () => {
    assert.deepEqual(ROLE_IDS.filter(peranIzinTerkunci), ["ADMIN_SISTEM"]);
  });
});

describe("izin impersonasi.kelola", () => {
  it("ada di katalog dan eksklusif milik ADMIN_SISTEM pada matriks bawaan", () => {
    assert.ok((PERMISSIONS as readonly string[]).includes("impersonasi.kelola"));
    const pemegang = ROLE_IDS.filter((r) =>
      (ROLE_PERMISSIONS[r] as readonly string[]).includes("impersonasi.kelola"),
    );
    assert.deepEqual(pemegang, ["ADMIN_SISTEM"]);
  });
});
