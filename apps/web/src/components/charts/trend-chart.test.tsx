import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { screen, within } from "@testing-library/react";
import { amplop, galatApi, pasangFetchPalsu, renderHalaman, type FetchPalsu } from "@/test/render";
import { TrendChart } from "./trend-chart";

const TREN = "/v1/harga/tren";

let api: FetchPalsu;
afterEach(() => api?.pulihkan());

describe("TrendChart: keadaan kosong dan galat", () => {
  it("meminta tren komoditas dan kabupaten bawaan; kosong menampilkan keterangan tanpa panel galat", async () => {
    api = pasangFetchPalsu({ [`GET ${TREN}`]: amplop([]) });
    renderHalaman(<TrendChart />, { url: "/operasional" });

    assert.ok(await screen.findByText("Belum ada data tren harga tervalidasi untuk kombinasi ini."));
    assert.equal(screen.queryByRole("alert"), null);
    assert.equal(api.ke("GET", TREN)[0].query.get("komoditas"), "CBM-KRT");
    assert.equal(api.ke("GET", TREN)[0].query.get("wilayah"), "1171");
  });

  it("galat: panel galat (bukan 'belum ada data' yang menyesatkan) dengan tombol coba lagi", async () => {
    let panggilan = 0;
    api = pasangFetchPalsu({ [`GET ${TREN}`]: () => (++panggilan === 1 ? galatApi(500, "Gagal mengambil tren harga") : amplop([])) });
    const { user } = renderHalaman(<TrendChart />, { url: "/operasional" });

    const alert = await screen.findByRole("alert");
    assert.match(alert.textContent ?? "", /Tren harga gagal dimuat/);
    assert.match(alert.textContent ?? "", /Gagal mengambil tren harga/);
    assert.equal(screen.queryByText("Belum ada data tren harga tervalidasi untuk kombinasi ini."), null);
    assert.equal(document.querySelectorAll(".shimmer").length, 0);

    await user.click(within(alert).getByRole("button", { name: /Coba lagi/ }));
    assert.ok(await screen.findByText("Belum ada data tren harga tervalidasi untuk kombinasi ini."));
    assert.equal(screen.queryByRole("alert"), null);
  });
});
