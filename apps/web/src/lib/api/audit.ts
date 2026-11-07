/**
 * Audit trail: pemetaan respons API Go (snake_case) ke model UI, kueri filter
 * sisi server dengan paginasi cursor, dan padanannya untuk mode mock.
 */

import { ROLES, type Role } from "@/lib/rbac/roles";
import type {
  AuditArsip,
  AuditDetail,
  AuditIntegritas,
  AuditIntegritasStatus,
  AuditPartisi,
  AuditPerubahan,
  AuditRow,
  AuditVerifikasi,
} from "@/lib/data/types";
import { formatAngka } from "@/lib/format";
import { API_BASE, ApiError } from "./client";
import { fetchSesi } from "@/lib/auth/sesi-client";

/** Satu baris audit sebagaimana dikirim backend Go. */
export interface AuditLogResponse {
  /** ID BIGINT ditulis sebagai string desimal; dipakai apa adanya di `GET /v1/audit/:id`. */
  id: string;
  /** ID lama (UUID teks atau demo-audit-NNNN); hanya baris yang ada sebelum migrasi 000115. Alias `id` di rute detail. */
  kode?: string;
  waktu: string;
  aktor_id?: string;
  aktor_nama?: string;
  aktor_email?: string;
  aktor_peran?: string;
  aksi: string;
  entitas: string;
  entitas_id: string;
  hasil: string;
  ip?: string;
  user_agent?: string;
  correlation_id?: string;
  metadata?: Record<string, unknown>;
}

/** `GET /v1/audit/:id`: baris audit beserta diff sebelum/sesudah. */
export interface AuditDetailResponse extends AuditLogResponse {
  perubahan: AuditPerubahan[];
}

/** Satu halaman `GET /v1/audit`; `next_cursor` kosong pada halaman terakhir. */
export interface AuditPageResponse {
  items: AuditLogResponse[];
  next_cursor?: string;
}

/** Filter yang dikirim ke server. `dari`/`sampai` berformat YYYY-MM-DD (sampai inklusif). */
export interface AuditFilter {
  dari?: string;
  sampai?: string;
  aktor?: string;
  aksi?: string;
  entitas?: string;
  hasil?: AuditRow["hasil"];
  /** ID objek persis (jejak satu objek). */
  entitas_id?: string;
  /** Semua baris dari satu permintaan. */
  correlation_id?: string;
  /** Pencarian bebas pada aksi, entitas, id entitas, aktor, dan correlation id. */
  q?: string;
}

/**
 * Filter audit dari kueri URL halaman /audit, mis. tautan "Lihat di audit trail" pada riwayat peran
 * (`/audit?entitas=pengguna&entitas_id=…`). Hanya kunci yang dikenal dan bernilai sah yang diambil; sisanya diabaikan.
 */
export function filterAuditDariQuery(params: { get(name: string): string | null }): AuditFilter {
  const filter: AuditFilter = {};
  for (const kunci of ["aktor", "aksi", "entitas", "entitas_id", "correlation_id", "q"] as const) {
    const nilai = params.get(kunci)?.trim();
    if (nilai) filter[kunci] = nilai.slice(0, 200);
  }
  for (const kunci of ["dari", "sampai"] as const) {
    const nilai = params.get(kunci);
    if (nilai && /^\d{4}-\d{2}-\d{2}$/.test(nilai)) filter[kunci] = nilai;
  }
  const hasil = params.get("hasil");
  if (hasil === "BERHASIL" || hasil === "GAGAL" || hasil === "DITOLAK") filter.hasil = hasil;
  return filter;
}

export interface AuditPage {
  rows: AuditRow[];
  nextCursor?: string;
}

export const AUDIT_PAGE_SIZE = 50;

/** Aksi yang dicatat backend; dipakai sebagai pilihan filter. */
export const AUDIT_AKSI = [
  "LOGIN",
  "LOGOUT",
  "CHANGE_PASSWORD",
  "RESET_PASSWORD",
  "RESET_PASSWORD_ADMIN",
  "CREATE_USER",
  "UPDATE_USER",
  "DELETE_USER",
  "ASSIGN_ROLE",
  "RESET_MFA",
  "UPDATE_ROLE_PERMISSIONS",
  "GRANT_USER_PERMISSION",
  "REVOKE_USER_PERMISSION",
  "BULK_aktifkan",
  "BULK_nonaktifkan",
  "BULK_wajibkan_mfa",
  "BULK_akhiri_sesi",
  "BULK_rencanakan_distribusi",
  "BULK_rekomendasi_olahan",
  "VALIDASI_HARGA",
  "PUBLIKASI_HARGA",
  "AKUI_EWS",
  "PINDAH_TAHAP",
  "SETUJUI_INTERVENSI",
  "UBAH_AMBANG_EWS",
  "EKSPOR_DATA",
  "AJUKAN_MASTER",
] as const;

