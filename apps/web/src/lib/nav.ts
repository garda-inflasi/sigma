import {
  BrainCircuit,
  Workflow,
  ClipboardCheck,
  Database,
  FileText,
  Gauge,
  GitPullRequest,
  Landmark,
  Map,
  PlugZap,
  ScrollText,
  Siren,
  Smartphone,
  Snowflake,
  SquareKanban,
  Target,
  Users,
  type LucideIcon,
} from "lucide-react";
import type { Permission } from "@/lib/rbac/permissions";

export interface NavItem {
  href: string;
  label: string;
  icon: LucideIcon;
  /** Tanpa izin = terbuka untuk seluruh pengguna konsol */
  perm?: Permission;
  badge?: "ews" | "validasi" | "master";
  keywords?: string[];
}

/** Navigasi dikelompokkan mengikuti 4 lapis arsitektur + tata kelola. */
export const NAV: { grup: string; items: NavItem[] }[] = [
  {
    grup: "Ringkasan",
    items: [
      {
        href: "/eksekutif",
        label: "Dashboard eksekutif",
        icon: Landmark,
        perm: "dashboard.eksekutif",
        keywords: ["inflasi", "neraca", "fiskal"],
      },
      {
        href: "/operasional",
        label: "Dashboard operasional",
        icon: Gauge,
        perm: "dashboard.operasional",
        keywords: ["kpi", "heatmap", "peta"],
      },
    ],
  },
  {
    grup: "Data",
    items: [
      {
        href: "/data/harga",
        label: "Validasi harga",
        icon: ClipboardCheck,
        perm: "harga.lihat",
        badge: "validasi",
        keywords: ["entri", "lapangan", "verifikasi"],
      },
      {
        href: "/sumber-data",
        label: "Sumber data & API",
        icon: PlugZap,
        perm: "sumber.lihat",
        keywords: ["bps", "bi", "bmkg", "scraper"],
      },
      {
        href: "/master/komoditas",
        label: "Master data",
        icon: Database,
        perm: "master.lihat",
        keywords: [
          "komoditas",
          "wilayah",
          "pasar",
          "kecamatan",
          "satuan",
          "pedagang",
        ],
      },
      {
        href: "/master/perubahan",
        label: "Antrean perubahan master",
        icon: GitPullRequest,
        perm: "master.lihat",
        badge: "master",
        keywords: ["usulan", "persetujuan", "setujui", "tolak", "versi"],
      },
    ],
  },
  {
    grup: "Analitik AI",
    items: [
      {
        href: "/ews",
        label: "Peringatan dini",
        icon: Siren,
        perm: "ews.lihat",
        badge: "ews",
        keywords: ["ews", "alert", "alarm"],
      },
      {
        href: "/prediksi",
        label: "Prediksi & XAI",
        icon: BrainCircuit,
        perm: "prediksi.lihat",
        keywords: ["proyeksi", "model", "explainable"],
      },
      {
        href: "/peta",
        label: "Peta surplus–defisit",
        icon: Map,
        perm: "spasial.lihat",
        keywords: ["spasial", "rute", "distribusi"],
      },
      {
        href: "/cold-storage",
        label: "Cold storage & food loss",
        icon: Snowflake,
        perm: "coldstorage.lihat",
        keywords: ["gudang", "shelf-life", "stok"],
      },
    ],
  },
  {
    grup: "Keputusan",
    items: [
      {
        href: "/intervensi",
        label: "Intervensi",
        icon: SquareKanban,
        perm: "intervensi.lihat",
        keywords: ["kanban", "siklus", "workflow"],
      },
      {
        href: "/policy-brief",
        label: "Policy brief",
        icon: FileText,
        perm: "brief.lihat",
        keywords: ["rekomendasi", "laporan"],
      },
      {
        href: "/kpi",
        label: "Indikator kinerja",
        icon: Target,
        perm: "kpi.lihat",
        keywords: ["kpi", "target"],
      },
    ],
  },
  {
    grup: "Tata kelola",
    items: [
      {
        href: "/pengguna",
        label: "Pengguna & peran",
        icon: Users,
        perm: "pengguna.kelola",
        keywords: ["rbac", "akses", "mfa"],
      },
      {
        href: "/audit",
        label: "Audit trail",
        icon: ScrollText,
        perm: "audit.lihat",
        keywords: ["log", "jejak"],
      },
      {
        href: "/alur-sistem",
        label: "Alur sistem & data",
        icon: Workflow,
        keywords: ["flow", "arsitektur", "pipeline"],
      },
    ],
  },
  {
    grup: "Lapangan",
    items: [
      {
        href: "/lapangan/input",
        label: "Aplikasi lapangan",
        icon: Smartphone,
        perm: "harga.input",
        keywords: ["mobile", "input harga"],
      },
    ],
  },
];

export const NAV_FLAT = NAV.flatMap((g) => g.items);

/** Halaman konsol yang tidak punya item sidebar (dibuka dari menu akun) tetapi tetap perlu judul di topbar. */
const HALAMAN_TANPA_NAV: readonly { href: string; label: string }[] = [
  { href: "/profil", label: "Profil saya" },
];

const JUDUL_BAWAAN = "Garda Inflasi";

/**
 * Entri nav aktif untuk satu path. Kecocokan persis (path sama atau anak dari href) didahulukan; baru bila tidak
 * ada, dipakai kecocokan tingkat grup (dua segmen pertama), mis. `/master/pasar` menyalakan "Master data" yang
 * href-nya `/master/komoditas`. Tanpa urutan ini `/master/perubahan` ikut menyalakan "Master data".
 */
export function activeNav(pathname: string, items = NAV_FLAT) {
  const terpanjang = (a: NavItem, b: NavItem) => b.href.length - a.href.length;
  const persis = items
    .filter((i) => pathname === i.href || pathname.startsWith(`${i.href}/`))
    .sort(terpanjang)[0];
  if (persis) return persis;
  return items
    .filter((i) =>
      pathname.startsWith(`${i.href.split("/").slice(0, 2).join("/")}/`),
    )
    .sort(terpanjang)[0];
}

/** Judul topbar untuk satu path: item nav aktif, lalu halaman tanpa item sidebar (mis. /profil), lalu nama aplikasi. */
export function judulHalaman(pathname: string, items = NAV_FLAT): string {
  const nav = activeNav(pathname, items);
  if (nav) return nav.label;
  const lain = HALAMAN_TANPA_NAV.find(
    (h) => pathname === h.href || pathname.startsWith(`${h.href}/`),
  );
  return lain?.label ?? JUDUL_BAWAAN;
}
