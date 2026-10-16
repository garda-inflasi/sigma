/**
 * Cold storage: bentuk data backend Go (snake_case), pemetaan ke model UI,
 * klien BFF, dan aturan tampilan (urgensi, nada warna) yang dipakai halaman.
 */

import type { Tone } from "@/lib/design/tokens";
import type { IkonKomoditas } from "@/lib/data/reference";
import type {
  ColdStorageView,
  FasilitasColdStorage,
  StokColdStorage,
} from "@/lib/data/types";
import { bffFetch } from "./client";
import {
  COLD_STORAGE_MAX_LIMIT,
  toQueryString,
  type ColdStorageBulkAction,
  type ColdStorageParams,
  type ColdStorageSort,
} from "./cold-storage-query";

// ── Bentuk data backend ───────────────────────────────────────────────

export interface FasilitasWire {
  id: string;
  nama: string;
  kabupaten_kode: string;
  /** Nama resmi pada master wilayah, mis. "KABUPATEN ACEH BESAR". */
  kabupaten_nama: string;
  lat: number;
  lng: number;
  kapasitas_ton: number;
  terisi_ton: number;
  utilisasi_persen: number;
  suhu_target_c: number;
  suhu_aktual_c: number | null;
  status: string;
}

export interface StokWire {
  id: string;
  cold_storage_id: string;
  cold_storage_nama: string;
  komoditas_kode: string;
  komoditas_nama: string;
  volume_ton: number;
  masuk_pada: string;
  kedaluwarsa_pada: string;
  sisa_masa_simpan_hari: number;
  risiko_food_loss: number;
  pemilik: string;
}

export interface ColdStorageWire {
  fasilitas: FasilitasWire[];
  stok: StokWire[];
  meta: {
    page: number;
    limit: number;
    total: number;
    total_pages: number;
    volume_hasil_ton: number;
  };
  ringkasan: {
    total_lot: number;
    total_volume_ton: number;
    risiko_tinggi_lot: number;
    risiko_tinggi_ton: number;
    ambang_risiko_tinggi: number;
  };
}

export interface BulkColdStorageWire {
  berhasil: number;
  gagal: number;
  volume_ton: number;
}

export interface BulkColdStorageResult {
  berhasil: number;
  gagal: number;
  volumeTon: number;
}

// ── Pemetaan backend → UI ─────────────────────────────────────────────

const STATUS_FASILITAS = ["OPERASIONAL", "PERAWATAN", "RENCANA"] as const;

function toStatus(status: string): FasilitasColdStorage["status"] {
  // Nilai tak dikenal diperlakukan sebagai rencana: tanpa telemetri dan tanpa stok.
  return (STATUS_FASILITAS as readonly string[]).includes(status)
    ? (status as FasilitasColdStorage["status"])
    : "RENCANA";
}

const titleCase = (s: string) =>
  s
    .toLowerCase()
    .replace(
      /(^|\s)(\p{L})/gu,
      (_, space: string, ch: string) => space + ch.toUpperCase(),
    );

/** Huruf besar hanya di awal kalimat (aturan penulisan Garda): "Cabai Merah Keriting" → "Cabai merah keriting". */
export function sentenceCase(s: string): string {
  const t = s.trim().toLowerCase();
  return t.charAt(0).toUpperCase() + t.slice(1);
}

/** Nama resmi wilayah → label ringkas: "KABUPATEN ACEH BESAR" → "Kab. Aceh Besar", "KOTA LANGSA" → "Kota Langsa". */
export function formatWilayah(namaResmi: string): string {
  const nama = namaResmi.trim().toUpperCase();
  if (nama.startsWith("KABUPATEN "))
    return `Kab. ${titleCase(nama.slice("KABUPATEN ".length))}`;
  if (nama.startsWith("KOTA "))
    return `Kota ${titleCase(nama.slice("KOTA ".length))}`;
  return titleCase(nama);
}

/** Kode komoditas backend (slug) → ikon; yang tak dikenal memakai ikon sayur. */
const IKON_PREFIKS: [string, IkonKomoditas][] = [
  ["cabai", "cabai"],
  ["bawang", "bawang"],
  ["beras", "beras"],
  ["telur", "telur"],
  ["daging_ayam", "ayam"],
  ["daging_sapi", "sapi"],
  ["ikan", "ikan"],
  ["minyak", "minyak"],
  ["gula", "gula"],
];

