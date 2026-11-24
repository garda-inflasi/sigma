import assert from "node:assert/strict";
import { afterEach, beforeEach, describe, it } from "node:test";
import { act, screen, waitFor, within } from "@testing-library/react";
import type { EwsAlertResponse } from "@/lib/api/ews";
import { toEwsAlert } from "@/lib/api/ews";
import { api as endpoint } from "@/lib/api/endpoints";
import { terapkanPeristiwaKritis } from "@/lib/ews/kritis";
import { qk } from "@/lib/query/keys";
import type { EwsAlert } from "@/lib/data/types";
import { useEwsStore } from "@/lib/stores/ews-store";
import {
  amplop,
  galatApi,
  pasangFetchPalsu,
  penggunaSesi,
  renderHalaman,
  type FetchPalsu,
} from "@/test/render";
import { BannerKritis } from "./banner-kritis";

const kritis = (id: string, status: EwsAlertResponse["status"] = "BARU"): EwsAlertResponse => ({
  id,
  kode: `EWS-${id}`,
  severity: "KRITIS",
  jenis: "LONJAKAN_HARGA",
  judul: `Lonjakan ${id}`,
  pesan: "Harga naik.",
  komoditas_kode: "CBM-KRT",
  komoditas_nama: "Cabai merah keriting",
  kabupaten_kode: "1171",
  wilayah: "KOTA BANDA ACEH",
  nilai: 21,
  satuan_nilai: "%",
  skor_dampak: 91,
  status,
  sumber: "Rule engine EWS",
  dibuat_pada: "2026-11-24T00:00:00Z",
});

let api: FetchPalsu;
let server: EwsAlertResponse[];

beforeEach(() => {
  server = [];
  sessionStorage.clear();
  useEwsStore.setState({ dikecilkan: [] });
  api = pasangFetchPalsu({ "GET /v1/ews": () => amplop(server) });
});
afterEach(() => api.pulihkan());
// Sonner melepas toast dengan timer ~200 ms; tunggu sebelum window tes dibongkar supaya timernya tidak melempar.
afterEach(() => new Promise<void>((selesai) => setTimeout(selesai, 260)));

const toast = () => screen.queryAllByTestId("toast-kritis");
const tombol = (nama: string) => screen.queryAllByRole("button", { name: nama });
const pemegangAkui = () => penggunaSesi({ role: "MANAJER_RISIKO" });
const tanpaAkui = () => penggunaSesi({ role: "STRATEGIS_BI" });

