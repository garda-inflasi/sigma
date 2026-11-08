/**
 * Tes komponen (bukan tes logika .ts): nav.ts mengimpor ikon lucide-react, yang membutuhkan React versi klien
 * dan gagal dimuat di bawah `--conditions=react-server` milik `pnpm test:unit`.
 */
import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { NAV_FLAT, activeNav, judulHalaman } from "./nav";

describe("activeNav", () => {
  it("antrean perubahan master menyala sendiri, bukan 'Master data'", () => {
    assert.equal(
      activeNav("/master/perubahan")?.label,
      "Antrean perubahan master",
    );
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

  it("antrean perubahan ada di navigasi dengan lencana master dan izin master.lihat", () => {
    const item = NAV_FLAT.find((i) => i.href === "/master/perubahan");
    assert.equal(item?.badge, "master");
    assert.equal(item?.perm, "master.lihat");
  });

  it("path anak dan halaman lain tetap benar", () => {
    assert.equal(activeNav("/data/harga")?.href, "/data/harga");
    assert.equal(activeNav("/data/harga/123")?.href, "/data/harga");
    assert.equal(activeNav("/audit")?.href, "/audit");
    assert.equal(activeNav("/tidak/ada"), undefined);
  });
});

describe("judulHalaman", () => {
  it("halaman dengan item nav memakai labelnya", () => {
    assert.equal(judulHalaman("/audit"), "Audit trail");
    assert.equal(judulHalaman("/master/perubahan"), "Antrean perubahan master");
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
