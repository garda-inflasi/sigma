import { API_BASE, MODE_MOCK, apiFetch, mockDelay } from "./client";
import type { SiklusMlResponse, SumberKesegaranMlResponse } from "./ml-prod";

export interface RingkasanKesegaran {
  segar: number;
  basi: number;
  gagal: number;
  dilewati: number;
  belum_ada_run_terjadwal: number;
}

export interface FreshnessResponse {
  tersedia: boolean;
  alasan?: string;
  dihitung_pada: string;
  ringkasan: RingkasanKesegaran;
  sumber: SumberKesegaranMlResponse[];
  sinyal_sumber: {
    sinyal_kunci: string;
    severity: string;
    judul: string;
    pesan: string;
  }[];
}

export interface DaftarSiklusMlResponse {
  tersedia: boolean;
  alasan?: string;
  page: number;
  limit: number;
  total: number;
  items: SiklusMlResponse[];
}

export interface WorkerJobStatus {
  configured: boolean;
  reachable: boolean;
  berjalan: boolean;
  job_id?: string;
  cycle_id?: string;
  sumber?: string[];
  gagal_saja?: boolean;
  tahap?: string;
  progress_pct: number;
  dataset_aktif?: string;
  selesai_dataset?: number;
  total_dataset?: number;
  mulai?: string;
  selesai?: string;
  durasi_ms?: number | null;
  logs: string[];
  error?: string;
  jadwal?: Record<string, unknown>;
  slot_terakhir?: Record<string, { terakhir?: string; selesai_pada?: string | null }>;
}

export interface ChainJobState {
  aktif: boolean;
  chain_id?: string;
  tahap: string;
  progress_pct: number;
  pesan?: string;
  mulai?: string;
  selesai?: string;
  error?: string;
  crawler_job_id?: string;
  pipeline_job_id?: string;
  logs: string[];
}

export interface PipelineJobsSnapshot {
  timestamp: string;
  crawler: WorkerJobStatus;
  pipeline: WorkerJobStatus;
  chain: ChainJobState;
}

