import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { screen, within } from "@testing-library/react";
import { SIKLUS_TAHAP } from "@/lib/data/siklus";
import type { KartuIntervensi } from "@/lib/data/types";
import { siklusMeta } from "@/lib/design/tokens";
import { amplop, pasangFetchPalsu, renderHalaman, type FetchPalsu } from "@/test/render";
import { KartuDetailSheet } from "./kartu-detail-sheet";

const UUID = "a7c1e4f0-3b2d-3c55-8e41-5d0f6b7a9c10";

const kartu: KartuIntervensi = {
  id: UUID,
  kode: "INT-301",
  siklus: "tekanan",
  tahap: "t-ews",
  judul: "Lonjakan cabai merah Banda Aceh",
  komoditasKode: "CBM-KRT",
  kabupatenKode: "1171",
  prioritas: "TINGGI",
  penanggungJawab: [],
  tenggat: "2026-10-17T09:00:00Z",
  rekomendasiAi: true,
  progres: { selesai: 1, total: 5 },
  versi: 1,
  dieskalasi: false,
  ewsAlertId: "12",
  ewsAlertKode: "EWS-1000",
  dibuatPada: "2026-10-10T08:00:00Z",
};

let api: FetchPalsu;
afterEach(() => api?.pulihkan());

/** Membuka lembar detail dan menunggu riwayat termuat supaya tidak ada permintaan yang tersisa sesudah tes. */
async function bukaDetail(k: KartuIntervensi) {
  pasang(k);
  const dialog = await screen.findByRole("dialog");
  await within(dialog).findByRole("list", { name: "Riwayat kartu" });
  return dialog;
}

function pasang(k: KartuIntervensi) {
  // Riwayat kartu dimuat dengan id UUID kartu; id riwayat berupa string desimal.
  api = pasangFetchPalsu({
    [`GET /v1/intervensi/${k.id}/riwayat`]: amplop([
      { id: "5", aksi: "BUAT", dari: null, ke: "t-ews", oleh_id: null, oleh_nama: null, waktu: "2026-10-10T08:00:00Z", alasan: "" },
    ]),
  });
  const tahap = SIKLUS_TAHAP[k.siklus];
  return renderHalaman(
    <KartuDetailSheet
      kartu={k}
      tahapSiklus={tahap}
      semuaTahap={Object.values(SIKLUS_TAHAP).flat()}
      alur={[]}
      onClose={() => {}}
    />,
  );
}

/** Teks untuk manusia memakai `kode ?? id`: lembar detail tidak boleh menampilkan UUID atau angka id mentah. */
describe("KartuDetailSheet", () => {
  it("deskripsi memakai kode tampilan kartu (INT-…), bukan UUID", async () => {
    const dialog = await bukaDetail(kartu);
    const label = `INT-301 · ${siklusMeta.tekanan.label} · EWS terpicu`;
    assert.ok(within(dialog).getByText(label));
    assert.equal(within(dialog).queryByText(new RegExp(UUID)), null);
  });

  it("baris Alert EWS memakai kode alert (EWS-…), bukan id desimal", async () => {
    const dialog = await bukaDetail(kartu);
    const baris = within(dialog).getByText("Alert EWS")
      .parentElement as HTMLElement;
    assert.equal(baris.textContent, "Alert EWSEWS-1000");
  });

  it("kartu tanpa kode (dibuat tanpa kode tampilan) jatuh ke id; alert tanpa kode jatuh ke id alert", async () => {
    const dialog = await bukaDetail({ ...kartu, kode: undefined, ewsAlertKode: undefined });
    assert.ok(within(dialog).getByText(new RegExp(`^${UUID} · `)));
    const baris = within(dialog).getByText("Alert EWS")
      .parentElement as HTMLElement;
    assert.equal(baris.textContent, "Alert EWS12");
  });

  it("kartu tanpa alert pemicu tidak menampilkan baris Alert EWS", async () => {
    const dialog = await bukaDetail({ ...kartu, ewsAlertId: undefined, ewsAlertKode: undefined });
    assert.equal(within(dialog).queryByText("Alert EWS"), null);
  });
});
