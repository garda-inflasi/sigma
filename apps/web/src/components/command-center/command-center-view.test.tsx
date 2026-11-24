import assert from "node:assert/strict";
import { afterEach, beforeEach, describe, it } from "node:test";
import { screen, waitFor } from "@testing-library/react";
import { PathnameContext } from "next/dist/shared/lib/hooks-client-context.shared-runtime";
import { muatFixture } from "@/lib/api/__fixtures__/muat";
import { Topbar } from "@/components/layout/topbar";
import { canAccessRoute } from "@/lib/rbac/routes";
import { judulHalaman } from "@/lib/nav";
import {
  amplop,
  pasangFetchPalsu,
  penggunaSesi,
  renderHalaman,
  type FetchPalsu,
} from "@/test/render";
import { tuntaskanTimer } from "@/test/impersonasi";
import { CommandCenterView } from "./command-center-view";

let fetchPalsu: FetchPalsu;
beforeEach(() => {
  fetchPalsu = pasangFetchPalsu();
});
afterEach(() => fetchPalsu.pulihkan());
afterEach(tuntaskanTimer);

describe("Akses Command Center di Topbar & RBAC", () => {
  it("menampilkan tombol 1-klik Command Center untuk pemegang izin operasional/eksekutif/EWS", () => {
    const sesi = penggunaSesi({ role: "STRATEGIS_FMIPA", nama: "Dr. Ilham" });
    renderHalaman(
      <PathnameContext.Provider value="/eksekutif">
        <Topbar />
      </PathnameContext.Provider>,
      { url: "/eksekutif", sesi },
    );
    const link = screen.getByRole("link", { name: "Buka Command Center" });
    assert.equal(link.getAttribute("href"), "/command-center");
    assert.equal(judulHalaman("/command-center"), "Command Center");
    assert.equal(canAccessRoute("STRATEGIS_FMIPA", "/command-center"), true);
    assert.equal(canAccessRoute("OPS_MAHASISWA", "/command-center"), false);
  });

  it("menyembunyikan tombol Command Center bagi OPS_MAHASISWA", () => {
    const sesi = penggunaSesi({ role: "OPS_MAHASISWA", nama: "Ani" });
    renderHalaman(
      <PathnameContext.Provider value="/lapangan/input">
        <Topbar />
      </PathnameContext.Provider>,
      { url: "/lapangan/input", sesi },
    );
    assert.equal(screen.queryByRole("link", { name: "Buka Command Center" }), null);
  });
});

