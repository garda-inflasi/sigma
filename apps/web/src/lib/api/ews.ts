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
  jenis: JenisEws;
  ambang_waspada: number;
  ambang_siaga: number;
  ambang_kritis: number;
  jendela_hari: number;
  kanal: string[];
  aktif: boolean;
  keterangan: string;
}

export interface EwsAturan {
  id: string;
  komoditasKode?: string;
  komoditasNama?: string;
  jenis: JenisEws;
  waspada: number;
  siaga: number;
  kritis: number;
  jendelaHari: number;
  kanal: string[];
  aktif: boolean;
  keterangan: string;
}

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
    jenis: r.jenis,
    waspada: r.ambang_waspada,
    siaga: r.ambang_siaga,
    kritis: r.ambang_kritis,
    jendelaHari: r.jendela_hari,
    kanal: r.kanal,
    aktif: r.aktif,
    keterangan: r.keterangan,
  };
}

/** Satuan ambang per jenis alert: lonjakan harga dan stok dalam %, cuaca dalam mm. */
export function satuanAmbang(jenis: JenisEws): string {
  return jenis === "ANOMALI_CUACA" ? "mm" : "%";
}
