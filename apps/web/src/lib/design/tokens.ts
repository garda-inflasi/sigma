/**
 * Cermin token Garda untuk konsumen JavaScript (Recharts, Leaflet, canvas).
 * Sumber kebenaran tetap app/globals.css — jaga agar nilai di sini identik.
 */
export const garda = {
  navy: {
    50: "#f8f9fa", 100: "#f1f3f5", 200: "#eaecef", 300: "#d1d5db", 400: "#9ca3af",
    500: "#6b7280", 600: "#4b5563", 700: "#374151", 800: "#18181b", 900: "#111827", 950: "#09090b",
  },
  data: {
    50: "#f0fdfa", 100: "#ccfbf1", 200: "#99f6e4", 300: "#5eead4", 400: "#2dd4bf",
    500: "#14b8a6", 600: "#0d9488", 700: "#0f766e", 800: "#115e59", 900: "#134e4a", 950: "#042f2e",
  },
  mist: { 50: "#fafafb", 100: "#f8f9fa", 200: "#eaecef", 300: "#d9dce1", 400: "#b5bac3" },
  status: {
    normal: { base: "#10b981", soft: "#ecfdf5", ink: "#059669" },
    waspada: { base: "#f59e0b", soft: "#fffbeb", ink: "#d97706" },
    defisit: { base: "#ef4444", soft: "#fef2f2", ink: "#dc2626" },
    surplus: { base: "#3b82f6", soft: "#eff6ff", ink: "#2563eb" },
    kritis: { base: "#dc2626", soft: "#fef2f2", ink: "#b91c1c" },
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
  data: { bg: "bg-data-600", soft: "bg-data-50 dark:bg-data-500/15", text: "text-data-700 dark:text-data-300", ring: "ring-data-500/30", dot: "bg-data-600" },
  normal: { bg: "bg-normal", soft: "bg-normal-soft dark:bg-normal/15", text: "text-normal-ink dark:text-emerald-300", ring: "ring-normal/30", dot: "bg-normal" },
  waspada: { bg: "bg-waspada", soft: "bg-waspada-soft dark:bg-waspada/15", text: "text-waspada-ink dark:text-amber-300", ring: "ring-waspada/30", dot: "bg-waspada" },
  defisit: { bg: "bg-defisit", soft: "bg-defisit-soft dark:bg-defisit/15", text: "text-defisit-ink dark:text-rose-300", ring: "ring-defisit/30", dot: "bg-defisit" },
  surplus: { bg: "bg-surplus", soft: "bg-surplus-soft dark:bg-surplus/15", text: "text-surplus-ink dark:text-blue-300", ring: "ring-surplus/30", dot: "bg-surplus" },
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
export const chartPalette = [garda.navy[800], garda.data[600], garda.status.waspada.base, garda.status.surplus.base, garda.status.defisit.base, garda.navy[400]];

/**
 * Skala heatmap perubahan harga (Δ% vs rata-rata 7 hari).
 * Turun = biru (indikasi surplus), stabil = emerald, naik = amber → rose.
 */
export function heatTone(pct: number): { fill: string; ink: string; label: string } {
  if (pct <= -8) return { fill: "#bfdbfe", ink: "#1d4ed8", label: "Turun tajam" };
  if (pct <= -3) return { fill: "#dbeafe", ink: "#2563eb", label: "Turun" };
  if (pct < 3) return { fill: "#d1fae5", ink: "#059669", label: "Stabil" };
  if (pct < 7) return { fill: "#fef3c7", ink: "#d97706", label: "Naik" };
  if (pct < 12) return { fill: "#fde68a", ink: "#b45309", label: "Naik tajam" };
  return { fill: "#fecaca", ink: "#dc2626", label: "Lonjakan" };
}

