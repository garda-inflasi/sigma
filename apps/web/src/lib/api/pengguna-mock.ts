/** Data mock detail pengguna dan daftar pengguna (mode tanpa backend). */

import type { PenggunaRow } from "@/lib/data/types";
import type { StateTabelPengguna } from "@/lib/pengguna/tabel";
import { ROLES, ROLE_IDS } from "@/lib/rbac/roles";
import {
  masihTerkunci,
  type HalamanPengguna,
  type PenggunaDetail,
} from "./pengguna";

export function mockPenggunaDetail(u: PenggunaRow): PenggunaDetail {
  return {
    ...u,
    kabupatenKode: undefined,
    dibuatPada: "2026-01-05T08:00:00.000Z",
  };
}

const URUTAN_STATUS = ["AKTIF", "NONAKTIF", "MENUNGGU"];

/** Nilai urut satu baris untuk kolom API; null diletakkan paling akhir (belum pernah masuk). */
function kunciUrutMock(
  u: PenggunaRow,
  kolom: NonNullable<StateTabelPengguna["sort"]>,
): string | number | null {
  switch (kolom) {
    case "nama":
      return u.nama.toLowerCase();
    case "email":
      return u.email.toLowerCase();
    case "lembaga":
      return u.lembaga.toLowerCase();
    case "peran":
      return ROLE_IDS.indexOf(u.role);
    case "status":
      return URUTAN_STATUS.indexOf(u.status);
    case "terakhir_masuk":
      return u.terakhirMasuk ? Date.parse(u.terakhirMasuk) : null;
    case "dibuat_pada":
      return u.dibuatPada ? Date.parse(u.dibuatPada) : 0;
  }
}

/**
 * Padanan sisi klien dari `GET /v1/users` untuk mode mock (tanpa backend): kata kunci, filter, urutan stabil
 * dengan tiebreaker id, dan halaman. Aturannya mengikuti API; filter `terkunci` memakai `terkunciSampai`.
 */
export function queryMockPengguna(
  all: PenggunaRow[],
  state: StateTabelPengguna,
  sekarang = Date.now(),
): HalamanPengguna {
  const cari = state.search.trim().toLowerCase();
  const cocok = all.filter((u) => {
    if (
      cari &&
      ![u.nama, u.email, u.role, u.role.replace(/_/g, " "), u.lembaga].some(
        (v) => v.toLowerCase().includes(cari),
      )
    )
      return false;
    if (state.peran && u.role !== state.peran) return false;
    if (state.status && u.status !== state.status) return false;
    if (state.lapis && ROLES[u.role].lapis !== state.lapis) return false;
    if (state.mfa && u.mfa !== (state.mfa === "true")) return false;
    if (
      state.terkunci &&
      masihTerkunci(u.terkunciSampai, sekarang) !== (state.terkunci === "true")
    )
      return false;
    return true;
  });

  const kolom = state.sort ?? "dibuat_pada";
  const arah = (state.sort ? state.arah === "desc" : true) ? -1 : 1;
  const urut = [...cocok].sort((a, b) => {
    const ka = kunciUrutMock(a, kolom);
    const kb = kunciUrutMock(b, kolom);
    if (ka === null || kb === null) {
      if (ka !== kb) return ka === null ? 1 : -1; // belum pernah masuk selalu di akhir
    } else if (ka !== kb) {
      return (ka < kb ? -1 : 1) * arah;
    }
    return a.id < b.id ? -arah : a.id > b.id ? arah : 0;
  });
  const mulai = (state.page - 1) * state.limit;
  return {
    rows: urut.slice(mulai, mulai + state.limit),
    total: cocok.length,
    page: state.page,
    limit: state.limit,
  };
}
