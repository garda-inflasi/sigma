"use client";

import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { MODE_MOCK } from "@/lib/api/client";
import {
  KODE_VERSI_KEBIJAKAN_BERUBAH,
  impersonasiApi,
  kodeGalatImpersonasi,
  tujuanAman,
  type HasilAlihAkun,
  type KebijakanInput,
  type MulaiInput,
  type ParamRiwayat,
} from "@/lib/api/impersonasi";

/** Kunci query impersonasi. Prefiks ["impersonasi"] memuat semuanya: satu invalidasi menyegarkan keduanya. */
export const kunciImpersonasi = {
  semua: ["impersonasi"] as const,
  targets: ["impersonasi", "targets"] as const,
  kebijakan: ["impersonasi", "kebijakan"] as const,
  riwayat: (status: "semua" | "aktif") =>
    ["impersonasi", "riwayat", status] as const,
};

const pesanGalat = (err: unknown) =>
  err instanceof Error ? err.message : "Coba lagi beberapa saat.";

const LIMA_MENIT = 5 * 60_000;

/**
 * Pemilih sasaran bagi pemanggil. `boleh: false` (bukan galat) bila tidak berhak, jadi menu hanya perlu satu
 * permintaan yang di-cache selama sesi halaman. Mode demo (tanpa API) tidak punya BFF impersonasi. Galat
 * (API lama tanpa endpoint, jaringan) berarti "tidak tersedia": tidak diulang dan tidak menampilkan menu.
 */
export function useTargetImpersonasi(aktif = true) {
  return useQuery({
    queryKey: kunciImpersonasi.targets,
    queryFn: impersonasiApi.targets,
    enabled: aktif && !MODE_MOCK,
    staleTime: LIMA_MENIT,
    retry: false,
    refetchOnWindowFocus: false,
  });
}

/** Kebijakan terkini, dibaca segar supaya `versi` (kunci optimistis) tidak basi. */
export const useKebijakanImpersonasi = () =>
  useQuery({
    queryKey: kunciImpersonasi.kebijakan,
    queryFn: impersonasiApi.kebijakan,
    staleTime: 0,
    retry: false,
  });

export type HasilSimpanKebijakan = "tersimpan" | "konflik";

/**
 * Menyimpan seluruh kebijakan. Konflik versi (superadmin lain menyimpan lebih dulu) dikembalikan sebagai hasil,
 * bukan galat, supaya komponen membuang draf dan memuat ulang; penolakan lain dilempar dan draf dipertahankan.
 */
export function useSimpanKebijakan() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (
      input: KebijakanInput,
    ): Promise<HasilSimpanKebijakan> => {
      try {
        await impersonasiApi.simpanKebijakan(input);
        return "tersimpan";
      } catch (err) {
        if (kodeGalatImpersonasi(err) === KODE_VERSI_KEBIJAKAN_BERUBAH) {
          return "konflik";
        }
        throw err;
      }
    },
    onSuccess: (hasil) => {
      qc.invalidateQueries({ queryKey: kunciImpersonasi.semua });
      if (hasil === "tersimpan") {
        toast.success("Kebijakan impersonasi disimpan", {
          description:
            "Berlaku pada request berikutnya, termasuk sesi impersonasi yang sedang berjalan. Tercatat di audit trail.",
        });
      } else {
        toast.warning("Kebijakan sudah diubah superadmin lain", {
          description:
            "Data dimuat ulang dan draf Anda dibuang. Periksa lalu ulangi perubahan.",
        });
      }
    },
    onError: (err) => {
      toast.error("Gagal menyimpan kebijakan impersonasi", {
        description: pesanGalat(err),
      });
    },
  });
}

/** Riwayat impersonasi berkursor; `status` aktif hanya menampilkan sesi yang masih berjalan. */
export function useRiwayatImpersonasi(status: "semua" | "aktif") {
  return useInfiniteQuery({
    queryKey: kunciImpersonasi.riwayat(status),
    queryFn: ({ pageParam }) => {
      const param: ParamRiwayat = { status, limit: 20, cursor: pageParam };
      return impersonasiApi.riwayat(param);
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (halaman) => halaman.next_cursor || undefined,
    staleTime: 0,
    retry: false,
  });
}

/** Mengakhiri paksa satu sesi impersonasi aktif (superadmin). */
export function useAkhiriImpersonasi() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => impersonasiApi.akhiri(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: kunciImpersonasi.semua });
      toast.success("Sesi impersonasi diakhiri", {
        description:
          "Pelaku keluar dari sesi sasaran pada request berikutnya. Tercatat di audit trail.",
      });
    },
    onError: (err) => {
      toast.error("Gagal mengakhiri sesi impersonasi", {
        description: pesanGalat(err),
      });
    },
  });
}

/**
 * Titik pindah halaman sesudah berganti akun. Sengaja pemuatan penuh (seperti keluar, lib/auth/logout.ts), bukan
 * `router.replace` + `router.refresh()`: cache query, store EWS, dan sambungan aliran EWS milik identitas lama
 * tidak boleh terbawa ke identitas baru, dan layout (sidebar, `canAccessRoute`, topbar) harus membaca sesi baru
 * dari cookie. Objek (bukan fungsi lepas) supaya tes dapat menggantinya.
 */
export const navigasi = {
  pindah: (url: string) => window.location.assign(url),
};

/** Halaman yang sedang dibuka (path + query) untuk tujuan sesudah berpindah; BFF yang memutuskan boleh-tidaknya. */
function dariHalamanIni(): { dari?: string } {
  if (typeof window === "undefined") return {};
  return { dari: `${window.location.pathname}${window.location.search}` };
}

/**
 * Berpindah akun: mulai atau ganti impersonasi, atau kembali ke akun asli. Cookie sesi baru sudah dipasang BFF;
 * sesudah sukses cache query dibuang dan halaman dimuat penuh ke tujuan aman yang ditentukan BFF.
 * `sibuk` menahan klik ganda selama permintaan atau pemindahan berjalan.
 */
export function useAlihAkun() {
  const qc = useQueryClient();
  const [pindah, setPindah] = useState(false);

  const sesudah = (hasil: HasilAlihAkun) => {
    setPindah(true);
    qc.clear();
    navigasi.pindah(tujuanAman(hasil.redirectTo));
  };

  const mulai = useMutation({
    mutationFn: (input: MulaiInput) =>
      impersonasiApi.mulai({ ...input, ...dariHalamanIni() }),
    onSuccess: sesudah,
  });
  const berhenti = useMutation({
    mutationFn: () => impersonasiApi.berhenti(dariHalamanIni().dari),
    onSuccess: sesudah,
    onError: (err) => {
      toast.error("Gagal kembali ke peran asal", {
        description: pesanGalat(err),
      });
    },
  });

  return {
    mulai,
    berhenti,
    sibuk: mulai.isPending || berhenti.isPending || pindah,
  };
}
