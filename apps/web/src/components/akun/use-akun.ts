"use client";

import { useQuery } from "@tanstack/react-query";
import { bffFetch } from "@/lib/api/client";
import type { UserResponse } from "@/lib/bff/api-client";
import { qk } from "@/lib/query/keys";

/**
 * Kunci anak `profil.saya`: invalidasi `profil.saya` (mis. oleh tab data diri, atau sesudah MFA berubah)
 * ikut menyegarkannya, tetapi isi cache-nya tidak bentrok dengan query lain di bawah kunci induk itu.
 */
export const kunciAkunKeamanan = () =>
  [...qk.profil.saya(), "keamanan"] as const;

/** Akun sendiri untuk panel keamanan (GET /api/auth/me): status MFA, kewajibannya, dan sisa kode cadangan. */
export function useAkunKeamanan() {
  return useQuery({
    queryKey: kunciAkunKeamanan(),
    queryFn: async () =>
      (await bffFetch<{ user: UserResponse }>("/api/auth/me")).user,
    // Data keamanan: selalu ambil yang terbaru saat panel dibuka.
    staleTime: 0,
  });
}
