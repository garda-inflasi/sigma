/**
 * Lampiran `ml` (hasil training MongoDB `datasafe` dan kesegaran `dataunseen`, dibaca API Go) dalam bentuk camelCase
 * untuk UI, beserta daftar siklus training (`GET /v1/ml/cycles`). Kontrak: docs/PIPELINE-DATA-ML.md §5.
 *
 * `data` endpoint lama tidak pernah disentuh di sini: modul ini hanya membaca kunci `ml` di samping `data`. Mode mock
 * (tanpa backend) tidak mengarang hasil training: semua blok dilaporkan tidak tersedia.
 */
import { apiFetch, apiFetchEnvelope, MODE_MOCK } from "./client";

// ── Respons mentah (snake_case, apa adanya dari API) ─────────────────────────

export type KelayakanMl = "LAYAK" | "TIDAK_LAYAK" | "TIDAK_DIKETAHUI";
export type StatusSiklusMl = "BERJALAN" | "BERHASIL" | "SEBAGIAN" | "GAGAL";
export type SatuanKontribusi = "poin_persen_harga" | "poin_persen_probabilitas";
export type KategoriKesegaran =
  "SEGAR" | "BASI" | "GAGAL" | "DILEWATI" | "BELUM_ADA_RUN_TERJADWAL";

interface MetaMentah {
  tersedia: boolean;
  alasan?: string;
  sumber_data?: string;
  cycle_id?: string;
  cycle_timestamp?: string;
}

interface SiklusRingkasMentah {
  cycle_id: string;
  cycle_timestamp: string;
  data_cutoff?: string | null;
  status: string;
  target_inflasi?: string | null;
  label_target?: string | null;
}

interface FaktorMentah {
  kelompok: string;
  label: string;
  kontribusi: number | null;
  arah: string;
  fitur?: { nama: string; kontribusi: number | null }[] | null;
}

interface PenjelasanMentah {
  metode: string;
  satuan_kontribusi?: string | null;
  faktor?: FaktorMentah[] | null;
  narasi?: string | null;
}

interface Nilai3Mentah {
  yoy: number | null;
  mtm: number | null;
  ytd: number | null;
}

interface InflasiWilayahMentah {
  wilayah_kode: string;
  tingkat?: string;
  periode?: string | null;
  kelas?: string | null;
  layak?: boolean | null;
  kelayakan?: string | null;
  alasan_kelayakan?: string | null;
  probabilitas?: number | null;
  skor_tekanan?: number | null;
  siklus?: string | null;
  target?: string | null;
  label_target?: string | null;
  bukan_inflasi_resmi?: boolean;
  proyeksi?: Nilai3Mentah | null;
  aktual?: (Nilai3Mentah & { periode?: string | null }) | null;
  penyumbang?:
    { komoditas_kode: string; nama: string; andil: number | null }[] | null;
  kalibrasi?: { metode: string | null; brier: number | null } | null;
  penjelasan?: PenjelasanMentah | null;
}

interface TitikKurvaMentah {
  horizon: number;
  tanggal: string;
  p10: number | null;
  p50: number | null;
  p90: number | null;
  perubahan_p50_pct?: number | null;
}

interface ForecastMentah {
  komoditas_kode: string;
  kabupaten_kode: string;
  harga_terakhir: number | null;
  tanggal_terakhir: string | null;
  satuan: string;
  kurva?: TitikKurvaMentah[] | null;
  model: {
    nama: string;
    adalah_baseline?: boolean;
    fallback?: boolean;
    alasan?: string | null;
    n_kandidat?: number | null;
  };
  metrik: {
    mape: number | null;
    rmse: number | null;
    baseline?: string | null;
    mape_baseline?: number | null;
    mengalahkan_baseline?: boolean | null;
    evaluasi?: string | null;
    n_lipatan?: number | null;
    n_lipatan_seleksi?: number | null;
    horizon?: number[] | null;
  };
}

interface ForecastDetailMentah extends ForecastMentah {
  riwayat?: { tanggal: string; nilai: number | null }[] | null;
  penjelasan?: PenjelasanMentah | null;
}

interface KlasifikasiMentah {
  model?: string | null;
  f1?: number | null;
  roc_auc?: number | null;
  brier?: number | null;
  brier_baseline?: number | null;
  n_uji?: number | null;
  kalibrasi?: string | null;
  layak?: boolean | null;
  kelayakan?: string | null;
  alasan?: string | null;
}

interface RegresiMentah {
  pemenang?: Record<string, number> | null;
  mape_median?: number | null;
  rmse_median?: number | null;
  mape_baseline_median?: number | null;
  n_seri?: number | null;
  mengalahkan_baseline?: number | null;
  n_dievaluasi_holdout?: number | null;
  n_dilewati?: number | null;
  n_gagal?: number | null;
  model_global?: string | null;
  tingkat?: string | null;
}