const isRole = (peran: string): peran is Role => peran in ROLES;

/** AuditLogResponse (API) → AuditRow (UI). */
export function toAuditRow(a: AuditLogResponse): AuditRow {
  return {
    id: a.id,
    waktu: a.waktu,
    aktor: a.aktor_nama ?? a.aktor_email ?? "Sistem",
    peran: a.aktor_peran
      ? isRole(a.aktor_peran)
        ? ROLES[a.aktor_peran].label
        : a.aktor_peran
      : "",
    aksi: a.aksi,
    entitas: a.entitas,
    entitasId: a.entitas_id,
    hasil: a.hasil === "GAGAL" || a.hasil === "DITOLAK" ? a.hasil : "BERHASIL",
    ip: a.ip ?? "",
  };
}

export function toAuditPage(res: AuditPageResponse): AuditPage {
  return {
    rows: res.items.map(toAuditRow),
    nextCursor: res.next_cursor || undefined,
  };
}

/** AuditDetailResponse (API) → AuditDetail (UI). */
export function toAuditDetail(a: AuditDetailResponse): AuditDetail {
  return {
    ...toAuditRow(a),
    userAgent: a.user_agent ?? "",
    correlationId: a.correlation_id ?? "",
    email: a.aktor_email ?? "",
    metadata: a.metadata,
    perubahan: a.perubahan,
  };
}

/** Detail mode mock: baris yang sama tanpa keadaan sebelum/sesudah. */
export function toMockAuditDetail(row: AuditRow): AuditDetail {
  return { ...row, userAgent: "", correlationId: "", email: "", perubahan: [] };
}

/** `masalah_pertama` pada verifikasi: baris pertama yang melanggar rantai hash. */
export interface AuditMasalahResponse {
  id: string;
  /** ISI_BERUBAH | RANTAI_PUTUS | ARSIP_TIDAK_COCOK. */
  jenis: string;
  keterangan?: string | null;
}

/** Satu putaran verifikasi rantai hash. ID adalah string desimal. */
export interface AuditVerifikasiResponse {
  id: string;
  mulai: string;
  selesai?: string | null;
  /** JADWAL | MANUAL. */
  pemicu: string;
  /** VALID | RUSAK | GAGAL. */
  status: string;
  jumlah_baris: number;
  jumlah_masalah: number;
  id_awal?: string | null;
  id_akhir?: string | null;
  hash_akhir?: string | null;
  masalah_pertama?: AuditMasalahResponse | null;
  keterangan?: string | null;
}

export interface AuditPartisiResponse {
  nama: string;
  dari: string;
  sampai: string;
  /** Perkiraan perencana kueri; null bila belum pernah dianalisis. */
  perkiraan_baris?: number | null;
}

export interface AuditArsipResponse {
  id: string;
  partisi: string;
  dari: string;
  sampai: string;
  jumlah_baris: number;
  id_pertama: string;
  id_terakhir: string;
  berkas: string;
  sha256: string;
  diarsipkan_pada: string;
}

/** `GET /v1/audit/integritas`: status rantai hash, partisi aktif, dan arsip. */
export interface AuditIntegritasResponse {
  /** Null bila verifikasi belum pernah berjalan. */
  verifikasi_terakhir?: AuditVerifikasiResponse | null;
  /** Terbaru dulu. */
  partisi?: AuditPartisiResponse[] | null;
  /** Terbaru dulu; biasanya kosong. */
  arsip?: AuditArsipResponse[] | null;
}

/** Teks kosong atau null menjadi undefined; ID desimal tetap string. */
const teks = (v: string | number | null | undefined): string | undefined =>
  v === null || v === undefined || v === "" ? undefined : String(v);

const STATUS_INTEGRITAS: readonly AuditIntegritasStatus[] = ["VALID", "RUSAK", "GAGAL"];

/** AuditVerifikasiResponse (API) → AuditVerifikasi (UI). Status tak dikenal dianggap GAGAL (keadaan rantai tak pasti). */
function toAuditVerifikasi(v: AuditVerifikasiResponse): AuditVerifikasi {
  const status = STATUS_INTEGRITAS.find((s) => s === v.status) ?? "GAGAL";
  return {
    id: String(v.id),
    mulai: v.mulai,
    selesai: teks(v.selesai),
    pemicu: v.pemicu === "MANUAL" ? "MANUAL" : "JADWAL",
    status,
    jumlahBaris: Number(v.jumlah_baris) || 0,
    jumlahMasalah: Number(v.jumlah_masalah) || 0,
    idAwal: teks(v.id_awal),
    idAkhir: teks(v.id_akhir),
    hashAkhir: teks(v.hash_akhir),
    masalahPertama: v.masalah_pertama
      ? {
          id: String(v.masalah_pertama.id),
          jenis: v.masalah_pertama.jenis,
          keterangan: v.masalah_pertama.keterangan ?? "",
        }
      : undefined,
    keterangan: teks(v.keterangan),
  };
}

