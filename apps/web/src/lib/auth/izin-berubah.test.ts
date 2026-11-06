import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { izinBerubah, izinDalamToken } from "./izin-berubah";

function jwt(payload: object): string {
  const b64 = (o: object) =>
    Buffer.from(JSON.stringify(o)).toString("base64url");
  return `${b64({ alg: "HS256" })}.${b64(payload)}.tanda-tangan`;
}

describe("izinDalamToken", () => {
  it("membaca klaim permissions", () => {
    assert.deepEqual(izinDalamToken(jwt({ permissions: ["a.b", "c.d"] })), [
      "a.b",
      "c.d",
    ]);
    assert.deepEqual(izinDalamToken(jwt({ permissions: [] })), []);
  });

  it("null untuk token rusak, tanpa klaim, atau klaim bukan larik string", () => {
    assert.equal(izinDalamToken(undefined), null);
    assert.equal(izinDalamToken("bukan-jwt"), null);
    assert.equal(izinDalamToken(jwt({ role: "ADMIN_SISTEM" })), null);
    assert.equal(izinDalamToken(jwt({ permissions: "a.b" })), null);
    assert.equal(izinDalamToken(jwt({ permissions: ["a.b", 1] })), null);
  });
});

describe("izinBerubah", () => {
  it("benar bila izin ditambah, dicabut, atau diganti", () => {
    const dasar = jwt({ permissions: ["harga.input", "tugas.lihat_sendiri"] });
    assert.equal(
      izinBerubah(
        dasar,
        jwt({
          permissions: ["harga.input", "tugas.lihat_sendiri", "kpi.lihat"],
        }),
      ),
      true,
    );
    assert.equal(
      izinBerubah(dasar, jwt({ permissions: ["harga.input"] })),
      true,
    );
    assert.equal(
      izinBerubah(dasar, jwt({ permissions: ["harga.input", "kpi.lihat"] })),
      true,
    );
    assert.equal(izinBerubah(dasar, jwt({ permissions: [] })), true);
  });

  it("salah bila himpunannya sama, apa pun urutannya", () => {
    const a = jwt({ permissions: ["a.b", "c.d"] });
    assert.equal(izinBerubah(a, jwt({ permissions: ["c.d", "a.b"] })), false);
    assert.equal(izinBerubah(a, a), false);
  });

  it("salah tanpa pembanding: token lama hilang atau salah satunya tanpa klaim", () => {
    const ada = jwt({ permissions: ["a.b"] });
    assert.equal(izinBerubah(undefined, ada), false);
    assert.equal(izinBerubah(ada, undefined), false);
    assert.equal(izinBerubah(ada, jwt({ role: "ADMIN_SISTEM" })), false);
  });
});
