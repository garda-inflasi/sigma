import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  cycleIdSah,
  denganCycleId,
  siklusFinalTerbaru,
  toDaftarSiklusMl,
  toKelayakan,
  toLampiranMl,
  type LampiranMlMentah,
} from "./ml";
import {
  EKSEKUTIF_C04,
  EKSEKUTIF_C05,
  PREDIKSI_DETAIL_1101,
  PREDIKSI_DETAIL_C04,
  PREDIKSI_DETAIL_C05,
  PREDIKSI_SIKLUS_SALAH,
  SIKLUS_NYATA,
  SPASIAL_WILAYAH_C05,
} from "./__fixtures__/ml/nyata";

describe("toKelayakan", () => {
  it("memakai kelayakan server bila sah, selain itu menurunkannya dari layak", () => {
    assert.equal(toKelayakan("LAYAK"), "LAYAK");
    assert.equal(toKelayakan("TIDAK_LAYAK", true), "TIDAK_LAYAK");
    assert.equal(toKelayakan(undefined, true), "LAYAK");
    assert.equal(toKelayakan(undefined, false), "TIDAK_LAYAK");
    // Dokumen lama tanpa `layak`.
    assert.equal(toKelayakan(undefined, null), "TIDAK_DIKETAHUI");
    assert.equal(toKelayakan(undefined), "TIDAK_DIKETAHUI");
    assert.equal(toKelayakan("ANEH"), "TIDAK_DIKETAHUI");
  });
});

