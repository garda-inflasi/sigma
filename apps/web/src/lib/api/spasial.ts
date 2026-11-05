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

/** Geometri GeoJSON satu wilayah (Polygon atau MultiPolygon). */
export type GeometriGeoJSON =
  | { type: "Polygon"; coordinates: number[][][] }
  | { type: "MultiPolygon"; coordinates: number[][][][] };

/** `GET /v1/spasial/geometri`: FeatureCollection; `perkiraan` benar bila bukan batas resmi. */
export interface GeometriResponse {
  type: "FeatureCollection";
  features: {
    id: string;
    properties: {
      kode: string;
      nama: string;
      level: string;
      sumber: string;
      perkiraan: boolean;
      keterangan: string;
    };
    geometry: GeometriGeoJSON;
  }[];
}

export interface GeometriWilayah {
  kode: string;
  nama: string;
  /** Poligon perkiraan, bukan batas administratif resmi. */
  perkiraan: boolean;
  geometry: GeometriGeoJSON;
  /** Cincin luar tiap poligon sebagai [bujur, lintang]; lubang diabaikan (dipakai peta skema SVG). */
  cincinLuar: [number, number][][];
}

export function toGeometri(r: GeometriResponse): GeometriWilayah[] {
  return r.features.map((f) => ({
    kode: f.properties.kode,
    nama: formatWilayah(f.properties.nama),
    perkiraan: f.properties.perkiraan,
    geometry: f.geometry,
    cincinLuar: (f.geometry.type === "Polygon"
      ? [f.geometry.coordinates]
      : f.geometry.coordinates
    ).map((poligon) => poligon[0].map(([lng, lat]) => [lng, lat] as [number, number])),
  }));
}

/** Tingkat zoom bawaan untuk geometri peta (cukup rinci untuk satu provinsi). */
export const ZOOM_GEOMETRI = 8;

/**
 * FeatureCollection untuk lapisan Mapbox: tiap wilayah membawa `status` marker-nya
 * supaya warna isian ditentukan data, bukan kode komponen.
 */
export function geometriKeFeatureCollection(
  geometri: GeometriWilayah[],
  markers: MarkerWilayah[],
) {
  const status = new Map(markers.map((m) => [m.kode, m.status]));
  return {
    type: "FeatureCollection" as const,
    features: geometri.map((g) => ({
      type: "Feature" as const,
      id: g.kode,
      properties: { kode: g.kode, nama: g.nama, status: status.get(g.kode) ?? "normal", perkiraan: g.perkiraan },
      geometry: g.geometry,
    })),
  };
}

/** Isian optimasi rute; nilai kosong memakai bawaan server. */
export interface OptimasiParams {
  komoditasKode: string;
  /** Tambahan/kurangan surplus (ton) per wilayah untuk simulasi skenario; tidak dapat disimpan. */
  penyesuaian?: { kabupatenKode: string; ton: number }[];
  maksJamTempuh?: number;
  simpan?: boolean;
}

export interface OptimasiResponse {
  komoditas_kode: string;
  komoditas_nama: string;
  periode: string;
  rute: {
    asal_kode: string;
    asal: string;
    tujuan_kode: string;
    tujuan: string;
    volume_ton: number;
    jarak_km: number;
    jam_tempuh: number;
    biaya: number;
  }[];
  total_volume_ton: number;
  total_biaya: number;
  belum_terpenuhi: { kode: string; nama: string; ton: number }[];
  surplus_sisa: { kode: string; nama: string; ton: number }[];
  /** Biaya rencana serakah (pasangan terdekat dulu) untuk volume yang sama; null bila tak sebanding. */
  biaya_pembanding: number | null;
  penghematan_persen: number | null;
  optimasi_id: string | null;
}

export interface RencanaOptimasi {
  komoditasNama: string;
  periode: string;
  rute: Rute[];
  totalVolumeTon: number;
  totalBiaya: number;
  /** Defisit yang tidak tertutup: nama wilayah dan ton. */
  belumTerpenuhi: { kode: string; nama: string; ton: number }[];
  surplusSisa: { kode: string; nama: string; ton: number }[];
  /** Penghematan terhadap rencana serakah (persen); null bila tidak sebanding. */
  penghematanPersen: number | null;
  /** Terisi bila rencana disimpan sebagai usulan rute. */
  optimasiId: string | null;
}

/** Badan permintaan POST /v1/spasial/optimasi-rute (snake_case, kolom kosong dibuang). */
export function toOptimasiBody(p: OptimasiParams): Record<string, unknown> {
  return {
    komoditas_kode: p.komoditasKode,
    penyesuaian: p.penyesuaian?.length
      ? p.penyesuaian.map((a) => ({ kabupaten_kode: a.kabupatenKode, ton: a.ton }))
      : undefined,
    maks_jam_tempuh: p.maksJamTempuh || undefined,
    simpan: p.simpan || undefined,
  };
}

export function toRencanaOptimasi(r: OptimasiResponse): RencanaOptimasi {
  return {
    komoditasNama: r.komoditas_nama,
    periode: r.periode,
    rute: r.rute.map((x) => ({
      id: `${x.asal_kode}-${x.tujuan_kode}`,
      asal: formatWilayah(x.asal),
      tujuan: formatWilayah(x.tujuan),
      komoditas: r.komoditas_nama,
      volumeTon: x.volume_ton,
      jarakKm: x.jarak_km,
      jamTempuh: x.jam_tempuh,
      biaya: x.biaya,
    })),
    totalVolumeTon: r.total_volume_ton,
    totalBiaya: r.total_biaya,
    belumTerpenuhi: r.belum_terpenuhi.map((s) => ({ ...s, nama: formatWilayah(s.nama) })),
    surplusSisa: r.surplus_sisa.map((s) => ({ ...s, nama: formatWilayah(s.nama) })),
    penghematanPersen: r.penghematan_persen,
    optimasiId: r.optimasi_id,
  };
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
