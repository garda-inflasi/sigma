import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  can,
  canAll,
  PERMISSIONS,
  ROLE_PERMISSIONS,
  type PemegangIzin,
} from "./permissions";
import { canAccessRoute } from "./routes";

describe("can: izin efektif dari token (02-C)", () => {
  it("peran saja memakai matriks bawaan (mode mock, pemanggil tanpa sesi)", () => {
    assert.equal(can("OPS_MAHASISWA", "harga.input"), true);
    assert.equal(can("OPS_MAHASISWA", "kpi.lihat"), false);
    assert.equal(can(undefined, "harga.input"), false);
    assert.equal(can(null, "harga.input"), false);
  });

  it("sesi dengan klaim memakai klaim, bukan matriks bawaan peran", () => {
    const sesi: PemegangIzin = {
      role: "OPS_MAHASISWA",
      permissions: ["harga.input", "kpi.lihat"],
    };
    assert.equal(
      can(sesi, "kpi.lihat"),
      true,
      "izin langsung dari token berlaku",
    );
    assert.equal(can(sesi, "harga.input"), true);
    assert.equal(
      can(sesi, "tugas.lihat_sendiri"),
      false,
      "izin bawaan yang dicabut dari matriks tidak lagi berlaku",
    );
  });

  it("sesi tanpa klaim jatuh ke matriks bawaan peran", () => {
    assert.equal(can({ role: "ADMIN_SISTEM" }, "pengguna.kelola"), true);
    assert.equal(
      can({ role: "ADMIN_SISTEM", permissions: undefined }, "pengguna.kelola"),
      true,
    );
    assert.equal(
      can({ role: "ADMIN_SISTEM", permissions: null }, "pengguna.kelola"),
      true,
    );
    assert.equal(can({ role: "OPS_MAHASISWA" }, "pengguna.kelola"), false);
  });

  it("ADMIN_SISTEM tanpa klaim memegang seluruh katalog", () => {
    for (const izin of PERMISSIONS) {
      assert.equal(can("ADMIN_SISTEM", izin), true, izin);
      assert.equal(can({ role: "ADMIN_SISTEM" }, izin), true, izin);
    }
  });

  it("sesi impersonasi memakai klaim sasaran, bukan katalog pelaku", () => {
    const sasaran: PemegangIzin = {
      role: "OPS_MAHASISWA",
      permissions: [...ROLE_PERMISSIONS.OPS_MAHASISWA],
    };
    assert.equal(can(sasaran, "harga.input"), true);
    assert.equal(can(sasaran, "pengguna.kelola"), false);
    assert.equal(can(sasaran, "impersonasi.kelola"), false);
  });

  it("klaim kosong berarti sungguh tanpa izin, bukan 'tidak ada klaim'", () => {
    const tanpaIzin: PemegangIzin = { role: "ADMIN_SISTEM", permissions: [] };
    assert.equal(can(tanpaIzin, "pengguna.kelola"), false);
    assert.equal(can(tanpaIzin, "audit.lihat"), false);
  });

  it("klaim yang sama dipakai ulang tanpa menyalahgunakan cache antar-larik", () => {
    const a = ["kpi.lihat"];
    const b = ["audit.lihat"];
    assert.equal(
      can({ role: "OPS_MAHASISWA", permissions: a }, "kpi.lihat"),
      true,
    );
    assert.equal(
      can({ role: "OPS_MAHASISWA", permissions: b }, "kpi.lihat"),
      false,
    );
    assert.equal(
      can({ role: "OPS_MAHASISWA", permissions: a }, "audit.lihat"),
      false,
    );
  });

  it("canAll mengikuti subjek yang sama", () => {
    const sesi: PemegangIzin = {
      role: "OPS_MAHASISWA",
      permissions: ["harga.input", "kpi.lihat"],
    };
    assert.equal(canAll(sesi, ["harga.input", "kpi.lihat"]), true);
    assert.equal(canAll(sesi, ["harga.input", "audit.lihat"]), false);
    assert.equal(
      canAll("ADMIN_SISTEM", ["pengguna.kelola", "audit.lihat"]),
      true,
    );
  });

  it("canAccessRoute memakai izin efektif sesi", () => {
    const biasa: PemegangIzin = {
      role: "OPS_MAHASISWA",
      permissions: [...ROLE_PERMISSIONS.OPS_MAHASISWA],
    };
    const dapatKpi: PemegangIzin = {
      role: "OPS_MAHASISWA",
      permissions: [...ROLE_PERMISSIONS.OPS_MAHASISWA, "kpi.lihat"],
    };
    assert.equal(canAccessRoute(biasa, "/kpi"), false);
    assert.equal(canAccessRoute(dapatKpi, "/kpi"), true);
    assert.equal(canAccessRoute(dapatKpi, "/kpi/detail"), true);
    assert.equal(canAccessRoute(dapatKpi, "/pengguna"), false);
    assert.equal(
      canAccessRoute("OPS_MAHASISWA", "/kpi"),
      false,
      "peran saja: matriks bawaan",
    );
    assert.equal(canAccessRoute(biasa, "/halaman-tanpa-aturan"), true);
  });

  it("/impersonasi mengikuti izin impersonasi.kelola, bukan pengguna.kelola", () => {
    const adminTanpa: PemegangIzin = {
      role: "ADMIN_SISTEM",
      permissions: ["pengguna.kelola"],
    };
    const adminDengan: PemegangIzin = {
      role: "ADMIN_SISTEM",
      permissions: ["pengguna.kelola", "impersonasi.kelola"],
    };
    assert.equal(canAccessRoute(adminTanpa, "/pengguna"), true);
    assert.equal(canAccessRoute(adminTanpa, "/impersonasi"), false);
    assert.equal(canAccessRoute(adminDengan, "/impersonasi"), true);
    assert.equal(canAccessRoute("ADMIN_SISTEM", "/impersonasi"), true);
    assert.equal(canAccessRoute("MANAJER_PROGRAM", "/impersonasi"), false);
  });
});