interface SinyalMentah {
  sinyal_kunci: string;
  severity: string;
  jenis: string;
  komoditas_kode?: string | null;
  kabupaten_kode?: string | null;
  nilai?: number | null;
  ambang?: number | null;
  metode?: string | null;
  judul: string;
  pesan: string;
  aktif: boolean;
  menjadi_alert?: boolean;
  alert?: { id: string | number; kode?: string | null; status: string } | null;
}

interface RingkasanEwsMentah {
  aktif: number;
  kritis: number;
  siaga: number;
  waspada: number;
  info: number;
}

interface SumberKesegaranMentah {
  sumber: string;
  dataset: string;
  kategori: string;
  status?: string | null;
  pesan?: string;
  last_available_date?: string | null;
  umur_hari?: number | null;
  run_terakhir?: RunMentah | null;
  backfill_terakhir?: RunMentah | null;
  menurut_siklus?: {
    status: string | null;
    last_available_date: string | null;
    crawled_at: string | null;
    umur_hari: number | null;
    pesan: string;
  } | null;
}

interface RunMentah {
  run_id: string;
  crawler?: string;
  crawl_date: string;
  mulai?: string | null;
  selesai?: string | null;
  status: string;
  jumlah_baru?: number | null;
  jumlah_diteruskan?: number | null;
  last_available_date?: string | null;
  pesan?: string;
  pemicu?: string;
  dilewati?: boolean;
}

/** Isi kunci `ml` sebagaimana dikirim API (semua blok opsional; hanya blok rute itu yang hadir). */
export interface LampiranMlMentah {
  tersedia: boolean;
  alasan?: string;
  data_utama?: string;
  siklus?: SiklusRingkasMentah | null;
  inflasi?: MetaMentah & {
    kelayakan?: string | null;
    alasan_kelayakan?: string | null;
    target?: string | null;
    label_target?: string | null;
    bukan_inflasi_resmi?: boolean;
    wilayah?: InflasiWilayahMentah[] | null;
  };
  forecast_ringkas?: MetaMentah & {
    kabupaten_kode?: string | null;
    wilayah_tersedia?: string[] | null;
    items?: ForecastMentah[] | null;
  };
  forecast?: MetaMentah & {
    komoditas_kode?: string;
    kabupaten_kode?: string;
    wilayah_tersedia?: string[] | null;
    item?: ForecastDetailMentah | null;
  };
  model?: MetaMentah & {
    status?: string;
    target_inflasi?: string | null;
    label_target?: string | null;
    klasifikasi?: KlasifikasiMentah | null;
    regresi?: RegresiMentah | null;
  };
  ews?: MetaMentah & {
    ringkasan?: RingkasanEwsMentah;
    sinyal?: SinyalMentah[] | null;
    terpotong?: boolean;
  };
  wilayah?: MetaMentah & {
    items?:
      | {
          wilayah_kode: string;
          tingkat?: string;
          komoditas_forecast?: string[] | null;
          inflasi?: {
            kelas?: string | null;
            kelayakan?: string | null;
            probabilitas?: number | null;
            skor_tekanan?: number | null;
            siklus?: string | null;
            target?: string | null;
            label_target?: string | null;
          } | null;
          sinyal_aktif?: RingkasanEwsMentah;
        }[]
      | null;
  };
  kesegaran?: MetaMentah & {
    dihitung_pada?: string;
    siklus?: SiklusRingkasMentah | null;
    alasan_siklus?: string;
    ringkasan?: {
      segar: number;
      basi: number;
      gagal: number;
      dilewati: number;
      belum_ada_run_terjadwal: number;
    };
    sumber?: SumberKesegaranMentah[] | null;
    sinyal_sumber?: SinyalMentah[] | null;
  };
  makro?: MetaMentah & {
    items?:
      | {
          domain: string;
          indikator: string;
          wilayah_kode: string | null;
          periode: string | null;
          nilai: number | null;
          satuan: string;
          perubahan: number | null;
          sumber: string;
          is_carried_forward: boolean;
          last_available_date: string | null;
        }[]
      | null;
  };
}

/** Satu siklus dari `GET /v1/ml/cycles`. */
export interface SiklusMlMentah extends SiklusRingkasMentah {
  final?: boolean;
  duration_ms?: number | null;
  pesan?: string;
  versi_kode?: string;
  jumlah?: Record<string, number> | null;
  model_metrics?: {
    klasifikasi?: KlasifikasiMentah | null;
    regresi?: RegresiMentah | null;
  } | null;
}