const MOCK_FRESHNESS: FreshnessResponse = {
  tersedia: true,
  dihitung_pada: new Date().toISOString(),
  ringkasan: {
    segar: 4,
    basi: 1,
    gagal: 1,
    dilewati: 1,
    belum_ada_run_terjadwal: 0,
  },
  sinyal_sumber: [],
  sumber: [
    {
      sumber: "PIHPS",
      dataset: "harga_harian",
      kategori: "SEGAR",
      status: "FRESH",
      pesan: "Data harga pasar harian terbaru diterima (23 kab/kota).",
      last_available_date: "2026-11-24",
      umur_hari: 0,
      run_terakhir: {
        run_id: "PIHPS-20261124-0600",
        crawl_date: "2026-11-24",
        selesai: "2026-11-23T23:03:12Z",
        status: "FRESH",
        jumlah_baru: 230,
      },
    },
    {
      sumber: "BPS",
      dataset: "ihk_bulanan",
      kategori: "SEGAR",
      status: "FRESH",
      pesan: "Rilis IHK bulanan sudah tersinkronisasi.",
      last_available_date: "2026-11-01",
      umur_hari: 0,
      run_terakhir: {
        run_id: "BPS-20261124-0600",
        crawl_date: "2026-11-24",
        selesai: "2026-11-23T23:04:01Z",
        status: "FRESH",
        jumlah_baru: 18,
      },
    },
    {
      sumber: "BMKG",
      dataset: "cuaca_harian",
      kategori: "SEGAR",
      status: "FRESH",
      pesan: "Prakiraan dan curah hujan 23 wilayah tersimpan.",
      last_available_date: "2026-11-24",
      umur_hari: 0,
      run_terakhir: {
        run_id: "BMKG-20261124-0600",
        crawl_date: "2026-11-24",
        selesai: "2026-11-23T23:02:18Z",
        status: "FRESH",
        jumlah_baru: 23,
      },
    },
    {
      sumber: "OPEN_METEO",
      dataset: "iklim_harian",
      kategori: "BASI",
      status: "STALE_CARRIED_FORWARD",
      pesan: "Throttle API eksternal; nilai hari sebelumnya dibawa maju.",
      last_available_date: "2026-11-22",
      umur_hari: 2,
      run_terakhir: {
        run_id: "OM-20261124-0600",
        crawl_date: "2026-11-24",
        selesai: "2026-11-23T23:05:11Z",
        status: "STALE_CARRIED_FORWARD",
        jumlah_baru: 0,
      },
    },
    {
      sumber: "SP2KP",
      dataset: "harga_konsumen",
      kategori: "GAGAL",
      status: "ERROR",
      pesan: "HTTP 504 Gateway Timeout dari endpoint upstream.",
      last_available_date: "2026-11-21",
      umur_hari: 3,
      run_terakhir: {
        run_id: "SP2KP-20261124-0600",
        crawl_date: "2026-11-24",
        selesai: "2026-11-23T23:06:40Z",
        status: "ERROR",
        jumlah_baru: 0,
      },
    },
    {
      sumber: "BAPANAS",
      dataset: "panel_harga",
      kategori: "SEGAR",
      status: "FRESH",
      pesan: "Panel harga pangan nasional/provinsi lengkap.",
      last_available_date: "2026-11-24",
      umur_hari: 0,
      run_terakhir: {
        run_id: "BAPANAS-20261124-0600",
        crawl_date: "2026-11-24",
        selesai: "2026-11-23T23:03:50Z",
        status: "FRESH",
        jumlah_baru: 161,
      },
    },
    {
      sumber: "LAPANGAN",
      dataset: "entri_lapangan",
      kategori: "DILEWATI",
      status: "FRESH",
      pesan: "Sinkronisasi per jam dari PostgreSQL aktif.",
      last_available_date: "2026-11-24",
      umur_hari: 0,
      run_terakhir: {
        run_id: "LAP-20261124-0400",
        crawl_date: "2026-11-24",
        selesai: "2026-11-24T04:01:05Z",
        status: "FRESH",
        jumlah_baru: 42,
      },
    },
  ],
};

const MOCK_CYCLES: DaftarSiklusMlResponse = {
  tersedia: true,
  page: 1,
  limit: 24,
  total: 2,
  items: [
    {
      cycle_id: "C20261124T04Z",
      cycle_timestamp: "2026-11-24T04:00:12Z",
      data_cutoff: "2026-11-24",
      status: "SEBAGIAN",
      target_inflasi: "INFLASI_PROKSI",
      label_target: "Indeks tekanan harga proksi",
    },
    {
      cycle_id: "C20261124T03Z",
      cycle_timestamp: "2026-11-24T03:00:08Z",
      data_cutoff: "2026-11-24",
      status: "BERHASIL",
      target_inflasi: "INFLASI_PROKSI",
      label_target: "Indeks tekanan harga proksi",
    },
  ],
};

let mockSnapshot: PipelineJobsSnapshot = {
  timestamp: new Date().toISOString(),
  crawler: {
    configured: true,
    reachable: true,
    berjalan: false,
    tahap: "SELESAI",
    progress_pct: 100,
    logs: ["[04:01:05 UTC] Sinkronisasi lapangan selesai."],
    jadwal: { harian: "Setiap hari 06:00 WIB", lapangan: "Tiap 1 jam (xx:00 WIB)" },
  },
  pipeline: {
    configured: true,
    reachable: true,
    berjalan: false,
    cycle_id: "C20261124T04Z",
    tahap: "SELESAI",
    progress_pct: 100,
    logs: ["[04:03:18 UTC] Siklus C20261124T04Z disimpan ke datasafe."],
    jadwal: { interval_jam: 1 },
  },
  chain: {
    aktif: false,
    tahap: "SIAGA",
    progress_pct: 0,
    logs: [],
  },
};

