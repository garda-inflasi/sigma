"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  KODE_PEMEGANG_TERAKHIR,
  KODE_VERSI_PERAN_BERUBAH,
  kodeGalat,
  rbacApi,
  type BeriIzinInput,
  type PeranAkses,
} from "@/lib/api/rbac";

/** Kunci query RBAC. Prefiks ["rbac"] juga memuat katalog baca-saja (qk.rbac.katalog): satu invalidasi menyegarkan keduanya. */
export const kunciRbac = {
  semua: ["rbac"] as const,
  matriks: ["rbac", "matriks"] as const,
  izinPengguna: (id: string) => ["rbac", "izin-pengguna", id] as const,
};

const pesanGalat = (err: unknown) =>
  err instanceof Error ? err.message : "Coba lagi beberapa saat.";

/** Matriks peran × izin dari basis data (dengan versi tiap peran), dibaca segar agar versinya tidak basi. */
export const useMatriksAkses = () =>
  useQuery({
    queryKey: kunciRbac.matriks,
    queryFn: rbacApi.matriks,
    staleTime: 0,
  });

export type HasilSimpan = "tersimpan" | "konflik" | "ditolak";

/**
 * Menyimpan seluruh izin satu peran. Mengembalikan hasilnya alih-alih melempar supaya komponen menentukan nasib drafnya:
 * konflik versi (admin lain menyimpan lebih dulu) membuang draf dan memuat ulang matriks; penolakan lain
 * (mis. pemegang terakhir pengguna.kelola) mempertahankan draf agar dapat diperbaiki.
 */
export function useSimpanPeran() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      peran,
      izin,
    }: {
      peran: PeranAkses;
      izin: string[];
    }): Promise<HasilSimpan> => {
      try {
        await rbacApi.simpanPeran(peran.id, peran.versi, izin);
        return "tersimpan";
      } catch (err) {
        const kode = kodeGalat(err);
        if (kode === KODE_VERSI_PERAN_BERUBAH) return "konflik";
        if (kode === KODE_PEMEGANG_TERAKHIR) return "ditolak";
        throw err;
      }
    },
    onSuccess: (hasil, { peran }) => {
      qc.invalidateQueries({ queryKey: kunciRbac.semua });
      if (hasil === "tersimpan") {
        toast.success(`Matriks ${peran.label} disimpan`, {
          description:
            "Berlaku pada request berikutnya; token yang sedang dipakai disegarkan otomatis. Tercatat di audit trail.",
        });
      } else if (hasil === "konflik") {
        toast.warning(`Matriks ${peran.label} sudah diubah admin lain`, {
          description:
            "Data dimuat ulang dan draf Anda untuk peran ini dibuang. Periksa lalu ulangi perubahan.",
        });
      } else {
        toast.error("Perubahan ditolak", {
          description:
            "Sistem harus tetap punya pengguna aktif yang memegang izin pengguna.kelola lewat perannya.",
        });
      }
    },
    onError: (err, { peran }) => {
      toast.error(`Gagal menyimpan matriks ${peran.label}`, {
        description: pesanGalat(err),
      });
    },
  });
}

/** Izin seorang pengguna (dari peran, langsung, dan yang berlaku); mati bila `id` kosong. Selalu segar. */
export const useIzinPengguna = (id: string | null) =>
  useQuery({
    queryKey: kunciRbac.izinPengguna(id ?? ""),
    queryFn: () => rbacApi.izinPengguna(id as string),
    enabled: !!id,
    retry: false,
    staleTime: 0,
  });

export function useBeriIzin(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: BeriIzinInput) => rbacApi.beriIzin(id, input),
    onSuccess: (g) => {
      qc.invalidateQueries({ queryKey: kunciRbac.izinPengguna(id) });
      toast.success("Izin langsung diberikan", {
        description: `${g.izin} · tercatat di audit trail.`,
      });
    },
    onError: (err) => {
      toast.error("Gagal memberi izin langsung", {
        description: pesanGalat(err),
      });
    },
  });
}

export function useCabutIzin(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (izin: string) => rbacApi.cabutIzin(id, izin),
    onSuccess: (_res, izin) => {
      qc.invalidateQueries({ queryKey: kunciRbac.izinPengguna(id) });
      toast.success("Izin langsung dicabut", {
        description: `${izin} · tercatat di audit trail.`,
      });
    },
    onError: (err) => {
      toast.error("Gagal mencabut izin langsung", {
        description: pesanGalat(err),
      });
    },
  });
}