export interface DaftarSiklusMlMentah {
  tersedia: boolean;
  alasan?: string;
  items?: SiklusMlMentah[] | null;
  total?: number;
  page?: number;
  limit?: number;
}

// ── Model UI (camelCase) ─────────────────────────────────────────────────────

/** Penanda yang dibawa setiap blok. Blok `tersedia: false` tidak boleh ditampilkan sebagai angka. */
export interface MetaBlokMl {
  tersedia: boolean;
  alasan: string | null;
  cycleId: string | null;
  cycleTimestamp: string | null;
}

export interface SiklusRingkasMl {
  cycleId: string;
  cycleTimestamp: string;
  dataCutoff: string | null;
  status: string;
  targetInflasi: string | null;
  labelTarget: string | null;
}

export interface FaktorMl {
  kelompok: string;
  label: string;
  kontribusi: number | null;
  arah: string;
  fitur: { nama: string; kontribusi: number | null }[];
}

export interface PenjelasanMl {
  metode: string;
  /** null pada dokumen lama: kontribusi tidak boleh dibaca sebagai poin persen. */
  satuanKontribusi: SatuanKontribusi | null;
  narasi: string;
  faktor: FaktorMl[];
}

export interface Nilai3Ml {
  yoy: number | null;
  mtm: number | null;
  ytd: number | null;
}

export interface InflasiWilayahMl {
  wilayahKode: string;
  tingkat: string;
  periode: string | null;
  /** Hanya boleh tampil sebagai prediksi bila `kelayakan` = LAYAK. */
  kelas: string | null;
  layak: boolean | null;
  kelayakan: KelayakanMl;
  alasanKelayakan: string | null;
  probabilitas: number | null;
  skorTekanan: number | null;
  siklus: string | null;
  target: string | null;
  labelTarget: string | null;
  bukanInflasiResmi: boolean;
  proyeksi: Nilai3Ml;
  aktual: Nilai3Ml & { periode: string | null };
  penyumbang: { komoditasKode: string; nama: string; andil: number | null }[];
  kalibrasi: { metode: string | null; brier: number | null };
  penjelasan: PenjelasanMl | null;
}

export interface BlokInflasiMl extends MetaBlokMl {
  kelayakan: KelayakanMl;
  alasanKelayakan: string | null;
  target: string | null;
  labelTarget: string | null;
  bukanInflasiResmi: boolean;
  wilayah: InflasiWilayahMl[];
}

export interface TitikKurvaMl {
  horizon: number;
  tanggal: string;
  p10: number | null;
  p50: number | null;
  p90: number | null;
  perubahanP50Pct: number | null;
}

export interface ModelForecastMl {
  nama: string;
  /** true: tidak ada kandidat yang mengalahkan baseline; yang dipublikasikan persistensi (naive/seasonal_naive). */
  adalahBaseline: boolean;
  fallback: boolean;
  alasan: string | null;
  nKandidat: number | null;
}

export interface MetrikForecastMl {
  mape: number | null;
  rmse: number | null;
  baseline: string | null;
  mapeBaseline: number | null;
  /** null bila tidak ada holdout. */
  mengalahkanBaseline: boolean | null;
  evaluasi: string | null;
  nLipatan: number | null;
  nLipatanSeleksi: number | null;
  horizon: number[];
}

export interface ForecastMl {
  komoditasKode: string;
  kabupatenKode: string;
  hargaTerakhir: number | null;
  tanggalTerakhir: string | null;
  satuan: string;
  kurva: TitikKurvaMl[];
  model: ModelForecastMl;
  metrik: MetrikForecastMl;
}

export interface ForecastDetailMl extends ForecastMl {
  riwayat: { tanggal: string; nilai: number | null }[];
  penjelasan: PenjelasanMl | null;
}

export interface BlokForecastRingkasMl extends MetaBlokMl {
  kabupatenKode: string | null;
  wilayahTersedia: string[];
  items: ForecastMl[];
}

export interface BlokForecastMl extends MetaBlokMl {
  komoditasKode: string;
  kabupatenKode: string;
  wilayahTersedia: string[];
  item: ForecastDetailMl | null;
}

export interface KlasifikasiMl {
  model: string | null;
  f1: number | null;
  rocAuc: number | null;
  brier: number | null;
  brierBaseline: number | null;
  nUji: number | null;
  kalibrasi: string | null;
  layak: boolean | null;
  kelayakan: KelayakanMl;
  alasan: string | null;
}

