/**
 * Pemetaan EWS dari representasi API Go (snake_case) ke model UI.
 * Endpoint: GET /v1/ews, GET /v1/ews/aturan.
 */

import type { EwsAlert, JenisEws, StatusEws } from "@/lib/data/types";
import type { Severity } from "@/lib/design/tokens";
import { formatWilayah } from "./cold-storage";

export interface EwsAlertResponse {
  id: string;
  severity: Severity;
  jenis: JenisEws;
  judul: string;
  pesan: string;
  komoditas_kode: string | null;
  komoditas_nama: string | null;
  kabupaten_kode: string;
  /** Nama resmi kabupaten/kota, mis. "KOTA BANDA ACEH". */
  wilayah: string;
  nilai: number;
  satuan_nilai: string;
  skor_dampak: number;
  status: StatusEws;
  sumber: string;
  dibuat_pada: string;
}

export interface EwsAturanResponse {
  id: string;
  komoditas_kode: string | null;
  komoditas_nama: string | null;
  kabupaten_kode: string | null;
  jenis: JenisEws;
  ambang_waspada: number;
  ambang_siaga: number;
  ambang_kritis: number;
  jendela_hari: number;
  cooldown_jam: number;
  kanal: string[];
  aktif: boolean;
  keterangan: string;
  versi: number;
}

export interface EwsAturan {
  id: string;
  komoditasKode?: string;
  komoditasNama?: string;
  kabupatenKode?: string;
  jenis: JenisEws;
  waspada: number;
  siaga: number;
  kritis: number;
  jendelaHari: number;
  cooldownJam: number;
  kanal: string[];
  aktif: boolean;
  keterangan: string;
  /** Versi yang dibaca; dikirim kembali saat menyimpan (optimistic locking, 409 bila usang). */
  versi: number;
}

/** Nilai yang dapat diubah pada aturan ambang (PUT /v1/ews/aturan/{id}). */
export interface UbahAturanInput {
  waspada: number;
  siaga: number;
  kritis: number;
  jendelaHari: number;
  cooldownJam: number;
  kanal: string[];
  aktif: boolean;
  keterangan: string;
  versi: number;
}

/** Ringkasan satu putaran rule engine (POST /v1/ews/evaluasi). */
export interface EvaluasiEws {
  aturan: number;
  diukur: number;
  dibuat: number;
  dinaikkan: number;
  dilewati: number;
  diredam: number;
  galat: number;
}

export const KANAL_EWS = ["DASHBOARD", "PUSH", "EMAIL", "WHATSAPP"] as const;

const SATUAN: readonly EwsAlert["satuanNilai"][] = ["%", "mm", "ton"];

export function toEwsAlert(r: EwsAlertResponse): EwsAlert {
  return {
    id: r.id,
    severity: r.severity,
    jenis: r.jenis,
    judul: r.judul,
    komoditasKode: r.komoditas_kode ?? undefined,
    komoditasNama: r.komoditas_nama ?? undefined,
    kabupatenKode: r.kabupaten_kode,
    wilayah: formatWilayah(r.wilayah),
    nilai: r.nilai,
    satuanNilai: SATUAN.find((s) => s === r.satuan_nilai) ?? "%",
    pesan: r.pesan,
    waktu: r.dibuat_pada,
    status: r.status,
    skorDampak: r.skor_dampak,
    sumber: r.sumber,
  };
}

export function toEwsAturan(r: EwsAturanResponse): EwsAturan {
  return {
    id: r.id,
    komoditasKode: r.komoditas_kode ?? undefined,
    komoditasNama: r.komoditas_nama ?? undefined,
    kabupatenKode: r.kabupaten_kode ?? undefined,
    jenis: r.jenis,
    waspada: r.ambang_waspada,
    siaga: r.ambang_siaga,
    kritis: r.ambang_kritis,
    jendelaHari: r.jendela_hari,
    cooldownJam: r.cooldown_jam,
    kanal: r.kanal,
    aktif: r.aktif,
    keterangan: r.keterangan,
    versi: r.versi,
  };
}

