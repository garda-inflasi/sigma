"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";
import {
  KUNCI_IZIN_BERUBAH,
  PERISTIWA_IZIN_BERUBAH,
} from "@/lib/auth/izin-berubah";

/**
 * Membaca ulang sesi di layout root begitu izin akun berubah (izin langsung diberi/dicabut/kedaluwarsa atau
 * matriks peran diubah, 02-C). API Go menjawab 401 PERMISSIONS_CHANGED untuk token yang salinan izinnya basi,
 * refresh senyap (01-E) menerbitkan token baru, dan refresh yang mengubah izin mengumumkannya di sini, juga ke
 * tab lain lewat peristiwa storage. `router.refresh()` merender ulang layout root dengan sesi baru sehingga sidebar,
 * palet perintah, dan tombol berizin ikut menyusul tanpa muat ulang manual. Dirender sekali di Providers.
 */
export function PenyegarIzin() {
  const router = useRouter();

  useEffect(() => {
    const segarkan = () => router.refresh();
    const dariTabLain = (e: StorageEvent) => {
      if (e.key === KUNCI_IZIN_BERUBAH) segarkan();
    };
    window.addEventListener(PERISTIWA_IZIN_BERUBAH, segarkan);
    window.addEventListener("storage", dariTabLain);
    return () => {
      window.removeEventListener(PERISTIWA_IZIN_BERUBAH, segarkan);
      window.removeEventListener("storage", dariTabLain);
    };
  }, [router]);

  return null;
}
