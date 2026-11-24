import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { ApiError, apiFetchEnvelope, HEADER_DATA_ENV } from "./client";
import {
  toBarisModelProd,
  toBarisProyeksiProd,
  toEwsListProd,
  toProyeksiXaiProd,
  toRingkasanEksekutifProd,
  toSiklusAktifProd,
  toSumberPipelineProd,
  type LampiranMlResponse,
} from "./ml-prod";
import { adaSinyalBelumDijembatani, dapatDitindaklanjutiEws } from "@/lib/ews/kritis";
import type { SiklusAktifResponse } from "./siklus";
import { qk } from "@/lib/query/keys";
import { getLingkunganData, useUi } from "@/lib/stores/ui-store";
import type { RingkasanEksekutifResponse } from "./eksekutif";

const fetchAsli = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = fetchAsli;
  useUi.setState({ lingkunganData: "staging", mlInfo: null });
});

const ML_NYATA: LampiranMlResponse = {
  tersedia: true,
  data_utama: "DATASAFE",
  siklus: {
    cycle_id: "C20261124T04Z",
    cycle_timestamp: "2026-11-24T04:00:00Z",
    data_cutoff: "2026-11-24T00:00:00Z",
    status: "BERHASIL",
    target_inflasi: "INDEKS_PANGAN",
    label_target: "Indeks pangan PIHPS",
  },
  inflasi: {
    tersedia: true,
    target: "INDEKS_PANGAN",
    label_target: "Indeks pangan PIHPS",
    bukan_inflasi_resmi: true,
    wilayah: [
      {
        wilayah_kode: "11",
        tingkat: "PROVINSI",
        periode: "2026-11-24",
        kelas: "INFLASI",
        probabilitas: 0.72,
        skor_tekanan: 68,
        siklus: "TEKANAN",
        target: "INDEKS_PANGAN",
        label_target: "Indeks pangan PIHPS",
        bukan_inflasi_resmi: true,
        proyeksi: { yoy: 3.15, mtm: 0.42, ytd: 2.1 },
        aktual: { yoy: 2.95, mtm: 0.35, ytd: 1.9, periode: "2026-11-01" },
        penyumbang: [
          {
            komoditas_kode: "CBM-KRT",
            nama: "Cabai merah keriting",
            andil: 0.21,
          },
        ],
        penjelasan: null,
      },
    ],
  },
  forecast_ringkas: {
    tersedia: true,
    kabupaten_kode: "1171",
    wilayah_tersedia: ["1171"],
    items: [
      {
        komoditas_kode: "CBM-KRT",
        kabupaten_kode: "1171",
        harga_terakhir: 54000,
        tanggal_terakhir: "2026-11-23",
        satuan: "Rp/kg",
        kurva: [
          {
            horizon: 14,
            tanggal: "2026-12-07",
            p10: 52000,
            p50: 57000,
            p90: 61000,
            perubahan_p50_pct: 5.6,
          },
        ],
        model: {
          nama: "lightgbm_quantile",
          fallback: false,
          alasan: null,
          n_kandidat: 3,
        },
        metrik: {
          mape: 4.1,
          rmse: 2100,
          baseline: "seasonal_naive",
          mape_baseline: 7.4,
          mengalahkan_baseline: true,
          horizon: [7, 14, 30],
        },
      },
    ],
  },
  model: {
    tersedia: true,
    status: "BERHASIL",
    target_inflasi: "INDEKS_PANGAN",
    label_target: "Indeks pangan PIHPS",
    klasifikasi: {
      model: "lightgbm",
      f1: 0.81,
      roc_auc: 0.86,
      brier: 0.11,
      n_uji: 90,
    },
    regresi: {
      pemenang: { lightgbm_quantile: 4 },
      mape_median: 4.1,
      rmse_median: 2100,
      mape_baseline_median: 7.4,
      n_seri: 4,
      mengalahkan_baseline: 4,
    },
  },
  ews: {
    tersedia: true,
    ringkasan: { aktif: 1, kritis: 1, siaga: 0, waspada: 0, info: 0 },
    sinyal: [
      {
        sinyal_kunci: "LH:CBM-KRT:1171",
        severity: "KRITIS",
        jenis: "PROYEKSI_LONJAKAN",
        komoditas_kode: "CBM-KRT",
        kabupaten_kode: "1171",
        nilai: 8.5,
        ambang: 5.0,
        metode: "lightgbm_quantile",
        judul: "Lonjakan Cabai Merah",
        pesan: "Proyeksi naik > 5%",
        aktif: true,
        menjadi_alert: true,
        alert: null,
      },
    ],
  },
  kesegaran: {
    tersedia: true,
    dihitung_pada: "2026-11-24T04:00:00Z",
    ringkasan: {
      segar: 4,
      basi: 1,
      gagal: 0,
      dilewati: 0,
      belum_ada_run_terjadwal: 0,
    },
    sumber: [],
  },
};

