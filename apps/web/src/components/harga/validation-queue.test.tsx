import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { screen, within } from "@testing-library/react";
import { amplop, galatApi, pasangFetchPalsu, renderHalaman, type FetchPalsu } from "@/test/render";
import { ValidationQueue } from "./validation-queue";

const HARGA = "/v1/harga";

let api: FetchPalsu;
afterEach(() => api?.pulihkan());

describe("ValidationQueue: keadaan kosong dan galat", () => {
  it("antrean kosong yang sah: ajakan bertindak, tanpa panel galat", async () => {
    api = pasangFetchPalsu({ [`GET ${HARGA}`]: amplop([]) });
    renderHalaman(<ValidationQueue />, { url: "/operasional" });

    assert.ok(await screen.findByText("Antrean validasi kosong"));
    assert.equal(screen.queryByRole("alert"), null);
    assert.equal(api.ke("GET", HARGA)[0].query.get("status"), "DIAJUKAN");
  });

  it("galat: panel galat, bukan 'Antrean validasi kosong; semua entri sudah diverifikasi' yang keliru; coba lagi memuat ulang", async () => {
    let panggilan = 0;
    api = pasangFetchPalsu({ [`GET ${HARGA}`]: () => (++panggilan === 1 ? galatApi(500, "Gagal mengambil entri harga") : amplop([])) });
    const { user } = renderHalaman(<ValidationQueue />, { url: "/operasional" });

    const alert = await screen.findByRole("alert");
    assert.match(alert.textContent ?? "", /Antrean validasi gagal dimuat/);
    assert.match(alert.textContent ?? "", /Gagal mengambil entri harga/);
    assert.equal(screen.queryByText("Antrean validasi kosong"), null);
    assert.ok(screen.getByRole("link", { name: "Buka semua" }), "judul dan tautan panel tetap ada");

    await user.click(within(alert).getByRole("button", { name: /Coba lagi/ }));
    assert.ok(await screen.findByText("Antrean validasi kosong"));
    assert.equal(screen.queryByRole("alert"), null);
  });
});
