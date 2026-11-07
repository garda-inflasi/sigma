/**
 * Klien master data: detail, hapus, kecamatan/satuan/pedagang, dan alur usulan perubahan (04-B/04-C).
 * Mode mock memakai backend palsu di memori (master-mock.ts) dengan bentuk respons yang sama.
 */

import { apiFetch, apiFetchHalaman, mockDelay, MODE_MOCK } from "./client";
import {
  toHalaman,
  toKecamatanQuery,
  toKecamatanRow,
  toKomoditasDetail,
  toPasarDetail,
  toPedagangQuery,
  toPedagangRow,
  toSatuanRow,
  toWilayahDetail,
  type Halaman,
  type KecamatanParams,
  type KecamatanResponse,
  type KecamatanRow,
  type KomoditasDetail,
  type KomoditasDetailResponse,
  type PasarDetail,
  type PasarDetailResponse,
  type PedagangParams,
  type PedagangResponse,
  type PedagangRow,
  type SatuanResponse,
  type SatuanRow,
  type WilayahDetail,
  type WilayahDetailResponse,
} from "./master";
import * as mock from "./master-mock";
import {
  toPerubahanMaster,
  toPerubahanQuery,
  type EntitasPerubahan,
  type PerubahanMaster,
  type PerubahanMasterResponse,
  type PerubahanParams,
  type UsulanBody,
} from "./master-perubahan";

const BASE = "/v1/master";
const enc = encodeURIComponent;
/** Ukuran halaman saat seluruh baris diambil bertahap (batas server 500). */
const HALAMAN_SEMUA = 100;
/** Pengaman: paling banyak halaman yang diambil bertahap. */
const MAKS_HALAMAN = 100;

/** Pelaku aksi untuk mode mock; di mode nyata server membacanya dari token. */
export type AktorMock = { id: string; nama: string } | null | undefined;
const aktorMock = (a: AktorMock) =>
  a ?? { id: "pengguna-mock", nama: "Pengguna mock" };

const cepat = [150, 350] as const;

/** Mengambil semua halaman satu daftar (mis. untuk ekspor CSV atau pilihan kecamatan satu kabupaten). */
export async function ambilSemuaHalaman<T>(
  ambil: (page: number, limit: number) => Promise<Halaman<T>>,
): Promise<T[]> {
  const semua: T[] = [];
  for (let page = 1; page <= MAKS_HALAMAN; page++) {
    const h = await ambil(page, HALAMAN_SEMUA);
    semua.push(...h.rows);
    if (h.rows.length === 0 || semua.length >= h.total) break;
  }
  return semua;
}

const halamanMock = <R, T>(
  hasil: { rows: R[]; total: number },
  petakan: (r: R) => T,
): Halaman<T> => ({ rows: hasil.rows.map(petakan), total: hasil.total });

