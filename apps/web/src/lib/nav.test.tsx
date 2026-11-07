/**
 * Tes komponen (bukan tes logika .ts): nav.ts mengimpor ikon lucide-react, yang membutuhkan React versi klien
 * dan gagal dimuat di bawah `--conditions=react-server` milik `pnpm test:unit`.
 */
import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { NAV_FLAT, activeNav } from "./nav";

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
