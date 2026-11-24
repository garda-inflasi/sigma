import assert from "node:assert/strict";
import { afterEach, beforeEach, describe, it } from "node:test";
import { screen, waitFor, within } from "@testing-library/react";
import { PanelTekananMl } from "@/components/eksekutif/tekanan-ml";
import { toLampiranMl } from "@/lib/api/ml";
import {
  EKSEKUTIF_C04,
  EKSEKUTIF_C05,
  PREDIKSI_DETAIL_1101,
  PREDIKSI_DETAIL_C04,
  PREDIKSI_DETAIL_C05,
  PREDIKSI_RINGKAS_C05,
  SIKLUS_NYATA,
} from "@/lib/api/__fixtures__/ml/nyata";
import {
  keadaanBlok,
  LABEL_BELUM_ANDAL,
  LABEL_INDEKS_PANGAN,
  LABEL_PERSISTENSI,
  LABEL_WILAYAH_TANPA_DATA,
} from "@/lib/ml/label";
import { useUi } from "@/lib/stores/ui-store";
import {
  amplop,
  pasangFetchPalsu,
  renderHalaman,
  type FetchPalsu,
  type PermintaanFetch,
} from "@/test/render";
import { PrediksiPage } from "./prediksi-page";

const denganMl = (ml: unknown, data: unknown = {}) => ({ ...amplop(data), ml });

/** Jawaban `/v1/prediksi/:kode` menurut kabupaten dan siklus yang diminta (data nyata siklus 05Z dan 04Z). */
function detail(req: PermintaanFetch) {
  if (req.query.get("kabupaten") === "1101")
    return denganMl(PREDIKSI_DETAIL_1101);
  return denganMl(
    req.query.get("cycle_id") === "C20261124T04Z"
      ? PREDIKSI_DETAIL_C04
      : PREDIKSI_DETAIL_C05,
  );
}

let f: FetchPalsu;
beforeEach(() => {
  useUi.setState({ lingkunganData: "prod" });
  f = pasangFetchPalsu({
    "GET /v1/ml/cycles": amplop(SIKLUS_NYATA),
    "GET /v1/prediksi/CBM-KRT": detail,
    "GET /v1/prediksi": () => denganMl(PREDIKSI_RINGKAS_C05, []),
    "GET /v1/prediksi/registri": () =>
      denganMl({ tersedia: true, model: PREDIKSI_RINGKAS_C05.model }, []),
    "GET /v1/prediksi/ml": amplop({
      dikonfigurasi: false,
      tersedia: false,
      versi: "",
    }),
  });
});
afterEach(async () => {
  f.pulihkan();
  useUi.setState({ lingkunganData: "staging", mlInfo: null });
  // Timer grafik/Radix yang tersisa dituntaskan sebelum DOM uji dilepas (after global di test/render.tsx).
  await new Promise((r) => setTimeout(r, 50));
});

const routerUrl = {
  replace: (h: string) => window.history.replaceState(null, "", h),
};

