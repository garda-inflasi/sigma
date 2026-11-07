import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { screen, waitFor, within } from "@testing-library/react";
import type { EwsAlertResponse } from "@/lib/api/ews";
import {
  amplop,
  pasangFetchPalsu,
  renderHalaman,
  type FetchPalsu,
} from "@/test/render";
import { EwsPage } from "./ews-page";

const dasar: EwsAlertResponse = {
  id: "1",
  kode: "EWS-0901",
  severity: "SIAGA",
  jenis: "ANOMALI_CUACA",
  judul: "Curah hujan ekstrem di sentra Gayo",
  pesan: "Curah hujan 2,1× normal.",
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
};
const alertA: EwsAlertResponse = dasar;
const alertB: EwsAlertResponse = {
  ...dasar,
  id: "2",
  kode: "EWS-0902",
  jenis: "KELANGKAAN_STOK",
  judul: "Stok cabai di Cold Storage Lambaro menipis",
  kabupaten_kode: "1108",
  wilayah: "KOTA BANDA ACEH",
  nilai: -32,
  satuan_nilai: "%",
  skor_dampak: 74,
  status: "BARU",
};

let api: FetchPalsu;
afterEach(() => api?.pulihkan());

function pasangApi(rujukan: string) {
  const path = `/v1/ews/${encodeURIComponent(rujukan)}`;
  const target =
    [alertA, alertB].find((a) => a.id === rujukan || a.kode === rujukan) ??
    alertA;
  api = pasangFetchPalsu({
    "GET /v1/ews": amplop([alertA, alertB]),
    "GET /v1/ews/aturan": amplop([]),
    "GET /v1/ews/notifikasi/preferensi": amplop({
      preferensi: [],
      vapid_public_key: "",
    }),
    [`GET ${path}`]: amplop(target),
    [`GET ${path}/riwayat`]: amplop([]),
  });
}

/**
 * Judul alert yang tersorot (kelas khusus pada sel peringatan). Dicari lewat DOM langsung: saat lembar detail
 * terbuka, Radix menyembunyikan sisa halaman dari pohon aksesibilitas sehingga `getByRole("row")` tidak berlaku.
 */
const tersorot = () =>
  [...document.querySelectorAll("p")]
    .filter((el) => [alertA.judul, alertB.judul].includes(el.textContent ?? "") && el.className.includes("text-data-700"))
    .map((el) => el.textContent);

describe("EwsPage: tautan ?id= menyorot dan membuka alert", () => {
  it("tautan baru memakai id desimal", async () => {
    pasangApi("2");
    renderHalaman(<EwsPage />, { url: "/ews?id=2" });

    const dialog = await screen.findByRole("dialog");
    await within(dialog).findByRole("heading", { name: alertB.judul });
    await waitFor(() => assert.deepEqual(tersorot(), [alertB.judul]));
  });

  it("tautan lama dari email atau push memakai kode (EWS-0901) dan menyorot baris yang kodenya sama", async () => {
    pasangApi("EWS-0901");
    renderHalaman(<EwsPage />, { url: "/ews?id=EWS-0901" });

    const dialog = await screen.findByRole("dialog");
    await within(dialog).findByRole("heading", { name: alertA.judul });
    await waitFor(() => assert.deepEqual(tersorot(), [alertA.judul]));
    // Permintaan detail meneruskan kode apa adanya; server menerjemahkannya sebagai alias.
    assert.equal(api.ke("GET", "/v1/ews/EWS-0901").length, 1);
  });

  it("tanpa ?id= tidak ada baris yang tersorot", async () => {
    pasangApi("1");
    renderHalaman(<EwsPage />, { url: "/ews" });

    await screen.findByText(alertA.judul);
    assert.deepEqual(tersorot(), []);
    assert.equal(screen.queryByRole("dialog"), null);
  });
});
