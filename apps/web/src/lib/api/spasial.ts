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
    ).map((poligon) =>
      poligon[0].map(([lng, lat]) => [lng, lat] as [number, number]),
    ),
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
      properties: {
        kode: g.kode,
        nama: g.nama,
        status: status.get(g.kode) ?? "normal",
        perkiraan: g.perkiraan,
      },
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
      ? p.penyesuaian.map((a) => ({
          kabupaten_kode: a.kabupatenKode,
          ton: a.ton,
        }))
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
    belumTerpenuhi: r.belum_terpenuhi.map((s) => ({
      ...s,
      nama: formatWilayah(s.nama),
    })),
    surplusSisa: r.surplus_sisa.map((s) => ({
      ...s,
      nama: formatWilayah(s.nama),
    })),
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

/** `GET /v1/spasial/neraca`: neraca pasokan satu komoditas di satu wilayah. */
export interface NeracaResponse {
  komoditas_kode: string;
  komoditas_nama: string;
  kabupaten_kode: string;
  /** Nama resmi, mis. "KOTA BANDA ACEH". */
  wilayah: string;
  periode: string;
  surplus_defisit_ton: number;
  delta_harga_pct: number;
  /** DEMO = data contoh; selain itu data resmi. */
  sumber: string;
}

export type StatusNeraca = "surplus" | "defisit" | "seimbang";

export interface NeracaWilayah {
  /** Kunci unik baris: komoditas + kabupaten. */
  id: string;
  komoditasKode: string;
  komoditas: string;
  kabupatenKode: string;
  wilayah: string;
  periode: string;
  /** Positif = surplus, negatif = defisit. */
  surplusDefisitTon: number;
  deltaHargaPct: number;
  sumber: string;
  demo: boolean;
  status: StatusNeraca;
}

export function statusNeraca(ton: number): StatusNeraca {
  return ton > 0 ? "surplus" : ton < 0 ? "defisit" : "seimbang";
}

export function toNeraca(r: NeracaResponse): NeracaWilayah {
  return {
    id: `${r.komoditas_kode}-${r.kabupaten_kode}`,
    komoditasKode: r.komoditas_kode,
    komoditas: r.komoditas_nama,
    kabupatenKode: r.kabupaten_kode,
    wilayah: formatWilayah(r.wilayah),
    periode: r.periode,
    surplusDefisitTon: r.surplus_defisit_ton,
    deltaHargaPct: r.delta_harga_pct,
    sumber: r.sumber,
    demo: r.sumber === "DEMO",
    status: statusNeraca(r.surplus_defisit_ton),
  };
}

/** Query neraca; komoditas/periode kosong dibuang (server memakai periode terbaru dan semua komoditas). */
export function toNeracaQuery(
  komoditasKode?: string,
  periode?: string,
): string {
  const q = new URLSearchParams();
  if (komoditasKode) q.set("komoditas", komoditasKode);
  if (periode) q.set("periode", periode);
  const s = q.toString();
  return s ? `?${s}` : "";
}

export interface RingkasanNeraca {
  surplusTon: number;
  defisitTon: number;
  jumlahSurplus: number;
  jumlahDefisit: number;
}

/** Total surplus dan defisit (ton, defisit sebagai angka positif) untuk satu tampilan neraca. */
export function ringkasNeraca(items: NeracaWilayah[]): RingkasanNeraca {
  const r: RingkasanNeraca = {
    surplusTon: 0,
    defisitTon: 0,
    jumlahSurplus: 0,
    jumlahDefisit: 0,
  };
  for (const n of items) {
    if (n.surplusDefisitTon > 0) {
      r.surplusTon += n.surplusDefisitTon;
      r.jumlahSurplus++;
    } else if (n.surplusDefisitTon < 0) {
      r.defisitTon += -n.surplusDefisitTon;
      r.jumlahDefisit++;
    }
  }
  return r;
}

