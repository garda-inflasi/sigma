/**
 * Data acuan validasi di perangkat: median 7 hari & rentang wajar per pasar.
 * Produksi: diunduh dari API saat online lalu di-cache (tugas "Unduh data acuan").
 */
import { seriHarga } from "@/lib/data/generators";
import { komoditasByKode, pasarById } from "@/lib/data/reference";
import type { TipeHarga } from "@/lib/data/types";

export const TOLERANSI = 0.25; // ±25% = rentang wajar
export const BATAS_EKSTREM = 0.6; // >60% = kemungkinan salah satuan

export function hargaAcuan(kode: string, pasarId: string, tipe: TipeHarga) {
  const kom = komoditasByKode[kode];
  const pasar = pasarById[pasarId];
  const tujuh = seriHarga(kom, pasar.kabupatenKode).slice(-7).map((p) => (tipe === "KONSUMEN" ? p.konsumen : p.produsen)).sort((a, b) => a - b);
  const median = tujuh[3];
  const r = (n: number) => Math.round(n / 100) * 100;
  return { median, bawah: r(median * (1 - TOLERANSI)), atas: r(median * (1 + TOLERANSI)) };
}

export function jarakMeter(a: { lat: number; lng: number }, b: { lat: number; lng: number }) {
  const R = 6371e3;
  const rad = (d: number) => (d * Math.PI) / 180;
  const dLat = rad(b.lat - a.lat);
  const dLng = rad(b.lng - a.lng);
  const h = Math.sin(dLat / 2) ** 2 + Math.cos(rad(a.lat)) * Math.cos(rad(b.lat)) * Math.sin(dLng / 2) ** 2;
  return 2 * R * Math.asin(Math.sqrt(h));
}

/** Kompres foto bukti ke JPEG ≤1280px agar hemat kuota & penyimpanan offline. */
export async function kompresFoto(file: File, max = 1280, q = 0.72): Promise<string> {
  const bmp = await createImageBitmap(file);
  const s = Math.min(1, max / Math.max(bmp.width, bmp.height));
  const c = document.createElement("canvas");
  c.width = Math.round(bmp.width * s);
  c.height = Math.round(bmp.height * s);
  c.getContext("2d")!.drawImage(bmp, 0, 0, c.width, c.height);
  return c.toDataURL("image/jpeg", q);
}
