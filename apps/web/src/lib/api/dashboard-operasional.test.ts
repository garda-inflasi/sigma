import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { nilaiTakWajar } from "@/test/jejak-kunci";
import { toAktivitasLapangan } from "./dashboard";
import {
  toAktivitasPetugas,
  toLogistik,
  toProdusen,
  toStok,
  type AktivitasPetugasItemResponse,
  type LogistikResponse,
  type ProdusenKomoditasResponse,
  type StokResponse,
} from "./dashboard-operasional";
import {
  mockAktivitasLapangan,
  mockAktivitasPetugas,
  mockLogistik,
  mockProdusen,
  mockStok,
} from "./dashboard-operasional-mock";

const petugas = (
  sisip: Partial<AktivitasPetugasItemResponse> = {},
): AktivitasPetugasItemResponse => ({
  id: "03dc79aa-484b-322e-a0ae-3937fb4f3a23",
  nama: "Aulia Rahman",
  kabupaten_kode: "1171",
  kabupaten_nama: "KOTA BANDA ACEH",
  entri_hari_ini: 10,
  target_harian: 14,
  menunggu_validasi: 19,
  ditolak_jendela: 11,
  entri_terakhir_pada: "2026-11-10T14:26:21.480678+07:00",
  tugas_aktif: 3,
  tugas_selesai: 1,
  tugas_terlambat: 1,
  ...sisip,
});

describe("toAktivitasPetugas", () => {
  it("memetakan semua kolom ke camelCase dan memakai nama kabupaten ringkas", () => {
    assert.deepEqual(
      toAktivitasPetugas({
        tanggal: "2026-11-10",
        jendela_hari: 7,
        petugas: [petugas()],
      }),
      {
        tanggal: "2026-11-10",
        jendelaHari: 7,
        petugas: [
          {
            id: "03dc79aa-484b-322e-a0ae-3937fb4f3a23",
            nama: "Aulia Rahman",
            kabupatenKode: "1171",
            kabupatenNama: "Banda Aceh",
            entriHariIni: 10,
            targetHarian: 14,
            menungguValidasi: 19,
            ditolakJendela: 11,
            entriTerakhirPada: "2026-11-10T14:26:21.480678+07:00",
            tugasAktif: 3,
            tugasSelesai: 1,
            tugasTerlambat: 1,
          },
        ],
      },
    );
  });

  it("petugas yang belum pernah mencatat: waktu null tetap null; kode kabupaten tak dikenal memakai nama resmi", () => {
    const {
      petugas: [p],
    } = toAktivitasPetugas({
      tanggal: "2026-11-10",
      jendela_hari: 7,
      petugas: [
        petugas({
          entri_terakhir_pada: null,
          kabupaten_kode: "9999",
          kabupaten_nama: "KABUPATEN BARU",
          entri_hari_ini: 0,
        }),
      ],
    });
    assert.equal(p.entriTerakhirPada, null);
    assert.equal(p.kabupatenNama, "Kab. Baru");
    assert.equal(p.entriHariIni, 0);
  });

  it("daftar kosong tetap valid; urutan server dipertahankan", () => {
    assert.deepEqual(
      toAktivitasPetugas({
        tanggal: "2026-11-10",
        jendela_hari: 7,
        petugas: [],
      }).petugas,
      [],
    );
    const nama = toAktivitasPetugas({
      tanggal: "x",
      jendela_hari: 7,
      petugas: [
        petugas({ id: "b", nama: "B" }),
        petugas({ id: "a", nama: "A" }),
      ],
    }).petugas.map((p) => p.nama);
    assert.deepEqual(nama, ["B", "A"]);
  });
});

const stok = (): StokResponse => ({
  tanggal: "2026-11-10",
  ambang_kedaluwarsa_hari: 7,
  ambang_risiko: 0.6,
  ringkasan: {
    total_ton: 780,
    jumlah_lot: 21,
    jumlah_fasilitas: 3,
    kapasitas_ton: 950,
    fasilitas_operasional: 3,
    utilisasi_pct: 82.1,
    mendekati_kedaluwarsa_ton: 114.4,
    risiko_tinggi_ton: 172.7,
    kedaluwarsa_lot: 2,
    kedaluwarsa_ton: 8.5,
    di_luar_operasional_ton: 33,
  },
  komoditas: [
    {
      komoditas_kode: "IKN-TGK",
      komoditas_nama: "Ikan tongkol",
      total_ton: 240,
      jumlah_lot: 5,
      jumlah_fasilitas: 1,
      mendekati_kedaluwarsa_ton: 0,
      risiko_tinggi_ton: 106.8,
    },
  ],
});