export function ikonKomoditas(kode: string): IkonKomoditas {
  const k = kode.toLowerCase();
  return (
    IKON_PREFIKS.find(([prefiks]) => k.startsWith(prefiks))?.[1] ?? "sayur"
  );
}

export function toFasilitas(w: FasilitasWire): FasilitasColdStorage {
  return {
    id: w.id,
    nama: w.nama,
    wilayah: formatWilayah(w.kabupaten_nama),
    kabupatenKode: w.kabupaten_kode,
    kapasitasTon: w.kapasitas_ton,
    terisiTon: w.terisi_ton,
    suhuTargetC: w.suhu_target_c,
    suhuAktualC: w.suhu_aktual_c,
    status: toStatus(w.status),
    lat: w.lat,
    lng: w.lng,
  };
}

export function toStok(w: StokWire): StokColdStorage {
  return {
    id: w.id,
    fasilitasId: w.cold_storage_id,
    fasilitasNama: w.cold_storage_nama,
    komoditasKode: w.komoditas_kode,
    komoditasNama: sentenceCase(w.komoditas_nama),
    ikon: ikonKomoditas(w.komoditas_kode),
    volumeTon: w.volume_ton,
    masuk: w.masuk_pada,
    sisaShelfLifeHari: w.sisa_masa_simpan_hari,
    risikoFoodLoss: w.risiko_food_loss,
    pemilik: w.pemilik,
  };
}

export function toColdStorageView(w: ColdStorageWire): ColdStorageView {
  return {
    fasilitas: w.fasilitas.map(toFasilitas),
    stok: w.stok.map(toStok),
    halaman: {
      page: w.meta.page,
      limit: w.meta.limit,
      total: w.meta.total,
      totalPages: w.meta.total_pages,
      volumeHasilTon: w.meta.volume_hasil_ton,
    },
    ringkasan: {
      totalLot: w.ringkasan.total_lot,
      totalVolumeTon: w.ringkasan.total_volume_ton,
      risikoTinggiLot: w.ringkasan.risiko_tinggi_lot,
      risikoTinggiTon: w.ringkasan.risiko_tinggi_ton,
      ambangRisikoTinggi: w.ringkasan.ambang_risiko_tinggi,
    },
  };
}

// ── Aturan tampilan ───────────────────────────────────────────────────

/** Sisa masa simpan (hari) pada atau di bawah batas ini ditandai merah / kuning. */
export const URGENSI_KRITIS_HARI = 5;
export const URGENSI_WASPADA_HARI = 14;
/** Selisih suhu aktual terhadap target (°C) yang dianggap menyimpang. */
export const AMBANG_DEVIASI_SUHU_C = 1.5;

export type Urgensi = "kritis" | "waspada" | "aman";

export function urgensiLot(sisaHari: number): Urgensi {
  if (sisaHari <= URGENSI_KRITIS_HARI) return "kritis";
  if (sisaHari <= URGENSI_WASPADA_HARI) return "waspada";
  return "aman";
}

/** Persentase terisi; 0 bila kapasitas belum ada. */
export function utilisasiPersen(
  f: Pick<FasilitasColdStorage, "terisiTon" | "kapasitasTon">,
): number {
  return f.kapasitasTon > 0 ? (f.terisiTon / f.kapasitasTon) * 100 : 0;
}

/** Warna batang utilisasi: hampir penuh (merah), optimal (teal), longgar (biru). */
export function nadaUtilisasi(
  persen: number,
): Extract<Tone, "defisit" | "data" | "surplus"> {
  if (persen > 85) return "defisit";
  if (persen > 65) return "data";
  return "surplus";
}

export function labelUtilisasi(persen: number): string {
  if (persen > 85) return "Hampir penuh";
  if (persen > 65) return "Terpakai optimal";
  return "Kapasitas longgar";
}

/** Selisih mutlak suhu aktual dan target; null bila tanpa telemetri. */
export function deviasiSuhu(
  f: Pick<FasilitasColdStorage, "suhuAktualC" | "suhuTargetC">,
): number | null {
  return f.suhuAktualC === null
    ? null
    : Math.abs(f.suhuAktualC - f.suhuTargetC);
}

