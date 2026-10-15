import type { Role } from "./roles";

export const PERMISSIONS = [
  "dashboard.operasional",
  "dashboard.eksekutif",
  "harga.input",
  "harga.lihat",
  "harga.validasi",
  "harga.publikasi",
  "harga.ekspor",
  "tugas.lihat_sendiri",
  "tugas.kelola",
  "ews.lihat",
  "ews.akui",
  "ews.eskalasi",
  "ews.atur_ambang",
  "prediksi.lihat",
  "model.kelola",
  "spasial.lihat",
  "logistik.kelola",
  "coldstorage.lihat",
  "coldstorage.kelola",
  "fiskal.lihat",
  "intervensi.lihat",
  "intervensi.kelola",
  "intervensi.setujui",
  "intervensi.setujui_fiskal",
  "brief.lihat",
  "brief.susun",
  "brief.setujui",
  "kpi.lihat",
  "master.lihat",
  "master.ajukan",
  "master.setujui",
  "sumber.lihat",
  "sumber.kelola",
  "pengguna.kelola",
  "audit.lihat",
] as const;
export type Permission = (typeof PERMISSIONS)[number];

const BACA_STRATEGIS: Permission[] = [
  "dashboard.eksekutif", "dashboard.operasional", "harga.lihat", "harga.ekspor", "ews.lihat",
  "prediksi.lihat", "spasial.lihat", "coldstorage.lihat", "fiskal.lihat", "intervensi.lihat",
  "brief.lihat", "kpi.lihat", "master.lihat", "sumber.lihat", "audit.lihat",
];
const KEPUTUSAN_STRATEGIS: Permission[] = ["intervensi.setujui", "brief.setujui"];

/**
 * Matriks peran → izin. Prinsip:
 * - Least privilege: tiap peran hanya izin yang dibutuhkan tugasnya.
 * - Pemisahan tugas (SoD): penginput ≠ validator ≠ publisher; admin tanpa hak persetujuan.
 * - Persetujuan dukungan fiskal khusus Kanwil DJPb.
 */
export const ROLE_PERMISSIONS: Record<Role, readonly Permission[]> = {
  STRATEGIS_FMIPA: [...BACA_STRATEGIS, ...KEPUTUSAN_STRATEGIS, "master.setujui"],
  STRATEGIS_BI: [...BACA_STRATEGIS, ...KEPUTUSAN_STRATEGIS],
  STRATEGIS_BPS: [...BACA_STRATEGIS, ...KEPUTUSAN_STRATEGIS, "master.setujui"],
  STRATEGIS_DJPB: [...BACA_STRATEGIS, ...KEPUTUSAN_STRATEGIS, "intervensi.setujui_fiskal"],

  MANAJER_PROGRAM: [
    "dashboard.operasional", "dashboard.eksekutif", "harga.lihat", "harga.ekspor", "tugas.kelola",
    "ews.lihat", "ews.akui", "ews.eskalasi", "prediksi.lihat", "spasial.lihat", "logistik.kelola",
    "coldstorage.lihat", "fiskal.lihat", "intervensi.lihat", "intervensi.kelola", "brief.lihat",
    "brief.susun", "kpi.lihat", "master.lihat", "master.ajukan", "sumber.lihat", "audit.lihat",
  ],
  MANAJER_DATA: [
    "dashboard.operasional", "harga.lihat", "harga.publikasi", "harga.ekspor", "ews.lihat", "ews.akui",
    "ews.eskalasi", "ews.atur_ambang", "prediksi.lihat", "model.kelola", "spasial.lihat",
    "intervensi.lihat", "brief.lihat", "kpi.lihat", "master.lihat", "master.ajukan", "sumber.lihat",
    "sumber.kelola", "audit.lihat",
  ],
  MANAJER_KEBIJAKAN: [
    "dashboard.operasional", "dashboard.eksekutif", "harga.lihat", "ews.lihat", "prediksi.lihat",
    "spasial.lihat", "fiskal.lihat", "intervensi.lihat", "intervensi.kelola", "brief.lihat",
    "brief.susun", "kpi.lihat",
  ],
  MANAJER_PEMBIAYAAN: [
    "dashboard.operasional", "dashboard.eksekutif", "harga.lihat", "spasial.lihat", "coldstorage.lihat",
    "fiskal.lihat", "intervensi.lihat", "brief.lihat", "kpi.lihat",
  ],
  MANAJER_RISIKO: [
    "dashboard.operasional", "harga.lihat", "harga.publikasi", "harga.ekspor", "ews.lihat", "ews.akui",
    "ews.eskalasi", "coldstorage.lihat", "intervensi.lihat", "kpi.lihat", "master.lihat", "audit.lihat",
  ],
  MANAJER_MUTU: [
    "dashboard.operasional", "harga.lihat", "ews.lihat", "prediksi.lihat", "coldstorage.lihat",
    "coldstorage.kelola", "intervensi.lihat", "intervensi.kelola", "kpi.lihat",
  ],

  OPS_KOORDINATOR: [
    "dashboard.operasional", "harga.input", "harga.lihat", "harga.validasi", "tugas.kelola",
    "tugas.lihat_sendiri", "ews.lihat", "ews.akui", "spasial.lihat", "intervensi.lihat", "master.lihat",
  ],
  OPS_AGREGATOR: [
    "dashboard.operasional", "harga.input", "harga.lihat", "tugas.lihat_sendiri", "ews.lihat",
    "coldstorage.lihat", "coldstorage.kelola", "logistik.kelola", "intervensi.lihat",
  ],
  OPS_MAHASISWA: ["harga.input", "tugas.lihat_sendiri"],

  ADMIN_SISTEM: ["pengguna.kelola", "master.lihat", "master.ajukan", "sumber.lihat", "sumber.kelola", "audit.lihat"],
};

const cache = new Map<Role, Set<Permission>>();
export function can(role: Role | undefined, perm: Permission): boolean {
  if (!role) return false;
  let set = cache.get(role);
  if (!set) {
    set = new Set(ROLE_PERMISSIONS[role]);
    cache.set(role, set);
  }
  return set.has(perm);
}

export function canAll(role: Role | undefined, perms: Permission[]) {
  return perms.every((p) => can(role, p));
}
