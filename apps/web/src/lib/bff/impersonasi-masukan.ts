/**
 * Validasi masukan route BFF impersonasi sebelum diteruskan ke API Go, dan penentuan halaman tujuan sesudah
 * berpindah akun. Murni: hanya mengembalikan nilai bersih atau pesan galat (400). API Go tetap otoritatif; ini
 * menolak isian cacat lebih awal dan memastikan hanya field yang dikenal yang sampai ke sana.
 */

import {
  MAKS_ALASAN,
  MAKS_DURASI_MENIT,
  MAKS_PELAKU,
  MAKS_TARGET_PER_PELAKU,
  MIN_DURASI_MENIT,
  PERAN_PALING_ISTIMEWA,
  adalahPeran,
  adalahUuid,
  type KebijakanInput,
  type MulaiInput,
  type PelakuKebijakanInput,
  type TargetKebijakanInput,
} from "@/lib/api/impersonasi";
import type { Role } from "@/lib/rbac/roles";
import { canAccessRoute } from "@/lib/rbac/routes";
import { validateRedirectUrl } from "./safe-redirect";

export type Hasil<T> = { ok: true; nilai: T } | { ok: false; pesan: string };

const gagal = (pesan: string): { ok: false; pesan: string } => ({
  ok: false,
  pesan,
});

const adalahObjek = (v: unknown): v is Record<string, unknown> =>
  v !== null && typeof v === "object" && !Array.isArray(v);

/** Jumlah karakter (titik kode), bukan unit UTF-16: sama dengan cara API Go menghitung. */
const panjang = (teks: string): number => [...teks].length;

/** Badan POST /api/impersonation/start. `dari` (halaman semula) hanya dipakai BFF dan tidak diteruskan ke API. */
export function bacaMulai(
  badan: unknown,
): Hasil<{ masukan: MulaiInput; dari?: string }> {
  if (!adalahObjek(badan)) return gagal("Badan permintaan tidak valid.");
  const { target_pengguna_id: id, target_peran: peran, alasan, dari } = badan;

  const masukan: MulaiInput = {};
  if (id !== undefined && id !== null) {
    if (!adalahUuid(id)) return gagal("ID akun sasaran tidak valid.");
    masukan.target_pengguna_id = id;
  }
  if (peran !== undefined && peran !== null) {
    if (!adalahPeran(peran)) return gagal("Peran sasaran tidak dikenal.");
    if (peran === PERAN_PALING_ISTIMEWA) {
      return gagal(
        "Administrator Sistem tidak dapat menjadi sasaran impersonasi.",
      );
    }
    masukan.target_peran = peran;
  }
  if (!masukan.target_pengguna_id && !masukan.target_peran) {
    return gagal("Pilih akun atau peran sasaran.");
  }
  if (alasan !== undefined && alasan !== null) {
    if (typeof alasan !== "string") return gagal("Alasan harus berupa teks.");
    const a = alasan.trim();
    if (panjang(a) > MAKS_ALASAN) {
      return gagal(`Alasan maksimal ${MAKS_ALASAN} karakter.`);
    }
    if (a) masukan.alasan = a;
  }
  if (dari !== undefined && dari !== null && typeof dari !== "string") {
    return gagal("Halaman semula tidak valid.");
  }
  return {
    ok: true,
    nilai: { masukan, dari: typeof dari === "string" ? dari : undefined },
  };
}

function bacaTarget(
  nilai: unknown,
  konteks: string,
): Hasil<TargetKebijakanInput> {
  if (!adalahObjek(nilai)) return gagal(`${konteks}: bentuk tidak valid.`);
  if (nilai.tipe === "PERAN") {
    if (!adalahPeran(nilai.peran))
      return gagal(`${konteks}: peran tidak dikenal.`);
    if (nilai.peran === PERAN_PALING_ISTIMEWA) {
      return gagal(`${konteks}: Administrator Sistem tidak boleh didaftarkan.`);
    }
    return { ok: true, nilai: { tipe: "PERAN", peran: nilai.peran } };
  }
  if (nilai.tipe === "PENGGUNA") {
    if (!adalahUuid(nilai.pengguna_id))
      return gagal(`${konteks}: ID pengguna tidak valid.`);
    return {
      ok: true,
      nilai: { tipe: "PENGGUNA", pengguna_id: nilai.pengguna_id },
    };
  }
  return gagal(`${konteks}: tipe harus PERAN atau PENGGUNA.`);
}

