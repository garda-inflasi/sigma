/**
 * Impersonasi peran dan akun (docs/impersonasi.md): bentuk kawat API Go (snake_case) lewat BFF, fungsi murni untuk
 * draf kebijakan dan validasi, serta klien BFF. Tipe kawat juga dipakai klien Go di lib/bff/api-client.ts.
 * Tidak mengimpor apa pun yang khusus server, jadi aman dipakai komponen klien dan tes.
 */

import { ApiError, bffFetch } from "./client";
import { ROLE_IDS, ROLES, type Role } from "@/lib/rbac/roles";

// ── Kode galat mesin (`kode` di badan galat BFF) ──────────────────────

export const KODE_VERSI_KEBIJAKAN_BERUBAH = "VERSI_KEBIJAKAN_BERUBAH";
export const KODE_TARGET_AMBIGU = "TARGET_AMBIGU";
export const KODE_TARGET_TIDAK_ADA = "TARGET_TIDAK_ADA";
export const KODE_IMPERSONASI_NONAKTIF = "IMPERSONASI_NONAKTIF";
export const KODE_IMPERSONASI_DITOLAK = "IMPERSONASI_DITOLAK";
export const KODE_ALASAN_WAJIB = "ALASAN_WAJIB";
export const KODE_IMPERSONASI_DIBATASI = "IMPERSONATION_RESTRICTED";
export const KODE_TIDAK_ADA_IMPERSONASI = "TIDAK_ADA_IMPERSONASI";

/** Kode galat mesin dari kegagalan panggilan BFF, atau undefined. */
export function kodeGalatImpersonasi(err: unknown): string | undefined {
  if (!(err instanceof ApiError)) return undefined;
  const kode = (err.payload as { kode?: unknown } | undefined)?.kode;
  return typeof kode === "string" ? kode : undefined;
}

// ── Bentuk kawat ──────────────────────────────────────────────────────

export type TipeAturan = "PERAN" | "PENGGUNA";

/** Satu target pada kebijakan (respons). Field `pengguna_*` hanya ada di respons. */
export interface TargetKebijakanWire {
  tipe: TipeAturan;
  peran?: string | null;
  pengguna_id?: string | null;
  pengguna_nama?: string | null;
  pengguna_email?: string | null;
  pengguna_peran?: string | null;
}

export interface PelakuKebijakanWire {
  tipe: TipeAturan;
  peran?: string | null;
  pengguna_id?: string | null;
  pengguna_nama?: string | null;
  pengguna_email?: string | null;
  pengguna_peran?: string | null;
  aktif: boolean;
  target: TargetKebijakanWire[];
}

export interface KebijakanWire {
  aktif: boolean;
  wajib_alasan: boolean;
  /** Menit; null = tanpa batas. */
  durasi_maks_menit: number | null;
  /** 0 bila kebijakan belum pernah disimpan (bawaan). */
  versi: number;
  diperbarui_pada: string | null;
  diperbarui_oleh?: string | null;
  diperbarui_oleh_nama?: string | null;
  pelaku: PelakuKebijakanWire[];
}

export interface TargetKebijakanInput {
  tipe: TipeAturan;
  peran?: string;
  pengguna_id?: string;
}

export interface PelakuKebijakanInput extends TargetKebijakanInput {
  aktif: boolean;
  target: TargetKebijakanInput[];
}

export interface KebijakanInput {
  versi: number;
  aktif: boolean;
  wajib_alasan: boolean;
  durasi_maks_menit: number | null;
  pelaku: PelakuKebijakanInput[];
}

export interface IdentitasRiwayatWire {
  id: string | null;
  nama: string;
  email?: string | null;
  peran: string;
}

export type AlasanBerakhir =
  | "STOP"
  | "BERGANTI"
  | "KEDALUWARSA"
  | "KEBIJAKAN"
  | "DIAKHIRI"
  | "LOGOUT"
  | "DICABUT";

export interface RiwayatImpersonasiWire {
  id: string;
  pelaku: IdentitasRiwayatWire;
  sasaran: IdentitasRiwayatWire;
  alasan: string | null;
  mulai_pada: string;
  kedaluwarsa_pada: string | null;
  berakhir_pada: string | null;
  alasan_berakhir: AlasanBerakhir | null;
  aktif: boolean;
  ip?: string | null;
  perangkat?: string | null;
}

export interface HalamanRiwayatWire {
  items: RiwayatImpersonasiWire[];
  next_cursor?: string;
}