export interface RegresiMl {
  /** Jumlah seri per model pemenang. */
  pemenang: Record<string, number>;
  mapeMedian: number | null;
  rmseMedian: number | null;
  mapeBaselineMedian: number | null;
  nSeri: number | null;
  /** Jumlah seri yang modelnya mengalahkan baseline. */
  mengalahkanBaseline: number | null;
  nDievaluasiHoldout: number | null;
  nDilewati: number | null;
  nGagal: number | null;
  modelGlobal: string | null;
  tingkat: string | null;
}

export interface BlokModelMl extends MetaBlokMl {
  status: string;
  targetInflasi: string | null;
  labelTarget: string | null;
  klasifikasi: KlasifikasiMl | null;
  regresi: RegresiMl | null;
}

export type SeveritySinyal = "KRITIS" | "SIAGA" | "WASPADA" | "INFO";

export interface SinyalEwsMl {
  sinyalKunci: string;
  severity: SeveritySinyal;
  jenis: string;
  komoditasKode: string | null;
  kabupatenKode: string | null;
  nilai: number | null;
  ambang: number | null;
  metode: string | null;
  judul: string;
  pesan: string;
  aktif: boolean;
  /** Sinyal aktif WASPADA ke atas yang dijembatani menjadi alert Postgres; INFO tidak pernah. */
  menjadiAlert: boolean;
  alert: { id: string; kode: string | null; status: string } | null;
}

export interface RingkasanEwsMl {
  aktif: number;
  kritis: number;
  siaga: number;
  waspada: number;
  info: number;
}

export interface BlokEwsMl extends MetaBlokMl {
  ringkasan: RingkasanEwsMl;
  sinyal: SinyalEwsMl[];
  terpotong: boolean;
}

export interface WilayahMl {
  wilayahKode: string;
  tingkat: string;
  komoditasForecast: string[];
  inflasi: {
    kelas: string | null;
    kelayakan: KelayakanMl;
    probabilitas: number | null;
    skorTekanan: number | null;
    siklus: string | null;
    target: string | null;
    labelTarget: string | null;
  } | null;
  sinyalAktif: RingkasanEwsMl;
}

export interface BlokWilayahMl extends MetaBlokMl {
  items: WilayahMl[];
}

export interface SumberKesegaranMl {
  sumber: string;
  dataset: string;
  kategori: KategoriKesegaran;
  status: string | null;
  pesan: string;
  lastAvailableDate: string | null;
  umurHari: number | null;
}

export interface BlokKesegaranMl extends MetaBlokMl {
  dihitungPada: string | null;
  ringkasan: {
    segar: number;
    basi: number;
    gagal: number;
    dilewati: number;
    belumAdaRunTerjadwal: number;
  };
  sumber: SumberKesegaranMl[];
  sinyalSumber: SinyalEwsMl[];
}

export interface MakroMl {
  domain: string;
  indikator: string;
  wilayahKode: string | null;
  periode: string | null;
  nilai: number | null;
  satuan: string;
  perubahan: number | null;
  sumber: string;
  isCarriedForward: boolean;
  lastAvailableDate: string | null;
}

export interface BlokMakroMl extends MetaBlokMl {
  items: MakroMl[];
}

export interface LampiranMl {
  tersedia: boolean;
  alasan: string | null;
  /** Asal `data` di samping `ml`: POSTGRES kecuali mode prod (DATASAFE). */
  dataUtama: string;
  siklus: SiklusRingkasMl | null;
  inflasi?: BlokInflasiMl;
  forecastRingkas?: BlokForecastRingkasMl;
  forecast?: BlokForecastMl;
  model?: BlokModelMl;
  ews?: BlokEwsMl;
  wilayah?: BlokWilayahMl;
  kesegaran?: BlokKesegaranMl;
  makro?: BlokMakroMl;
}

export interface SiklusMl {
  cycleId: string;
  cycleTimestamp: string;
  dataCutoff: string | null;
  status: StatusSiklusMl | string;
  /** BERHASIL/SEBAGIAN: isinya boleh dibaca. BERJALAN/GAGAL hanya metadata. */
  final: boolean;
  durationMs: number | null;
  pesan: string;
  versiKode: string;
  jumlah: Record<string, number>;
  targetInflasi: string | null;
  labelTarget: string | null;
  klasifikasi: KlasifikasiMl | null;
  regresi: RegresiMl | null;
}

export interface DaftarSiklusMl {
  tersedia: boolean;
  alasan: string | null;
  items: SiklusMl[];
  total: number;
}

// ── Mapper ───────────────────────────────────────────────────────────────────

const STATUS_FINAL = new Set(["BERHASIL", "SEBAGIAN"]);
const SEVERITY = new Set<SeveritySinyal>([
  "KRITIS",
  "SIAGA",
  "WASPADA",
  "INFO",
]);
const KATEGORI = new Set<KategoriKesegaran>([
  "SEGAR",
  "BASI",
  "GAGAL",
  "DILEWATI",
  "BELUM_ADA_RUN_TERJADWAL",
]);
const RINGKASAN_EWS_KOSONG: RingkasanEwsMl = {
  aktif: 0,
  kritis: 0,
  siaga: 0,
  waspada: 0,
  info: 0,
};

