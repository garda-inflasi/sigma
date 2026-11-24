/**
 * Olahan murni forecast hasil training untuk tampilan: seri grafik P10/P50/P90, titik horizon, ringkasan per
 * wilayah (kesegaran harga terakhir), dan baris tabel proyeksi lama. Tidak ada angka yang dikarang: titik tanpa nilai
 * dibiarkan kosong.
 */
import type { BarisProyeksi } from "@/lib/api/prediksi";
import type { ForecastDetailMl, ForecastMl, TitikKurvaMl } from "@/lib/api/ml";
import { namaKomoditasMl } from "./label";

/** Horizon wajib kontrak (hari). */
export const HORIZON_ML = [7, 14, 30] as const;

export const titikHorizon = (kurva: readonly TitikKurvaMl[], horizon: number) =>
  kurva.find((t) => t.horizon === horizon);

/** "Rp/kg" → "/kg"; satuan lain ditulis utuh di belakang angka. */
export function akhiranSatuan(satuan: string | null | undefined): string {
  if (!satuan) return "";
  return satuan.startsWith("Rp/") ? satuan.slice(2) : ` ${satuan}`;
}

const ts = (tanggal: string) => Date.parse(`${tanggal.slice(0, 10)}T00:00:00Z`);

export interface TitikGrafikMl {
  ts: number;
  tanggal: string;
  aktual?: number;
  p50?: number;
  rentang?: [number, number];
}

/**
 * Seri grafik: riwayat harga asli, lalu titik jangkar harga terakhir (pita mulai dari sana), lalu P50 dan pita
 * P10–P90 tiap horizon. Titik kurva tanpa P50 tidak digambar.
 */
export function seriGrafikForecast(
  item: Pick<
    ForecastDetailMl,
    "riwayat" | "kurva" | "hargaTerakhir" | "tanggalTerakhir"
  >,
): TitikGrafikMl[] {
  const seri: TitikGrafikMl[] = item.riwayat
    .filter((r) => r.nilai !== null)
    .map((r) => ({
      ts: ts(r.tanggal),
      tanggal: r.tanggal,
      aktual: r.nilai as number,
    }));
  if (item.hargaTerakhir !== null && item.tanggalTerakhir) {
    const jangkar = ts(item.tanggalTerakhir);
    const ada = seri.find((s) => s.ts === jangkar);
    const h = item.hargaTerakhir;
    if (ada)
      Object.assign(ada, { p50: h, rentang: [h, h] as [number, number] });
    else
      seri.push({
        ts: jangkar,
        tanggal: item.tanggalTerakhir,
        aktual: h,
        p50: h,
        rentang: [h, h],
      });
  }
  for (const t of item.kurva) {
    if (t.p50 === null) continue;
    seri.push({
      ts: ts(t.tanggal),
      tanggal: t.tanggal,
      p50: t.p50,
      rentang: t.p10 !== null && t.p90 !== null ? [t.p10, t.p90] : undefined,
    });
  }
  return seri.sort((a, b) => a.ts - b.ts);
}

/** Baris tabel proyeksi lama (`/prediksi` tab Komoditas) dari forecast siklus pilihan, horizon 14 hari. */
export function barisDariForecast(f: ForecastMl): BarisProyeksi {
  const t = titikHorizon(f.kurva, 14) ?? f.kurva[0];
  const kini = f.hargaTerakhir ?? 0;
  const p50 = t?.p50 ?? kini;
  const pct = (v: number | null | undefined) =>
    kini > 0 && v !== null && v !== undefined
      ? Math.round(((v - kini) / kini) * 1000) / 10
      : 0;
  return {
    id: f.komoditasKode,
    nama: namaKomoditasMl(f.komoditasKode),
    kini: Math.round(kini),
    proyeksi: Math.round(p50),
    pct: t?.perubahanP50Pct ?? pct(p50),
    lo: pct(t?.p10),
    hi: pct(t?.p90),
    mape: f.metrik.mape ?? 0,
    demo: false,
  };
}

export interface RingkasanWilayahForecast {
  kabupatenKode: string;
  jumlahKomoditas: number;
  /** Tanggal harga asli terakhir paling baru di wilayah itu (YYYY-MM-DD). */
  tanggalTerakhir: string | null;
  /** Umur hari harga terakhir terhadap `acuan` (null bila tidak diketahui). */
  umurHari: number | null;
  /** Komoditas dengan perubahan P50 H+14 terbesar (naik). */
  naikTerbesar: { komoditasKode: string; pct: number } | null;
  /** Median perubahan P50 H+14 seluruh komoditas wilayah. */
  medianPct: number | null;
  jumlahPersistensi: number;
}

function median(v: number[]): number | null {
  if (v.length === 0) return null;
  const s = [...v].sort((a, b) => a - b);
  const m = Math.floor(s.length / 2);
  return s.length % 2 ? s[m] : Math.round(((s[m - 1] + s[m]) / 2) * 10) / 10;
}

/**
 * Ringkasan forecast per kabupaten: jumlah komoditas, kesegaran harga terakhir, kenaikan P50 H+14 terbesar dan
 * mediannya, serta berapa yang dipublikasikan sebagai persistensi. `acuan` = tanggal hari ini (YYYY-MM-DD, WIB).
 */
export function ringkasForecastPerWilayah(
  items: readonly ForecastMl[],
  acuan: string,
): RingkasanWilayahForecast[] {
  const per = new Map<string, ForecastMl[]>();
  for (const f of items)
    per.set(f.kabupatenKode, [...(per.get(f.kabupatenKode) ?? []), f]);
  return [...per.entries()]
    .map(([kabupatenKode, daftar]) => {
      const tanggal =
        daftar
          .map((f) => f.tanggalTerakhir)
          .filter((t): t is string => !!t)
          .sort()
          .at(-1) ?? null;
      const perubahan = daftar
        .map((f) => ({
          komoditasKode: f.komoditasKode,
          pct: titikHorizon(f.kurva, 14)?.perubahanP50Pct ?? null,
        }))
        .filter(
          (p): p is { komoditasKode: string; pct: number } => p.pct !== null,
        );
      const naik = [...perubahan].sort((a, b) => b.pct - a.pct)[0];
      return {
        kabupatenKode,
        jumlahKomoditas: daftar.length,
        tanggalTerakhir: tanggal,
        umurHari: tanggal
          ? Math.max(0, Math.round((ts(acuan) - ts(tanggal)) / 86_400_000))
          : null,
        naikTerbesar: naik && naik.pct > 0 ? naik : null,
        medianPct: median(perubahan.map((p) => p.pct)),
        jumlahPersistensi: daftar.filter((f) => f.model.adalahBaseline).length,
      };
    })
    .sort((a, b) => a.kabupatenKode.localeCompare(b.kabupatenKode));
}

/** Tanggal hari ini menurut WIB (YYYY-MM-DD). */
export function hariIniWib(sekarang = new Date()): string {
  return new Date(sekarang.getTime() + 7 * 3_600_000)
    .toISOString()
    .slice(0, 10);
}