export const masterApi = {
  komoditasDetail: (kode: string): Promise<KomoditasDetail> =>
    MODE_MOCK
      ? mockDelay(
          () => toKomoditasDetail(mock.mockKomoditasDetail(kode)),
          ...cepat,
        )
      : apiFetch<KomoditasDetailResponse>(
          `${BASE}/komoditas/${enc(kode)}`,
        ).then(toKomoditasDetail),
  wilayahDetail: (kode: string): Promise<WilayahDetail> =>
    MODE_MOCK
      ? mockDelay(() => toWilayahDetail(mock.mockWilayahDetail(kode)), ...cepat)
      : apiFetch<WilayahDetailResponse>(`${BASE}/wilayah/${enc(kode)}`).then(
          toWilayahDetail,
        ),
  pasarDetail: (kode: string): Promise<PasarDetail> =>
    MODE_MOCK
      ? mockDelay(() => toPasarDetail(mock.mockPasarDetail(kode)), ...cepat)
      : apiFetch<PasarDetailResponse>(`${BASE}/pasar/${enc(kode)}`).then(
          toPasarDetail,
        ),

  /** Menghapus komoditas/pasar (izin master.setujui). 409 MASTER_DIRUJUK / MASTER_ADA_USULAN, 404 bila tidak ada. */
  hapus: (entitas: EntitasPerubahan, kode: string): Promise<void> =>
    MODE_MOCK
      ? mockDelay(() => mock.mockHapus(entitas, kode), ...cepat)
      : apiFetch<void>(`${BASE}/${entitas}/${enc(kode)}`, { method: "DELETE" }),

  /** Satu halaman kecamatan; pencarian dan paginasi dijalankan server, total dari X-Total-Count. */
  kecamatan: async (
    p: KecamatanParams = {},
  ): Promise<Halaman<KecamatanRow>> => {
    const page = p.page ?? 1;
    const limit = p.limit ?? 25;
    if (MODE_MOCK)
      return mockDelay(
        () =>
          halamanMock(
            mock.mockKecamatanList({ ...p, page, limit }),
            toKecamatanRow,
          ),
        ...cepat,
      );
    const { data, total } = await apiFetchHalaman<KecamatanResponse>(
      `${BASE}/kecamatan${toKecamatanQuery({ ...p, page, limit })}`,
    );
    return toHalaman(data, total, page, limit, toKecamatanRow);
  },
  /** Seluruh kecamatan satu kabupaten/kota (pilihan di form pasar); diambil bertahap. */
  kecamatanKabupaten: (kabupatenKode: string): Promise<KecamatanRow[]> =>
    ambilSemuaHalaman((page, limit) =>
      masterApi.kecamatan({ kabupatenKode, page, limit }),
    ),
  kecamatanDetail: (kode: string): Promise<KecamatanRow> =>
    MODE_MOCK
      ? mockDelay(
          () => toKecamatanRow(mock.mockKecamatanDetail(kode)),
          ...cepat,
        )
      : apiFetch<KecamatanResponse>(`${BASE}/kecamatan/${enc(kode)}`).then(
          toKecamatanRow,
        ),

  satuan: (): Promise<SatuanRow[]> =>
    MODE_MOCK
      ? mockDelay(() => mock.mockSatuanList().map(toSatuanRow), ...cepat)
      : apiFetch<SatuanResponse[]>(`${BASE}/satuan`).then((rows) =>
          rows.map(toSatuanRow),
        ),

  pedagang: async (p: PedagangParams = {}): Promise<Halaman<PedagangRow>> => {
    const page = p.page ?? 1;
    const limit = p.limit ?? 25;
    if (MODE_MOCK)
      return mockDelay(
        () =>
          halamanMock(
            mock.mockPedagangList({ ...p, page, limit }),
            toPedagangRow,
          ),
        ...cepat,
      );
    const { data, total } = await apiFetchHalaman<PedagangResponse>(
      `${BASE}/pedagang${toPedagangQuery({ ...p, page, limit })}`,
    );
    return toHalaman(data, total, page, limit, toPedagangRow);
  },

  /** Antrean/riwayat usulan: MENUNGGU terlama dahulu, selain itu terbaru dahulu. */
  perubahan: async (
    p: PerubahanParams = {},
  ): Promise<Halaman<PerubahanMaster>> => {
    const page = p.page ?? 1;
    const limit = p.limit ?? 25;
    if (MODE_MOCK)
      return mockDelay(
        () =>
          halamanMock(
            mock.mockPerubahanList({ ...p, page, limit }),
            toPerubahanMaster,
          ),
        ...cepat,
      );
    const { data, total } = await apiFetchHalaman<PerubahanMasterResponse>(
      `${BASE}/perubahan${toPerubahanQuery({ ...p, page, limit })}`,
    );
    return toHalaman(data, total, page, limit, toPerubahanMaster);
  },
  perubahanDetail: (id: string): Promise<PerubahanMaster> =>
    MODE_MOCK
      ? mockDelay(
          () => toPerubahanMaster(mock.mockPerubahanDetail(id)),
          ...cepat,
        )
      : apiFetch<PerubahanMasterResponse>(`${BASE}/perubahan/${enc(id)}`).then(
          toPerubahanMaster,
        ),
  /** Jumlah usulan yang menunggu (untuk lencana navigasi); hanya meminta satu baris. */
  jumlahMenunggu: async (): Promise<number> =>
    (await masterApi.perubahan({ status: "MENUNGGU", page: 1, limit: 1 }))
      .total,

  ajukan: (body: UsulanBody, aktor?: AktorMock): Promise<PerubahanMaster> =>
    MODE_MOCK
      ? mockDelay(
          () => toPerubahanMaster(mock.mockAjukan(body, aktorMock(aktor))),
          ...cepat,
        )
      : apiFetch<PerubahanMasterResponse>(`${BASE}/perubahan`, {
          method: "POST",
          body: JSON.stringify(body),
        }).then(toPerubahanMaster),
  setujui: (id: string, aktor?: AktorMock): Promise<PerubahanMaster> =>
    MODE_MOCK
      ? mockDelay(
          () => toPerubahanMaster(mock.mockSetujui(id, aktorMock(aktor))),
          ...cepat,
        )
      : apiFetch<PerubahanMasterResponse>(
          `${BASE}/perubahan/${enc(id)}/setujui`,
          { method: "POST" },
        ).then(toPerubahanMaster),
  tolak: (
    id: string,
    alasan: string,
    aktor?: AktorMock,
  ): Promise<PerubahanMaster> =>
    MODE_MOCK
      ? mockDelay(
          () => toPerubahanMaster(mock.mockTolak(id, alasan, aktorMock(aktor))),
          ...cepat,
        )
      : apiFetch<PerubahanMasterResponse>(
          `${BASE}/perubahan/${enc(id)}/tolak`,
          { method: "POST", body: JSON.stringify({ alasan }) },
        ).then(toPerubahanMaster),
  batalkan: (
    id: string,
    alasan?: string,
    aktor?: AktorMock,
  ): Promise<PerubahanMaster> =>
    MODE_MOCK
      ? mockDelay(
          () =>
            toPerubahanMaster(mock.mockBatalkan(id, alasan, aktorMock(aktor))),
          ...cepat,
        )
      : apiFetch<PerubahanMasterResponse>(
          `${BASE}/perubahan/${enc(id)}/batalkan`,
          { method: "POST", body: JSON.stringify(alasan ? { alasan } : {}) },
        ).then(toPerubahanMaster),
};
