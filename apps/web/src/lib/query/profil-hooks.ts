"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { MODE_MOCK, bffFetch } from "@/lib/api/client";
import { useOptionalSession } from "@/lib/auth/session-context";
import type { MeUserResponse, UpdateProfilRequest } from "@/lib/bff/api-client";
import { namaWilayah } from "@/lib/data/generators";
import { terapkanPerubahan } from "@/lib/profil/isian";
import { can } from "@/lib/rbac/permissions";
import { useMasterWilayah } from "./hooks";
import { qk } from "./keys";

/** Profil sendiri: data diri, penugasan, dan status MFA. Mode mock tidak punya BFF auth. */
export function useProfil() {
  return useQuery({
    queryKey: qk.profil.saya(),
    queryFn: async () =>
      (await bffFetch<{ user: MeUserResponse }>("/api/auth/me")).user,
    enabled: !MODE_MOCK,
    // Penugasan dan peran bisa diubah administrator: baca ulang setiap halaman dibuka.
    staleTime: 0,
  });
}

/**
 * Mengubah nama dan/atau telepon. Tampilan langsung berubah (optimistis) dan dikembalikan bila server
 * menolak; jawaban server menggantikan cache. Bila nama baru berbeda dari nama di sesi, BFF sudah memasang
 * cookie baru, jadi `router.refresh()` memuat ulang sesi di layout root (nama di topbar ikut berubah).
 * Pesan galat dan toast diurus pemanggil.
 */
export function useUbahProfil() {
  const qc = useQueryClient();
  const router = useRouter();
  const sesi = useOptionalSession();
  return useMutation({
    mutationFn: async (data: UpdateProfilRequest) =>
      (
        await bffFetch<{ user: MeUserResponse }>("/api/auth/me", {
          method: "PATCH",
          body: JSON.stringify(data),
        })
      ).user,
    onMutate: async (data) => {
      await qc.cancelQueries({ queryKey: qk.profil.saya() });
      const sebelum = qc.getQueryData<MeUserResponse>(qk.profil.saya());
      if (sebelum) {
        qc.setQueryData(qk.profil.saya(), terapkanPerubahan(sebelum, data));
      }
      return { sebelum };
    },
    onError: (_err, _data, konteks) => {
      if (konteks?.sebelum) qc.setQueryData(qk.profil.saya(), konteks.sebelum);
    },
    onSuccess: (user) => {
      qc.setQueryData(qk.profil.saya(), user);
      if (sesi && sesi.nama !== user.nama) router.refresh();
    },
    // Tanpa mengembalikan promise-nya: React Query menunggu promise callback, dan simpan tidak boleh ikut menunggu
    // muat ulang (form tetap terkunci selama GET lambat).
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: qk.profil.saya() });
    },
  });
}

/**
 * Nama kabupaten/kota dari kode BPS. Master wilayah (izin `master.lihat`) bila boleh dibaca; peran lain, atau
 * saat master belum termuat, memakai data acuan bawaan. Tanpa kode (cakupan provinsi) hasilnya undefined.
 */
export function useNamaKabupaten(kode: string | undefined): string | undefined {
  const sesi = useOptionalSession();
  const bolehBaca = !MODE_MOCK && !!kode && can(sesi, "master.lihat");
  const { data } = useMasterWilayah(bolehBaca);
  if (!kode) return undefined;
  return data?.find((w) => w.kode === kode)?.nama ?? namaWilayah(kode);
}
