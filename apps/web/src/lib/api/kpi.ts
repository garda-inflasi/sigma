/**
 * Indikator kinerja (KPI): pemetaan dari representasi API Go (snake_case) ke model UI.
 * Endpoint: GET /v1/kpi dan GET /v1/kpi/{kode}/riwayat.
 */

export type KpiKelompok =
  "DATA_SISTEM" | "LOGISTIK" | "DAMPAK_EKONOMI" | "TINDAK_LANJUT";
export type KpiStatus =
  "TERCAPAI" | "WASPADA" | "TIDAK_TERCAPAI" | "TANPA_TARGET" | "MENUNGGU_DATA";

export const KPI_KELOMPOK: readonly KpiKelompok[] = [
  "DATA_SISTEM",
  "LOGISTIK",
  "DAMPAK_EKONOMI",
  "TINDAK_LANJUT",
];

export interface KpiIndikator {
  kode: string;
  kelompok: KpiKelompok;
  nama: string;
  deskripsi: string;
  rumus: string;
  satuan: string;
  naikBaik: boolean;
  /** Ada penghitung otomatis; false = menunggu model atau data lain (16-B). */
  otomatis: boolean;
  target: number | null;
  /** Nilai terbaru; null = menunggu data. */
  nilai: number | null;
  /** Periode nilai terbaru (ISO lokal siang hari). */
  periode: string | null;
  /** Nilai beberapa periode terakhir, urut naik. */
  tren: number[];
  status: KpiStatus;
}

interface NilaiResponse {
  periode: string;
  nilai: number;
  versi: number;
  dihitung_pada: string;
}

export interface KpiResponse {
  kode: string;
  kelompok: string;
  nama: string;
  deskripsi: string;
  rumus: string;
  satuan: string;
  arah_baik: string;
  frekuensi: string;
  otomatis: boolean;
  target: {
    nilai: number;
    ambang_waspada: number | null;
    baseline: number | null;
    periode_mulai: string;
  } | null;
  nilai: NilaiResponse | null;
  tren: NilaiResponse[];
  status: string;
}

const STATUS: readonly KpiStatus[] = [
  "TERCAPAI",
  "WASPADA",
  "TIDAK_TERCAPAI",
  "TANPA_TARGET",
  "MENUNGGU_DATA",
];

export function toKpiIndikator(k: KpiResponse): KpiIndikator {
  return {
    kode: k.kode,
    kelompok: (KPI_KELOMPOK as readonly string[]).includes(k.kelompok)
      ? (k.kelompok as KpiKelompok)
      : "DATA_SISTEM",
    nama: k.nama,
    deskripsi: k.deskripsi,
    rumus: k.rumus,
    satuan: k.satuan,
    naikBaik: k.arah_baik !== "TURUN",
    otomatis: k.otomatis,
    target: k.target?.nilai ?? null,
    nilai: k.nilai?.nilai ?? null,
    // Siang hari lokal agar tanggal tidak bergeser oleh zona waktu.
    periode: k.nilai ? `${k.nilai.periode.slice(0, 10)}T12:00:00` : null,
    tren: k.tren.map((t) => t.nilai),
    // Status tak dikenal diperlakukan sebagai menunggu data, bukan capaian palsu.
    status: (STATUS as readonly string[]).includes(k.status)
      ? (k.status as KpiStatus)
      : "MENUNGGU_DATA",
  };
}

/** Satu baris riwayat: nilai berlaku sebuah periode dan capaiannya terhadap target saat ini. */
export interface KpiRiwayatBaris {
  /** Periode (ISO lokal siang hari). */
  periode: string;
  nilai: number;
  /** Versi nilai; di atas 1 berarti sudah direvisi. */
  versi: number;
  dihitungPada: string;
  status: KpiStatus;
}

export interface KpiRiwayat {
  kpi: KpiIndikator;
  /** Urut naik menurut periode (terlama dulu). */
  baris: KpiRiwayatBaris[];
  /** Ambang waspada target saat ini; null bila tidak ada. */
  ambangWaspada: number | null;
}

export interface KpiRiwayatResponse {
  kpi: KpiResponse;
  riwayat: NilaiResponse[];
}

/**
 * Menilai satu nilai terhadap target, aturan yang sama dengan `domain.StatusCapaian` di API:
 * NAIK tercapai bila nilai >= target, TURUN bila nilai <= target; ambang waspada adalah batas longgar.
 */
export function statusTerhadapTarget(
  naikBaik: boolean,
  nilai: number,
  target: number | null,
  ambangWaspada: number | null,
): KpiStatus {
  if (target === null) return "TANPA_TARGET";
  const tercapai = naikBaik ? nilai >= target : nilai <= target;
  if (tercapai) return "TERCAPAI";
  const waspada =
    ambangWaspada !== null &&
    (naikBaik ? nilai >= ambangWaspada : nilai <= ambangWaspada);
  return waspada ? "WASPADA" : "TIDAK_TERCAPAI";
}

