import { apiFetch, mockDelay, MODE_MOCK } from "./client";
import { toAuditPage, toAuditQuery, queryMockAudit, type AuditFilter, type AuditPage, type AuditPageResponse } from "./audit";
import { coldStorageClient, queryMockColdStorage, type BulkColdStorageResult } from "./cold-storage";
import { COLD_STORAGE_MAX_LIMIT, type ColdStorageBulkAction, type ColdStorageParams } from "./cold-storage-query";
import {
  toMasterKomoditasRow, toMasterPasarRow, toMasterWilayahRow,
  type MasterKomoditasResponse, type MasterPasarResponse, type MasterWilayahResponse,
} from "./master";
import { toRingkasanEksekutif, type RingkasanEksekutifResponse } from "./eksekutif";
import { penggunaApi, type KatalogAkses } from "./pengguna";
import { toSumberData, type SumberBulkAksi, type SumberDataResponse } from "./sumber";
import * as m from "./mock";
import type {
  ColdStorageView, EntriHarga, EwsAlert, KartuIntervensi, KpiOperasional, MasterKomoditasRow, MasterPasarRow, MasterWilayahRow,
  PenggunaRow, PolicyBrief, ProyeksiXai, RingkasanEksekutif, SelHeatmap, StokColdStorage, SumberData, TugasLapangan,
} from "@/lib/data/types";

export type BulkResult = { berhasil: number; gagal: number; pesan?: string };
type Heatmap = { kabupaten: { kode: string; nama: string }[]; komoditas: { kode: string; nama: string; pilot: boolean }[]; sel: SelHeatmap[] };

const get = <T,>(path: string, mock: () => T) => (MODE_MOCK ? mockDelay(mock) : apiFetch<T>(path));

/** Aksi massal generik: POST /v1/{sumber}/bulk { aksi, ids } */
export function bulk(sumber: string, aksi: string, ids: string[]): Promise<BulkResult> {
  if (MODE_MOCK) return mockDelay({ berhasil: ids.length, gagal: 0 }, 500, 1100);
  return apiFetch<BulkResult>(`/v1/${sumber}/bulk`, { method: "POST", body: JSON.stringify({ aksi, ids }), idempotencyKey: crypto.randomUUID() });
}

/** Satu halaman cold storage: BFF bila backend aktif, data mock bila tidak. */
const listColdStorage = (params: ColdStorageParams = {}): Promise<ColdStorageView> =>
  MODE_MOCK ? mockDelay(() => queryMockColdStorage(m.mockColdStorage(), params)) : coldStorageClient.list(params);