export interface AkunTargetWire {
  id: string;
  nama: string;
  email: string;
  lembaga: string;
  kabupaten_kode?: string | null;
}

export interface PeranTargetWire {
  peran: string;
  label: string;
  lapis: string;
  akun: AkunTargetWire[];
}

export interface TargetsWire {
  boleh: boolean;
  aktif: boolean;
  wajib_alasan: boolean;
  durasi_maks_menit: number | null;
  peran: PeranTargetWire[];
}

/** `impersonasi` pada GET /auth/me dan hasil start (ada hanya saat impersonasi aktif). */
export interface ImpersonasiMetaWire {
  aktif: boolean;
  sesi_id: string;
  pelaku: { id: string; nama: string; email: string; peran: string };
  sasaran: { id: string; nama: string; peran: string };
  mulai_pada: string;
  kedaluwarsa_pada: string | null;
  alasan: string | null;
}

/** Badan permintaan mulai: `target_pengguna_id` didahulukan atas `target_peran`. */
export interface MulaiInput {
  target_pengguna_id?: string;
  target_peran?: string;
  alasan?: string;
}

/** Jawaban BFF untuk start/stop: token sudah dipasang sebagai cookie, tidak pernah sampai ke browser. */
export interface HasilAlihAkun {
  /** Profil akun efektif (bentuk UserResponse penuh; hanya field ini yang dipakai UI). */
  user: { id: string; nama: string; peran: string };
  impersonasi?: ImpersonasiMetaWire;
  /** Halaman yang dibuka sesudahnya: halaman semula bila peran barunya boleh, selain itu "/". */
  redirectTo: string;
}

// ── Konstanta dan batas (cermin validasi API Go) ──────────────────────

export const PERAN_PALING_ISTIMEWA: Role = "ADMIN_SISTEM";
export const MAKS_ALASAN = 500;
export const MIN_DURASI_MENIT = 5;
export const MAKS_DURASI_MENIT = 1440;
export const MAKS_PELAKU = 100;
export const MAKS_TARGET_PER_PELAKU = 200;

/** Peran yang dapat menjadi pelaku tambahan atau sasaran: semua kecuali Administrator Sistem. */
export const PERAN_DAPAT_DIPILIH: readonly Role[] = ROLE_IDS.filter(
  (r) => r !== PERAN_PALING_ISTIMEWA,
);

/** Peran lapis operasional (pintasan "pilih semua peran operasional"). */
export const PERAN_OPERASIONAL: readonly Role[] = PERAN_DAPAT_DIPILIH.filter(
  (r) => ROLES[r].lapis === "OPERASIONAL",
);

export const adalahPeran = (v: unknown): v is Role =>
  typeof v === "string" && (ROLE_IDS as readonly string[]).includes(v);

const POLA_UUID =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
export const adalahUuid = (v: unknown): v is string =>
  typeof v === "string" && POLA_UUID.test(v);

export const LABEL_ALASAN_BERAKHIR: Record<AlasanBerakhir, string> = {
  STOP: "Dihentikan pelaku",
  BERGANTI: "Berganti ke akun lain",
  KEDALUWARSA: "Batas durasi habis",
  KEBIJAKAN: "Kebijakan berubah",
  DIAKHIRI: "Diakhiri administrator",
  LOGOUT: "Pelaku keluar",
  DICABUT: "Sesi dicabut",
};

export function labelAlasanBerakhir(kode: string | null | undefined): string {
  if (!kode) return "";
  return LABEL_ALASAN_BERAKHIR[kode as AlasanBerakhir] ?? kode;
}

/** Label peran untuk tampilan; nilai di luar enum tampil apa adanya. */
export function labelPeran(peran: string | null | undefined): string {
  if (!peran) return "";
  return adalahPeran(peran) ? ROLES[peran].label : peran;
}

// ── Draf kebijakan (murni) ────────────────────────────────────────────

export interface DrafTarget {
  tipe: TipeAturan;
  peran?: Role;
  penggunaId?: string;
  /** Hanya untuk tampilan. */
  nama?: string;
  email?: string;
  penggunaPeran?: string;
}

export interface DrafPelaku extends DrafTarget {
  /** Kunci lokal stabil (bukan dikirim ke server) untuk daftar React. */
  kunci: string;
  aktif: boolean;
  target: DrafTarget[];
}

export interface DrafKebijakan {
  aktif: boolean;
  wajibAlasan: boolean;
  /** Teks isian menit; kosong = tanpa batas. */
  durasiMaks: string;
  pelaku: DrafPelaku[];
}

