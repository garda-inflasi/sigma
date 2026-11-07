import { ApiError, apiFetch, mockDelay, MODE_MOCK } from "./client";
import {
  mockAuditIntegritas, queryMockAudit, toAuditDetail, toAuditIntegritas, toAuditPage, toAuditQuery, toMockAuditDetail, unduhEksporAudit,
  type AuditDetailResponse, type AuditFilter, type AuditIntegritasResponse, type AuditPage, type AuditPageResponse, type HasilEkspor,
} from "./audit";
import { exportCsv } from "@/lib/csv";
import { coldStorageClient, queryMockColdStorage, type BulkColdStorageResult, type HasilTelemetri, type PembacaanManualPayload, type TransaksiHasil, type TransaksiPayload, type UbahFasilitasPayload } from "./cold-storage";
import { COLD_STORAGE_MAX_LIMIT, type ColdStorageBulkAction, type ColdStorageParams } from "./cold-storage-query";
import {
  toMasterBulkHasil, toMasterKomoditasRow, toMasterPasarRow, toMasterWilayahRow,
  type MasterBulkAksi, type MasterBulkHasil, type MasterBulkResponse, type MasterKomoditasResponse, type MasterPasarResponse, type MasterWilayahResponse,
} from "./master";
import { masterApi, type AktorMock } from "./master-api";
import { mockBulkKomoditas, mockKomoditasList, mockPasarList, mockWilayahList } from "./master-mock";
import {
  aktivitasMock, toAktivitasLapangan, toHeatmap, toKpiOperasional,
  type AktivitasLapangan, type AktivitasLapanganResponse, type CuacaResponse, type CuacaWilayah, type Heatmap, type HeatmapResponse, type KpiResponse,
  type PembandingHarga, type PembandingHargaResponse, toCuacaWilayah, toPembandingHarga,
} from "./dashboard";
import {
  filterMockEntri, ringkasanDariEntri, toEntriHarga, toHargaQuery, toRiwayatStatus, toDetailHarga,
  type DetailHarga, type EntriHargaResponse, type HargaDetailResponse, type HargaParams, type RincianBulk, type RiwayatStatus, type RiwayatStatusResponse, type RingkasanHarga,
} from "./harga";
import { toLineageHarga, type LineageResponse } from "./harga-lineage";
import { mockLineageById } from "./harga-lineage-mock";
import { mockEwsDetail, mockEwsRiwayat } from "./ews-mock";
import { mockSumberDetail } from "./sumber-mock";
import { toTugasDetail, toTugasLapangan, type TugasDetail, type TugasDetailResponse, type TugasLapanganResponse } from "./lapangan";
import { mockTugasDetail } from "./lapangan-mock";
import { toRingkasanEksekutif, type RingkasanEksekutifResponse } from "./eksekutif";
import { kpiMock, kpiRiwayatMock, toKpiIndikator, toKpiRiwayat, type KpiIndikator, type KpiResponse as KpiIndikatorResponse, type KpiRiwayat, type KpiRiwayatResponse } from "./kpi";
import {
  cocokRujukanAlert, toBatalPushBody, toEwsAlert, toEwsAturan, toEwsDetail, toEwsRiwayat, toPreferensiBody, toPreferensiView, toUbahAturanBody,
  type EvaluasiEws, type EwsAlertResponse, type EwsAturan, type EwsAturanResponse, type EwsDetail, type EwsRiwayat, type EwsRiwayatResponse, type PreferensiNotifikasi, type PreferensiNotifikasiResponse, type PreferensiView, type UbahAturanInput,
} from "./ews";
import {
  toAlur, toBuatBody, toKartu, toPersetujuan, toRiwayat, toTahapPerSiklus,
  type AksiKartu, type Alur, type AlurResponse, type BuatIntervensiInput, type BulkKartuResult, type KartuBaru, type KartuResponse,
  type KeputusanPersetujuan, type Persetujuan, type PersetujuanResponse, type RiwayatKartu, type RiwayatResponse, type TahapResponse,
} from "./intervensi";
import { alurMock, riwayatMock, tahapMock } from "./intervensi-mock";
import type { Tahap } from "@/lib/data/siklus";
import type { Siklus } from "@/lib/design/tokens";
import { toGeometri, toKandidat, toLokasi, toLokasiQuery, toMarker, toNeraca, toNeracaQuery, toOptimasiBody, toRencanaOptimasi, toRiwayatOptimasi, toRute, type GeometriResponse, type GeometriWilayah, type KandidatColdStorage, type KandidatResponse, type LokasiResponse, type LokasiWilayah, type MarkerResponse, type NeracaResponse, type NeracaWilayah, type OptimasiParams, type OptimasiResponse, type RencanaOptimasi, type RiwayatOptimasi, type RiwayatOptimasiResponse, type Rute, type RuteResponse } from "./spasial";
import { mockLokasi, mockNeraca, mockRiwayatOptimasi } from "./spasial-mock";
import type { MarkerWilayah } from "@/components/map/marker-pin";
import { toPolicyBrief, toPolicyBriefDetail, type PolicyBriefDetail, type PolicyBriefDetailResponse, type PolicyBriefResponse } from "./brief";
import { mockBriefDetail } from "./brief-mock";
import {
  toBarisModel, toBarisProyeksi, toHasilSimulasi, toProyeksiXai,
  type AksiModel, type AksiModelResult, type BarisModel, type BarisProyeksi, type EvaluasiResponse, type HasilSimulasi, type ModelResponse,
  type ProyeksiResponse, type RingkasanResponse, type SegarkanResponse, type SimulasiInput, type SimulasiResponse, type StatusMl,
} from "./prediksi";
import { mockBarisModel, mockBarisProyeksi, mockSimulasi } from "./prediksi-mock";
import { penggunaApi, type KatalogAkses, type PenggunaDetail } from "./pengguna";
import { mockPenggunaDetail, queryMockPengguna } from "./pengguna-mock";
import type { StateTabelPengguna } from "@/lib/pengguna/tabel";
import { toRiwayatPeran, type RiwayatPeran } from "@/lib/pengguna/riwayat-peran";
import { USERS } from "@/lib/data/reference";
import { toEksekusi, toPipeline, toSampel, toSumberData, toSumberDetail, type SumberDetailData, type EksekusiResponse, type PipelineResponse, type SampelResponse, type SumberBulkAksi, type SumberDataResponse } from "./sumber";
import * as m from "./mock";
import type {
  AuditDetail, AuditIntegritas, ColdStorageView, DetailFasilitas, FasilitasColdStorage, EksekusiSumber, EntriHarga, EwsAlert, KartuIntervensi, KpiOperasional, LineageHarga, MasterKomoditasRow, MasterPasarRow,
  MasterWilayahRow, PenggunaRow, PipelineSumber, PolicyBrief, ProyeksiXai, RingkasanEksekutif, StokColdStorage, SampelSumber, SumberData, TugasLapangan,
} from "@/lib/data/types";

