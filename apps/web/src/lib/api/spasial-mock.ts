/** Data mock spasial untuk mode tanpa backend (NEXT_PUBLIC_API_BASE_URL kosong). */

import { KOMODITAS } from "@/lib/data/reference";
import { mockMapMarkers } from "./mock";
import {
  statusNeraca,
  type LokasiWilayah,
  type NeracaWilayah,
  type RiwayatOptimasi,
} from "./spasial";

/** Komoditas yang punya neraca di seeder `spasial.demo`. */
const KODE_NERACA = ["BRS-MED", "CBM-KRT", "CBR-RWT", "BWM", "TLR", "DAG-AYM"];

/** Neraca mock: surplus/defisit berlawanan arah dengan deviasi harga wilayah (seperti seeder demo). */
export function mockNeraca(komoditasKode?: string): NeracaWilayah[] {
  const periode = "2026-11-01";
  const kodeDipakai = KODE_NERACA.filter(
    (k) => !komoditasKode || k === komoditasKode,
  );
  return mockMapMarkers().flatMap((m) =>
    kodeDipakai.map((kode, i) => {
      const delta = Math.round(m.delta * (1 - i * 0.12) * 10) / 10;
      const ton = Math.round(-delta * (3 + i * 2) * 10) / 10;
      return {
        id: `${kode}-${m.kode}`,
        komoditasKode: kode,
        komoditas: KOMODITAS.find((k) => k.kode === kode)?.nama ?? kode,
        kabupatenKode: m.kode,
        wilayah: m.nama,
        periode,
        surplusDefisitTon: ton,
        deltaHargaPct: delta,
        sumber: "DEMO",
        demo: true,
        status: statusNeraca(ton),
      };
    }),
  );
}

/** Lokasi mock: wilayah dengan pusat terdekat (tanpa poligon, jadi `diDalam` selalu salah). */
export function mockLokasi(lat: number, lng: number): LokasiWilayah {
  const km = (a: { lat: number; lng: number }) =>
    Math.hypot(
      (a.lat - lat) * 111,
      (a.lng - lng) * 111 * Math.cos((lat * Math.PI) / 180),
    );
  const terdekat = mockMapMarkers().reduce((a, b) => (km(b) < km(a) ? b : a));
  return {
    kode: terdekat.kode,
    nama: terdekat.nama,
    diDalam: false,
    jarakPusatKm: Math.round(km(terdekat) * 10) / 10,
  };
}

export function mockRiwayatOptimasi(): RiwayatOptimasi[] {
  return [
    {
      id: "OPT-mock-1",
      komoditasKode: "CBM-KRT",
      komoditas: "Cabai merah keriting",
      periode: "2026-11-01",
      totalVolumeTon: 96.4,
      totalBiaya: 128_500_000,
      belumTerpenuhiTon: 12.3,
      dibuatOleh: "Manajer Data",
      dibuatPada: "2026-11-03T09:15:00Z",
    },
    {
      id: "OPT-mock-2",
      komoditasKode: "BWM",
      komoditas: "Bawang merah",
      periode: "2026-11-01",
      totalVolumeTon: 54,
      totalBiaya: 61_200_000,
      belumTerpenuhiTon: 0,
      dibuatOleh: "Manajer Data",
      dibuatPada: "2026-11-02T14:40:00Z",
    },
  ];
}
