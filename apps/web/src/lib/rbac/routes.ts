import type { Permission, PemegangIzin } from "./permissions";
import { can } from "./permissions";
import type { Role } from "./roles";

/** Prefix rute → izin minimum. Dibaca oleh proxy.ts (server) dan sidebar (klien). */
export const ROUTE_ACCESS: { prefix: string; perm: Permission }[] = [
  { prefix: "/operasional", perm: "dashboard.operasional" },
  { prefix: "/eksekutif", perm: "dashboard.eksekutif" },
  { prefix: "/data/harga", perm: "harga.lihat" },
  { prefix: "/ews", perm: "ews.lihat" },
  { prefix: "/prediksi", perm: "prediksi.lihat" },
  { prefix: "/peta", perm: "spasial.lihat" },
  { prefix: "/cold-storage", perm: "coldstorage.lihat" },
  { prefix: "/intervensi", perm: "intervensi.lihat" },
  { prefix: "/policy-brief", perm: "brief.lihat" },
  { prefix: "/kpi", perm: "kpi.lihat" },
  { prefix: "/master", perm: "master.lihat" },
  { prefix: "/sumber-data", perm: "sumber.lihat" },
  { prefix: "/pengguna", perm: "pengguna.kelola" },
  // Halaman kebijakan dan riwayat impersonasi: eksklusif superadmin (ADMIN_SISTEM). API BFF `/api/impersonation/*`
  // tidak dijaga di edge seperti API BFF lain: API Go yang otoritatif (kebijakan dan peran), dan `stop` harus selalu
  // terjangkau dari sesi peran sasaran yang tidak punya izin apa pun atas impersonasi.
  { prefix: "/impersonasi", perm: "impersonasi.kelola" },
  { prefix: "/audit", perm: "audit.lihat" },
  { prefix: "/lapangan", perm: "harga.input" },
];

export function requiredPermission(pathname: string): Permission | null {
  const hit = ROUTE_ACCESS.filter((r) => pathname === r.prefix || pathname.startsWith(`${r.prefix}/`))
    .sort((a, b) => b.prefix.length - a.prefix.length)[0];
  return hit?.perm ?? null;
}

export function canAccessRoute(subject: Role | PemegangIzin | undefined, pathname: string) {
  if (pathname === "/command-center" || pathname.startsWith("/command-center/")) {
    return (
      can(subject, "dashboard.operasional") ||
      can(subject, "dashboard.eksekutif") ||
      can(subject, "ews.lihat")
    );
  }
  const perm = requiredPermission(pathname);
  return perm ? can(subject, perm) : true;
}
