"use client";

import { useState } from "react";
import { FormBff, PesanField, idGalatField } from "@/components/form/form-bff";
import { SpringButton } from "@/components/garda/spring-button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  BATAS,
  pesanGalatMaster,
  validasiAlasan,
} from "@/lib/api/master-perubahan";
import { skemaDariValidasi } from "@/lib/form/skema";
import { useFormBff } from "@/lib/form/use-form-bff";

interface IsianAlasan {
  alasan: string;
}

/** Wajib: 3–1000 karakter (aturan server). Opsional: kosong boleh, paling banyak 1000. */
const skemaWajib = skemaDariValidasi<IsianAlasan>((i) => {
  const g = validasiAlasan(i.alasan);
  return g ? { alasan: g } : null;
});
const skemaOpsional = skemaDariValidasi<IsianAlasan>((i) =>
  i.alasan.trim().length > BATAS.alasan.max
    ? { alasan: `Alasan paling banyak ${BATAS.alasan.max} karakter.` }
    : null,
);

interface Props {
  judul: string;
  deskripsi: string;
  label: string;
  tombol: string;
  wajib: boolean;
  onKirim: (alasan: string) => Promise<unknown>;
}

/**
 * Dialog satu isian alasan, untuk menolak (wajib) atau membatalkan (opsional) usulan. Galat server (mis.
 * 409 BUKAN_MENUNGGU, 403 PEMISAHAN_TUGAS) tampil di dalam dialog dan dialog tetap terbuka.
 */
export function AlasanDialog({
  open,
  onOpenChange,
  ...props
}: Props & { open: boolean; onOpenChange: (open: boolean) => void }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <IsiAlasan {...props} onOpenChange={onOpenChange} />
      </DialogContent>
    </Dialog>
  );
}

function IsiAlasan({
  judul,
  deskripsi,
  label,
  tombol,
  wajib,
  onKirim,
  onOpenChange,
}: Props & { onOpenChange: (open: boolean) => void }) {
  const [alasan, setAlasan] = useState("");
  const form = useFormBff<IsianAlasan>({
    skema: wajib ? skemaWajib : skemaOpsional,
    kirim: (n) => onKirim(n.alasan.trim()),
    sukses: () => onOpenChange(false),
    petaGalat: (err) => ({ pesan: pesanGalatMaster(err) }),
  });
  return (
    <FormBff
      onKirim={form.kirimDari(() => ({ alasan }))}
      sibuk={form.pending}
      className="space-y-4"
    >
      <DialogHeader>
        <DialogTitle>{judul}</DialogTitle>
        <DialogDescription>{deskripsi}</DialogDescription>
      </DialogHeader>
      <div className="space-y-1.5">
        <Label htmlFor="alasan-keputusan">{label}</Label>
        <Textarea
          id="alasan-keputusan"
          value={alasan}
          rows={4}
          maxLength={BATAS.alasan.max}
          onChange={(e) => setAlasan(e.target.value)}
          aria-invalid={!!form.galatField.alasan}
          aria-describedby={
            form.galatField.alasan ? idGalatField("alasan") : undefined
          }
        />
        <PesanField field="alasan" pesan={form.galatField.alasan} />
      </div>
      {form.galat && (
        <p role="alert" className="text-sm text-defisit-ink dark:text-rose-300">
          {form.galat.pesan}
        </p>
      )}
      <DialogFooter>
        <SpringButton
          variant="soft"
          onClick={() => onOpenChange(false)}
          disabled={form.pending}
        >
          Kembali
        </SpringButton>
        <SpringButton
          type="submit"
          variant={wajib ? "danger" : "primary"}
          disabled={form.pending}
        >
          {form.pending ? "Menyimpan…" : tombol}
        </SpringButton>
      </DialogFooter>
    </FormBff>
  );
}