describe("toLampiranMl: data nyata", () => {
  it("siklus lama tanpa `layak` (C20261124T04Z) menjadi TIDAK_DIKETAHUI di blok dan tiap wilayah", () => {
    const l = toLampiranMl(EKSEKUTIF_C04);
    assert.equal(l.tersedia, true);
    assert.equal(l.dataUtama, "POSTGRES");
    assert.equal(l.siklus?.cycleId, "C20261124T04Z");
    assert.equal(l.siklus?.targetInflasi, "INDEKS_PANGAN");
    const inf = l.inflasi!;
    assert.equal(inf.tersedia, true);
    assert.equal(inf.kelayakan, "TIDAK_DIKETAHUI");
    assert.equal(inf.bukanInflasiResmi, true);
    assert.deepEqual(
      inf.wilayah.map((w) => w.wilayahKode),
      ["1107", "1171", "1174"],
    );
    for (const w of inf.wilayah) {
      assert.equal(w.layak, null);
      assert.equal(w.kelayakan, "TIDAK_DIKETAHUI");
      assert.equal(w.bukanInflasiResmi, true);
    }
    // XAI format lama tanpa satuan: tidak boleh ditebak sebagai poin persen.
    assert.equal(inf.wilayah[0].penjelasan?.satuanKontribusi, null);
    assert.equal(l.model?.klasifikasi?.kelayakan, "TIDAK_DIKETAHUI");
    assert.equal(l.makro?.tersedia, false);
    assert.equal(l.makro?.alasan, "TIDAK_ADA_DATA");
  });

  it("siklus format baru TIDAK_LAYAK (C20261124T05Z): kelas, skor, dan siklus kosong, alasan dibawa", () => {
    const inf = toLampiranMl(EKSEKUTIF_C05).inflasi!;
    assert.equal(inf.kelayakan, "TIDAK_LAYAK");
    assert.match(inf.alasanKelayakan ?? "", /ROC-AUC holdout 0.41 < 0.60/);
    const w = inf.wilayah[0];
    assert.equal(w.layak, false);
    assert.equal(w.kelayakan, "TIDAK_LAYAK");
    assert.equal(w.kelas, null);
    assert.equal(w.skorTekanan, null);
    // API mengirim "" untuk siklus yang tidak diterbitkan.
    assert.equal(w.siklus, null);
    assert.equal(typeof w.probabilitas, "number");
    assert.equal(w.penjelasan, null);
  });

  it("forecast detail: kurva H+7/14/30 berurutan, baseline ditandai, satuan XAI baru terbaca", () => {
    const f = toLampiranMl(PREDIKSI_DETAIL_C05).forecast!;
    assert.equal(f.tersedia, true);
    assert.deepEqual(f.wilayahTersedia, ["1107", "1171", "1174"]);
    const it = f.item!;
    assert.deepEqual(
      it.kurva.map((t) => t.horizon),
      [7, 14, 30],
    );
    assert.equal(it.model.nama, "seasonal_naive");
    assert.equal(it.model.adalahBaseline, true);
    assert.equal(it.metrik.mengalahkanBaseline, false);
    assert.equal(it.metrik.nLipatanSeleksi, 4);
    assert.equal(it.penjelasan?.satuanKontribusi, "poin_persen_harga");
    assert.equal(it.riwayat.length, 8);

    const lama = toLampiranMl(PREDIKSI_DETAIL_C04).forecast!.item!;
    assert.equal(lama.penjelasan?.satuanKontribusi, null);
  });

  it("wilayah tanpa forecast: blok tidak tersedia WILAYAH_TANPA_DATA tanpa item", () => {
    const f = toLampiranMl(PREDIKSI_DETAIL_1101).forecast!;
    assert.equal(f.tersedia, false);
    assert.equal(f.alasan, "WILAYAH_TANPA_DATA");
    assert.equal(f.item, null);
    assert.deepEqual(f.wilayahTersedia, ["1107", "1171", "1174"]);
  });

  it("cycle_id salah: lampiran tidak tersedia dengan alasannya", () => {
    const l = toLampiranMl(PREDIKSI_SIKLUS_SALAH);
    assert.equal(l.tersedia, false);
    assert.equal(l.alasan, "CYCLE_ID_TIDAK_VALID");
    assert.equal(l.siklus, null);
    assert.equal(l.forecastRingkas?.tersedia, false);
  });

  it("blok wilayah dan kesegaran", () => {
    const w = toLampiranMl(SPASIAL_WILAYAH_C05).wilayah!;
    assert.equal(w.tersedia, true);
    assert.deepEqual(
      w.items.map((i) => i.wilayahKode),
      ["1107", "1171", "1174"],
    );
    assert.equal(w.items[0].inflasi?.kelayakan, "TIDAK_LAYAK");
    assert.equal(w.items[0].komoditasForecast.length, 10);

    const k = toLampiranMl(EKSEKUTIF_C05).kesegaran!;
    assert.equal(k.tersedia, true);
    assert.ok(k.sumber.some((s) => s.kategori === "DILEWATI"));
    assert.ok(k.ringkasan.dilewati > 0);
  });

  it("respons tanpa `ml` dan nilai mentah yang tidak lengkap tidak menghasilkan angka karangan", () => {
    assert.deepEqual(toLampiranMl(undefined), {
      tersedia: false,
      alasan: "TIDAK_ADA_LAMPIRAN",
      dataUtama: "POSTGRES",
      siklus: null,
    });
    const mentah: LampiranMlMentah = {
      tersedia: true,
      ews: {
        tersedia: true,
        sinyal: [
          {
            sinyal_kunci: "A",
            severity: "SIAGA",
            jenis: "ANOMALI_HARGA",
            judul: "x",
            pesan: "y",
            aktif: true,
            alert: { id: 42, kode: null, status: "BARU" },
          },
          {
            sinyal_kunci: "B",
            severity: "INFO",
            jenis: "SUMBER_DATA",
            judul: "x",
            pesan: "y",
            aktif: true,
          },
        ],
      },
      forecast_ringkas: {
        tersedia: true,
        items: [
          {
            komoditas_kode: "BRS-MED",
            kabupaten_kode: "1171",
            harga_terakhir: null,
            tanggal_terakhir: null,
            satuan: "Rp/kg",
            kurva: [
              {
                horizon: 30,
                tanggal: "2026-12-24",
                p10: null,
                p50: 1,
                p90: null,
              },
              { horizon: 7, tanggal: "2026-12-01", p10: 1, p50: 1, p90: 1 },
            ],
            model: { nama: "naive" },
            metrik: { mape: null, rmse: null },
          },
        ],
      },
    };
    const l = toLampiranMl(mentah);
    assert.equal(l.ews!.sinyal[0].alert?.id, "42");
    assert.equal(l.ews!.sinyal[0].menjadiAlert, true);
    assert.equal(l.ews!.sinyal[1].menjadiAlert, false);
    assert.deepEqual(l.ews!.ringkasan, {
      aktif: 0,
      kritis: 0,
      siaga: 0,
      waspada: 0,
      info: 0,
    });
    const f = l.forecastRingkas!.items[0];
    assert.deepEqual(
      f.kurva.map((t) => t.horizon),
      [7, 30],
    );
    // Dokumen lama tanpa `adalah_baseline`: diturunkan dari nama model.
    assert.equal(f.model.adalahBaseline, true);
    assert.equal(f.kurva[1].perubahanP50Pct, null);
    assert.equal(f.hargaTerakhir, null);
  });
});

