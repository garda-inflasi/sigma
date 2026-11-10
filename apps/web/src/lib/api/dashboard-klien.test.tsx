/**
 * Klien dashboard operasional terhadap `fetch` palsu (butuh alamat API, jadi dijalankan bersama tes komponen):
 * jalur dan kueri yang dikirim, amplop yang dibuka, pemetaan respons nyata, dan galat 400/403 sebagai ApiError.
 */
import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { screen, waitFor } from "@testing-library/react";
import {
  amplop,
  galatApi,
  pasangFetchPalsu,
  renderHalaman,
  type FetchPalsu,
} from "@/test/render";
import { useHeatmap, useKpiOperasional } from "@/lib/query/hooks";
import { muatFixture } from "./__fixtures__/muat";
import { ApiError } from "./client";
import { api } from "./endpoints";

const D = "/v1/dashboard/operasional";

let fetchPalsu: FetchPalsu;
afterEach(() => fetchPalsu?.pulihkan());

const hanya = (kueri: URLSearchParams) => Object.fromEntries(kueri.entries());

describe("api.operasional.kpi dan heatmap: filter opsional", () => {
  it("tanpa filter: permintaan tanpa kueri (perilaku bawaan tidak berubah)", async () => {
    fetchPalsu = pasangFetchPalsu({
      [`GET ${D}/kpi`]: amplop(muatFixture("provinsi", "kpi")),
      [`GET ${D}/heatmap`]: amplop(muatFixture("provinsi", "heatmap")),
    });
    const kpi = await api.operasional.kpi();
    const heatmap = await api.operasional.heatmap();

    assert.equal(
      fetchPalsu.ke("GET", `${D}/kpi`)[0].url,
      `http://api.test${D}/kpi`,
    );
    assert.equal(
      fetchPalsu.ke("GET", `${D}/heatmap`)[0].url,
      `http://api.test${D}/heatmap`,
    );
    assert.equal(kpi.length, 4);
    assert.equal(heatmap.sel.length, 142);
  });

  it("semua filter: kabupaten_kode, komoditas_kode, dari, sampai", async () => {
    fetchPalsu = pasangFetchPalsu({
      [`GET ${D}/kpi`]: amplop(muatFixture("provinsi", "kpi-terfilter")),
      [`GET ${D}/heatmap`]: amplop(
        muatFixture("provinsi", "heatmap-terfilter"),
      ),
    });
    const filter = {
      kabupatenKode: "1171",
      komoditasKode: "CBM-KRT",
      dari: "2026-11-01",
      sampai: "2026-11-05",
    };
    const kpi = await api.operasional.kpi(filter);
    const heatmap = await api.operasional.heatmap(filter);

    const kueri = {
      kabupaten_kode: "1171",
      komoditas_kode: "CBM-KRT",
      dari: "2026-11-01",
      sampai: "2026-11-05",
    };
    assert.deepEqual(hanya(fetchPalsu.ke("GET", `${D}/kpi`)[0].query), kueri);
    assert.deepEqual(
      hanya(fetchPalsu.ke("GET", `${D}/heatmap`)[0].query),
      kueri,
    );
    assert.equal(kpi[0].label, "Entri harga 5 Nov 2026");
    assert.equal(heatmap.sel.length, 1);
  });

  it("sebagian filter: hanya yang terisi dikirim", async () => {
    fetchPalsu = pasangFetchPalsu({
      [`GET ${D}/kpi`]: amplop(muatFixture("provinsi", "kpi")),
    });
    await api.operasional.kpi({ komoditasKode: "BRS-MED", kabupatenKode: "" });
    assert.deepEqual(hanya(fetchPalsu.ke("GET", `${D}/kpi`)[0].query), {
      komoditas_kode: "BRS-MED",
    });
  });

  it("filter salah: 400 dengan pesan server menjadi ApiError", async () => {
    const pesan =
      "parameter tidak valid: rentang dari..sampai paling lama 90 hari";
    fetchPalsu = pasangFetchPalsu({ [`GET ${D}/kpi`]: galatApi(400, pesan) });
    await assert.rejects(
      api.operasional.kpi({ dari: "2026-01-01", sampai: "2026-11-05" }),
      (err: unknown) => {
        assert.ok(err instanceof ApiError);
        assert.equal(err.status, 400);
        assert.equal(err.message, pesan);
        return true;
      },
    );
  });
});