let hitungKunci = 0;
export const kunciBaru = (): string => `p${++hitungKunci}`;

const keTargetDraf = (t: TargetKebijakanWire): DrafTarget => ({
  tipe: t.tipe,
  ...(t.tipe === "PERAN" && adalahPeran(t.peran) ? { peran: t.peran } : {}),
  ...(t.tipe === "PENGGUNA" && t.pengguna_id
    ? {
        penggunaId: t.pengguna_id,
        nama: t.pengguna_nama ?? undefined,
        email: t.pengguna_email ?? undefined,
        penggunaPeran: t.pengguna_peran ?? undefined,
      }
    : {}),
});

/** Kebijakan dari server → draf yang dapat disunting. */
export function toDraf(k: KebijakanWire): DrafKebijakan {
  return {
    aktif: k.aktif,
    wajibAlasan: k.wajib_alasan,
    durasiMaks: k.durasi_maks_menit == null ? "" : String(k.durasi_maks_menit),
    pelaku: k.pelaku.map((p) => ({
      ...keTargetDraf(p),
      kunci: kunciBaru(),
      aktif: p.aktif,
      target: p.target.map(keTargetDraf),
    })),
  };
}

const keTargetInput = (t: DrafTarget): TargetKebijakanInput =>
  t.tipe === "PERAN"
    ? { tipe: "PERAN", peran: t.peran }
    : { tipe: "PENGGUNA", pengguna_id: t.penggunaId };

/** Teks durasi → menit (bilangan bulat) atau null bila kosong; NaN bila bukan bilangan bulat. */
export function bacaDurasi(teks: string): number | null {
  const t = teks.trim();
  if (t === "") return null;
  return /^\d+$/.test(t) ? Number(t) : Number.NaN;
}

/** Draf → badan PUT. `versi` adalah versi kebijakan yang dibaca (kunci optimistis). */
export function toInput(d: DrafKebijakan, versi: number): KebijakanInput {
  return {
    versi,
    aktif: d.aktif,
    wajib_alasan: d.wajibAlasan,
    durasi_maks_menit: bacaDurasi(d.durasiMaks),
    pelaku: d.pelaku.map((p) => ({
      ...keTargetInput(p),
      aktif: p.aktif,
      target: p.target.map(keTargetInput),
    })),
  };
}

const kunciTarget = (t: DrafTarget): string =>
  t.tipe === "PERAN"
    ? `PERAN:${t.peran ?? ""}`
    : `PENGGUNA:${t.penggunaId ?? ""}`;

/** Apakah draf berbeda dari kebijakan di server (untuk tombol simpan dan peringatan keluar). */
export function drafBerubah(d: DrafKebijakan, k: KebijakanWire): boolean {
  const awal = toDraf(k);
  if (d.aktif !== awal.aktif || d.wajibAlasan !== awal.wajibAlasan) return true;
  if (d.durasiMaks.trim() !== awal.durasiMaks) return true;
  const ringkas = (p: DrafPelaku) =>
    `${kunciTarget(p)}|${p.aktif}|${p.target.map(kunciTarget).sort().join(",")}`;
  const a = d.pelaku.map(ringkas).sort();
  const b = awal.pelaku.map(ringkas).sort();
  return a.length !== b.length || a.some((v, i) => v !== b[i]);
}

