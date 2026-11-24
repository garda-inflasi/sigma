/**
 * Label kejujuran hasil training (docs/PIPELINE-DATA-ML.md §5.3 "Aturan tampilan untuk UI"). Semua teks yang
 * menerangkan target, kelayakan, model pemenang, wilayah tanpa forecast, dan kesegaran sumber berasal dari sini,
 * supaya tidak ada halaman yang menyebut indeks pangan sebagai inflasi atau menampilkan klasifikasi yang belum layak
 * sebagai prediksi.
 */
import type {
  BlokModelMl,
  InflasiWilayahMl,
  KelayakanMl,
  LampiranMl,
  MetaBlokMl,
  ModelForecastMl,
  RegresiMl,
  SatuanKontribusi,
  SiklusMl,
  SumberKesegaranMl,
} from "@/lib/api/ml";
import { KABUPATEN, KOMODITAS } from "@/lib/data/reference";
import type { Tone } from "@/lib/design/tokens";

export const LABEL_INDEKS_PANGAN =
  "Indeks harga pangan (bukan inflasi resmi BPS)";
export const LABEL_INFLASI_RESMI = "Inflasi resmi BPS";
export const LABEL_BELUM_ANDAL = "Model belum dapat diandalkan";
export const LABEL_PERSISTENSI =
  "Persistensi (belum ada model yang mengalahkan baseline)";
export const LABEL_WILAYAH_TANPA_DATA =
  "Belum ada data harga resmi untuk wilayah ini";

/** Nama target sinyal inflasi. Selain RESMI_BPS selalu indeks pangan: tidak pernah disebut "inflasi". */
export function labelTargetMl(target: string | null | undefined): string {
  if (target === "RESMI_BPS") return LABEL_INFLASI_RESMI;
  if (!target) return "Target belum ditetapkan siklus ini";
  return LABEL_INDEKS_PANGAN;
}

/** Kelas, skor tekanan, dan siklus hanya boleh tampil sebagai prediksi bila model klasifikasinya LAYAK. */
export const dapatDiandalkan = (kelayakan: KelayakanMl | null | undefined) =>
  kelayakan === "LAYAK";

/** Alasan "belum dapat diandalkan" yang ditampilkan bersama labelnya. */
export function alasanBelumAndal(
  kelayakan: KelayakanMl,
  alasan?: string | null,
): string {
  const a = alasan?.trim();
  if (kelayakan === "TIDAK_LAYAK") {
    return a
      ? `Tidak lolos uji kelayakan: ${a}`
      : "Tidak lolos uji kelayakan (ROC-AUC holdout ≥ 0,60 dan Brier lebih rendah dari tebakan dasar).";
  }
  return a
    ? `Kelayakan belum diuji: ${a}`
    : "Kelayakan belum diuji: siklus ini dibuat sebelum uji kelayakan dicatat.";
}

const NAMA_MODEL: Record<string, string> = {
  lightgbm_quantile: "LightGBM kuantil",
  hgb_quantile: "HistGradientBoosting kuantil",
  sarimax: "SARIMAX",
  holt_winters: "Holt-Winters",
  lightgbm: "LightGBM",
  naive: "Persistensi (naive)",
  seasonal_naive: "Persistensi musiman",
};

export const namaModel = (nama: string | null | undefined) =>
  nama ? (NAMA_MODEL[nama] ?? nama) : "Tidak diketahui";

/**
 * Narasi pipeline menyebut nama teknis model ("(model seasonal_naive)"); bila yang dipublikasikan baseline, sebutan itu
 * diganti persistensi supaya narasi tidak tampak seperti model terlatih.
 */
export function narasiJujur(
  narasi: string,
  model: Pick<ModelForecastMl, "adalahBaseline">,
): string {
  if (!model.adalahBaseline) return narasi;
  return narasi.replace(
    /\(model (?:seasonal_naive|naive)\)/g,
    "(persistensi, belum ada model yang mengalahkan baseline)",
  );
}

/** Model pemenang forecast; baseline ditulis jujur sebagai persistensi. */
export function labelModelForecast(
  model: Pick<ModelForecastMl, "nama" | "adalahBaseline">,
): string {
  return model.adalahBaseline ? LABEL_PERSISTENSI : namaModel(model.nama);
}

