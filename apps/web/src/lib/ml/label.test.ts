import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { toLampiranMl } from "@/lib/api/ml";
import {
  EKSEKUTIF_C04,
  EKSEKUTIF_C05,
  PREDIKSI_DETAIL_1101,
  PREDIKSI_DETAIL_C03,
  PREDIKSI_DETAIL_C05,
  PREDIKSI_SIKLUS_SALAH,
} from "@/lib/api/__fixtures__/ml/nyata";
import {
  alasanBelumAndal,
  keadaanBlok,
  LABEL_BELUM_ANDAL,
  LABEL_INDEKS_PANGAN,
  LABEL_PERSISTENSI,
  LABEL_WILAYAH_TANPA_DATA,
  labelKesegaran,
  labelModelForecast,
  labelSatuanKontribusi,
  labelTargetMl,
  narasiJujur,
  ringkasBlokModel,
  ringkasModelSiklus,
  ringkasPemenang,
  sinyalTekanan,
  tanggalBisnis,
  waktuWib,
} from "./label";

describe("label target", () => {
  it("INDEKS_PANGAN selalu indeks harga pangan, tidak pernah inflasi", () => {
    assert.equal(
      LABEL_INDEKS_PANGAN,
      "Indeks harga pangan (bukan inflasi resmi BPS)",
    );
    assert.equal(labelTargetMl("INDEKS_PANGAN"), LABEL_INDEKS_PANGAN);
    // Target tak dikenal tidak boleh naik menjadi "inflasi".
    assert.equal(labelTargetMl("INFLASI_PROKSI"), LABEL_INDEKS_PANGAN);
    assert.equal(labelTargetMl("RESMI_BPS"), "Inflasi resmi BPS");
    assert.doesNotMatch(labelTargetMl(null), /inflasi/i);
  });
});

describe("kelayakan klasifikasi", () => {
  it("siklus lama tanpa `layak` (TIDAK_DIKETAHUI): belum dapat diandalkan, tanpa kelas/skor/siklus", () => {
    const w = toLampiranMl(EKSEKUTIF_C04).inflasi!.wilayah.find(
      (x) => x.wilayahKode === "1171",
    )!;
    // Data nyata membawa kelas INFLASI, skor 49,54, dan siklus NORMAL: semuanya tidak boleh tampil sebagai prediksi.
    assert.equal(w.kelas, "INFLASI");
    const s = sinyalTekanan(w);
    assert.equal(s.andal, false);
    assert.equal(s.judul, LABEL_BELUM_ANDAL);
    assert.equal(s.judul, "Model belum dapat diandalkan");
    assert.match(s.keterangan, /Kelayakan belum diuji/);
    for (const teks of [s.judul, s.keterangan]) {
      assert.doesNotMatch(teks, /INFLASI|NORMAL|TEKANAN|49|skor/i);
    }
  });

  it("TIDAK_LAYAK (C20261124T05Z) menyebut alasan dari pipeline", () => {
    const w = toLampiranMl(EKSEKUTIF_C05).inflasi!.wilayah[0];
    const s = sinyalTekanan(w);
    assert.equal(s.andal, false);
    assert.equal(s.judul, LABEL_BELUM_ANDAL);
    assert.match(
      s.keterangan,
      /^Tidak lolos uji kelayakan: ROC-AUC holdout 0.41 < 0.60/,
    );
    assert.match(
      alasanBelumAndal("TIDAK_LAYAK", null),
      /ROC-AUC holdout ≥ 0,60/,
    );
  });

  it("hanya LAYAK yang menampilkan kelas sebagai prediksi indeks pangan", () => {
    const s = sinyalTekanan({
      kelayakan: "LAYAK",
      alasanKelayakan: null,
      kelas: "INFLASI",
      skorTekanan: 72.4,
      siklus: "TEKANAN",
    });
    assert.equal(s.andal, true);
    assert.equal(s.judul, "Diproyeksi naik dalam 30 hari");
    assert.match(s.keterangan, /skor tekanan 72\/100 · siklus tekanan/);
    assert.equal(s.tone, "defisit");
    assert.doesNotMatch(s.judul, /inflasi/i);
  });

  it("ringkasan blok model KPI jujur soal target dan kelayakan", () => {
    const r = ringkasBlokModel(toLampiranMl(EKSEKUTIF_C05).model!);
    assert.match(
      r.klasifikasi,
      /^Indeks harga pangan \(bukan inflasi resmi BPS\): Model belum dapat diandalkan/,
    );
    assert.equal(r.regresi, `${LABEL_PERSISTENSI}, 30 seri`);
  });
});

