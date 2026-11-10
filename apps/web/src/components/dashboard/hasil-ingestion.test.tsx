import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { screen, within } from "@testing-library/react";
import { muatFixture } from "@/lib/api/__fixtures__/muat";
import type { CuacaResponse, PembandingHargaResponse } from "@/lib/api/dashboard";
import { amplop, galatApi, pasangFetchPalsu, renderHalaman, type FetchPalsu } from "@/test/render";
import { CuacaWilayahCard, PembandingHargaCard } from "./hasil-ingestion";

const PEMBANDING = "/v1/dashboard/operasional/pembanding-harga";
const CUACA = "/v1/dashboard/operasional/cuaca";

let api: FetchPalsu;
afterEach(() => api?.pulihkan());

describe("PembandingHargaCard", () => {
  it("respons nyata provinsi: ringkasan kesepakatan dan satu baris per pasangan", async () => {
    const data = muatFixture<PembandingHargaResponse>("provinsi", "pembanding-harga");
    api = pasangFetchPalsu({ [`GET ${PEMBANDING}`]: amplop(data) });
    renderHalaman(<PembandingHargaCard />, { url: "/operasional" });

    assert.ok(await screen.findByText(/11 dari 16 pasangan sepakat \(selisih ≤ 10%\), 7 hari terakhir, termasuk data contoh/));
    assert.equal(screen.getAllByRole("row").length, data.items.length + 1, "baris data ditambah baris judul");
    assert.ok(screen.getByText("Rp63.900"));
    assert.equal(screen.queryByRole("alert"), null);
  });

  it("kosong yang sah: ajakan menjalankan ingestion, bukan galat", async () => {
    api = pasangFetchPalsu({ [`GET ${PEMBANDING}`]: amplop({ pasangan: 0, sepakat: 0, batas_pct: 10, items: [] }) });
    renderHalaman(<PembandingHargaCard />, { url: "/operasional" });
    assert.ok(await screen.findByText(/Belum ada pasangan harga lapangan dan eksternal/));
    assert.equal(screen.queryByRole("alert"), null);
  });

  it("galat: panel galat, bukan ajakan menjalankan ingestion yang menyesatkan", async () => {
    api = pasangFetchPalsu({ [`GET ${PEMBANDING}`]: galatApi(500, "Gagal mengambil pembanding harga") });
    renderHalaman(<PembandingHargaCard />, { url: "/operasional" });

    const alert = await screen.findByRole("alert");
    assert.match(alert.textContent ?? "", /Validasi silang gagal dimuat/);
    assert.equal(screen.queryByText(/Belum ada pasangan harga/), null);
    assert.ok(within(alert).getByRole("button", { name: /Coba lagi/ }));
  });
});

describe("CuacaWilayahCard", () => {
  it("respons nyata provinsi: satu baris per kabupaten/kota, hujan lebat dihitung", async () => {
    const data = muatFixture<CuacaResponse[]>("provinsi", "cuaca");
    api = pasangFetchPalsu({ [`GET ${CUACA}`]: amplop(data) });
    renderHalaman(<CuacaWilayahCard />, { url: "/operasional" });

    const anomali = data.filter((c) => c.anomali).length;
    assert.ok(await screen.findByText(new RegExp(`${anomali} wilayah hujan lebat \\(≥ 50 mm/hari\\), termasuk data contoh`)));
    assert.equal(screen.getAllByRole("listitem").length, data.length);
    assert.ok(screen.getByText("Banda Aceh"));
    assert.equal(screen.queryByRole("alert"), null);
  });

  it("kosong yang sah: ajakan menjalankan ingestion BMKG", async () => {
    api = pasangFetchPalsu({ [`GET ${CUACA}`]: amplop([]) });
    renderHalaman(<CuacaWilayahCard />, { url: "/operasional" });
    assert.ok(await screen.findByText(/Belum ada data cuaca/));
    assert.equal(screen.queryByRole("alert"), null);
  });

  it("galat: panel galat, bukan 'Belum ada data cuaca'", async () => {
    api = pasangFetchPalsu({ [`GET ${CUACA}`]: galatApi(500, "Gagal mengambil cuaca") });
    renderHalaman(<CuacaWilayahCard />, { url: "/operasional" });

    const alert = await screen.findByRole("alert");
    assert.match(alert.textContent ?? "", /Cuaca wilayah gagal dimuat/);
    assert.equal(screen.queryByText(/Belum ada data cuaca/), null);
  });
});
