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
import {
  validasiBuat,
  type BuatIntervensiInput,
  type GalatBuat,
} from "@/lib/api/intervensi";
import { useSession } from "@/lib/auth/session-context";
import { KABUPATEN, KOMODITAS } from "@/lib/data/reference";
import { SIKLUS_URUTAN } from "@/lib/data/siklus";
import type { Prioritas } from "@/lib/data/types";
import { siklusMeta, type Siklus } from "@/lib/design/tokens";
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
  const [form, setForm] = useState({
    judul: "",
    deskripsi: "",
    siklus: siklusAwal,
    prioritas: "SEDANG" as Prioritas,
    komoditasKode: KOMODITAS[0].kode,
    kabupatenKode: "1171",
    tenggat: tanggalDepan(7),
    tugaskanSaya: true,
  });
  const [galat, setGalat] = useState<GalatBuat>({});
  const set = <K extends keyof typeof form>(k: K, v: (typeof form)[K]) =>
    setForm((f) => ({ ...f, [k]: v }));

  function kirim(e: React.FormEvent) {
    e.preventDefault();
    const input: BuatIntervensiInput = {
      judul: form.judul,
      deskripsi: form.deskripsi,
      siklus: form.siklus,
      prioritas: form.prioritas,
      komoditasKode: form.komoditasKode,
      kabupatenKode: form.kabupatenKode,
      tenggat: form.tenggat,
      penanggungJawabIds: form.tugaskanSaya && userId ? [userId] : [],
    };
    const g = validasiBuat(input);
    setGalat(g);
    if (Object.keys(g).length > 0) return;
    buat.mutate(input, {
      onSuccess: () => {
        onDibuat(input.siklus);
        onOpenChange(false);
      },
    });
  }

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
        <form onSubmit={kirim} noValidate className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="int-judul">Judul</Label>
            <Input
              id="int-judul"
              value={form.judul}
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
              value={form.deskripsi}
              onChange={(e) => set("deskripsi", e.target.value)}
              rows={3}
            />
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label htmlFor="int-siklus">Siklus</Label>
              <Select
                value={form.siklus}
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
                value={form.prioritas}
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
                value={form.komoditasKode}
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
                value={form.kabupatenKode}
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
              value={form.tenggat}
              onChange={(e) => set("tenggat", e.target.value)}
              aria-invalid={!!galat.tenggat}
              aria-describedby={galat.tenggat ? "int-tenggat-galat" : undefined}
            />
            <Galat id="int-tenggat-galat" pesan={galat.tenggat} />
          </div>
          <div className="flex items-center gap-2">
            <Checkbox
              id="int-saya"
              checked={form.tugaskanSaya}
              onCheckedChange={(v) => set("tugaskanSaya", v === true)}
            />
            <Label htmlFor="int-saya" className="font-normal">
              Jadikan saya penanggung jawab
            </Label>
          </div>
          <DialogFooter>
            <SpringButton variant="soft" onClick={() => onOpenChange(false)}>
              Batal
            </SpringButton>
            <SpringButton
              type="submit"
              variant="primary"
              disabled={buat.isPending}
            >
              {buat.isPending ? "Menyimpan…" : "Simpan intervensi"}
            </SpringButton>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