/** Badan PUT /api/impersonation/policy; field tampilan (`pengguna_nama`, dst.) dibuang. */
export function bacaKebijakan(badan: unknown): Hasil<KebijakanInput> {
  if (!adalahObjek(badan)) return gagal("Badan permintaan tidak valid.");
  const {
    versi,
    aktif,
    wajib_alasan: wajibAlasan,
    durasi_maks_menit: durasi,
    pelaku,
  } = badan;

  if (typeof versi !== "number" || !Number.isInteger(versi) || versi < 0) {
    return gagal("Versi kebijakan wajib diisi.");
  }
  if (typeof aktif !== "boolean")
    return gagal("Status aktif harus ya atau tidak.");
  if (typeof wajibAlasan !== "boolean")
    return gagal("Kewajiban alasan harus ya atau tidak.");
  if (durasi !== null) {
    if (
      typeof durasi !== "number" ||
      !Number.isInteger(durasi) ||
      durasi < MIN_DURASI_MENIT ||
      durasi > MAKS_DURASI_MENIT
    ) {
      return gagal(
        `Durasi maksimal harus ${MIN_DURASI_MENIT} sampai ${MAKS_DURASI_MENIT} menit, atau kosong.`,
      );
    }
  }
  if (!Array.isArray(pelaku)) return gagal("Daftar pelaku wajib berupa larik.");
  if (pelaku.length > MAKS_PELAKU)
    return gagal(`Pelaku maksimal ${MAKS_PELAKU}.`);

  const daftar: PelakuKebijakanInput[] = [];
  for (const [i, p] of pelaku.entries()) {
    const konteks = `Pelaku ke-${i + 1}`;
    if (!adalahObjek(p)) return gagal(`${konteks}: bentuk tidak valid.`);
    if (typeof p.aktif !== "boolean")
      return gagal(`${konteks}: status aktif tidak valid.`);
    if (!Array.isArray(p.target))
      return gagal(`${konteks}: daftar target wajib berupa larik.`);
    if (p.target.length > MAKS_TARGET_PER_PELAKU) {
      return gagal(`${konteks}: target maksimal ${MAKS_TARGET_PER_PELAKU}.`);
    }
    // Pelaku berperan ADMIN_SISTEM ditolak sama seperti target: ia selalu boleh dan tidak perlu didaftarkan.
    const dasar = bacaTarget(p, konteks);
    if (!dasar.ok) return dasar;
    const target: TargetKebijakanInput[] = [];
    for (const [j, t] of p.target.entries()) {
      const hasil = bacaTarget(t, `${konteks}, target ke-${j + 1}`);
      if (!hasil.ok) return hasil;
      target.push(hasil.nilai);
    }
    daftar.push({ ...dasar.nilai, aktif: p.aktif, target });
  }
  return {
    ok: true,
    nilai: {
      versi,
      aktif,
      wajib_alasan: wajibAlasan,
      durasi_maks_menit: durasi,
      pelaku: daftar,
    },
  };
}

/** Parameter query riwayat yang diteruskan ke API; selain ini dibuang. Mengembalikan query string (tanpa "?"). */
export function bacaParamRiwayat(params: URLSearchParams): Hasil<string> {
  const keluar = new URLSearchParams();
  const status = params.get("status");
  if (status !== null) {
    if (status !== "semua" && status !== "aktif") {
      return gagal("Status riwayat harus semua atau aktif.");
    }
    keluar.set("status", status);
  }
  const limit = params.get("limit");
  if (limit !== null) {
    if (!/^\d{1,3}$/.test(limit) || Number(limit) < 1 || Number(limit) > 100) {
      return gagal("Batas halaman harus 1 sampai 100.");
    }
    keluar.set("limit", limit);
  }
  const cursor = params.get("cursor");
  if (cursor !== null) {
    if (!/^[A-Za-z0-9_-]{1,512}$/.test(cursor))
      return gagal("Cursor tidak valid.");
    keluar.set("cursor", cursor);
  }
  return { ok: true, nilai: keluar.toString() };
}

/**
 * Halaman tujuan sesudah berpindah akun: halaman semula bila aman dan boleh dibuka peran baru, selain itu "/"
 * (edge mengalihkannya ke beranda peran). Rute `/api/*` dan halaman masuk tidak pernah menjadi tujuan.
 */
export function tentukanTujuan(
  dari: string | undefined,
  peran: Role,
  izin: readonly string[] | null,
): string {
  const aman = validateRedirectUrl(dari ?? null);
  if (!aman) return "/";
  let path: string;
  try {
    path = new URL(aman, "http://lokal").pathname;
  } catch {
    return "/";
  }
  if (
    path === "/" ||
    path.startsWith("/api/") ||
    path === "/masuk" ||
    path.startsWith("/masuk/")
  ) {
    return "/";
  }
  return canAccessRoute({ role: peran, permissions: izin }, path) ? aman : "/";
}