function toAuditPartisi(p: AuditPartisiResponse): AuditPartisi {
  return {
    nama: p.nama,
    dari: p.dari,
    sampai: p.sampai,
    perkiraanBaris: p.perkiraan_baris ?? undefined,
  };
}

function toAuditArsip(a: AuditArsipResponse): AuditArsip {
  return {
    id: String(a.id),
    partisi: a.partisi,
    dari: a.dari,
    sampai: a.sampai,
    jumlahBaris: Number(a.jumlah_baris) || 0,
    idPertama: String(a.id_pertama),
    idTerakhir: String(a.id_terakhir),
    berkas: a.berkas,
    sha256: a.sha256,
    diarsipkanPada: a.diarsipkan_pada,
  };
}

/** AuditIntegritasResponse (API) → AuditIntegritas (UI). */
export function toAuditIntegritas(res: AuditIntegritasResponse): AuditIntegritas {
  return {
    verifikasiTerakhir: res.verifikasi_terakhir
      ? toAuditVerifikasi(res.verifikasi_terakhir)
      : null,
    partisi: (res.partisi ?? []).map(toAuditPartisi),
    arsip: (res.arsip ?? []).map(toAuditArsip),
  };
}

const LABEL_MASALAH: Record<string, string> = {
  ISI_BERUBAH: "isi baris berubah",
  RANTAI_PUTUS: "rantai putus",
  ARSIP_TIDAK_COCOK: "tidak cocok dengan arsip",
};

/** Nama jenis masalah untuk manusia; jenis yang belum dikenal ditampilkan apa adanya. */
export const labelMasalahAudit = (jenis: string): string =>
  LABEL_MASALAH[jenis] ?? jenis;

/** Kalimat satu baris yang menjelaskan hasil verifikasi terakhir. */
export function penjelasanIntegritas(v: AuditVerifikasi | null): string {
  if (!v) return "Verifikasi rantai hash belum pernah dijalankan.";
  if (v.status === "VALID")
    return `Rantai hash utuh: ${formatAngka(v.jumlahBaris)} baris diverifikasi.`;
  if (v.status === "RUSAK") {
    const lain = v.jumlahMasalah - 1;
    const sisa = lain > 0 ? `; ${formatAngka(lain)} masalah lain.` : ".";
    return v.masalahPertama
      ? `Terdeteksi perubahan pada baris #${v.masalahPertama.id} (${labelMasalahAudit(v.masalahPertama.jenis)})${sisa}`
      : `Terdeteksi ${formatAngka(v.jumlahMasalah)} masalah pada rantai hash.`;
  }
  return v.keterangan
    ? `Verifikasi gagal dijalankan: ${v.keterangan}`
    : "Verifikasi gagal dijalankan.";
}

/** Jumlah partisi aktif beserta bulan tertua dan terbaru (ISO, awal partisi); kosong bila tak ada partisi. */
export function rentangPartisiAudit(partisi: AuditPartisi[]): {
  jumlah: number;
  terlama?: string;
  terbaru?: string;
} {
  const waktu = partisi
    .map((p) => p.dari)
    .sort((a, b) => Date.parse(a) - Date.parse(b));
  return { jumlah: partisi.length, terlama: waktu[0], terbaru: waktu.at(-1) };
}

/** Awal bulan (UTC) dengan selisih `offset` bulan dari `sekarang`. */
const awalBulan = (sekarang: Date, offset: number) =>
  new Date(Date.UTC(sekarang.getUTCFullYear(), sekarang.getUTCMonth() + offset, 1));

/** Padanan mode mock: VALID, tiga partisi (bulan depan, ini, lalu; terbaru dulu), tanpa arsip. */
export function mockAuditIntegritas(sekarang = new Date()): AuditIntegritas {
  const partisi = [1, 0, -1].map((offset): AuditPartisi => {
    const dari = awalBulan(sekarang, offset);
    const kode = `${dari.getUTCFullYear()}${String(dari.getUTCMonth() + 1).padStart(2, "0")}`;
    return {
      nama: `audit_log_p${kode}`,
      dari: dari.toISOString(),
      sampai: awalBulan(sekarang, offset + 1).toISOString(),
      perkiraanBaris: offset === 1 ? 0 : offset === 0 ? 120 : 1480,
    };
  });
  const selesai = new Date(sekarang.getTime() - 6 * 3_600_000);
  return {
    verifikasiTerakhir: {
      id: "12",
      mulai: new Date(selesai.getTime() - 4_000).toISOString(),
      selesai: selesai.toISOString(),
      pemicu: "JADWAL",
      status: "VALID",
      jumlahBaris: 1600,
      jumlahMasalah: 0,
      idAwal: "1",
      idAkhir: "1600",
      hashAkhir:
        "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
    },
    partisi,
    arsip: [],
  };
}

