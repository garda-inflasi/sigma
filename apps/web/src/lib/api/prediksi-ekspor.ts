/**
 * Ekspor proyeksi satu komoditas: unduh CSV dari `GET /v1/prediksi/{kode}/export`
 * (seri riwayat aktual dan jalur proyeksi, dibuat server). Mode mock tidak punya berkas server.
 */

import { API_BASE, ApiError, MODE_MOCK } from "./client";

/** Nama berkas unduhan; sama dengan yang dikirim server. */
export function namaBerkasProyeksi(komoditasKode: string): string {
  return `proyeksi_${komoditasKode}.csv`;
}

/** Mengunduh proyeksi komoditas (Kota Banda Aceh, 14 hari) dan memicu simpan berkas. */
export async function unduhProyeksiCsv(komoditasKode: string): Promise<void> {
  if (MODE_MOCK)
    throw new Error("Unduhan proyeksi tersedia setelah backend aktif.");
  const res = await fetch(
    `${API_BASE}/v1/prediksi/${encodeURIComponent(komoditasKode)}/export`,
    {
      credentials: "include",
      headers: { Accept: "text/csv" },
    },
  );
  if (!res.ok) {
    let pesan = `Unduhan gagal (${res.status})`;
    try {
      const payload = (await res.json()) as { message?: string };
      pesan = payload.message ?? pesan;
    } catch {}
    throw new ApiError(res.status, pesan);
  }
  const url = URL.createObjectURL(await res.blob());
  const a = Object.assign(document.createElement("a"), {
    href: url,
    download: namaBerkasProyeksi(komoditasKode),
  });
  a.click();
  URL.revokeObjectURL(url);
}
