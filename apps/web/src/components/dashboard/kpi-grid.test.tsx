import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { screen, within } from "@testing-library/react";
import { muatFixture } from "@/lib/api/__fixtures__/muat";
import type { KpiResponse } from "@/lib/api/dashboard";
import {
  amplop,
  galatApi,
  pasangFetchPalsu,
  renderHalaman,
  type FetchPalsu,
} from "@/test/render";
import { KpiGrid } from "./kpi-grid";

const KPI = "/v1/dashboard/operasional/kpi";

let api: FetchPalsu;
afterEach(() => api?.pulihkan());

const kartu = (label: string) => screen.getByText(label).closest("a")!;
/** Angka kartu (CountUp memberi aria-label nilai akhir, sebab teksnya menghitung naik). */
const angka = (label: string) =>
  kartu(label).querySelector("[aria-label]")?.getAttribute("aria-label");
/** Baris perubahan di bawah angka: panah, selisih, dan keterangan. */
const perubahan = (label: string) => kartu(label).querySelector("p.text-xs")!;

describe("KpiGrid dengan respons nyata", () => {
  it("provinsi: empat kartu, angka, perubahan, dan tautan dari server; satuan 'poin' tidak ganda", async () => {
    api = pasangFetchPalsu({
      [`GET ${KPI}`]: amplop(muatFixture<KpiResponse[]>("provinsi", "kpi")),
    });
    renderHalaman(<KpiGrid />, { url: "/operasional" });

    assert.ok(
      document.querySelectorAll(".shimmer").length > 0,
      "kerangka selama memuat",
    );
    assert.ok(await screen.findByText("Entri harga hari ini"));
    assert.equal(document.querySelectorAll(".shimmer").length, 0);
    assert.equal(screen.getAllByRole("link").length, 4);
    assert.deepEqual(
      [
        "Entri harga hari ini",
        "Tingkat validasi",
        "Komoditas di atas ambang",
        "Utilisasi cold storage",
      ].map(angka),
      ["96", "77,6%", "1", "82,1%"],
    );
    assert.equal(
      kartu("Tingkat validasi").getAttribute("href"),
      "/data/harga?status=DIAJUKAN",
    );
    assert.equal(
      kartu("Utilisasi cold storage").getAttribute("href"),
      "/cold-storage",
    );

    assert.equal(
      perubahan("Entri harga hari ini").textContent,
      "7,5% dari 7 hari sebelumnya",
    );
    assert.equal(
      perubahan("Tingkat validasi").textContent,
      "14,1 poin dari pekan lalu",
      "bukan '14,1 poin poin dari pekan lalu'",
    );
    assert.equal(
      perubahan("Komoditas di atas ambang").textContent,
      "1 komoditas sejak kemarin",
    );
    assert.equal(
      perubahan("Utilisasi cold storage").textContent,
      "12,9 poin dari pekan lalu",
    );
    assert.equal(screen.queryByRole("alert"), null);
  });

  it("provinsi: arah dinilai menurut makna kartu (validasi turun buruk, komoditas di atas ambang turun baik)", async () => {
    api = pasangFetchPalsu({
      [`GET ${KPI}`]: amplop(muatFixture<KpiResponse[]>("provinsi", "kpi")),
    });
    renderHalaman(<KpiGrid />, { url: "/operasional" });
    await screen.findByText("Entri harga hari ini");

    const warna = (label: string) =>
      perubahan(label).querySelector("span")!.className;
    assert.match(
      warna("Tingkat validasi"),
      /text-defisit-ink/,
      "validasi turun 14,1 poin",
    );
    assert.match(
      warna("Komoditas di atas ambang"),
      /text-normal-ink/,
      "turun satu komoditas: membaik",
    );
    assert.match(warna("Utilisasi cold storage"), /text-normal-ink/);
  });

  it("Koordinator: kartu bernilai nol dan bernada netral adalah data sah, bukan galat atau kerangka", async () => {
    api = pasangFetchPalsu({
      [`GET ${KPI}`]: amplop(muatFixture<KpiResponse[]>("koordinator", "kpi")),
    });
    renderHalaman(<KpiGrid />, { url: "/operasional" });

    assert.ok(await screen.findByText("Utilisasi cold storage"));
    assert.equal(screen.queryByRole("alert"), null);
    assert.equal(document.querySelectorAll(".shimmer").length, 0);
    assert.equal(screen.getAllByRole("link").length, 4);
    assert.equal(angka("Utilisasi cold storage"), "0,0%");
    assert.equal(angka("Komoditas di atas ambang"), "0");
    assert.equal(angka("Entri harga hari ini"), "12");
    // Semua kartu bernada `data`: titik nada berwarna teal, tidak ada nada status.
    for (const label of [
      "Entri harga hari ini",
      "Tingkat validasi",
      "Komoditas di atas ambang",
      "Utilisasi cold storage",
    ]) {
      const titik = kartu(label).querySelector("span.rounded-full")!;
      assert.match(titik.className, /bg-data-500/, label);
    }
  });

  it("perubahan nol tampil netral (tanpa panah naik dinilai baik atau buruk)", async () => {
    api = pasangFetchPalsu({
      [`GET ${KPI}`]: amplop(muatFixture<KpiResponse[]>("koordinator", "kpi")),
    });
    renderHalaman(<KpiGrid />, { url: "/operasional" });
    await screen.findByText("Utilisasi cold storage");

    for (const label of [
      "Komoditas di atas ambang",
      "Utilisasi cold storage",
    ]) {
      const baris = perubahan(label);
      const penanda = baris.querySelector("span")!;
      assert.match(penanda.className, /text-muted-foreground/, label);
      assert.doesNotMatch(
        penanda.className,
        /text-(defisit|normal)-ink/,
        label,
      );
    }
    assert.equal(
      perubahan("Komoditas di atas ambang").textContent,
      "0 komoditas sejak kemarin",
    );
    assert.equal(
      perubahan("Utilisasi cold storage").textContent,
      "0 poin dari pekan lalu",
    );
  });

  it("filter satu kabupaten dan komoditas: label menyebut tanggal acuan dan semua kartu netral", async () => {
    api = pasangFetchPalsu({
      [`GET ${KPI}`]: amplop(
        muatFixture<KpiResponse[]>("provinsi", "kpi-terfilter"),
      ),
    });
    renderHalaman(<KpiGrid />, { url: "/operasional" });

    assert.ok(await screen.findByText("Entri harga 5 Nov 2026"));
    assert.equal(angka("Entri harga 5 Nov 2026"), "2");
    assert.equal(
      perubahan("Entri harga 5 Nov 2026").textContent,
      "0% dari 7 hari sebelumnya",
    );
  });
});

