/** Data mock EWS untuk detail dan riwayat alert (mode tanpa backend). */

import type { EwsAlert, StatusEws } from "@/lib/data/types";
import type { EwsDetail, EwsRiwayat } from "./ews";

export function mockEwsDetail(a: EwsAlert): EwsDetail {
  return { ...a };
}

/** Riwayat tiruan: dibuat sistem, lalu satu langkah per status yang telah dilewati. */
export function mockEwsRiwayat(a: EwsAlert): EwsRiwayat[] {
  const urutan: StatusEws[] = ["BARU", "DIAKUI", "DIESKALASI", "SELESAI"];
  const sampai = urutan.indexOf(a.status);
  const awal = new Date(a.waktu).getTime();
  return urutan.slice(0, sampai + 1).map((ke, i) => ({
    id: `${a.id}-${i}`,
    dari: i === 0 ? undefined : urutan[i - 1],
    ke,
    olehNama: i === 0 ? undefined : "Petugas contoh",
    waktu: new Date(awal + i * 45 * 60_000).toISOString(),
    catatan: i === 0 ? "Dibuat oleh rule engine" : "",
  }));
}
