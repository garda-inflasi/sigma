import { apiFetch, mockDelay, MODE_MOCK } from "./client";
import { toAuditRow, type AuditLogResponse } from "./audit";
import { penggunaApi } from "./pengguna";
import * as m from "./mock";
import type {
  EntriHarga, EwsAlert, FasilitasColdStorage, KartuIntervensi, KpiOperasional, MasterKomoditasRow,
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
    ringkasan: () => get<RingkasanEksekutif>("/v1/dashboard/eksekutif", m.mockEksekutif),
    xai: (kode: string) => get<ProyeksiXai>(`/v1/prediksi/${kode}`, () => m.mockXai(kode)),
  },
  harga: { list: () => get<EntriHarga[]>("/v1/harga?limit=500", () => m.mockEntri()) },
  ews: { list: () => get<EwsAlert[]>("/v1/ews?limit=200", m.mockEws) },
  intervensi: {
    list: () => get<KartuIntervensi[]>("/v1/intervensi", m.mockKanban),
    pindah: (id: string, tahap: string, urutan: number) =>
      MODE_MOCK ? mockDelay({ ok: true }, 200, 400) : apiFetch(`/v1/intervensi/${id}/tahap`, { method: "PATCH", body: JSON.stringify({ tahap, urutan }) }),
  },
  coldStorage: { ringkasan: () => get<{ fasilitas: FasilitasColdStorage[]; stok: StokColdStorage[] }>("/v1/cold-storage", m.mockColdStorage) },
  sumber: { list: () => get<SumberData[]>("/v1/sumber-data", m.mockSumber) },
  pengguna: { list: () => (MODE_MOCK ? mockDelay(m.mockPengguna) : penggunaApi.list()) },
  audit: { list: () => (MODE_MOCK ? mockDelay(m.mockAudit) : apiFetch<AuditLogResponse[]>("/v1/audit?limit=500").then((rows) => rows.map(toAuditRow))) },
  brief: {
    list: () => get<PolicyBrief[]>("/v1/policy-brief", m.mockBriefs),
    susun: (periode: string) =>
      MODE_MOCK ? mockDelay({ id: "PB-2026-10" }, 2200, 3200) : apiFetch<{ id: string }>("/v1/policy-brief/generate", { method: "POST", body: JSON.stringify({ periode }) }),
  },
  tugas: { saya: () => get<TugasLapangan[]>("/v1/lapangan/tugas/saya", m.mockTugas) },
  master: { komoditas: () => get<MasterKomoditasRow[]>("/v1/master/komoditas", m.mockMasterKomoditas) },
};
