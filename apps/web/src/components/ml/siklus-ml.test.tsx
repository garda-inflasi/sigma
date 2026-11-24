import assert from "node:assert/strict";
import { afterEach, beforeEach, describe, it } from "node:test";
import { screen, waitFor } from "@testing-library/react";
import { SIKLUS_NYATA } from "@/lib/api/__fixtures__/ml/nyata";
import { useUi } from "@/lib/stores/ui-store";
import {
  amplop,
  pasangFetchPalsu,
  penggunaSesi,
  renderHalaman,
  type FetchPalsu,
} from "@/test/render";
import { BadgeSiklusMl, PemilihSiklusMl } from "./siklus-ml";

let f: FetchPalsu;
beforeEach(() => {
  useUi.setState({ lingkunganData: "prod" });
  f = pasangFetchPalsu({ "GET /v1/ml/cycles": amplop(SIKLUS_NYATA) });
});
afterEach(async () => {
  f.pulihkan();
  useUi.setState({ lingkunganData: "staging", mlInfo: null });
  // Timer grafik/Radix yang tersisa dituntaskan sebelum DOM uji dilepas (after global di test/render.tsx).
  await new Promise((r) => setTimeout(r, 50));
});

/** Router yang benar-benar mengganti URL, supaya `useSearchParams` mengikuti pilihan. */
const routerUrl = {
  replace: (h: string) => window.history.replaceState(null, "", h),
};

describe("PemilihSiklusMl", () => {
  it("bawaannya siklus final terbaru dalam WIB, dan opsinya menampilkan status serta model", async () => {
    const { user } = renderHalaman(<PemilihSiklusMl />, { url: "/prediksi" });
    const trigger = await screen.findByRole("combobox", {
      name: "Pilih siklus training",
    });
    // C20261124T05Z = 05.00 UTC = 12.00 WIB.
    assert.match(
      trigger.textContent ?? "",
      /Siklus 24 Nov 12\.00 WIB · terbaru/,
    );
    assert.match(f.ke("GET", "/v1/ml/cycles")[0].url, /limit=24/);

    await user.click(trigger);
    const opsi = await screen.findAllByRole("option");
    assert.equal(opsi.length, 3);
    assert.match(
      opsi[0].textContent ?? "",
      /24 Nov 2026 12\.00 WIB.*Berhasil.*terbaru/,
    );
    assert.match(opsi[0].textContent ?? "", /Persistensi \(baseline\)/);
    assert.match(opsi[2].textContent ?? "", /11\.00|10\.00/);
    assert.match(opsi[2].textContent ?? "", /Sebagian.*Tanpa forecast/);
  });

  it("memilih siklus lain menyimpan ?cycle_id= di URL; memilih terbaru menghapusnya", async () => {
    const { user } = renderHalaman(<PemilihSiklusMl />, {
      url: "/prediksi?tab=model",
      router: routerUrl,
    });
    await user.click(
      await screen.findByRole("combobox", { name: "Pilih siklus training" }),
    );
    await user.click(
      await screen.findByRole("option", { name: /24 Nov 2026 11\.00 WIB/ }),
    );
    await waitFor(() =>
      assert.equal(window.location.search, "?tab=model&cycle_id=C20261124T04Z"),
    );
    const trigger = screen.getByRole("combobox", {
      name: "Pilih siklus training",
    });
    assert.match(trigger.textContent ?? "", /24 Nov 11\.00 WIB/);
    assert.doesNotMatch(trigger.textContent ?? "", /terbaru/);

    await user.click(trigger);
    await user.click(
      await screen.findByRole("option", { name: /24 Nov 2026 12\.00 WIB/ }),
    );
    await waitFor(() => assert.equal(window.location.search, "?tab=model"));
  });

  it("siklus BERJALAN dan GAGAL hanya metadata: tidak bisa dipilih", async () => {
    f.atur(
      "GET /v1/ml/cycles",
      amplop({
        ...SIKLUS_NYATA,
        items: [
          {
            cycle_id: "C20261124T06Z",
            cycle_timestamp: "2026-11-24T06:00:00Z",
            status: "BERJALAN",
            final: false,
          },
          ...(SIKLUS_NYATA.items ?? []),
        ],
      }),
    );
    const { user } = renderHalaman(<PemilihSiklusMl />, { url: "/ews" });
    const trigger = await screen.findByRole("combobox", {
      name: "Pilih siklus training",
    });
    // Terbaru tetap siklus final, bukan yang masih berjalan.
    assert.match(trigger.textContent ?? "", /12\.00 WIB · terbaru/);
    await user.click(trigger);
    const berjalan = await screen.findByRole("option", { name: /13\.00 WIB/ });
    assert.equal(berjalan.getAttribute("aria-disabled"), "true");
    assert.match(berjalan.textContent ?? "", /Berjalan.*Belum selesai/);
  });

  it("tersembunyi di mode Staging, tanpa izin prediksi.lihat, atau tanpa Mongo", async () => {
    useUi.setState({ lingkunganData: "staging" });
    const a = renderHalaman(<PemilihSiklusMl />, { url: "/prediksi" });
    await new Promise((r) => setTimeout(r, 20));
    assert.equal(a.container.textContent, "");
    assert.equal(f.ke("GET", "/v1/ml/cycles").length, 0);
    a.unmount();

    useUi.setState({ lingkunganData: "prod" });
    const tanpaIzin = penggunaSesi({ role: "OPS_KOORDINATOR" });
    assert.equal(tanpaIzin.permissions?.includes("prediksi.lihat") ?? false, false);
    const b = renderHalaman(<PemilihSiklusMl />, {
      url: "/ews",
      sesi: tanpaIzin,
    });
    await new Promise((r) => setTimeout(r, 20));
    assert.equal(b.container.textContent, "");
    assert.equal(f.ke("GET", "/v1/ml/cycles").length, 0);
    b.unmount();

    f.atur(
      "GET /v1/ml/cycles",
      amplop({
        tersedia: false,
        alasan: "TIDAK_DIKONFIGURASI",
        items: [],
        total: 0,
        page: 1,
        limit: 24,
      }),
    );
    const c = renderHalaman(<PemilihSiklusMl />, { url: "/eksekutif" });
    await waitFor(() => assert.equal(f.ke("GET", "/v1/ml/cycles").length, 1));
    await new Promise((r) => setTimeout(r, 20));
    assert.equal(c.container.textContent, "");
  });
});

describe("BadgeSiklusMl", () => {
  it("menampilkan siklus yang dipakai halaman; tanpa siklus memakai final terbaru", async () => {
    const a = renderHalaman(
      <BadgeSiklusMl
        siklus={{
          cycleId: "C20261124T04Z",
          cycleTimestamp: "2026-11-24T04:00:00Z",
          dataCutoff: null,
          status: "BERHASIL",
          targetInflasi: "INDEKS_PANGAN",
          labelTarget: null,
        }}
      />,
      { url: "/kpi" },
    );
    assert.match(
      screen.getByTestId("badge-siklus-ml").textContent ?? "",
      /Siklus training 24 Nov 11\.00 WIB/,
    );
    assert.equal(f.ke("GET", "/v1/ml/cycles").length, 0);
    a.unmount();

    renderHalaman(<BadgeSiklusMl />, { url: "/intervensi" });
    await waitFor(() =>
      assert.match(
        screen.getByTestId("badge-siklus-ml").textContent ?? "",
        /24 Nov 12\.00 WIB/,
      ),
    );
  });
});
