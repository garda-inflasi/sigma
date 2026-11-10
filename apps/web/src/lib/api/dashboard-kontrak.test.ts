/**
 * Tes kontrak dengan respons NYATA API Go (fixture di __fixtures__/operasional, lihat muat.ts): untuk peran provinsi dan
 * Koordinator, tiap endpoint dashboard operasional dan /v1/siklus/aktif dipetakan oleh pemeta produksi, lalu diperiksa:
 *
 * 1. setiap kunci di fixture dibaca pemeta (periksaKontrak): field baru backend yang belum dipetakan membuat tes gagal;
 *    yang sengaja tidak dipakai UI didaftarkan dengan alasannya;
 * 2. pemeta tidak membaca kunci yang tidak dikirim backend;
 * 3. model UI tidak memuat undefined, NaN, atau tak hingga, null ditangani, dan jumlah elemennya cocok dengan fixture.
 */
import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { nilaiTakWajar, periksaKontrak } from "@/test/jejak-kunci";
import { muatFixture, type PersonaFixture } from "./__fixtures__/muat";
import {
  capaianPct,
  toAktivitasLapangan,
  toCuacaWilayah,
  toHeatmap,
  toKpiOperasional,
  toPembandingHarga,
  type AktivitasLapanganResponse,
  type CuacaResponse,
  type HeatmapResponse,
  type KpiResponse,
  type PembandingHargaResponse,
} from "./dashboard";
import {
  toAktivitasPetugas,
  toLogistik,
  toProdusen,
  toStok,
  type AktivitasPetugasResponse,
  type LogistikResponse,
  type ProdusenResponse,
  type StokResponse,
} from "./dashboard-operasional";
import { toSiklusAktif, type SiklusAktifResponse } from "./siklus";

const PERSONA: readonly PersonaFixture[] = ["provinsi", "koordinator"];

/** Memuat fixture, memetakannya, dan menegakkan butir 1 sampai 3 di atas. */
function kontrak<T, U>(
  persona: PersonaFixture,
  nama: string,
  petakan: (data: T) => U,
  abaikan: readonly string[] = [],
) {
  const fixture = muatFixture<T>(persona, nama);
  const r = periksaKontrak(fixture, petakan, abaikan);
  const lokasi = `${persona}/${nama}`;
  assert.deepEqual(
    r.takTerpetakan,
    [],
    `${lokasi}: field fixture yang tidak dibaca pemeta (field baru backend?)`,
  );
  assert.deepEqual(
    r.takAda,
    [],
    `${lokasi}: pemeta membaca field yang tidak dikirim backend`,
  );
  assert.deepEqual(
    r.abaikanBasi,
    [],
    `${lokasi}: daftar abaikan basi (field kini dibaca atau sudah tidak ada)`,
  );
  assert.deepEqual(
    nilaiTakWajar(r.hasil),
    [],
    `${lokasi}: model UI memuat undefined/NaN`,
  );
  return { fixture, hasil: r.hasil };
}

const waktuSah = (iso: string) => !Number.isNaN(new Date(iso).getTime());

