/**
 * Pemetaan policy brief dari representasi API Go (snake_case) ke model UI.
 * Endpoint: GET /v1/policy-brief, POST /v1/policy-brief/generate.
 */

import type { PolicyBrief } from "@/lib/data/types";

export interface PolicyBriefResponse {
  id: string;
  periode: string;
  judul: string;
  status: string;
  ringkasan: string;
  disusun_oleh: string;
  jumlah_rekomendasi: number;
  halaman: number;
  dibuat_pada: string;
  diperbarui: string;
}

const STATUS: readonly PolicyBrief["status"][] = [
  "DRAF",
  "TELAAH",
  "DISETUJUI",
  "DIDISTRIBUSIKAN",
];

function toStatus(status: string): PolicyBrief["status"] {
  return (STATUS as readonly string[]).includes(status)
    ? (status as PolicyBrief["status"])
    : "DRAF";
}

export function toPolicyBrief(b: PolicyBriefResponse): PolicyBrief {
  return {
    id: b.id,
    periode: b.periode,
    judul: b.judul,
    status: toStatus(b.status),
    ringkasan: b.ringkasan,
    disusunOleh: b.disusun_oleh,
    diperbarui: b.diperbarui,
    jumlahRekomendasi: b.jumlah_rekomendasi,
    halaman: b.halaman,
  };
}

/** Periode bulan berjalan dalam format API (YYYY-MM), memakai zona waktu perangkat. */
export function periodeBulanIni(now: Date = new Date()): string {
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`;
}
