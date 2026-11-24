"use client";

import { useState } from "react";
import { SpringButton } from "@/components/garda/spring-button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { DateField } from "@/components/ui/date-field";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { FormDialog } from "@/components/form/form-bff";
import { FormAlert, FormField } from "@/components/form/form-field";
import { validasiBuat, type BuatIntervensiInput } from "@/lib/api/intervensi";
import { useSession } from "@/lib/auth/session-context";
import { KABUPATEN, KOMODITAS } from "@/lib/data/reference";
import { OptionCards } from "@/components/garda/option-cards";
import { SIKLUS_URUTAN } from "@/lib/data/siklus";
import type { Prioritas } from "@/lib/data/types";
import { siklusMeta, type Siklus } from "@/lib/design/tokens";
import { Flame, Sprout, Waves } from "lucide-react";

import { skemaDariValidasi } from "@/lib/form/skema";
import { useFormBff } from "@/lib/form/use-form-bff";
import { useBuatIntervensi } from "@/lib/query/hooks";
const IKON_SIKLUS = { normal: Waves, tekanan: Flame, surplus: Sprout } as const;
const AKSEN_SIKLUS: Record<Siklus, string> = { normal: "text-normal-ink", tekanan: "text-defisit-ink", surplus: "text-surplus-ink" };

const PRIORITAS: { value: Prioritas; label: string }[] = [
  { value: "TINGGI", label: "Tinggi" },
  { value: "SEDANG", label: "Sedang" },
  { value: "RENDAH", label: "Rendah" },
];