describe("hook KPI dan heatmap meneruskan filter ke klien (bukan konteks query)", () => {
  function Probe() {
    const kpi = useKpiOperasional({
      kabupatenKode: "1171",
      dari: "2026-11-01",
    });
    const heatmap = useHeatmap({ komoditasKode: "CBM-KRT" });
    return (
      <p>
        {kpi.data ? `kpi:${kpi.data.length}` : "kpi:-"}{" "}
        {heatmap.data ? `heatmap:${heatmap.data.sel.length}` : "heatmap:-"}
      </p>
    );
  }

  it("kueri filter tiba di server dan hasilnya dipetakan", async () => {
    fetchPalsu = pasangFetchPalsu({
      [`GET ${D}/kpi`]: amplop(muatFixture("koordinator", "kpi")),
      [`GET ${D}/heatmap`]: amplop(muatFixture("koordinator", "heatmap")),
    });
    renderHalaman(<Probe />, { url: "/operasional" });

    await waitFor(() => assert.ok(screen.getByText("kpi:4 heatmap:14")));
    assert.deepEqual(hanya(fetchPalsu.ke("GET", `${D}/kpi`)[0].query), {
      kabupaten_kode: "1171",
      dari: "2026-11-01",
    });
    assert.deepEqual(hanya(fetchPalsu.ke("GET", `${D}/heatmap`)[0].query), {
      komoditas_kode: "CBM-KRT",
    });
  });
});

describe("api.operasional.aktivitasPetugas", () => {
  it("tanpa filter: tanpa kueri, respons nyata dipetakan", async () => {
    fetchPalsu = pasangFetchPalsu({
      [`GET ${D}/aktivitas-petugas`]: amplop(
        muatFixture("provinsi", "aktivitas-petugas"),
      ),
    });
    const hasil = await api.operasional.aktivitasPetugas();

    assert.equal(
      fetchPalsu.ke("GET", `${D}/aktivitas-petugas`)[0].query.size,
      0,
    );
    assert.equal(hasil.petugas.length, 10);
    assert.equal(hasil.petugas[0].kabupatenNama, "Aceh Barat");
    assert.equal(hasil.jendelaHari, 7);
  });

  it("dengan kabupaten: kueri kabupaten_kode", async () => {
    fetchPalsu = pasangFetchPalsu({
      [`GET ${D}/aktivitas-petugas`]: amplop(
        muatFixture("koordinator", "aktivitas-petugas"),
      ),
    });
    await api.operasional.aktivitasPetugas("1171");
    assert.deepEqual(
      hanya(fetchPalsu.ke("GET", `${D}/aktivitas-petugas`)[0].query),
      { kabupaten_kode: "1171" },
    );
  });

  it("403 (tanpa izin harga.lihat): ApiError berstatus 403 dengan pesan server", async () => {
    fetchPalsu = pasangFetchPalsu({
      [`GET ${D}/aktivitas-petugas`]: galatApi(403, "Akses ditolak"),
    });
    await assert.rejects(api.operasional.aktivitasPetugas(), (err: unknown) => {
      assert.ok(err instanceof ApiError);
      assert.equal(err.status, 403);
      assert.equal(err.message, "Akses ditolak");
      return true;
    });
  });

  it("kabupaten salah: 400 menjadi ApiError", async () => {
    fetchPalsu = pasangFetchPalsu({
      [`GET ${D}/aktivitas-petugas`]: galatApi(
        400,
        "parameter tidak valid: kabupaten_kode harus 4 digit kode BPS kabupaten/kota, mis. 1171",
      ),
    });
    await assert.rejects(
      api.operasional.aktivitasPetugas("1"),
      (err: unknown) => err instanceof ApiError && err.status === 400,
    );
  });
});