const BASELINE = new Set(["naive", "seasonal_naive"]);

/** Ringkasan pemenang regresi siklus: berapa seri yang dimenangi persistensi dan berapa yang mengalahkannya. */
export function ringkasPemenang(
  r: RegresiMl | null | undefined,
): { label: string; semuaBaseline: boolean } | null {
  if (!r) return null;
  const total = r.nSeri ?? Object.values(r.pemenang).reduce((a, b) => a + b, 0);
  if (total <= 0) return null;
  const nBaseline = Object.entries(r.pemenang)
    .filter(([k]) => BASELINE.has(k))
    .reduce((a, [, v]) => a + v, 0);
  const unggul = r.mengalahkanBaseline ?? total - nBaseline;
  if (unggul <= 0)
    return {
      label: `${LABEL_PERSISTENSI}, ${total} seri`,
      semuaBaseline: true,
    };
  return {
    label: `${unggul} dari ${total} seri mengalahkan baseline; sisanya persistensi`,
    semuaBaseline: false,
  };
}

export function labelSatuanKontribusi(
  s: SatuanKontribusi | null | undefined,
): string {
  if (s === "poin_persen_harga") return "poin persen harga";
  if (s === "poin_persen_probabilitas") return "poin persen probabilitas";
  return "satuan tidak tercatat, format lama";
}

const ALASAN: Record<string, string> = {
  TIDAK_DIKONFIGURASI: "Pipeline ML belum dikonfigurasi di server.",
  KONFIGURASI_TIDAK_VALID: "Konfigurasi pipeline ML di server tidak valid.",
  MONGO_TIDAK_TERSEDIA:
    "Penyimpanan hasil training sedang tidak dapat dihubungi.",
  MONGO_LAMBAT: "Penyimpanan hasil training terlalu lambat menjawab.",
  BELUM_ADA_SIKLUS: "Belum ada siklus training yang selesai.",
  CYCLE_ID_TIDAK_VALID: "Kode siklus tidak valid.",
  SIKLUS_TIDAK_DITEMUKAN: "Siklus tidak ditemukan.",
  SIKLUS_BELUM_FINAL:
    "Siklus masih berjalan atau gagal, isinya belum boleh dibaca.",
  TANPA_IZIN: "Anda tidak punya izin melihat blok ini.",
  CAKUPAN_TIDAK_DAPAT_DIPERIKSA: "Cakupan wilayah Anda tidak dapat diperiksa.",
  TIDAK_ADA_DATA: "Siklus ini tidak punya data untuk blok ini.",
  WILAYAH_TANPA_DATA: LABEL_WILAYAH_TANPA_DATA,
  LUAR_CAKUPAN: "Wilayah ini di luar cakupan tugas Anda.",
  MODE_STAGING: "Mode Staging hanya memakai data demo PostgreSQL.",
};

export const labelAlasanMl = (alasan: string | null | undefined) =>
  (alasan && ALASAN[alasan]) || "Hasil training tidak tersedia.";

/** Alasan yang berarti hasil training tidak bisa dibaca sama sekali: bagian ML disembunyikan rapi. */
const ALASAN_SEMBUNYI = new Set([
  "TIDAK_DIKONFIGURASI",
  "KONFIGURASI_TIDAK_VALID",
  "MONGO_TIDAK_TERSEDIA",
  "MONGO_LAMBAT",
  "MODE_STAGING",
  "TIDAK_ADA_LAMPIRAN",
  "TANPA_IZIN",
  "CAKUPAN_TIDAK_DAPAT_DIPERIKSA",
]);
/** Alasan milik `cycle_id` yang dipilih: tampil sebagai pemberitahuan dengan pilihan kembali ke terbaru. */
const ALASAN_SIKLUS = new Set([
  "CYCLE_ID_TIDAK_VALID",
  "SIKLUS_TIDAK_DITEMUKAN",
  "SIKLUS_BELUM_FINAL",
]);

