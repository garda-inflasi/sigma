import { MODE_MOCK, apiFetch, mockDelay } from "./client";
import {
  estimasiJalanKm,
  estimasiMenitTempuh,
  haversineKm,
  risikoFoodLossDariMenit,
} from "@/lib/peta/aceh";

export type JenisTitikDesa = "PERTANIAN" | "PERKAMPUNGAN";

export interface TitikDesa {
  id: string;
  kode: string;
  jenis: JenisTitikDesa;
  nama: string;
  kabupaten_kode: string;
  kabupaten_nama: string;
  kecamatan_kode?: string | null;
  komoditas_utama: string;
  luas_ha?: number | null;
  estimasi_panen_ton?: number | null;
  populasi?: number | null;
  lat: number;
  lng: number;
}

export interface FasilitasTerdekat {
  kode: string;
  nama: string;
  kabupaten_kode: string;
  kabupaten_nama: string;
  lat: number;
  lng: number;
  jarak_lurus_km: number;
  jarak_jalan_km: number;
  menit_tempuh: number;
  status?: string;
}

export interface JarakSpasialResult {
  asal_lat: number;
  asal_lng: number;
  tujuan_lat?: number;
  tujuan_lng?: number;
  faktor_jalan: number;
  jarak_lurus_km?: number;
  jarak_jalan_km?: number;
  jam_tempuh?: number;
  menit_tempuh?: number;
  risiko_food_loss?: "RENDAH" | "SEDANG" | "TINGGI";
  pasar_terdekat: FasilitasTerdekat[];
  cold_storage_terdekat: FasilitasTerdekat[];
}

export const MOCK_TITIK_DESA: TitikDesa[] = [
  {
    id: "stn-1106-01",
    kode: "STN-1106-01",
    jenis: "PERTANIAN",
    nama: "Sentra Hortikultura Pegasing",
    kabupaten_kode: "1106",
    kabupaten_nama: "Kab. Aceh Tengah",
    komoditas_utama: "Cabai merah keriting & Tomat",
    luas_ha: 420.5,
    estimasi_panen_ton: 680,
    lat: 4.5975,
    lng: 96.8142,
  },
  {
    id: "stn-1117-01",
    kode: "STN-1117-01",
    jenis: "PERTANIAN",
    nama: "Sentra Sayuran Bukit Redelong",
    kabupaten_kode: "1117",
    kabupaten_nama: "Kab. Bener Meriah",
    komoditas_utama: "Cabai merah & Bawang merah",
    luas_ha: 365,
    estimasi_panen_ton: 540,
    lat: 4.7495,
    lng: 96.885,
  },
  {
    id: "stn-1109-01",
    kode: "STN-1109-01",
    jenis: "PERTANIAN",
    nama: "Sentra Cabai & Padi Indrajaya",
    kabupaten_kode: "1109",
    kabupaten_nama: "Kab. Pidie",
    komoditas_utama: "Beras medium & Cabai rawit",
    luas_ha: 890,
    estimasi_panen_ton: 1450,
    lat: 5.3312,
    lng: 95.9218,
  },
  {
    id: "stn-1108-01",
    kode: "STN-1108-01",
    jenis: "PERTANIAN",
    nama: "Sentra Padi Seulimeum",
    kabupaten_kode: "1108",
    kabupaten_nama: "Kab. Aceh Besar",
    komoditas_utama: "Beras premium & Bawang merah",
    luas_ha: 760,
    estimasi_panen_ton: 1180,
    lat: 5.4428,
    lng: 95.5914,
  },
  {
    id: "kmp-1171-01",
    kode: "KMP-1171-01",
    jenis: "PERKAMPUNGAN",
    nama: "Gampong Lambaro Skep",
    kabupaten_kode: "1171",
    kabupaten_nama: "Kota Banda Aceh",
    komoditas_utama: "Konsumsi Rumah Tangga",
    populasi: 6420,
    lat: 5.5681,
    lng: 95.3372,
  },
  {
    id: "kmp-1106-01",
    kode: "KMP-1106-01",
    jenis: "PERKAMPUNGAN",
    nama: "Kampung Asir-Asir Lut Tawar",
    kabupaten_kode: "1106",
    kabupaten_nama: "Kab. Aceh Tengah",
    komoditas_utama: "Petani Hortikultura",
    populasi: 3920,
    lat: 4.6215,
    lng: 96.854,
  },
];

export async function listTitikDesa(
  jenis?: JenisTitikDesa,
  kabupatenKode?: string,
): Promise<TitikDesa[]> {
  if (MODE_MOCK) {
    return mockDelay(
      MOCK_TITIK_DESA.filter(
        (x) =>
          (!jenis || x.jenis === jenis) &&
          (!kabupatenKode || x.kabupaten_kode === kabupatenKode),
      ),
      100,
      180,
    );
  }
  const q = new URLSearchParams();
  if (jenis) q.set("jenis", jenis);
  if (kabupatenKode) q.set("kabupaten_kode", kabupatenKode);
  const qs = q.toString();
  return apiFetch<TitikDesa[]>(`/v1/spasial/titik-desa${qs ? `?${qs}` : ""}`);
}

export async function hitungJarakSpasial(params: {
  asalLat: number;
  asalLng: number;
  tujuanLat?: number;
  tujuanLng?: number;
  faktorJalan?: number;
}): Promise<JarakSpasialResult> {
  const faktor = params.faktorJalan ?? 1.28;
  if (MODE_MOCK) {
    const hasDest =
      typeof params.tujuanLat === "number" && typeof params.tujuanLng === "number";
    const lurus = hasDest
      ? Math.round(
          haversineKm(
            params.asalLat,
            params.asalLng,
            params.tujuanLat!,
            params.tujuanLng!,
          ) * 10,
        ) / 10
      : undefined;
    const jalan = lurus !== undefined ? estimasiJalanKm(lurus, faktor) : undefined;
    const menit = jalan !== undefined ? estimasiMenitTempuh(jalan) : undefined;
    return mockDelay(
      {
        asal_lat: params.asalLat,
        asal_lng: params.asalLng,
        tujuan_lat: params.tujuanLat,
        tujuan_lng: params.tujuanLng,
        faktor_jalan: faktor,
        jarak_lurus_km: lurus,
        jarak_jalan_km: jalan,
        jam_tempuh: menit !== undefined ? Math.round((menit / 60) * 10) / 10 : undefined,
        menit_tempuh: menit,
        risiko_food_loss: menit !== undefined ? risikoFoodLossDariMenit(menit) : undefined,
        pasar_terdekat: [],
        cold_storage_terdekat: [],
      },
      60,
      140,
    );
  }
  const q = new URLSearchParams({
    asal_lat: String(params.asalLat),
    asal_lng: String(params.asalLng),
    faktor_jalan: String(faktor),
  });
  if (typeof params.tujuanLat === "number" && typeof params.tujuanLng === "number") {
    q.set("tujuan_lat", String(params.tujuanLat));
    q.set("tujuan_lng", String(params.tujuanLng));
  }
  return apiFetch<JarakSpasialResult>(`/v1/spasial/jarak?${q.toString()}`);
}