/** Pesan galat pengisian (kosong bila sah); cermin validasi API supaya galat jelas sebelum dikirim. */
export function validasiDraf(d: DrafKebijakan): string[] {
  const galat: string[] = [];
  const durasi = bacaDurasi(d.durasiMaks);
  if (
    durasi !== null &&
    (!Number.isInteger(durasi) ||
      durasi < MIN_DURASI_MENIT ||
      durasi > MAKS_DURASI_MENIT)
  ) {
    galat.push(
      `Durasi maksimal harus ${MIN_DURASI_MENIT} sampai ${MAKS_DURASI_MENIT} menit, atau kosong untuk tanpa batas.`,
    );
  }
  if (d.pelaku.length > MAKS_PELAKU) {
    galat.push(`Pelaku maksimal ${MAKS_PELAKU}.`);
  }
  const terlihat = new Set<string>();
  for (const p of d.pelaku) {
    const nama =
      p.tipe === "PERAN"
        ? labelPeran(p.peran)
        : p.nama || p.penggunaId || "pengguna";
    if (p.tipe === "PERAN" && !adalahPeran(p.peran)) {
      galat.push("Ada pelaku berperan yang belum dipilih.");
      continue;
    }
    if (p.tipe === "PENGGUNA" && !p.penggunaId) {
      galat.push("Ada pelaku berakun yang belum dipilih.");
      continue;
    }
    if (p.tipe === "PERAN" && p.peran === PERAN_PALING_ISTIMEWA) {
      galat.push(
        "Administrator Sistem selalu boleh meng-impersonate; tidak perlu didaftarkan sebagai pelaku.",
      );
    }
    const k = kunciTarget(p);
    if (terlihat.has(k))
      galat.push(`Pelaku "${nama}" terdaftar lebih dari sekali.`);
    terlihat.add(k);
    if (p.target.length > MAKS_TARGET_PER_PELAKU) {
      galat.push(`Target "${nama}" maksimal ${MAKS_TARGET_PER_PELAKU}.`);
    }
    const target = new Set<string>();
    for (const t of p.target) {
      if (t.tipe === "PERAN" && t.peran === PERAN_PALING_ISTIMEWA) {
        galat.push(
          `Administrator Sistem tidak boleh menjadi target (pelaku "${nama}").`,
        );
      }
      const kt = kunciTarget(t);
      if (target.has(kt)) galat.push(`Target ganda pada pelaku "${nama}".`);
      target.add(kt);
    }
    if (p.aktif && p.target.length === 0) {
      galat.push(`Pelaku "${nama}" aktif tetapi belum punya target.`);
    }
  }
  return [...new Set(galat)];
}

/** Menambah/membuang peran pada daftar target (kembalian baru; urutan mengikuti ROLE_IDS). */
export function aturPeranTarget(
  target: readonly DrafTarget[],
  peran: Role,
  pilih: boolean,
): DrafTarget[] {
  const tanpa = target.filter(
    (t) => !(t.tipe === "PERAN" && t.peran === peran),
  );
  if (!pilih) return tanpa;
  const akun = tanpa.filter((t) => t.tipe === "PENGGUNA");
  const daftarPeran = [
    ...new Set([
      ...tanpa.filter((t) => t.tipe === "PERAN").map((t) => t.peran as Role),
      peran,
    ]),
  ].sort((a, b) => ROLE_IDS.indexOf(a) - ROLE_IDS.indexOf(b));
  return [
    ...daftarPeran.map((p) => ({ tipe: "PERAN" as const, peran: p })),
    ...akun,
  ];
}

/** Mengganti seluruh target berperan dengan `peran` (akun tetap): pintasan "pilih semua". */
export function setPeranTarget(
  target: readonly DrafTarget[],
  peran: readonly Role[],
): DrafTarget[] {
  const akun = target.filter((t) => t.tipe === "PENGGUNA");
  const urut = [...new Set(peran)]
    .filter((p) => p !== PERAN_PALING_ISTIMEWA)
    .sort((a, b) => ROLE_IDS.indexOf(a) - ROLE_IDS.indexOf(b));
  return [...urut.map((p) => ({ tipe: "PERAN" as const, peran: p })), ...akun];
}

/**
 * Tujuan pemuatan halaman sesudah berpindah akun, divalidasi ulang di klien (lapis kedua sesudah BFF): hanya path
 * internal satu garis miring. Selain itu "/" (edge mengalihkannya ke beranda peran).
 */
export function tujuanAman(tujuan: unknown): string {
  return typeof tujuan === "string" && /^\/(?![/\\])[^\u0000-\u001f\u007f]*$/.test(tujuan)
    ? tujuan
    : "/";
}

// ── Pemilih sasaran (murni) ───────────────────────────────────────────

/** Pilihan pengguna di dialog: peran saja (server memilih akun) atau akun tertentu. */
export interface PilihanSasaran {
  peran: string;
  /** Kosong: kirim `target_peran`; terisi: kirim `target_pengguna_id`. */
  penggunaId?: string;
}

/**
 * Badan permintaan start dari pilihan dan alasan. Akun tertentu didahulukan; alasan dipangkas dan dibuang bila
 * kosong (API yang menolak bila kebijakan mewajibkannya).
 */
export function susunMulai(
  pilihan: PilihanSasaran,
  alasan: string,
): MulaiInput {
  const a = alasan.trim();
  return {
    ...(pilihan.penggunaId
      ? { target_pengguna_id: pilihan.penggunaId }
      : { target_peran: pilihan.peran }),
    ...(a ? { alasan: a } : {}),
  };
}

