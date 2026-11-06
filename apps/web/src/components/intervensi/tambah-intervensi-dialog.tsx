"use client";

import { useState } from "react";
import { SpringButton } from "@/components/garda/spring-button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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
import { FormBff } from "@/components/form/form-bff";
import { validasiBuat, type BuatIntervensiInput } from "@/lib/api/intervensi";
import { useSession } from "@/lib/auth/session-context";
import { KABUPATEN, KOMODITAS } from "@/lib/data/reference";
import { SIKLUS_URUTAN } from "@/lib/data/siklus";
import type { Prioritas } from "@/lib/data/types";
import { siklusMeta, type Siklus } from "@/lib/design/tokens";
import { skemaDariValidasi } from "@/lib/form/skema";
import { useFormBff } from "@/lib/form/use-form-bff";
import { useBuatIntervensi } from "@/lib/query/hooks";

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
        <FormBff onKirim={kirim} sibuk={form.pending} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="int-judul">Judul</Label>
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
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="int-deskripsi">Deskripsi (opsional)</Label>
            <Textarea
              id="int-deskripsi"
              value={isian.deskripsi}
              onChange={(e) => set("deskripsi", e.target.value)}
              rows={3}
            />
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label htmlFor="int-siklus">Siklus</Label>
              <Select
                value={isian.siklus}
                onValueChange={(v) => set("siklus", v as Siklus)}
              >
                <SelectTrigger id="int-siklus" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {SIKLUS_URUTAN.map((s) => (
                    <SelectItem key={s} value={s}>
                      {siklusMeta[s].label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="int-prioritas">Prioritas</Label>
              <Select
                value={isian.prioritas}
                onValueChange={(v) => set("prioritas", v as Prioritas)}
              >
                <SelectTrigger id="int-prioritas" className="w-full">
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
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="int-komoditas">Komoditas</Label>
              <Select
                value={isian.komoditasKode}
                onValueChange={(v) => set("komoditasKode", v)}
              >
                <SelectTrigger
                  id="int-komoditas"
                  className="w-full"
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
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="int-wilayah">Wilayah</Label>
              <Select
                value={isian.kabupatenKode}
                onValueChange={(v) => set("kabupatenKode", v)}
              >
                <SelectTrigger
                  id="int-wilayah"
                  className="w-full"
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
            </div>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="int-tenggat">Tenggat</Label>
            <Input
              id="int-tenggat"
              type="date"
              value={isian.tenggat}
              onChange={(e) => set("tenggat", e.target.value)}
              aria-invalid={!!galat.tenggat}
              aria-describedby={galat.tenggat ? "int-tenggat-galat" : undefined}
            />
            <Galat id="int-tenggat-galat" pesan={galat.tenggat} />
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
          {form.galat && (
            <p role="alert" className="text-sm text-defisit-ink dark:text-rose-300">
              {form.galat.pesan}
            </p>
          )}
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
        </FormBff>
      </DialogContent>
    </Dialog>
  );
}
