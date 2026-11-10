import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { screen, within } from "@testing-library/react";
import { muatFixture } from "@/lib/api/__fixtures__/muat";
import type { HeatmapResponse } from "@/lib/api/dashboard";
import {
  amplop,
  galatApi,
  pasangFetchPalsu,
  renderHalaman,
  type FetchPalsu,
} from "@/test/render";
import { PriceHeatmap } from "./price-heatmap";

const HEATMAP = "/v1/dashboard/operasional/heatmap";

let api: FetchPalsu;
afterEach(() => api?.pulihkan());

const sel = () => screen.queryAllByRole("button");

describe("PriceHeatmap dengan respons nyata", () => {
  it("provinsi: satu kolom per kabupaten, satu baris per komoditas, dan satu sel per data", async () => {
    const data = muatFixture<HeatmapResponse>("provinsi", "heatmap");
    api = pasangFetchPalsu({ [`GET ${HEATMAP}`]: amplop(data) });
    const { container } = renderHalaman(<PriceHeatmap />, {
      url: "/operasional",
    });

    assert.ok(
      document.querySelectorAll(".shimmer").length > 0,
      "kerangka selama memuat",
    );
    await screen.findByRole("button", { name: /Beras medium, Aceh Selatan/ });
    assert.equal(sel().length, data.sel.length, "satu tombol per sel");
    for (const kab of [
      "Aceh Selatan",
      "Aceh Timur",
      "Pidie",
      "Banda Aceh",
      "Lhokseumawe",
    ])
      assert.ok(within(container).getAllByTitle(kab).length > 0, kab);
    assert.equal(
      screen.getByRole("button", { name: "Beras medium, Aceh Selatan: +0,8%" })
        .textContent,
      "+0,8",
    );
    assert.equal(screen.queryByRole("alert"), null);
    assert.doesNotMatch(container.innerHTML, /repeat\(0/);
  });

  it("sel dari harga eksternal ditandai pada label aksesibilitasnya", async () => {
    const data = muatFixture<HeatmapResponse>("provinsi", "heatmap");
    api = pasangFetchPalsu({ [`GET ${HEATMAP}`]: amplop(data) });
    renderHalaman(<PriceHeatmap />, { url: "/operasional" });

    await screen.findAllByRole("button");
    assert.equal(
      screen.getAllByRole("button", { name: /\(harga eksternal\)/ }).length,
      data.sel.filter((s) => s.sumber === "EKSTERNAL").length,
    );
  });

  it("memilih Pilot menyisakan hanya baris komoditas pilot", async () => {
    const data = muatFixture<HeatmapResponse>("provinsi", "heatmap");
    api = pasangFetchPalsu({ [`GET ${HEATMAP}`]: amplop(data) });
    const { user } = renderHalaman(<PriceHeatmap />, { url: "/operasional" });
    await screen.findAllByRole("button");

    await user.click(screen.getByRole("tab", { name: "Pilot" }));
    const pilot = new Set(
      data.komoditas.filter((k) => k.pilot).map((k) => k.kode),
    );
    assert.equal(
      sel().length,
      data.sel.filter((s) => pilot.has(s.komoditas_kode)).length,
    );
    assert.ok(pilot.size > 0 && pilot.size < data.komoditas.length);
  });

  it("Koordinator: satu kolom (kabupaten tugasnya) dan empat belas sel", async () => {
    api = pasangFetchPalsu({
      [`GET ${HEATMAP}`]: amplop(
        muatFixture<HeatmapResponse>("koordinator", "heatmap"),
      ),
    });
    renderHalaman(<PriceHeatmap />, { url: "/operasional" });
    await screen.findAllByRole("button");
    assert.equal(sel().length, 14);
    assert.ok(screen.getByRole("button", { name: /Beras medium, Banda Aceh/ }));
  });

  it("menyorot sel menampilkan harga dan perubahan (bacaan di atas grid)", async () => {
    api = pasangFetchPalsu({
      [`GET ${HEATMAP}`]: amplop(
        muatFixture<HeatmapResponse>("koordinator", "heatmap"),
      ),
    });
    const { user } = renderHalaman(<PriceHeatmap />, { url: "/operasional" });
    await user.hover(
      await screen.findByRole("button", { name: /Beras medium, Banda Aceh/ }),
    );
    assert.match(
      document.querySelector("[aria-live=polite]")?.textContent ?? "",
      /Beras medium di Banda Aceh: Rp14\.300\/kg, \+1,0%/,
    );
  });
});

describe("PriceHeatmap: kosong dan galat", () => {
  it("cakupan tanpa data (jawaban nyata untuk kabupaten di luar cakupan): keterangan kosong, bukan grid tanpa kolom", async () => {
    api = pasangFetchPalsu({
      [`GET ${HEATMAP}`]: amplop(
        muatFixture<HeatmapResponse>("koordinator", "heatmap-kosong"),
      ),
    });
    const { container } = renderHalaman(<PriceHeatmap />, {
      url: "/operasional",
    });

    assert.ok(
      await screen.findByText(
        "Belum ada harga tervalidasi untuk dipetakan di wilayah ini.",
      ),
    );
    assert.equal(sel().length, 0);
    assert.equal(screen.queryByRole("alert"), null, "kosong bukan galat");
    assert.doesNotMatch(
      container.innerHTML,
      /repeat\(0/,
      "CSS repeat(0, ...) tidak sah",
    );
    assert.ok(screen.getByText("Turun tajam"), "legenda tetap ada");
  });

  it("memilih Pilot pada data tanpa komoditas pilot: keterangan tersendiri", async () => {
    const data = muatFixture<HeatmapResponse>("koordinator", "heatmap");
    data.komoditas = data.komoditas.map((k) => ({ ...k, pilot: false }));
    api = pasangFetchPalsu({ [`GET ${HEATMAP}`]: amplop(data) });
    const { user } = renderHalaman(<PriceHeatmap />, { url: "/operasional" });
    await screen.findAllByRole("button");

    await user.click(screen.getByRole("tab", { name: "Pilot" }));
    assert.ok(screen.getByText("Tidak ada komoditas pilot pada data ini."));
    assert.equal(sel().length, 0);
  });

  it("galat: panel galat dengan tombol coba lagi; coba lagi memuat ulang", async () => {
    let panggilan = 0;
    api = pasangFetchPalsu({
      [`GET ${HEATMAP}`]: () =>
        ++panggilan === 1
          ? galatApi(500, "Gagal mengambil data heatmap")
          : amplop(muatFixture<HeatmapResponse>("koordinator", "heatmap")),
    });
    const { user } = renderHalaman(<PriceHeatmap />, { url: "/operasional" });

    const alert = await screen.findByRole("alert");
    assert.match(alert.textContent ?? "", /Peta panas harga gagal dimuat/);
    assert.match(alert.textContent ?? "", /Gagal mengambil data heatmap/);
    assert.equal(document.querySelectorAll(".shimmer").length, 0);

    await user.click(within(alert).getByRole("button", { name: /Coba lagi/ }));
    await screen.findAllByRole("button", { name: /Beras medium/ });
    assert.equal(screen.queryByRole("alert"), null);
  });
});
