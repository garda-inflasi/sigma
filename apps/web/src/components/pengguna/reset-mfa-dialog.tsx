"use client";

import { z } from "zod";
import { FormBff } from "@/components/form/form-bff";
import { SpringButton } from "@/components/garda/spring-button";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { PenggunaRow } from "@/lib/data/types";
import { useFormBff } from "@/lib/form/use-form-bff";
import { useResetMfaPengguna } from "@/lib/query/hooks";
import { ROLES } from "@/lib/rbac/roles";

const skemaKosong = z.object({});

/**
 * Konfirmasi reset MFA oleh admin. Menjelaskan akibatnya sebelum dijalankan karena tindakan ini melepas faktor
 * kedua akun dan mengakhiri semua sesinya. Toast berhasil/gagal datang dari hook mutasi; galat juga tampil di dialog.
 */
export function ResetMfaDialog({
  user,
  open,
  onOpenChange,
}: {
  user: PenggunaRow | null;
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const reset = useResetMfaPengguna();
  const kiriman = useFormBff<z.output<typeof skemaKosong>>({
    skema: skemaKosong,
    kirim: () => reset.mutateAsync({ id: user!.id, nama: user!.nama }),
    sukses: () => onOpenChange(false),
  });
  const wajib = user ? ROLES[user.role].mfaWajib : false;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <FormBff
          onKirim={kiriman.kirimDari(() => ({}))}
          sibuk={kiriman.pending}
          className="contents"
        >
          <DialogHeader>
            <DialogTitle>Reset MFA</DialogTitle>
            <DialogDescription>
              Hapus autentikator {user?.nama} karena perangkatnya hilang atau
              tidak bisa dipakai lagi.
            </DialogDescription>
          </DialogHeader>
          <ul className="grid list-disc gap-1.5 py-2 pl-5 text-sm text-muted-foreground">
            <li>
              MFA dinonaktifkan; aplikasi autentikator yang terdaftar dan semua
              kode cadangan dihapus.
            </li>
            <li>
              Seluruh sesi aktif {user?.nama} diakhiri seketika di semua
              perangkat.
            </li>
            <li>
              {wajib
                ? "Peran ini wajib MFA: pada masuk berikutnya pengguna harus mendaftarkan autentikator baru sebelum bisa memakai aplikasi."
                : "Pengguna masuk dengan kata sandi saja sampai ia mendaftarkan autentikator baru."}
            </li>
            <li>Tindakan ini tercatat di audit trail.</li>
          </ul>
          {kiriman.galat && (
            <p role="alert" className="text-sm text-destructive">
              {kiriman.galat.pesan}
            </p>
          )}
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={kiriman.pending}
            >
              Batal
            </Button>
            <SpringButton
              type="submit"
              variant="danger"
              size="md"
              disabled={kiriman.pending}
            >
              {kiriman.pending ? "Mereset…" : "Reset MFA"}
            </SpringButton>
          </DialogFooter>
        </FormBff>
      </DialogContent>
    </Dialog>
  );
}
