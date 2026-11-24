import assert from "node:assert/strict";
import { afterEach, before, describe, it } from "node:test";
import { PREDIKSI_DETAIL_C05, SIKLUS_NYATA } from "./__fixtures__/ml/nyata";

// API_BASE dibaca saat modul dimuat, jadi harus diisi sebelum modul ml diimpor (pola ews-push.test.ts).
process.env.NEXT_PUBLIC_API_BASE_URL = "http://api.test";
let ml: typeof import("./ml");
before(async () => {
  ml = await import("./ml");
});

const fetchAsli = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = fetchAsli;
});

describe("akses API hasil training", () => {
  it("bacaLampiranMl mengirim cycle_id dan hanya mengembalikan `ml`; daftarSiklusMl memanggil /v1/ml/cycles", async () => {
    const url: string[] = [];
    globalThis.fetch = (async (u: RequestInfo | URL) => {
      url.push(String(u));
      const body = String(u).includes("/v1/ml/cycles")
        ? { code: "200", message: "OK", data: SIKLUS_NYATA }
        : {
            code: "200",
            message: "OK",
            data: { sumber: "DEMO" },
            ml: PREDIKSI_DETAIL_C05,
          };
      return new Response(JSON.stringify(body), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }) as typeof fetch;

    const l = await ml.bacaLampiranMl(
      "/v1/prediksi/CBM-KRT?kabupaten=1171",
      "C20261124T05Z",
    );
    assert.match(
      url[0],
      /\/v1\/prediksi\/CBM-KRT\?kabupaten=1171&cycle_id=C20261124T05Z$/,
    );
    assert.equal(l.forecast?.item?.komoditasKode, "CBM-KRT");
    assert.equal("data" in l, false);

    const d = await ml.daftarSiklusMl();
    assert.match(url[1], /\/v1\/ml\/cycles\?limit=24$/);
    assert.equal(d.items.length, 3);
  });
});
