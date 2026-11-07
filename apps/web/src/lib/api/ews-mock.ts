/** Data mock EWS untuk detail dan riwayat alert (mode tanpa backend). */

import type { EwsAlert, StatusEws } from "@/lib/data/types";
import type { EwsDetail, EwsRiwayat } from "./ews";
import { mockEwsAturan } from "./mock";

/** Detail tiruan; alert yang jenis dan komoditasnya punya aturan ambang membawa rujukan aturannya (id dan kode). */
export function mockEwsDetail(a: EwsAlert): EwsDetail {
  const aturan = mockEwsAturan().find((r) => r.jenis === a.jenis && r.komoditasKode === a.komoditasKode);
  return { ...a, ...(aturan ? { aturanId: aturan.id, aturanKode: aturan.kode } : {}) };
}

/** Riwayat tiruan: dibuat sistem, lalu satu langkah per status yang telah dilewati. */
export function mockEwsRiwayat(a: EwsAlert): EwsRiwayat[] {
  const urutan: StatusEws[] = ["BARU", "DIAKUI", "DIESKALASI", "SELESAI"];
  const sampai = urutan.indexOf(a.status);
  const awal = new Date(a.waktu).getTime();
  return urutan.slice(0, sampai + 1).map((ke, i) => ({
    id: String(i + 1),
    dari: i === 0 ? undefined : urutan[i - 1],
    ke,
    olehNama: i === 0 ? undefined : "Petugas contoh",
    waktu: new Date(awal + i * 45 * 60_000).toISOString(),
    catatan: i === 0 ? "Dibuat oleh rule engine" : "",
  }));
}
