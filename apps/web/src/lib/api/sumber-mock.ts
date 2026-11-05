/** Data mock detail sumber data (mode tanpa backend). */

import type { SumberData } from "@/lib/data/types";
import type { SumberDetailData } from "./sumber";

export function mockSumberDetail(s: SumberData): SumberDetailData {
  return {
    ...s,
    jadwalCron: "0 6 * * *",
    robotsTxtPatuh: true,
    atribusi: `Sumber: ${s.penyedia}`,
    urlDasar: null,
  };
}
