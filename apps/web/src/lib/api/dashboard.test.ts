import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { EntriHarga } from "@/lib/data/types";
import {
  aktivitasMock,
  capaianPct,
  toAktivitasLapangan,
  toHeatmap,
  toKpiOperasional,
} from "./dashboard";

describe("toKpiOperasional", () => {
  it("memetakan keterangan_delta ke keteranganDelta", () => {
    const kpi = toKpiOperasional({
      id: "validasi",
      label: "Tingkat validasi",
      nilai: 87.4,
      format: "persen",
      delta: 2.1,
      keterangan_delta: "poin dari pekan lalu",
      tren: [80, 87.4],
      tone: "waspada",
      href: "/data/harga?status=DIAJUKAN",
    });
    assert.deepEqual(kpi, {
      id: "validasi",
      label: "Tingkat validasi",
      nilai: 87.4,
      format: "persen",
      delta: 2.1,
      keteranganDelta: "poin dari pekan lalu",
      tren: [80, 87.4],
      tone: "waspada",
      href: "/data/harga?status=DIAJUKAN",
    });
  });
});

describe("toHeatmap", () => {
  it("memetakan sel ke camelCase dan memakai nama ringkas kabupaten", () => {
    const h = toHeatmap({
      kabupaten: [
        { kode: "1108", nama: "KABUPATEN ACEH BESAR" },
        { kode: "9999", nama: "KOTA BARU" },
      ],
      komoditas: [
        { kode: "CBM-KRT", nama: "Cabai merah keriting", pilot: true },
      ],
      sel: [
        {
          komoditas_kode: "CBM-KRT",
          kabupaten_kode: "1108",
          delta: 12.3,
          harga: 61500,
        },
      ],
    });
    assert.deepEqual(h.kabupaten, [
      { kode: "1108", nama: "Aceh Besar" },
      { kode: "9999", nama: "Kota Baru" },
    ]);
    assert.deepEqual(h.sel, [
      {
        komoditasKode: "CBM-KRT",
        kabupatenKode: "1108",
        delta: 12.3,
        harga: 61500,
      },
    ]);
    assert.equal(h.komoditas[0].pilot, true);
  });

  it("heatmap kosong tetap valid", () => {
    assert.deepEqual(toHeatmap({ kabupaten: [], komoditas: [], sel: [] }), {
      kabupaten: [],
      komoditas: [],
      sel: [],
    });
  });
});

describe("aktivitas lapangan", () => {
  it("toAktivitasLapangan memetakan kolom API", () => {
    assert.deepEqual(
      toAktivitasLapangan({
        kabupaten_kode: "1171",
        kabupaten_nama: "KOTA BANDA ACEH",
        jumlah_petugas: 2,
        entri: 20,
        target: 28,
      }),
      {
        kode: "1171",
        nama: "Banda Aceh",
        jumlahPetugas: 2,
        entri: 20,
        target: 28,
      },
    );
  });

  it("capaianPct dibatasi 100 dan aman untuk target nol", () => {
    assert.equal(capaianPct({ entri: 7, target: 14 }), 50);
    assert.equal(capaianPct({ entri: 30, target: 14 }), 100);
    assert.equal(capaianPct({ entri: 3, target: 0 }), 0);
  });

  it("aktivitasMock menghitung entri konsumen per kabupaten terhadap target", () => {
    const e = (kab: string, tipe: EntriHarga["tipe"]) =>
      ({ kabupatenKode: kab, tipe }) as EntriHarga;
    const hasil = aktivitasMock(
      [
        e("1171", "KONSUMEN"),
        e("1171", "KONSUMEN"),
        e("1171", "PRODUSEN"),
        e("1108", "KONSUMEN"),
      ],
      new Map([
        ["1171", 2],
        ["1108", 1],
      ]),
    );
    assert.deepEqual(
      hasil.map((a) => [a.kode, a.entri, a.target]),
      [
        ["1171", 2, 28],
        ["1108", 1, 14],
      ],
    );
  });
});
