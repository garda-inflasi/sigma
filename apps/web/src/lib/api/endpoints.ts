import { apiFetch, mockDelay, MODE_MOCK } from "./client";
import { toAuditPage, toAuditQuery, queryMockAudit, type AuditFilter, type AuditPage, type AuditPageResponse } from "./audit";
import { coldStorageClient, queryMockColdStorage, type BulkColdStorageResult } from "./cold-storage";
import { COLD_STORAGE_MAX_LIMIT, type ColdStorageBulkAction, type ColdStorageParams } from "./cold-storage-query";
import {
  toMasterKomoditasRow, toMasterPasarRow, toMasterWilayahRow,
  type MasterKomoditasResponse, type MasterPasarResponse, type MasterWilayahResponse,
} from "./master";
import {
  aktivitasMock, toAktivitasLapangan, toHeatmap, toKpiOperasional,
  type AktivitasLapangan, type AktivitasLapanganResponse, type Heatmap, type HeatmapResponse, type KpiResponse,
} from "./dashboard";
import {
  filterMockEntri, ringkasanDariEntri, toEntriHarga, toHargaQuery,
  type EntriHargaResponse, type HargaParams, type RingkasanHarga,
} from "./harga";
import { toTugasLapangan, type TugasLapanganResponse } from "./lapangan";
import { penggunaApi, type KatalogAkses } from "./pengguna";
import { USERS } from "@/lib/data/reference";
import * as m from "./mock";
import type {
  ColdStorageView, EntriHarga, EwsAlert, KartuIntervensi, KpiOperasional, MasterKomoditasRow, MasterPasarRow, MasterWilayahRow,
  PenggunaRow, PolicyBrief, ProyeksiXai, RingkasanEksekutif, StokColdStorage, SumberData, TugasLapangan,
} from "@/lib/data/types";

export type BulkResult = { berhasil: number; gagal: number; pesan?: string };

/** Jumlah petugas lapangan mock per kabupaten, untuk target aktivitas harian. */
const petugasPerKabupatenMock = () => {
  const n = new Map<string, number>();
  for (const u of USERS) if (u.role === "OPS_MAHASISWA" && u.kabupatenKode) n.set(u.kabupatenKode, (n.get(u.kabupatenKode) ?? 0) + 1);
  return n;
};

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
    kpi: (): Promise<KpiOperasional[]> =>
      MODE_MOCK ? mockDelay(m.mockKpi) : apiFetch<KpiResponse[]>("/v1/dashboard/operasional/kpi").then((rows) => rows.map(toKpiOperasional)),
    heatmap: (): Promise<Heatmap> =>
      MODE_MOCK ? mockDelay(m.mockHeatmap) : apiFetch<HeatmapResponse>("/v1/dashboard/operasional/heatmap").then(toHeatmap),
    aktivitas: (): Promise<AktivitasLapangan[]> =>
      MODE_MOCK
        ? mockDelay(() => aktivitasMock(m.mockEntri(), petugasPerKabupatenMock()))
        : apiFetch<AktivitasLapanganResponse[]>("/v1/dashboard/operasional/aktivitas-lapangan").then((rows) => rows.map(toAktivitasLapangan)),
    marker: () => get<ReturnType<typeof m.mockMapMarkers>>("/v1/spasial/wilayah", m.mockMapMarkers),
    tren: (kode: string, kab: string) =>
      get<ReturnType<typeof m.mockTrenHarga>>(`/v1/harga/tren?komoditas=${kode}&wilayah=${kab}`, () => m.mockTrenHarga(kode, kab)),
  },
  eksekutif: {
    ringkasan: () => get<RingkasanEksekutif>("/v1/dashboard/eksekutif", m.mockEksekutif),
    xai: (kode: string) => get<ProyeksiXai>(`/v1/prediksi/${kode}`, () => m.mockXai(kode)),
  },
  harga: {
    list: (params: HargaParams = {}): Promise<EntriHarga[]> =>
      MODE_MOCK ? mockDelay(() => filterMockEntri(m.mockEntri(), params)) : apiFetch<EntriHargaResponse[]>(`/v1/harga${toHargaQuery(params)}`).then((rows) => rows.map(toEntriHarga)),
    /** Entri milik pengguna yang masuk (riwayat petugas lapangan). */
    saya: (): Promise<EntriHarga[]> =>
      MODE_MOCK ? mockDelay(m.mockEntri) : apiFetch<EntriHargaResponse[]>("/v1/harga/saya?limit=100").then((rows) => rows.map(toEntriHarga)),
    ringkasan: (hari = 7): Promise<RingkasanHarga> =>
      MODE_MOCK ? mockDelay(() => ringkasanDariEntri(m.mockEntri())) : apiFetch<RingkasanHarga>(`/v1/harga/ringkasan?hari=${hari}`),
  },
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
  sumber: { list: () => get<SumberData[]>("/v1/sumber-data", m.mockSumber) },
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
  tugas: {
    saya: (): Promise<TugasLapangan[]> =>
      MODE_MOCK ? mockDelay(m.mockTugas) : apiFetch<TugasLapanganResponse[]>("/v1/lapangan/tugas/saya").then((rows) => rows.map(toTugasLapangan)),
  },
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