export type KeadaanBlok<B> =
  | { jenis: "memuat" }
  | { jenis: "sembunyi" }
  | { jenis: "siklus"; pesan: string }
  | { jenis: "kosong"; pesan: string; alasan: string | null; blok?: B }
  | { jenis: "tampil"; blok: B };

/**
 * Keputusan tampil satu blok. Hanya `tampil` yang boleh berisi angka (aturan 1 §5.3). `sembunyi` bila hasil
 * training tidak bisa dibaca (tanpa Mongo, mode staging, tanpa izin), `siklus` bila `cycle_id` pilihan tidak
 * dapat dipakai, dan `kosong` bila siklus tidak punya data untuk blok atau wilayah ini.
 */
export function keadaanBlok<B extends MetaBlokMl>(
  lampiran: LampiranMl | undefined,
  blok: B | undefined,
  memuat = false,
): KeadaanBlok<B> {
  if (!lampiran) return memuat ? { jenis: "memuat" } : { jenis: "sembunyi" };
  if (!lampiran.tersedia) {
    if (lampiran.alasan && ALASAN_SIKLUS.has(lampiran.alasan))
      return { jenis: "siklus", pesan: labelAlasanMl(lampiran.alasan) };
    if (lampiran.alasan === "BELUM_ADA_SIKLUS")
      return {
        jenis: "kosong",
        pesan: labelAlasanMl(lampiran.alasan),
        alasan: lampiran.alasan,
      };
    return { jenis: "sembunyi" };
  }
  if (!blok) return { jenis: "sembunyi" };
  if (blok.tersedia) return { jenis: "tampil", blok };
  if (!blok.alasan || ALASAN_SEMBUNYI.has(blok.alasan))
    return { jenis: "sembunyi" };
  if (ALASAN_SIKLUS.has(blok.alasan))
    return { jenis: "siklus", pesan: labelAlasanMl(blok.alasan) };
  return {
    jenis: "kosong",
    pesan: labelAlasanMl(blok.alasan),
    alasan: blok.alasan,
    blok,
  };
}

const STATUS_SIKLUS: Record<string, { label: string; tone: Tone }> = {
  BERHASIL: { label: "Berhasil", tone: "normal" },
  SEBAGIAN: { label: "Sebagian", tone: "waspada" },
  BERJALAN: { label: "Berjalan", tone: "data" },
  GAGAL: { label: "Gagal", tone: "defisit" },
};

export const statusSiklus = (status: string) =>
  STATUS_SIKLUS[status] ?? { label: status, tone: "netral" as Tone };

const formatWib = new Intl.DateTimeFormat("id-ID", {
  timeZone: "Asia/Jakarta",
  day: "numeric",
  month: "short",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
  hourCycle: "h23",
});
const formatWibPendek = new Intl.DateTimeFormat("id-ID", {
  timeZone: "Asia/Jakarta",
  day: "numeric",
  month: "short",
  hour: "2-digit",
  minute: "2-digit",
  hourCycle: "h23",
});

/** Waktu siklus (RFC3339 UTC) dalam WIB, mis. "24 Nov 2026 11.00 WIB", apa pun zona waktu peramban. */
export function waktuWib(
  iso: string | null | undefined,
  pendek = false,
): string {
  if (!iso) return "–";
  const t = new Date(iso);
  if (Number.isNaN(t.getTime())) return "–";
  return `${(pendek ? formatWibPendek : formatWib).format(t).replace(",", "")} WIB`;
}

const formatTanggalBisnis = new Intl.DateTimeFormat("id-ID", {
  timeZone: "UTC",
  day: "numeric",
  month: "short",
  year: "numeric",
});

/** Tanggal bisnis `YYYY-MM-DD` (tengah malam UTC dari tanggal WIB) tanpa geser zona waktu. */
export function tanggalBisnis(tanggal: string | null | undefined): string {
  if (!tanggal) return "tidak diketahui";
  const t = new Date(`${tanggal.slice(0, 10)}T00:00:00Z`);
  return Number.isNaN(t.getTime()) ? tanggal : formatTanggalBisnis.format(t);
}

