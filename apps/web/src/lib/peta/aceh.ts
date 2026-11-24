/**
 * Konstanta dan helper geografis Provinsi Aceh untuk Leaflet. Leaflet memakai urutan `[lat, lng]`, sedangkan GeoJSON
 * memakai `[lng, lat]`: konversi hanya lewat `lngLatKeLatLng` atau `L.geoJSON` supaya sumbu tidak pernah tertukar.
 */

export type StatusWilayah = "defisit" | "waspada" | "surplus" | "normal";

export interface MarkerWilayah {
  kode: string;
  nama: string;
  lat: number;
  lng: number;
  delta: number;
  komoditas: string;
  status: StatusWilayah;
  punyaPasar: boolean;
}

/** Titik tambahan di peta selain pusat kabupaten/kota. */
export interface TitikPeta {
  id: string;
  jenis: "pasar" | "cold-storage";
  nama: string;
  keterangan?: string;
  lat: number;
  lng: number;
}

export type LatLng = [lat: number, lng: number];
export type BatasLatLng = [baratDaya: LatLng, timurLaut: LatLng];

/** Kotak terkecil yang memuat 23 kabupaten/kota (hasil batas BPS 2020): Simeulue/Singkil sampai Sabang. */
export const BATAS_ACEH: BatasLatLng = [
  [2.004, 95.011],
  [6.077, 98.287],
];

/** Batas geser peta: cukup longgar agar seluruh Aceh tetap bisa ditengahkan di layar lebar maupun sempit. */
export const BATAS_GESER: BatasLatLng = [
  [0.9, 93.4],
  [7.2, 99.9],
];

export const TILE_PETA = {
  url: "https://tile.openstreetmap.org/{z}/{x}/{y}.png",
  atribusi:
    '&copy; <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noreferrer">OpenStreetMap</a> · Batas: BPS 2020 via <a href="https://www.geoboundaries.org" target="_blank" rel="noreferrer">geoBoundaries</a> (CC BY 3.0 IGO)',
} as const;

export const URL_GEOJSON_ACEH = "/geojson/aceh-kabupaten.geojson";

/** Label, arti dalam bahasa sehari-hari, dan warna tiap status harga wilayah. */
export const META_STATUS: Record<
  StatusWilayah,
  { label: string; arti: string; warna: string; urutan: number }
> = {
  defisit: {
    label: "Defisit",
    arti: "Harga naik tajam karena pasokan kurang. Perlu tindakan segera.",
    warna: "#ef4444",
    urutan: 0,
  },
  waspada: {
    label: "Waspada",
    arti: "Harga mulai naik. Pantau dan siapkan langkah antisipasi.",
    warna: "#f59e0b",
    urutan: 1,
  },
  normal: {
    label: "Normal",
    arti: "Harga stabil, tidak perlu tindakan.",
    warna: "#10b981",
    urutan: 2,
  },
  surplus: {
    label: "Surplus",
    arti: "Pasokan berlebih dan harga turun. Bisa menjadi sumber distribusi.",
    warna: "#3b82f6",
    urutan: 3,
  },
};

export const lngLatKeLatLng = ([lng, lat]: readonly [
  number,
  number,
]): LatLng => [lat, lng];

/** Titik berada di dalam kotak Provinsi Aceh. */
export function diDalamBatasAceh(lat: number, lng: number): boolean {
  const [[minLat, minLng], [maxLat, maxLng]] = BATAS_ACEH;
  return lat >= minLat && lat <= maxLat && lng >= minLng && lng <= maxLng;
}

/** Perubahan harga bertanda untuk kartu peta, mis. "+12,4%". */
export function formatDelta(delta: number): string {
  const nilai = Math.abs(delta).toLocaleString("id-ID", {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  });
  return `${delta > 0 ? "+" : delta < 0 ? "−" : ""}${nilai}%`;
}
