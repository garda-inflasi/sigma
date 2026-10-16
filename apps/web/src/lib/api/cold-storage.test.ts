import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { ColdStorageView, FasilitasColdStorage, StokColdStorage } from "@/lib/data/types";
import {
  AMBANG_DEVIASI_SUHU_C,
  deviasiSuhu,
  formatWilayah,
  ikonKomoditas,
  labelUtilisasi,
  nadaRisiko,
  nadaUtilisasi,
  queryMockColdStorage,
  sentenceCase,
  toColdStorageView,
  toFasilitas,
  toStok,
  urgensiLot,
  utilisasiPersen,
  type ColdStorageWire,
} from "./cold-storage";
import { mockColdStorage } from "./mock";

// Respons backend Go yang disederhanakan (bentuk sama dengan hasil uji integrasi).
const wire: ColdStorageWire = {
  fasilitas: [
    { id: "CS-01", nama: "Cold Storage Lambaro", kabupaten_kode: "1106", kabupaten_nama: "KABUPATEN ACEH BESAR", lat: 5.505, lng: 95.37, kapasitas_ton: 400, terisi_ton: 347, utilisasi_persen: 86.8, suhu_target_c: 4, suhu_aktual_c: 3.5, status: "OPERASIONAL" },
    { id: "CS-05", nama: "Gudang Dingin Langsa", kabupaten_kode: "1174", kabupaten_nama: "KOTA LANGSA", lat: 4.48, lng: 97.96, kapasitas_ton: 120, terisi_ton: 0, utilisasi_persen: 0, suhu_target_c: 4, suhu_aktual_c: null, status: "RENCANA" },
  ],
  stok: [
    { id: "STK-CS01-01", cold_storage_id: "CS-01", cold_storage_nama: "Cold Storage Lambaro", komoditas_kode: "cabai_merah_keriting", komoditas_nama: "Cabai Merah Keriting", volume_ton: 34.9, masuk_pada: "2026-09-30T09:00:00+07:00", kedaluwarsa_pada: "2026-10-21T09:00:00+07:00", sisa_masa_simpan_hari: 5, risiko_food_loss: 0.68, pemilik: "KUD Pidie Jaya" },
  ],
  meta: { page: 1, limit: 10, total: 23, total_pages: 3, volume_hasil_ton: 813 },
  ringkasan: { total_lot: 23, total_volume_ton: 813, risiko_tinggi_lot: 4, risiko_tinggi_ton: 172.7, ambang_risiko_tinggi: 0.6 },
};

describe("formatWilayah", () => {
  it("menyingkat kabupaten dan kota", () => {
    assert.equal(formatWilayah("KABUPATEN ACEH BESAR"), "Kab. Aceh Besar");
    assert.equal(formatWilayah("KOTA LHOKSEUMAWE"), "Kota Lhokseumawe");
    assert.equal(formatWilayah("KABUPATEN ACEH BARAT DAYA"), "Kab. Aceh Barat Daya");
  });

  it("tidak peka huruf besar/kecil dan spasi tepi", () => {
    assert.equal(formatWilayah("  kota langsa "), "Kota Langsa");
  });

  it("nama tanpa awalan hanya dirapikan", () => {
    assert.equal(formatWilayah("ACEH"), "Aceh");
  });
});

describe("sentenceCase", () => {
  it("huruf besar hanya di awal", () => {
    assert.equal(sentenceCase("Cabai Merah Keriting"), "Cabai merah keriting");
    assert.equal(sentenceCase("TOMAT"), "Tomat");
    assert.equal(sentenceCase("  telur ayam "), "Telur ayam");
    assert.equal(sentenceCase(""), "");
  });
});

describe("ikonKomoditas", () => {
  const kasus: [string, string][] = [
    ["cabai_merah_keriting", "cabai"],
    ["cabai_rawit_merah", "cabai"],
    ["bawang_merah", "bawang"],
    ["bawang_putih", "bawang"],
    ["beras_medium", "beras"],
    ["telur_ayam", "telur"],
    ["daging_ayam", "ayam"],
    ["daging_sapi", "sapi"],
    ["ikan_tongkol", "ikan"],
    ["minyak_goreng", "minyak"],
    ["gula_pasir", "gula"],
    ["tomat", "sayur"],
    ["kentang", "sayur"],
    ["komoditas_baru_tak_dikenal", "sayur"],
  ];
  for (const [kode, ikon] of kasus) {
    it(`${kode} → ${ikon}`, () => assert.equal(ikonKomoditas(kode), ikon));
  }
});