/** Model yang dipakai siklus, ringkas untuk pemilih siklus. */
export function ringkasModelSiklus(
  s: Pick<SiklusMl, "regresi" | "klasifikasi" | "final" | "status">,
): string {
  if (!s.final)
    return s.status === "GAGAL" ? "Gagal, tanpa hasil" : "Belum selesai";
  const p = ringkasPemenang(s.regresi);
  if (!p) return "Tanpa forecast";
  return p.semuaBaseline ? "Persistensi (baseline)" : p.label;
}

/** Status kesegaran sumber; sumber basi, dilewati, atau gagal selalu menyebut tanggal data asli terakhirnya. */
export function labelKesegaran(
  s: Pick<SumberKesegaranMl, "kategori" | "lastAvailableDate">,
): { label: string; tone: Tone } {
  const terakhir = `data terakhir ${tanggalBisnis(s.lastAvailableDate)}`;
  switch (s.kategori) {
    case "SEGAR":
      return { label: "Segar", tone: "normal" };
    case "BASI":
      return { label: `Basi, ${terakhir}`, tone: "waspada" };
    case "DILEWATI":
      return { label: `Dilewati, ${terakhir}`, tone: "netral" };
    case "BELUM_ADA_RUN_TERJADWAL":
      return { label: `Belum ada run terjadwal, ${terakhir}`, tone: "netral" };
    default:
      return { label: `Gagal, ${terakhir}`, tone: "defisit" };
  }
}

export function namaWilayahMl(kode: string | null | undefined): string {
  if (!kode) return "Tingkat sistem";
  if (kode.length <= 2) return "Provinsi Aceh";
  return KABUPATEN.find((k) => k.kode === kode)?.nama ?? `Wilayah ${kode}`;
}

export const namaKomoditasMl = (
  kode: string | null | undefined,
  cadangan?: string,
) =>
  (kode && KOMODITAS.find((k) => k.kode === kode)?.nama) ||
  cadangan ||
  kode ||
  "Komoditas";

/**
 * Sinyal tekanan satu wilayah, siap tampil. Bila model tidak LAYAK, judulnya "Model belum dapat diandalkan" beserta
 * alasannya, dan kelas, skor, maupun siklus tidak ikut ditampilkan (`andal: false`).
 */
export function sinyalTekanan(
  w: Pick<
    InflasiWilayahMl,
    "kelayakan" | "alasanKelayakan" | "kelas" | "skorTekanan" | "siklus"
  >,
): { andal: boolean; judul: string; keterangan: string; tone: Tone } {
  if (!dapatDiandalkan(w.kelayakan)) {
    return {
      andal: false,
      judul: LABEL_BELUM_ANDAL,
      keterangan: alasanBelumAndal(w.kelayakan, w.alasanKelayakan),
      tone: "netral",
    };
  }
  const naik = w.kelas === "INFLASI";
  const siklus = w.siklus ? ` · siklus ${w.siklus.toLowerCase()}` : "";
  const skor =
    w.skorTekanan !== null
      ? `skor tekanan ${Math.round(w.skorTekanan)}/100`
      : "skor tekanan tidak ada";
  return {
    andal: true,
    judul: naik
      ? "Diproyeksi naik dalam 30 hari"
      : "Tidak diproyeksi naik dalam 30 hari",
    keterangan: `${skor}${siklus}`,
    tone: naik
      ? w.siklus === "TEKANAN"
        ? "defisit"
        : "waspada"
      : w.siklus === "SURPLUS"
        ? "surplus"
        : "normal",
  };
}

/** Ringkasan blok model untuk konteks halaman (KPI): kelayakan klasifikasi dan pemenang regresi. */
export function ringkasBlokModel(m: BlokModelMl): {
  klasifikasi: string;
  regresi: string | null;
} {
  const k = m.klasifikasi;
  const target = labelTargetMl(m.targetInflasi);
  const klasifikasi = !k
    ? `${target}: metrik klasifikasi tidak ada.`
    : dapatDiandalkan(k.kelayakan)
      ? `${target}: model klasifikasi layak dipakai.`
      : `${target}: ${LABEL_BELUM_ANDAL} (${alasanBelumAndal(k.kelayakan, k.alasan)})`;
  return { klasifikasi, regresi: ringkasPemenang(m.regresi)?.label ?? null };
}
