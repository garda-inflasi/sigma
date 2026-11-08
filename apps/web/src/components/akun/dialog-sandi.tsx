"use client";

import { Loader2 } from "lucide-react";
import type { FormEvent, ReactNode } from "react";
import { KolomSandi } from "@/components/auth/kolom-sandi";
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

/**
 * Dialog konfirmasi dengan kata sandi untuk aksi keamanan yang sensitif (kode cadangan baru, nonaktifkan MFA).
 * Isian dan kirimannya dikelola pemanggil; dialog tidak bisa ditutup selagi kiriman berjalan.
 */
export function DialogSandi({
  open,
  onTutup,
  judul,
  deskripsi,
  tombol,
  bahaya = false,
  sandi,
  onSandi,
  onKirim,
  sibuk,
  galatField,
  galat,
}: {
  open: boolean;
  onTutup: () => void;
  judul: string;
  deskripsi: ReactNode;
  /** Teks tombol konfirmasi. */
  tombol: string;
  /** Aksi merusak: tombol konfirmasi berwarna bahaya. */
  bahaya?: boolean;
  sandi: string;
  onSandi: (nilai: string) => void;
  onKirim: (e: FormEvent<HTMLFormElement>) => void;
  sibuk: boolean;
  /** Galat isian kata sandi (skema klien atau 400/422 server). */
  galatField?: string;
  /** Galat tingkat form (mis. kata sandi salah, akun terkunci). */
  galat?: string | null;
}) {
  return (
    <Dialog open={open} onOpenChange={(v) => !v && !sibuk && onTutup()}>
      <DialogContent className="sm:max-w-md">
        <FormBff onKirim={onKirim} sibuk={sibuk} className="contents">
          <DialogHeader>
            <DialogTitle>{judul}</DialogTitle>
            <DialogDescription>{deskripsi}</DialogDescription>
          </DialogHeader>
          <KolomSandi
            id="sandi"
            label="Kata sandi (untuk mengonfirmasi)"
            value={sandi}
            onChange={onSandi}
            disabled={sibuk}
            galat={galatField}
            autoComplete="current-password"
          />
          {/* Galat isian sudah tampil di bawah kolom; galat tingkat form hanya bila tidak ada galat isian. */}
          {galat && !galatField && (
            <p role="alert" className="text-sm font-medium text-destructive">
              {galat}
            </p>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={onTutup} disabled={sibuk}>
              Batal
            </Button>
            <SpringButton
              type="submit"
              variant={bahaya ? "danger" : "primary"}
              size="md"
              disabled={sibuk || !sandi}
            >
              {sibuk && <Loader2 className="animate-spin" aria-hidden />}
              {tombol}
            </SpringButton>
          </DialogFooter>
        </FormBff>
      </DialogContent>
    </Dialog>
  );
}
