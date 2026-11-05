"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { MODE_MOCK } from "@/lib/api/client";
import { penggunaApi } from "@/lib/api/pengguna";
import { keamananApi } from "@/lib/api/sesi";
import { qk } from "./keys";

const pesan = (err: unknown) =>
  err instanceof Error ? err.message : "Coba lagi beberapa saat.";

/** Sesi aktif milik sendiri. Mode mock tidak punya sesi server. */
export function useSesiAktif() {
  return useQuery({
    queryKey: qk.sesi.aktif(),
    queryFn: keamananApi.daftar,
    enabled: !MODE_MOCK,
    // Data keamanan: selalu ambil yang terbaru saat halaman dibuka lagi.
    staleTime: 0,
  });
}

/** Cabut satu sesi lain ("Cabut"). */
export function useCabutSesi() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => keamananApi.cabut(id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: qk.sesi.aktif() });
      toast.success("Sesi dicabut", {
        description: "Perangkat itu harus masuk ulang.",
      });
    },
    onError: (err) => {
      void qc.invalidateQueries({ queryKey: qk.sesi.aktif() });
      toast.error("Gagal mencabut sesi", { description: pesan(err) });
    },
  });
}

/** Keluar dari semua perangkat lain; sesi ini tetap masuk. */
export function useCabutSesiLain() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => keamananApi.cabutLain(),
    onSuccess: ({ dicabut }) => {
      void qc.invalidateQueries({ queryKey: qk.sesi.aktif() });
      toast.success("Perangkat lain dikeluarkan", {
        description: `${dicabut.toLocaleString("id-ID")} sesi diakhiri.`,
      });
    },
    onError: (err) => {
      void qc.invalidateQueries({ queryKey: qk.sesi.aktif() });
      toast.error("Gagal mengeluarkan perangkat lain", {
        description: pesan(err),
      });
    },
  });
}

/** Paksa keluar seorang pengguna (admin, pengguna.kelola): semua sesinya diakhiri. */
export function usePaksaKeluarPengguna() {
  return useMutation({
    mutationFn: ({ id }: { id: string; nama: string }) =>
      penggunaApi.paksaKeluar(id),
    onSuccess: ({ dicabut }, { nama }) => {
      toast.success("Pengguna dipaksa keluar", {
        description:
          dicabut > 0
            ? `${nama}: ${dicabut.toLocaleString("id-ID")} sesi diakhiri. Tercatat di audit trail.`
            : `${nama} tidak memiliki sesi aktif.`,
      });
    },
    onError: (err) => {
      toast.error("Gagal memaksa keluar pengguna", { description: pesan(err) });
    },
  });
}
