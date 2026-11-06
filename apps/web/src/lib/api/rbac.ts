/**
 * RBAC berbasis basis data (02-C): matriks peran × izin yang dapat diedit dan izin langsung per pengguna.
 * Pemetaan snake_case API ↔ model UI, fungsi murni untuk draf matriks dan masa berlaku, serta klien BFF.
 * Katalog izin didefinisikan di kode (API); UI tidak dapat menciptakan izin baru.
 */

import { ApiError, bffFetch } from "./client";
import { ROLE_IDS, type Role } from "@/lib/rbac/roles";

// ── Bentuk kawat (API Go lewat BFF) ───────────────────────────────────

export interface PeranWire {
  id: string;
  label: string;
  lapis: string;
  permissions: string[];
  bawaan: string[];
  versi: number;
  diperbarui_pada?: string;
  diperbarui_oleh?: string | null;
}

export interface IzinWire {
  id: string;
  deskripsi: string;
  kelompok: string;
}

export interface IzinLangsungWire {
  izin: string;
  deskripsi: string;
  kelompok: string;
  berlaku_sampai: string | null;
  alasan: string;
  diberikan_oleh: string | null;
  diberikan_oleh_nama: string | null;
  diberikan_pada: string;
  aktif: boolean;
}

export interface IzinPenggunaWire {
  peran: string;
  izin_peran: string[];
  langsung: IzinLangsungWire[];
  efektif: string[];
}

// ── Model UI ──────────────────────────────────────────────────────────

export interface PeranAkses {
  id: Role;
  label: string;
  versi: number;
  /** Izin peran yang berlaku (basis data). */
  izin: ReadonlySet<string>;
  /** Izin bawaan di kode: pembanding "pulihkan bawaan". */
  bawaan: ReadonlySet<string>;
  diperbaruiPada?: string;
}

export interface IzinKatalog {
  id: string;
  deskripsi: string;
  kelompok: string;
}

export interface KelompokIzin {
  nama: string;
  izin: IzinKatalog[];
}

export interface MatriksAkses {
  peran: PeranAkses[];
  izin: IzinKatalog[];
  /** Izin per kelompok, dalam urutan katalog. */
  kelompok: KelompokIzin[];
}

const adalahPeran = (id: string): id is Role =>
  (ROLE_IDS as readonly string[]).includes(id);

/** Daftar peran dan katalog izin dari API → model matriks. Peran di luar 14 nilai enum diabaikan. */
export function toMatriksAkses(
  roles: PeranWire[],
  permissions: IzinWire[],
): MatriksAkses {
  const peran: PeranAkses[] = roles
    .filter((r) => adalahPeran(r.id))
    .map((r) => ({
      id: r.id as Role,
      label: r.label,
      versi: r.versi,
      izin: new Set(r.permissions),
      bawaan: new Set(r.bawaan),
      diperbaruiPada: r.diperbarui_pada,
    }));
  const izin = permissions.map((p) => ({
    id: p.id,
    deskripsi: p.deskripsi,
    kelompok: p.kelompok,
  }));
  const kelompok: KelompokIzin[] = [];
  for (const p of izin) {
    let k = kelompok.find((g) => g.nama === p.kelompok);
    if (!k) {
      k = { nama: p.kelompok, izin: [] };
      kelompok.push(k);
    }
    k.izin.push(p);
  }
  return { peran, izin, kelompok };
}

// ── Draf matriks (murni) ──────────────────────────────────────────────

/** Apakah dua himpunan izin berisi anggota yang sama. */
export function samaIzin(
  a: ReadonlySet<string>,
  b: ReadonlySet<string>,
): boolean {
  return a.size === b.size && [...a].every((p) => b.has(p));
}

/** Izin yang ditambah dan dicabut draf dibanding keadaan di server. */
export function selisihDraf(
  server: ReadonlySet<string>,
  draf: ReadonlySet<string>,
): { ditambah: string[]; dicabut: string[] } {
  return {
    ditambah: [...draf].filter((p) => !server.has(p)).sort(),
    dicabut: [...server].filter((p) => !draf.has(p)).sort(),
  };
}

/** Himpunan izin → larik dalam urutan katalog (urutan stabil untuk PUT dan tes). Izin di luar katalog dibuang. */
export function urutkanSesuaiKatalog(
  izin: ReadonlySet<string>,
  katalog: readonly IzinKatalog[],
): string[] {
  return katalog.filter((k) => izin.has(k.id)).map((k) => k.id);
}

/** Membalik satu izin pada salinan himpunan. */
export function balikIzin(
  izin: ReadonlySet<string>,
  nama: string,
): Set<string> {
  const baru = new Set(izin);
  if (baru.has(nama)) baru.delete(nama);
  else baru.add(nama);
  return baru;
}

// ── Izin langsung ─────────────────────────────────────────────────────

export interface IzinLangsung {
  izin: string;
  deskripsi: string;
  kelompok: string;
  /** ISO 8601; kosong berarti tanpa batas. */
  berlakuSampai?: string;
  alasan: string;
  diberikanOleh?: string;
  diberikanPada: string;
  /** False bila masa berlakunya sudah lewat (riwayat sampai dicabut). */
  aktif: boolean;
}

export interface IzinPengguna {
  peran: string;
  izinPeran: string[];
  langsung: IzinLangsung[];
  efektif: string[];
}

