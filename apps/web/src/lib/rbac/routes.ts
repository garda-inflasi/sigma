import type { Permission } from "./permissions";
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
  { prefix: "/audit", perm: "audit.lihat" },
  { prefix: "/lapangan", perm: "harga.input" },
];

export function requiredPermission(pathname: string): Permission | null {
  const hit = ROUTE_ACCESS.filter((r) => pathname === r.prefix || pathname.startsWith(`${r.prefix}/`))
    .sort((a, b) => b.prefix.length - a.prefix.length)[0];
  return hit?.perm ?? null;
}

export function canAccessRoute(role: Role | undefined, pathname: string) {
  const perm = requiredPermission(pathname);
  return perm ? can(role, perm) : true;
}