describe("kontrak /kpi", () => {
  for (const persona of PERSONA) {
    it(`${persona}: empat kartu berurutan, tren lengkap, format dan nada dalam kosakata UI`, () => {
      const { fixture, hasil } = kontrak<
        KpiResponse[],
        ReturnType<typeof toKpiOperasional>[]
      >(persona, "kpi", (rows) => rows.map(toKpiOperasional));
      assert.deepEqual(
        hasil.map((k) => k.id),
        ["entri", "validasi", "waspada", "cs"],
      );
      assert.equal(hasil.length, fixture.length);
      for (const k of hasil) {
        assert.equal(k.tren.length, 14, `${k.id}: 14 hari berakhir hari ini`);
        assert.ok(["angka", "persen"].includes(k.format), k.format);
        assert.ok(
          ["data", "normal", "waspada", "defisit"].includes(k.tone),
          k.tone,
        );
        assert.ok(k.href.startsWith("/"));
        assert.ok(k.keteranganDelta.length > 0);
      }
    });
  }

  it("Koordinator: semua kartu bernada netral `data`, dan kartu bernilai nol adalah data sah", () => {
    const { hasil } = kontrak<
      KpiResponse[],
      ReturnType<typeof toKpiOperasional>[]
    >("koordinator", "kpi", (rows) => rows.map(toKpiOperasional));
    assert.deepEqual(new Set(hasil.map((k) => k.tone)), new Set(["data"]));
    const cs = hasil.find((k) => k.id === "cs")!;
    assert.equal(cs.nilai, 0, "kabupaten tanpa cold storage operasional");
    assert.deepEqual(cs.tren, Array(14).fill(0));
  });

  it("provinsi: nada mengikuti target (bukan semuanya `data`)", () => {
    const { hasil } = kontrak<
      KpiResponse[],
      ReturnType<typeof toKpiOperasional>[]
    >("provinsi", "kpi", (rows) => rows.map(toKpiOperasional));
    assert.ok(hasil.some((k) => k.tone !== "data"));
  });

  it("provinsi terfilter: label memuat tanggal acuan dan tren sepanjang periode", () => {
    const { hasil } = kontrak<
      KpiResponse[],
      ReturnType<typeof toKpiOperasional>[]
    >("provinsi", "kpi-terfilter", (rows) => rows.map(toKpiOperasional));
    assert.equal(
      hasil.find((k) => k.id === "entri")!.label,
      "Entri harga 5 Nov 2026",
    );
    for (const k of hasil) assert.equal(k.tren.length, 5, "1 sampai 5 Nov");
    assert.deepEqual(
      new Set(hasil.map((k) => k.tone)),
      new Set(["data"]),
      "tampilan terfilter selalu netral",
    );
  });
});

describe("kontrak /heatmap", () => {
  it("provinsi: kabupaten bernama ringkas, semua sel merujuk kabupaten dan komoditas yang dikirim", () => {
    const { fixture, hasil } = kontrak<
      HeatmapResponse,
      ReturnType<typeof toHeatmap>
    >("provinsi", "heatmap", toHeatmap);
    assert.equal(hasil.kabupaten.length, 11);
    assert.equal(hasil.komoditas.length, 14);
    assert.equal(hasil.sel.length, fixture.sel.length);
    assert.equal(hasil.sel.length, 142);
    assert.ok(
      hasil.kabupaten.every((k) => !/^(KABUPATEN|KOTA) /.test(k.nama)),
      "nama resmi diganti nama ringkas",
    );
    assert.deepEqual(hasil.kabupaten[0], {
      kode: "1103",
      nama: "Aceh Selatan",
    });
    const kab = new Set(hasil.kabupaten.map((k) => k.kode));
    const kom = new Set(hasil.komoditas.map((k) => k.kode));
    assert.ok(
      hasil.sel.every(
        (s) => kab.has(s.kabupatenKode) && kom.has(s.komoditasKode),
      ),
    );
    assert.deepEqual(
      new Set(hasil.sel.map((s) => s.sumber)),
      new Set(["LAPANGAN", "EKSTERNAL"]),
    );
    assert.equal(hasil.sel.filter((s) => s.sumber === "EKSTERNAL").length, 4);
  });

  it("koordinator: hanya kabupaten tugasnya; terfilter: satu sel", () => {
    const k = kontrak<HeatmapResponse, ReturnType<typeof toHeatmap>>(
      "koordinator",
      "heatmap",
      toHeatmap,
    ).hasil;
    assert.deepEqual(k.kabupaten, [{ kode: "1171", nama: "Banda Aceh" }]);
    assert.equal(k.sel.length, 14);
    const f = kontrak<HeatmapResponse, ReturnType<typeof toHeatmap>>(
      "provinsi",
      "heatmap-terfilter",
      toHeatmap,
    ).hasil;
    assert.equal(f.sel.length, 1);
    assert.equal(f.sel[0].komoditasKode, "CBM-KRT");
  });

  it("kabupaten di luar cakupan: tiga larik kosong, bukan galat", () => {
    const { hasil } = kontrak<HeatmapResponse, ReturnType<typeof toHeatmap>>(
      "koordinator",
      "heatmap-kosong",
      toHeatmap,
    );
    assert.deepEqual(hasil, { kabupaten: [], komoditas: [], sel: [] });
  });
});

