/**
 * Pemetaan peta spasial dari representasi API Go (snake_case) ke model UI.
 * Endpoint: GET /v1/spasial/{wilayah,rute,kandidat-cold-storage}.
 */

import type { MarkerWilayah } from "@/components/map/marker-pin";
import { formatWilayah } from "./cold-storage";

export interface MarkerResponse {
  kode: string;
  /** Nama resmi, mis. "KOTA BANDA ACEH". */
  nama: string;
  lat: number;
  lng: number;
  delta: number;
  komoditas: string;
  status: MarkerWilayah["status"];
  punya_pasar: boolean;
}

export interface RuteResponse {
  id: string;
  asal: string;
  tujuan: string;
  komoditas_nama: string;
  volume_ton: number;
  jarak_km: number;
  jam_tempuh: number;
  biaya: number;
}

export interface KandidatResponse {
  id: string;
  lokasi: string;
  skor: number;
  alasan: string;
  status: string;
}

export interface Rute {
  id: string;
  asal: string;
  tujuan: string;
  komoditas: string;
  volumeTon: number;
  jarakKm: number;
  jamTempuh: number;
  biaya: number;
}

export interface KandidatColdStorage {
  id: string;
  lokasi: string;
  skor: number;
  alasan: string;
  /** Sudah direncanakan atau usulan baru. */
  rencana: boolean;
}

export function toMarker(r: MarkerResponse): MarkerWilayah {
  return {
    kode: r.kode,
    nama: formatWilayah(r.nama),
    lat: r.lat,
    lng: r.lng,
    delta: r.delta,
    komoditas: r.komoditas,
    status: r.status,
    punyaPasar: r.punya_pasar,
  };
}

export function toRute(r: RuteResponse): Rute {
  return {
    id: r.id,
    asal: formatWilayah(r.asal),
    tujuan: formatWilayah(r.tujuan),
    komoditas: r.komoditas_nama,
    volumeTon: r.volume_ton,
    jarakKm: r.jarak_km,
    jamTempuh: r.jam_tempuh,
    biaya: r.biaya,
  };
}

export function toKandidat(r: KandidatResponse): KandidatColdStorage {
  return {
    id: r.id,
    lokasi: r.lokasi,
    skor: r.skor,
    alasan: r.alasan,
    rencana: r.status === "RENCANA",
  };
}