export async function getCrawlerFreshness(): Promise<FreshnessResponse> {
  if (MODE_MOCK) {
    return mockDelay(MOCK_FRESHNESS);
  }
  return apiFetch<FreshnessResponse>("/v1/ml/freshness");
}

export async function listMlCycles(limit = 24): Promise<DaftarSiklusMlResponse> {
  if (MODE_MOCK) {
    return mockDelay(MOCK_CYCLES);
  }
  return apiFetch<DaftarSiklusMlResponse>(`/v1/ml/cycles?limit=${limit}`);
}

export async function getPipelineJobs(): Promise<PipelineJobsSnapshot> {
  if (MODE_MOCK) {
    return mockDelay({ ...mockSnapshot, timestamp: new Date().toISOString() });
  }
  return apiFetch<PipelineJobsSnapshot>("/v1/ml/jobs");
}

export async function triggerCrawlerJob(input: {
  sumber?: string[];
  gagal_saja?: boolean;
  lanjut_ml?: boolean;
}): Promise<PipelineJobsSnapshot> {
  if (MODE_MOCK) {
    mockSnapshot = {
      ...mockSnapshot,
      timestamp: new Date().toISOString(),
      crawler: {
        ...mockSnapshot.crawler,
        berjalan: false,
        job_id: `CRW-MOCK-${Date.now()}`,
        sumber: input.sumber ?? ["SP2KP"],
        gagal_saja: Boolean(input.gagal_saja),
        tahap: "SELESAI",
        progress_pct: 100,
        logs: [
          `[${new Date().toISOString().slice(11, 19)} UTC] Simulasi retry sumber selesai.`,
        ],
      },
      chain: input.lanjut_ml
        ? {
            aktif: false,
            chain_id: `CHN-MOCK-${Date.now()}`,
            tahap: "SELESAI",
            progress_pct: 100,
            pesan: "Rantai Crawler -> ML simulasi selesai.",
            logs: ["Simulasi rantai Crawler -> ML selesai."],
          }
        : mockSnapshot.chain,
    };
    return mockDelay(mockSnapshot, 200, 350);
  }
  return apiFetch<PipelineJobsSnapshot>("/v1/ml/crawler/run", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function triggerPipelineJob(): Promise<PipelineJobsSnapshot> {
  if (MODE_MOCK) {
    mockSnapshot = {
      ...mockSnapshot,
      timestamp: new Date().toISOString(),
      pipeline: {
        ...mockSnapshot.pipeline,
        berjalan: false,
        job_id: `MLP-MOCK-${Date.now()}`,
        cycle_id: "C20261124T12Z",
        tahap: "SELESAI",
        progress_pct: 100,
        logs: [
          `[${new Date().toISOString().slice(11, 19)} UTC] Training ulang ML datasafe selesai.`,
        ],
      },
    };
    return mockDelay(mockSnapshot, 200, 350);
  }
  return apiFetch<PipelineJobsSnapshot>("/v1/ml/pipeline/run", {
    method: "POST",
    body: "{}",
  });
}

/**
 * Menghubungkan klien ke aliran SSE `GET /v1/ml/jobs/stream` untuk pembaruan progres crawler & training ML realtime.
 */
export function subscribePipelineJobsStream(
  onSnapshot: (snap: PipelineJobsSnapshot) => void,
  onStatus?: (connected: boolean) => void,
): () => void {
  if (MODE_MOCK || typeof EventSource === "undefined") {
    return () => {};
  }
  const es = new EventSource(`${API_BASE}/v1/ml/jobs/stream`, {
    withCredentials: true,
  });
  es.onopen = () => onStatus?.(true);
  es.addEventListener("snapshot", (ev) => {
    try {
      const parsed = JSON.parse((ev as MessageEvent<string>).data) as PipelineJobsSnapshot;
      onStatus?.(true);
      onSnapshot(parsed);
    } catch {
      // Abaikan frame yang tidak lengkap
    }
  });
  es.onerror = () => onStatus?.(false);
  return () => {
    es.close();
  };
}
