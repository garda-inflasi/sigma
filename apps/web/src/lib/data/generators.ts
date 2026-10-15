/**
 * Generator data sintetis deterministik (PRNG ber-seed) — dipakai seeder & mock.
 * Skenario per 3 Oktober 2026: cabai sedang dalam tekanan (curah hujan tinggi
 * di sentra Aceh Tengah/Bener Meriah), bawang merah naik moderat, kentang & tomat
 * surplus di dataran tinggi Gayo.
 */
import { KABUPATEN, KOMODITAS, PASAR, type Komoditas, type Pasar, kabupatenByKode } from "./reference";

export const ANCHOR_DATE = new Date("2026-10-03T08:00:00+07:00");
export const HARI_SERI = 90;

export function hashSeed(str: string) {
  let h = 2166136261;
  for (let i = 0; i < str.length; i++) {
    h ^= str.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return h >>> 0;
}

export function mulberry32(seed: number) {
  let a = seed;
  return () => {
    a |= 0;
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

export function gauss(rng: () => number) {
  const u = 1 - rng();
  const v = rng();
  return Math.sqrt(-2 * Math.log(u)) * Math.cos(2 * Math.PI * v);
}

export const pick = <T,>(rng: () => number, arr: readonly T[]) => arr[Math.floor(rng() * arr.length)];

/** Guncangan per komoditas: puncak (hari ke-), lebar, amplitudo (+ naik / − turun). */
const SHOCK: Record<string, { pusat: number; lebar: number; amp: number }[]> = {
  "CBM-KRT": [{ pusat: 93, lebar: 6.5, amp: 0.38 }],
  "CBR-RWT": [{ pusat: 92, lebar: 6, amp: 0.3 }],
  BWM: [{ pusat: 88, lebar: 14, amp: 0.12 }],
  "IKN-TGK": [{ pusat: 40, lebar: 6, amp: 0.15 }],
  KTG: [{ pusat: 86, lebar: 12, amp: -0.18 }],
  TMT: [{ pusat: 91, lebar: 7, amp: -0.26 }],
};

/** Premi harga wilayah (wilayah kepulauan/terpencil lebih mahal). */
const PREMI_WILAYAH: Record<string, number> = { "1101": 0.11, "1104": 0.06, "1103": 0.04, "1171": 0.03, "1106": -0.04, "1117": -0.03 };

/** Sentra produksi: guncangan teredam/terbalik untuk komoditas sentra. */
const SENTRA: Record<string, string[]> = { "1106": ["KTG", "TMT", "CBM-KRT"], "1117": ["KTG", "CBM-KRT"], "1109": ["BWM"] };

export function tanggalMundur(hariKe: number, total = HARI_SERI) {
  const d = new Date(ANCHOR_DATE);
  d.setDate(d.getDate() - (total - 1 - hariKe));
  d.setHours(8, 0, 0, 0);
  return d;
}

export interface TitikHarga {
  tanggal: Date;
  produsen: number;
  konsumen: number;
}

const bulat = (n: number, ke = 100) => Math.round(n / ke) * ke;

export function seriHarga(kom: Komoditas, kabupatenKode: string, salt = "", hari = HARI_SERI): TitikHarga[] {
  const rng = mulberry32(hashSeed(`${kom.kode}:${kabupatenKode}:${salt}`));
  const premi = PREMI_WILAYAH[kabupatenKode] ?? 0;
  const diSentra = SENTRA[kabupatenKode]?.includes(kom.kode);
  const shocks = SHOCK[kom.kode] ?? [];
  const out: TitikHarga[] = [];
  let walk = 0;
  for (let t = 0; t < hari; t++) {
    walk = 0.88 * walk + gauss(rng) * kom.volatilitas;
    const musiman = Math.sin((t / hari) * Math.PI * 2 + hashSeed(kom.kode) % 7) * kom.volatilitas * 2.2;
    let guncang = 0;
    for (const s of shocks) {
      const amp = diSentra && s.amp > 0 ? s.amp * 0.55 : s.amp;
      guncang += amp * Math.exp(-((t - s.pusat) ** 2) / (2 * s.lebar ** 2));
    }
    const faktor = 1 + premi + walk + musiman + guncang;
    const konsumen = kom.hargaAcuan * faktor;
    const margin = kom.margin * (1 + gauss(rng) * 0.015) * (diSentra ? 0.95 : 1);
    out.push({ tanggal: tanggalMundur(t, hari), konsumen: bulat(konsumen), produsen: bulat(konsumen / margin) });
  }
  return out;
}

export interface CuacaHarian {
  tanggal: Date;
  curahHujanMm: number;
  suhuC: number;
  kelembapan: number;
  anomali: boolean;
}

export function seriCuaca(kabupatenKode: string, hari = HARI_SERI): CuacaHarian[] {
  const rng = mulberry32(hashSeed(`cuaca:${kabupatenKode}`));
  const dataranTinggi = ["1106", "1117", "1113"].includes(kabupatenKode);
  return Array.from({ length: hari }, (_, t) => {
    // Oktober: awal musim hujan; anomali curah hujan dataran tinggi 10 hari terakhir
    const basis = 6 + (t / hari) * 7;
    const anomaliAktif = dataranTinggi && t > hari - 12;
    const hujan = Math.max(0, basis * (anomaliAktif ? 2.1 : 1) + gauss(rng) * 5);
    return {
      tanggal: tanggalMundur(t, hari),
      curahHujanMm: Math.round(hujan * 10) / 10,
      suhuC: Math.round(((dataranTinggi ? 20 : 27.5) + gauss(rng) * 1.3) * 10) / 10,
      kelembapan: Math.round(78 + gauss(rng) * 6),
      anomali: anomaliAktif && hujan > basis * 1.6,
    };
  });
}

/** Δ% harga konsumen hari terakhir terhadap rata-rata 7 hari sebelumnya. */
export function delta7(seri: TitikHarga[]) {
  const n = seri.length;
  const last = seri[n - 1].konsumen;
  const prev = seri.slice(n - 8, n - 1);
  const mean = prev.reduce((s, p) => s + p.konsumen, 0) / prev.length;
  return ((last - mean) / mean) * 100;
}

/** Proyeksi Holt (level + tren) dengan pita ketidakpastian melebar. */
export function proyeksi(seri: number[], horizon = 14, alpha = 0.5, beta = 0.3) {
  let level = seri[0];
  let tren = seri[1] - seri[0];
  const residu: number[] = [];
  for (let i = 1; i < seri.length; i++) {
    const prevLevel = level;
    const fitted = level + tren;
    residu.push(seri[i] - fitted);
    level = alpha * seri[i] + (1 - alpha) * (level + tren);
    tren = beta * (level - prevLevel) + (1 - beta) * tren;
  }
  // redam tren agar proyeksi tidak meledak
  const sd = Math.sqrt(residu.slice(-30).reduce((s, r) => s + r * r, 0) / Math.min(30, residu.length));
  return Array.from({ length: horizon }, (_, h) => {
    const damp = (1 - Math.pow(0.85, h + 1)) / 0.15;
    const nilai = level + tren * damp;
    const lebar = 1.28 * sd * Math.sqrt(h + 1);
    return { h: h + 1, nilai: bulat(nilai), bawah: bulat(nilai - lebar), atas: bulat(nilai + lebar) };
  });
}

export function kabupatenBerpasar() {
  const kode = new Set(PASAR.map((p) => p.kabupatenKode));
  return KABUPATEN.filter((k) => kode.has(k.kode));
}

export function pasarDi(kabupatenKode: string): Pasar[] {
  return PASAR.filter((p) => p.kabupatenKode === kabupatenKode);
}

export function namaWilayah(kode: string) {
  const k = kabupatenByKode[kode];
  return k ? (k.jenis === "Kota" ? `Kota ${k.nama}` : `Kab. ${k.nama}`) : kode;
}

export { KOMODITAS, PASAR, KABUPATEN };
