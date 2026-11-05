/** Data mock detail pengguna (mode tanpa backend). */

import type { PenggunaRow } from "@/lib/data/types";
import type { PenggunaDetail } from "./pengguna";

export function mockPenggunaDetail(u: PenggunaRow): PenggunaDetail {
  return {
    ...u,
    kabupatenKode: undefined,
    dibuatPada: "2026-01-05T08:00:00.000Z",
  };
}
