"use client";

import { Loader2, Lock, Save } from "lucide-react";
import { useState, type ChangeEvent, type ReactNode } from "react";
import { toast } from "sonner";
import { FormBff, PesanField, idGalatField } from "@/components/form/form-bff";
import { PanelTitle } from "@/components/garda/page-header";
import { SpringButton } from "@/components/garda/spring-button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { MeUserResponse, UpdateProfilRequest } from "@/lib/bff/api-client";
import {
  isianDari,
  periksaDraf,
  type DataDiri,
  type DrafProfil,
  type IsianProfil,
} from "@/lib/profil/isian";
import { useFormBff } from "@/lib/form/use-form-bff";
import { useUbahProfil } from "@/lib/query/profil-hooks";
import { cn } from "@/lib/utils";

const idPetunjuk = (id: string) => `${id}-petunjuk`;

/** Label, kontrol, dan pesan (galat bila ada, selain itu petunjuk) satu isian. */
function Kolom({
  id,
  label,
  petunjuk,
  galat,
  className,
  children,
}: {
  id: string;
  label: string;
  petunjuk?: string;
  galat?: string;
  className?: string;
  children: ReactNode;
}) {
  return (
    <div className={cn("flex flex-col gap-2", className)}>
      <Label htmlFor={id}>{label}</Label>
      {children}
      {galat ? (
        <PesanField field={id} pesan={galat} />
      ) : (
        petunjuk && (
          <p id={idPetunjuk(id)} className="text-xs text-muted-foreground">
            {petunjuk}
          </p>
        )
      )}
    </div>
  );
}

const atributIsian = (
  id: string,
  galat: string | undefined,
  petunjuk: boolean,
) => ({
  "aria-invalid": galat ? true : undefined,
  "aria-describedby": galat
    ? idGalatField(id)
    : petunjuk
      ? idPetunjuk(id)
      : undefined,
});

/**
 * Form "Data pribadi": nama dan telepon dapat diubah, email hanya-baca. Yang tampil di kotak isian adalah
 * draf ketikan atau, untuk field yang belum disentuh, nilai tersimpan, sehingga data yang diperbarui server
 * langsung tampil tanpa menyalin state. Validasi memakai aturan API dan tampil selagi mengetik; tombol simpan
 * baru aktif bila ada perubahan yang sah.
 */
export function FormDataPribadi({
  tersimpan,
  email,
  bacaSaja = false,
}: {
  tersimpan: DataDiri;
  email: string;
  /** Mode demo: perubahan tidak tersedia, seluruh isian dan tombol nonaktif. */
  bacaSaja?: boolean;
}) {
  const mutasi = useUbahProfil();
  const [draf, setDraf] = useState<DrafProfil>({});

  const dasar = isianDari(tersimpan);
  const nilai: IsianProfil = { ...dasar, ...draf };
  const { perubahan, galat } = periksaDraf(tersimpan, draf);
  const adaPerubahan = Object.keys(perubahan).length > 0;
  const valid = Object.keys(galat).length === 0;
  const berbedaDariTersimpan =
    nilai.nama !== dasar.nama || nilai.telepon !== dasar.telepon;

  const form = useFormBff<UpdateProfilRequest, MeUserResponse>({
    kirim: (p) => mutasi.mutateAsync(p),
    sukses: () => {
      toast.success("Profil diperbarui");
      setDraf({});
    },
    judulToastGagal: "Gagal menyimpan profil",
  });
  const menyimpan = form.pending;
  const bisaSimpan = adaPerubahan && valid && !menyimpan && !bacaSaja;

  const ketik =
    (kunci: keyof IsianProfil) => (e: ChangeEvent<HTMLInputElement>) => {
      if (form.galat) form.resetGalat();
      setDraf((d) => ({ ...d, [kunci]: e.target.value }));
    };
  const batalkan = () => {
    form.resetGalat();
    setDraf({});
  };

  return (
    <FormBff
      onKirim={(e) => {
        if (bisaSimpan) form.kirimDari(() => perubahan)(e);
      }}
      sibuk={menyimpan}
      className="glass rounded-xl p-5 sm:p-6"
    >
      <PanelTitle
        title="Data pribadi"
        description="Nama dan nomor telepon dapat Anda ubah sendiri."
      />

      {bacaSaja && (
        <p
          role="note"
          className="mt-4 rounded-lg bg-muted px-3 py-2 text-sm text-muted-foreground"
        >
          Perubahan profil tidak tersedia dalam mode demo
        </p>
      )}

      <div className="mt-5 grid gap-4 md:grid-cols-2">
        <Kolom id="profil-nama" label="Nama lengkap" galat={galat.nama}>
          <Input
            id="profil-nama"
            value={nilai.nama}
            onChange={ketik("nama")}
            autoComplete="name"
            disabled={bacaSaja}
            readOnly={menyimpan}
            aria-required="true"
            className="h-11"
            {...atributIsian("profil-nama", galat.nama, false)}
          />
        </Kolom>

        <Kolom
          id="profil-telepon"
          label="Nomor telepon / WhatsApp"
          petunjuk="Opsional. Boleh diawali 08, 62, atau +62."
          galat={galat.telepon}
        >
          <Input
            id="profil-telepon"
            type="tel"
            inputMode="tel"
            autoComplete="tel"
            placeholder="0812-3456-7890"
            value={nilai.telepon}
            onChange={ketik("telepon")}
            disabled={bacaSaja}
            readOnly={menyimpan}
            className="h-11"
            {...atributIsian("profil-telepon", galat.telepon, true)}
          />
        </Kolom>

        <Kolom id="profil-email" label="Email" className="md:col-span-2">
          <div className="relative">
            <Input
              id="profil-email"
              type="email"
              value={email}
              readOnly
              autoComplete="email"
              aria-describedby="profil-email-catatan"
              className="h-11 bg-muted/60 pr-10 text-muted-foreground"
            />
            <Lock
              aria-hidden
              className="pointer-events-none absolute top-1/2 right-3.5 size-4 -translate-y-1/2 text-muted-foreground"
            />
          </div>
          <p
            id="profil-email-catatan"
            className="text-xs text-muted-foreground"
          >
            Hubungi administrator untuk perubahan email instansi
          </p>
        </Kolom>
      </div>

      {form.galat && (
        <p
          role="alert"
          className="mt-4 rounded-lg bg-defisit-soft/80 px-3 py-2 text-sm text-defisit-ink dark:bg-defisit/15 dark:text-rose-300"
        >
          {form.galat.pesan}
        </p>
      )}

      <div className="mt-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <p aria-live="polite" className="min-h-4 text-xs text-muted-foreground">
          {adaPerubahan && valid ? "Ada perubahan yang belum disimpan." : ""}
        </p>
        <div className="flex flex-col-reverse gap-2 sm:flex-row">
          <SpringButton
            type="button"
            variant="outline"
            className="h-11 px-5"
            disabled={!berbedaDariTersimpan || menyimpan || bacaSaja}
            onClick={batalkan}
          >
            Batalkan perubahan
          </SpringButton>
          <SpringButton
            type="submit"
            variant="primary"
            className="h-11 px-5"
            disabled={!bisaSimpan}
          >
            {menyimpan ? <Loader2 className="animate-spin" /> : <Save />}
            Simpan perubahan
          </SpringButton>
        </div>
      </div>
    </FormBff>
  );
}
