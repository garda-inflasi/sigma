"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback } from "react";

/**
 * Tab tampilan yang tersimpan di URL (`?tab=`): bisa dibagikan, dan tombol kembali tetap bekerja. Tab bawaan tidak
 * menambah parameter. Halaman pemakainya harus berada di dalam `<Suspense>` karena memakai `useSearchParams`.
 */
export function useTabHalaman<T extends string>(
  daftar: readonly T[],
  bawaan: T,
  kunci = "tab",
): readonly [T, (nilai: T) => void] {
  const params = useSearchParams();
  const pathname = usePathname();
  const router = useRouter();
  const mentah = params.get(kunci);
  const aktif = daftar.includes(mentah as T) ? (mentah as T) : bawaan;

  const ubah = useCallback(
    (nilai: T) => {
      const q = new URLSearchParams(params.toString());
      if (nilai === bawaan) q.delete(kunci);
      else q.set(kunci, nilai);
      const s = q.toString();
      router.replace(s ? `${pathname}?${s}` : pathname, { scroll: false });
    },
    [params, pathname, router, bawaan, kunci],
  );

  return [aktif, ubah] as const;
}
