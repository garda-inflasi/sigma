/**
 * Klien server BFF → API Go untuk RBAC berbasis basis data (02-C): matriks peran dan izin langsung.
 * Hanya dipakai Route Handler (server). Izin `pengguna.kelola` ditegakkan API Go.
 */

import "server-only";

import { apiFetch } from "./api-client";

const bearer = (accessToken: string) => ({
  Authorization: `Bearer ${accessToken}`,
});

/** Nama izin domain.aksi (katalog di kode); menolak segmen path aneh sebelum sampai ke API. */
export const POLA_NAMA_IZIN = /^[a-z]+\.[a-z_]+$/;

export const rbacApiServer = {
  updateRolePermissions: (
    accessToken: string,
    peran: string,
    body: { permissions: string[]; versi: number },
  ) =>
    apiFetch<unknown>(`/roles/${encodeURIComponent(peran)}/permissions`, {
      method: "PUT",
      headers: bearer(accessToken),
      body: JSON.stringify(body),
    }),

  listUserPermissions: (accessToken: string, id: string) =>
    apiFetch<unknown>(`/users/${encodeURIComponent(id)}/permissions`, {
      method: "GET",
      headers: bearer(accessToken),
    }),

  grantUserPermission: (
    accessToken: string,
    id: string,
    body: { izin: string; alasan: string; berlaku_sampai?: string },
  ) =>
    apiFetch<unknown>(`/users/${encodeURIComponent(id)}/permissions`, {
      method: "POST",
      headers: bearer(accessToken),
      body: JSON.stringify(body),
    }),

  revokeUserPermission: (accessToken: string, id: string, izin: string) =>
    apiFetch<unknown>(
      `/users/${encodeURIComponent(id)}/permissions/${encodeURIComponent(izin)}`,
      {
        method: "DELETE",
        headers: bearer(accessToken),
      },
    ),
};
