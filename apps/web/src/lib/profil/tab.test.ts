import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { TAB_BAWAAN, TAB_PROFIL, bacaTabProfil, hrefTabProfil } from "./tab";

describe("bacaTabProfil", () => {
  it("menerima setiap tab yang dikenal", () => {
    for (const tab of TAB_PROFIL) assert.equal(bacaTabProfil(tab), tab);
  });

  it("nilai kosong, tidak dikenal, atau beda huruf jatuh ke tab bawaan", () => {
    for (const nilai of [
      null,
      undefined,
      "",
      "MFA",
      "sesi",
      "keamanan/",
      " mfa",
    ]) {
      assert.equal(bacaTabProfil(nilai), TAB_BAWAAN, String(nilai));
    }
  });
});

describe("hrefTabProfil", () => {
  it("tab bawaan tanpa query, tab lain dengan ?tab=", () => {
    assert.equal(hrefTabProfil("profil"), "/profil");
    assert.equal(hrefTabProfil("ganti-password"), "/profil?tab=ganti-password");
    assert.equal(hrefTabProfil("mfa"), "/profil?tab=mfa");
    assert.equal(hrefTabProfil("keamanan"), "/profil?tab=keamanan");
  });

  it("URL setiap tab terbaca kembali sebagai tab yang sama", () => {
    for (const tab of TAB_PROFIL) {
      const url = new URL(hrefTabProfil(tab), "http://localhost");
      assert.equal(bacaTabProfil(url.searchParams.get("tab")), tab);
    }
  });
});