const num = (v: unknown): number | null =>
  typeof v === "number" && Number.isFinite(v) ? v : null;
const str = (v: unknown): string | null =>
  typeof v === "string" && v !== "" ? v : null;

/**
 * Kelayakan klasifikasi. Nilai `kelayakan` dari server dipakai bila sah; bila tidak ada (dokumen lama), diturunkan
 * dari `layak`: true → LAYAK, false → TIDAK_LAYAK, null/tidak ada → TIDAK_DIKETAHUI.
 */
export function toKelayakan(
  kelayakan: unknown,
  layak?: boolean | null,
): KelayakanMl {
  if (
    kelayakan === "LAYAK" ||
    kelayakan === "TIDAK_LAYAK" ||
    kelayakan === "TIDAK_DIKETAHUI"
  )
    return kelayakan;
  if (layak === true) return "LAYAK";
  if (layak === false) return "TIDAK_LAYAK";
  return "TIDAK_DIKETAHUI";
}

function toMeta(m: MetaMentah | undefined): MetaBlokMl {
  return {
    tersedia: m?.tersedia === true,
    alasan: str(m?.alasan),
    cycleId: str(m?.cycle_id),
    cycleTimestamp: str(m?.cycle_timestamp),
  };
}

function toSiklusRingkas(
  s: SiklusRingkasMentah | null | undefined,
): SiklusRingkasMl | null {
  if (!s?.cycle_id) return null;
  return {
    cycleId: s.cycle_id,
    cycleTimestamp: s.cycle_timestamp,
    dataCutoff: str(s.data_cutoff),
    status: s.status,
    targetInflasi: str(s.target_inflasi),
    labelTarget: str(s.label_target),
  };
}

function toSatuan(v: unknown): SatuanKontribusi | null {
  return v === "poin_persen_harga" || v === "poin_persen_probabilitas"
    ? v
    : null;
}

export function toPenjelasanMl(
  p: PenjelasanMentah | null | undefined,
): PenjelasanMl | null {
  if (!p) return null;
  return {
    metode: p.metode,
    satuanKontribusi: toSatuan(p.satuan_kontribusi),
    narasi: p.narasi ?? "",
    faktor: (p.faktor ?? []).map((f) => ({
      kelompok: f.kelompok,
      label: f.label || f.kelompok,
      kontribusi: num(f.kontribusi),
      arah: f.arah,
      fitur: (f.fitur ?? []).map((ft) => ({
        nama: ft.nama,
        kontribusi: num(ft.kontribusi),
      })),
    })),
  };
}

function toNilai3(n: Nilai3Mentah | null | undefined): Nilai3Ml {
  return { yoy: num(n?.yoy), mtm: num(n?.mtm), ytd: num(n?.ytd) };
}

export function toInflasiWilayahMl(w: InflasiWilayahMentah): InflasiWilayahMl {
  const kelayakan = toKelayakan(w.kelayakan, w.layak);
  return {
    wilayahKode: w.wilayah_kode,
    tingkat:
      w.tingkat ?? (w.wilayah_kode.length <= 2 ? "PROVINSI" : "KABUPATEN"),
    periode: str(w.periode),
    kelas: str(w.kelas),
    layak: typeof w.layak === "boolean" ? w.layak : null,
    kelayakan,
    alasanKelayakan: str(w.alasan_kelayakan),
    probabilitas: num(w.probabilitas),
    skorTekanan: num(w.skor_tekanan),
    siklus: str(w.siklus),
    target: str(w.target),
    labelTarget: str(w.label_target),
    bukanInflasiResmi: w.bukan_inflasi_resmi ?? w.target !== "RESMI_BPS",
    proyeksi: toNilai3(w.proyeksi),
    aktual: { ...toNilai3(w.aktual), periode: str(w.aktual?.periode) },
    penyumbang: (w.penyumbang ?? []).map((p) => ({
      komoditasKode: p.komoditas_kode,
      nama: p.nama,
      andil: num(p.andil),
    })),
    kalibrasi: {
      metode: str(w.kalibrasi?.metode),
      brier: num(w.kalibrasi?.brier),
    },
    penjelasan: toPenjelasanMl(w.penjelasan),
  };
}

/** Model pemenang adalah baseline bila server menandainya, atau (dokumen lama) namanya naive/seasonal_naive. */
export const namaBaseline = (nama: string) =>
  nama === "naive" || nama === "seasonal_naive";

