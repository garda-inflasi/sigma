"use client";

import { useMemo, useState, type ReactNode } from "react";
import { FormBff, PesanField } from "@/components/form/form-bff";
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
  BATAS,
  bodyUsulan,
  isianKomoditasDari,
  isianKomoditasKosong,
  isianPasarDari,
  isianPasarKosong,
  pesanGalatMaster,
  perubahanKomoditas,
  perubahanPasar,
  validasiKomoditas,
  validasiPasar,
  type IsianKomoditas,
  type IsianPasar,
} from "@/lib/api/master-perubahan";
import type { MasterKomoditasRow, MasterPasarRow } from "@/lib/data/types";
import { skemaDariValidasi } from "@/lib/form/skema";
import { useFormBff } from "@/lib/form/use-form-bff";
import {
  useAjukanPerubahanMaster,
  useKecamatanKabupaten,
  useMasterWilayah,
  useSatuan,
} from "@/lib/query/hooks";

/**
 * Sasaran dialog usulan. Dengan `awal` = ubah data yang ada (hanya bidang yang berbeda dikirim); tanpa `awal`
 * = tambah data baru (semua bidang wajib dikirim).
 */
export type TargetUsulan =
  | { entitas: "komoditas"; awal?: MasterKomoditasRow }
  | { entitas: "pasar"; awal?: MasterPasarRow };

const KOSONG = "__kosong";

function Field({
  id,
  label,
  galat,
  bantuan,
  children,
}: {
  id: string;
  label: string;
  galat?: string;
  bantuan?: string;
  children: ReactNode;
}) {
  return (
    <div className="space-y-1.5">
      <Label htmlFor={id}>{label}</Label>
      {children}
      {bantuan && !galat && (
        <p className="text-xs text-muted-foreground">{bantuan}</p>
      )}
      <PesanField field={id} pesan={galat} />
    </div>
  );
}

