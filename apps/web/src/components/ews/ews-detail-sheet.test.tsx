import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { screen, within } from "@testing-library/react";
import type { EwsAlertResponse } from "@/lib/api/ews";
import {
  amplop,
  pasangFetchPalsu,
  renderHalaman,
  type FetchPalsu,
} from "@/test/render";
import { EwsDetailSheet } from "./ews-detail-sheet";

const alert: EwsAlertResponse = {
  id: "7",
  kode: "EWS-0901",
  severity: "SIAGA",
  jenis: "ANOMALI_CUACA",
  judul: "Curah hujan ekstrem di sentra Gayo",
  pesan: "Curah hujan 2,1× normal selama 10 hari.",
  komoditas_kode: "CBM-KRT",
  komoditas_nama: "Cabai merah keriting",
  kabupaten_kode: "1106",
  wilayah: "KABUPATEN ACEH TENGAH",
  nilai: 148,
  satuan_nilai: "mm",
  skor_dampak: 81,
  status: "DIAKUI",
  sumber: "BMKG",
  dibuat_pada: "2026-10-16T08:56:00Z",
  aturan_id: "3",
  aturan_kode: "ATR-CU-CBM-KRT",
};

let api: FetchPalsu;
afterEach(() => api?.pulihkan());

function pasangApi(rujukan: string, over: Partial<EwsAlertResponse> = {}) {
  const path = `/v1/ews/${encodeURIComponent(rujukan)}`;
  api = pasangFetchPalsu({
    [`GET ${path}`]: amplop({ ...alert, ...over }),
    [`GET ${path}/riwayat`]: amplop([
      {
        id: "11",
        dari: null,
        ke: "BARU",
        oleh_id: null,
        oleh_nama: null,
        waktu: alert.dibuat_pada,
        catatan: "Dibuat oleh rule engine",
      },
    ]),
  });
}

/**
 * Detail alert menurut kontrak ID baru: `id` string desimal (dipakai di tautan dan kunci), `kode` dan `aturan_kode`
 * untuk teks yang dibaca manusia, dan tautan lama berbasis kode tetap diteruskan ke server sebagai alias.
 */
describe("EwsDetailSheet", () => {
  it("baris Aturan menampilkan kode aturan (ATR-…), bukan id desimal", async () => {
    pasangApi("7");
    renderHalaman(<EwsDetailSheet id="7" onClose={() => {}} />);

    const dialog = await screen.findByRole("dialog");
    const aturan = await within(dialog).findByText("Aturan");
    const baris = aturan.parentElement as HTMLElement;
    assert.equal(baris.textContent, "AturanATR-CU-CBM-KRT");
  });

  it("aturan tanpa kode jatuh ke id aturan; alert tanpa aturan tidak menampilkan baris Aturan", async () => {
    pasangApi("7", { aturan_kode: null });
    const r = renderHalaman(<EwsDetailSheet id="7" onClose={() => {}} />);
    const aturan = await within(await screen.findByRole("dialog")).findByText(
      "Aturan",
    );
    assert.equal((aturan.parentElement as HTMLElement).textContent, "Aturan3");
    r.unmount();
    api.pulihkan();

    pasangApi("7", { aturan_id: null, aturan_kode: null });
    renderHalaman(<EwsDetailSheet id="7" onClose={() => {}} />);
    const dialog = await screen.findByRole("dialog");
    await within(dialog).findByRole("heading", { name: alert.judul });
    assert.equal(within(dialog).queryByText("Aturan"), null);
  });

  it("tautan lama dengan kode (?id=EWS-0901) diteruskan ke server sebagai alias dan membuka alert yang sama", async () => {
    pasangApi("EWS-0901");
    renderHalaman(<EwsDetailSheet id="EWS-0901" onClose={() => {}} />);

    const dialog = await screen.findByRole("dialog");
    await within(dialog).findByRole("heading", { name: alert.judul });
    assert.equal(api.ke("GET", "/v1/ews/EWS-0901").length, 1);
    assert.equal(api.ke("GET", "/v1/ews/EWS-0901/riwayat").length, 1);
    assert.ok(await within(dialog).findByText("Dibuat oleh rule engine"));
  });
});