export type BulkResult = { berhasil: number; gagal: number; pesan?: string; rincian?: RincianBulk[] };

/** Jumlah petugas lapangan mock per kabupaten, untuk target aktivitas harian. */
const petugasPerKabupatenMock = () => {
  const n = new Map<string, number>();
  for (const u of USERS) if (u.role === "OPS_MAHASISWA" && u.kabupatenKode) n.set(u.kabupatenKode, (n.get(u.kabupatenKode) ?? 0) + 1);
  return n;
};

const get = <T,>(path: string, mock: () => T) => (MODE_MOCK ? mockDelay(mock) : apiFetch<T>(path));

/** Aksi massal generik: POST /v1/{sumber}/bulk { action, ids } */
export function bulk(sumber: string, aksi: string, ids: string[], alasan?: string): Promise<BulkResult> {
  if (MODE_MOCK) return mockDelay({ berhasil: ids.length, gagal: 0 }, 500, 1100);
  return apiFetch<BulkResult>(`/v1/${sumber}/bulk`, { method: "POST", body: JSON.stringify({ action: aksi, ids, alasan }), idempotencyKey: crypto.randomUUID() });
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
    /** Validasi silang harga lapangan vs eksternal (hasil ingestion). */
    pembandingHarga: (): Promise<PembandingHarga> =>
      MODE_MOCK
        ? mockDelay({ pasangan: 0, sepakat: 0, batasPct: 10, items: [] })
        : apiFetch<PembandingHargaResponse>("/v1/dashboard/operasional/pembanding-harga").then(toPembandingHarga),
    /** Cuaca harian terbaru per kabupaten/kota (BMKG). */
    cuaca: (): Promise<CuacaWilayah[]> =>
      MODE_MOCK ? mockDelay([]) : apiFetch<CuacaResponse[]>("/v1/dashboard/operasional/cuaca").then((rows) => rows.map(toCuacaWilayah)),
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
    /** Meramal ulang proyeksi model produksi hasil pelatihan (tanpa melatih ulang); 503 bila layanan ML mati, 409 bila model produksi masih demo. */
    segarkan: (komoditasKode: string): Promise<SegarkanResponse> =>
      MODE_MOCK
        ? Promise.reject(new Error("Menyegarkan proyeksi memerlukan backend dan layanan ML."))
        : apiFetch<SegarkanResponse>("/v1/prediksi/segarkan", { method: "POST", body: JSON.stringify({ komoditas_kode: komoditasKode }) }),
    /** Menghitung galat nyata proyeksi yang sudah jatuh tempo (dasar MAPE realisasi). */
    evaluasi: (): Promise<EvaluasiResponse> =>
      MODE_MOCK
        ? Promise.reject(new Error("Evaluasi akurasi memerlukan backend."))
        : apiFetch<EvaluasiResponse>("/v1/prediksi/evaluasi", { method: "POST" }),
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
    /** Rencana distribusi optimal surplus → defisit; `simpan` menulisnya sebagai usulan rute. */
    optimasi: (p: OptimasiParams): Promise<RencanaOptimasi> =>
      MODE_MOCK
        ? mockDelay(() => ({ komoditasNama: "", periode: "", rute: [], totalVolumeTon: 0, totalBiaya: 0, belumTerpenuhi: [], surplusSisa: [], penghematanPersen: null, optimasiId: null, optimasiKode: null }), 400, 800)
        : apiFetch<OptimasiResponse>("/v1/spasial/optimasi-rute", { method: "POST", body: JSON.stringify(toOptimasiBody(p)) }).then(toRencanaOptimasi),
    /** Poligon wilayah (perkiraan pada data demo); mode mock tidak punya geometri. */
    geometri: (zoom: number): Promise<GeometriWilayah[]> =>
      MODE_MOCK ? mockDelay(() => []) : apiFetch<GeometriResponse>(`/v1/spasial/geometri?zoom=${zoom}`).then(toGeometri),
    rute: (): Promise<Rute[]> =>
      MODE_MOCK ? mockDelay(m.mockRute) : apiFetch<RuteResponse[]>("/v1/spasial/rute").then((rows) => rows.map(toRute)),
    kandidatColdStorage: (): Promise<KandidatColdStorage[]> =>
      MODE_MOCK ? mockDelay(m.mockKandidatColdStorage) : apiFetch<KandidatResponse[]>("/v1/spasial/kandidat-cold-storage").then((rows) => rows.map(toKandidat)),
    /** Neraca surplus/defisit per wilayah; tanpa periode dipakai periode terbaru, tanpa komoditas semua komoditas. */
    neraca: (komoditasKode?: string, periode?: string): Promise<NeracaWilayah[]> =>
      MODE_MOCK
        ? mockDelay(() => mockNeraca(komoditasKode))
        : apiFetch<NeracaResponse[]>(`/v1/spasial/neraca${toNeracaQuery(komoditasKode, periode)}`).then((rows) => rows.map(toNeraca)),
    /** Kabupaten/kota untuk satu koordinat; 404 bila belum ada data wilayah. */
    lokasi: (lat: number, lng: number): Promise<LokasiWilayah> =>
      MODE_MOCK ? mockDelay(() => mockLokasi(lat, lng), 200, 400) : apiFetch<LokasiResponse>(`/v1/spasial/lokasi${toLokasiQuery(lat, lng)}`).then(toLokasi),
    /** Riwayat rencana optimasi yang pernah disimpan (terbaru dulu). */
    riwayatOptimasi: (): Promise<RiwayatOptimasi[]> =>
      MODE_MOCK ? mockDelay(mockRiwayatOptimasi) : apiFetch<RiwayatOptimasiResponse[]>("/v1/spasial/optimasi-rute").then((rows) => rows.map(toRiwayatOptimasi)),
  },
  harga: {
    list: (params: HargaParams = {}): Promise<EntriHarga[]> =>
      MODE_MOCK ? mockDelay(() => filterMockEntri(m.mockEntri(), params)) : apiFetch<EntriHargaResponse[]>(`/v1/harga${toHargaQuery(params)}`).then((rows) => rows.map(toEntriHarga)),
    /** Entri milik pengguna yang masuk (riwayat petugas lapangan). */
    saya: (): Promise<EntriHarga[]> =>
      MODE_MOCK ? mockDelay(m.mockEntri) : apiFetch<EntriHargaResponse[]>("/v1/harga/saya?limit=100").then((rows) => rows.map(toEntriHarga)),
    /** Bukti dan bendera validasi otomatis satu entri; mode mock tidak punya data ini. */
    detail: (id: string): Promise<DetailHarga> =>
      MODE_MOCK ? mockDelay({ bendera: [] }) : apiFetch<HargaDetailResponse>(`/v1/harga/${encodeURIComponent(id)}`).then(toDetailHarga),
    /** Perpindahan status satu entri; mode mock tidak punya riwayat tersimpan. */
    riwayat: (id: string): Promise<RiwayatStatus[]> =>
      MODE_MOCK ? mockDelay([]) : apiFetch<RiwayatStatusResponse[]>(`/v1/harga/${encodeURIComponent(id)}/riwayat`).then((rows) => rows.map(toRiwayatStatus)),
    /** Jejak asal satu entri: pencatat, cek otomatis, validasi, publikasi, versi, dan rujukan audit; mode mock menurunkannya dari entri mock. */
    lineage: (id: string): Promise<LineageHarga> =>
      MODE_MOCK ? mockDelay(() => mockLineageById(id)) : apiFetch<LineageResponse>(`/v1/harga/${encodeURIComponent(id)}/lineage`).then(toLineageHarga),
    ringkasan: (hari = 7): Promise<RingkasanHarga> =>
      MODE_MOCK ? mockDelay(() => ringkasanDariEntri(m.mockEntri())) : apiFetch<RingkasanHarga>(`/v1/harga/ringkasan?hari=${hari}`),
  },
  ews: {
    list: (): Promise<EwsAlert[]> =>
      MODE_MOCK ? mockDelay(m.mockEws) : apiFetch<EwsAlertResponse[]>("/v1/ews?limit=200").then((rows) => rows.map(toEwsAlert)),
    /**
     * Satu alert lengkap (jejak pengakuan, eskalasi, penutupan); 404 (ApiError) bila tidak ada. `id` boleh
     * ID desimal atau kode lama (EWS-0901): server menerima keduanya (alias tautan lama).
     */
    detail: (id: string): Promise<EwsDetail> =>
      MODE_MOCK
        ? mockDelay(() => {
            const a = m.mockEws().find((x) => cocokRujukanAlert(x, id));
            if (!a) throw new ApiError(404, "Alert EWS tidak ditemukan");
            return mockEwsDetail(a);
          })
        : apiFetch<EwsAlertResponse>(`/v1/ews/${encodeURIComponent(id)}`).then(toEwsDetail),
    /** Perubahan status alert (terlama dahulu): siapa, kapan, dan catatannya. */
    riwayat: (id: string): Promise<EwsRiwayat[]> =>
      MODE_MOCK
        ? mockDelay(() => {
            const a = m.mockEws().find((x) => cocokRujukanAlert(x, id));
            return a ? mockEwsRiwayat(a) : [];
          })
        : apiFetch<EwsRiwayatResponse[]>(`/v1/ews/${encodeURIComponent(id)}/riwayat`).then((rows) => rows.map(toEwsRiwayat)),
    /** Aturan ambang per komoditas dan jenis, beserta kanal notifikasi. */
    aturan: (): Promise<EwsAturan[]> =>
      MODE_MOCK ? mockDelay(m.mockEwsAturan) : apiFetch<EwsAturanResponse[]>("/v1/ews/aturan").then((rows) => rows.map(toEwsAturan)),
    /** Mengubah ambang aturan; 409 (ApiError) bila aturan sudah diubah pengguna lain sejak dibaca. */
    ubahAturan: (id: string, input: UbahAturanInput): Promise<EwsAturan> =>
      MODE_MOCK
        ? mockDelay(() => ({ ...m.mockEwsAturan().find((a) => a.id === id || a.kode === id)!, ...input, versi: input.versi + 1 }), 200, 400)
        : apiFetch<EwsAturanResponse>(`/v1/ews/aturan/${encodeURIComponent(id)}`, { method: "PUT", body: JSON.stringify(toUbahAturanBody(input)) }).then(toEwsAturan),
    /** Menjalankan rule engine sekarang (di luar jadwal). */
    evaluasi: (): Promise<EvaluasiEws> =>
      MODE_MOCK
        ? mockDelay({ aturan: 9, diukur: 0, dibuat: 0, dinaikkan: 0, dilewati: 0, diredam: 0, galat: 0 }, 300, 600)
        : apiFetch<EvaluasiEws>("/v1/ews/evaluasi", { method: "POST" }),
    /** Preferensi notifikasi milik pengguna yang masuk (bawaan bila belum diatur). */
    preferensi: (): Promise<PreferensiView> =>
      MODE_MOCK
        ? mockDelay<PreferensiView>({ vapidPublicKey: "", preferensi: [{ kanal: "EMAIL", severityMin: "SIAGA", aktif: true, tersedia: false }, { kanal: "PUSH", severityMin: "SIAGA", aktif: true, tersedia: false }] })
        : apiFetch<PreferensiNotifikasiResponse>("/v1/ews/notifikasi/preferensi").then(toPreferensiView),
    simpanPreferensi: (prefs: PreferensiNotifikasi[]): Promise<PreferensiView> =>
      MODE_MOCK
        ? mockDelay<PreferensiView>({ vapidPublicKey: "", preferensi: prefs }, 200, 400)
        : apiFetch<PreferensiNotifikasiResponse>("/v1/ews/notifikasi/preferensi", { method: "PUT", body: JSON.stringify(toPreferensiBody(prefs)) }).then(toPreferensiView),
    /** Mendaftarkan perangkat ini untuk Web Push. */
    daftarPush: (body: { endpoint: string; p256dh: string; auth: string }) =>
      MODE_MOCK ? mockDelay({ ok: true }, 200, 400) : apiFetch("/v1/ews/notifikasi/langganan-push", { method: "PUT", body: JSON.stringify(body) }),
    /** Membatalkan langganan Web Push perangkat ini. 404 berarti server sudah tidak menyimpannya: sama saja bagi pemanggil. */
    batalPush: async (endpoint: string): Promise<void> => {
      if (MODE_MOCK) return void (await mockDelay(undefined, 200, 400));
      try {
        await apiFetch("/v1/ews/notifikasi/langganan-push", { method: "DELETE", body: JSON.stringify(toBatalPushBody(endpoint)) });
      } catch (err) {
        if (!(err instanceof ApiError && err.status === 404)) throw err;
      }
    },
  },
  intervensi: {
    list: (): Promise<KartuIntervensi[]> =>
      MODE_MOCK ? mockDelay(m.mockKanban) : apiFetch<KartuResponse[]>("/v1/intervensi").then((rows) => rows.map(toKartu)),
    /** Definisi tahap per siklus (kolom papan, izin masuk, SLA). */
    tahap: (): Promise<Record<Siklus, Tahap[]>> =>
      MODE_MOCK
        ? mockDelay(() => toTahapPerSiklus(tahapMock()), 120, 250)
        : apiFetch<TahapResponse[]>("/v1/intervensi/tahap").then(toTahapPerSiklus),
    /** Alur persetujuan berjenjang (rantai langkah dan izinnya) per tahap tujuan. */
    alur: (): Promise<Alur[]> =>
      MODE_MOCK ? mockDelay(() => alurMock.map(toAlur), 120, 250) : apiFetch<AlurResponse[]>("/v1/intervensi/alur").then((rows) => rows.map(toAlur)),
    /** Riwayat satu kartu, terlama dahulu. 404 (ApiError) bila kartu tidak ada. */
    riwayat: (id: string): Promise<RiwayatKartu[]> =>
      MODE_MOCK
        ? mockDelay(() => riwayatMock(m.mockKanban().find((k) => k.id === id || k.kode === id) ?? m.mockKanban()[0]).map(toRiwayat), 150, 300)
        : apiFetch<RiwayatResponse[]>(`/v1/intervensi/${encodeURIComponent(id)}/riwayat`).then((rows) => rows.map(toRiwayat)),
    /** Membuat kartu di tahap pertama siklusnya; butuh backend. */
    buat: (input: BuatIntervensiInput): Promise<KartuBaru> =>
      MODE_MOCK
        ? Promise.reject(new Error("Membuat intervensi membutuhkan backend aktif."))
        : apiFetch<KartuBaru>("/v1/intervensi", { method: "POST", body: JSON.stringify(toBuatBody(input)) }),
    /** Memindahkan kartu; mengembalikan versi baru. 409 (ApiError) bila kartu sudah diubah pengguna lain. */
    pindah: (id: string, tahap: string, urutan: number, versi: number): Promise<{ versi: number }> =>
      MODE_MOCK
        ? mockDelay({ versi: versi + 1 }, 200, 400)
        : apiFetch<{ versi: number }>(`/v1/intervensi/${encodeURIComponent(id)}/tahap`, { method: "PATCH", body: JSON.stringify({ tahap, urutan, versi }) }),
    /** Antrean permintaan persetujuan (terbaru dahulu); status kosong = semua. */
    persetujuan: (status?: Persetujuan["status"]): Promise<Persetujuan[]> =>
      MODE_MOCK
        ? mockDelay<Persetujuan[]>([])
        : apiFetch<PersetujuanResponse[]>(`/v1/intervensi/persetujuan${status ? `?status=${status}` : ""}`).then((rows) => rows.map(toPersetujuan)),
    /** Meminta persetujuan memindahkan kartu ke tahap persetujuan atau sesudahnya. */
    ajukan: (id: string, tahapTujuan: string, catatan = ""): Promise<Persetujuan> =>
      MODE_MOCK
        ? Promise.reject(new Error("Persetujuan membutuhkan backend aktif."))
        : apiFetch<PersetujuanResponse>(`/v1/intervensi/${encodeURIComponent(id)}/persetujuan`, { method: "POST", body: JSON.stringify({ tahap_tujuan: tahapTujuan, catatan }) }).then(toPersetujuan),
    /** Memutuskan langkah yang menunggu; TOLAK wajib bercatatan. */
    putus: (id: string, keputusan: KeputusanPersetujuan, catatan = ""): Promise<Persetujuan> =>
      MODE_MOCK
        ? Promise.reject(new Error("Persetujuan membutuhkan backend aktif."))
        : apiFetch<PersetujuanResponse>(`/v1/intervensi/persetujuan/${encodeURIComponent(id)}/keputusan`, { method: "POST", body: JSON.stringify({ keputusan, catatan }) }).then(toPersetujuan),
    /** Menugaskan kartu ke pengguna yang masuk, atau mengeskalasinya ke Dewan Pengarah; hasil per kartu. */
    aksi: (aksi: AksiKartu, ids: string[]): Promise<BulkKartuResult> =>
      MODE_MOCK
        ? mockDelay({ berhasil: ids.length, gagal: 0, hasil: ids.map((id) => ({ id, berhasil: true })) }, 300, 600)
        : apiFetch<BulkKartuResult>("/v1/intervensi/bulk", { method: "POST", body: JSON.stringify({ action: aksi, ids }) }),
  },
  coldStorage: {
    list: listColdStorage,
    detail: (id: string): Promise<DetailFasilitas> =>
      MODE_MOCK
        ? mockDelay(() => {
            const { fasilitas, stok } = m.mockColdStorage();
            const f = fasilitas.find((x) => x.id === id);
            if (!f) throw new Error("Fasilitas tidak ditemukan");
            return { fasilitas: f, telemetri: [], stok: stok.filter((s) => s.fasilitasId === id), transaksi: [] };
          })
        : coldStorageClient.detail(id),
    transaksi: (p: TransaksiPayload): Promise<TransaksiHasil> =>
      MODE_MOCK
        ? mockDelay(() => ({ transaksiId: "1", lotId: "lotId" in p ? p.lotId : "1", lotBaruId: null, volumeSisaTon: 0, statusLot: "AKTIF" }), 300, 600)
        : coldStorageClient.transaksi(p),
    /** Ubah kapasitas, batas alarm suhu, atau status fasilitas (izin `coldstorage.kelola`, tercatat di audit). */
    ubahFasilitas: (id: string, p: UbahFasilitasPayload): Promise<FasilitasColdStorage> =>
      MODE_MOCK ? Promise.reject(new Error("Mengubah fasilitas memerlukan backend.")) : coldStorageClient.ubahFasilitas(id, p),
    /** Catat satu pembacaan suhu manual; duplikat dan alarm di luar rentang dilaporkan di hasil. */
    catatTelemetri: (p: PembacaanManualPayload): Promise<HasilTelemetri> =>
      MODE_MOCK ? Promise.reject(new Error("Mencatat suhu memerlukan backend.")) : coldStorageClient.catatTelemetri(p),
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
        ? mockDelay({ berhasil: rows.length, gagal: 0, volumeTon: Math.round(rows.reduce((a, r) => a + r.volumeTon, 0) * 10) / 10, rincian: rows.map((r) => ({ id: r.id, berhasil: true, pesan: "" })) }, 500, 1100)
        : coldStorageClient.bulk(action, rows.map((r) => r.id)),
  },
  sumber: {
    list: (): Promise<SumberData[]> =>
      MODE_MOCK ? mockDelay(m.mockSumber) : apiFetch<SumberDataResponse[]>("/v1/sumber-data").then((rows) => rows.map(toSumberData)),
    /** Satu sumber lengkap dengan data segar (status, terakhir jalan, atribusi, URL dasar); 404 (ApiError) bila tidak ada. */
    detail: (id: string): Promise<SumberDetailData> =>
      MODE_MOCK
        ? mockDelay(() => {
            const s = m.mockSumber().find((x) => x.id === id);
            if (!s) throw new ApiError(404, "Sumber data tidak ditemukan");
            return mockSumberDetail(s);
          })
        : apiFetch<SumberDataResponse>(`/v1/sumber-data/${encodeURIComponent(id)}`).then(toSumberDetail),
    /** Jeda atau lanjutkan sumber; sumber dijeda tidak dijadwalkan dan tidak bisa dijalankan manual. */
    bulk: (action: SumberBulkAksi, ids: string[]): Promise<BulkResult> =>
      MODE_MOCK ? mockDelay({ berhasil: ids.length, gagal: 0 }, 300, 600) : apiFetch<BulkResult>("/v1/sumber-data/bulk", { method: "POST", body: JSON.stringify({ action, ids }) }),
    /** Isi lapisan bronze/silver/gold dan mutu data dari eksekusi nyata. */
    pipeline: (): Promise<PipelineSumber> =>
      MODE_MOCK
        ? mockDelay({ bronze: 0, silver: 0, gold: 0, demo: false, mutu: { kelengkapan: null, akurasi: null, ketepatanWaktu: null, konsistensi: null } })
        : apiFetch<PipelineResponse>("/v1/sumber-data/pipeline").then(toPipeline),
    /** Riwayat eksekusi ingestion, terbaru lebih dulu. */
    eksekusi: (id: string): Promise<EksekusiSumber[]> =>
      MODE_MOCK ? mockDelay([]) : apiFetch<EksekusiResponse[]>(`/v1/sumber-data/${encodeURIComponent(id)}/eksekusi`).then((rows) => rows.map(toEksekusi)),
    /** Cuplikan payload mentah (bronze) terakhir. */
    sampel: (id: string): Promise<SampelSumber[]> =>
      MODE_MOCK ? mockDelay([]) : apiFetch<SampelResponse[]>(`/v1/sumber-data/${encodeURIComponent(id)}/sampel?limit=3`).then((rows) => rows.map(toSampel)),
    /** Memulai ingestion sekarang (izin sumber.kelola); hasil dibaca dari riwayat eksekusi. */
    jalankan: (id: string): Promise<EksekusiSumber> =>
      MODE_MOCK
        ? Promise.reject(new Error("Menjalankan ingestion membutuhkan backend aktif."))
        : apiFetch<EksekusiResponse>(`/v1/sumber-data/${encodeURIComponent(id)}/jalankan`, { method: "POST" }).then(toEksekusi),
  },
  kpi: {
    list: (): Promise<KpiIndikator[]> =>
      MODE_MOCK ? mockDelay(kpiMock) : apiFetch<KpiIndikatorResponse[]>("/v1/kpi").then((rows) => rows.map(toKpiIndikator)),
    /** Nilai berlaku tiap periode (paling banyak 36) satu indikator. */
    riwayat: (kode: string): Promise<KpiRiwayat> =>
      MODE_MOCK ? mockDelay(() => kpiRiwayatMock(kode)) : apiFetch<KpiRiwayatResponse>(`/v1/kpi/${encodeURIComponent(kode)}/riwayat`).then(toKpiRiwayat),
  },
  pengguna: {
    list: () => (MODE_MOCK ? mockDelay(m.mockPengguna) : penggunaApi.list()),
    /** Satu halaman daftar pengguna; mode mock menyaring, mengurutkan, dan memaginasi di memori. */
    halaman: (state: StateTabelPengguna) => (MODE_MOCK ? mockDelay(() => queryMockPengguna(m.mockPengguna(), state)) : penggunaApi.daftar(state)),
    riwayatPeran: (id: string): Promise<RiwayatPeran> =>
      MODE_MOCK ? mockDelay(toRiwayatPeran({ items: [], terpotong: false })) : penggunaApi.riwayatPeran(id),
    detail: (id: string): Promise<PenggunaDetail> =>
      MODE_MOCK
        ? mockDelay(() => {
            const u = m.mockPengguna().find((x) => x.id === id);
            if (!u) throw new ApiError(404, "Pengguna tidak ditemukan");
            return mockPenggunaDetail(u);
          })
        : penggunaApi.get(id),
  },
  rbac: { katalog: (): Promise<KatalogAkses> => (MODE_MOCK ? mockDelay(m.mockKatalogAkses) : penggunaApi.katalog()) },
  audit: {
    /** Satu halaman audit; filter dan paginasi cursor dijalankan server. */
    page: (filter: AuditFilter, cursor?: string): Promise<AuditPage> =>
      MODE_MOCK
        ? mockDelay(() => queryMockAudit(m.mockAudit(), filter, cursor))
        : apiFetch<AuditPageResponse>(`/v1/audit${toAuditQuery(filter, cursor)}`).then(toAuditPage),
    /** Satu entri lengkap dengan diff sebelum/sesudah. */
    detail: (id: string): Promise<AuditDetail> =>
      MODE_MOCK
        ? mockDelay(() => {
            const row = m.mockAudit().find((r) => r.id === id);
            if (!row) throw new Error("Entri audit tidak ditemukan");
            return toMockAuditDetail(row);
          })
        : apiFetch<AuditDetailResponse>(`/v1/audit/${encodeURIComponent(id)}`).then(toAuditDetail),
    /** Status rantai hash audit: verifikasi terakhir, partisi bulanan aktif, dan arsip. */
    integritas: (): Promise<AuditIntegritas> =>
      MODE_MOCK
        ? mockDelay(() => mockAuditIntegritas())
        : apiFetch<AuditIntegritasResponse>("/v1/audit/integritas").then(toAuditIntegritas),
    /** Mengunduh CSV sesuai filter (dibuat server; mode mock membuatnya di klien). */
    ekspor: (filter: AuditFilter): Promise<HasilEkspor> =>
      MODE_MOCK
        ? mockDelay(() => {
            const { rows } = queryMockAudit(m.mockAudit(), filter, undefined, Number.MAX_SAFE_INTEGER);
            exportCsv(
              rows,
              [
                { header: "Waktu", value: (a) => a.waktu },
                { header: "Aktor", value: (a) => a.aktor },
                { header: "Peran", value: (a) => a.peran },
                { header: "Aksi", value: (a) => a.aksi },
                { header: "Entitas", value: (a) => `${a.entitas} ${a.entitasId}` },
                { header: "Hasil", value: (a) => a.hasil },
                { header: "IP", value: (a) => a.ip },
              ],
              "audit-trail",
            );
            return { baris: rows.length, terpotong: false };
          })
        : unduhEksporAudit(filter),
  },
  brief: {
    list: (): Promise<PolicyBrief[]> =>
      MODE_MOCK ? mockDelay(m.mockBriefs) : apiFetch<PolicyBriefResponse[]>("/v1/policy-brief").then((rows) => rows.map(toPolicyBrief)),
    /** Brief lengkap dengan isi dan jejak penyusunan; dipantau berkala selama berstatus DIPROSES. */
    detail: (id: string): Promise<PolicyBriefDetail> =>
      MODE_MOCK ? mockDelay(() => mockBriefDetail(id)) : apiFetch<PolicyBriefDetailResponse>(`/v1/policy-brief/${encodeURIComponent(id)}`).then(toPolicyBriefDetail),
    /** Memulai penyusunan draf brief untuk periode YYYY-MM; kembaliannya brief berstatus DIPROSES (di server, penyusunan berjalan di latar belakang). */
    susun: (periode: string): Promise<PolicyBrief> =>
      MODE_MOCK
        ? mockDelay(() => m.mockBriefs()[0], 1200, 1800)
        : apiFetch<PolicyBriefResponse>("/v1/policy-brief/generate", { method: "POST", body: JSON.stringify({ periode }) }).then(toPolicyBrief),
  },
  tugas: {
    saya: (): Promise<TugasLapangan[]> =>
      MODE_MOCK ? mockDelay(m.mockTugas) : apiFetch<TugasLapanganResponse[]>("/v1/lapangan/tugas/saya").then((rows) => rows.map(toTugasLapangan)),
    /** Satu tugas dengan rincian komoditas terisi/belum; 404 (ApiError) bila bukan milik pemanggil. */
    detail: (id: string): Promise<TugasDetail> =>
      MODE_MOCK
        ? mockDelay(() => {
            const t = m.mockTugas().find((x) => x.id === id);
            if (!t) throw new ApiError(404, "Tugas tidak ditemukan");
            return mockTugasDetail(t);
          })
        : apiFetch<TugasDetailResponse>(`/v1/lapangan/tugas/${encodeURIComponent(id)}`).then(toTugasDetail),
  },
  master: {
    komoditas: (): Promise<MasterKomoditasRow[]> =>
      MODE_MOCK ? mockDelay(() => mockKomoditasList().map(toMasterKomoditasRow)) : apiFetch<MasterKomoditasResponse[]>("/v1/master/komoditas").then((rows) => rows.map(toMasterKomoditasRow)),
    wilayah: (): Promise<MasterWilayahRow[]> =>
      MODE_MOCK ? mockDelay(() => mockWilayahList().map(toMasterWilayahRow)) : apiFetch<MasterWilayahResponse[]>("/v1/master/wilayah").then((rows) => rows.map(toMasterWilayahRow)),
    /** Pasar beserta nama kabupaten/kota pemiliknya (memuat wilayah juga). */
    pasar: async (): Promise<MasterPasarRow[]> => {
      if (MODE_MOCK) {
        return mockDelay(() => {
          const nama = new Map(mockWilayahList().map((w) => [w.kode, toMasterWilayahRow(w).nama]));
          return mockPasarList().map((p) => toMasterPasarRow(p, nama));
        });
      }
      const [pasar, wilayah] = await Promise.all([
        apiFetch<MasterPasarResponse[]>("/v1/master/pasar"),
        apiFetch<MasterWilayahResponse[]>("/v1/master/wilayah"),
      ]);
      const nama = new Map(wilayah.map((w) => [w.kode, toMasterWilayahRow(w).nama]));
      return pasar.map((p) => toMasterPasarRow(p, nama));
    },
    /** Ajukan/setujui/arsipkan komoditas; izin per aksi diperiksa server, hasil dilaporkan per kode. */
    bulkKomoditas: (aksi: MasterBulkAksi, kode: string[], aktor?: AktorMock): Promise<MasterBulkHasil> =>
      MODE_MOCK
        ? mockDelay(() => toMasterBulkHasil(mockBulkKomoditas(aksi, kode, aktor ?? { id: "pengguna-mock", nama: "Pengguna mock" })), 200, 450)
        : apiFetch<MasterBulkResponse>("/v1/master/komoditas/bulk", { method: "POST", body: JSON.stringify({ action: aksi, kode }) }).then(toMasterBulkHasil),
    ...masterApi,
  },
};
