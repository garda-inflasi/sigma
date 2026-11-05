/**
 * Laporan KPI periodik: unduh CSV dari `GET /v1/kpi/export` (dibuat server, tercatat
 * di audit trail). Mode mock tidak punya laporan server.
 */

import { API_BASE, ApiError, MODE_MOCK } from "./client";

/** Nama berkas unduhan: `laporan-kpi-YYYY-MM.csv` untuk bulan berjalan. */
export function namaBerkasLaporan(sekarang: Date = new Date()): string {
  return `laporan-kpi-${sekarang.toISOString().slice(0, 7)}.csv`;
}

/** Mengunduh laporan KPI bulan berjalan (seluruh provinsi) dan memicu simpan berkas. */
export async function unduhLaporanKpi(): Promise<void> {
  if (MODE_MOCK) throw new Error("Laporan KPI tersedia setelah backend aktif.");
  const res = await fetch(`${API_BASE}/v1/kpi/export`, {
    credentials: "include",
    headers: { Accept: "text/csv" },
  });
  if (!res.ok) {
    let pesan = `Ekspor gagal (${res.status})`;
    try {
      const payload = (await res.json()) as { message?: string };
      pesan = payload.message ?? pesan;
    } catch {}
    throw new ApiError(res.status, pesan);
  }
  const url = URL.createObjectURL(await res.blob());
  const a = Object.assign(document.createElement("a"), {
    href: url,
    download: namaBerkasLaporan(),
  });
  a.click();
  URL.revokeObjectURL(url);
}
