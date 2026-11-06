import assert from "node:assert/strict";
import { afterEach, before, beforeEach, describe, it } from "node:test";

let validateRedirectUrl: (typeof import("./safe-redirect"))["validateRedirectUrl"];
let getSafeRedirectUrl: (typeof import("./safe-redirect"))["getSafeRedirectUrl"];

const APP_URL_ASLI = process.env.NEXT_PUBLIC_APP_URL;

before(async () => {
  ({ validateRedirectUrl, getSafeRedirectUrl } =
    await import("./safe-redirect"));
});

beforeEach(() => {
  process.env.NEXT_PUBLIC_APP_URL = "https://garda.example";
});

afterEach(() => {
  if (APP_URL_ASLI === undefined) delete process.env.NEXT_PUBLIC_APP_URL;
  else process.env.NEXT_PUBLIC_APP_URL = APP_URL_ASLI;
});

describe("validateRedirectUrl: path internal", () => {
  for (const aman of [
    "/",
    "/dashboard",
    "/harga/entri?tanggal=2026-01-01#atas",
    "/ganti-password",
  ]) {
    it(`meloloskan ${aman}`, () => {
      assert.equal(validateRedirectUrl(aman), aman);
    });
  }

  it("kosong atau null tidak punya tujuan", () => {
    assert.equal(validateRedirectUrl(null), null);
    assert.equal(validateRedirectUrl(""), null);
  });
});

// Regresi: kondisi lama (`includes("//") && !startsWith("//")`) justru meloloskan `//evil.com`, sehingga
// /mfa?lanjut=//evil.com dan /ganti-password?lanjut=//evil.com mengarahkan pengguna keluar situs setelah
// formulir selesai (window.location.assign(lanjut)).
describe("validateRedirectUrl: pengalihan keluar origin ditolak", () => {
  const berbahaya: Array<[string, string]> = [
    ["//evil.com", "protocol-relative"],
    ["//evil.com/masuk", "protocol-relative dengan path"],
    ["///evil.com", "tiga garis miring"],
    ["/\\evil.com", "garis miring terbalik (peramban menganggapnya '/')"],
    ["/\\/evil.com", "campuran"],
    ["/aman\\..\\evil", "garis miring terbalik di tengah path"],
    ["/\t/evil.com", "tab dibuang peramban sehingga menjadi //evil.com"],
    ["/\n/evil.com", "newline dibuang peramban"],
    ["/\r/evil.com", "carriage return dibuang peramban"],
    ["/\u0000/evil.com", "NUL"],
    ["/\u007f/evil.com", "DEL"],
    ["/halaman//lain", "garis miring ganda di tengah path"],
    ["https://evil.com", "URL absolut origin lain"],
    ["http://garda.example.evil.com/", "domain yang hanya diawali host sah"],
    ["https://garda.example@evil.com/", "userinfo menyamar sebagai host"],
    ["https://evil.com#https://garda.example", "origin sah hanya di fragmen"],
    ["javascript:alert(1)", "javascript:"],
    ["data:text/html,<script>alert(1)</script>", "data:"],
    ["evil.com/path", "tanpa garis miring depan dan bukan URL absolut"],
    ["\\\\evil.com", "garis miring terbalik di depan"],
  ];
  for (const [url, alasan] of berbahaya) {
    it(`menolak ${JSON.stringify(url)} (${alasan})`, () => {
      assert.equal(validateRedirectUrl(url), null);
    });
  }
});

describe("validateRedirectUrl: URL absolut", () => {
  it("meloloskan origin yang sama dengan NEXT_PUBLIC_APP_URL", () => {
    assert.equal(
      validateRedirectUrl("https://garda.example/dashboard"),
      "https://garda.example/dashboard",
    );
  });

  it("origin dengan skema atau port berbeda dianggap origin lain", () => {
    assert.equal(validateRedirectUrl("http://garda.example/x"), null);
    assert.equal(validateRedirectUrl("https://garda.example:8443/x"), null);
  });

  it("tanpa NEXT_PUBLIC_APP_URL, bawaan hanya http://localhost:3000", () => {
    delete process.env.NEXT_PUBLIC_APP_URL;
    assert.equal(
      validateRedirectUrl("http://localhost:3000/x"),
      "http://localhost:3000/x",
    );
    assert.equal(validateRedirectUrl("https://garda.example/x"), null);
  });

  it("NEXT_PUBLIC_APP_URL yang rusak tidak membuka pengalihan (gagal tertutup)", () => {
    process.env.NEXT_PUBLIC_APP_URL = "bukan url";
    assert.equal(validateRedirectUrl("https://garda.example/x"), null);
  });
});

describe("getSafeRedirectUrl", () => {
  it("membaca parameter bernama dan memakainya bila aman", () => {
    const params = new URLSearchParams({ lanjut: "/harga" });
    assert.equal(getSafeRedirectUrl(params, "lanjut"), "/harga");
  });

  it("parameter bawaan bernama 'redirect' dan bawaan fallback '/'", () => {
    assert.equal(
      getSafeRedirectUrl(new URLSearchParams({ redirect: "/kpi" })),
      "/kpi",
    );
    assert.equal(getSafeRedirectUrl(new URLSearchParams()), "/");
  });

  it("jatuh ke fallback untuk nilai berbahaya, bukan meneruskannya", () => {
    const params = new URLSearchParams({ redirect: "//evil.com" });
    assert.equal(getSafeRedirectUrl(params), "/");
    assert.equal(getSafeRedirectUrl(params, "redirect", "/masuk"), "/masuk");
  });
});
