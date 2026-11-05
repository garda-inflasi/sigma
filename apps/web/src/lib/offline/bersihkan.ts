/**
 * Pembersihan data offline milik pengguna di perangkat: antrean entri (IndexedDB), cache data acuan, dan
 * halaman berpengguna di Cache Storage milik service worker. Murni dan diinjeksi (tanpa menyentuh
 * `indexedDB`, `caches`, atau store zustand langsung) agar teruji; sambungan ke API peramban ada di
 * `bersihkan-klien.ts`.
 *
 * Yang DIHAPUS saat keluar atau ganti pengguna:
 *  - seluruh isi IndexedDB aplikasi (antrean entri, data acuan, penanda pemilik);
 *  - isi Cache Storage `garda-*` yang bukan aset statis: HTML navigasi `/lapangan/*`, `/masuk`, dst.
 *    Halaman itu dirender server dengan nama, peran, dan wilayah pengguna ("Halo, Aulia"), jadi bila
 *    dibiarkan, pengguna berikutnya yang membuka aplikasi dalam keadaan offline akan disuguhi halaman
 *    pengguna sebelumnya.
 *
 * Yang DIPERTAHANKAN di cache: `/_next/static/*` (aset ber-hash), `/icons/*`, berkas statis
 * (woff2/png/svg/css/js), `/offline`, dan `/manifest.webmanifest`. Semuanya sama untuk semua pengguna dan
 * dibutuhkan agar cangkang aplikasi tetap bisa dimuat offline (termasuk halaman /offline yang dipakai
 * service worker saat peramban sedang luring dan /masuk tidak tersimpan).
 *
 * Catatan `/offline`: layout akar menyisipkan sesi pengguna ke HTML semua halaman, jadi salinan yang diambil
 * dengan cookie memuat nama pengguna. Karena itu public/sw.js mengambil halaman publik (`SHELL_PUBLIK`)
 * dengan `credentials: "omit"` saat pemasangan dan tidak menyimpan hasil navigasi ke /offline; salinan
 * yang tersimpan anonim. Tes memeriksa keselarasan aturan ini dengan sw.js.
 */
import type { EntriLokal } from "@/lib/stores/field-queue-store";

/** Cache milik service worker aplikasi (`VERSI = "garda-v1"` di public/sw.js). Cache pihak ketiga tidak disentuh. */
export const POLA_NAMA_CACHE = /^garda-/;

/** Sama dengan aturan "aset statis" di public/sw.js. */
const BERKAS_STATIS = /\.(woff2|png|svg|css|js)$/;

/** Path yang sama untuk semua pengguna dan boleh tetap berada di cache sesudah keluar. */
export function adalahAsetStatis(pathname: string): boolean {
  return (
    pathname.startsWith("/_next/static/") ||
    pathname.startsWith("/icons/") ||
    pathname === "/offline" ||
    pathname === "/manifest.webmanifest" ||
    BERKAS_STATIS.test(pathname)
  );
}

/**
 * Entri cache dipertahankan hanya bila berasal dari origin aplikasi dan berupa aset statis. URL yang tidak
 * terbaca atau berasal dari origin lain dianggap bukan milik cangkang statis, jadi dihapus (gagal tertutup).
 */
export function dipertahankanDiCache(url: string, originApp: string): boolean {
  try {
    const u = new URL(url);
    return u.origin === originApp && adalahAsetStatis(u.pathname);
  } catch {
    return false;
  }
}

/** Entri cache sebagaimana dikembalikan `Cache.keys()`; tipe generik agar objek `Request` asli diteruskan ke penghapusan. */
export interface DepsCache<E extends { url: string }> {
  originApp: string;
  namaCache(): Promise<string[]>;
  buka(nama: string): Promise<{
    daftar(): Promise<readonly E[]>;
    hapus(entri: E): Promise<boolean>;
  }>;
}

export interface HasilCache {
  dihapus: number;
  dipertahankan: number;
}