/** Tanggal yyyy-mm-dd `hari` hari dari sekarang (zona waktu perangkat). */
function tanggalDepan(hari: number) {
  const d = new Date(Date.now() + hari * 86_400_000);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

/** Validasi klien yang mencerminkan permintaan API (lib/api/intervensi), dipakai sebagai skema form. */
const skemaBuat = skemaDariValidasi<BuatIntervensiInput>(validasiBuat);

function Galat({ id, pesan }: { id: string; pesan?: string }) {
  return pesan ? (
    <p
      id={id}
      role="alert"
      className="text-xs text-defisit-ink dark:text-rose-300"
    >
      {pesan}
    </p>
  ) : null;
}

/**
 * Dialog “Tambah intervensi” (POST /v1/intervensi, izin intervensi.kelola). Kartu baru masuk ke tahap
 * pertama siklus yang dipilih; validasi sisi klien mencerminkan permintaan API, server tetap berwenang.
 */
export function TambahIntervensiDialog({
  open,
  onOpenChange,
  siklusAwal,
  onDibuat,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  siklusAwal: Siklus;
  /** Dipanggil setelah kartu tersimpan, mis. untuk menampilkan siklus kartu itu di papan. */
  onDibuat: (siklus: Siklus) => void;
}) {
  const { id: userId } = useSession();
  const buat = useBuatIntervensi();
  const [isian, setIsian] = useState({
    judul: "",
    deskripsi: "",
    siklus: siklusAwal,
    prioritas: "SEDANG" as Prioritas,
    komoditasKode: KOMODITAS[0].kode,
    kabupatenKode: "1171",
    tenggat: tanggalDepan(7),
    tugaskanSaya: true,
  });
  const set = <K extends keyof typeof isian>(k: K, v: (typeof isian)[K]) =>
    setIsian((f) => ({ ...f, [k]: v }));

  const form = useFormBff<BuatIntervensiInput>({
    skema: skemaBuat,
    // Toast sukses/gagal dari hook mutasi; galat 400/422 juga tampil di bawah form.
    kirim: (input) => buat.mutateAsync(input),
    sukses: (_, input) => {
      onDibuat(input.siklus);
      onOpenChange(false);
    },
  });
  const galat = form.galatField;
  const kirim = form.kirimDari(() => ({
    judul: isian.judul,
    deskripsi: isian.deskripsi,
    siklus: isian.siklus,
    prioritas: isian.prioritas,
    komoditasKode: isian.komoditasKode,
    kabupatenKode: isian.kabupatenKode,
    tenggat: isian.tenggat,
    penanggungJawabIds: isian.tugaskanSaya && userId ? [userId] : [],
  }));

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Tambah intervensi</DialogTitle>
          <DialogDescription>
            Kartu baru masuk ke tahap pertama siklus yang dipilih dan tercatat
            di riwayat serta audit trail.
          </DialogDescription>
        </DialogHeader>
        <FormDialog onKirim={kirim} sibuk={form.pending}>
          <DialogBody>
            <FormField id="int-judul" label="Judul">
              <Input
                id="int-judul"
                value={isian.judul}
                maxLength={255}
                onChange={(e) => set("judul", e.target.value)}
                aria-invalid={!!galat.judul}
                aria-describedby={galat.judul ? "int-judul-galat" : undefined}
                placeholder="Mis. Operasi pasar beras Banda Aceh"
              />
              <Galat id="int-judul-galat" pesan={galat.judul} />
            </FormField>
            <FormField id="int-deskripsi" label="Deskripsi" opsional>
              <Textarea
                id="int-deskripsi"
                value={isian.deskripsi}
                onChange={(e) => set("deskripsi", e.target.value)}
                rows={3}
              />
            </FormField>
            <div className="flex min-w-0 flex-col gap-1.5">
              <Label id="int-siklus-label">Siklus</Label>
              <OptionCards
                name="int-siklus"
                label="Siklus"
                value={isian.siklus}
                onChange={(v) => set("siklus", v)}
                options={SIKLUS_URUTAN.map((s) => ({
                  value: s,
                  title: siklusMeta[s].label,
                  description: siklusMeta[s].deskripsi,
                  icon: IKON_SIKLUS[s],
                  accent: AKSEN_SIKLUS[s],
                }))}
              />
            </div>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <FormField id="int-prioritas" label="Prioritas">
                <Select
                  value={isian.prioritas}
                  onValueChange={(v) => set("prioritas", v as Prioritas)}
                >
                  <SelectTrigger id="int-prioritas">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {PRIORITAS.map((p) => (
                      <SelectItem key={p.value} value={p.value}>
                        {p.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </FormField>
              <FormField id="int-komoditas" label="Komoditas">
                <Select
                  value={isian.komoditasKode}
                  onValueChange={(v) => set("komoditasKode", v)}
                >
                  <SelectTrigger
                    id="int-komoditas"
                    aria-invalid={!!galat.komoditasKode}
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {KOMODITAS.map((k) => (
                      <SelectItem key={k.kode} value={k.kode}>
                        {k.nama}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Galat id="int-komoditas-galat" pesan={galat.komoditasKode} />
              </FormField>
              <FormField id="int-wilayah" label="Wilayah">
                <Select
                  value={isian.kabupatenKode}
                  onValueChange={(v) => set("kabupatenKode", v)}
                >
                  <SelectTrigger
                    id="int-wilayah"
                    aria-invalid={!!galat.kabupatenKode}
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {KABUPATEN.map((k) => (
                      <SelectItem key={k.kode} value={k.kode}>
                        {k.jenis === "Kota" ? `Kota ${k.nama}` : k.nama}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Galat id="int-wilayah-galat" pesan={galat.kabupatenKode} />
              </FormField>
              <FormField id="int-tenggat" label="Tenggat">
                <DateField
                  id="int-tenggat"
                  value={isian.tenggat}
                  onValueChange={(v) => set("tenggat", v)}
                  aria-invalid={!!galat.tenggat}
                  aria-describedby={
                    galat.tenggat ? "int-tenggat-galat" : undefined
                  }
                />
                <Galat id="int-tenggat-galat" pesan={galat.tenggat} />
              </FormField>
            </div>
            <div className="flex items-center gap-2">
              <Checkbox
                id="int-saya"
                checked={isian.tugaskanSaya}
                onCheckedChange={(v) => set("tugaskanSaya", v === true)}
              />
              <Label htmlFor="int-saya" className="font-normal">
                Jadikan saya penanggung jawab
              </Label>
            </div>
            <FormAlert>{form.galat?.pesan}</FormAlert>
          </DialogBody>
          <DialogFooter>
            <SpringButton variant="soft" onClick={() => onOpenChange(false)}>
              Batal
            </SpringButton>
            <SpringButton
              type="submit"
              variant="primary"
              disabled={form.pending}
            >
              {form.pending ? "Menyimpan…" : "Simpan intervensi"}
            </SpringButton>
          </DialogFooter>
        </FormDialog>
      </DialogContent>
    </Dialog>
  );
}