describe("KpiGrid: galat dan kosong", () => {
  it("galat: panel galat dengan tombol coba lagi, bukan kerangka tanpa akhir; coba lagi memuat ulang", async () => {
    let panggilan = 0;
    api = pasangFetchPalsu({
      [`GET ${KPI}`]: () =>
        ++panggilan === 1
          ? galatApi(500, "Gagal mengambil data KPI")
          : amplop(muatFixture<KpiResponse[]>("provinsi", "kpi")),
    });
    const { user } = renderHalaman(<KpiGrid />, { url: "/operasional" });

    const alert = await screen.findByRole("alert");
    assert.match(alert.textContent ?? "", /Indikator KPI gagal dimuat/);
    assert.match(alert.textContent ?? "", /Gagal mengambil data KPI/);
    assert.equal(
      document.querySelectorAll(".shimmer").length,
      0,
      "kerangka tidak menggantung",
    );

    await user.click(within(alert).getByRole("button", { name: /Coba lagi/ }));
    assert.ok(await screen.findByText("Entri harga hari ini"));
    assert.equal(screen.queryByRole("alert"), null);
    assert.equal(panggilan, 2);
  });

  it("jaringan putus: petunjuk sambungan berbahasa Indonesia, bukan pesan teknis", async () => {
    api = pasangFetchPalsu({
      [`GET ${KPI}`]: () => {
        throw new TypeError("Failed to fetch");
      },
    });
    renderHalaman(<KpiGrid />, { url: "/operasional" });
    const alert = await screen.findByRole("alert");
    assert.match(alert.textContent ?? "", /Periksa sambungan internet/);
    assert.doesNotMatch(alert.textContent ?? "", /Failed to fetch/);
  });

  it("daftar kosong: keterangan, bukan grid tanpa kartu", async () => {
    api = pasangFetchPalsu({ [`GET ${KPI}`]: amplop([]) });
    renderHalaman(<KpiGrid />, { url: "/operasional" });
    assert.ok(
      await screen.findByText("Belum ada indikator KPI untuk ditampilkan."),
    );
    assert.equal(screen.queryAllByRole("link").length, 0);
    assert.equal(screen.queryByRole("alert"), null);
  });
});