describe("pemetaan respons backend", () => {
  const view: ColdStorageView = toColdStorageView(wire);

  it("fasilitas: wilayah diformat dan suhu kosong dipertahankan", () => {
    assert.equal(view.fasilitas[0].wilayah, "Kab. Aceh Besar");
    assert.equal(view.fasilitas[0].terisiTon, 347);
    assert.equal(view.fasilitas[0].suhuAktualC, 3.5);
    assert.equal(view.fasilitas[1].wilayah, "Kota Langsa");
    assert.equal(view.fasilitas[1].suhuAktualC, null);
    assert.equal(view.fasilitas[1].status, "RENCANA");
  });

  it("stok: nama komoditas sentence case, ikon dari kode, tanggal apa adanya", () => {
    const s = view.stok[0];
    assert.equal(s.komoditasNama, "Cabai merah keriting");
    assert.equal(s.ikon, "cabai");
    assert.equal(s.fasilitasId, "CS-01");
    assert.equal(s.fasilitasNama, "Cold Storage Lambaro");
    assert.equal(s.sisaShelfLifeHari, 5);
    assert.equal(s.risikoFoodLoss, 0.68);
    assert.equal(s.masuk, "2026-09-30T09:00:00+07:00");
  });

  it("halaman dan ringkasan", () => {
    assert.deepEqual(view.halaman, { page: 1, limit: 10, total: 23, totalPages: 3, volumeHasilTon: 813 });
    assert.deepEqual(view.ringkasan, { totalLot: 23, totalVolumeTon: 813, risikoTinggiLot: 4, risikoTinggiTon: 172.7, ambangRisikoTinggi: 0.6 });
  });

  it("status tak dikenal diperlakukan sebagai rencana", () => {
    assert.equal(toFasilitas({ ...wire.fasilitas[0], status: "ENTAH" }).status, "RENCANA");
  });

  it("pemetaan tunggal konsisten dengan pemetaan layar", () => {
    assert.deepEqual(toStok(wire.stok[0]), view.stok[0]);
  });
});

describe("urgensiLot", () => {
  it("batas merah dan kuning", () => {
    assert.equal(urgensiLot(0), "kritis");
    assert.equal(urgensiLot(5), "kritis");
    assert.equal(urgensiLot(6), "waspada");
    assert.equal(urgensiLot(14), "waspada");
    assert.equal(urgensiLot(15), "aman");
    assert.equal(urgensiLot(120), "aman");
  });
});

describe("utilisasi", () => {
  it("persentase dan kapasitas nol", () => {
    assert.equal(utilisasiPersen({ terisiTon: 240, kapasitasTon: 300 }), 80);
    assert.equal(utilisasiPersen({ terisiTon: 0, kapasitasTon: 0 }), 0);
  });

  it("nada dan label per ambang", () => {
    assert.equal(nadaUtilisasi(87), "defisit");
    assert.equal(nadaUtilisasi(85), "data");
    assert.equal(nadaUtilisasi(77), "data");
    assert.equal(nadaUtilisasi(65), "surplus");
    assert.equal(nadaUtilisasi(22), "surplus");
    assert.equal(labelUtilisasi(87), "Hampir penuh");
    assert.equal(labelUtilisasi(80), "Terpakai optimal");
    assert.equal(labelUtilisasi(22), "Kapasitas longgar");
  });
});

describe("deviasiSuhu & nadaRisiko", () => {
  it("selisih mutlak, atau null tanpa telemetri", () => {
    assert.equal(deviasiSuhu({ suhuAktualC: -20.1, suhuTargetC: -20 })?.toFixed(1), "0.1");
    assert.equal(deviasiSuhu({ suhuAktualC: 4.8, suhuTargetC: 4 })?.toFixed(1), "0.8");
    assert.equal(deviasiSuhu({ suhuAktualC: null, suhuTargetC: 4 }), null);
    assert.ok(AMBANG_DEVIASI_SUHU_C > 0.8, "4,8 °C terhadap target 4 °C belum dianggap menyimpang");
  });

  it("risiko di atas ambang merah, menengah kuning, rendah hijau", () => {
    assert.equal(nadaRisiko(0.68, 0.6), "defisit");
    assert.equal(nadaRisiko(0.6, 0.6), "waspada", "tepat di ambang belum berisiko tinggi (sama dengan agregat server: > ambang)");
    assert.equal(nadaRisiko(0.5, 0.6), "waspada");
    assert.equal(nadaRisiko(0.35, 0.6), "normal");
    assert.equal(nadaRisiko(0.09, 0.6), "normal");
  });
});