describe("toStok", () => {
  it("memetakan ringkasan dan komoditas apa adanya (tidak menghitung ulang utilisasi)", () => {
    const s = toStok({
      ...stok(),
      ringkasan: { ...stok().ringkasan, utilisasi_pct: 50 },
    });
    assert.equal(
      s.ringkasan.utilisasiPct,
      50,
      "angka server, bukan 100 x 780 / 950",
    );
    assert.equal(s.ringkasan.totalTon, 780);
    assert.equal(s.ringkasan.kapasitasTon, 950);
    assert.equal(s.ringkasan.kedaluwarsaLot, 2);
    assert.equal(s.ringkasan.diLuarOperasionalTon, 33);
    assert.equal(s.ambangKedaluwarsaHari, 7);
    assert.equal(s.ambangRisiko, 0.6);
    assert.deepEqual(s.komoditas[0], {
      komoditasKode: "IKN-TGK",
      komoditasNama: "Ikan tongkol",
      totalTon: 240,
      jumlahLot: 5,
      jumlahFasilitas: 1,
      mendekatiKedaluwarsaTon: 0,
      risikoTinggiTon: 106.8,
    });
  });

  it("tanpa stok: semua nol dan komoditas []", () => {
    const nol = Object.fromEntries(
      Object.keys(stok().ringkasan).map((k) => [k, 0]),
    ) as unknown as StokResponse["ringkasan"];
    const s = toStok({ ...stok(), ringkasan: nol, komoditas: [] });
    assert.deepEqual(nilaiTakWajar(s), []);
    assert.deepEqual(s.komoditas, []);
    assert.equal(s.ringkasan.totalTon, 0);
  });
});

const sisi = (harga: number) => ({
  harga,
  jumlah_entri: 5,
  jumlah_kabupaten: 2,
});

describe("toProdusen", () => {
  const komoditas = (
    sisip: Partial<ProdusenKomoditasResponse> = {},
  ): ProdusenKomoditasResponse => ({
    komoditas_kode: "CBM-KRT",
    komoditas_nama: "Cabai merah keriting",
    satuan: "kg",
    produsen: sisi(41682),
    konsumen: sisi(57470),
    margin: { selisih: 15788, persen_produsen: 37.9, persen_konsumen: 27.5 },
    produksi: { ton: 3100, periode: "2026-10-01", sumber: "DEMO", demo: true },
    ...sisip,
  });

  it("memetakan sisi harga, margin, dan produksi", () => {
    const {
      komoditas: [k],
    } = toProdusen({
      dari: "2026-11-04",
      sampai: "2026-11-10",
      komoditas: [komoditas()],
    });
    assert.deepEqual(k.produsen, {
      harga: 41682,
      jumlahEntri: 5,
      jumlahKabupaten: 2,
    });
    assert.deepEqual(k.konsumen, {
      harga: 57470,
      jumlahEntri: 5,
      jumlahKabupaten: 2,
    });
    assert.deepEqual(k.margin, {
      selisih: 15788,
      persenProdusen: 37.9,
      persenKonsumen: 27.5,
    });
    assert.deepEqual(k.produksi, {
      ton: 3100,
      periode: "2026-10-01",
      sumber: "DEMO",
      demo: true,
    });
  });

  it("margin diteruskan dari server dan tidak dihitung ulang dari kedua sisi harga", () => {
    const {
      komoditas: [k],
    } = toProdusen({
      dari: "a",
      sampai: "b",
      komoditas: [
        komoditas({
          margin: { selisih: 1, persen_produsen: 2, persen_konsumen: 3 },
        }),
      ],
    });
    assert.deepEqual(k.margin, {
      selisih: 1,
      persenProdusen: 2,
      persenKonsumen: 3,
    });
  });

  it("blok null tetap null: tanpa harga produsen tidak ada margin; tanpa neraca tidak ada produksi (bukan nol)", () => {
    const {
      komoditas: [k],
    } = toProdusen({
      dari: "a",
      sampai: "b",
      komoditas: [
        komoditas({
          produsen: null,
          konsumen: null,
          margin: null,
          produksi: null,
        }),
      ],
    });
    assert.deepEqual(
      [k.produsen, k.konsumen, k.margin, k.produksi],
      [null, null, null, null],
    );
  });

  it("margin negatif (konsumen di bawah produsen) dan produksi resmi (bukan contoh) tidak diubah", () => {
    const {
      komoditas: [k],
    } = toProdusen({
      dari: "a",
      sampai: "b",
      komoditas: [
        komoditas({
          margin: {
            selisih: -500,
            persen_produsen: -2.1,
            persen_konsumen: -2.2,
          },
          produksi: {
            ton: 1,
            periode: "2026-10-01",
            sumber: "BPS",
            demo: false,
          },
        }),
      ],
    });
    assert.equal(k.margin?.selisih, -500);
    assert.equal(k.produksi?.demo, false);
    assert.equal(k.produksi?.sumber, "BPS");
  });
});