export function toForecastMl(f: ForecastMentah): ForecastMl {
  const kurva = (f.kurva ?? [])
    .map((t) => ({
      horizon: t.horizon,
      tanggal: t.tanggal,
      p10: num(t.p10),
      p50: num(t.p50),
      p90: num(t.p90),
      perubahanP50Pct: num(t.perubahan_p50_pct),
    }))
    .sort((a, b) => a.horizon - b.horizon);
  return {
    komoditasKode: f.komoditas_kode,
    kabupatenKode: f.kabupaten_kode,
    hargaTerakhir: num(f.harga_terakhir),
    tanggalTerakhir: str(f.tanggal_terakhir),
    satuan: f.satuan,
    kurva,
    model: {
      nama: f.model.nama,
      adalahBaseline: f.model.adalah_baseline ?? namaBaseline(f.model.nama),
      fallback: f.model.fallback === true,
      alasan: str(f.model.alasan),
      nKandidat: num(f.model.n_kandidat),
    },
    metrik: {
      mape: num(f.metrik.mape),
      rmse: num(f.metrik.rmse),
      baseline: str(f.metrik.baseline),
      mapeBaseline: num(f.metrik.mape_baseline),
      mengalahkanBaseline:
        typeof f.metrik.mengalahkan_baseline === "boolean"
          ? f.metrik.mengalahkan_baseline
          : null,
      evaluasi: str(f.metrik.evaluasi),
      nLipatan: num(f.metrik.n_lipatan),
      nLipatanSeleksi: num(f.metrik.n_lipatan_seleksi),
      horizon: f.metrik.horizon ?? kurva.map((t) => t.horizon),
    },
  };
}

export function toForecastDetailMl(f: ForecastDetailMentah): ForecastDetailMl {
  return {
    ...toForecastMl(f),
    riwayat: (f.riwayat ?? []).map((r) => ({
      tanggal: r.tanggal,
      nilai: num(r.nilai),
    })),
    penjelasan: toPenjelasanMl(f.penjelasan),
  };
}

export function toKlasifikasiMl(
  k: KlasifikasiMentah | null | undefined,
): KlasifikasiMl | null {
  if (!k) return null;
  return {
    model: str(k.model),
    f1: num(k.f1),
    rocAuc: num(k.roc_auc),
    brier: num(k.brier),
    brierBaseline: num(k.brier_baseline),
    nUji: num(k.n_uji),
    kalibrasi: str(k.kalibrasi),
    layak: typeof k.layak === "boolean" ? k.layak : null,
    kelayakan: toKelayakan(k.kelayakan, k.layak),
    alasan: str(k.alasan),
  };
}

export function toRegresiMl(
  r: RegresiMentah | null | undefined,
): RegresiMl | null {
  if (!r) return null;
  return {
    pemenang: { ...(r.pemenang ?? {}) },
    mapeMedian: num(r.mape_median),
    rmseMedian: num(r.rmse_median),
    mapeBaselineMedian: num(r.mape_baseline_median),
    nSeri: num(r.n_seri),
    mengalahkanBaseline: num(r.mengalahkan_baseline),
    nDievaluasiHoldout: num(r.n_dievaluasi_holdout),
    nDilewati: num(r.n_dilewati),
    nGagal: num(r.n_gagal),
    modelGlobal: str(r.model_global),
    tingkat: str(r.tingkat),
  };
}

function toSeverity(v: string): SeveritySinyal {
  return SEVERITY.has(v as SeveritySinyal) ? (v as SeveritySinyal) : "INFO";
}

export function toSinyalEwsMl(s: SinyalMentah): SinyalEwsMl {
  const severity = toSeverity(s.severity);
  return {
    sinyalKunci: s.sinyal_kunci,
    severity,
    jenis: s.jenis,
    komoditasKode: str(s.komoditas_kode),
    kabupatenKode: str(s.kabupaten_kode),
    nilai: num(s.nilai),
    ambang: num(s.ambang),
    metode: str(s.metode),
    judul: s.judul,
    pesan: s.pesan,
    aktif: s.aktif,
    // Server menghitung ini; bila tidak dikirim, aturan §5.4: aktif dan WASPADA ke atas.
    menjadiAlert: s.menjadi_alert ?? (s.aktif && severity !== "INFO"),
    alert: s.alert
      ? {
          id: String(s.alert.id),
          kode: str(s.alert.kode),
          status: s.alert.status,
        }
      : null,
  };
}

function toRingkasanEws(r: RingkasanEwsMentah | undefined): RingkasanEwsMl {
  return r
    ? {
        aktif: r.aktif,
        kritis: r.kritis,
        siaga: r.siaga,
        waspada: r.waspada,
        info: r.info,
      }
    : { ...RINGKASAN_EWS_KOSONG };
}

