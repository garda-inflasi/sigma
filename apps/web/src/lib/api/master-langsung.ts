import { MODE_MOCK, apiFetch, apiFetchHalaman, mockDelay } from "./client";
import { MOCK_TITIK_DESA, type JenisTitikDesa } from "./spasial-jarak";

export interface LokasiSpasialRow {
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
  aktif: boolean;
  versi: number;
}

export interface LokasiSpasialParams {
  jenis?: JenisTitikDesa;
  kabupatenKode?: string;
  q?: string;
  page?: number;
  limit?: number;
}

let mockLokasiStore: LokasiSpasialRow[] = MOCK_TITIK_DESA.map((t) => ({
  ...t,
  aktif: true,
  versi: 1,
}));

export async function listLokasiSpasial(
  params: LokasiSpasialParams = {},
): Promise<{ data: LokasiSpasialRow[]; total: number | null }> {
  if (MODE_MOCK) {
    const q = (params.q ?? "").trim().toLowerCase();
    const filtered = mockLokasiStore.filter(
      (r) =>
        (!params.jenis || r.jenis === params.jenis) &&
        (!params.kabupatenKode || r.kabupaten_kode === params.kabupatenKode) &&
        (!q ||
          r.kode.toLowerCase().includes(q) ||
          r.nama.toLowerCase().includes(q) ||
          r.komoditas_utama.toLowerCase().includes(q)),
    );
    return mockDelay({ data: filtered, total: filtered.length }, 80, 140);
  }
  const sp = new URLSearchParams();
  if (params.jenis) sp.set("jenis", params.jenis);
  if (params.kabupatenKode) sp.set("kabupaten_kode", params.kabupatenKode);
  if (params.q) sp.set("q", params.q);
  if (params.page) sp.set("page", String(params.page));
  if (params.limit) sp.set("limit", String(params.limit));
  const qs = sp.toString();
  return apiFetchHalaman<LokasiSpasialRow>(
    `/v1/master/lokasi-spasial${qs ? `?${qs}` : ""}`,
  );
}

export type EntitasLangsung =
  | "komoditas"
  | "pasar"
  | "kecamatan"
  | "satuan"
  | "pedagang"
  | "pertanian"
  | "perkampungan";

export async function simpanMasterLangsung(
  entitas: EntitasLangsung,
  payload: Record<string, unknown>,
  kodeEdit?: string,
): Promise<{ kode: string }> {
  if (MODE_MOCK) {
    if (entitas === "pertanian" || entitas === "perkampungan") {
      const jenis = entitas === "pertanian" ? "PERTANIAN" : "PERKAMPUNGAN";
      const row: LokasiSpasialRow = {
        id: String(payload.kode ?? kodeEdit ?? Date.now()),
        kode: String(kodeEdit ?? payload.kode),
        jenis,
        nama: String(payload.nama ?? ""),
        kabupaten_kode: String(payload.kabupaten_kode ?? "1171"),
        kabupaten_nama: "Aceh",
        komoditas_utama: String(payload.komoditas_utama ?? ""),
        luas_ha: payload.luas_ha ? Number(payload.luas_ha) : null,
        estimasi_panen_ton: payload.estimasi_panen_ton
          ? Number(payload.estimasi_panen_ton)
          : null,
        populasi: payload.populasi ? Number(payload.populasi) : null,
        lat: Number(payload.lat ?? 5.55),
        lng: Number(payload.lng ?? 95.32),
        aktif: payload.aktif !== false,
        versi: 1,
      };
      const idx = mockLokasiStore.findIndex((r) => r.kode === row.kode);
      if (idx >= 0) {
        mockLokasiStore[idx] = { ...row, versi: mockLokasiStore[idx].versi + 1 };
      } else {
        mockLokasiStore = [row, ...mockLokasiStore];
      }
    }
    return mockDelay({ kode: String(kodeEdit ?? payload.kode ?? "") }, 100, 180);
  }

  if (kodeEdit) {
    return apiFetch<{ kode: string }>(
      `/v1/master/langsung/${entitas}/${encodeURIComponent(kodeEdit)}`,
      {
        method: "PUT",
        body: JSON.stringify(payload),
      },
    );
  }
  return apiFetch<{ kode: string }>(`/v1/master/langsung/${entitas}`, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export async function hapusMasterLangsung(
  entitas: EntitasLangsung,
  kode: string,
): Promise<void> {
  if (MODE_MOCK) {
    mockLokasiStore = mockLokasiStore.filter((r) => r.kode !== kode);
    return mockDelay(undefined, 80, 140);
  }
  await apiFetch<void>(
    `/v1/master/langsung/${entitas}/${encodeURIComponent(kode)}`,
    { method: "DELETE" },
  );
}
