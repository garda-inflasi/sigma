/**
 * Cermin token Garda untuk konsumen JavaScript (Recharts, Mapbox, canvas).
 * Sumber kebenaran tetap app/globals.css — jaga agar nilai di sini identik.
 */
export const garda = {
  navy: {
    50: "#f2f6fc", 100: "#e3ebf7", 200: "#c6d5ee", 300: "#9ab4df", 400: "#6a8dcb",
    500: "#4569b3", 600: "#2f5096", 700: "#233e7a", 800: "#1a2f5e", 900: "#13234a", 950: "#0b1630",
  },
  data: {
    50: "#effcfa", 100: "#cff6f1", 200: "#a0ece3", 300: "#67dbd0", 400: "#33c2b8",
    500: "#14a69e", 600: "#0c857f", 700: "#0e6a67", 800: "#105553", 900: "#124745", 950: "#032a2a",
  },
  mist: { 50: "#f7f9fc", 100: "#eff3f8", 200: "#e2e9f2", 300: "#cfd9e6", 400: "#aebdd0" },
  status: {
    normal: { base: "#10b981", soft: "#d1fae5", ink: "#047857" },
    waspada: { base: "#f59e0b", soft: "#fef3c7", ink: "#b45309" },
    defisit: { base: "#f43f5e", soft: "#ffe4e6", ink: "#be123c" },
    surplus: { base: "#3b82f6", soft: "#dbeafe", ink: "#1d4ed8" },
    kritis: { base: "#e11d48", soft: "#ffe4e6", ink: "#9f1239" },
  },
} as const;

export type Siklus = "normal" | "tekanan" | "surplus";
export type Tone = "data" | "normal" | "waspada" | "defisit" | "surplus" | "kritis" | "netral";

export const siklusMeta: Record<Siklus, { label: string; tone: Tone; color: string; deskripsi: string }> = {
  normal: { label: "Siklus normal", tone: "normal", color: garda.status.normal.base, deskripsi: "Harga dan pasokan dalam koridor wajar" },
  tekanan: { label: "Tekanan harga / defisit", tone: "defisit", color: garda.status.defisit.base, deskripsi: "EWS terpicu, perlu respons cepat" },
  surplus: { label: "Surplus pangan", tone: "surplus", color: garda.status.surplus.base, deskripsi: "Pasokan berlebih, arahkan ke cold storage" },
};

/** Kelas Tailwind statis per tone (harus literal agar terdeteksi oleh scanner). */
export const toneClass: Record<Tone, { bg: string; soft: string; text: string; ring: string; dot: string }> = {
  data: { bg: "bg-data-500", soft: "bg-data-50 dark:bg-data-500/15", text: "text-data-700 dark:text-data-300", ring: "ring-data-500/30", dot: "bg-data-500" },
  normal: { bg: "bg-normal", soft: "bg-normal-soft/70 dark:bg-normal/15", text: "text-normal-ink dark:text-emerald-300", ring: "ring-normal/30", dot: "bg-normal" },
  waspada: { bg: "bg-waspada", soft: "bg-waspada-soft/80 dark:bg-waspada/15", text: "text-waspada-ink dark:text-amber-300", ring: "ring-waspada/30", dot: "bg-waspada" },
  defisit: { bg: "bg-defisit", soft: "bg-defisit-soft/80 dark:bg-defisit/15", text: "text-defisit-ink dark:text-rose-300", ring: "ring-defisit/30", dot: "bg-defisit" },
  surplus: { bg: "bg-surplus", soft: "bg-surplus-soft/80 dark:bg-surplus/15", text: "text-surplus-ink dark:text-blue-300", ring: "ring-surplus/30", dot: "bg-surplus" },
  kritis: { bg: "bg-kritis", soft: "bg-defisit-soft dark:bg-kritis/20", text: "text-kritis dark:text-rose-300", ring: "ring-kritis/40", dot: "bg-kritis" },
  netral: { bg: "bg-mist-400", soft: "bg-mist-100 dark:bg-white/5", text: "text-muted-foreground", ring: "ring-mist-300", dot: "bg-mist-400" },
};

export type Severity = "INFO" | "WASPADA" | "SIAGA" | "KRITIS";
export const severityMeta: Record<Severity, { label: string; tone: Tone; color: string; rank: number }> = {
  INFO: { label: "Info", tone: "data", color: garda.data[500], rank: 0 },
  WASPADA: { label: "Waspada", tone: "waspada", color: garda.status.waspada.base, rank: 1 },
  SIAGA: { label: "Siaga", tone: "defisit", color: garda.status.defisit.base, rank: 2 },
  KRITIS: { label: "Kritis", tone: "kritis", color: garda.status.kritis.base, rank: 3 },
};

/** Palet seri grafik — urutan dipilih agar tetap terbaca untuk buta warna merah-hijau. */
export const chartPalette = [garda.navy[700], garda.data[500], garda.status.waspada.base, garda.status.surplus.base, garda.status.defisit.base, garda.navy[400]];

/**
 * Skala heatmap perubahan harga (Δ% vs rata-rata 7 hari).
 * Turun = biru (indikasi surplus), stabil = emerald, naik = amber → rose.
 */
export function heatTone(pct: number): { fill: string; ink: string; label: string } {
  if (pct <= -8) return { fill: "#bfdbfe", ink: "#1d4ed8", label: "Turun tajam" };
  if (pct <= -3) return { fill: "#dbeafe", ink: "#1d4ed8", label: "Turun" };
  if (pct < 3) return { fill: "#d1fae5", ink: "#047857", label: "Stabil" };
  if (pct < 7) return { fill: "#fef3c7", ink: "#b45309", label: "Naik" };
  if (pct < 12) return { fill: "#fde68a", ink: "#92400e", label: "Naik tajam" };
  return { fill: "#fecdd3", ink: "#be123c", label: "Lonjakan" };
}

export const mapConfig = {
  center: { longitude: 96.75, latitude: 4.45 },
  zoom: 6.55,
  style: "mapbox://styles/mapbox/light-v11",
  styleDark: "mapbox://styles/mapbox/dark-v11",
  bounds: [[94.9, 1.9], [98.4, 6.1]] as [[number, number], [number, number]],
};