describe("kontrak /aktivitas-lapangan", () => {
  for (const persona of PERSONA) {
    it(`${persona}: satu baris per kabupaten, capaian dalam 0 sampai 100`, () => {
      const { fixture, hasil } = kontrak<
        AktivitasLapanganResponse[],
        ReturnType<typeof toAktivitasLapangan>[]
      >(persona, "aktivitas-lapangan", (rows) => rows.map(toAktivitasLapangan));
      assert.equal(hasil.length, fixture.length);
      assert.equal(hasil.length, persona === "provinsi" ? 10 : 1);
      for (const a of hasil) {
        const pct = capaianPct(a);
        assert.ok(pct >= 0 && pct <= 100, `${a.nama}: ${pct}`);
        assert.ok(!/^(KABUPATEN|KOTA) /.test(a.nama));
      }
    });
  }
});

describe("kontrak /pembanding-harga", () => {
  // Kode komoditas ada di respons, tetapi kartu hanya menampilkan namanya.
  const ABAIKAN = ["items[].komoditas_kode"];
  for (const persona of PERSONA) {
    it(`${persona}: ringkasan dan item selisih terbesar lebih dulu`, () => {
      const { fixture, hasil } = kontrak<
        PembandingHargaResponse,
        ReturnType<typeof toPembandingHarga>
      >(persona, "pembanding-harga", toPembandingHarga, ABAIKAN);
      assert.equal(hasil.pasangan, fixture.pasangan);
      assert.equal(hasil.sepakat, fixture.sepakat);
      assert.equal(hasil.batasPct, 10);
      assert.equal(hasil.items.length, fixture.items.length);
      const mutlak = hasil.items.map((i) => Math.abs(i.selisihPct));
      assert.deepEqual(
        mutlak,
        [...mutlak].sort((a, b) => b - a),
        "urutan server dipertahankan",
      );
      for (const i of hasil.items)
        assert.match(i.tanggal, /^\d{4}-\d{2}-\d{2}$/);
    });
  }
});

describe("kontrak /cuaca", () => {
  for (const persona of PERSONA) {
    it(`${persona}: satu baris per kabupaten/kota yang punya data`, () => {
      const { fixture, hasil } = kontrak<
        CuacaResponse[],
        ReturnType<typeof toCuacaWilayah>[]
      >(persona, "cuaca", (rows) => rows.map(toCuacaWilayah));
      assert.equal(hasil.length, fixture.length);
      assert.equal(hasil.length, persona === "provinsi" ? 23 : 1);
      assert.ok(
        hasil[0].anomali,
        "anomali dan curah hujan terbesar lebih dulu",
      );
      assert.ok(hasil.every((c) => Number.isInteger(c.kelembapan)));
    });
  }
});

