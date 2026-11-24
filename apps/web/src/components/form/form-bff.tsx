"use client";

import type { ComponentProps, FormEvent, FormHTMLAttributes } from "react";
import { cn } from "@/lib/utils";

type AtributForm = Omit<
  FormHTMLAttributes<HTMLFormElement>,
  "action" | "method" | "encType" | "target" | "onSubmit" | "noValidate"
>;

/**
 * Elemen `<form>` aplikasi. Tidak menerima `action`/`method` (tipe dan aturan ESLint menolaknya), selalu
 * `noValidate` (validasi ditampilkan sendiri oleh form), dan mencegah kiriman native browser walaupun
 * handler-nya lupa: tidak ada reload dan tidak ada POST di riwayat. Pakai bersama `useFormBff`:
 *
 *   const form = useFormBff({ kirim: (n) => bffFetch(...), sukses: () => "/tujuan" });
 *   <FormBff onKirim={form.kirimDari(() => ({ email, sandi }))} sibuk={form.pending}>…</FormBff>
 */
export function FormBff({
  onKirim,
  sibuk,
  children,
  ...atribut
}: AtributForm & {
  onKirim: (e: FormEvent<HTMLFormElement>) => void;
  /** Menandai form sedang mengirim (aria-busy) untuk teknologi bantu. */
  sibuk?: boolean;
}) {
  return (
    // eslint-disable-next-line no-restricted-syntax -- satu-satunya <form> mentah: pembungkus ini sendiri.
    <form
      {...atribut}
      noValidate
      aria-busy={sibuk || undefined}
      onSubmit={(e) => {
        e.preventDefault();
        onKirim(e);
      }}
    >
      {children}
    </form>
  );
}

/**
 * `FormBff` sebagai anak langsung `DialogContent`: kolom flex yang mewarisi batas tinggi modal, sehingga
 * `DialogHeader`, `DialogBody`, dan `DialogFooter` di dalamnya tersusun sama seperti tanpa form.
 */
export function FormDialog({
  className,
  ...props
}: ComponentProps<typeof FormBff>) {
  return (
    <FormBff
      {...props}
      className={cn("flex min-h-0 min-w-0 flex-1 flex-col gap-4", className)}
    />
  );
}

/** Id elemen pesan galat field; pasangkan ke `aria-describedby` input-nya. */
export const idGalatField = (field: string) => `${field}-galat`;

/** Pesan galat satu field (dari skema klien atau 400/422 server); tidak merender apa pun tanpa pesan. */
export function PesanField({
  field,
  pesan,
}: {
  field: string;
  pesan?: string;
}) {
  if (!pesan) return null;
  return (
    <p
      id={idGalatField(field)}
      className="text-xs font-medium text-destructive"
    >
      {pesan}
    </p>
  );
}
