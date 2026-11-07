/**
 * Cold storage: bentuk data backend Go (snake_case), pemetaan ke model UI,
 * klien BFF, dan aturan tampilan (urgensi, nada warna) yang dipakai halaman.
 */

import type { Tone } from "@/lib/design/tokens";
import { komoditasByKode, type IkonKomoditas } from "@/lib/data/reference";
import type {
  ColdStorageView,
  DetailFasilitas,
  FasilitasColdStorage,
  StokColdStorage,
  TitikSuhu,
  TransaksiStok,
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
  /** UUID fasilitas. */
  id: string;
  /** Kode tampilan (CS-01), alias di URL API. */
  kode?: string | null;
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
  batas_min_c?: number;
  batas_maks_c?: number;
  alarm_suhu?: boolean;
  kelembapan_aktual_pct?: number | null;
  telemetri_pada?: string | null;
}

export interface StokWire {
  /** ID BIGINT lot (string desimal). */
  id: string;
  kode?: string | null;
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
  status?: string;
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
  rincian?: { id: string; berhasil: boolean; pesan: string }[];
}

export interface BulkColdStorageResult {
  berhasil: number;
  gagal: number;
  volumeTon: number;
  /** Hasil per lot menurut urutan permintaan. */
  rincian: { id: string; berhasil: boolean; pesan: string }[];
}

export interface TransaksiStokWire {
  /** ID BIGINT transaksi (string desimal). */
  id: string;
  kode?: string | null;
  lot_id: string;
  lot_baru_id: string | null;
  jenis: TransaksiStok["jenis"];
  volume_ton: number;
  komoditas_nama: string;
  fasilitas_asal_id: string | null;
  fasilitas_asal_kode?: string | null;
  fasilitas_tujuan_id: string | null;
  fasilitas_tujuan_kode?: string | null;
  oleh_nama: string | null;
  waktu: string;
  alasan: string;
}

export interface DetailFasilitasWire {
  fasilitas: FasilitasWire;
  telemetri: {
    waktu: string;
    suhu_c: number;
    kelembapan_pct: number | null;
    sumber?: string;
    di_luar_rentang: boolean;
  }[];
  stok: StokWire[];
  transaksi: TransaksiStokWire[];
}

/** Isian transaksi stok; kolom yang wajib bergantung pada jenis. */
export type TransaksiPayload =
  | {
      jenis: "MASUK";
      fasilitasId: string;
      komoditasKode: string;
      pemilik: string;
      volumeTon: number;
    }
  | {
      jenis: "KELUAR";
      lotId: string;
      volumeTon?: number;
      semua?: boolean;
      alasan: string;
    }
  | {
      jenis: "PINDAH";
      lotId: string;
      fasilitasTujuanId: string;
      volumeTon?: number;
      semua?: boolean;
    };

export interface TransaksiHasil {
  transaksiId: string;
  lotId: string;
  lotBaruId: string | null;
  volumeSisaTon: number;
  statusLot: string;
}

/** Badan permintaan transaksi dalam bentuk kontrak API (snake_case). */
export function toTransaksiBody(p: TransaksiPayload): Record<string, unknown> {
  switch (p.jenis) {
    case "MASUK":
      return {
        jenis: p.jenis,
        fasilitas_id: p.fasilitasId,
        komoditas_kode: p.komoditasKode,
        pemilik: p.pemilik,
        volume_ton: p.volumeTon,
      };
    case "KELUAR":
      return {
        jenis: p.jenis,
        lot_id: p.lotId,
        volume_ton: p.volumeTon,
        semua: p.semua,
        alasan: p.alasan,
      };
    case "PINDAH":
      return {
        jenis: p.jenis,
        lot_id: p.lotId,
        fasilitas_tujuan_id: p.fasilitasTujuanId,
        volume_ton: p.volumeTon,
        semua: p.semua,
      };
  }
}

/** Perubahan fasilitas (PATCH); hanya kolom yang diisi yang dikirim dan diubah. */
export interface UbahFasilitasPayload {
  kapasitasTon?: number;
  suhuMinC?: number;
  suhuMaksC?: number;
  status?: FasilitasColdStorage["status"];
}

/** Badan PATCH /v1/cold-storage/fasilitas/:id dalam bentuk kontrak API. */
export function toUbahFasilitasBody(p: UbahFasilitasPayload): Record<string, unknown> {
  const body: Record<string, unknown> = {};
  if (p.kapasitasTon !== undefined) body.kapasitas_ton = p.kapasitasTon;
  if (p.suhuMinC !== undefined) body.suhu_min_c = p.suhuMinC;
  if (p.suhuMaksC !== undefined) body.suhu_maks_c = p.suhuMaksC;
  if (p.status !== undefined) body.status = p.status;
  return body;
}

