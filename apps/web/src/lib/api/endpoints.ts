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
import { toRingkasanEksekutif, type RingkasanEksekutifResponse } from "./eksekutif";
import { kpiMock, toKpiIndikator, type KpiIndikator, type KpiResponse as KpiIndikatorResponse } from "./kpi";
import { toEwsAlert, toEwsAturan, type EwsAlertResponse, type EwsAturan, type EwsAturanResponse } from "./ews";
import { toKartu, type KartuResponse } from "./intervensi";
import { toKandidat, toMarker, toRute, type KandidatColdStorage, type KandidatResponse, type MarkerResponse, type Rute, type RuteResponse } from "./spasial";
import type { MarkerWilayah } from "@/components/map/marker-pin";
import { toPolicyBrief, type PolicyBriefResponse } from "./brief";
import {
  toBarisModel, toBarisProyeksi, toHasilSimulasi, toProyeksiXai,
  type AksiModel, type AksiModelResult, type BarisModel, type BarisProyeksi, type HasilSimulasi, type ModelResponse,
  type ProyeksiResponse, type RingkasanResponse, type SimulasiInput, type SimulasiResponse, type StatusMl,
} from "./prediksi";
import { mockBarisModel, mockBarisProyeksi, mockSimulasi } from "./prediksi-mock";
import { penggunaApi, type KatalogAkses } from "./pengguna";
import { USERS } from "@/lib/data/reference";
import { toSumberData, type SumberBulkAksi, type SumberDataResponse } from "./sumber";
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