describe("api.operasional.stok, produsen, logistik", () => {
  const saringan = { kabupatenKode: "1108", komoditasKode: "CBM-KRT" };
  const kueri = { kabupaten_kode: "1108", komoditas_kode: "CBM-KRT" };

  it("stok: jalur, saringan, dan pemetaan", async () => {
    fetchPalsu = pasangFetchPalsu({
      [`GET ${D}/stok`]: amplop(muatFixture("provinsi", "stok")),
    });
    const bawaan = await api.operasional.stok();
    const terfilter = await api.operasional.stok(saringan);

    const [a, b] = fetchPalsu.ke("GET", `${D}/stok`);
    assert.equal(a.query.size, 0);
    assert.deepEqual(hanya(b.query), kueri);
    assert.equal(bawaan.ringkasan.totalTon, 780);
    assert.equal(terfilter.komoditas.length, 6);
  });

  it("produsen: jalur, saringan, dan pemetaan (blok null tetap null)", async () => {
    fetchPalsu = pasangFetchPalsu({
      [`GET ${D}/produsen`]: amplop(muatFixture("provinsi", "produsen")),
    });
    const bawaan = await api.operasional.produsen();
    await api.operasional.produsen(saringan);

    const [a, b] = fetchPalsu.ke("GET", `${D}/produsen`);
    assert.equal(a.query.size, 0);
    assert.deepEqual(hanya(b.query), kueri);
    assert.equal(bawaan.komoditas.length, 8);
    assert.equal(
      bawaan.komoditas.find((k) => k.komoditasKode === "BRS-MED")?.margin,
      null,
    );
  });

  it("logistik: jalur, saringan, dan pemetaan", async () => {
    fetchPalsu = pasangFetchPalsu({
      [`GET ${D}/logistik`]: amplop(muatFixture("provinsi", "logistik")),
    });
    const bawaan = await api.operasional.logistik();
    await api.operasional.logistik(saringan);

    const [a, b] = fetchPalsu.ke("GET", `${D}/logistik`);
    assert.equal(a.query.size, 0);
    assert.deepEqual(hanya(b.query), kueri);
    assert.equal(bawaan.ringkasan.masuk.jumlah, 4);
    assert.equal(bawaan.rute.items.length, 5);
  });

  it("ketiganya: 400 saringan salah dan 403 menjadi ApiError", async () => {
    fetchPalsu = pasangFetchPalsu({
      [`GET ${D}/stok`]: galatApi(
        400,
        "parameter tidak valid: komoditas_kode harus berupa huruf besar",
      ),
      [`GET ${D}/produsen`]: galatApi(403, "Akses ditolak"),
      [`GET ${D}/logistik`]: galatApi(
        500,
        "Gagal mengambil pergerakan logistik",
      ),
    });
    await assert.rejects(
      api.operasional.stok({ komoditasKode: "cbm" }),
      (e: unknown) => e instanceof ApiError && e.status === 400,
    );
    await assert.rejects(
      api.operasional.produsen(),
      (e: unknown) => e instanceof ApiError && e.status === 403,
    );
    await assert.rejects(
      api.operasional.logistik(),
      (e: unknown) => e instanceof ApiError && e.status === 500,
    );
  });
});

describe("api.siklus.aktif", () => {
  it("memetakan otomatis dan alasan dari respons nyata", async () => {
    fetchPalsu = pasangFetchPalsu({
      "GET /v1/siklus/aktif": amplop(
        muatFixture("provinsi", "siklus-aktif-otomatis-tekanan"),
      ),
    });
    const s = await api.siklus.aktif();
    assert.equal(s.siklus, "tekanan");
    assert.equal(s.otomatis, true);
    assert.equal(s.alasan?.pemicu.length, 2);
    assert.equal(s.alasan?.ditekan[0].aturan, "SKL-SRP-UTILISASI-STOK");
    assert.equal(fetchPalsu.ke("GET", "/v1/siklus/aktif")[0].query.size, 0);
  });

  it("siklus dari seed: alasan null; 404 (belum pernah dievaluasi) menjadi ApiError", async () => {
    fetchPalsu = pasangFetchPalsu({
      "GET /v1/siklus/aktif": amplop(
        muatFixture("koordinator", "siklus-aktif"),
      ),
    });
    assert.deepEqual(await api.siklus.aktif(), {
      siklus: "tekanan",
      sejak: "2026-11-04",
      otomatis: false,
      alasan: null,
    });
    fetchPalsu.atur(
      "GET /v1/siklus/aktif",
      galatApi(404, "Siklus aktif tidak ditemukan untuk wilayah ini"),
    );
    await assert.rejects(
      api.siklus.aktif(),
      (e: unknown) => e instanceof ApiError && e.status === 404,
    );
  });
});
