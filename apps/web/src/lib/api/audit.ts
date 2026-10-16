/**
 * Pemetaan audit trail dari representasi API Go (snake_case) ke model UI.
 * Backend mengirim `aktor_id`/`entitas_id`; UI memakai `aktor`/`entitasId`.
 */

import type { AuditRow } from "@/lib/data/types";

/** Bentuk data audit sebagaimana dikirim backend Go. */
export interface AuditLogResponse {
  id: string;
  waktu: string;
  aktor_id?: string;
  aksi: string;
  entitas: string;
  entitas_id: string;
  hasil: string;
  ip?: string;
  metadata?: string;
}

/** AuditLogResponse (API) → AuditRow (UI). */
export function toAuditRow(a: AuditLogResponse): AuditRow {
  return {
    id: a.id,
    waktu: a.waktu,
    aktor: a.aktor_id ?? "Sistem",
    peran: "",
    aksi: a.aksi,
    entitas: a.entitas,
    entitasId: a.entitas_id,
    hasil: a.hasil === "DITOLAK" ? "DITOLAK" : "BERHASIL",
    ip: a.ip ?? "",
  };
}