describe("LingkunganData store & query keys", () => {
  it("memisahkan query key antara staging dan prod", () => {
    useUi.setState({ lingkunganData: "staging" });
    assert.equal(getLingkunganData(), "staging");
    const keyStaging = qk.eksekutif.ringkasan();

    useUi.setState({ lingkunganData: "prod" });
    assert.equal(getLingkunganData(), "prod");
    const keyProd = qk.eksekutif.ringkasan();

    assert.notDeepEqual(keyStaging, keyProd);
    assert.equal(keyStaging.at(-1), "staging");
    assert.equal(keyProd.at(-1), "prod");
  });

  it("apiFetchEnvelope mengirim header X-Garda-Data-Env sesuai mode aktif dan membaca lampiran ml", async () => {
    const headersDikirim: string[] = [];
    globalThis.fetch = (async (_url, init) => {
      const h = new Headers(init?.headers);
      headersDikirim.push(h.get(HEADER_DATA_ENV) ?? "");
      return new Response(
        JSON.stringify({
          code: "200",
          message: "OK",
          data: { ok: true },
          ml: ML_NYATA,
          data_env: "prod",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }) as typeof fetch;

    useUi.setState({ lingkunganData: "staging" });
    await apiFetchEnvelope("/v1/dashboard/eksekutif");
    useUi.setState({ lingkunganData: "prod" });
    const env = await apiFetchEnvelope<{ ok: boolean }>(
      "/v1/dashboard/eksekutif",
    );

    assert.deepEqual(headersDikirim, ["staging", "prod"]);
    assert.equal(env.data.ok, true);
    assert.equal(env.ml?.siklus?.cycle_id, "C20261124T04Z");
    assert.equal(useUi.getState().mlInfo?.cycleId, "C20261124T04Z");
  });
});

describe("Mapper Prod (MongoDB datasafe/dataunseen) tanpa fallback DEMO", () => {
  it("toRingkasanEksekutifProd membuang blok DEMO PostgreSQL saat Mongo kosong dan mengisi data datasafe bila tersedia", () => {
    const pgDemo: RingkasanEksekutifResponse = {
      demo: true,
      inflasi: {
        periode: "2026-10-01",
        sumber: "DEMO",
        diimpor_pada: "2026-10-01T00:00:00Z",
        ihk: 105,
        yoy: 9.9,
        mtm: 1.2,
        ytd: 4.5,
        sasaran: { tahun: 2026, bawah: 1.5, atas: 3.5 },
        seri: [{ periode: "2026-10-01", yoy: 9.9 }],
        penyumbang: [{ kode: "BRS-MED", nama: "Beras demo", andil: 0.9 }],
      },
      neraca: { periode: "2026-10-01", sumber: "DEMO", items: [] },
      fiskal: {
        sumber: "DEMO",
        per_tanggal: "2026-10-01",
        pagu: 100,
        realisasi: 20,
        skor: 80,
        status: "MEMADAI",
        instrumen: [],
      },
      ews: { KRITIS: 99, SIAGA: 0, WASPADA: 0, INFO: 0 },
      intervensi: { normal: { aktif: 0, selesai: 0 } },
      siklus_dominan: "normal",
    };

    const kosong = toRingkasanEksekutifProd(pgDemo, undefined);
    assert.equal(kosong.demo, false);
    assert.equal(kosong.inflasi, null);
    assert.equal(kosong.neraca, null);
    assert.equal(kosong.fiskal, null);
    assert.equal(kosong.ews.KRITIS, 0);

    // INDEKS_PANGAN bukan inflasi resmi: tidak boleh mengisi kartu inflasi. Dokumen lama tanpa `layak` (kelayakan
    // TIDAK_DIKETAHUI) juga tidak boleh menetapkan siklus dominan.
    const isi = toRingkasanEksekutifProd(pgDemo, ML_NYATA);
    assert.equal(isi.demo, false);
    assert.equal(isi.inflasi, null);
    assert.equal(isi.siklusDominan, "normal");
    assert.equal(isi.ews.KRITIS, 1);

    const resmiLayak: LampiranMlResponse = {
      ...ML_NYATA,
      inflasi: {
        ...ML_NYATA.inflasi!,
        target: "RESMI_BPS",
        bukan_inflasi_resmi: false,
        wilayah: ML_NYATA.inflasi!.wilayah.map((w) => ({
          ...w,
          target: "RESMI_BPS",
          bukan_inflasi_resmi: false,
          layak: true,
          kelayakan: "LAYAK",
        })),
      },
    };
    const resmi = toRingkasanEksekutifProd(pgDemo, resmiLayak);
    assert.equal(resmi.inflasi?.yoy, 2.95);
    assert.equal(resmi.inflasi?.sumber, "BPS");
    assert.equal(resmi.inflasi?.penyumbang[0]?.nama, "Cabai merah keriting");
    assert.equal(resmi.siklusDominan, "tekanan");
  });

  it("toSiklusAktifProd hanya memakai siklus klasifikasi yang LAYAK", () => {
    const pg: SiklusAktifResponse = {
      siklus: "normal",
      sejak: "2026-11-01",
      otomatis: true,
      alasan: null,
    };
    assert.equal(toSiklusAktifProd(pg, ML_NYATA).siklus, "normal");
    const layak: LampiranMlResponse = {
      ...ML_NYATA,
      inflasi: {
        ...ML_NYATA.inflasi!,
        wilayah: ML_NYATA.inflasi!.wilayah.map((w) => ({
          ...w,
          layak: true,
          kelayakan: "LAYAK",
        })),
      },
    };
    assert.equal(toSiklusAktifProd(pg, layak).siklus, "tekanan");
  });

  it("toProyeksiXaiProd menulis model baseline sebagai persistensi", () => {
    const ml: LampiranMlResponse = {
      ...ML_NYATA,
      forecast: {
        tersedia: true,
        komoditas_kode: "CBM-KRT",
        kabupaten_kode: "1171",
        wilayah_tersedia: ["1171"],
        item: {
          ...ML_NYATA.forecast_ringkas!.items[0],
          model: {
            nama: "seasonal_naive",
            fallback: false,
            alasan: null,
            n_kandidat: 5,
          },
          riwayat: [],
          penjelasan: null,
        },
      },
    };
    assert.equal(
      toProyeksiXaiProd("CBM-KRT", ml).model.nama,
      "Persistensi (belum ada model yang mengalahkan baseline)",
    );
  });

  it("toBarisProyeksiProd, toBarisModelProd, dan toProyeksiXaiProd memakai datasafe tanpa fallback demo", () => {
    assert.deepEqual(toBarisProyeksiProd(undefined), []);
    assert.deepEqual(toBarisModelProd(undefined), []);
    assert.throws(() => toProyeksiXaiProd("CBM-KRT", undefined), ApiError);

    const baris = toBarisProyeksiProd(ML_NYATA);
    assert.equal(baris.length, 1);
    assert.equal(baris[0].id, "CBM-KRT");
    assert.equal(baris[0].kini, 54000);
    assert.equal(baris[0].proyeksi, 57000);
    assert.equal(baris[0].demo, false);

    const model = toBarisModelProd(ML_NYATA);
    assert.equal(model.length, 1);
    assert.match(model[0].versi, /lightgbm_quantile/);
    assert.equal(model[0].demo, false);
  });

  it("toEwsListProd menyaring baris DEMO PostgreSQL dan menyertakan sinyal MongoDB datasafe", () => {
    const res = toEwsListProd(
      [
        {
          id: "1",
          kode: "EWS-DEMO",
          severity: "SIAGA",
          jenis: "LONJAKAN_HARGA",
          judul: "Demo",
          pesan: "Demo",
          komoditas_kode: "BRS-MED",
          komoditas_nama: "Beras",
          kabupaten_kode: "1171",
          wilayah: "KOTA BANDA ACEH",
          nilai: 10,
          satuan_nilai: "%",
          skor_dampak: 50,
          status: "BARU",
          sumber: "DEMO",
          dibuat_pada: "2026-11-24T00:00:00Z",
        },
      ],
      {
        ...ML_NYATA,
        ews: {
          ...ML_NYATA.ews!,
          sinyal: [
            ...ML_NYATA.ews!.sinyal,
            {
              sinyal_kunci: "SUMBER_DATA:BPS:bps_inflasi",
              severity: "INFO",
              jenis: "SUMBER_DATA",
              komoditas_kode: null,
              kabupaten_kode: null,
              nilai: null,
              ambang: 0,
              metode: "kesegaran_sumber",
              judul: "Sumber BPS dilewati",
              pesan: "DILEWATI: BPS_API_KEY belum diisi",
              aktif: true,
              menjadi_alert: false,
              alert: null,
            },
          ],
        },
      },
    );
    // INFO tidak pernah menjadi alert (§5.4), jadi tidak muncul di daftar alert.
    assert.equal(res.length, 1);
    assert.equal(res[0].sumber, "ML_DATASAFE");
    assert.equal(res[0].kode, "LH:CBM-KRT:1171");
    assert.equal(dapatDitindaklanjutiEws(res[0], false), false);
    assert.equal(adaSinyalBelumDijembatani(res, false), true);
    assert.equal(dapatDitindaklanjutiEws({ id: "42", sumber: "ML_DATASAFE" }, false), true);
    assert.equal(dapatDitindaklanjutiEws({ id: "SUMBER_DATA:HARGA:SEMUA", sumber: "ML_DATASAFE" }, false), false);
    assert.equal(adaSinyalBelumDijembatani([{ id: "42", sumber: "ML_DATASAFE" }], false), false);
  });

  it("toSumberPipelineProd tidak pernah mengembalikan demo=true", () => {
    const got = toSumberPipelineProd(
      {
        bronze: 100,
        silver: 50,
        gold: 10,
        demo: true,
        mutu: {
          kelengkapan: 90,
          akurasi: 90,
          ketepatan_waktu: 90,
          konsistensi: 90,
        },
      },
      ML_NYATA,
    );
    assert.equal(got.demo, false);
    assert.equal(got.bronze, 5);
    assert.equal(got.silver, 4);
    assert.equal(got.gold, 1);
  });
});
