import {
  BrainCircuit, Workflow, ClipboardCheck, Database, FileText, Gauge, Landmark, Map, PlugZap, ScrollText, Siren, Smartphone,
  Snowflake, SquareKanban, Target, Users, type LucideIcon,
} from "lucide-react";
import type { Permission } from "@/lib/rbac/permissions";

export interface NavItem {
  href: string;
  label: string;
  icon: LucideIcon;
  /** Tanpa izin = terbuka untuk seluruh pengguna konsol */
  perm?: Permission;
  badge?: "ews" | "validasi";
  keywords?: string[];
}

/** Navigasi dikelompokkan mengikuti 4 lapis arsitektur + tata kelola. */
export const NAV: { grup: string; items: NavItem[] }[] = [
  {
    grup: "Ringkasan",
    items: [
      { href: "/eksekutif", label: "Dashboard eksekutif", icon: Landmark, perm: "dashboard.eksekutif", keywords: ["inflasi", "neraca", "fiskal"] },
      { href: "/operasional", label: "Dashboard operasional", icon: Gauge, perm: "dashboard.operasional", keywords: ["kpi", "heatmap", "peta"] },
    ],
  },
  {
    grup: "Data",
    items: [
      { href: "/data/harga", label: "Validasi harga", icon: ClipboardCheck, perm: "harga.lihat", badge: "validasi", keywords: ["entri", "lapangan", "verifikasi"] },
      { href: "/sumber-data", label: "Sumber data & API", icon: PlugZap, perm: "sumber.lihat", keywords: ["bps", "bi", "bmkg", "scraper"] },
      { href: "/master/komoditas", label: "Master data", icon: Database, perm: "master.lihat", keywords: ["komoditas", "wilayah", "pasar"] },
    ],
  },
  {
    grup: "Analitik AI",
    items: [
      { href: "/ews", label: "Peringatan dini", icon: Siren, perm: "ews.lihat", badge: "ews", keywords: ["ews", "alert", "alarm"] },
      { href: "/prediksi", label: "Prediksi & XAI", icon: BrainCircuit, perm: "prediksi.lihat", keywords: ["proyeksi", "model", "explainable"] },
      { href: "/peta", label: "Peta surplus–defisit", icon: Map, perm: "spasial.lihat", keywords: ["spasial", "rute", "distribusi"] },
      { href: "/cold-storage", label: "Cold storage & food loss", icon: Snowflake, perm: "coldstorage.lihat", keywords: ["gudang", "shelf-life", "stok"] },
    ],
  },
  {
    grup: "Keputusan",
    items: [
      { href: "/intervensi", label: "Intervensi", icon: SquareKanban, perm: "intervensi.lihat", keywords: ["kanban", "siklus", "workflow"] },
      { href: "/policy-brief", label: "Policy brief", icon: FileText, perm: "brief.lihat", keywords: ["rekomendasi", "laporan"] },
      { href: "/kpi", label: "Indikator kinerja", icon: Target, perm: "kpi.lihat", keywords: ["kpi", "target"] },
    ],
  },
  {
    grup: "Tata kelola",
    items: [
      { href: "/pengguna", label: "Pengguna & peran", icon: Users, perm: "pengguna.kelola", keywords: ["rbac", "akses", "mfa"] },
      { href: "/audit", label: "Audit trail", icon: ScrollText, perm: "audit.lihat", keywords: ["log", "jejak"] },
      { href: "/alur-sistem", label: "Alur sistem & data", icon: Workflow, keywords: ["flow", "arsitektur", "pipeline"] },
    ],
  },
  {
    grup: "Lapangan",
    items: [{ href: "/lapangan/input", label: "Aplikasi lapangan", icon: Smartphone, perm: "harga.input", keywords: ["mobile", "input harga"] }],
  },
];

export const NAV_FLAT = NAV.flatMap((g) => g.items);

export function activeNav(pathname: string, items = NAV_FLAT) {
  return items.filter((i) => pathname === i.href || pathname.startsWith(`${i.href.split("/").slice(0, 2).join("/")}/`) || pathname.startsWith(`${i.href}/`))
    .sort((a, b) => b.href.length - a.href.length)[0];
}