describe("BannerKritis", () => {
  it("tanpa KRITIS berstatus BARU tidak menampilkan apa pun", async () => {
    server = [];
    renderHalaman(<BannerKritis />, { sesi: pemegangAkui() });
    await waitFor(() => assert.equal(api.ke("GET", "/v1/ews").length, 1));
    assert.equal(screen.queryByTestId("banner-kritis"), null);
    assert.equal(toast().length, 0);
  });

  it("pengguna tanpa ews.lihat tidak memuat data dan tidak melihat banner", async () => {
    server = [kritis("1")];
    renderHalaman(<BannerKritis />, { sesi: penggunaSesi({ role: "OPS_MAHASISWA" }) });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 20));
    });
    assert.equal(api.ke("GET", "/v1/ews").length, 0);
    assert.equal(screen.queryByTestId("banner-kritis"), null);
  });

  it("puluhan KRITIS: paling banyak tiga toast, sisanya terwakili jumlah di banner yang menaut ke /ews", async () => {
    server = Array.from({ length: 12 }, (_, i) => kritis(String(i + 1)));
    renderHalaman(<BannerKritis />, { sesi: pemegangAkui() });

    const banner = await screen.findByTestId("banner-kritis");
    assert.match(banner.textContent ?? "", /12 peringatan kritis belum ditindaklanjuti/);
    assert.equal(within(banner).getByRole("link").getAttribute("href"), "/ews?severity=KRITIS");
    assert.equal(within(banner).queryByRole("button"), null, "banner tidak bisa ditutup");
    await waitFor(() => assert.equal(toast().length, 3));
  });

  it("toast KRITIS tanpa tombol tutup, dan 'Lihat detail' tidak menutupnya", async () => {
    server = [kritis("1")];
    const { user, router } = renderHalaman(<BannerKritis />, { sesi: pemegangAkui() });

    await waitFor(() => assert.equal(toast().length, 1));
    assert.equal(tombol("Tutup").length, 0);
    await user.click(screen.getByRole("button", { name: "Lihat detail" }));

    assert.deepEqual(router.push, ["/ews?id=1"]);
    assert.equal(toast().length, 1);
  });

  it("Akui: toast bertahan sampai server menyatakan statusnya berubah (tidak optimistis)", async () => {
    server = [kritis("1")];
    let lepas!: () => void;
    const ditahan = new Promise<void>((r) => (lepas = r));
    api.atur("POST /v1/ews/bulk", async () => {
      await ditahan;
      server = [];
      return amplop({ berhasil: 1, gagal: 0 });
    });
    const { user } = renderHalaman(<BannerKritis />, { sesi: pemegangAkui() });
    await waitFor(() => assert.equal(toast().length, 1));

    await user.click(screen.getByRole("button", { name: "Akui" }));
    assert.equal(toast().length, 1, "masih menunggu jawaban server");
    assert.ok(screen.getByTestId("banner-kritis"));

    lepas();
    await waitFor(() => assert.equal(toast().length, 0));
    await waitFor(() => assert.equal(screen.queryByTestId("banner-kritis"), null));
  });

  it("Akui gagal di server: toast dan banner tetap ada", async () => {
    server = [kritis("1")];
    api.atur("POST /v1/ews/bulk", galatApi(500, "gagal"));
    const { user } = renderHalaman(<BannerKritis />, { sesi: pemegangAkui() });
    await waitFor(() => assert.equal(toast().length, 1));

    await user.click(screen.getByRole("button", { name: "Akui" }));

    await screen.findByText("Aksi massal gagal");
    assert.equal(toast().length, 1);
    assert.ok(screen.getByTestId("banner-kritis"));
  });

  it("tanpa ews.akui: tidak ada Akui; Kecilkan memindahkan toast ke banner yang tetap tampil", async () => {
    server = [kritis("1"), kritis("2")];
    const { user } = renderHalaman(<BannerKritis />, { sesi: tanpaAkui() });
    await waitFor(() => assert.equal(toast().length, 2));
    assert.equal(tombol("Akui").length, 0);

    await user.click(tombol("Kecilkan")[0]);

    await waitFor(() => assert.equal(toast().length, 1));
    assert.match(screen.getByTestId("banner-kritis").textContent ?? "", /2 peringatan kritis/);
    assert.equal(useEwsStore.getState().dikecilkan.length, 1);
  });

  it("toast yang dikecilkan tidak muncul lagi setelah muat ulang, tetapi banner tetap", async () => {
    server = [kritis("1")];
    const pertama = renderHalaman(<BannerKritis />, { sesi: tanpaAkui() });
    await waitFor(() => assert.equal(toast().length, 1));
    await pertama.user.click(tombol("Kecilkan")[0]);
    await waitFor(() => assert.equal(toast().length, 0));
    pertama.unmount();

    useEwsStore.setState({ dikecilkan: [] }); // memori hilang saat muat ulang; sessionStorage tetap
    renderHalaman(<BannerKritis />, { sesi: tanpaAkui() });

    await screen.findByTestId("banner-kritis");
    await act(async () => {
      await new Promise((r) => setTimeout(r, 20));
    });
    assert.equal(toast().length, 0);
  });

  it("muat ulang tanpa tindak lanjut memunculkan toast kembali", async () => {
    server = [kritis("1")];
    const pertama = renderHalaman(<BannerKritis />, { sesi: pemegangAkui() });
    await waitFor(() => assert.equal(toast().length, 1));
    pertama.unmount();
    await waitFor(() => assert.equal(toast().length, 0));

    renderHalaman(<BannerKritis />, { sesi: pemegangAkui() });
    await waitFor(() => assert.equal(toast().length, 1));
  });

  it("ditindaklanjuti dari tab atau pengguna lain (peristiwa SSE): toast dan banner hilang", async () => {
    server = [kritis("1")];
    const { qc } = renderHalaman(<BannerKritis />, { sesi: pemegangAkui() });
    await waitFor(() => assert.equal(toast().length, 1));

    act(() => {
      qc.setQueryData<EwsAlert[]>(qk.ews.kritisBaru(), (lama) =>
        terapkanPeristiwaKritis(lama, toEwsAlert(kritis("1", "DIESKALASI"))),
      );
    });

    await waitFor(() => assert.equal(toast().length, 0));
    assert.equal(screen.queryByTestId("banner-kritis"), null);
  });
});

describe("daftar EWS tidak memotong KRITIS", () => {
  const halaman = (mulai: number, n: number, severity: "KRITIS" | "SIAGA") =>
    Array.from({ length: n }, (_, i) => ({ ...kritis(String(mulai + i)), severity }));

  it("list mengambil halaman berikutnya selama halaman penuh masih berakhir dengan KRITIS aktif", async () => {
    api.atur("GET /v1/ews", (req: { query: URLSearchParams }) => {
      const page = Number(req.query.get("page"));
      if (page === 1) return amplop(halaman(1, 200, "KRITIS"));
      if (page === 2) return amplop([...halaman(201, 50, "KRITIS"), ...halaman(251, 150, "SIAGA")]);
      return amplop(halaman(401, 200, "SIAGA"));
    });

    const hasil = await endpoint.ews.list();

    assert.equal(hasil.filter((a) => a.severity === "KRITIS").length, 250);
    assert.equal(api.ke("GET", "/v1/ews").length, 2, "berhenti setelah KRITIS aktif habis");
  });

  it("kritisBaru meminta status BARU dan severity KRITIS, lintas halaman", async () => {
    api.atur("GET /v1/ews", (req: { query: URLSearchParams }) =>
      amplop(req.query.get("page") === "1" ? halaman(1, 1000, "KRITIS") : halaman(1001, 5, "KRITIS")),
    );

    const hasil = await endpoint.ews.kritisBaru();

    assert.equal(hasil.length, 1005);
    const q = api.ke("GET", "/v1/ews")[0].query;
    assert.equal(q.get("status"), "BARU");
    assert.equal(q.get("severity"), "KRITIS");
  });
});
