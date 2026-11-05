"use client";

import { clear, get, set } from "idb-keyval";
import { useFieldQueue } from "@/lib/stores/field-queue-store";
import { useAcuanServer } from "./acuan-server";
import {
  bersihkanCache,
  bersihkanDataLokal,
  hitungBelumTerkirim,
  hitungBelumTerkirimDariItems,
  type HasilBersih,
} from "./bersihkan";
import { KUNCI_ANTREAN, KUNCI_PEMILIK } from "./kunci";
import { pastikanPemilik, type HasilPemilik } from "./pemilik";
import { tutupPenulisanIdb } from "./penutup";

/**
 * Sambungan pembersihan data offline ke API peramban (IndexedDB, Cache Storage) dan store zustand.
 * Logika keputusannya ada di bersihkan.ts dan pemilik.ts (murni, teruji).
 */

const adaIdb = () => typeof indexedDB !== "undefined";

/** Menghapus seluruh isi IndexedDB aplikasi (store bawaan idb-keyval: antrean, data acuan, penanda pemilik). */
async function hapusIdb(): Promise<void> {
  if (adaIdb()) await clear();
}

async function hapusCacheBerpengguna(): Promise<void> {
  if (typeof caches === "undefined") return;
  await bersihkanCache<Request>({
    originApp: window.location.origin,
    namaCache: () => caches.keys(),
    buka: async (nama) => {
      const cache = await caches.open(nama);
      return {
        daftar: () => cache.keys(),
        // Entri HTML bervariasi menurut header Next (rsc, state tree); tanpa ignoreVary penghapusan tidak cocok.
        hapus: (entri) => cache.delete(entri, { ignoreVary: true }),
      };
    },
  });
}

function kosongkanMemori(): void {
  // Antrean kosong: putaran sinkron baru tidak punya apa pun untuk dikirim, dan putaran yang sudah berjalan
  // hanya memperbarui item yang sudah tiada.
  useFieldQueue.setState({ items: [], lastSyncAt: undefined });
  useAcuanServer.setState({ data: undefined });
}

/**
 * Pembersihan penuh saat keluar: berhenti menulis, kosongkan memori, hapus IndexedDB dan cache berpengguna.
 * Tidak melempar; galat per langkah ada di hasil.
 */
export function bersihkanSaatKeluar(): Promise<HasilBersih> {
  return bersihkanDataLokal({
    // Menutup penulisan idbStorage dan cache acuan, supaya sinkronisasi/unduhan yang masih berjalan tidak menghidupkan data lagi.
    hentikanPersistensi: tutupPenulisanIdb,
    kosongkanMemori,
    hapusIdb,
    hapusCache: hapusCacheBerpengguna,
  });
}

/**
 * Entri yang belum sampai di server. Memori lebih mutakhir bila antrean sudah dipulihkan (halaman lapangan);
 * di halaman konsol antrean belum dipulihkan sehingga dibaca langsung dari IndexedDB.
 */
export async function bacaBelumTerkirim(): Promise<number> {
  const st = useFieldQueue.getState();
  if (st.hydrated) return hitungBelumTerkirimDariItems(st.items);
  if (!adaIdb()) return 0;
  return hitungBelumTerkirim(await get<string>(KUNCI_ANTREAN));
}

// ── Pemilik data ──────────────────────────────────────────────────────

let pemilikTerakhir: { id: string; hasil: Promise<HasilPemilik> } | null = null;

/**
 * Memastikan data offline di perangkat milik `userId`; bila tidak, data pengguna sebelumnya dihapus. Dipanggil
 * dari Providers (semua halaman berpengguna) dan FieldShell (sebelum antrean dipulihkan). Satu eksekusi per
 * pengguna per pemuatan halaman: pemanggil berikutnya mendapat hasil yang sama.
 */
export function pastikanPemilikOffline(userId: string): Promise<HasilPemilik> {
  if (pemilikTerakhir?.id === userId) return pemilikTerakhir.hasil;
  const hasil = pastikanPemilik(userId, {
    bacaPemilik: async () => {
      if (!adaIdb()) throw new Error("IndexedDB tidak tersedia");
      return (await get<string>(KUNCI_PEMILIK)) ?? undefined;
    },
    tulisPemilik: (id) => set(KUNCI_PEMILIK, id),
    hitungBelumTerkirim: async () =>
      hitungBelumTerkirim(await get<string>(KUNCI_ANTREAN)),
    // Belum ada antrean di memori saat ini (pemulihan menunggu hasil ini), jadi cukup menghapus penyimpanan.
    bersihkan: () =>
      bersihkanDataLokal({ hapusIdb, hapusCache: hapusCacheBerpengguna }),
  });
  pemilikTerakhir = { id: userId, hasil };
  return hasil;
}

/** Pengecekan pemilik yang sudah dimulai di halaman ini (null bila belum); dipakai sebelum memulihkan antrean di luar FieldShell. */
export function pemilikSaatIni(): Promise<HasilPemilik> | null {
  return pemilikTerakhir?.hasil ?? null;
}
