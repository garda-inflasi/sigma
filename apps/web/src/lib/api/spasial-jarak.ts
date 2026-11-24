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
  jenis?: "PASAR" | "COLD_STORAGE";
  kode: string;
  nama: string;
  kabupaten_kode: string;
  kabupaten_nama?: string;
  lat: number;
  lng: number;
  jarak_lurus_km: number;
  jarak_jalan_km: number;
  estimasi_menit_tempuh?: number;
  menit_tempuh?: number;
  kapasitas_ton?: number;
  status?: string;
}

export type JenisGangguanJalan =
  | "LONGSOR"
  | "BANJIR"
  | "JEMBATAN_PUTUS"
  | "JALAN_RUSAK"
  | "PENUTUPAN";

export type TingkatGangguanJalan = "TUTUP_TOTAL" | "TERHAMBAT";

export interface GangguanJalanItem {
  id: string;
  kode: string;
  judul: string;
  jenis: JenisGangguanJalan;
  tingkat: TingkatGangguanJalan;
  kabupaten_kode?: string;
  kabupaten_nama?: string;
  ruas_jalan: string;
  lat: number;
  lng: number;
  radius_m: number;
  keterangan: string;
  aktif: boolean;
  mulai?: string;
  perkiraan_selesai?: string;
}

export interface CreateGangguanInput {
  judul: string;
  jenis: JenisGangguanJalan;
  tingkat: TingkatGangguanJalan;
  ruas_jalan?: string;
  lat: number;
  lng: number;
  radius_m?: number;
  keterangan?: string;
}

export interface UpdateGangguanInput {
  judul?: string;
  jenis?: JenisGangguanJalan;
  tingkat?: TingkatGangguanJalan;
  ruas_jalan?: string;
  radius_m?: number;
  keterangan?: string;
  aktif?: boolean;
}

export interface GeoJSONLineString {
  type: "LineString";
  coordinates: [number, number][];
}

export interface OpsiRuteJalan {
  tipe:
    | "Jalur Utama"
    | "Jalur Pengalihan"
    | "Jalur Alternatif"
    | "Jalur Terdampak";
  ringkasan_jalan: string;
  jarak_km: number;
  durasi_menit: number;
  risiko_food_loss: "RENDAH" | "SEDANG" | "TINGGI";
  snap_asal_m?: number;
  snap_tujuan_m?: number;
  terdampak_gangguan: boolean;
  selisih_km?: number;
  selisih_menit?: number;
  geometry: GeoJSONLineString;
}