function toKategori(v: string): KategoriKesegaran {
  return KATEGORI.has(v as KategoriKesegaran)
    ? (v as KategoriKesegaran)
    : "GAGAL";
}

/** Memetakan kunci `ml` endpoint lama. `undefined` (respons tanpa `ml`) berarti tidak ada hasil training untuk dipakai. */
export function toLampiranMl(
  raw: LampiranMlMentah | null | undefined,
): LampiranMl {
  if (!raw)
    return {
      tersedia: false,
      alasan: "TIDAK_ADA_LAMPIRAN",
      dataUtama: "POSTGRES",
      siklus: null,
    };
  const out: LampiranMl = {
    tersedia: raw.tersedia === true,
    alasan: str(raw.alasan),
    dataUtama: raw.data_utama ?? "POSTGRES",
    siklus: toSiklusRingkas(raw.siklus),
  };
  if (raw.inflasi) {
    const b = raw.inflasi;
    out.inflasi = {
      ...toMeta(b),
      kelayakan: toKelayakan(b.kelayakan),
      alasanKelayakan: str(b.alasan_kelayakan),
      target: str(b.target),
      labelTarget: str(b.label_target),
      bukanInflasiResmi: b.bukan_inflasi_resmi ?? true,
      wilayah: (b.wilayah ?? []).map(toInflasiWilayahMl),
    };
  }
  if (raw.forecast_ringkas) {
    const b = raw.forecast_ringkas;
    out.forecastRingkas = {
      ...toMeta(b),
      kabupatenKode: str(b.kabupaten_kode),
      wilayahTersedia: [...(b.wilayah_tersedia ?? [])],
      items: (b.items ?? []).map(toForecastMl),
    };
  }
  if (raw.forecast) {
    const b = raw.forecast;
    out.forecast = {
      ...toMeta(b),
      komoditasKode: b.komoditas_kode ?? "",
      kabupatenKode: b.kabupaten_kode ?? "",
      wilayahTersedia: [...(b.wilayah_tersedia ?? [])],
      item: b.item ? toForecastDetailMl(b.item) : null,
    };
  }
  if (raw.model) {
    const b = raw.model;
    out.model = {
      ...toMeta(b),
      status: b.status ?? "",
      targetInflasi: str(b.target_inflasi),
      labelTarget: str(b.label_target),
      klasifikasi: toKlasifikasiMl(b.klasifikasi),
      regresi: toRegresiMl(b.regresi),
    };
  }
  if (raw.ews) {
    const b = raw.ews;
    out.ews = {
      ...toMeta(b),
      ringkasan: toRingkasanEws(b.ringkasan),
      sinyal: (b.sinyal ?? []).map(toSinyalEwsMl),
      terpotong: b.terpotong === true,
    };
  }
  if (raw.wilayah) {
    const b = raw.wilayah;
    out.wilayah = {
      ...toMeta(b),
      items: (b.items ?? []).map((w) => ({
        wilayahKode: w.wilayah_kode,
        tingkat:
          w.tingkat ?? (w.wilayah_kode.length <= 2 ? "PROVINSI" : "KABUPATEN"),
        komoditasForecast: [...(w.komoditas_forecast ?? [])],
        inflasi: w.inflasi
          ? {
              kelas: str(w.inflasi.kelas),
              kelayakan: toKelayakan(w.inflasi.kelayakan),
              probabilitas: num(w.inflasi.probabilitas),
              skorTekanan: num(w.inflasi.skor_tekanan),
              siklus: str(w.inflasi.siklus),
              target: str(w.inflasi.target),
              labelTarget: str(w.inflasi.label_target),
            }
          : null,
        sinyalAktif: toRingkasanEws(w.sinyal_aktif),
      })),
    };
  }
  if (raw.kesegaran) {
    const b = raw.kesegaran;
    const r = b.ringkasan;
    out.kesegaran = {
      ...toMeta(b),
      dihitungPada: str(b.dihitung_pada),
      ringkasan: {
        segar: r?.segar ?? 0,
        basi: r?.basi ?? 0,
        gagal: r?.gagal ?? 0,
        dilewati: r?.dilewati ?? 0,
        belumAdaRunTerjadwal: r?.belum_ada_run_terjadwal ?? 0,
      },
      sumber: (b.sumber ?? []).map((s) => ({
        sumber: s.sumber,
        dataset: s.dataset,
        kategori: toKategori(s.kategori),
        status: str(s.status),
        pesan: s.pesan ?? "",
        lastAvailableDate: str(s.last_available_date),
        umurHari: num(s.umur_hari),
      })),
      sinyalSumber: (b.sinyal_sumber ?? []).map(toSinyalEwsMl),
    };
  }
  if (raw.makro) {
    const b = raw.makro;
    out.makro = {
      ...toMeta(b),
      items: (b.items ?? []).map((m) => ({
        domain: m.domain,
        indikator: m.indikator,
        wilayahKode: m.wilayah_kode,
        periode: m.periode,
        nilai: num(m.nilai),
        satuan: m.satuan,
        perubahan: num(m.perubahan),
        sumber: m.sumber,
        isCarriedForward: m.is_carried_forward,
        lastAvailableDate: m.last_available_date,
      })),
    };
  }
  return out;
}