describe("/prediksi dengan hasil training (mode Prod)", () => {
  it("menampilkan kuantil H+7/14/30, model persistensi, metrik vs baseline, dan XAI bersatuan", async () => {
    renderHalaman(<PrediksiPage />, { url: "/prediksi" });
    const panel = (
      await screen.findByRole("heading", { name: /^Proyeksi harga / })
    ).closest("section")!;
    const tabel = within(panel).getByRole("table");
    for (const h of ["H+7", "H+14", "H+30"])
      assert.match(tabel.textContent ?? "", new RegExp(h.replace("+", "\\+")));
    for (const k of ["P10", "P50", "P90"])
      assert.ok(within(tabel).getByRole("rowheader", { name: k }));

    assert.ok((await screen.findAllByText(LABEL_PERSISTENSI)).length > 0);
    assert.ok(screen.getByText("MAPE model"));
    assert.ok(screen.getByText(/MAPE baseline/));
    assert.ok(screen.getByText("RMSE"));
    assert.ok(screen.getByText("(poin persen harga)"));
    // Narasi pipeline "(model seasonal_naive)" ditulis ulang sebagai persistensi.
    assert.doesNotMatch(
      document.body.textContent ?? "",
      /model seasonal_naive/,
    );
    // Permintaan lampiran ML membawa kabupaten; tanpa pilihan siklus tidak ada cycle_id (= final terbaru).
    const mlReq = f.ke("GET", "/v1/prediksi/CBM-KRT").filter((r) => r.query.has("kabupaten"));
    assert.ok(mlReq.length > 0);
    assert.equal(mlReq[0].query.get("kabupaten"), "1171");
    assert.equal(mlReq[0].query.get("cycle_id"), null);
  });

  it("memilih siklus lain mengirim ?cycle_id= ke endpoint halaman; format lama ditandai", async () => {
    const { user } = renderHalaman(<PrediksiPage />, {
      url: "/prediksi",
      router: routerUrl,
    });
    await screen.findByText("(poin persen harga)");
    await user.click(
      await screen.findByRole("combobox", { name: "Pilih siklus training" }),
    );
    await user.click(
      await screen.findByRole("option", { name: /24 Nov 2026 11\.00 WIB/ }),
    );

    await waitFor(() =>
      assert.ok(
        f
          .ke("GET", "/v1/prediksi/CBM-KRT")
          .some((r) => r.query.get("cycle_id") === "C20261124T04Z"),
      ),
    );
    assert.ok(
      f
        .ke("GET", "/v1/prediksi")
        .some((r) => r.query.get("cycle_id") === "C20261124T04Z"),
    );
    await screen.findByText("(satuan tidak tercatat, format lama)");
    assert.match(
      screen.getByText(/^Hasil training siklus/).textContent ?? "",
      /C20261124T04Z/,
    );
  });

  it("wilayah tanpa forecast: 'Belum ada data harga resmi untuk wilayah ini' tanpa angka", async () => {
    const { user } = renderHalaman(<PrediksiPage />, { url: "/prediksi" });
    await user.click(
      await screen.findByRole("combobox", { name: "Pilih wilayah forecast" }),
    );
    await user.click(await screen.findByRole("option", { name: /Simeulue/ }));
    const judul = await screen.findByText(LABEL_WILAYAH_TANPA_DATA);
    const panel = judul.closest("section")!;
    assert.doesNotMatch(panel.textContent ?? "", /Rp|P50|%/);
    assert.match(panel.textContent ?? "", /Aceh Barat, Banda Aceh, Lhokseumawe/);
    assert.equal(
      f.ke("GET", "/v1/prediksi/CBM-KRT").at(-1)?.query.get("kabupaten"),
      "1101",
    );
  });

  it("mode Staging tidak meminta hasil training dan memakai tampilan lama", async () => {
    useUi.setState({ lingkunganData: "staging" });
    renderHalaman(<PrediksiPage />, { url: "/prediksi" });
    await new Promise((r) => setTimeout(r, 30));
    assert.equal(f.ke("GET", "/v1/ml/cycles").length, 0);
    assert.equal(
      screen.queryByRole("combobox", { name: "Pilih siklus training" }),
      null,
    );
    assert.equal(screen.queryByText(LABEL_PERSISTENSI), null);
  });
});

describe("PanelTekananMl (eksekutif)", () => {
  const render = (ml: typeof EKSEKUTIF_C04) => {
    const l = toLampiranMl(ml);
    return renderHalaman(
      <PanelTekananMl
        inflasi={keadaanBlok(l, l.inflasi)}
        kesegaran={keadaanBlok(l, l.kesegaran)}
        cycleId={l.siklus?.cycleId}
      />,
      { url: "/eksekutif" },
    );
  };

  it("siklus lama tanpa `layak`: indeks pangan, 'belum dapat diandalkan', tanpa kelas/skor/siklus", async () => {
    render(EKSEKUTIF_C04);
    assert.ok(screen.getByText(LABEL_INDEKS_PANGAN));
    assert.ok(screen.getAllByText(new RegExp(LABEL_BELUM_ANDAL)).length > 0);
    const teks = document.body.textContent ?? "";
    // Data nyata: kelas INFLASI/TIDAK_INFLASI, skor 49,54, siklus NORMAL. Tidak satu pun tampil.
    assert.doesNotMatch(
      teks,
      /\bINFLASI\b|TIDAK_INFLASI|NORMAL|49,54|47,29|skor tekanan \d/,
    );
    assert.doesNotMatch(teks, /Inflasi tahunan|inflasi daerah/i);
    // Penyumbang dan perubahan indeks aktual tetap tampil (data aktual, bukan prediksi).
    assert.ok(screen.getByRole("list", { name: "Penyumbang Aceh Barat" }));
    assert.match(teks, /Indeks aktual/);
    // Kesegaran: sumber dilewati menyebut tanggal data terakhir.
    assert.match(teks, /dilewati, data terakhir/);
    await waitFor(() =>
      assert.ok(
        f
          .ke("GET", "/v1/prediksi")
          .some((r) => r.query.get("kabupaten") === "1107"),
      ),
    );
    // Kurva ringkas penyumbang utama dari forecast siklus yang sama.
    await screen.findByRole("list", { name: "Kurva ringkas" });
  });

  it("siklus TIDAK_LAYAK menyebut alasan pipeline", async () => {
    render(EKSEKUTIF_C05);
    await screen.findByRole("list", { name: "Kurva ringkas" });
    assert.match(
      document.body.textContent ?? "",
      /Tidak lolos uji kelayakan: ROC-AUC holdout 0.41 < 0.60/,
    );
  });
});
