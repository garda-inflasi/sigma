/**
 * Logika murni aliran EWS (SSE `GET /v1/ews/stream`). Payload event `alert` berbentuk
 * respons API (snake_case, sama dengan GET /v1/ews) dan harus dipetakan lewat `toEwsAlert`.
 */

import type { EwsAlert } from "@/lib/data/types";
import { severityMeta } from "@/lib/design/tokens";
import { toEwsAlert, type EwsAlertResponse } from "./ews";

/** Mengurai `data` event `alert`; null bila bukan JSON objek alert yang lengkap (event dibuang, bukan dilempar). */
export function parseAlertEvent(data: string): EwsAlert | null {
  let raw: unknown;
  try {
    raw = JSON.parse(data);
  } catch {
    return null;
  }
  if (!raw || typeof raw !== "object") return null;
  const r = raw as Partial<EwsAlertResponse>;
  if (
    typeof r.id !== "string" ||
    typeof r.severity !== "string" ||
    !(r.severity in severityMeta) ||
    typeof r.kabupaten_kode !== "string"
  )
    return null;
  return toEwsAlert(raw as EwsAlertResponse);
}

/** Memasukkan alert ke daftar: alert yang sudah ada diganti di tempatnya, yang baru ditaruh di depan. Daftar belum dimuat → tetap. */
export function mergeAlert(
  list: EwsAlert[] | undefined,
  alert: EwsAlert,
): EwsAlert[] | undefined {
  if (!list) return list;
  return list.some((a) => a.id === alert.id)
    ? list.map((a) => (a.id === alert.id ? alert : a))
    : [alert, ...list];
}

/** Alert layak diumumkan bila baru, atau tingkatnya naik dibanding yang sudah ada di daftar. */
export function layakDiumumkan(
  list: EwsAlert[] | undefined,
  alert: EwsAlert,
): boolean {
  const lama = list?.find((a) => a.id === alert.id);
  return (
    !lama ||
    severityMeta[alert.severity].rank > severityMeta[lama.severity].rank
  );
}