const jenis = (jumlah: number, volume_ton: number) => ({ jumlah, volume_ton });
const logistik = (): LogistikResponse => ({
  dari: "2026-11-04",
  sampai: "2026-11-10",
  ringkasan: {
    masuk: jenis(1, 36),
    keluar: jenis(0, 0),
    pindah: jenis(1, 5.5),
  },
  komoditas: [
    {
      komoditas_kode: "CBM-KRT",
      komoditas_nama: "Cabai merah keriting",
      masuk: jenis(1, 36),
      keluar: jenis(0, 0),
      pindah: jenis(1, 5.5),
    },
  ],
  terbaru: [
    {
      id: "6",
      waktu: "2026-11-07T14:55:54.69624+07:00",
      jenis: "MASUK",
      komoditas_kode: "CBM-KRT",
      komoditas_nama: "Cabai merah keriting",
      volume_ton: 36,
      asal: null,
      tujuan: {
        id: "aef12e36-fce3-3252-8c8a-ccdf9bd9dfcd",
        kode: "CS-01",
        nama: "Cold Storage Lambaro",
        kabupaten_kode: "1108",
        kabupaten_nama: "KABUPATEN ACEH BESAR",
      },
    },
    {
      id: "7",
      waktu: "2026-11-07T15:00:00+07:00",
      jenis: "PINDAH",
      komoditas_kode: "CBM-KRT",
      komoditas_nama: "Cabai merah keriting",
      volume_ton: 5.5,
      asal: {
        id: "a",
        kode: null,
        nama: "Gudang A",
        kabupaten_kode: "1106",
        kabupaten_nama: "KABUPATEN ACEH TENGAH",
      },
      tujuan: {
        id: "b",
        kode: "CS-02",
        nama: "Gudang B",
        kabupaten_kode: "1174",
        kabupaten_nama: "KOTA LHOKSEUMAWE",
      },
    },
  ],
  rute: {
    jumlah: 12,
    volume_ton: 300,
    biaya: 99_000_000,
    items: [
      {
        id: "1",
        kode: null,
        asal_kode: "1106",
        asal: "KABUPATEN ACEH TENGAH",
        tujuan_kode: "1171",
        tujuan: "KOTA BANDA ACEH",
        komoditas_kode: "CBM-KRT",
        komoditas_nama: "Cabai merah keriting",
        volume_ton: 42,
        jarak_km: 268,
        jam_tempuh: 6,
        biaya: 47_275_123,
        status: "USULAN",
        sumber: "DEMO",
        dibuat_pada: "2026-11-10T14:55:54.678602+07:00",
      },
    ],
  },
});

describe("toLogistik", () => {
  it("memetakan ringkasan, komoditas, transaksi, dan rute dengan nama kabupaten ringkas", () => {
    const l = toLogistik(logistik());
    assert.deepEqual(l.ringkasan, {
      masuk: { jumlah: 1, volumeTon: 36 },
      keluar: { jumlah: 0, volumeTon: 0 },
      pindah: { jumlah: 1, volumeTon: 5.5 },
    });
    assert.deepEqual(l.komoditas[0].pindah, { jumlah: 1, volumeTon: 5.5 });
    assert.equal(l.komoditas[0].komoditasNama, "Cabai merah keriting");
    assert.deepEqual(l.terbaru[0].tujuan, {
      id: "aef12e36-fce3-3252-8c8a-ccdf9bd9dfcd",
      kode: "CS-01",
      nama: "Cold Storage Lambaro",
      kabupatenKode: "1108",
      kabupatenNama: "Aceh Besar",
    });
    assert.equal(l.rute.items[0].asalNama, "Aceh Tengah");
    assert.equal(l.rute.items[0].tujuanNama, "Banda Aceh");
    assert.equal(l.rute.items[0].biaya, 47_275_123);
    assert.equal(
      l.rute.jumlah,
      12,
      "jumlah atas semua rute aktif, bukan hanya items",
    );
    assert.equal(l.rute.items.length, 1);
  });

  it("MASUK tanpa asal, PINDAH dengan kedua ujung, kode fasilitas boleh null", () => {
    const [masuk, pindah] = toLogistik(logistik()).terbaru;
    assert.equal(masuk.asal, null);
    assert.ok(masuk.tujuan);
    assert.equal(pindah.asal?.kode, null);
    assert.equal(pindah.tujuan?.kabupatenNama, "Lhokseumawe");
  });

  it("tanpa data: ketiga jenis nol, larik [], rute kosong", () => {
    const l = toLogistik({
      dari: "a",
      sampai: "b",
      ringkasan: {
        masuk: jenis(0, 0),
        keluar: jenis(0, 0),
        pindah: jenis(0, 0),
      },
      komoditas: [],
      terbaru: [],
      rute: { jumlah: 0, volume_ton: 0, biaya: 0, items: [] },
    });
    assert.deepEqual(nilaiTakWajar(l), []);
    assert.deepEqual([l.komoditas, l.terbaru, l.rute.items], [[], [], []]);
    assert.equal(l.ringkasan.keluar.volumeTon, 0);
  });
});