describe("kontrak /aktivitas-petugas", () => {
  it("provinsi: sepuluh petugas, nama ringkas, waktu terbaca, tanpa data pribadi", () => {
    const { fixture, hasil } = kontrak<
      AktivitasPetugasResponse,
      ReturnType<typeof toAktivitasPetugas>
    >("provinsi", "aktivitas-petugas", toAktivitasPetugas);
    assert.equal(hasil.petugas.length, fixture.petugas.length);
    assert.equal(hasil.petugas.length, 10);
    assert.equal(hasil.jendelaHari, 7);
    assert.match(hasil.tanggal, /^\d{4}-\d{2}-\d{2}$/);
    for (const p of hasil.petugas) {
      assert.match(p.id, /^[0-9a-f-]{36}$/, "UUID pengguna");
      assert.ok(p.nama.length > 0);
      assert.ok(!/^(KABUPATEN|KOTA) /.test(p.kabupatenNama));
      assert.ok(p.targetHarian > 0);
      assert.ok(p.entriTerakhirPada === null || waktuSah(p.entriTerakhirPada));
      assert.deepEqual(
        Object.keys(p).sort(),
        [
          "id",
          "nama",
          "kabupatenKode",
          "kabupatenNama",
          "entriHariIni",
          "targetHarian",
          "menungguValidasi",
          "ditolakJendela",
          "entriTerakhirPada",
          "tugasAktif",
          "tugasSelesai",
          "tugasTerlambat",
        ].sort(),
        "tidak ada email atau telepon",
      );
    }
    assert.equal(
      new Set(hasil.petugas.map((p) => p.id)).size,
      10,
      "ID stabil dan unik",
    );
  });

  it("koordinator: hanya petugas di kabupatennya; di luar cakupan: daftar kosong", () => {
    const { hasil } = kontrak<
      AktivitasPetugasResponse,
      ReturnType<typeof toAktivitasPetugas>
    >("koordinator", "aktivitas-petugas", toAktivitasPetugas);
    assert.deepEqual(
      hasil.petugas.map((p) => [p.nama, p.kabupatenKode, p.kabupatenNama]),
      [["Aulia Rahman", "1171", "Banda Aceh"]],
    );
    const kosong = kontrak<
      AktivitasPetugasResponse,
      ReturnType<typeof toAktivitasPetugas>
    >("koordinator", "aktivitas-petugas-kosong", toAktivitasPetugas).hasil;
    assert.deepEqual(kosong.petugas, []);
    assert.equal(kosong.jendelaHari, 7);
  });

  it("petugas yang belum pernah mencatat (jawaban nyata): waktu entri terakhir null dan semua hitungan nol", () => {
    const { hasil } = kontrak<AktivitasPetugasResponse, ReturnType<typeof toAktivitasPetugas>>("provinsi", "aktivitas-petugas-tanpa-entri", toAktivitasPetugas);
    assert.equal(hasil.petugas.length, 11);
    const baru = hasil.petugas.find((p) => p.nama === "Petugas Baru Contoh")!;
    assert.equal(baru.entriTerakhirPada, null);
    assert.equal(baru.kabupatenNama, "Simeulue");
    assert.deepEqual(
      [baru.entriHariIni, baru.menungguValidasi, baru.ditolakJendela, baru.tugasAktif, baru.tugasSelesai, baru.tugasTerlambat],
      [0, 0, 0, 0, 0, 0],
    );
    assert.equal(baru.targetHarian, 14, "target tetap jumlah komoditas aktif");
    assert.ok(hasil.petugas.filter((p) => p !== baru).every((p) => p.entriTerakhirPada !== null));
  });

  it("jumlah entri per petugas menjumlah ke capaian per kabupaten (satu petugas per kabupaten di data demo)", () => {
    const petugas = toAktivitasPetugas(
      muatFixture<AktivitasPetugasResponse>("provinsi", "aktivitas-petugas"),
    ).petugas;
    const wilayah = muatFixture<AktivitasLapanganResponse[]>(
      "provinsi",
      "aktivitas-lapangan",
    ).map(toAktivitasLapangan);
    for (const w of wilayah) {
      const milik = petugas.filter((p) => p.kabupatenKode === w.kode);
      assert.equal(
        milik.reduce((t, p) => t + p.entriHariIni, 0),
        w.entri,
        w.nama,
      );
      assert.equal(
        milik.reduce((t, p) => t + p.targetHarian, 0),
        w.target,
        w.nama,
      );
    }
  });
});

