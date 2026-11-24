"use client";

import { Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import { z } from "zod";
import { FormBff, idGalatField } from "@/components/form/form-bff";
import { FormAlert, FormField } from "@/components/form/form-field";
import { SpringButton } from "@/components/garda/spring-button";
import { StatusBadge } from "@/components/garda/status-badge";
import { Button } from "@/components/ui/button";
import {
  DialogBody,
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
import { Textarea } from "@/components/ui/textarea";
import { MODE_MOCK } from "@/lib/api/client";
import {
  akhirHari,
  galatTanggalBerlaku,
  hariIni,
  izinTersedia,
  type IzinLangsung,
} from "@/lib/api/rbac";
import type { PenggunaRow } from "@/lib/data/types";
import { useCan } from "@/lib/auth/session-context";
import { formatRelatif, formatTanggal } from "@/lib/format";
import { useFormBff } from "@/lib/form/use-form-bff";
import {
  useBeriIzin,
  useCabutIzin,
  useIzinPengguna,
  useMatriksAkses,
} from "@/lib/query/rbac-hooks";
import { Segmented } from "@/components/garda/segmented";

export type BagianDialogPengguna = "akun" | "izin";

/**
 * Pemilih bagian dialog ubah pengguna: "Data akun" atau "Izin langsung". Tidak dirender tanpa izin pengguna.kelola
 * atau di mode mock (API izin langsung tidak ada). Bagian "Izin langsung" punya formulirnya sendiri, jadi ia
 * berdiri di samping formulir akun (bukan di dalamnya: `<form>` tidak boleh bersarang).
 */
export function PilihBagianPengguna({
  nilai,
  onUbah,
}: {
  nilai: BagianDialogPengguna;
  onUbah: (v: BagianDialogPengguna) => void;
}) {
  const boleh = useCan("pengguna.kelola");
  if (!boleh || MODE_MOCK) return null;
  return (
    <Segmented
      layoutId="pengguna-dialog-bagian"
      size="sm"
      className="shrink-0 self-start"
      value={nilai}
      onChange={onUbah}
      options={[
        { value: "akun", label: "Data akun" },
        { value: "izin", label: "Izin langsung" },
      ]}
    />
  );
}

const skemaBeriIzin = z
  .object({
    izin: z.string().min(1, "Pilih izin yang akan diberikan."),
    berlaku: z.string(),
    alasan: z
      .string()
      .trim()
      .min(1, "Alasan wajib diisi.")
      .max(500, "Alasan maksimal 500 karakter."),
  })
  .superRefine((n, ctx) => {
    const galat = galatTanggalBerlaku(n.berlaku);
    if (galat)
      ctx.addIssue({ code: "custom", path: ["berlaku"], message: galat });
  });
type IsianBeriIzin = z.input<typeof skemaBeriIzin>;

const KOSONG: IsianBeriIzin = { izin: "", berlaku: "", alasan: "" };

function Berlaku({ g }: { g: IzinLangsung }) {
  if (!g.berlakuSampai) return <>Tanpa batas waktu</>;
  return (
    <>
      {g.aktif ? "Berlaku sampai" : "Berakhir"}{" "}
      {formatTanggal(g.berlakuSampai, "d MMM yyyy HH:mm")}
    </>
  );
}

function DaftarIzinLangsung({
  userId,
  langsung,
}: {
  userId: string;
  langsung: IzinLangsung[];
}) {
  const cabut = useCabutIzin(userId);
  if (langsung.length === 0) {
    return (
      <p className="rounded-lg bg-secondary p-3 text-sm text-muted-foreground">
        Belum ada izin langsung untuk pengguna ini.
      </p>
    );
  }
  return (
    <ul aria-label="Izin langsung" className="grid grid-cols-1 gap-2">
      {langsung.map((g) => (
        <li
          key={g.izin}
          data-izin={g.izin}
          className="grid min-w-0 gap-1.5 rounded-lg border border-border p-3 text-sm"
        >
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="min-w-0">
              <p className="num font-medium break-all">{g.izin}</p>
              <p className="text-xs break-words text-muted-foreground">{g.deskripsi}</p>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <StatusBadge tone="data" dot={false}>
                Langsung
              </StatusBadge>
              <StatusBadge tone={g.aktif ? "normal" : "netral"}>
                {g.aktif ? "Aktif" : "Kedaluwarsa"}
              </StatusBadge>
              <Button
                variant="outline"
                size="sm"
                aria-label={`Cabut izin ${g.izin}`}
                disabled={cabut.isPending}
                onClick={() => cabut.mutate(g.izin)}
              >
                <Trash2 /> Cabut
              </Button>
            </div>
          </div>
          <dl className="grid gap-0.5 text-xs">
            <div className="flex gap-2">
              <dt className="shrink-0 text-muted-foreground">Masa berlaku</dt>
              <dd className="min-w-0">
                <Berlaku g={g} />
              </dd>
            </div>
            <div className="flex gap-2">
              <dt className="shrink-0 text-muted-foreground">Alasan</dt>
              <dd className="min-w-0 break-words">{g.alasan}</dd>
            </div>
            <div className="flex gap-2">
              <dt className="shrink-0 text-muted-foreground">Diberikan</dt>
              <dd className="min-w-0 break-words">
                {g.diberikanOleh ? `oleh ${g.diberikanOleh}, ` : ""}
                {formatRelatif(g.diberikanPada)} (
                {formatTanggal(g.diberikanPada, "d MMM yyyy HH:mm")})
              </dd>
            </div>
          </dl>
        </li>
      ))}
    </ul>
  );
}

function FormBeriIzin({
  userId,
  tersedia,
}: {
  userId: string;
  tersedia: { id: string; deskripsi: string }[];
}) {
  const beri = useBeriIzin(userId);
  const [isian, setIsian] = useState<IsianBeriIzin>(KOSONG);
  // Toast galat datang dari hook mutasi; galat juga tampil di form (field bila server merincinya).
  const kiriman = useFormBff<
    z.output<typeof skemaBeriIzin>,
    unknown,
    { pesan: string },
    IsianBeriIzin
  >({
    skema: skemaBeriIzin,
    kirim: (n) =>
      beri.mutateAsync({
        izin: n.izin,
        alasan: n.alasan,
        berlakuSampai: n.berlaku
          ? (akhirHari(n.berlaku) ?? undefined)
          : undefined,
      }),
    sukses: () => setIsian(KOSONG),
  });
  const g = kiriman.galatField;
  const atribut = (field: keyof IsianBeriIzin) =>
    g[field]
      ? {
          "aria-invalid": true,
          "aria-describedby": idGalatField(`il-${field}`),
        }
      : {};

  return (
    <FormBff
      onKirim={kiriman.kirimDari(() => isian)}
      sibuk={kiriman.pending}
      className="flex min-w-0 flex-col gap-3 rounded-lg bg-secondary p-3"
      aria-label="Beri izin langsung"
    >
      <p className="text-sm font-medium">Beri izin langsung</p>
      <FormField id="il-izin" label="Izin" galat={g.izin}>
        <Select
          value={isian.izin}
          onValueChange={(v) => setIsian({ ...isian, izin: v })}
          disabled={tersedia.length === 0}
        >
          <SelectTrigger id="il-izin" {...atribut("izin")}>
            <SelectValue
              placeholder={
                tersedia.length === 0
                  ? "Semua izin sudah dimiliki"
                  : "Pilih izin"
              }
            />
          </SelectTrigger>
          <SelectContent>
            {tersedia.map((i) => (
              <SelectItem key={i.id} value={i.id}>
                {i.id} — {i.deskripsi}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </FormField>
      <FormField
        id="il-berlaku"
        label="Berlaku sampai"
        opsional
        bantuan="Kosongkan untuk izin tanpa batas waktu. Izin berhenti berlaku di akhir hari yang dipilih."
        galat={g.berlaku}
      >
        <Input
          id="il-berlaku"
          type="date"
          min={hariIni()}
          value={isian.berlaku}
          onChange={(e) => setIsian({ ...isian, berlaku: e.target.value })}
          {...atribut("berlaku")}
        />
      </FormField>
      <FormField id="il-alasan" label="Alasan" galat={g.alasan}>
        <Textarea
          id="il-alasan"
          rows={2}
          maxLength={500}
          value={isian.alasan}
          onChange={(e) => setIsian({ ...isian, alasan: e.target.value })}
          placeholder="Mengapa pengguna ini perlu izin tambahan?"
          {...atribut("alasan")}
        />
      </FormField>
      <FormAlert>{kiriman.galat?.pesan}</FormAlert>
      <div className="flex justify-end">
        <SpringButton
          type="submit"
          variant="primary"
          size="md"
          disabled={kiriman.pending || tersedia.length === 0}
        >
          {kiriman.pending ? "Memberi izin…" : "Berikan izin"}
        </SpringButton>
      </div>
    </FormBff>
  );
}

/** Bagian "Izin langsung" pada dialog ubah pengguna: daftar (sumber, masa berlaku, alasan, pemberi), pemberian, pencabutan. */
export function IzinLangsungPanel({
  user,
  onTutup,
}: {
  user: PenggunaRow;
  onTutup: () => void;
}) {
  const izin = useIzinPengguna(user.id);
  const matriks = useMatriksAkses();
  const tersedia = useMemo(
    () =>
      izin.data && matriks.data
        ? izinTersedia(matriks.data.izin, izin.data)
        : [],
    [izin.data, matriks.data],
  );

  return (
    <>
      <DialogHeader>
        <DialogTitle>Izin langsung — {user.nama}</DialogTitle>
        <DialogDescription>
          Izin tambahan di luar peran, dengan alasan dan masa berlaku. Berlaku
          pada request berikutnya dan tercatat di audit trail.
        </DialogDescription>
      </DialogHeader>

      <DialogBody>
        {izin.isError ? (
          <FormAlert>
            Izin pengguna gagal dimuat. Tutup lalu buka lagi dialog ini.
          </FormAlert>
        ) : !izin.data || !matriks.data ? (
          <div
            className="h-48 shrink-0 animate-pulse rounded-lg bg-secondary"
            aria-busy="true"
          />
        ) : (
          <>
            <DaftarIzinLangsung
              userId={user.id}
              langsung={izin.data.langsung}
            />
            <details className="min-w-0 rounded-lg border border-border p-3 text-sm">
              <summary className="cursor-pointer font-medium">
                Izin dari peran ({izin.data.izinPeran.length})
              </summary>
              <ul
                aria-label="Izin dari peran"
                className="mt-2 flex flex-wrap gap-1.5"
              >
                {izin.data.izinPeran.map((p) => (
                  <li key={p} className="max-w-full">
                    <span className="num inline-flex min-h-6 max-w-full items-center rounded-md bg-secondary px-2 text-xs break-all">
                      {p}
                    </span>
                  </li>
                ))}
              </ul>
              <p className="mt-2 text-xs text-muted-foreground">
                Sumber: peran pengguna. Ubah lewat tab "Matriks akses".
              </p>
            </details>
            <FormBeriIzin userId={user.id} tersedia={tersedia} />
          </>
        )}
      </DialogBody>

      <DialogFooter>
        <Button variant="outline" onClick={onTutup}>
          Tutup
        </Button>
      </DialogFooter>
    </>
  );
}