describe("mock mode tanpa backend", () => {
  it("petugas: berasal dari referensi, tanpa data pribadi, terurut menurut kabupaten dan nama", () => {
    const r = mockAktivitasPetugas();
    assert.equal(r.petugas.length, 12);
    assert.equal(r.jendela_hari, 7);
    assert.match(r.tanggal, /^\d{4}-\d{2}-\d{2}$/);
    for (const p of r.petugas) {
      assert.match(p.id, /^[0-9a-f-]{36}$/);
      assert.match(
        p.kabupaten_nama,
        /^(KABUPATEN|KOTA) [A-Z ]+$/,
        "nama resmi seperti API",
      );
      assert.equal(p.target_harian, 14);
      assert.deepEqual(Object.keys(p).sort(), Object.keys(petugas()).sort());
    }
    const urut = r.petugas.map((p) => `${p.kabupaten_nama}|${p.nama}`);
    assert.deepEqual(
      urut,
      [...urut].sort(
        (a, b) =>
          a.split("|")[0].localeCompare(b.split("|")[0]) ||
          a.split("|")[1].localeCompare(b.split("|")[1]),
      ),
    );
    assert.deepEqual(nilaiTakWajar(toAktivitasPetugas(r)), []);
  });

  it("petugas: jumlah entri per kabupaten sama dengan capaian per wilayah mock; filter kabupaten", () => {
    const wilayah = mockAktivitasLapangan();
    const hasil = toAktivitasPetugas(mockAktivitasPetugas());
    for (const w of wilayah) {
      const milik = hasil.petugas.filter((p) => p.kabupatenKode === w.kode);
      assert.equal(
        milik.reduce((t, p) => t + p.entriHariIni, 0),
        w.entri,
        w.nama,
      );
      assert.equal(milik.length, w.jumlahPetugas, w.nama);
    }
    assert.deepEqual(
      mockAktivitasPetugas("1171").petugas.map((p) => p.kabupaten_kode),
      ["1171", "1171"],
    );
    assert.deepEqual(mockAktivitasPetugas("9999").petugas, []);
    assert.ok(
      wilayah.every(
        (w) =>
          toAktivitasLapangan({
            kabupaten_kode: w.kode,
            kabupaten_nama: w.nama,
            jumlah_petugas: w.jumlahPetugas,
            entri: w.entri,
            target: w.target,
          }).target === w.target,
      ),
    );
  });

  it("stok: ringkasan konsisten dengan lot dan komoditas, fasilitas perawatan di luar stok berlaku", () => {
    const r = mockStok();
    const s = toStok(r);
    assert.ok(s.ringkasan.totalTon > 0 && s.ringkasan.kapasitasTon > 0);
    assert.ok(
      s.ringkasan.diLuarOperasionalTon > 0,
      "Gudang Dingin Meulaboh sedang perawatan",
    );
    assert.equal(
      s.ringkasan.utilisasiPct,
      Math.round((1000 * s.ringkasan.totalTon) / s.ringkasan.kapasitasTon) / 10,
    );
    assert.ok(
      Math.abs(
        s.komoditas.reduce((t, k) => t + k.totalTon, 0) - s.ringkasan.totalTon,
      ) <
        0.1 * s.komoditas.length,
    );
    assert.equal(
      s.komoditas.reduce((t, k) => t + k.jumlahLot, 0),
      s.ringkasan.jumlahLot,
    );
    assert.deepEqual(nilaiTakWajar(s), []);
  });

  it("stok: filter komoditas menyempitkan stok tetapi tidak kapasitas; kabupaten tanpa fasilitas menghasilkan nol", () => {
    const semua = toStok(mockStok());
    const satu = toStok(mockStok({ komoditasKode: "CBM-KRT" }));
    assert.ok(satu.ringkasan.totalTon < semua.ringkasan.totalTon);
    assert.equal(satu.ringkasan.kapasitasTon, semua.ringkasan.kapasitasTon);
    assert.deepEqual(
      satu.komoditas.map((k) => k.komoditasKode),
      ["CBM-KRT"],
    );
    const kosong = toStok(mockStok({ kabupatenKode: "9999" }));
    assert.equal(kosong.ringkasan.totalTon, 0);
    assert.equal(kosong.ringkasan.utilisasiPct, 0);
    assert.deepEqual(kosong.komoditas, []);
  });

  it("produsen: pilot lebih dulu; margin hanya bila kedua sisi harga ada dan konsisten dengan harga", () => {
    const p = toProdusen(mockProdusen());
    assert.ok(p.komoditas.length > 0);
    const pilot = p.komoditas.map((k) => k.komoditasKode).slice(0, 3);
    assert.deepEqual([...pilot].sort(), ["BRS-MED", "BWM", "CBM-KRT"].sort());
    for (const k of p.komoditas) {
      assert.equal(
        k.margin !== null,
        k.produsen !== null && k.konsumen !== null,
        k.komoditasKode,
      );
      if (k.margin && k.produsen && k.konsumen) {
        assert.equal(k.margin.selisih, k.konsumen.harga - k.produsen.harga);
        assert.equal(
          k.margin.persenProdusen,
          Math.round((k.margin.selisih / k.produsen.harga) * 1000) / 10,
        );
      }
      assert.ok(
        k.produksi && k.produksi.demo === (k.produksi.sumber === "DEMO"),
      );
    }
    assert.ok(
      p.komoditas.some((k) => k.produsen === null),
      "ada komoditas yang hanya punya produksi",
    );
    assert.deepEqual(
      toProdusen(mockProdusen({ komoditasKode: "CBM-KRT" })).komoditas.map(
        (k) => k.komoditasKode,
      ),
      ["CBM-KRT"],
    );
    assert.deepEqual(
      toProdusen(mockProdusen({ komoditasKode: "TIDAK-ADA" })).komoditas,
      [],
    );
    assert.deepEqual(nilaiTakWajar(p), []);
  });

  it("logistik: jumlah transaksi dan rute konsisten, jenis ketiganya ada, filter bekerja", () => {
    const l = toLogistik(mockLogistik());
    const { masuk, keluar, pindah } = l.ringkasan;
    assert.equal(
      masuk.jumlah + keluar.jumlah + pindah.jumlah,
      l.terbaru.length,
      "kurang dari 10 transaksi: semuanya ada di terbaru",
    );
    assert.ok(masuk.jumlah > 0 && pindah.jumlah === 1 && keluar.jumlah === 1);
    assert.equal(
      l.komoditas.reduce(
        (t, k) => t + k.masuk.jumlah + k.keluar.jumlah + k.pindah.jumlah,
        0,
      ),
      l.terbaru.length,
    );
    for (const p of l.terbaru) {
      assert.equal(p.asal === null, p.jenis === "MASUK");
      assert.equal(p.tujuan === null, p.jenis === "KELUAR");
    }
    assert.equal(l.rute.jumlah, l.rute.items.length);
    assert.equal(
      l.rute.biaya,
      l.rute.items.reduce((t, r) => t + r.biaya, 0),
    );
    assert.ok(
      l.rute.items.every((r) => r.asalKode && r.tujuanKode && r.komoditasKode),
    );
    assert.ok(
      l.rute.items.every((r, i, a) => i === 0 || a[i - 1].biaya >= r.biaya),
      "biaya terbesar lebih dulu",
    );

    const cabai = toLogistik(mockLogistik({ komoditasKode: "CBM-KRT" }));
    assert.ok(cabai.terbaru.every((p) => p.komoditasKode === "CBM-KRT"));
    assert.ok(cabai.rute.items.every((r) => r.komoditasKode === "CBM-KRT"));
    const kosong = toLogistik(mockLogistik({ kabupatenKode: "9999" }));
    assert.deepEqual(
      [kosong.terbaru, kosong.komoditas, kosong.rute.items],
      [[], [], []],
    );
    assert.equal(kosong.rute.jumlah, 0);
    assert.deepEqual(nilaiTakWajar(l), []);
  });
});
