/**
 * Klien undang pengguna lewat email, impor CSV, dan ekspor CSV (02-D). Semua panggilan melalui Route Handler BFF
 * (`/api/pengguna/*`), sama seperti penggunaApi; pemetaan wire → model UI ada di sini supaya teruji di Node.
 */

import { fetchSesi } from "@/lib/auth/sesi-client";
import {
  toHasilImpor,
  type HasilImpor,
  type ImporWire,
} from "@/lib/pengguna/impor";
import type { UndangPenggunaInput } from "@/lib/pengguna/undang";
import type { PenggunaRow } from "@/lib/data/types";
import { ApiError, bffFetch } from "./client";
import { toPenggunaRow, type UserResponse } from "./pengguna";

/** Bentuk jawaban `POST /v1/users/undang` dan `/undang-ulang`. */
export interface UndanganWire {
  user: UserResponse;
  berlaku_sampai: string;
  email_terkirim: boolean;
}

export interface HasilUndangan {
  pengguna: PenggunaRow;
  /** Akhir masa berlaku tautan (ISO 8601). */
  berlakuSampai: string;
  /** False: akun dan undangan tersimpan tetapi email belum terkirim; kirim ulang undangannya. */
  emailTerkirim: boolean;
}

export function toHasilUndangan(w: UndanganWire): HasilUndangan {
  return {
    pengguna: toPenggunaRow(w.user),
    berlakuSampai: w.berlaku_sampai,
    emailTerkirim: w.email_terkirim,
  };
}

/** Pilihan "ekspor yang dipilih" dikirim di query string; batas sama dengan server (EksporMaksID). */
export const MAKS_ID_EKSPOR = 500;

export interface FilterEkspor {
  search?: string;
  ids?: string[];
}

/** Query string `GET /api/pengguna/ekspor`; kosong bila tanpa filter. */
export function queryEksporPengguna(filter: FilterEkspor = {}): string {
  const params = new URLSearchParams();
  const search = filter.search?.trim();
  if (search) params.set("search", search);
  if (filter.ids && filter.ids.length > 0)
    params.set("ids", filter.ids.join(","));
  const query = params.toString();
  return query ? `?${query}` : "";
}

export interface HasilEkspor {
  baris: number;
  /** Server memotong hasil pada batas baris; persempit dengan filter. */
  terpotong: boolean;
}

/** Nama berkas dari header Content-Disposition, atau bawaan bertanggal. */
export function namaBerkasEkspor(
  contentDisposition: string | null,
  sekarang: Date = new Date(),
): string {
  const cocok = contentDisposition?.match(/filename="([^"\\/]+)"/);
  return cocok?.[1] ?? `pengguna-${sekarang.toISOString().slice(0, 10)}.csv`;
}

export const undanganApi = {
  /** Membuat akun MENUNGGU dan mengirim email undangan. 409 bila email sudah terdaftar. */
  undang: async (input: UndangPenggunaInput) =>
    toHasilUndangan(
      await bffFetch<UndanganWire>("/api/pengguna/undang", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    ),

  /** Mengganti undangan akun yang masih menunggu: tautan lama hangus, tautan baru dikirim. */
  undangUlang: async (id: string) =>
    toHasilUndangan(
      await bffFetch<UndanganWire>(
        `/api/pengguna/${encodeURIComponent(id)}/undang-ulang`,
        { method: "POST" },
      ),
    ),

  /** Memeriksa (dryRun) atau mengimpor teks CSV. */
  impor: async (csv: string, dryRun: boolean): Promise<HasilImpor> =>
    toHasilImpor(
      await bffFetch<ImporWire>("/api/pengguna/impor", {
        method: "POST",
        body: JSON.stringify({ csv, dry_run: dryRun }),
      }),
    ),
};

/**
 * Mengunduh CSV pengguna dari server (tercatat sebagai EKSPOR_DATA di audit trail) dan memicu simpan berkas.
 * Tanpa filter: semua pengguna; `ids`: hanya yang dipilih.
 */
export async function unduhEksporPengguna(
  filter: FilterEkspor = {},
): Promise<HasilEkspor> {
  if ((filter.ids?.length ?? 0) > MAKS_ID_EKSPOR) {
    throw new Error(
      `Pilih paling banyak ${MAKS_ID_EKSPOR} pengguna, atau gunakan "Ekspor semua".`,
    );
  }
  const res = await fetchSesi(
    `/api/pengguna/ekspor${queryEksporPengguna(filter)}`,
    {
      credentials: "same-origin",
      headers: { Accept: "text/csv" },
    },
  );
  if (!res.ok) {
    let pesan = `Ekspor gagal (${res.status})`;
    try {
      const payload = (await res.json()) as { message?: string };
      pesan = payload.message ?? pesan;
    } catch {}
    throw new ApiError(res.status, pesan);
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = Object.assign(document.createElement("a"), {
    href: url,
    download: namaBerkasEkspor(res.headers.get("Content-Disposition")),
  });
  a.click();
  URL.revokeObjectURL(url);
  return {
    baris: Number(res.headers.get("X-Export-Rows") ?? 0),
    terpotong: res.headers.get("X-Export-Truncated") === "true",
  };
}
