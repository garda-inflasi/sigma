import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { screen, within } from "@testing-library/react";
import type { MarkerResponse } from "@/lib/api/spasial";
import { amplop, galatApi, pasangFetchPalsu, renderHalaman, type FetchPalsu } from "@/test/render";
import { OpsMap } from "./ops-map";

const WILAYAH = "/v1/spasial/wilayah";

let api: FetchPalsu;
afterEach(() => api?.pulihkan());

const marker = (sisip: Partial<MarkerResponse> = {}): MarkerResponse => ({
  kode: "1171",
  nama: "KOTA BANDA ACEH",
  lat: 5.55,
  lng: 95.32,
  delta: 12.4,
  komoditas: "Cabai merah keriting",
  status: "defisit",
  punya_pasar: true,
  ...sisip,
});

describe("OpsMap", () => {
  it("memuat kerangka lalu menampilkan penanda wilayah", async () => {
    api = pasangFetchPalsu({ [`GET ${WILAYAH}`]: amplop([marker(), marker({ kode: "1108", nama: "KABUPATEN ACEH BESAR", status: "normal" })]) });
    renderHalaman(<OpsMap />, { url: "/operasional" });

    assert.ok(document.querySelectorAll(".shimmer").length > 0);
    assert.ok(await screen.findByRole("button", { name: "Kota Banda Aceh: Cabai merah keriting" }));
    assert.ok(screen.getByRole("button", { name: "Kab. Aceh Besar: Cabai merah keriting" }));
    assert.equal(screen.queryByRole("alert"), null);
  });

  it("tanpa wilayah: peta tetap tampil (kosong bukan galat)", async () => {
    api = pasangFetchPalsu({ [`GET ${WILAYAH}`]: amplop([]) });
    renderHalaman(<OpsMap />, { url: "/operasional" });
    assert.ok(await screen.findByLabelText("Peta skema Aceh"));
    assert.equal(screen.queryByRole("alert"), null);
  });

  it("galat: panel galat dengan tombol coba lagi, bukan kerangka tanpa akhir; coba lagi memuat ulang", async () => {
    let panggilan = 0;
    api = pasangFetchPalsu({ [`GET ${WILAYAH}`]: () => (++panggilan === 1 ? galatApi(500, "Gagal mengambil penanda wilayah") : amplop([marker()])) });
    const { user } = renderHalaman(<OpsMap />, { url: "/operasional" });

    const alert = await screen.findByRole("alert");
    assert.match(alert.textContent ?? "", /Peta status wilayah gagal dimuat/);
    assert.match(alert.textContent ?? "", /Gagal mengambil penanda wilayah/);
    assert.equal(document.querySelectorAll(".shimmer").length, 0);
    assert.ok(screen.getByRole("link", { name: "Peta lengkap" }), "judul dan tautan panel tetap ada");

    await user.click(within(alert).getByRole("button", { name: /Coba lagi/ }));
    assert.ok(await screen.findByRole("button", { name: /Kota Banda Aceh/ }));
    assert.equal(screen.queryByRole("alert"), null);
  });
});
