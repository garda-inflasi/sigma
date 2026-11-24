"use client";

import { Info, Loader2, Lock, RotateCcw, Save } from "lucide-react";
import { useState, type ChangeEvent } from "react";
import { toast } from "sonner";
import { FormBff, idGalatField } from "@/components/form/form-bff";
import { FormAlert, FormField } from "@/components/form/form-field";
import { PanelTitle } from "@/components/garda/page-header";
import { SpringButton } from "@/components/garda/spring-button";
import { Input } from "@/components/ui/input";
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

const idPetunjuk = (id: string) => `${id}-petunjuk`;

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
      className="glass scrollbar-thin flex min-w-0 flex-col rounded-xl p-4 sm:p-5 lg:max-h-full lg:min-h-0 lg:overflow-y-auto"
    >
      <PanelTitle
        title="Data pribadi"
        description="Nama tampilan dan kontak WhatsApp/telepon yang dapat Anda perbarui sendiri."
      />

      {bacaSaja && (
        <div
          role="note"
          className="mt-4 flex items-center gap-2 rounded-lg border border-border bg-secondary/70 px-3 py-2 text-xs text-muted-foreground"
        >
          <Info className="size-4 shrink-0" aria-hidden />
          <span>Perubahan profil tidak tersedia dalam mode demo</span>
        </div>
      )}

      <div className="mt-4 mb-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
        <FormField id="profil-nama" label="Nama lengkap" galat={galat.nama}>
          <Input
            id="profil-nama"
            value={nilai.nama}
            onChange={ketik("nama")}
            autoComplete="name"
            disabled={bacaSaja}
            readOnly={menyimpan}
            aria-required="true"
            className="h-11 lg:h-9"
            {...atributIsian("profil-nama", galat.nama, false)}
          />
        </FormField>

        <FormField
          id="profil-telepon"
          label="Nomor telepon / WhatsApp"
          bantuan={
            !galat.telepon && (
              <span id={idPetunjuk("profil-telepon")}>
                Opsional. Boleh diawali 08, 62, atau +62.
              </span>
            )
          }
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
            className="h-11 lg:h-9"
            {...atributIsian("profil-telepon", galat.telepon, true)}
          />
        </FormField>

        <FormField
          id="profil-email"
          label="Email"
          className="sm:col-span-2"
          bantuan={
            <span id="profil-email-catatan">
              Hubungi administrator untuk perubahan email instansi
            </span>
          }
        >
          <div className="relative">
            <Input
              id="profil-email"
              type="email"
              value={email}
              readOnly
              autoComplete="email"
              aria-describedby="profil-email-catatan"
              className="h-11 bg-secondary/60 pr-28 text-muted-foreground lg:h-9 dark:bg-white/[0.03]"
            />
            <span
              aria-hidden
              className="pointer-events-none absolute top-1/2 right-2.5 inline-flex -translate-y-1/2 items-center gap-1 rounded-md bg-card px-2 py-0.5 text-[11px] font-medium text-muted-foreground ring-1 ring-border"
            >
              <Lock className="size-3" />
              Hanya-baca
            </span>
          </div>
        </FormField>
      </div>

      <FormAlert className="mb-4">{form.galat?.pesan}</FormAlert>

      <div className="mt-2 flex flex-col gap-3 border-t border-border pt-3.5 sm:flex-row sm:items-center sm:justify-between">
        <p
          aria-live="polite"
          className="flex min-h-5 items-center gap-1.5 text-xs text-muted-foreground"
        >
          {adaPerubahan && valid ? (
            <>
              <span
                aria-hidden
                className="size-1.5 shrink-0 rounded-full bg-waspada"
              />
              <span className="font-medium text-waspada-ink dark:text-amber-300">
                Ada perubahan yang belum disimpan.
              </span>
            </>
          ) : null}
        </p>
        <div className="flex flex-col-reverse gap-2 sm:flex-row">
          <SpringButton
            type="button"
            variant="outline"
            className="h-11 px-4 lg:h-9"
            disabled={!berbedaDariTersimpan || menyimpan || bacaSaja}
            onClick={batalkan}
          >
            <RotateCcw aria-hidden />
            Batalkan perubahan
          </SpringButton>
          <SpringButton
            type="submit"
            variant="primary"
            className="h-11 px-4 lg:h-9"
            disabled={!bisaSimpan}
          >
            {menyimpan ? (
              <Loader2 className="animate-spin" aria-hidden />
            ) : (
              <Save aria-hidden />
            )}
            Simpan perubahan
          </SpringButton>
        </div>
      </div>
    </FormBff>
  );
}
