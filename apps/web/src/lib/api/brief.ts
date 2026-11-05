/**
 * Pemetaan policy brief dari representasi API Go (snake_case) ke model UI.
 * Endpoint: GET /v1/policy-brief, GET /v1/policy-brief/{id}, POST /v1/policy-brief/generate.
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
  sumber_narasi: string;
  label_draf: string;
  dibuat_pada: string;
  diperbarui: string;
}

/** Satu bagian brief (ringkasan, temuan, risiko, rekomendasi) berisi butir teks final. */
export interface BagianBrief {
  kode: string;
  judul: string;
  butir: string[];
}

/** Sitasi sumber untuk satu angka yang dipakai di narasi. */
export interface SitasiBrief {
  id: string;
  label: string;
  nilai: string;
  sumber: string;
}

export interface PolicyBriefDetailResponse extends PolicyBriefResponse {
  /** Kosong ({}) selama brief belum selesai disusun atau gagal. */
  konten: {
    label?: string;
    catatan?: string;
    bagian?: BagianBrief[];
    lampiran?: SitasiBrief[];
  };
  model_llm: string | null;
  prompt_versi: string | null;
  prompt_hash: string | null;
  galat: string | null;
}

/** Brief lengkap dengan isi, sitasi, dan jejak penyusunan (model dan prompt bila LLM dipakai). */
export interface PolicyBriefDetail extends PolicyBrief {
  bagian: BagianBrief[];
  sitasi: SitasiBrief[];
  /** Mis. alasan LLM digantikan templat. */
  catatan?: string;
  model?: string;
  promptVersi?: string;
  promptHash?: string;
  /** Alasan kegagalan penyusunan. */
  galat?: string;
}

const STATUS: readonly PolicyBrief["status"][] = [
  "DIPROSES",
  "GAGAL",
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

const SUMBER_NARASI: readonly NonNullable<PolicyBrief["sumberNarasi"]>[] = ["LLM", "TEMPLAT", "DEMO"];

function toSumberNarasi(sumber: string): PolicyBrief["sumberNarasi"] {
  return (SUMBER_NARASI as readonly string[]).includes(sumber) ? (sumber as PolicyBrief["sumberNarasi"]) : undefined;
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
    sumberNarasi: toSumberNarasi(b.sumber_narasi),
    labelDraf: b.label_draf,
  };
}

export function toPolicyBriefDetail(b: PolicyBriefDetailResponse): PolicyBriefDetail {
  return {
    ...toPolicyBrief(b),
    bagian: b.konten.bagian ?? [],
    sitasi: b.konten.lampiran ?? [],
    ...(b.konten.catatan ? { catatan: b.konten.catatan } : {}),
    ...(b.model_llm ? { model: b.model_llm } : {}),
    ...(b.prompt_versi ? { promptVersi: b.prompt_versi } : {}),
    ...(b.prompt_hash ? { promptHash: b.prompt_hash } : {}),
    ...(b.galat ? { galat: b.galat } : {}),
  };
}

/** Periode bulan berjalan dalam format API (YYYY-MM), memakai zona waktu perangkat. */
export function periodeBulanIni(now: Date = new Date()): string {
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`;
}