/** Dialog "Ajukan perubahan" / "Tambah" komoditas dan pasar (POST /v1/master/perubahan, izin master.ajukan). */
export function UsulanDialog({
  target,
  open,
  onOpenChange,
}: {
  target: TargetUsulan | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  return (
    <Dialog open={open && !!target} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-lg">
        {target?.entitas === "komoditas" && (
          <FormKomoditas awal={target.awal} onOpenChange={onOpenChange} />
        )}
        {target?.entitas === "pasar" && (
          <FormPasar awal={target.awal} onOpenChange={onOpenChange} />
        )}
      </DialogContent>
    </Dialog>
  );
}

function Kepala({ judul, deskripsi }: { judul: string; deskripsi: string }) {
  return (
    <DialogHeader>
      <DialogTitle>{judul}</DialogTitle>
      <DialogDescription>{deskripsi}</DialogDescription>
    </DialogHeader>
  );
}

function Alasan({
  nilai,
  galat,
  onChange,
}: {
  nilai: string;
  galat?: string;
  onChange: (v: string) => void;
}) {
  return (
    <Field
      id="usulan-alasan"
      label="Alasan pengajuan"
      galat={galat}
      bantuan={`${BATAS.alasan.min}–${BATAS.alasan.max} karakter. Dibaca penyetuju dan tercatat di audit trail.`}
    >
      <Textarea
        id="usulan-alasan"
        value={nilai}
        rows={3}
        maxLength={BATAS.alasan.max}
        onChange={(e) => onChange(e.target.value)}
        aria-invalid={!!galat}
      />
    </Field>
  );
}

function Kaki({
  pending,
  tambah,
  galat,
  onBatal,
}: {
  pending: boolean;
  tambah: boolean;
  galat?: string;
  onBatal: () => void;
}) {
  return (
    <>
      {galat && (
        <p role="alert" className="text-sm text-defisit-ink dark:text-rose-300">
          {galat}
        </p>
      )}
      <DialogFooter>
        <SpringButton variant="soft" onClick={onBatal} disabled={pending}>
          Batal
        </SpringButton>
        <SpringButton type="submit" variant="primary" disabled={pending}>
          {pending
            ? "Mengajukan…"
            : tambah
              ? "Ajukan data baru"
              : "Ajukan perubahan"}
        </SpringButton>
      </DialogFooter>
    </>
  );
}

function FormKomoditas({
  awal,
  onOpenChange,
}: {
  awal?: MasterKomoditasRow;
  onOpenChange: (v: boolean) => void;
}) {
  const satuan = useSatuan();
  const ajukan = useAjukanPerubahanMaster();
  const [isian, setIsian] = useState<IsianKomoditas>(() =>
    awal ? isianKomoditasDari(awal) : isianKomoditasKosong(),
  );
  const set = <K extends keyof IsianKomoditas>(k: K, v: IsianKomoditas[K]) =>
    setIsian((f) => ({ ...f, [k]: v }));
  const kodeSatuan = useMemo(
    () => satuan.data?.map((s) => s.kode),
    [satuan.data],
  );
  const skema = useMemo(
    () =>
      skemaDariValidasi<IsianKomoditas>((i) =>
        validasiKomoditas(i, { awal, satuan: kodeSatuan }),
      ),
    [awal, kodeSatuan],
  );

  const form = useFormBff<IsianKomoditas>({
    skema,
    kirim: (n) =>
      ajukan.mutateAsync(
        bodyUsulan({
          entitas: "komoditas",
          jenis: awal ? "UBAH" : "TAMBAH",
          entitasId: awal ? awal.kode : n.kode,
          perubahan: perubahanKomoditas(n, awal),
          alasan: n.alasan,
        }),
      ),
    sukses: () => onOpenChange(false),
    petaGalat: (err) => ({ pesan: pesanGalatMaster(err) }),
  });
  const g = form.galatField;

  return (
    <FormBff
      onKirim={form.kirimDari(() => isian)}
      sibuk={form.pending}
      className="space-y-4"
    >
      <Kepala
        judul={awal ? `Ajukan perubahan ${awal.nama}` : "Tambah komoditas"}
        deskripsi={
          awal
            ? "Hanya bidang yang Anda ubah yang diajukan. Perubahan berlaku sesudah disetujui orang lain."
            : "Komoditas baru aktif sesudah usulan disetujui orang lain."
        }
      />
      {!awal && (
        <Field
          id="usulan-kode"
          label="Kode"
          galat={g.kode}
          bantuan="Huruf kapital, angka, atau tanda hubung (mis. KDL-HJU). Tidak dapat diubah kemudian."
        >
          <Input
            id="usulan-kode"
            value={isian.kode}
            maxLength={BATAS.kodeKomoditas.max}
            onChange={(e) => set("kode", e.target.value.toUpperCase())}
            aria-invalid={!!g.kode}
          />
        </Field>
      )}
      <Field id="usulan-nama" label="Nama" galat={g.nama}>
        <Input
          id="usulan-nama"
          value={isian.nama}
          maxLength={BATAS.nama.max}
          onChange={(e) => set("nama", e.target.value)}
          aria-invalid={!!g.nama}
        />
      </Field>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field id="usulan-kategori" label="Kategori" galat={g.kategori}>
          <Input
            id="usulan-kategori"
            value={isian.kategori}
            maxLength={BATAS.kategori.max}
            onChange={(e) => set("kategori", e.target.value)}
            aria-invalid={!!g.kategori}
          />
        </Field>
        <Field
          id="usulan-satuan"
          label="Satuan"
          galat={
            g.satuan ??
            (satuan.isError ? "Daftar satuan tidak dapat dimuat." : undefined)
          }
        >
          <Select
            value={isian.satuan}
            onValueChange={(v) => set("satuan", v)}
            disabled={satuan.isLoading}
          >
            <SelectTrigger
              id="usulan-satuan"
              className="w-full"
              aria-invalid={!!g.satuan}
            >
              <SelectValue
                placeholder={
                  satuan.isLoading ? "Memuat satuan…" : "Pilih satuan"
                }
              />
            </SelectTrigger>
            <SelectContent>
              {(satuan.data ?? []).map((s) => (
                <SelectItem key={s.kode} value={s.kode}>
                  {s.nama} ({s.kode})
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </Field>
        <Field id="usulan-harga" label="Harga acuan (Rp)" galat={g.harga_acuan}>
          <Input
            id="usulan-harga"
            inputMode="numeric"
            value={isian.harga_acuan}
            onChange={(e) => set("harga_acuan", e.target.value)}
            aria-invalid={!!g.harga_acuan}
          />
        </Field>
        <Field
          id="usulan-umur"
          label="Umur simpan (hari)"
          galat={g.shelf_life_hari}
          bantuan={`${BATAS.shelfLife.min}–${BATAS.shelfLife.max} hari`}
        >
          <Input
            id="usulan-umur"
            inputMode="numeric"
            value={isian.shelf_life_hari}
            onChange={(e) => set("shelf_life_hari", e.target.value)}
            aria-invalid={!!g.shelf_life_hari}
          />
        </Field>
      </div>
      <div className="flex items-center gap-2">
        <Checkbox
          id="usulan-pilot"
          checked={isian.pilot}
          onCheckedChange={(v) => set("pilot", v === true)}
        />
        <Label htmlFor="usulan-pilot" className="font-normal">
          Komoditas pilot
        </Label>
      </div>
      <Alasan
        nilai={isian.alasan}
        galat={g.alasan}
        onChange={(v) => set("alasan", v)}
      />
      <Kaki
        pending={form.pending}
        tambah={!awal}
        galat={form.galat?.pesan}
        onBatal={() => onOpenChange(false)}
      />
    </FormBff>
  );
}

function FormPasar({
  awal,
  onOpenChange,
}: {
  awal?: MasterPasarRow;
  onOpenChange: (v: boolean) => void;
}) {
  const wilayah = useMasterWilayah(!awal);
  const ajukan = useAjukanPerubahanMaster();
  const [isian, setIsian] = useState<IsianPasar>(() =>
    awal ? isianPasarDari(awal) : isianPasarKosong(),
  );
  const set = <K extends keyof IsianPasar>(k: K, v: IsianPasar[K]) =>
    setIsian((f) => ({ ...f, [k]: v }));
  const kecamatan = useKecamatanKabupaten(isian.kabupaten_kode || null);
  const skema = useMemo(
    () => skemaDariValidasi<IsianPasar>((i) => validasiPasar(i, { awal })),
    [awal],
  );

  const form = useFormBff<IsianPasar>({
    skema,
    kirim: (n) =>
      ajukan.mutateAsync(
        bodyUsulan({
          entitas: "pasar",
          jenis: awal ? "UBAH" : "TAMBAH",
          entitasId: awal ? awal.id : n.id,
          perubahan: perubahanPasar(n, awal),
          alasan: n.alasan,
        }),
      ),
    sukses: () => onOpenChange(false),
    petaGalat: (err) => ({ pesan: pesanGalatMaster(err) }),
  });
  const g = form.galatField;

  /** Memilih kabupaten mengosongkan kecamatan (milik kabupaten lain) dan mengisi awalan ID pasar bila masih kosong. */
  function pilihKabupaten(kode: string) {
    setIsian((f) => ({
      ...f,
      kabupaten_kode: kode,
      kecamatan_kode: "",
      id: f.id === "" || /^PSR-\d{4}-$/.test(f.id) ? `PSR-${kode}-` : f.id,
    }));
  }

  return (
    <FormBff
      onKirim={form.kirimDari(() => isian)}
      sibuk={form.pending}
      className="space-y-4"
    >
      <Kepala
        judul={awal ? `Ajukan perubahan ${awal.nama}` : "Tambah pasar"}
        deskripsi={
          awal
            ? "Hanya bidang yang Anda ubah yang diajukan. Perubahan berlaku sesudah disetujui orang lain."
            : "Pasar baru aktif sesudah usulan disetujui orang lain."
        }
      />
      {!awal && (
        <>
          <Field
            id="usulan-kabupaten"
            label="Kabupaten/kota"
            galat={g.kabupaten_kode}
          >
            <Select
              value={isian.kabupaten_kode}
              onValueChange={pilihKabupaten}
              disabled={wilayah.isLoading}
            >
              <SelectTrigger
                id="usulan-kabupaten"
                className="w-full"
                aria-invalid={!!g.kabupaten_kode}
              >
                <SelectValue
                  placeholder={
                    wilayah.isLoading
                      ? "Memuat wilayah…"
                      : "Pilih kabupaten/kota"
                  }
                />
              </SelectTrigger>
              <SelectContent>
                {(wilayah.data ?? []).map((w) => (
                  <SelectItem key={w.kode} value={w.kode}>
                    {w.nama}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
          <Field
            id="usulan-id"
            label="ID pasar"
            galat={g.id}
            bantuan="Mis. PSR-1171-05. Tidak dapat diubah kemudian."
          >
            <Input
              id="usulan-id"
              value={isian.id}
              maxLength={BATAS.kodeKomoditas.max}
              onChange={(e) => set("id", e.target.value.toUpperCase())}
              aria-invalid={!!g.id}
            />
          </Field>
        </>
      )}
      <Field id="usulan-nama" label="Nama" galat={g.nama}>
        <Input
          id="usulan-nama"
          value={isian.nama}
          maxLength={BATAS.nama.max}
          onChange={(e) => set("nama", e.target.value)}
          aria-invalid={!!g.nama}
        />
      </Field>
      {awal && (
        <p className="text-xs text-muted-foreground">
          Kabupaten/kota: {awal.kabupaten}. Tidak dapat diubah lewat usulan.
        </p>
      )}
      <Field
        id="usulan-kecamatan"
        label="Kecamatan"
        galat={
          g.kecamatan_kode ??
          (kecamatan.isError
            ? "Daftar kecamatan tidak dapat dimuat."
            : undefined)
        }
        bantuan={
          isian.kabupaten_kode ? undefined : "Pilih kabupaten/kota lebih dulu."
        }
      >
        <Select
          value={isian.kecamatan_kode || KOSONG}
          onValueChange={(v) => set("kecamatan_kode", v === KOSONG ? "" : v)}
          disabled={!isian.kabupaten_kode || kecamatan.isLoading}
        >
          <SelectTrigger
            id="usulan-kecamatan"
            className="w-full"
            aria-invalid={!!g.kecamatan_kode}
          >
            <SelectValue
              placeholder={
                kecamatan.isLoading ? "Memuat kecamatan…" : "Tanpa kecamatan"
              }
            />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={KOSONG}>Tanpa kecamatan</SelectItem>
            {(kecamatan.data ?? []).map((k) => (
              <SelectItem key={k.kode} value={k.kode}>
                {k.nama} ({k.kode})
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>
      <div className="grid gap-4 sm:grid-cols-3">
        <Field id="usulan-lat" label="Lintang" galat={g.lat}>
          <Input
            id="usulan-lat"
            inputMode="decimal"
            value={isian.lat}
            onChange={(e) => set("lat", e.target.value)}
            aria-invalid={!!g.lat}
          />
        </Field>
        <Field id="usulan-lng" label="Bujur" galat={g.lng}>
          <Input
            id="usulan-lng"
            inputMode="decimal"
            value={isian.lng}
            onChange={(e) => set("lng", e.target.value)}
            aria-invalid={!!g.lng}
          />
        </Field>
        <Field
          id="usulan-radius"
          label="Radius geofence (m)"
          galat={g.radius_meter}
        >
          <Input
            id="usulan-radius"
            inputMode="numeric"
            value={isian.radius_meter}
            onChange={(e) => set("radius_meter", e.target.value)}
            aria-invalid={!!g.radius_meter}
          />
        </Field>
      </div>
      {awal && (
        <div className="flex items-center gap-2">
          <Checkbox
            id="usulan-aktif"
            checked={isian.aktif}
            onCheckedChange={(v) => set("aktif", v === true)}
          />
          <Label htmlFor="usulan-aktif" className="font-normal">
            Pasar aktif (muncul di tugas lapangan)
          </Label>
        </div>
      )}
      <Alasan
        nilai={isian.alasan}
        galat={g.alasan}
        onChange={(v) => set("alasan", v)}
      />
      <Kaki
        pending={form.pending}
        tambah={!awal}
        galat={form.galat?.pesan}
        onBatal={() => onOpenChange(false)}
      />
    </FormBff>
  );
}
