import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { activeNav, NAV_FLAT, remahRoti } from "@/lib/nav";
import { ENTITAS } from "@/lib/data/master";
import { getCrawlerFreshness, getPipelineJobs } from "@/lib/api/pipeline-jobs";

describe("navigasi, master entitas, & pipeline jobs", () => {
  it("mendaftarkan menu Pipeline & Crawler (/crawler) di grup Data", () => {
    const item = activeNav("/crawler", NAV_FLAT);
    assert.equal(item?.href, "/crawler");
    assert.equal(item?.label, "Pipeline & Crawler");
    assert.deepEqual(remahRoti("/crawler"), ["Data", "Pipeline & Crawler"]);
  });

  it("menyediakan entitas sentra pertanian dan perkampungan di master", () => {
    assert.equal(ENTITAS.pertanian, "Sentra pertanian");
    assert.equal(ENTITAS.perkampungan, "Perkampungan");
  });

  it("memuat endpoint freshness & jobs snapshot dari API", async () => {
    const origFetch = globalThis.fetch;
    globalThis.fetch = (async (url: string | URL) => {
      const s = String(url);
      if (s.includes("/v1/ml/freshness")) {
        return new Response(
          JSON.stringify({
            code: "200",
            message: "OK",
            data: {
              tersedia: true,
              dihitung_pada: "2026-11-24T04:00:00Z",
              ringkasan: { segar: 4, basi: 1, gagal: 1, dilewati: 0, belum_ada_run_terjadwal: 0 },
              sumber: [{ sumber: "SP2KP", dataset: "harga_konsumen", kategori: "GAGAL", status: "ERROR", pesan: "Timeout", last_available_date: "2026-11-21", umur_hari: 3, run_terakhir: null }],
              sinyal_sumber: [],
            },
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        );
      }
      return new Response(
        JSON.stringify({
          code: "200",
          message: "OK",
          data: {
            timestamp: "2026-11-24T04:00:00Z",
            crawler: { configured: true, reachable: true, berjalan: false, progress_pct: 100, logs: [] },
            pipeline: { configured: true, reachable: true, berjalan: false, progress_pct: 100, logs: [] },
            chain: { aktif: false, tahap: "SIAGA", progress_pct: 0, logs: [] },
          },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }) as typeof fetch;

    try {
      const f = await getCrawlerFreshness();
      assert.equal(f.tersedia, true);
      assert.equal(f.sumber[0]?.sumber, "SP2KP");
      const j = await getPipelineJobs();
      assert.equal(j.crawler.berjalan, false);
      assert.equal(j.chain.tahap, "SIAGA");
    } finally {
      globalThis.fetch = origFetch;
    }
  });
});
