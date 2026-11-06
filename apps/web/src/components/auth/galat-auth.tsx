"use client";

import { useEffect, useState } from "react";
import {
  bagianPesan,
  masihMenunggu,
  type GalatAuth,
} from "@/lib/auth/pesan-login";

/**
 * Jam untuk hitung mundur. `null` sampai komponen terpasang di klien: render server dan hidrasi pertama
 * memakai teks tanpa waktu, sehingga HTML-nya sama persis dan tidak ada galat hidrasi. Berdetak tiap
 * detik hanya selama `aktif`.
 */
export function useSekarang(aktif: boolean): number | null {
  const [sekarang, setSekarang] = useState<number | null>(null);
  useEffect(() => {
    setSekarang(Date.now());
    if (!aktif) return;
    const id = setInterval(() => setSekarang(Date.now()), 1000);
    return () => clearInterval(id);
  }, [aktif]);
  return sekarang;
}

/** Hitung mundur untuk galat yang dikelola di tempat lain (mis. `useFormBff`). */
export function useTungguGalat(galat: GalatAuth | null) {
  const sekarang = useSekarang(galat?.sampaiMs !== undefined);
  const tertahan = sekarang !== null && masihMenunggu(galat, sekarang);
  return { sekarang, tertahan };
}

/**
 * Galat auth satu form beserta hitung mundurnya. `tertahan` true selama kunci akun atau batas laju masih
 * berlaku: form menonaktifkan tombol kirimnya.
 */
export function useGalatAuth(awal: GalatAuth | null = null) {
  const [galat, setGalat] = useState<GalatAuth | null>(awal);
  const sekarang = useSekarang(galat?.sampaiMs !== undefined);
  const tertahan = sekarang !== null && masihMenunggu(galat, sekarang);
  return { galat, setGalat, sekarang, tertahan };
}

/**
 * Kotak galat form auth. Sisa waktu dirender di `role="timer"` supaya pembaca layar tidak membacakannya
 * tiap detik; kalimat selebihnya tetap (`role="alert"`).
 */
export function KotakGalatAuth({
  galat,
  sekarang,
  children,
}: {
  galat: GalatAuth;
  sekarang: number | null;
  children?: React.ReactNode;
}) {
  const { teks, sisa } = bagianPesan(galat, sekarang);
  return (
    <div role="alert" className="space-y-1 text-sm">
      <p className="font-medium text-destructive">
        {teks}
        {sisa && (
          <>
            {" "}
            Coba lagi dalam <span role="timer">{sisa}</span>.
          </>
        )}
      </p>
      {children}
    </div>
  );
}