describe("kontrak /stok", () => {
  for (const persona of PERSONA) {
    it(`${persona}: ringkasan dan rincian komoditas konsisten`, () => {
      const { fixture, hasil } = kontrak<
        StokResponse,
        ReturnType<typeof toStok>
      >(persona, "stok", toStok);
      assert.equal(hasil.komoditas.length, fixture.komoditas.length);
      assert.equal(hasil.ambangKedaluwarsaHari, 7);
      assert.equal(hasil.ambangRisiko, 0.6);
      const jumlah = hasil.komoditas.reduce((t, k) => t + k.totalTon, 0);
      assert.ok(
        Math.abs(jumlah - hasil.ringkasan.totalTon) <
          0.1 * Math.max(1, hasil.komoditas.length),
        "jumlah baris boleh berselisih pembulatan",
      );
      const urut = hasil.komoditas.map((k) => k.totalTon);
      assert.deepEqual(
        urut,
        [...urut].sort((a, b) => b - a),
        "terbesar lebih dulu",
      );
    });
  }

  it("utilisasi stok sama dengan kartu KPI `cs` (definisi yang sama, dua endpoint)", () => {
    for (const persona of PERSONA) {
      const kpi = muatFixture<KpiResponse[]>(persona, "kpi").map(
        toKpiOperasional,
      );
      const stok = toStok(muatFixture<StokResponse>(persona, "stok"));
      assert.equal(
        stok.ringkasan.utilisasiPct,
        kpi.find((k) => k.id === "cs")!.nilai,
        persona,
      );
    }
  });

  it("koordinator di kabupaten tanpa cold storage: stok nol dan larik kosong, bukan galat", () => {
    const { hasil } = kontrak<StokResponse, ReturnType<typeof toStok>>(
      "koordinator",
      "stok",
      toStok,
    );
    assert.equal(hasil.ringkasan.totalTon, 0);
    assert.equal(hasil.ringkasan.kapasitasTon, 0);
    assert.equal(hasil.ringkasan.utilisasiPct, 0);
    assert.deepEqual(hasil.komoditas, []);
  });
});

describe("kontrak /produsen", () => {
  it("provinsi: sisi harga, margin, dan produksi null ditangani; margin diteruskan dari server", () => {
    const { fixture, hasil } = kontrak<
      ProdusenResponse,
      ReturnType<typeof toProdusen>
    >("provinsi", "produsen", toProdusen);
    assert.equal(hasil.komoditas.length, fixture.komoditas.length);
    assert.equal(hasil.komoditas.length, 8);
    const tanpaHarga = hasil.komoditas.filter((k) => k.produsen === null);
    assert.ok(tanpaHarga.length > 0, "ada komoditas yang hanya punya produksi");
    for (const k of tanpaHarga)
      assert.equal(
        k.margin,
        null,
        `${k.komoditasKode}: margin hanya bila kedua sisi ada`,
      );
    for (const k of hasil.komoditas.filter((k) => k.margin)) {
      assert.ok(k.produsen && k.konsumen, k.komoditasKode);
      assert.equal(
        k.margin!.selisih,
        k.konsumen.harga - k.produsen.harga,
        "server menghitung dari harga yang sudah dibulatkan",
      );
    }
    assert.ok(
      hasil.komoditas.every(
        (k) => k.produksi && k.produksi.demo === (k.produksi.sumber === "DEMO"),
      ),
    );
    assert.match(hasil.dari, /^\d{4}-\d{2}-\d{2}$/);
  });

  it("koordinator: produksi null (tak ada neraca kabupaten), harga tetap ada", () => {
    const { hasil } = kontrak<ProdusenResponse, ReturnType<typeof toProdusen>>(
      "koordinator",
      "produsen",
      toProdusen,
    );
    assert.equal(hasil.komoditas.length, 4);
    assert.ok(
      hasil.komoditas.every(
        (k) => k.produksi === null && k.produsen && k.konsumen && k.margin,
      ),
    );
  });
});