describe("daftar siklus", () => {
  it("memetakan siklus nyata terbaru dahulu dengan kelayakan per siklus", () => {
    const d = toDaftarSiklusMl(SIKLUS_NYATA);
    assert.equal(d.tersedia, true);
    assert.deepEqual(
      d.items.map((s) => s.cycleId),
      ["C20261124T05Z", "C20261124T04Z", "C20261124T03Z"],
    );
    assert.deepEqual(
      d.items.map((s) => s.klasifikasi?.kelayakan),
      ["TIDAK_LAYAK", "TIDAK_DIKETAHUI", "TIDAK_DIKETAHUI"],
    );
    assert.equal(d.items[2].status, "SEBAGIAN");
    assert.equal(d.items[2].final, true);
    assert.equal(siklusFinalTerbaru(d)?.cycleId, "C20261124T05Z");
  });

  it("siklus BERJALAN/GAGAL bukan terbaru; tanpa Mongo daftar tidak tersedia", () => {
    const d = toDaftarSiklusMl({
      tersedia: true,
      items: [
        {
          cycle_id: "C20261124T06Z",
          cycle_timestamp: "2026-11-24T06:00:00Z",
          status: "BERJALAN",
        },
        {
          cycle_id: "C20261124T05Z",
          cycle_timestamp: "2026-11-24T05:00:00Z",
          status: "GAGAL",
        },
        {
          cycle_id: "C20261124T04Z",
          cycle_timestamp: "2026-11-24T04:00:00Z",
          status: "BERHASIL",
        },
      ],
    });
    assert.deepEqual(
      d.items.map((s) => s.final),
      [false, false, true],
    );
    assert.equal(siklusFinalTerbaru(d)?.cycleId, "C20261124T04Z");
    assert.equal(
      siklusFinalTerbaru(
        toDaftarSiklusMl({
          tersedia: false,
          alasan: "TIDAK_DIKONFIGURASI",
          items: [],
        }),
      ),
      undefined,
    );
  });
});

describe("cycle_id", () => {
  it("hanya format C{YYYYMMDD}T{HH}Z yang sah dan dikirim", () => {
    assert.equal(cycleIdSah("C20261124T04Z"), "C20261124T04Z");
    assert.equal(cycleIdSah(" C20261124T04Z "), "C20261124T04Z");
    assert.equal(cycleIdSah("salah"), undefined);
    assert.equal(cycleIdSah(null), undefined);
    assert.equal(denganCycleId("/v1/prediksi"), "/v1/prediksi");
    assert.equal(
      denganCycleId("/v1/prediksi", "C20261124T04Z"),
      "/v1/prediksi?cycle_id=C20261124T04Z",
    );
    assert.equal(
      denganCycleId("/v1/prediksi/CBM-KRT?kabupaten=1171", "C20261124T04Z"),
      "/v1/prediksi/CBM-KRT?kabupaten=1171&cycle_id=C20261124T04Z",
    );
    assert.equal(denganCycleId("/v1/ews", "x"), "/v1/ews");
  });

  it("memetakan klaster K-Means dan indikator makro BPS dari ml_predictions", () => {
    const l = toLampiranMl({
      tersedia: true,
      data_utama: "DATASAFE",
      wilayah: {
        tersedia: true,
        items: [
          {
            wilayah_kode: "1108",
            tingkat: "KABUPATEN",
            komoditas_forecast: ["BRS-MED"],
            klaster: {
              cluster_id: 0,
              cluster_label: "Relatif Stabil",
              centroid_distance: 0.6282,
              volatility_score: 0.0205,
              climate_exposure_score: 0,
            },
          },
        ],
      },
      makro: {
        tersedia: true,
        items: [
          {
            domain: "INFLASI",
            indikator: "inflasi_yoy_bps_tbl_22_inflasi_yoy_01_makanan",
            wilayah_kode: "1171",
            periode: "2026-10-01",
            nilai: 3.12,
            satuan: "%",
            perubahan: 0.1,
            sumber: "BPS_TABLES",
            is_carried_forward: false,
            last_available_date: "2026-10-01",
          },
        ],
      },
    });
    assert.equal(l.wilayah?.items[0].klaster?.clusterLabel, "Relatif Stabil");
    assert.equal(l.wilayah?.items[0].klaster?.volatilityScore, 0.0205);
    assert.equal(l.makro?.tersedia, true);
    assert.equal(l.makro?.items[0].nilai, 3.12);
    assert.equal(l.makro?.items[0].sumber, "BPS_TABLES");
  });
});