/** Query string `GET /v1/audit` dari filter dan cursor; nilai kosong dibuang. */
export function toAuditQuery(
  filter: AuditFilter,
  cursor?: string,
  limit = AUDIT_PAGE_SIZE,
): string {
  const params = new URLSearchParams({ limit: String(limit) });
  for (const [key, value] of Object.entries({ ...filter, cursor })) {
    if (value) params.set(key, value);
  }
  return `?${params}`;
}

/** Query string `GET /v1/audit/export`: hanya filter (server mengabaikan limit/cursor). */
export function toExportQuery(filter: AuditFilter): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(filter)) {
    if (value) params.set(key, value);
  }
  const query = params.toString();
  return query ? `?${query}` : "";
}

export interface HasilEkspor {
  baris: number;
  /** Server memotong hasil pada batas baris; persempit filter. */
  terpotong: boolean;
}

/** Mengunduh CSV audit sesuai filter dari server dan memicu simpan berkas. */
export async function unduhEksporAudit(
  filter: AuditFilter,
): Promise<HasilEkspor> {
  const res = await fetchSesi(
    `${API_BASE}/v1/audit/export${toExportQuery(filter)}`,
    {
      credentials: "include",
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
    download: `audit-trail-${new Date().toISOString().slice(0, 10)}.csv`,
  });
  a.click();
  URL.revokeObjectURL(url);
  return {
    baris: Number(res.headers.get("X-Export-Rows") ?? 0),
    terpotong: res.headers.get("X-Export-Truncated") === "true",
  };
}

/** Halaman daftar yang memuat objek yang dirujuk entri audit (tanpa tautan ke satu baris). */
const HALAMAN_OBJEK: Record<string, string> = {
  pengguna: "/pengguna",
  entriharga: "/data/harga",
  ewsalert: "/ews",
  aturanews: "/ews",
  intervensi: "/intervensi",
  komoditas: "/master/komoditas",
  pasar: "/master/pasar",
  wilayah: "/master/wilayah",
  stokcoldstorage: "/cold-storage",
  policybrief: "/policy-brief",
  sumberdata: "/sumber-data",
};

/**
 * Halaman tempat objek entri audit dikelola, atau undefined bila tak dikenal.
 * Nama entitas dinormalkan (huruf kecil tanpa pemisah) karena backend memakai
 * `entri_harga` sedangkan data mock memakai `EntriHarga`.
 */
export function halamanObjekAudit(entitas: string): string | undefined {
  return HALAMAN_OBJEK[entitas.toLowerCase().replace(/[^a-z0-9]/g, "")];
}

/** Menampilkan nilai JSON diff sebagai teks ringkas. */
export function formatNilaiAudit(nilai: unknown): string {
  if (nilai === undefined) return "";
  return typeof nilai === "string" ? nilai : JSON.stringify(nilai);
}

/**
 * Padanan sisi klien dari filter + paginasi server untuk mode mock. Cursor
 * mock adalah posisi baris berikutnya pada daftar terurut menurun.
 */
export function queryMockAudit(
  all: AuditRow[],
  filter: AuditFilter,
  cursor?: string,
  limit = AUDIT_PAGE_SIZE,
): AuditPage {
  const aktor = filter.aktor?.trim().toLowerCase();
  const q = filter.q?.trim().toLowerCase();
  const dari = filter.dari ? Date.parse(`${filter.dari}T00:00:00`) : undefined;
  const sampai = filter.sampai
    ? Date.parse(`${filter.sampai}T00:00:00`) + 24 * 3_600_000
    : undefined;

  const cocok = all
    .filter((r) => {
      const waktu = Date.parse(r.waktu);
      return (
        (dari === undefined || waktu >= dari) &&
        (sampai === undefined || waktu < sampai) &&
        (!aktor || r.aktor.toLowerCase().includes(aktor)) &&
        (!filter.aksi || r.aksi === filter.aksi) &&
        (!filter.entitas || r.entitas === filter.entitas) &&
        (!filter.hasil || r.hasil === filter.hasil) &&
        (!filter.entitas_id || r.entitasId === filter.entitas_id) &&
        (!q ||
          [r.aksi, r.entitas, r.entitasId, r.aktor].some((v) =>
            v.toLowerCase().includes(q),
          ))
      );
    })
    .sort((a, b) => Date.parse(b.waktu) - Date.parse(a.waktu));

  const start = cursor ? Number(cursor) : 0;
  const end = start + limit;
  return {
    rows: cocok.slice(start, end),
    nextCursor: end < cocok.length ? String(end) : undefined,
  };
}