/** Badan permintaan PUT /v1/ews/aturan/{id}. */
export function toUbahAturanBody(i: UbahAturanInput) {
  return {
    versi: i.versi,
    ambang_waspada: i.waspada,
    ambang_siaga: i.siaga,
    ambang_kritis: i.kritis,
    jendela_hari: i.jendelaHari,
    cooldown_jam: i.cooldownJam,
    kanal: i.kanal,
    aktif: i.aktif,
    keterangan: i.keterangan,
  };
}

/** Galat bila ambang tidak positif dan menaik (waspada < siaga < kritis); null bila sah. */
export function validasiAmbang(i: Pick<UbahAturanInput, "waspada" | "siaga" | "kritis">): string | null {
  if (![i.waspada, i.siaga, i.kritis].every(Number.isFinite) || i.waspada <= 0) return "Ambang harus berupa angka positif.";
  if (!(i.waspada < i.siaga && i.siaga < i.kritis)) return "Urutan ambang harus waspada < siaga < kritis.";
  return null;
}

/** Satuan ambang per jenis alert: lonjakan harga dan stok dalam %, cuaca dalam mm. */
export function satuanAmbang(jenis: JenisEws): string {
  return jenis === "ANOMALI_CUACA" ? "mm" : "%";
}

/** Kanal notifikasi yang dapat diatur pengguna (dikirim lewat outbox). */
export type KanalNotifikasi = "EMAIL" | "PUSH";

export interface PreferensiResponse {
  kanal: KanalNotifikasi;
  severity_min: Severity;
  aktif: boolean;
  /** false: server belum mengonfigurasi pengiriman nyata untuk kanal ini (notifikasi hanya dicatat). */
  tersedia: boolean;
}

export interface PreferensiNotifikasiResponse {
  preferensi: PreferensiResponse[];
  vapid_public_key: string;
}

export interface PreferensiNotifikasi {
  kanal: KanalNotifikasi;
  severityMin: Severity;
  aktif: boolean;
  tersedia: boolean;
}

export interface PreferensiView {
  preferensi: PreferensiNotifikasi[];
  /** Kunci publik VAPID untuk berlangganan Web Push; kosong bila push tidak dikonfigurasi. */
  vapidPublicKey: string;
}

export function toPreferensiView(r: PreferensiNotifikasiResponse): PreferensiView {
  return {
    preferensi: r.preferensi.map((p) => ({ kanal: p.kanal, severityMin: p.severity_min, aktif: p.aktif, tersedia: p.tersedia })),
    vapidPublicKey: r.vapid_public_key,
  };
}

/** Badan PUT /v1/ews/notifikasi/preferensi. */
export function toPreferensiBody(prefs: Pick<PreferensiNotifikasi, "kanal" | "severityMin" | "aktif">[]) {
  return { preferensi: prefs.map((p) => ({ kanal: p.kanal, severity_min: p.severityMin, aktif: p.aktif })) };
}

/** Badan PUT /v1/ews/notifikasi/langganan-push dari PushSubscription.toJSON(); null bila tidak lengkap. */
export function toLanggananPushBody(sub: { endpoint?: string; keys?: Record<string, string> }) {
  const { endpoint, keys } = sub;
  if (!endpoint || !keys?.p256dh || !keys.auth) return null;
  return { endpoint, p256dh: keys.p256dh, auth: keys.auth };
}

/** Kunci VAPID (base64url) → bytes untuk pushManager.subscribe. */
export function kunciVapidKeBytes(base64Url: string): Uint8Array {
  const padded = base64Url.replace(/-/g, "+").replace(/_/g, "/").padEnd(Math.ceil(base64Url.length / 4) * 4, "=");
  return Uint8Array.from(atob(padded), (c) => c.charCodeAt(0));
}