const BATAS_SUHU_C = { min: -80, maks: 60 } as const;

/**
 * Memeriksa isian ubah fasilitas sebelum dikirim; null bila sah. `suhuMinC`/`suhuMaksC` adalah
 * batas efektif setelah perubahan, jadi mengubah satu sisi tetap diuji terhadap sisi lainnya.
 */
export function galatUbahFasilitas(p: { kapasitasTon?: number; suhuMinC: number; suhuMaksC: number }): string | null {
  if (p.kapasitasTon !== undefined && (!Number.isInteger(p.kapasitasTon) || p.kapasitasTon < 1))
    return "Kapasitas harus bilangan bulat lebih dari 0 ton.";
  for (const v of [p.suhuMinC, p.suhuMaksC])
    if (!Number.isFinite(v) || v < BATAS_SUHU_C.min || v > BATAS_SUHU_C.maks)
      return `Suhu harus antara ${BATAS_SUHU_C.min} dan ${BATAS_SUHU_C.maks} °C.`;
  if (p.suhuMinC >= p.suhuMaksC) return "Suhu minimum harus lebih kecil dari suhu maksimum.";
  return null;
}

/** Satu pembacaan suhu yang dicatat petugas (sumber selalu MANUAL). */
export interface PembacaanManualPayload {
  fasilitasId: string;
  /** ISO 8601 (UTC). */
  waktu: string;
  suhuC: number;
  kelembapanPct?: number;
}

/** Badan POST /v1/cold-storage/telemetri untuk satu pembacaan manual. */
export function toTelemetriBody(p: PembacaanManualPayload): Record<string, unknown> {
  return {
    pembacaan: [
      {
        fasilitas_id: p.fasilitasId,
        waktu: p.waktu,
        suhu_c: p.suhuC,
        ...(p.kelembapanPct !== undefined ? { kelembapan_pct: p.kelembapanPct } : {}),
        sumber: "MANUAL",
      },
    ],
  };
}

export interface HasilTelemetriWire {
  diterima: number;
  duplikat: number;
  di_luar_rentang: {
    fasilitas_id: string;
    waktu: string;
    suhu_c: number;
    batas_min_c: number;
    batas_maks_c: number;
  }[];
}

export interface HasilTelemetri {
  diterima: number;
  duplikat: number;
  /** Pembacaan yang melewati batas alarm fasilitas. */
  diLuarRentang: { waktu: string; suhuC: number; batasMinC: number; batasMaksC: number }[];
}

export function toHasilTelemetri(w: HasilTelemetriWire): HasilTelemetri {
  return {
    diterima: w.diterima,
    duplikat: w.duplikat,
    diLuarRentang: (w.di_luar_rentang ?? []).map((a) => ({
      waktu: a.waktu,
      suhuC: a.suhu_c,
      batasMinC: a.batas_min_c,
      batasMaksC: a.batas_maks_c,
    })),
  };
}

const dua = (n: number) => String(n).padStart(2, "0");

/** Date → nilai `<input type="datetime-local">` (waktu setempat, sampai detik). */
export function keDatetimeLocal(d: Date): string {
  return `${d.getFullYear()}-${dua(d.getMonth() + 1)}-${dua(d.getDate())}T${dua(d.getHours())}:${dua(d.getMinutes())}:${dua(d.getSeconds())}`;
}

/** Nilai datetime-local (waktu setempat) → Date; null bila kosong atau tidak valid. */
export function dariDatetimeLocal(v: string): Date | null {
  if (!v) return null;
  const d = new Date(v);
  return Number.isNaN(d.getTime()) ? null : d;
}

/** Selisih jam sensor yang masih ditoleransi backend terhadap waktu server. */
const TOLERANSI_MASA_DEPAN_MS = 5 * 60_000;

/** Memeriksa isian catat suhu manual; null bila sah. */
export function galatPembacaan(p: { suhuC: number; kelembapanPct?: number; waktu: Date | null }, sekarang = new Date()): string | null {
  if (!Number.isFinite(p.suhuC) || p.suhuC < BATAS_SUHU_C.min || p.suhuC > BATAS_SUHU_C.maks)
    return `Suhu harus antara ${BATAS_SUHU_C.min} dan ${BATAS_SUHU_C.maks} °C.`;
  if (p.kelembapanPct !== undefined && (!Number.isFinite(p.kelembapanPct) || p.kelembapanPct < 0 || p.kelembapanPct > 100))
    return "Kelembapan harus antara 0 dan 100%.";
  if (!p.waktu) return "Waktu pembacaan wajib diisi.";
  if (p.waktu.getTime() > sekarang.getTime() + TOLERANSI_MASA_DEPAN_MS) return "Waktu pembacaan tidak boleh di masa depan.";
  return null;
}