export const api = {
  /** Siklus ekonomi dominan wilayah — menggerakkan warna ambient seluruh UI */
  siklus: { aktif: () => get<{ siklus: "normal" | "tekanan" | "surplus"; sejak: string }>("/v1/siklus/aktif", () => ({ siklus: "tekanan", sejak: "2026-09-27" })) },
  operasional: {
    kpi: () => get<KpiOperasional[]>("/v1/dashboard/operasional/kpi", m.mockKpi),
    heatmap: () => get<Heatmap>("/v1/dashboard/operasional/heatmap", m.mockHeatmap),
    marker: () => get<ReturnType<typeof m.mockMapMarkers>>("/v1/spasial/wilayah", m.mockMapMarkers),
    tren: (kode: string, kab: string) =>
      get<ReturnType<typeof m.mockTrenHarga>>(`/v1/harga/tren?komoditas=${kode}&wilayah=${kab}`, () => m.mockTrenHarga(kode, kab)),
  },
  eksekutif: {
    ringkasan: (): Promise<RingkasanEksekutif> =>
      MODE_MOCK ? mockDelay(m.mockEksekutif) : apiFetch<RingkasanEksekutifResponse>("/v1/dashboard/eksekutif").then(toRingkasanEksekutif),
    xai: (kode: string) => get<ProyeksiXai>(`/v1/prediksi/${kode}`, () => m.mockXai(kode)),
  },
  harga: { list: () => get<EntriHarga[]>("/v1/harga?limit=500", () => m.mockEntri()) },
  ews: { list: () => get<EwsAlert[]>("/v1/ews?limit=200", m.mockEws) },
  intervensi: {
    list: () => get<KartuIntervensi[]>("/v1/intervensi", m.mockKanban),
    pindah: (id: string, tahap: string, urutan: number) =>
      MODE_MOCK ? mockDelay({ ok: true }, 200, 400) : apiFetch(`/v1/intervensi/${id}/tahap`, { method: "PATCH", body: JSON.stringify({ tahap, urutan }) }),
  },
  coldStorage: {
    list: listColdStorage,
    /** Seluruh lot yang cocok dengan filter (semua halaman), mis. untuk "pilih semua hasil". */
    semua: async (params: Omit<ColdStorageParams, "page" | "limit"> = {}): Promise<StokColdStorage[]> => {
      const rows: StokColdStorage[] = [];
      for (let page = 1, last = 1; page <= last; page++) {
        const v = await listColdStorage({ ...params, page, limit: COLD_STORAGE_MAX_LIMIT });
        rows.push(...v.stok);
        last = v.halaman.totalPages;
      }
      return rows;
    },
    bulk: (action: ColdStorageBulkAction, rows: StokColdStorage[]): Promise<BulkColdStorageResult> =>
      MODE_MOCK
        ? mockDelay({ berhasil: rows.length, gagal: 0, volumeTon: Math.round(rows.reduce((a, r) => a + r.volumeTon, 0) * 10) / 10 }, 500, 1100)
        : coldStorageClient.bulk(action, rows.map((r) => r.id)),
  },
  sumber: {
    list: (): Promise<SumberData[]> =>
      MODE_MOCK ? mockDelay(m.mockSumber) : apiFetch<SumberDataResponse[]>("/v1/sumber-data").then((rows) => rows.map(toSumberData)),
    /** Jeda atau lanjutkan sumber; mengubah status katalog (penjadwal nyata menyusul di 05-B). */
    bulk: (action: SumberBulkAksi, ids: string[]): Promise<BulkResult> =>
      MODE_MOCK ? mockDelay({ berhasil: ids.length, gagal: 0 }, 300, 600) : apiFetch<BulkResult>("/v1/sumber-data/bulk", { method: "POST", body: JSON.stringify({ action, ids }) }),
  },
  pengguna: { list: () => (MODE_MOCK ? mockDelay(m.mockPengguna) : penggunaApi.list()) },
  rbac: { katalog: (): Promise<KatalogAkses> => (MODE_MOCK ? mockDelay(m.mockKatalogAkses) : penggunaApi.katalog()) },
  audit: {
    /** Satu halaman audit; filter dan paginasi cursor dijalankan server. */
    page: (filter: AuditFilter, cursor?: string): Promise<AuditPage> =>
      MODE_MOCK
        ? mockDelay(() => queryMockAudit(m.mockAudit(), filter, cursor))
        : apiFetch<AuditPageResponse>(`/v1/audit${toAuditQuery(filter, cursor)}`).then(toAuditPage),
  },
  brief: {
    list: () => get<PolicyBrief[]>("/v1/policy-brief", m.mockBriefs),
    susun: (periode: string) =>
      MODE_MOCK ? mockDelay({ id: "PB-2026-10" }, 2200, 3200) : apiFetch<{ id: string }>("/v1/policy-brief/generate", { method: "POST", body: JSON.stringify({ periode }) }),
  },
  tugas: { saya: () => get<TugasLapangan[]>("/v1/lapangan/tugas/saya", m.mockTugas) },
  master: {
    komoditas: (): Promise<MasterKomoditasRow[]> =>
      MODE_MOCK ? mockDelay(m.mockMasterKomoditas) : apiFetch<MasterKomoditasResponse[]>("/v1/master/komoditas").then((rows) => rows.map(toMasterKomoditasRow)),
    wilayah: (): Promise<MasterWilayahRow[]> =>
      MODE_MOCK ? mockDelay(m.mockMasterWilayah) : apiFetch<MasterWilayahResponse[]>("/v1/master/wilayah").then((rows) => rows.map(toMasterWilayahRow)),
    /** Pasar beserta nama kabupaten/kota pemiliknya (memuat wilayah juga). */
    pasar: async (): Promise<MasterPasarRow[]> => {
      if (MODE_MOCK) return mockDelay(m.mockMasterPasar);
      const [pasar, wilayah] = await Promise.all([
        apiFetch<MasterPasarResponse[]>("/v1/master/pasar"),
        apiFetch<MasterWilayahResponse[]>("/v1/master/wilayah"),
      ]);
      const nama = new Map(wilayah.map((w) => [w.kode, toMasterWilayahRow(w).nama]));
      return pasar.map((p) => toMasterPasarRow(p, nama));
    },
  },
};
