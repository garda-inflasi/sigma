import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, it } from "node:test";

import {
  adalahKodeGalatSso,
  hrefMasukGoogle,
  kodeGalatSsoDikenal,
  pesanGalatSso,
  SSO_GOOGLE_CALLBACK_PATH,
  SSO_GOOGLE_START_PATH,
} from "./sso";
import { galatSsoDariApi } from "../bff/sso";

describe("pesanGalatSso", () => {
  it("memberi pesan terbaca untuk setiap kode yang dikenal", () => {
    for (const kode of kodeGalatSsoDikenal()) {
      const pesan = pesanGalatSso(kode);
      assert.ok(pesan && pesan.length > 10, kode);
      assert.ok(
        !pesan.includes("sso_"),
        `pesan ${kode} tidak boleh memuat kode mentah`,
      );
    }
  });

  it("bukan kode SSO -> null supaya perilaku lama (pesan apa adanya) tetap berlaku", () => {
    assert.equal(pesanGalatSso("Email atau password salah"), null);
    assert.equal(pesanGalatSso(""), null);
    assert.equal(pesanGalatSso(undefined), null);
    assert.equal(pesanGalatSso(null), null);
  });

  it("kode sso_* tak dikenal diberi pesan umum, bukan teks yang dikirim penyerang", () => {
    const umum = pesanGalatSso("sso_gagal");
    assert.equal(pesanGalatSso("sso_kode_karangan"), umum);
    assert.equal(pesanGalatSso("sso_<script>alert(1)</script>"), umum);
    assert.equal(pesanGalatSso("sso___proto__"), umum);
    assert.equal(pesanGalatSso("sso_constructor"), umum);
  });

  it("adalahKodeGalatSso hanya mengenali awalan sso_", () => {
    assert.equal(adalahKodeGalatSso("sso_gagal"), true);
    assert.equal(adalahKodeGalatSso("SSO_GAGAL"), false);
    assert.equal(adalahKodeGalatSso("gagal"), false);
    assert.equal(adalahKodeGalatSso(undefined), false);
  });
});

describe("hrefMasukGoogle", () => {
  it("tanpa tujuan khusus -> rute awal polos", () => {
    assert.equal(hrefMasukGoogle("/"), SSO_GOOGLE_START_PATH);
    assert.equal(hrefMasukGoogle(""), SSO_GOOGLE_START_PATH);
  });

  it("membawa lanjut yang di-encode", () => {
    assert.equal(
      hrefMasukGoogle("/harga?wilayah=1171&x=a b"),
      `${SSO_GOOGLE_START_PATH}?lanjut=${encodeURIComponent("/harga?wilayah=1171&x=a b")}`,
    );
  });

  it("rute callback konsisten dengan prefiks publik edge guard dan prodcheck API", () => {
    assert.equal(SSO_GOOGLE_CALLBACK_PATH, "/api/auth/sso/google/callback");
    assert.ok(SSO_GOOGLE_CALLBACK_PATH.startsWith(`${SSO_GOOGLE_START_PATH}/`));
  });
});

describe("kontrak kode galat dengan API Go", () => {
  const sumberGo = resolve(
    process.cwd(),
    "../api/internal/auth/delivery/http/handler_sso.go",
  );

  it(
    "setiap kode SSO_* di handler Go punya pemetaan BFF ke pesan khusus",
    { skip: !existsSync(sumberGo) },
    () => {
      const kode = [
        ...readFileSync(sumberGo, "utf8").matchAll(/"(SSO_[A-Z_]+)"/g),
      ].map((m) => m[1]);
      assert.ok(
        kode.length >= 10,
        "kode SSO_* tidak ditemukan di handler_sso.go",
      );
      const umum = pesanGalatSso("sso_gagal");
      for (const k of new Set(kode)) {
        const dipetakan = galatSsoDariApi({ status: 400, code: k });
        assert.notEqual(
          dipetakan,
          "sso_gagal",
          `${k} belum dipetakan di lib/bff/sso.ts`,
        );
        const pesan = pesanGalatSso(dipetakan);
        assert.ok(pesan, `${k} -> ${dipetakan} tanpa pesan`);
        assert.notEqual(
          pesan,
          umum,
          `${k} -> ${dipetakan} jatuh ke pesan umum`,
        );
      }
    },
  );
});