export function toIzinLangsung(w: IzinLangsungWire): IzinLangsung {
  return {
    izin: w.izin,
    deskripsi: w.deskripsi,
    kelompok: w.kelompok,
    berlakuSampai: w.berlaku_sampai ?? undefined,
    alasan: w.alasan,
    diberikanOleh: w.diberikan_oleh_nama ?? w.diberikan_oleh ?? undefined,
    diberikanPada: w.diberikan_pada,
    aktif: w.aktif,
  };
}

export function toIzinPengguna(w: IzinPenggunaWire): IzinPengguna {
  return {
    peran: w.peran,
    izinPeran: w.izin_peran,
    langsung: w.langsung.map(toIzinLangsung),
    efektif: w.efektif,
  };
}

/** Izin yang masih dapat diberikan: belum dimiliki lewat peran dan belum ada izin langsung yang berlaku. */
export function izinTersedia(
  katalog: readonly IzinKatalog[],
  pengguna: IzinPengguna,
): IzinKatalog[] {
  const dimiliki = new Set([
    ...pengguna.izinPeran,
    ...pengguna.langsung.filter((g) => g.aktif).map((g) => g.izin),
  ]);
  return katalog.filter((k) => !dimiliki.has(k.id));
}

/**
 * Tanggal (YYYY-MM-DD dari input date) → akhir hari itu di zona waktu peramban sebagai ISO 8601, atau null bila
 * bukan tanggal kalender yang sah.
 */
export function akhirHari(tanggal: string): string | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(tanggal);
  if (!m) return null;
  const [y, bln, tgl] = [Number(m[1]), Number(m[2]), Number(m[3])];
  const d = new Date(y, bln - 1, tgl, 23, 59, 59, 999);
  if (d.getFullYear() !== y || d.getMonth() !== bln - 1 || d.getDate() !== tgl)
    return null;
  return d.toISOString();
}

/** Pesan galat untuk tanggal berlaku (kosong = tanpa batas, sah); undefined bila sah. */
export function galatTanggalBerlaku(
  tanggal: string,
  sekarang: Date = new Date(),
): string | undefined {
  if (!tanggal) return undefined;
  const akhir = akhirHari(tanggal);
  if (!akhir) return "Tanggal tidak valid.";
  if (new Date(akhir).getTime() <= sekarang.getTime())
    return "Masa berlaku harus di masa depan.";
  return undefined;
}

/** Tanggal hari ini (YYYY-MM-DD, zona peramban) untuk atribut `min` input date. */
export function hariIni(sekarang: Date = new Date()): string {
  const p = (n: number) => String(n).padStart(2, "0");
  return `${sekarang.getFullYear()}-${p(sekarang.getMonth() + 1)}-${p(sekarang.getDate())}`;
}

// ── Klien BFF ─────────────────────────────────────────────────────────

export interface BeriIzinInput {
  izin: string;
  /** ISO 8601 di masa depan; kosong = tanpa batas. */
  berlakuSampai?: string;
  alasan: string;
}

/** Kode galat mesin dari API yang ditangani UI secara khusus. */
export const KODE_VERSI_PERAN_BERUBAH = "ROLE_VERSION_CONFLICT";
export const KODE_PEMEGANG_TERAKHIR = "LAST_PERMISSION_HOLDER";

/** Kode galat mesin (`kode` di badan galat BFF) dari kegagalan panggilan API, atau undefined. */
export function kodeGalat(err: unknown): string | undefined {
  if (!(err instanceof ApiError)) return undefined;
  const kode = (err.payload as { kode?: unknown } | undefined)?.kode;
  return typeof kode === "string" ? kode : undefined;
}

export const rbacApi = {
  matriks: async (): Promise<MatriksAkses> => {
    const [roles, permissions] = await Promise.all([
      bffFetch<PeranWire[]>("/api/rbac/roles"),
      bffFetch<IzinWire[]>("/api/rbac/permissions"),
    ]);
    return toMatriksAkses(roles, permissions);
  },

  /** Mengganti seluruh izin satu peran; `versi` adalah kunci optimistis (409 ROLE_VERSION_CONFLICT bila basi). */
  simpanPeran: (id: Role, versi: number, izin: string[]) =>
    bffFetch<PeranWire>(
      `/api/rbac/roles/${encodeURIComponent(id)}/permissions`,
      {
        method: "PUT",
        body: JSON.stringify({ permissions: izin, versi }),
      },
    ),

  izinPengguna: async (id: string): Promise<IzinPengguna> =>
    toIzinPengguna(
      await bffFetch<IzinPenggunaWire>(
        `/api/pengguna/${encodeURIComponent(id)}/permissions`,
      ),
    ),

  beriIzin: async (id: string, input: BeriIzinInput): Promise<IzinLangsung> =>
    toIzinLangsung(
      await bffFetch<IzinLangsungWire>(
        `/api/pengguna/${encodeURIComponent(id)}/permissions`,
        {
          method: "POST",
          body: JSON.stringify({
            izin: input.izin,
            alasan: input.alasan,
            ...(input.berlakuSampai
              ? { berlaku_sampai: input.berlakuSampai }
              : {}),
          }),
        },
      ),
    ),

  cabutIzin: (id: string, izin: string) =>
    bffFetch<{ id: string; izin: string }>(
      `/api/pengguna/${encodeURIComponent(id)}/permissions/${encodeURIComponent(izin)}`,
      { method: "DELETE" },
    ),
};
