import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  getRedirectStatusCodeFromError,
  getRedirectTypeFromError,
  getURLFromRedirectError,
} from "next/dist/client/components/redirect";
import { isRedirectError } from "next/dist/client/components/redirect-error";
import KeamananPage from "./page";

describe("/keamanan", () => {
  it("dialihkan di server ke tab 'Perangkat & Sesi Aktif' di /profil (K2)", () => {
    let galat: unknown;
    try {
      KeamananPage();
    } catch (e) {
      galat = e;
    }

    assert.ok(isRedirectError(galat), "halaman melempar NEXT_REDIRECT");
    assert.equal(getURLFromRedirectError(galat), "/profil?tab=keamanan");
    assert.equal(getRedirectTypeFromError(galat), "replace");
    assert.equal(getRedirectStatusCodeFromError(galat), 307);
  });
});