/** Menghapus entri berpengguna dari semua cache `garda-*`; satu entri yang gagal tidak menghentikan sisanya. */
export async function bersihkanCache<E extends { url: string }>(
  deps: DepsCache<E>,
): Promise<HasilCache> {
  const hasil: HasilCache = { dihapus: 0, dipertahankan: 0 };
  let galat: unknown;
  for (const nama of await deps.namaCache()) {
    if (!POLA_NAMA_CACHE.test(nama)) continue;
    const cache = await deps.buka(nama);
    for (const entri of await cache.daftar()) {
      if (dipertahankanDiCache(entri.url, deps.originApp)) {
        hasil.dipertahankan++;
        continue;
      }
      try {
        if (await cache.hapus(entri)) hasil.dihapus++;
      } catch (e) {
        galat ??= e;
      }
    }
  }
  if (galat !== undefined) throw galat;
  return hasil;
}

export interface DepsBersih {
  /** Menghentikan penulisan balik store ke IndexedDB, supaya sinkronisasi yang masih berjalan tidak menghidupkan lagi data yang baru dihapus. */
  hentikanPersistensi?: () => void;
  /** Mengosongkan state in-memory (antrean, data acuan). */
  kosongkanMemori?: () => void;
  hapusIdb: () => Promise<unknown>;
  hapusCache: () => Promise<unknown>;
}

export interface HasilBersih {
  /** Satu entri per langkah yang gagal; kosong berarti seluruh data lokal sudah dibersihkan. */
  galat: string[];
}

/**
 * Menjalankan semua langkah pembersihan; satu langkah yang gagal tidak menghentikan yang lain dan tidak
 * pernah melempar, karena pemanggil (keluar, ganti pengguna) harus tetap melanjutkan.
 * Urutan penting: berhenti menulis dulu, lalu kosongkan memori, baru hapus penyimpanan.
 */
export async function bersihkanDataLokal(
  deps: DepsBersih,
): Promise<HasilBersih> {
  const galat: string[] = [];
  const coba = async (nama: string, langkah?: () => unknown) => {
    if (!langkah) return;
    try {
      await langkah();
    } catch (e) {
      galat.push(`${nama}: ${e instanceof Error ? e.message : String(e)}`);
    }
  };
  await coba("hentikan persistensi", deps.hentikanPersistensi);
  await coba("kosongkan memori", deps.kosongkanMemori);
  await Promise.all([
    coba("indexeddb", deps.hapusIdb),
    coba("cache", deps.hapusCache),
  ]);
  return { galat };
}

/** Status antrean yang masih harus dikirim atau diselesaikan; hanya TERSINKRON yang aman di server. */
const STATUS_AMAN: ReadonlyArray<EntriLokal["status"]> = ["TERSINKRON"];

/**
 * Jumlah entri antrean yang belum sampai di server (antre, mengirim, gagal, konflik, dan draf) dari nilai
 * mentah yang ditulis zustand persist (`{"state":{"items":[...]},"version":0}`). Dibaca langsung dari
 * penyimpanan karena di halaman konsol store belum dipulihkan. Nilai rusak dihitung nol.
 */
export function hitungBelumTerkirim(mentah: unknown): number {
  if (typeof mentah !== "string") return 0;
  try {
    const items = (JSON.parse(mentah) as { state?: { items?: unknown } })?.state
      ?.items;
    if (!Array.isArray(items)) return 0;
    return items.filter(
      (i) =>
        i &&
        typeof i === "object" &&
        !STATUS_AMAN.includes(
          (i as { status?: EntriLokal["status"] })
            .status as EntriLokal["status"],
        ),
    ).length;
  } catch {
    return 0;
  }
}

/** Sama dengan `hitungBelumTerkirim` untuk item yang sudah ada di memori. */
export function hitungBelumTerkirimDariItems(
  items: ReadonlyArray<Pick<EntriLokal, "status">>,
): number {
  return items.filter((i) => !STATUS_AMAN.includes(i.status)).length;
}
