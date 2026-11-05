"use client";

import { useEffect } from "react";
import { toast } from "sonner";
import { useOptionalSession } from "@/lib/auth/session-context";
import { pastikanPemilikOffline } from "@/lib/offline/bersihkan-klien";

/**
 * Menjaga agar data offline di perangkat (antrean entri, data acuan, cache halaman) hanya milik pengguna
 * yang sedang masuk. Sesi bisa berakhir tanpa menekan Keluar (token kedaluwarsa, pengguna lain masuk di
 * perangkat yang sama); begitu pengguna yang berbeda memuat aplikasi, data pengguna sebelumnya dihapus.
 *
 * Dirender di Providers sehingga berlaku di semua halaman berpengguna, bukan hanya cangkang lapangan.
 * FieldShell memanggil fungsi yang sama dan menunggu hasilnya sebelum memulihkan antrean (satu eksekusi
 * per pengguna per pemuatan halaman).
 */
export function PenjagaDataOffline() {
  const userId = useOptionalSession()?.id;

  useEffect(() => {
    if (!userId) return;
    let batal = false;
    void pastikanPemilikOffline(userId).then((h) => {
      if (batal || h.status !== "dialihkan" || h.entriDibuang === 0) return;
      toast.warning("Data offline pengguna sebelumnya dihapus", {
        description: `${h.entriDibuang} entri yang belum terkirim milik akun lain di perangkat ini dibuang.`,
      });
    });
    return () => {
      batal = true;
    };
  }, [userId]);

  return null;
}
