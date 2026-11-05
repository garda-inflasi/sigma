/** Data mock detail tugas lapangan (mode tanpa backend). */

import { komoditasByKode } from "@/lib/data/reference";
import type { TugasLapangan } from "@/lib/data/types";
import type { TugasDetail } from "./lapangan";

/** Komoditas terisi dianggap berurutan dari awal sebanyak `terisi`. */
export function mockTugasDetail(t: TugasLapangan): TugasDetail {
  return {
    ...t,
    rincian: t.komoditas.map((kode, i) => ({
      kode,
      nama: komoditasByKode[kode]?.nama ?? kode,
      terisi: i < t.terisi,
    })),
  };
}