describe("model pemenang", () => {
  it("baseline naive/seasonal_naive ditulis persistensi", () => {
    assert.equal(
      LABEL_PERSISTENSI,
      "Persistensi (belum ada model yang mengalahkan baseline)",
    );
    const it = toLampiranMl(PREDIKSI_DETAIL_C05).forecast!.item!;
    assert.equal(labelModelForecast(it.model), LABEL_PERSISTENSI);
    assert.equal(
      labelModelForecast({ nama: "naive", adalahBaseline: true }),
      LABEL_PERSISTENSI,
    );
    assert.equal(
      labelModelForecast({ nama: "lightgbm_quantile", adalahBaseline: false }),
      "LightGBM kuantil",
    );
  });

  it("narasi pipeline tidak menyebut baseline sebagai model terlatih", () => {
    const it = toLampiranMl(PREDIKSI_DETAIL_C05).forecast!.item!;
    const n = narasiJujur(it.penjelasan!.narasi, it.model);
    assert.doesNotMatch(n, /model seasonal_naive/);
    assert.match(n, /persistensi, belum ada model yang mengalahkan baseline/);
    assert.equal(
      narasiJujur("(model lightgbm_quantile)", { adalahBaseline: false }),
      "(model lightgbm_quantile)",
    );
  });

  it("ringkasan pemenang siklus", () => {
    assert.deepEqual(
      ringkasPemenang({
        pemenang: { naive: 22, seasonal_naive: 8, lightgbm_quantile: 0 },
        mapeMedian: 0.97,
        rmseMedian: 363,
        mapeBaselineMedian: 0.97,
        nSeri: 30,
        mengalahkanBaseline: 0,
        nDievaluasiHoldout: 30,
        nDilewati: 0,
        nGagal: 0,
        modelGlobal: null,
        tingkat: null,
      }),
      { label: `${LABEL_PERSISTENSI}, 30 seri`, semuaBaseline: true },
    );
    assert.equal(ringkasPemenang(null), null);
    assert.equal(
      ringkasModelSiklus({
        final: false,
        status: "BERJALAN",
        regresi: null,
        klasifikasi: null,
      }),
      "Belum selesai",
    );
  });
});

describe("satuan, waktu, dan kesegaran", () => {
  it("satuan kontribusi XAI; format lama ditandai, tidak ditebak", () => {
    assert.equal(
      labelSatuanKontribusi("poin_persen_harga"),
      "poin persen harga",
    );
    assert.equal(
      labelSatuanKontribusi("poin_persen_probabilitas"),
      "poin persen probabilitas",
    );
    assert.equal(
      labelSatuanKontribusi(null),
      "satuan tidak tercatat, format lama",
    );
  });

  it("waktu siklus selalu WIB dan tanggal bisnis tidak bergeser zona", () => {
    assert.equal(waktuWib("2026-11-24T04:00:00Z"), "24 Nov 2026 11.00 WIB");
    assert.equal(waktuWib("2026-11-24T17:00:00Z", true), "25 Nov 00.00 WIB");
    assert.equal(waktuWib(null), "–");
    assert.equal(tanggalBisnis("2026-11-23"), "23 Nov 2026");
    assert.equal(tanggalBisnis(null), "tidak diketahui");
  });

  it("sumber basi atau dilewati menyebut last_available_date", () => {
    assert.equal(
      labelKesegaran({ kategori: "BASI", lastAvailableDate: "2026-11-22" })
        .label,
      "Basi, data terakhir 22 Nov 2026",
    );
    assert.equal(
      labelKesegaran({ kategori: "DILEWATI", lastAvailableDate: null }).label,
      "Dilewati, data terakhir tidak diketahui",
    );
    assert.equal(
      labelKesegaran({ kategori: "SEGAR", lastAvailableDate: "2026-11-24" })
        .label,
      "Segar",
    );
    const k = toLampiranMl(EKSEKUTIF_C05).kesegaran!;
    const basi = k.sumber.find((s) => s.kategori === "BASI");
    if (basi)
      assert.match(
        labelKesegaran(basi).label,
        /data terakhir \d{1,2} \w{3} \d{4}/,
      );
  });
});

describe("keadaanBlok: angka hanya bila tersedia", () => {
  it("memetakan keadaan nyata ke tampil/kosong/siklus/sembunyi", () => {
    const tampil = toLampiranMl(PREDIKSI_DETAIL_C05);
    assert.equal(keadaanBlok(tampil, tampil.forecast).jenis, "tampil");

    const tanpaWilayah = toLampiranMl(PREDIKSI_DETAIL_1101);
    const k = keadaanBlok(tanpaWilayah, tanpaWilayah.forecast);
    assert.equal(k.jenis, "kosong");
    assert.equal(k.jenis === "kosong" && k.pesan, LABEL_WILAYAH_TANPA_DATA);
    assert.equal(k.jenis === "kosong" && k.blok?.wilayahTersedia.length, 3);

    const t03 = toLampiranMl(PREDIKSI_DETAIL_C03);
    assert.equal(keadaanBlok(t03, t03.forecast).jenis, "kosong");

    const salah = toLampiranMl(PREDIKSI_SIKLUS_SALAH);
    assert.equal(keadaanBlok(salah, salah.forecastRingkas).jenis, "siklus");

    // Tanpa Mongo, mode staging, atau tanpa izin: bagian ML disembunyikan rapi.
    for (const alasan of [
      "TIDAK_DIKONFIGURASI",
      "MONGO_TIDAK_TERSEDIA",
      "MODE_STAGING",
    ]) {
      assert.equal(
        keadaanBlok(
          { tersedia: false, alasan, dataUtama: "POSTGRES", siklus: null },
          undefined,
        ).jenis,
        "sembunyi",
      );
    }
    const tanpaIzin = {
      ...tampil,
      forecast: { ...tampil.forecast!, tersedia: false, alasan: "TANPA_IZIN" },
    };
    assert.equal(keadaanBlok(tanpaIzin, tanpaIzin.forecast).jenis, "sembunyi");
    assert.equal(keadaanBlok(undefined, undefined, true).jenis, "memuat");
    assert.equal(keadaanBlok(undefined, undefined).jenis, "sembunyi");
  });
});
