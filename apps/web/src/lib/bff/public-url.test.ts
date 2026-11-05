import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { urlPublik } from "./public-url";

const dari = (h: Record<string, string>) => ({
  headers: { get: (n: string) => h[n.toLowerCase()] ?? null },
});

describe("urlPublik", () => {
  it("memakai host dan protokol yang diteruskan proxy", () => {
    assert.equal(
      urlPublik(
        dari({
          "x-forwarded-host": "app.garda.id",
          "x-forwarded-proto": "https",
          host: "web:3000",
        }),
        "/masuk?lanjut=%2Fx",
      ),
      "https://app.garda.id/masuk?lanjut=%2Fx",
    );
  });

  it("tanpa x-forwarded-host memakai Host", () => {
    assert.equal(
      urlPublik(dari({ host: "localhost" }), "/kpi"),
      "http://localhost/kpi",
    );
  });

  it("tanpa header apa pun jatuh ke localhost http", () => {
    assert.equal(urlPublik(dari({}), "/"), "http://localhost/");
  });
});