/** Angka isian (boleh berkoma desimal) → number; NaN bila kosong atau bukan angka. */
export function angkaIsian(v: string): number {
  const t = v.trim().replace(",", ".");
  return t === "" ? Number.NaN : Number(t);
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

/** Kode komoditas master (sama dengan `reference.ts`) → ikon; kode yang tak dikenal memakai ikon sayur. */
export function ikonKomoditas(kode: string): IkonKomoditas {
  return komoditasByKode[kode]?.ikon ?? "sayur";
}

export function toFasilitas(w: FasilitasWire): FasilitasColdStorage {
  return {
    id: w.id,
    kode: w.kode ?? null,
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
    batasMinC: w.batas_min_c,
    batasMaksC: w.batas_maks_c,
    alarmSuhu: w.alarm_suhu,
    telemetriPada: w.telemetri_pada,
  };
}

export function toStok(w: StokWire): StokColdStorage {
  return {
    id: w.id,
    kode: w.kode ?? null,
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
    status: w.status === "PRIORITAS_JUAL" ? "PRIORITAS_JUAL" : "AKTIF",
  };
}

export function toTransaksi(w: TransaksiStokWire): TransaksiStok {
  return {
    id: w.id,
    kode: w.kode ?? null,
    lotId: w.lot_id,
    lotBaruId: w.lot_baru_id,
    jenis: w.jenis,
    volumeTon: w.volume_ton,
    komoditasNama: sentenceCase(w.komoditas_nama),
    fasilitasAsalId: w.fasilitas_asal_id,
    fasilitasAsalKode: w.fasilitas_asal_kode ?? null,
    fasilitasTujuanId: w.fasilitas_tujuan_id,
    fasilitasTujuanKode: w.fasilitas_tujuan_kode ?? null,
    olehNama: w.oleh_nama,
    waktu: w.waktu,
    alasan: w.alasan,
  };
}

export function toDetailFasilitas(w: DetailFasilitasWire): DetailFasilitas {
  return {
    fasilitas: toFasilitas(w.fasilitas),
    telemetri: w.telemetri.map<TitikSuhu>((t) => ({
      waktu: t.waktu,
      suhuC: t.suhu_c,
      kelembapanPct: t.kelembapan_pct,
      sumber: t.sumber,
      diLuarRentang: t.di_luar_rentang,
    })),
    stok: w.stok.map(toStok),
    transaksi: w.transaksi.map(toTransaksi),
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

/** Pemutus seri: id BIGINT diurut secara numerik ("9" sebelum "10"), id lain secara leksikografis. */
function bandingId(a: string, b: string): number {
  if (/^\d+$/.test(a) && /^\d+$/.test(b)) {
    const x = BigInt(a);
    const y = BigInt(b);
    return x < y ? -1 : x > y ? 1 : 0;
  }
  return a < b ? -1 : a > b ? 1 : 0;
}

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
    return c !== 0 ? c * arah : bandingId(a.id, b.id);
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
      rincian: res.rincian ?? [],
    };
  },

  detail: async (id: string): Promise<DetailFasilitas> =>
    toDetailFasilitas(
      await bffFetch<DetailFasilitasWire>(
        `/api/cold-storage/fasilitas/${encodeURIComponent(id)}`,
      ),
    ),

  ubahFasilitas: async (id: string, p: UbahFasilitasPayload): Promise<FasilitasColdStorage> =>
    toFasilitas(
      await bffFetch<FasilitasWire>(`/api/cold-storage/fasilitas/${encodeURIComponent(id)}`, {
        method: "PATCH",
        body: JSON.stringify(toUbahFasilitasBody(p)),
      }),
    ),

  catatTelemetri: async (p: PembacaanManualPayload): Promise<HasilTelemetri> =>
    toHasilTelemetri(
      await bffFetch<HasilTelemetriWire>("/api/cold-storage/telemetri", {
        method: "POST",
        body: JSON.stringify(toTelemetriBody(p)),
      }),
    ),

  transaksi: async (p: TransaksiPayload): Promise<TransaksiHasil> => {
    const res = await bffFetch<{
      transaksi_id: string;
      lot_id: string;
      lot_baru_id: string | null;
      volume_sisa_ton: number;
      status_lot: string;
    }>("/api/cold-storage/transaksi", {
      method: "POST",
      body: JSON.stringify(toTransaksiBody(p)),
    });
    return {
      transaksiId: res.transaksi_id,
      lotId: res.lot_id,
      lotBaruId: res.lot_baru_id,
      volumeSisaTon: res.volume_sisa_ton,
      statusLot: res.status_lot,
    };
  },
};