export function toKpiRiwayat(r: KpiRiwayatResponse): KpiRiwayat {
  const kpi = toKpiIndikator(r.kpi);
  const ambangWaspada = r.kpi.target?.ambang_waspada ?? null;
  return {
    kpi,
    ambangWaspada,
    baris: r.riwayat.map((n) => ({
      periode: `${n.periode.slice(0, 10)}T12:00:00`,
      nilai: n.nilai,
      versi: n.versi,
      dihitungPada: n.dihitung_pada,
      status: statusTerhadapTarget(kpi.naikBaik, n.nilai, kpi.target, ambangWaspada),
    })),
  };
}

/**
 * Persentase capaian untuk bilah progres: nilai/target bila naik lebih baik,
 * target/nilai bila turun lebih baik; dibatasi 0–100. Null bila tidak dapat dihitung.
 */
export function capaianPersen(
  k: Pick<KpiIndikator, "nilai" | "target" | "naikBaik">,
): number | null {
  if (k.nilai === null || k.target === null) return null;
  const rasio = k.naikBaik
    ? k.nilai / k.target
    : k.nilai > 0
      ? k.target / k.nilai
      : 1;
  return Number.isFinite(rasio)
    ? Math.max(0, Math.min(100, rasio * 100))
    : null;
}

/** Angka KPI dalam format Indonesia dengan satuannya ("82,1%", "4,2/5", "5,4 hari"). */
export function formatKpi(n: number, satuan: string): string {
  const angka = n.toLocaleString("id-ID", { maximumFractionDigits: 2 });
  if (!satuan) return angka;
  return satuan === "%" || satuan === "/5"
    ? `${angka}${satuan}`
    : `${angka} ${satuan}`;
}

const mock = (
  kode: string,
  kelompok: KpiKelompok,
  nama: string,
  satuan: string,
  naikBaik: boolean,
  target: number,
  tren: number[] | null,
): KpiIndikator => ({
  kode,
  kelompok,
  nama,
  deskripsi: "",
  rumus: "",
  satuan,
  naikBaik,
  otomatis: tren !== null,
  target,
  nilai: tren ? tren[tren.length - 1] : null,
  periode: tren ? "2026-10-01T12:00:00" : null,
  tren: tren ?? [],
  status: !tren
    ? "MENUNGGU_DATA"
    : (
          naikBaik
            ? tren[tren.length - 1] >= target
            : tren[tren.length - 1] <= target
        )
      ? "TERCAPAI"
      : "TIDAK_TERCAPAI",
});

/** Data contoh mode mock (tanpa backend); bentuknya sama dengan hasil toKpiIndikator. */
export function kpiMock(): KpiIndikator[] {
  return [
    mock(
      "DATA-PRODUSEN-TERVERIFIKASI",
      "DATA_SISTEM",
      "Kelengkapan produsen terverifikasi",
      "%",
      true,
      95,
      [88, 90, 91.5, 92.4, 93.8, 94.6],
    ),
    mock(
      "DATA-KELENGKAPAN-HARIAN",
      "DATA_SISTEM",
      "Kelengkapan entri harian",
      "%",
      true,
      95,
      [84, 86, 88, 89, 90.8, 91.5],
    ),
    mock(
      "DATA-MAPE-HARGA",
      "DATA_SISTEM",
      "MAPE prediksi harga",
      "%",
      false,
      5.4,
      null,
    ),
    mock(
      "LOG-UTILISASI-CS",
      "LOGISTIK",
      "Utilisasi cold storage",
      "%",
      true,
      75,
      [52, 58, 63, 69, 75, 82.1],
    ),
    mock(
      "LOG-RISIKO-FOODLOSS",
      "LOGISTIK",
      "Risiko food loss stok",
      "%",
      false,
      40,
      [48, 46, 44, 42, 41, 39.8],
    ),
    mock(
      "EKO-VOLATILITAS",
      "DAMPAK_EKONOMI",
      "Volatilitas harga produsen (CV)",
      "%",
      false,
      7.8,
      null,
    ),
    mock(
      "TL-REKOMENDASI-AI",
      "TINDAK_LANJUT",
      "Rekomendasi AI ditindaklanjuti",
      "%",
      true,
      78,
      [41, 48, 55, 60, 64, 68],
    ),
  ];
}

/** Riwayat contoh mode mock: tren indikator mock dijadikan periode bulanan berakhir 2026-10. */
export function kpiRiwayatMock(kode: string): KpiRiwayat {
  const kpi = kpiMock().find((k) => k.kode === kode);
  if (!kpi) throw new Error("Indikator tidak ditemukan.");
  const n = kpi.tren.length;
  return {
    kpi,
    ambangWaspada: null,
    baris: kpi.tren.map((nilai, i) => ({
      periode: `2026-${String(10 - (n - 1 - i)).padStart(2, "0")}-01T12:00:00`,
      nilai,
      versi: 1,
      dihitungPada: "2026-11-01T01:00:00Z",
      status: statusTerhadapTarget(kpi.naikBaik, nilai, kpi.target, null),
    })),
  };
}
