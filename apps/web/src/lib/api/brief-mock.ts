/** Detail brief mode mock (tanpa backend): isi sederhana dari ringkasan brief contoh. */

import type { PolicyBriefDetail } from "./brief";
import { mockBriefs } from "./mock";

export function mockBriefDetail(id: string): PolicyBriefDetail {
  const brief = mockBriefs().find((b) => b.id === id) ?? mockBriefs()[0];
  return {
    ...brief,
    sumberNarasi: "DEMO",
    labelDraf: "Data demo — bukan brief resmi",
    bagian: [{ kode: "ringkasan", judul: "Ringkasan", butir: [brief.judul] }],
    sitasi: [],
  };
}