export function toSiklusMl(s: SiklusMlMentah): SiklusMl {
  return {
    cycleId: s.cycle_id,
    cycleTimestamp: s.cycle_timestamp,
    dataCutoff: str(s.data_cutoff),
    status: s.status,
    final: s.final ?? STATUS_FINAL.has(s.status),
    durationMs: num(s.duration_ms),
    pesan: s.pesan ?? "",
    versiKode: s.versi_kode ?? "",
    jumlah: { ...(s.jumlah ?? {}) },
    targetInflasi: str(s.target_inflasi),
    labelTarget: str(s.label_target),
    klasifikasi: toKlasifikasiMl(s.model_metrics?.klasifikasi),
    regresi: toRegresiMl(s.model_metrics?.regresi),
  };
}

/** Daftar siklus terbaru dahulu (urutan server dipertahankan, lalu dipastikan menurut waktu siklus). */
export function toDaftarSiklusMl(
  raw: DaftarSiklusMlMentah | null | undefined,
): DaftarSiklusMl {
  if (!raw)
    return {
      tersedia: false,
      alasan: "TIDAK_ADA_RESPONS",
      items: [],
      total: 0,
    };
  const items = (raw.items ?? [])
    .map(toSiklusMl)
    .sort(
      (a, b) => Date.parse(b.cycleTimestamp) - Date.parse(a.cycleTimestamp),
    );
  return {
    tersedia: raw.tersedia === true,
    alasan: str(raw.alasan),
    items,
    total: raw.total ?? items.length,
  };
}

/** Siklus final terbaru: yang dipakai server bila `cycle_id` kosong. */
export function siklusFinalTerbaru(
  daftar: DaftarSiklusMl | undefined,
): SiklusMl | undefined {
  return daftar?.items.find((s) => s.final);
}

// ── `cycle_id` ───────────────────────────────────────────────────────────────

const POLA_CYCLE_ID = /^C\d{8}T\d{2}Z$/;

/** `cycle_id` berformat `C{YYYYMMDD}T{HH}Z`; selain itu dianggap tidak ada. */
export function cycleIdSah(v: string | null | undefined): string | undefined {
  const s = v?.trim();
  return s && POLA_CYCLE_ID.test(s) ? s : undefined;
}

/** Menambahkan `cycle_id` ke path API (kosong = siklus final terbaru, path tidak berubah). */
export function denganCycleId(path: string, cycleId?: string): string {
  const id = cycleIdSah(cycleId);
  if (!id) return path;
  return `${path}${path.includes("?") ? "&" : "?"}cycle_id=${encodeURIComponent(id)}`;
}

// ── Akses API ────────────────────────────────────────────────────────────────

const MOCK_TIDAK_TERSEDIA = "TIDAK_DIKONFIGURASI";

/**
 * Membaca lampiran `ml` sebuah GET endpoint lama (dengan `?cycle_id=` opsional). `data` di sampingnya diabaikan:
 * halaman tetap memuat `data` lewat hook lamanya, sehingga perilaku pemanggil lama tidak berubah.
 */
export async function bacaLampiranMl(
  path: string,
  cycleId?: string,
): Promise<LampiranMl> {
  if (MODE_MOCK)
    return {
      tersedia: false,
      alasan: MOCK_TIDAK_TERSEDIA,
      dataUtama: "POSTGRES",
      siklus: null,
    };
  const { ml } = await apiFetchEnvelope<unknown>(denganCycleId(path, cycleId));
  return toLampiranMl(ml as LampiranMlMentah | undefined);
}

/** Riwayat siklus training, terbaru dahulu, semua status (BERJALAN dan GAGAL hanya metadata). */
export async function daftarSiklusMl(limit = 24): Promise<DaftarSiklusMl> {
  if (MODE_MOCK)
    return {
      tersedia: false,
      alasan: MOCK_TIDAK_TERSEDIA,
      items: [],
      total: 0,
    };
  return toDaftarSiklusMl(
    await apiFetch<DaftarSiklusMlMentah>(`/v1/ml/cycles?limit=${limit}`),
  );
}