/** Kelompok per lapis untuk dialog pemilih: urutan lapis dan peran mengikuti LAPIS_META dan ROLE_IDS. */
export function kelompokkanPeranTarget(
  peran: readonly PeranTargetWire[],
): { lapis: string; peran: PeranTargetWire[] }[] {
  const urutLapis = ["STRATEGIS", "MANAJERIAL", "OPERASIONAL", "SISTEM"];
  const hasil = new Map<string, PeranTargetWire[]>();
  for (const p of [...peran].sort(
    (a, b) =>
      ROLE_IDS.indexOf(a.peran as Role) - ROLE_IDS.indexOf(b.peran as Role),
  )) {
    hasil.set(p.lapis, [...(hasil.get(p.lapis) ?? []), p]);
  }
  return [...hasil]
    .sort(
      ([a], [b]) =>
        (urutLapis.indexOf(a) + 1 || 99) - (urutLapis.indexOf(b) + 1 || 99),
    )
    .map(([lapis, daftar]) => ({ lapis, peran: daftar }));
}

/** Memasang pencarian sederhana pada akun (nama, email, lembaga); tak peka huruf besar-kecil. */
export function saringAkun(
  akun: readonly AkunTargetWire[],
  cari: string,
): AkunTargetWire[] {
  const q = cari.trim().toLowerCase();
  if (!q) return [...akun];
  return akun.filter((a) =>
    `${a.nama} ${a.email} ${a.lembaga}`.toLowerCase().includes(q),
  );
}

// ── Klien BFF ─────────────────────────────────────────────────────────

export interface ParamRiwayat {
  status?: "semua" | "aktif";
  cursor?: string;
  limit?: number;
}

export function queryRiwayat(p: ParamRiwayat): string {
  const qs = new URLSearchParams();
  if (p.status) qs.set("status", p.status);
  if (p.cursor) qs.set("cursor", p.cursor);
  if (p.limit) qs.set("limit", String(p.limit));
  const q = qs.toString();
  return q ? `?${q}` : "";
}

const URL_DASAR = "/api/impersonation";

export const impersonasiApi = {
  targets: () => bffFetch<TargetsWire>(`${URL_DASAR}/targets`),

  kebijakan: () => bffFetch<KebijakanWire>(`${URL_DASAR}/policy`),

  simpanKebijakan: (input: KebijakanInput) =>
    bffFetch<KebijakanWire>(`${URL_DASAR}/policy`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),

  riwayat: (p: ParamRiwayat = {}) =>
    bffFetch<HalamanRiwayatWire>(`${URL_DASAR}/riwayat${queryRiwayat(p)}`),

  akhiri: (id: string) =>
    bffFetch<{ berakhir: boolean }>(
      `${URL_DASAR}/riwayat/${encodeURIComponent(id)}/akhiri`,
      { method: "POST" },
    ),

  mulai: (input: MulaiInput & { dari?: string }) =>
    bffFetch<HasilAlihAkun>(`${URL_DASAR}/start`, {
      method: "POST",
      body: JSON.stringify(input),
    }),

  berhenti: (dari?: string) =>
    bffFetch<HasilAlihAkun>(`${URL_DASAR}/stop`, {
      method: "POST",
      body: JSON.stringify(dari ? { dari } : {}),
    }),
};

// ── Pencarian pengguna untuk editor kebijakan ─────────────────────────

/** Satu hasil pencarian akun (dari /api/pengguna, daftar pengguna yang sudah ada). */
export interface PenggunaCari {
  id: string;
  nama: string;
  email: string;
  peran: string;
}

const BATAS_HASIL_CARI = 8;

/**
 * Mencari akun AKTIF bukan Administrator Sistem (yang bukan target maupun pelaku tambahan) lewat API pengguna
 * yang sudah ada. Pencarian kosong mengembalikan daftar kosong tanpa permintaan.
 */
export async function cariPengguna(kata: string): Promise<PenggunaCari[]> {
  const q = kata.trim();
  if (q.length < 2) return [];
  const qs = new URLSearchParams({ search: q, status: "AKTIF", limit: "25" });
  const hasil = await bffFetch<{ items: PenggunaCari[] }>(`/api/pengguna?${qs}`);
  return hasil.items
    .filter((u) => u.peran !== PERAN_PALING_ISTIMEWA)
    .slice(0, BATAS_HASIL_CARI);
}