describe("CommandCenterView: data live, prioritas KRITIS, & Fullscreen", () => {
  it("memuat data live tanpa 403, menampilkan alert KRITIS persisten, dan mendukung Fullscreen API", async () => {
    const kpiFix = muatFixture("provinsi", "kpi");

    fetchPalsu.atur(
      "GET /v1/siklus/aktif",
      amplop({
        siklus: "tekanan",
        mulai_pada: "2026-11-24T00:00:00+07:00",
        sumber: "OTOMATIS",
      }),
    );
    fetchPalsu.atur(
      "GET /v1/dashboard/eksekutif",
      amplop({
        demo: false,
        inflasi: {
          periode: "2026-11-01",
          sumber: "BPS",
          diimpor_pada: "2026-11-02T00:00:00+07:00",
          ihk: 106.4,
          yoy: 3.12,
          mtm: 0.28,
          ytd: 2.04,
          sasaran: { tahun: 2026, bawah: 1.5, atas: 3.5 },
          seri: [
            { periode: "2026-09-01", yoy: 2.8 },
            { periode: "2026-10-01", yoy: 2.95 },
            { periode: "2026-11-01", yoy: 3.12 },
          ],
          penyumbang: [{ kode: "CBM-KRT", nama: "Cabai merah keriting", andil: 0.42 }],
        },
        neraca: null,
        fiskal: null,
        ews: { KRITIS: 1 },
        intervensi: {},
        siklus_dominan: "tekanan",
      }),
    );
    fetchPalsu.atur("GET /v1/dashboard/operasional/kpi", amplop(kpiFix));
    fetchPalsu.atur(
      "GET /v1/dashboard/operasional/heatmap",
      amplop({ kabupaten: [], komoditas: [], sel: [] }),
    );
    fetchPalsu.atur("GET /v1/dashboard/operasional/aktivitas-lapangan", amplop([]));
    fetchPalsu.atur(
      "GET /v1/dashboard/operasional/stok",
      amplop({
        tanggal: "2026-11-24",
        ambang_kedaluwarsa_hari: 14,
        ambang_risiko: 0.5,
        ringkasan: {
          total_ton: 320,
          jumlah_lot: 12,
          jumlah_fasilitas: 4,
          kapasitas_ton: 800,
          fasilitas_operasional: 3,
          utilisasi_pct: 40,
          mendekati_kedaluwarsa_ton: 12.5,
          risiko_tinggi_ton: 0,
          kedaluwarsa_lot: 0,
          kedaluwarsa_ton: 0,
          di_luar_operasional_ton: 0,
        },
        komoditas: [],
      }),
    );
    fetchPalsu.atur(
      "GET /v1/dashboard/operasional/logistik",
      amplop({
        dari: "2026-11-17",
        sampai: "2026-11-24",
        ringkasan: {
          masuk: { jumlah: 1, volume_ton: 10 },
          keluar: { jumlah: 0, volume_ton: 0 },
          pindah: { jumlah: 0, volume_ton: 0 },
        },
        komoditas: [],
        terbaru: [],
        rute: { jumlah: 2, volume_ton: 25, biaya: 1000000, items: [] },
      }),
    );
    fetchPalsu.atur("GET /v1/dashboard/operasional/cuaca", amplop([]));
    fetchPalsu.atur(
      "GET /v1/dashboard/operasional/tren",
      amplop([]),
    );
    fetchPalsu.atur(
      "GET /v1/spasial/wilayah",
      amplop([
        {
          kode: "1106",
          nama: "KABUPATEN ACEH TENGAH",
          lat: 4.627,
          lng: 96.846,
          delta: 18.4,
          komoditas: "Cabai merah keriting",
          status: "defisit",
          punya_pasar: true,
        },
      ]),
    );
    fetchPalsu.atur(
      "GET /v1/spasial/geometri",
      amplop({ type: "FeatureCollection", features: [] }),
    );
    fetchPalsu.atur("GET /v1/spasial/titik-desa", amplop([]));
    fetchPalsu.atur("GET /v1/master/pasar", amplop([]));
    fetchPalsu.atur("GET /v1/master/wilayah", amplop([]));
    fetchPalsu.atur(
      "GET /api/cold-storage",
      amplop({ fasilitas: [], stok: [], ringkasan: null }),
    );
    fetchPalsu.atur(
      "GET /v1/prediksi/ml/status",
      amplop({ dikonfigurasi: true, tersedia: true, versi: "v2.1" }),
    );
    fetchPalsu.atur("GET /v1/ews", (req) => {
      if (req.query.get("severity") === "KRITIS") {
        return amplop([
          {
            id: "KR-TEST-1",
            kode: "EWS-KR-1",
            severity: "KRITIS",
            jenis: "LONJAKAN_HARGA",
            judul: "Lonjakan Ekstrem Cabai Takengon",
            komoditas_kode: "CBM-KRT",
            komoditas_nama: "Cabai merah keriting",
            kabupaten_kode: "1106",
            wilayah: "KABUPATEN ACEH TENGAH",
            nilai: 24.5,
            satuan_nilai: "%",
            pesan: "Harga naik melewati ambang kritis",
            dibuat_pada: "2026-11-24T04:00:00+07:00",
            status: "BARU",
            skor_dampak: 95,
            sumber: "RULE",
          },
        ]);
      }
      return amplop([]);
    });

    const sesi = penggunaSesi({ role: "ADMIN_SISTEM", nama: "Superadmin" });
    const { user } = renderHalaman(<CommandCenterView />, {
      url: "/command-center",
      sesi,
    });

    await waitFor(() => {
      assert.ok(screen.getByText("Command Center Aceh"));
      assert.ok(screen.getByText("Lonjakan Ekstrem Cabai Takengon"));
    });

    const root = screen.getByTestId("command-center-root");
    let reqCalled = false;
    let exitCalled = false;
    Object.defineProperty(root, "requestFullscreen", {
      configurable: true,
      value: async () => {
        reqCalled = true;
        Object.defineProperty(document, "fullscreenElement", {
          configurable: true,
          value: root,
        });
        document.dispatchEvent(new Event("fullscreenchange"));
      },
    });
    Object.defineProperty(document, "exitFullscreen", {
      configurable: true,
      value: async () => {
        exitCalled = true;
        Object.defineProperty(document, "fullscreenElement", {
          configurable: true,
          value: null,
        });
        document.dispatchEvent(new Event("fullscreenchange"));
      },
    });

    await user.click(screen.getByRole("button", { name: "Mode Fullscreen" }));
    assert.equal(reqCalled, true);
    assert.equal(root.getAttribute("data-fullscreen"), "true");

    await user.click(screen.getByRole("button", { name: "Keluar Fullscreen" }));
    assert.equal(exitCalled, true);
    assert.equal(root.getAttribute("data-fullscreen"), "false");
  });
});
