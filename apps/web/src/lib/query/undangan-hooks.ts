"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  undanganApi,
  unduhEksporPengguna,
  type FilterEkspor,
  type HasilUndangan,
} from "@/lib/api/pengguna-undangan";
import type { UndangPenggunaInput } from "@/lib/pengguna/undang";
import { qk } from "./keys";

const pesan = (err: unknown) =>
  err instanceof Error ? err.message : "Coba lagi beberapa saat.";

/** Toast hasil undangan: email terkirim atau belum (akun tetap ada, kirim ulang undangannya). */
function toastUndangan(judul: string, h: HasilUndangan) {
  if (h.emailTerkirim) {
    toast.success(judul, {
      description: `Email berisi tautan aktivasi dikirim ke ${h.pengguna.email}. Berlaku 72 jam.`,
    });
  } else {
    toast.warning(`${judul}, tetapi email belum terkirim`, {
      description: `Akun ${h.pengguna.email} sudah ada. Pilih "Kirim ulang undangan" di menu baris setelah SMTP pulih.`,
    });
  }
}

/** Mengundang pengguna lewat email: akun MENUNGGU tanpa kata sandi. */
export function useUndangPengguna() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: UndangPenggunaInput) => undanganApi.undang(input),
    onSuccess: (hasil) => {
      void qc.invalidateQueries({ queryKey: qk.pengguna.all() });
      toastUndangan("Undangan dikirim", hasil);
    },
    // Galat tampil di dialog (useFormBff); toast hanya untuk jalur lain.
  });
}

/** Kirim ulang undangan akun MENUNGGU: tautan lama hangus. */
export function useUndangUlangPengguna() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id }: { id: string; nama: string }) =>
      undanganApi.undangUlang(id),
    onSuccess: (hasil) => {
      void qc.invalidateQueries({ queryKey: qk.pengguna.all() });
      toastUndangan("Undangan dikirim ulang", hasil);
    },
    onError: (err) => {
      toast.error("Gagal mengirim ulang undangan", { description: pesan(err) });
    },
  });
}

/** Memeriksa (dryRun) atau mengimpor CSV pengguna. Komit menyegarkan daftar; galat ditampilkan dialog. */
export function useImporPengguna() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ csv, dryRun }: { csv: string; dryRun: boolean }) =>
      undanganApi.impor(csv, dryRun),
    onSuccess: (hasil) => {
      if (!hasil.dryRun)
        void qc.invalidateQueries({ queryKey: qk.pengguna.all() });
    },
  });
}

/**
 * Mengunduh CSV pengguna dari server (tercatat di audit trail). `jalankan` tidak pernah menolak: galat
 * ditampilkan sebagai toast, sehingga aman dipanggil dari aksi massal.
 */
export function useEksporPengguna() {
  const mut = useMutation({
    mutationFn: (filter: FilterEkspor) => unduhEksporPengguna(filter),
    onSuccess: ({ baris, terpotong }) => {
      toast.success("Ekspor pengguna selesai", {
        description: terpotong
          ? `${baris.toLocaleString("id-ID")} baris (dipotong pada batas server; persempit dengan pencarian). Tercatat di audit trail.`
          : `${baris.toLocaleString("id-ID")} baris. Tercatat di audit trail.`,
      });
    },
    onError: (err) => {
      toast.error("Gagal mengekspor pengguna", { description: pesan(err) });
    },
  });
  return {
    pending: mut.isPending,
    jalankan: (filter: FilterEkspor = {}) =>
      mut.mutateAsync(filter).catch(() => undefined),
  };
}
