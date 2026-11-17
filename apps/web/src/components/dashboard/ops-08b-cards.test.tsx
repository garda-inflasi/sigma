import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import {
  amplop,
  galatApi,
  pasangFetchPalsu,
  renderHalaman,
  type FetchPalsu,
} from "@/test/render";
import { OpsFilterBar } from "./ops-filter-bar";
import { OpsLogisticsCard } from "./ops-logistics-card";
import { OpsProducerCard } from "./ops-producer-card";
import { OpsStockCard } from "./ops-stock-card";

let api: FetchPalsu;
afterEach(() => api?.pulihkan());

describe("Kartu 08-B Operasional (Stok, Produsen, Logistik, Filter)", () => {
  it("OpsStockCard menampilkan utilisasi kapasitas, kedaluwarsa, risiko, dan komoditas", async () => {
    api = pasangFetchPalsu({
      "GET /v1/dashboard/operasional/stok": amplop({
        tanggal: "2026-11-17",
        ambang_kedaluwarsa_hari: 7,
        ambang_risiko: 0.6,
        ringkasan: {
          total_ton: 42.5,
          jumlah_lot: 8,
          jumlah_fasilitas: 3,
          kapasitas_ton: 100,
          fasilitas_operasional: 3,
          utilisasi_pct: 42.5,
          mendekati_kedaluwarsa_ton: 6.2,
          risiko_tinggi_ton: 3.1,
          kedaluwarsa_lot: 0,
          kedaluwarsa_ton: 0,
          di_luar_operasional_ton: 0,
        },
        komoditas: [
          {
            komoditas_kode: "CBM-KRT",
            komoditas_nama: "Cabai merah keriting",
            total_ton: 18.4,
            jumlah_lot: 3,
            jumlah_fasilitas: 2,
            mendekati_kedaluwarsa_ton: 2.0,
            risiko_tinggi_ton: 3.1,
          },
        ],
      }),
    });

    renderHalaman(<OpsStockCard />, { url: "/operasional" });

    assert.ok(await screen.findByText(/42,5%/));
    assert.ok(screen.getByText(/6,2 ton/));
    assert.ok(screen.getByText(/3,1 ton/));
    assert.ok(screen.getByText("Cabai merah keriting"));
  });

  it("OpsStockCard menampilkan PanelGalat dan memanggil ulang saat Coba lagi diklik", async () => {
    let percobaan = 0;
    api = pasangFetchPalsu({
      "GET /v1/dashboard/operasional/stok": () => {
        percobaan++;
        if (percobaan === 1) {
          return galatApi(503, "Layanan stok sibuk");
        }
        return amplop({
          tanggal: "2026-11-17",
          ambang_kedaluwarsa_hari: 7,
          ambang_risiko: 0.6,
          ringkasan: {
            total_ton: 10,
            jumlah_lot: 1,
            jumlah_fasilitas: 1,
            kapasitas_ton: 50,
            fasilitas_operasional: 1,
            utilisasi_pct: 20,
            mendekati_kedaluwarsa_ton: 0,
            risiko_tinggi_ton: 0,
            kedaluwarsa_lot: 0,
            kedaluwarsa_ton: 0,
            di_luar_operasional_ton: 0,
          },
          komoditas: [],
        });
      },
    });

    renderHalaman(<OpsStockCard />, { url: "/operasional" });

    assert.ok(await screen.findByText("Stok cold storage gagal dimuat"));
    fireEvent.click(screen.getByRole("button", { name: /coba lagi/i }));
    await waitFor(() => assert.ok(screen.getByText(/20,0%/)));
    assert.equal(percobaan, 2);
  });

  it("OpsProducerCard menampilkan harga produsen, konsumen, dan margin", async () => {
    api = pasangFetchPalsu({
      "GET /v1/dashboard/operasional/produsen": amplop({
        dari: "2026-11-10",
        sampai: "2026-11-17",
        komoditas: [
          {
            komoditas_kode: "BRS-MED",
            komoditas_nama: "Beras medium",
            satuan: "kg",
            produsen: { harga: 11000, jumlah_entri: 4, jumlah_kabupaten: 2 },
            konsumen: { harga: 14000, jumlah_entri: 8, jumlah_kabupaten: 2 },
            margin: { selisih: 3000, persen_produsen: 27.3, persen_konsumen: 21.4 },
            produksi: { ton: 1200, periode: "2026-11-01", sumber: "BPS", demo: false },
          },
        ],
      }),
    });

    renderHalaman(<OpsProducerCard />, { url: "/operasional" });

    assert.ok(await screen.findByText("Beras medium"));
    assert.ok(screen.getByText(/21,4%/));
    assert.ok(screen.getByText(/Rp\s*3\.000/));
  });

  it("OpsLogisticsCard menampilkan ringkasan pergerakan dan rute aktif", async () => {
    api = pasangFetchPalsu({
      "GET /v1/dashboard/operasional/logistik": amplop({
        dari: "2026-11-10",
        sampai: "2026-11-17",
        ringkasan: {
          masuk: { jumlah: 4, volume_ton: 25.5 },
          keluar: { jumlah: 3, volume_ton: 12.0 },
          pindah: { jumlah: 1, volume_ton: 5.0 },
        },
        komoditas: [],
        terbaru: [],
        rute: {
          jumlah: 1,
          volume_ton: 15,
          biaya: 4500000,
          items: [
            {
              id: "1",
              kode: "RT-1",
              asal_kode: "1106",
              asal: "Aceh Besar",
              tujuan_kode: "1171",
              tujuan: "Banda Aceh",
              komoditas_kode: "BRS-MED",
              komoditas_nama: "Beras medium",
              volume_ton: 15,
              jarak_km: 24,
              jam_tempuh: 0.7,
              biaya: 4500000,
              status: "AKTIF",
              sumber: "OPTIMASI",
              dibuat_pada: "2026-11-16T08:00:00Z",
            },
          ],
        },
      }),
    });

    renderHalaman(<OpsLogisticsCard />, { url: "/operasional" });

    assert.ok(await screen.findByText(/^25,5/));
    assert.ok(screen.getByText(/^12,0/));
    assert.ok(screen.getByText(/^5,0/));
    assert.ok(screen.getByText(/Banda Aceh/));
  });

  it("OpsFilterBar menampilkan peringatan bila rentang tanggal melebihi 90 hari", async () => {
    renderHalaman(<OpsFilterBar />, {
      url: "/operasional?dari=2026-01-01&sampai=2026-10-01",
    });

    assert.ok(await screen.findByRole("alert"));
    assert.match(screen.getByRole("alert").textContent ?? "", /90 hari/);
  });
});