/** `GET /v1/spasial/lokasi`: wilayah untuk satu titik. */
export interface LokasiResponse {
  kode: string;
  nama: string;
  di_dalam: boolean;
  jarak_pusat_km: number;
}

export interface LokasiWilayah {
  kode: string;
  nama: string;
  /** Titik berada di dalam poligon wilayah (poligon demo adalah perkiraan). */
  diDalam: boolean;
  jarakPusatKm: number;
}

export function toLokasi(r: LokasiResponse): LokasiWilayah {
  return {
    kode: r.kode,
    nama: formatWilayah(r.nama),
    diDalam: r.di_dalam,
    jarakPusatKm: r.jarak_pusat_km,
  };
}

export function toLokasiQuery(lat: number, lng: number): string {
  return `?${new URLSearchParams({ lat: String(lat), lng: String(lng) }).toString()}`;
}

/** Keterangan posisi titik terhadap wilayah hasil pencarian. */
export function keteranganLokasi(l: LokasiWilayah): string {
  const km = l.jarakPusatKm.toLocaleString("id-ID", {
    maximumFractionDigits: 1,
    minimumFractionDigits: 1,
  });
  return l.diDalam
    ? `di dalam batas (perkiraan), ${km} km dari pusat`
    : `terdekat, ${km} km dari pusat`;
}

/** Pesan galat geolokasi peramban (GeolocationPositionError.code). */
export function pesanGalatGeolokasi(code: number): string {
  switch (code) {
    case 1:
      return "Izin lokasi ditolak. Izinkan akses lokasi di pengaturan peramban, lalu coba lagi.";
    case 2:
      return "Posisi perangkat tidak dapat ditentukan. Periksa sambungan atau GPS Anda.";
    case 3:
      return "Penentuan posisi melewati batas waktu. Coba lagi.";
    default:
      return "Gagal menentukan lokasi Anda.";
  }
}

/** Skema peta Aceh tanpa token Mapbox: proyeksi linear bujur/lintang ke persen kotak. */
export const BATAS_SKEMA = {
  minLng: 94.9,
  maxLng: 98.5,
  minLat: 1.9,
  maxLat: 6.1,
};

/** Kebalikan proyeksi skema: pecahan (0-1) posisi klik di kotak peta menjadi lintang/bujur. */
export function titikDariSkema(
  xPecahan: number,
  yPecahan: number,
): { lat: number; lng: number } {
  const b = BATAS_SKEMA;
  return {
    lng: b.minLng + xPecahan * (b.maxLng - b.minLng),
    lat: b.maxLat - yPecahan * (b.maxLat - b.minLat),
  };
}

/** `GET /v1/spasial/optimasi-rute`: ringkasan satu rencana optimasi yang tersimpan. */
export interface RiwayatOptimasiResponse {
  id: string;
  komoditas_kode: string;
  komoditas_nama: string;
  periode: string;
  total_volume_ton: number;
  total_biaya: number;
  belum_terpenuhi_ton: number;
  /** Nama pembuat; null bila akunnya sudah dihapus. */
  dibuat_oleh: string | null;
  dibuat_pada: string;
}

export interface RiwayatOptimasi {
  id: string;
  komoditasKode: string;
  komoditas: string;
  periode: string;
  totalVolumeTon: number;
  totalBiaya: number;
  belumTerpenuhiTon: number;
  dibuatOleh: string | null;
  dibuatPada: string;
}

export function toRiwayatOptimasi(r: RiwayatOptimasiResponse): RiwayatOptimasi {
  return {
    id: r.id,
    komoditasKode: r.komoditas_kode,
    komoditas: r.komoditas_nama,
    periode: r.periode,
    totalVolumeTon: r.total_volume_ton,
    totalBiaya: r.total_biaya,
    belumTerpenuhiTon: r.belum_terpenuhi_ton,
    dibuatOleh: r.dibuat_oleh,
    dibuatPada: r.dibuat_pada,
  };
}