/** Warna batang risiko food loss: di atas ambang berisiko tinggi (merah), menengah (kuning), rendah (hijau). */
export function nadaRisiko(
  risiko: number,
  ambangTinggi: number,
): Extract<Tone, "defisit" | "waspada" | "normal"> {
  if (risiko > ambangTinggi) return "defisit";
  if (risiko > 0.35) return "waspada";
  return "normal";
}

// ── Mode mock ─────────────────────────────────────────────────────────

const DEFAULT_LIMIT = 10;
const AMBANG_RISIKO_TINGGI = 0.6;
const round1 = (n: number) => Math.round(n * 10) / 10;

const SORT_KEY: Record<
  ColdStorageSort,
  (s: StokColdStorage) => string | number
> = {
  sisa_masa_simpan: (s) => s.sisaShelfLifeHari,
  komoditas: (s) => s.komoditasNama.toLowerCase(),
  fasilitas: (s) => s.fasilitasNama.toLowerCase(),
  volume: (s) => s.volumeTon,
  masuk: (s) => Date.parse(s.masuk),
  risiko: (s) => s.risikoFoodLoss,
  pemilik: (s) => s.pemilik.toLowerCase(),
};

/**
 * Menjalankan kriteria daftar pada data mock dengan semantik yang sama seperti
 * backend (filter, cari, urut dengan id sebagai pemutus seri, halaman, agregat),
 * agar halaman berjalan identik tanpa backend.
 */
export function queryMockColdStorage(
  all: { fasilitas: FasilitasColdStorage[]; stok: StokColdStorage[] },
  p: ColdStorageParams = {},
): ColdStorageView {
  const limit = Math.min(
    Math.max(p.limit ?? DEFAULT_LIMIT, 1),
    COLD_STORAGE_MAX_LIMIT,
  );
  const q = p.q?.trim().toLowerCase();

  const cocok = all.stok.filter(
    (s) =>
      (!p.fasilitasId || s.fasilitasId === p.fasilitasId) &&
      (!q ||
        s.komoditasNama.toLowerCase().includes(q) ||
        s.pemilik.toLowerCase().includes(q)),
  );

  const key = SORT_KEY[p.sort ?? "sisa_masa_simpan"];
  const arah = p.desc ? -1 : 1;
  const urut = [...cocok].sort((a, b) => {
    const ka = key(a);
    const kb = key(b);
    const c = ka < kb ? -1 : ka > kb ? 1 : 0;
    return c !== 0 ? c * arah : a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
  });

  const totalPages = Math.ceil(urut.length / limit);
  const page = Math.max(p.page ?? 1, 1);
  const berisiko = all.stok.filter(
    (s) => s.risikoFoodLoss > AMBANG_RISIKO_TINGGI,
  );

  return {
    fasilitas: all.fasilitas,
    stok: urut.slice((page - 1) * limit, page * limit),
    halaman: {
      page,
      limit,
      total: urut.length,
      totalPages,
      volumeHasilTon: round1(urut.reduce((a, s) => a + s.volumeTon, 0)),
    },
    ringkasan: {
      totalLot: all.stok.length,
      totalVolumeTon: round1(all.stok.reduce((a, s) => a + s.volumeTon, 0)),
      risikoTinggiLot: berisiko.length,
      risikoTinggiTon: round1(berisiko.reduce((a, s) => a + s.volumeTon, 0)),
      ambangRisikoTinggi: AMBANG_RISIKO_TINGGI,
    },
  };
}

// ── Klien BFF ─────────────────────────────────────────────────────────

/** Klien cold storage — semua panggilan melalui Route Handler BFF (`/api/cold-storage`). */
export const coldStorageClient = {
  list: async (params: ColdStorageParams = {}): Promise<ColdStorageView> => {
    const qs = toQueryString(params);
    return toColdStorageView(
      await bffFetch<ColdStorageWire>(`/api/cold-storage${qs ? `?${qs}` : ""}`),
    );
  },

  bulk: async (
    action: ColdStorageBulkAction,
    ids: string[],
  ): Promise<BulkColdStorageResult> => {
    const res = await bffFetch<BulkColdStorageWire>("/api/cold-storage/bulk", {
      method: "POST",
      body: JSON.stringify({ action, ids }),
    });
    return {
      berhasil: res.berhasil,
      gagal: res.gagal,
      volumeTon: res.volume_ton,
    };
  },
};