/** Aksi massal generik: POST /v1/{sumber}/bulk { action, ids } */
export function bulk(sumber: string, aksi: string, ids: string[]): Promise<BulkResult> {
  if (MODE_MOCK) return mockDelay({ berhasil: ids.length, gagal: 0 }, 500, 1100);
  return apiFetch<BulkResult>(`/v1/${sumber}/bulk`, { method: "POST", body: JSON.stringify({ action: aksi, ids }), idempotencyKey: crypto.randomUUID() });
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
    marker: (): Promise<MarkerWilayah[]> =>
      MODE_MOCK ? mockDelay(m.mockMapMarkers) : apiFetch<MarkerResponse[]>("/v1/spasial/wilayah").then((rows) => rows.map(toMarker)),
    tren: (kode: string, kab: string) =>
      get<ReturnType<typeof m.mockTrenHarga>>(`/v1/harga/tren?komoditas=${kode}&wilayah=${kab}`, () => m.mockTrenHarga(kode, kab)),
  },
  eksekutif: {
    ringkasan: (): Promise<RingkasanEksekutif> =>
      MODE_MOCK ? mockDelay(m.mockEksekutif) : apiFetch<RingkasanEksekutifResponse>("/v1/dashboard/eksekutif").then(toRingkasanEksekutif),
    xai: (kode: string): Promise<ProyeksiXai> =>
      MODE_MOCK ? mockDelay(() => m.mockXai(kode)) : apiFetch<ProyeksiResponse>(`/v1/prediksi/${encodeURIComponent(kode)}`).then(toProyeksiXai),
  },
  prediksi: {
    /** Proyeksi 14 hari seluruh komoditas (Kota Banda Aceh). */
    ringkasan: (): Promise<BarisProyeksi[]> =>
      MODE_MOCK ? mockDelay(mockBarisProyeksi) : apiFetch<RingkasanResponse[]>("/v1/prediksi").then((rows) => rows.map(toBarisProyeksi)),
    /** Registri model seluruh komoditas. */
    model: (): Promise<BarisModel[]> =>
      MODE_MOCK ? mockDelay(mockBarisModel) : apiFetch<ModelResponse[]>("/v1/prediksi/registri").then((rows) => rows.map(toBarisModel)),
    /** Keadaan layanan ML (pelatihan hanya bisa bila tersedia). */
    statusMl: (): Promise<StatusMl> =>
      MODE_MOCK ? mockDelay<StatusMl>(() => ({ dikonfigurasi: false, tersedia: false, versi: "" })) : apiFetch<StatusMl>("/v1/prediksi/ml"),
    /** Melatih model kandidat dari harga tervalidasi; galat 503 bila layanan ML mati. */
    latih: (komoditasKode: string): Promise<BarisModel> =>
      MODE_MOCK
        ? Promise.reject(new Error("Pelatihan model memerlukan backend dan layanan ML."))
        : apiFetch<ModelResponse>("/v1/prediksi/latih", { method: "POST", body: JSON.stringify({ komoditas_kode: komoditasKode }) }).then(toBarisModel),
    /** Promosi (kandidat/arsip ke produksi) atau arsip kandidat; izin `model.kelola`, dicatat di audit. */
    aksiModel: (aksi: AksiModel, ids: string[]): Promise<AksiModelResult> =>
      MODE_MOCK
        ? mockDelay({ berhasil: ids.length, gagal: 0, detail: [] }, 300, 600)
        : apiFetch<AksiModelResult>("/v1/prediksi/registri/bulk", { method: "POST", body: JSON.stringify({ action: aksi, ids }), idempotencyKey: crypto.randomUUID() }),
    /** Simulasi skenario what-if yang dihitung dan disimpan server. */
    simulasi: (input: SimulasiInput): Promise<HasilSimulasi> =>
      MODE_MOCK
        ? mockDelay(() => mockSimulasi(input), 200, 400)
        : apiFetch<SimulasiResponse>("/v1/prediksi/simulasi", {
            method: "POST",
            body: JSON.stringify({ komoditas_kode: input.komoditasKode, hujan_pct: input.hujanPct, pasokan_pct: input.pasokanPct, subsidi_pct: input.subsidiPct }),
          }).then(toHasilSimulasi),
  },
  spasial: {
    rute: (): Promise<Rute[]> =>
      MODE_MOCK ? mockDelay(m.mockRute) : apiFetch<RuteResponse[]>("/v1/spasial/rute").then((rows) => rows.map(toRute)),
    kandidatColdStorage: (): Promise<KandidatColdStorage[]> =>
      MODE_MOCK ? mockDelay(m.mockKandidatColdStorage) : apiFetch<KandidatResponse[]>("/v1/spasial/kandidat-cold-storage").then((rows) => rows.map(toKandidat)),
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
  ews: {
    list: (): Promise<EwsAlert[]> =>
      MODE_MOCK ? mockDelay(m.mockEws) : apiFetch<EwsAlertResponse[]>("/v1/ews?limit=200").then((rows) => rows.map(toEwsAlert)),
    /** Aturan ambang per komoditas dan jenis, beserta kanal notifikasi. */
    aturan: (): Promise<EwsAturan[]> =>
      MODE_MOCK ? mockDelay(m.mockEwsAturan) : apiFetch<EwsAturanResponse[]>("/v1/ews/aturan").then((rows) => rows.map(toEwsAturan)),
  },
  intervensi: {
    list: (): Promise<KartuIntervensi[]> =>
      MODE_MOCK ? mockDelay(m.mockKanban) : apiFetch<KartuResponse[]>("/v1/intervensi").then((rows) => rows.map(toKartu)),
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
  kpi: {
    list: (): Promise<KpiIndikator[]> =>
      MODE_MOCK ? mockDelay(kpiMock) : apiFetch<KpiIndikatorResponse[]>("/v1/kpi").then((rows) => rows.map(toKpiIndikator)),
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
    list: (): Promise<PolicyBrief[]> =>
      MODE_MOCK ? mockDelay(m.mockBriefs) : apiFetch<PolicyBriefResponse[]>("/v1/policy-brief").then((rows) => rows.map(toPolicyBrief)),
    /** Menyimpan draf brief untuk periode YYYY-MM; kembaliannya brief yang tersimpan. */
    susun: (periode: string): Promise<PolicyBrief> =>
      MODE_MOCK
        ? mockDelay(() => m.mockBriefs()[0], 1200, 1800)
        : apiFetch<PolicyBriefResponse>("/v1/policy-brief/generate", { method: "POST", body: JSON.stringify({ periode }) }).then(toPolicyBrief),
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
