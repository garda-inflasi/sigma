"use client";

import { SearchParamsContext } from "next/dist/shared/lib/hooks-client-context.shared-runtime";
import { useRouter } from "next/navigation";
import { useCallback, useContext, useMemo } from "react";
import {
  bersihkanFilter,
  toFilterQuery,
  type FilterOperasional,
  type SaringanOperasional,
} from "@/lib/api/dashboard";

const MAKS_RENTANG_HARI = 90;

/** Memeriksa apakah pasangan tanggal dari..sampai valid (dari <= sampai dan rentang <= 90 hari). */
export function validasiRentangTanggal(dari?: string, sampai?: string): string | null {
  if (!dari && !sampai) return null;
  if (dari && sampai) {
    const tDari = Date.parse(dari);
    const tSampai = Date.parse(sampai);
    if (Number.isNaN(tDari) || Number.isNaN(tSampai)) {
      return "Format tanggal tidak valid.";
    }
    if (tDari > tSampai) {
      return "Tanggal mulai tidak boleh melebihi tanggal akhir.";
    }
    const selisihHari = Math.round((tSampai - tDari) / (1000 * 60 * 60 * 24));
    if (selisihHari > MAKS_RENTANG_HARI) {
      return `Rentang tanggal maksimal ${MAKS_RENTANG_HARI} hari.`;
    }
  }
  return null;
}

/**
 * Membaca dan menyinkronkan filter dashboard operasional (?kabupaten_kode=&komoditas_kode=&dari=&sampai=)
 * dengan URL query string.
 */
export function useFilterOperasional() {
  const searchParams = useContext(SearchParamsContext);
  const router = useRouter();

  const mentah = useMemo<FilterOperasional>(
    () =>
      bersihkanFilter({
        kabupatenKode: searchParams?.get("kabupaten_kode") ?? undefined,
        komoditasKode: searchParams?.get("komoditas_kode") ?? undefined,
        dari: searchParams?.get("dari") ?? undefined,
        sampai: searchParams?.get("sampai") ?? undefined,
      }),
    [searchParams],
  );

  const galatRentang = useMemo(
    () => validasiRentangTanggal(mentah.dari, mentah.sampai),
    [mentah.dari, mentah.sampai],
  );

  // Bila rentang tanggal tidak valid, jangan kirim dari/sampai ke API agar server tidak membalas 400.
  const filter = useMemo<FilterOperasional>(
    () =>
      galatRentang
        ? bersihkanFilter({
            kabupatenKode: mentah.kabupatenKode,
            komoditasKode: mentah.komoditasKode,
          })
        : mentah,
    [mentah, galatRentang],
  );

  const saringan = useMemo<SaringanOperasional>(
    () =>
      bersihkanFilter({
        kabupatenKode: mentah.kabupatenKode,
        komoditasKode: mentah.komoditasKode,
      }),
    [mentah.kabupatenKode, mentah.komoditasKode],
  );

  const setFilter = useCallback(
    (patch: Partial<FilterOperasional>) => {
      const merged = bersihkanFilter({ ...mentah, ...patch });
      router.replace(`/operasional${toFilterQuery(merged)}`, { scroll: false });
    },
    [mentah, router],
  );

  const resetFilter = useCallback(() => {
    router.replace("/operasional", { scroll: false });
  }, [router]);

  const adaFilter = Boolean(
    mentah.kabupatenKode || mentah.komoditasKode || mentah.dari || mentah.sampai,
  );

  return {
    mentah,
    filter,
    saringan,
    galatRentang,
    adaFilter,
    setFilter,
    resetFilter,
  };
}