describe("kontrak /logistik", () => {
  it("provinsi: ringkasan tiga jenis, transaksi MASUK tanpa asal, rute aktif", () => {
    const { fixture, hasil } = kontrak<
      LogistikResponse,
      ReturnType<typeof toLogistik>
    >("provinsi", "logistik", toLogistik);
    assert.deepEqual(Object.keys(hasil.ringkasan).sort(), [
      "keluar",
      "masuk",
      "pindah",
    ]);
    assert.equal(hasil.terbaru.length, fixture.terbaru.length);
    assert.equal(hasil.komoditas.length, fixture.komoditas.length);
    assert.equal(hasil.rute.items.length, fixture.rute.items.length);
    assert.ok(
      hasil.rute.jumlah >= hasil.rute.items.length,
      "jumlah atas SEMUA rute aktif, items hanya yang terbesar",
    );
    for (const p of hasil.terbaru) {
      assert.ok(waktuSah(p.waktu));
      if (p.jenis === "MASUK") assert.equal(p.asal, null);
      if (p.jenis === "KELUAR") assert.equal(p.tujuan, null);
      for (const f of [p.asal, p.tujuan])
        if (f) assert.ok(!/^(KABUPATEN|KOTA) /.test(f.kabupatenNama));
    }
    for (const r of hasil.rute.items)
      assert.ok(
        !/^(KABUPATEN|KOTA) /.test(r.asalNama) &&
          !/^(KABUPATEN|KOTA) /.test(r.tujuanNama),
      );
    const biaya = hasil.rute.items.map((r) => r.biaya);
    assert.deepEqual(
      biaya,
      [...biaya].sort((a, b) => b - a),
      "biaya terbesar lebih dulu",
    );
  });

  it("koordinator: tanpa transaksi larik kosong dan ketiga jenis tetap ada; rute yang menyentuh kabupatennya tetap tampil", () => {
    const { hasil } = kontrak<LogistikResponse, ReturnType<typeof toLogistik>>(
      "koordinator",
      "logistik",
      toLogistik,
    );
    assert.deepEqual(hasil.terbaru, []);
    assert.deepEqual(hasil.komoditas, []);
    assert.deepEqual(hasil.ringkasan.masuk, { jumlah: 0, volumeTon: 0 });
    assert.equal(hasil.rute.jumlah, 1);
    assert.equal(hasil.rute.items[0].tujuanKode, "1171");
  });
});

describe("kontrak /v1/siklus/aktif", () => {
  const siklus = (persona: PersonaFixture, nama: string) =>
    kontrak<SiklusAktifResponse, ReturnType<typeof toSiklusAktif>>(
      persona,
      nama,
      toSiklusAktif,
    ).hasil;

  for (const persona of PERSONA) {
    it(`${persona}: siklus dari seed (tidak otomatis) tanpa alasan`, () => {
      assert.deepEqual(siklus(persona, "siklus-aktif"), {
        siklus: "tekanan",
        sejak: "2026-11-04",
        otomatis: false,
        alasan: null,
      });
    });
  }

  it("otomatis TEKANAN: pemicu dan aturan yang ditekan beserta rujukan alert", () => {
    const s = siklus("provinsi", "siklus-aktif-otomatis-tekanan");
    assert.equal(s.siklus, "tekanan");
    assert.equal(s.otomatis, true);
    assert.ok(s.alasan);
    assert.match(s.alasan.ringkasan, /^Siklus awal TEKANAN/);
    assert.equal(s.alasan.pemicu.length, 2);
    assert.deepEqual(
      s.alasan.pemicu.map((p) => p.aturan),
      ["SKL-TKN-LONJAKAN-HARGA", "SKL-TKN-ANOMALI-CUACA"],
    );
    assert.ok(
      s.alasan.pemicu.every(
        (p) => p.rujukan.length > 0 && p.satuan === "alert",
      ),
    );
    assert.deepEqual(
      s.alasan.ditekan.map((p) => [p.aturan, p.siklus, p.satuan, p.rincian]),
      [["SKL-SRP-UTILISASI-STOK", "surplus", "%", "780 dari 950 ton"]],
    );
  });

  it("otomatis NORMAL: pemicu dan ditekan larik kosong (bukan null)", () => {
    const s = siklus("provinsi", "siklus-aktif-otomatis-normal");
    assert.equal(s.siklus, "normal");
    assert.deepEqual(s.alasan?.pemicu, []);
    assert.deepEqual(s.alasan?.ditekan, []);
  });

  it("otomatis SURPLUS: nilai utilisasi apa adanya dan rincian angka", () => {
    const s = siklus("provinsi", "siklus-aktif-otomatis-surplus");
    assert.equal(s.siklus, "surplus");
    assert.equal(s.alasan?.pemicu[0].indikator, "UTILISASI_STOK");
    assert.equal(s.alasan?.pemicu[0].rincian, "780 dari 950 ton");
    assert.deepEqual(s.alasan?.pemicu[0].rujukan, [], "rujukan alert hanya untuk ALERT_EWS, tetap larik");
  });
});