export interface JarakSpasialResult {
  asal_kode?: string;
  tujuan_kode?: string;
  asal_lat: number;
  asal_lng: number;
  tujuan_lat?: number;
  tujuan_lng?: number;
  faktor_jalan: number;
  jarak_lurus_km?: number;
  jarak_jalan_km?: number;
  jam_tempuh?: number;
  estimasi_menit_tempuh?: number;
  menit_tempuh?: number;
  risiko_food_loss?: "RENDAH" | "SEDANG" | "TINGGI";
  sumber_rute?: string;
  profil_kendaraan?: string;
  jalur_utama_diblokir?: boolean;
  rute?: OpsiRuteJalan[];
  rute_diblokir?: OpsiRuteJalan | null;
  gangguan_terkait?: GangguanJalanItem[];
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

let MOCK_GANGGUAN_STORE: GangguanJalanItem[] = [
  {
    id: "mock-ggj-1",
    kode: "GGJ-LONG-01",
    judul: "Longsor Ruas Takengon – Bireuen",
    jenis: "LONGSOR",
    tingkat: "TUTUP_TOTAL",
    kabupaten_kode: "1104",
    kabupaten_nama: "Aceh Tengah",
    ruas_jalan: "Jl. Bireuen – Takengon KM 42",
    lat: 4.918,
    lng: 96.762,
    radius_m: 600,
    keterangan: "Material tebing menutup badan jalan; dialihkan ke Lintas Tengah-Barat.",
    aktif: false,
  },
];

function buatJejakMock(
  lat1: number,
  lng1: number,
  lat2: number,
  lng2: number,
  offsetLat: number,
  offsetLng: number,
): [number, number][] {
  const m1Lat = lat1 + (lat2 - lat1) * 0.35 + offsetLat;
  const m1Lng = lng1 + (lng2 - lng1) * 0.35 + offsetLng;
  const m2Lat = lat1 + (lat2 - lat1) * 0.68 + offsetLat * 0.65;
  const m2Lng = lng1 + (lng2 - lng1) * 0.68 + offsetLng * 0.65;
  return [
    [lng1, lat1],
    [m1Lng, m1Lat],
    [m2Lng, m2Lat],
    [lng2, lat2],
  ];
}

function titikDekatSegmenMock(
  pLat: number,
  pLng: number,
  coords: [number, number][],
): number {
  let minKm = Infinity;
  for (let i = 0; i < coords.length - 1; i++) {
    const [aLng, aLat] = coords[i];
    const [bLng, bLat] = coords[i + 1];
    const d1 = haversineKm(pLat, pLng, aLat, aLng);
    const d2 = haversineKm(pLat, pLng, bLat, bLng);
    const dm = haversineKm(pLat, pLng, (aLat + bLat) / 2, (aLng + bLng) / 2);
    minKm = Math.min(minKm, d1, d2, dm);
  }
  return minKm * 1000;
}

export async function hitungJarakSpasial(params: {
  asalKode?: string;
  tujuanKode?: string;
  asalLat: number;
  asalLng: number;
  tujuanLat?: number;
  tujuanLng?: number;
  faktorJalan?: number;
  profil?: "truck" | "auto";
}): Promise<JarakSpasialResult> {
  const faktor = params.faktorJalan ?? 1.28;
  const profil = params.profil ?? "truck";
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
    const dLat = hasDest ? params.tujuanLat! - params.asalLat : 0;
    const dLng = hasDest ? params.tujuanLng! - params.asalLng : 0;
    const span = Math.max(Math.hypot(dLat, dLng), 0.04);

    let rute: OpsiRuteJalan[] | undefined;
    let ruteDiblokir: OpsiRuteJalan | null = null;
    let diblokir = false;
    let gangguanTerkait: GangguanJalanItem[] = [];

    if (hasDest && jalan !== undefined && menit !== undefined) {
      const utamaCoords = buatJejakMock(
        params.asalLat,
        params.asalLng,
        params.tujuanLat!,
        params.tujuanLng!,
        -dLng * 0.08,
        dLat * 0.08,
      );
      const altCoords = buatJejakMock(
        params.asalLat,
        params.asalLng,
        params.tujuanLat!,
        params.tujuanLng!,
        dLng * 0.14 + span * 0.04,
        -dLat * 0.14 - span * 0.04,
      );
      const altKm = Math.round(jalan * 1.16 * 10) / 10;
      const altMenit = Math.max(menit + 8, Math.round(menit * 1.18));

      const utamaOpt: OpsiRuteJalan = {
        tipe: "Jalur Utama",
        ringkasan_jalan: "Jalan Lintas Utama Aceh",
        jarak_km: jalan,
        durasi_menit: menit,
        risiko_food_loss: risikoFoodLossDariMenit(menit),
        terdampak_gangguan: false,
        geometry: { type: "LineString", coordinates: utamaCoords },
      };
      const altOpt: OpsiRuteJalan = {
        tipe: "Jalur Alternatif",
        ringkasan_jalan: "Koridor Penghubung Kabupaten",
        jarak_km: altKm,
        durasi_menit: altMenit,
        risiko_food_loss: risikoFoodLossDariMenit(altMenit),
        terdampak_gangguan: false,
        geometry: { type: "LineString", coordinates: altCoords },
      };

      const aktif = MOCK_GANGGUAN_STORE.filter((g) => g.aktif);
      const kenaUtama = aktif.filter(
        (g) => titikDekatSegmenMock(g.lat, g.lng, utamaCoords) <= Math.max(g.radius_m, 1500),
      );
      const tutupTotal = kenaUtama.filter((g) => g.tingkat === "TUTUP_TOTAL");
      if (tutupTotal.length > 0) {
        diblokir = true;
        gangguanTerkait = kenaUtama;
        ruteDiblokir = { ...utamaOpt, tipe: "Jalur Terdampak", terdampak_gangguan: true };
        rute = [
          {
            ...altOpt,
            tipe: "Jalur Pengalihan",
            selisih_km: Math.round((altKm - jalan) * 10) / 10,
            selisih_menit: altMenit - menit,
          },
        ];
      } else {
        if (kenaUtama.length > 0) {
          utamaOpt.terdampak_gangguan = true;
          gangguanTerkait = kenaUtama;
        }
        rute = [utamaOpt, altOpt];
      }
    }

    const utama = rute?.[0];
    return mockDelay(
      {
        asal_kode: params.asalKode,
        tujuan_kode: params.tujuanKode,
        asal_lat: params.asalLat,
        asal_lng: params.asalLng,
        tujuan_lat: params.tujuanLat,
        tujuan_lng: params.tujuanLng,
        faktor_jalan: faktor,
        jarak_lurus_km: lurus,
        jarak_jalan_km: utama?.jarak_km ?? jalan,
        jam_tempuh:
          (utama?.durasi_menit ?? menit) !== undefined
            ? Math.round(((utama?.durasi_menit ?? menit)! / 60) * 10) / 10
            : undefined,
        estimasi_menit_tempuh: utama?.durasi_menit ?? menit,
        menit_tempuh: utama?.durasi_menit ?? menit,
        risiko_food_loss:
          utama?.risiko_food_loss ??
          (menit !== undefined ? risikoFoodLossDariMenit(menit) : undefined),
        sumber_rute: "SIMULASI_JARINGAN_JALAN",
        profil_kendaraan: profil,
        jalur_utama_diblokir: diblokir,
        rute,
        rute_diblokir: ruteDiblokir,
        gangguan_terkait: gangguanTerkait,
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
    profil,
  });
  if (params.asalKode) q.set("asal_kode", params.asalKode);
  if (params.tujuanKode) q.set("tujuan_kode", params.tujuanKode);
  if (typeof params.tujuanLat === "number" && typeof params.tujuanLng === "number") {
    q.set("tujuan_lat", String(params.tujuanLat));
    q.set("tujuan_lng", String(params.tujuanLng));
  }
  return apiFetch<JarakSpasialResult>(`/v1/spasial/jarak?${q.toString()}`);
}

export async function listGangguanJalan(
  aktif?: boolean,
): Promise<GangguanJalanItem[]> {
  if (MODE_MOCK) {
    const filtered =
      aktif === undefined
        ? MOCK_GANGGUAN_STORE
        : MOCK_GANGGUAN_STORE.filter((g) => g.aktif === aktif);
    return mockDelay([...filtered], 40, 90);
  }
  const qs = aktif === undefined ? "" : `?aktif=${String(aktif)}`;
  return apiFetch<GangguanJalanItem[]>(`/v1/spasial/gangguan-jalan${qs}`);
}

export async function createGangguanJalan(
  payload: CreateGangguanInput,
): Promise<{ id: string; kode: string }> {
  if (MODE_MOCK) {
    const item: GangguanJalanItem = {
      id: `mock-ggj-${Date.now()}`,
      kode: `GGJ-${Math.floor(1000 + Math.random() * 9000)}`,
      judul: payload.judul,
      jenis: payload.jenis,
      tingkat: payload.tingkat,
      ruas_jalan: payload.ruas_jalan ?? "",
      lat: payload.lat,
      lng: payload.lng,
      radius_m: payload.radius_m ?? 450,
      keterangan: payload.keterangan ?? "",
      aktif: true,
    };
    MOCK_GANGGUAN_STORE = [item, ...MOCK_GANGGUAN_STORE];
    return mockDelay({ id: item.id, kode: item.kode }, 50, 100);
  }
  return apiFetch<{ id: string; kode: string }>("/v1/spasial/gangguan-jalan", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export async function updateGangguanJalan(
  id: string,
  payload: UpdateGangguanInput,
): Promise<{ id: string; updated: boolean }> {
  if (MODE_MOCK) {
    MOCK_GANGGUAN_STORE = MOCK_GANGGUAN_STORE.map((g) =>
      g.id === id ? { ...g, ...payload } : g,
    );
    return mockDelay({ id, updated: true }, 40, 80);
  }
  return apiFetch<{ id: string; updated: boolean }>(
    `/v1/spasial/gangguan-jalan/${encodeURIComponent(id)}`,
    {
      method: "PATCH",
      body: JSON.stringify(payload),
    },
  );
}

export async function deleteGangguanJalan(
  id: string,
): Promise<{ id: string; deleted: boolean }> {
  if (MODE_MOCK) {
    MOCK_GANGGUAN_STORE = MOCK_GANGGUAN_STORE.filter((g) => g.id !== id);
    return mockDelay({ id, deleted: true }, 40, 80);
  }
  return apiFetch<{ id: string; deleted: boolean }>(
    `/v1/spasial/gangguan-jalan/${encodeURIComponent(id)}`,
    {
      method: "DELETE",
    },
  );
}
