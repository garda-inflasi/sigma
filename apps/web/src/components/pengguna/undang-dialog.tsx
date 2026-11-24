"use client";

import { Mail } from "lucide-react";
import { useState } from "react";
import { FormDialog, idGalatField } from "@/components/form/form-bff";
import { FormAlert, FormField } from "@/components/form/form-field";
import { SpringButton } from "@/components/garda/spring-button";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { KABUPATEN } from "@/lib/data/reference";
import type { GalatForm } from "@/lib/form/kiriman";
import { useFormBff } from "@/lib/form/use-form-bff";
import {
  KABUPATEN_SEMUA,
  isianUndangAwal,
  keInputUndang,
  peranButuhKabupaten,
  skemaUndang,
  type IsianUndang,
  type NilaiUndang,
} from "@/lib/pengguna/undang";
import { useUndangPengguna } from "@/lib/query/undangan-hooks";
import { LAPIS_META, ROLES, ROLE_IDS, type Role } from "@/lib/rbac/roles";

const KABUPATEN_URUT = [...KABUPATEN].sort((a, b) =>
  a.nama.localeCompare(b.nama, "id"),
);

function FormUndang({
  onOpenChange,
  onBuatSementara,
}: {
  onOpenChange: (v: boolean) => void;
  onBuatSementara?: () => void;
}) {
  const undang = useUndangPengguna();
  const [form, setForm] = useState<IsianUndang>(isianUndangAwal);

  const kiriman = useFormBff<NilaiUndang, unknown, GalatForm, IsianUndang>({
    skema: skemaUndang,
    kirim: (n) => undang.mutateAsync(keInputUndang(n)),
    sukses: () => onOpenChange(false),
  });
  const sibuk = kiriman.pending;
  const g = kiriman.galatField;
  const atributGalat = (field: keyof IsianUndang) =>
    g[field]
      ? {
          "aria-invalid": true,
          "aria-describedby": idGalatField(`und-${field}`),
        }
      : {};
  const butuhKabupaten = peranButuhKabupaten(form.peran as Role);

  return (
    <FormDialog onKirim={kiriman.kirimDari(() => form)} sibuk={sibuk}>
      <DialogHeader>
        <DialogTitle>Undang pengguna</DialogTitle>
        <DialogDescription>
          Pengguna menerima email berisi tautan untuk menetapkan kata sandinya
          sendiri. Tautan berlaku 72 jam dan hanya dapat dipakai satu kali.
        </DialogDescription>
      </DialogHeader>

      <DialogBody>
        <FormField id="und-nama" label="Nama lengkap" galat={g.nama}>
          <Input
            id="und-nama"
            autoComplete="off"
            value={form.nama}
            onChange={(e) => setForm({ ...form, nama: e.target.value })}
            {...atributGalat("nama")}
          />
        </FormField>
        <FormField id="und-email" label="Email" galat={g.email}>
          <Input
            id="und-email"
            type="email"
            autoComplete="off"
            value={form.email}
            onChange={(e) => setForm({ ...form, email: e.target.value })}
            {...atributGalat("email")}
          />
        </FormField>
        <FormField id="und-peran" label="Peran">
          <Select
            value={form.peran}
            onValueChange={(v) =>
              setForm({ ...form, peran: v as Role, kabupaten: KABUPATEN_SEMUA })
            }
          >
            <SelectTrigger id="und-peran">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {ROLE_IDS.map((r) => (
                <SelectItem key={r} value={r}>
                  {ROLES[r].label} —{" "}
                  {LAPIS_META[ROLES[r].lapis].label.split(" — ")[0]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </FormField>
        <FormField id="und-lembaga" label="Lembaga" galat={g.lembaga}>
          <Input
            id="und-lembaga"
            autoComplete="off"
            value={form.lembaga}
            onChange={(e) => setForm({ ...form, lembaga: e.target.value })}
            {...atributGalat("lembaga")}
          />
        </FormField>
        {butuhKabupaten && (
          <FormField
            id="und-kabupaten"
            label="Kabupaten/kota"
            opsional
            bantuan="Cakupan data peran ini dibatasi pada kabupaten/kota yang dipilih."
          >
            <Select
              value={form.kabupaten}
              onValueChange={(v) => setForm({ ...form, kabupaten: v })}
            >
              <SelectTrigger id="und-kabupaten">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={KABUPATEN_SEMUA}>Seluruh Aceh</SelectItem>
                {KABUPATEN_URUT.map((k) => (
                  <SelectItem key={k.kode} value={k.kode}>
                    {k.jenis === "Kota" ? "Kota" : "Kab."} {k.nama}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </FormField>
        )}
        <FormAlert>{kiriman.galat?.pesan}</FormAlert>
      </DialogBody>

      <DialogFooter
        bantuan={
          onBuatSementara ? (
            <Button
              variant="link"
              size="sm"
              className="h-auto min-h-6 max-w-full justify-start px-0 py-0.5 text-left text-xs font-normal whitespace-normal text-muted-foreground hover:text-foreground"
              onClick={onBuatSementara}
              disabled={sibuk}
            >
              Email tidak bisa dipakai? Buat dengan kata sandi sementara
            </Button>
          ) : undefined
        }
      >
        <Button
          variant="outline"
          onClick={() => onOpenChange(false)}
          disabled={sibuk}
        >
          Batal
        </Button>
        <SpringButton
          type="submit"
          variant="primary"
          size="md"
          disabled={sibuk}
        >
          <Mail /> {sibuk ? "Mengirim…" : "Kirim undangan"}
        </SpringButton>
      </DialogFooter>
    </FormDialog>
  );
}

/**
 * Dialog "Undang pengguna": cara utama menambah akun. Akun dibuat berstatus "Menunggu aktivasi" tanpa kata
 * sandi; pemilik email menetapkannya sendiri lewat tautan. `onBuatSementara` membuka dialog lama (kata sandi
 * sementara) untuk lingkungan tanpa email atau pengguna yang tidak punya akses email.
 */
export function UndangDialog({
  open,
  onOpenChange,
  onBuatSementara,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  onBuatSementara?: () => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <FormUndang
          onOpenChange={onOpenChange}
          onBuatSementara={onBuatSementara}
        />
      </DialogContent>
    </Dialog>
  );
}
