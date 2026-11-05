/** Data prediksi mode mock (tanpa backend): tabel proyeksi dan registri yang dulu dihitung di halaman. */

import { KOMODITAS } from "@/lib/data/reference";
import type { BarisModel, BarisProyeksi } from "./prediksi";
import { mockXai } from "./mock";

export function mockBarisProyeksi(): BarisProyeksi[] {
  return KOMODITAS.map((k) => {
    const x = mockXai(k.kode);
    return {
      id: k.kode,
      nama: k.nama,
      kini: x.hargaKini,
      proyeksi: Math.round(x.hargaKini * (1 + x.perubahanPct / 100)),
      pct: x.perubahanPct,
      lo: x.interval[0],
      hi: x.interval[1],
      mape: x.model.mape,
      demo: true,
    };
  });
}

export function mockBarisModel(): BarisModel[] {
  return KOMODITAS.flatMap((k, i) => [
    {
      id: `${k.kode}-231`,
      komoditas: k.nama,
      versi: "v2.3.1",
      mape: +(k.volatilitas * 180 + 1.5).toFixed(1),
      drift: +((i % 5) * 0.03).toFixed(2),
      status: "PRODUKSI" as const,
      demo: true,
    },
    {
      id: `${k.kode}-240`,
      komoditas: k.nama,
      versi: "v2.4.0-rc1",
      mape: +((k.volatilitas * 180 + 1.5) * 0.93).toFixed(1),
      drift: 0,
      status: "KANDIDAT" as const,
      demo: true,
    },
  ]);
}