describe("queryMockColdStorage", () => {
  const all = mockColdStorage();
  const semuaId = all.stok.map((s) => s.id);

  it("bawaan: 10 baris, sisa masa simpan terpendek lebih dulu", () => {
    const v = queryMockColdStorage(all);
    assert.equal(v.stok.length, Math.min(10, all.stok.length));
    const sisa = v.stok.map((s) => s.sisaShelfLifeHari);
    assert.deepEqual(sisa, [...sisa].sort((a, b) => a - b));
    assert.equal(v.halaman.total, all.stok.length);
    assert.equal(v.halaman.totalPages, Math.ceil(all.stok.length / 10));
    assert.equal(v.fasilitas.length, all.fasilitas.length, "kartu fasilitas selalu lengkap");
  });

  it("paginasi mencakup setiap lot tepat satu kali", () => {
    const limit = 4;
    const first = queryMockColdStorage(all, { limit });
    const ids: string[] = [];
    for (let page = 1; page <= first.halaman.totalPages; page++) ids.push(...queryMockColdStorage(all, { limit, page }).stok.map((s) => s.id));
    assert.deepEqual([...ids].sort(), [...semuaId].sort());
    assert.equal(new Set(ids).size, ids.length);
  });

  it("halaman di luar jangkauan kosong dengan total tetap", () => {
    const v = queryMockColdStorage(all, { page: 99 });
    assert.deepEqual(v.stok, []);
    assert.equal(v.halaman.total, all.stok.length);
  });

  it("limit dijepit ke 1..100", () => {
    assert.equal(queryMockColdStorage(all, { limit: 0 }).halaman.limit, 1);
    assert.equal(queryMockColdStorage(all, { limit: 5000 }).halaman.limit, 100);
  });

  it("filter fasilitas: stok, total volume mengikuti filter, ringkasan tidak", () => {
    const id = all.stok[0].fasilitasId;
    const milik = all.stok.filter((s) => s.fasilitasId === id);
    const v = queryMockColdStorage(all, { fasilitasId: id, limit: 100 });
    assert.equal(v.stok.length, milik.length);
    assert.ok(v.stok.every((s) => s.fasilitasId === id));
    assert.equal(v.halaman.volumeHasilTon, Math.round(milik.reduce((a, s) => a + s.volumeTon, 0) * 10) / 10);
    assert.equal(v.ringkasan.totalLot, all.stok.length);
  });

  it("cari pada komoditas atau pemilik tanpa membedakan huruf", () => {
    const pemilik = all.stok[0].pemilik;
    const v = queryMockColdStorage(all, { q: pemilik.toUpperCase(), limit: 100 });
    assert.ok(v.stok.length > 0);
    assert.ok(v.stok.every((s) => s.pemilik === pemilik || s.komoditasNama.toLowerCase().includes(pemilik.toLowerCase())));

    const kom = all.stok[0].komoditasNama;
    const k = queryMockColdStorage(all, { q: kom.slice(0, 4), limit: 100 });
    assert.ok(k.stok.some((s) => s.komoditasNama === kom));

    assert.equal(queryMockColdStorage(all, { q: "tidak-ada-yang-begini" }).stok.length, 0);
  });

  it("urut menurun dan pemutus seri berdasarkan id", () => {
    const v = queryMockColdStorage(all, { sort: "volume", desc: true, limit: 100 });
    const vol = v.stok.map((s) => s.volumeTon);
    assert.deepEqual(vol, [...vol].sort((a, b) => b - a));

    // Kunci urut sama (mis. pemilik) → id naik.
    const p = queryMockColdStorage(all, { sort: "pemilik", limit: 100 }).stok;
    for (let i = 1; i < p.length; i++) {
      if (p[i - 1].pemilik.toLowerCase() === p[i].pemilik.toLowerCase()) assert.ok(p[i - 1].id < p[i].id);
    }
  });

  it("setiap kolom urut menghasilkan urutan monoton", () => {
    const kunci: Record<string, (s: StokColdStorage) => string | number> = {
      komoditas: (s) => s.komoditasNama.toLowerCase(),
      fasilitas: (s) => s.fasilitasNama.toLowerCase(),
      volume: (s) => s.volumeTon,
      masuk: (s) => Date.parse(s.masuk),
      risiko: (s) => s.risikoFoodLoss,
      pemilik: (s) => s.pemilik.toLowerCase(),
      sisa_masa_simpan: (s) => s.sisaShelfLifeHari,
    };
    for (const [sort, key] of Object.entries(kunci)) {
      for (const desc of [false, true]) {
        const rows = queryMockColdStorage(all, { sort: sort as never, desc, limit: 100 }).stok;
        for (let i = 1; i < rows.length; i++) {
          const [a, b] = [key(rows[i - 1]), key(rows[i])];
          assert.ok(desc ? a >= b : a <= b, `${sort} ${desc ? "turun" : "naik"} baris ${i}`);
        }
      }
    }
  });

  it("ringkasan risiko tinggi memakai ambang > 0,6 atas seluruh stok", () => {
    const tinggi = all.stok.filter((s) => s.risikoFoodLoss > 0.6);
    const v = queryMockColdStorage(all, { q: "tidak-ada" });
    assert.equal(v.ringkasan.risikoTinggiLot, tinggi.length);
    assert.equal(v.ringkasan.risikoTinggiTon, Math.round(tinggi.reduce((a, s) => a + s.volumeTon, 0) * 10) / 10);
    assert.equal(v.ringkasan.ambangRisikoTinggi, 0.6);
  });

  it("fasilitas rencana tidak punya stok", () => {
    const rencana = all.fasilitas.filter((f: FasilitasColdStorage) => f.status === "RENCANA");
    assert.ok(rencana.length > 0);
    for (const f of rencana) assert.equal(all.stok.filter((s) => s.fasilitasId === f.id).length, 0);
  });
});
