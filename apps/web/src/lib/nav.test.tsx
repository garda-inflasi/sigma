/**
 * Tes komponen (bukan tes logika .ts): nav.ts mengimpor ikon lucide-react, yang membutuhkan React versi klien
 * dan gagal dimuat di bawah `--conditions=react-server` milik `pnpm test:unit`.
 */
import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  NAV,
  NAV_FLAT,
  NAV_PINTASAN,
  activeNav,
  judulHalaman,
  remahRoti,
} from "./nav";
import { can } from "./rbac/permissions";
import { canAccessRoute } from "./rbac/routes";

describe("activeNav", () => {
  it("antrean perubahan menyalakan 'Master data' karena tidak lagi punya item sidebar", () => {
    assert.equal(activeNav("/master/perubahan")?.label, "Master data");
    assert.equal(activeNav("/master/perubahan/12")?.label, "Master data");
  });

  it("tab entitas master menyalakan 'Master data'", () => {
    for (const e of [
      "komoditas",
      "wilayah",
      "pasar",
      "kecamatan",
      "satuan",
      "pedagang",
    ]) {
      assert.equal(activeNav(`/master/${e}`)?.label, "Master data", e);
    }
  });

  it("sidebar tidak memuat antrean; lencana usulan pindah ke 'Master data' dan antrean tetap bisa dicari", () => {
    assert.equal(
      NAV_FLAT.some((i) => i.href === "/master/perubahan"),
      false,
    );
    assert.deepEqual(
      NAV.find((g) => g.grup === "Data")?.items.map((i) => i.label),
      ["Validasi harga", "Sumber data & API", "Pipeline & Crawler", "Master data"],
    );
    const master = NAV_FLAT.find((i) => i.href === "/master/komoditas");
    assert.equal(master?.badge, "master");
    assert.equal(master?.perm, "master.lihat");
    const pintasan = NAV_PINTASAN.find((i) => i.href === "/master/perubahan");
    assert.equal(pintasan?.perm, "master.lihat");
  });

  it("remah roti antrean perubahan berada di bawah Master data", () => {
    assert.deepEqual(remahRoti("/master/perubahan"), [
      "Data",
      "Master data",
      "Antrean perubahan",
    ]);
    assert.deepEqual(remahRoti("/master/pasar"), ["Data", "Master data"]);
  });

  it("path anak dan halaman lain tetap benar", () => {
    assert.equal(activeNav("/data/harga")?.href, "/data/harga");
    assert.equal(activeNav("/data/harga/123")?.href, "/data/harga");
    assert.equal(activeNav("/audit")?.href, "/audit");
    assert.equal(activeNav("/tidak/ada"), undefined);
  });
});

describe("item nav Impersonasi", () => {
  it("ada di grup Tata kelola, dijaga izin impersonasi.kelola, dan menyala sendiri (bukan 'Pengguna & peran')", () => {
    const item = NAV_FLAT.find((i) => i.href === "/impersonasi");
    assert.equal(item?.label, "Impersonasi");
    assert.equal(item?.perm, "impersonasi.kelola");
    assert.equal(NAV.find((g) => g.items.includes(item!))?.grup, "Tata kelola");
    assert.equal(activeNav("/impersonasi")?.href, "/impersonasi");
    assert.equal(activeNav("/pengguna")?.href, "/pengguna");
    assert.equal(judulHalaman("/impersonasi"), "Impersonasi");
    assert.deepEqual(remahRoti("/impersonasi"), ["Tata kelola", "Impersonasi"]);
  });
});

describe("menu ADMIN_SISTEM", () => {
  it("seluruh menu konsol, lapangan, dan pintasan terbuka", () => {
    for (const item of [...NAV_FLAT, ...NAV_PINTASAN]) {
      assert.ok(!item.perm || can("ADMIN_SISTEM", item.perm), item.href);
      assert.ok(canAccessRoute("ADMIN_SISTEM", item.href), item.href);
    }
    assert.ok(NAV_FLAT.some((i) => i.href === "/lapangan/input"));
  });
});

describe("judulHalaman", () => {
  it("halaman dengan item nav memakai labelnya", () => {
    assert.equal(judulHalaman("/audit"), "Audit trail");
    assert.equal(judulHalaman("/master/perubahan"), "Antrean perubahan");
    assert.equal(judulHalaman("/data/harga/123"), "Validasi harga");
  });

  it("/profil (dibuka dari menu akun, tanpa item sidebar) berjudul 'Profil saya'", () => {
    assert.equal(judulHalaman("/profil"), "Profil saya");
    assert.equal(judulHalaman("/profil/apa-saja"), "Profil saya");
  });

  it("/profil tidak menjadi item sidebar dan tidak menyalakan item nav mana pun", () => {
    assert.equal(
      NAV_FLAT.some((i) => i.href === "/profil"),
      false,
    );
    assert.equal(activeNav("/profil"), undefined);
  });

  it("path yang hanya mirip /profil dan halaman tak dikenal memakai nama aplikasi", () => {
    assert.equal(judulHalaman("/profilx"), "Garda Inflasi");
    assert.equal(judulHalaman("/tidak/ada"), "Garda Inflasi");
    assert.equal(judulHalaman("/"), "Garda Inflasi");
  });
});
