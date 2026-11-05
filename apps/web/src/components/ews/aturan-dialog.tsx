"use client";

import { useState } from "react";
import { SpringButton } from "@/components/garda/spring-button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api/client";
import {
  KANAL_EWS,
  satuanAmbang,
  validasiAmbang,
  type EwsAturan,
} from "@/lib/api/ews";
import { useUbahAturanEws } from "@/lib/query/hooks";
import { JENIS_EWS } from "./ews-meta";

const KANAL_LABEL: Record<string, string> = {
  DASHBOARD: "Dashboard",
  PUSH: "Push",
  EMAIL: "Email",
  WHATSAPP: "WhatsApp",
};

/** Formulir ambang satu aturan. Dipasang ulang (key = id + versi) tiap aturan atau versinya berganti. */
function FormAturan({
  aturan,
  onDone,
}: {
  aturan: EwsAturan;
  onDone: () => void;
}) {
  const simpan = useUbahAturanEws();
  const [form, setForm] = useState({
    waspada: String(aturan.waspada),
    siaga: String(aturan.siaga),
    kritis: String(aturan.kritis),
    jendelaHari: String(aturan.jendelaHari),
    cooldownJam: String(aturan.cooldownJam),
    kanal: aturan.kanal,
    aktif: aturan.aktif,
    keterangan: aturan.keterangan,
  });
  const [galat, setGalat] = useState<string | null>(null);
  const satuan = satuanAmbang(aturan.jenis);

  function kirim(e: React.FormEvent) {
    e.preventDefault();
    const input = {
      waspada: Number(form.waspada),
      siaga: Number(form.siaga),
      kritis: Number(form.kritis),
      jendelaHari: Number(form.jendelaHari),
      cooldownJam: Number(form.cooldownJam),
      kanal: form.kanal,
      aktif: form.aktif,
      keterangan: form.keterangan,
      versi: aturan.versi,
    };
    const salah =
      validasiAmbang(input) ??
      (!Number.isInteger(input.jendelaHari) ||
      input.jendelaHari < 1 ||
      input.jendelaHari > 90
        ? "Jendela harus bilangan bulat 1–90 hari."
        : null) ??
      (!Number.isInteger(input.cooldownJam) ||
      input.cooldownJam < 0 ||
      input.cooldownJam > 720
        ? "Peredaman harus bilangan bulat 0–720 jam."
        : null) ??
      (input.kanal.length === 0 ? "Pilih sedikitnya satu kanal." : null);
    setGalat(salah);
    if (salah) return;
    simpan.mutate(
      { id: aturan.id, input },
      {
        onSuccess: onDone,
        onError: (err) =>
          setGalat(
            err instanceof ApiError && err.status === 409
              ? "Aturan ini baru diubah pengguna lain. Data sudah dimuat ulang; tinjau nilainya lalu simpan lagi."
              : err instanceof Error
                ? err.message
                : "Ambang gagal disimpan.",
          ),
      },
    );
  }

  const ambang = [
    ["waspada", "Waspada"],
    ["siaga", "Siaga"],
    ["kritis", "Kritis"],
  ] as const;
  return (
    <form onSubmit={kirim} className="space-y-4">
      <div className="grid grid-cols-3 gap-3">
        {ambang.map(([kunci, label]) => (
          <div key={kunci} className="space-y-1.5">
            <Label htmlFor={`ambang-${kunci}`}>
              {label} ({satuan})
            </Label>
            <Input
              id={`ambang-${kunci}`}
              type="number"
              inputMode="decimal"
              step="any"
              min="0"
              value={form[kunci]}
              onChange={(e) => setForm({ ...form, [kunci]: e.target.value })}
            />
          </div>
        ))}
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label htmlFor="ambang-jendela">Jendela (hari)</Label>
          <Input
            id="ambang-jendela"
            type="number"
            min="1"
            max="90"
            value={form.jendelaHari}
            onChange={(e) => setForm({ ...form, jendelaHari: e.target.value })}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="ambang-cooldown">
            Peredaman setelah selesai (jam)
          </Label>
          <Input
            id="ambang-cooldown"
            type="number"
            min="0"
            max="720"
            value={form.cooldownJam}
            onChange={(e) => setForm({ ...form, cooldownJam: e.target.value })}
          />
        </div>
      </div>
      <fieldset className="space-y-2">
        <legend className="text-sm font-medium">Kanal notifikasi</legend>
        <div className="flex flex-wrap gap-4">
          {KANAL_EWS.map((k) => (
            <label key={k} className="flex items-center gap-2 text-sm">
              <Checkbox
                checked={form.kanal.includes(k)}
                onCheckedChange={(on) =>
                  setForm({
                    ...form,
                    kanal: on
                      ? [...form.kanal, k]
                      : form.kanal.filter((x) => x !== k),
                  })
                }
              />
              {KANAL_LABEL[k]}
            </label>
          ))}
        </div>
      </fieldset>
      <label className="flex items-center gap-2 text-sm">
        <Checkbox
          checked={form.aktif}
          onCheckedChange={(on) => setForm({ ...form, aktif: on === true })}
        />
        Aturan aktif (dievaluasi rule engine)
      </label>
      <div className="space-y-1.5">
        <Label htmlFor="ambang-ket">Keterangan kalibrasi</Label>
        <Input
          id="ambang-ket"
          maxLength={500}
          value={form.keterangan}
          onChange={(e) => setForm({ ...form, keterangan: e.target.value })}
        />
      </div>
      {galat && (
        <p role="alert" className="text-sm text-defisit-ink">
          {galat}
        </p>
      )}
      <div className="flex justify-end gap-2">
        <SpringButton variant="soft" onClick={onDone}>
          Batal
        </SpringButton>
        <SpringButton type="submit" disabled={simpan.isPending}>
          {simpan.isPending ? "Menyimpan…" : "Simpan ambang"}
        </SpringButton>
      </div>
    </form>
  );
}

/** Dialog ubah ambang aturan EWS (izin `ews.atur_ambang`). */
export function AturanDialog({
  aturan,
  onClose,
}: {
  aturan: EwsAturan | null;
  onClose: () => void;
}) {
  return (
    <Dialog
      open={aturan !== null}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className="glass-strong max-h-[90dvh] overflow-y-auto rounded-xl border-0 sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Ubah ambang peringatan</DialogTitle>
          <DialogDescription>
            {aturan
              ? `${aturan.komoditasNama ?? "Semua komoditas"} · ${JENIS_EWS[aturan.jenis].label}. `
              : ""}
            Perubahan berlaku pada evaluasi berikutnya dan tercatat di audit
            trail.
          </DialogDescription>
        </DialogHeader>
        {aturan && (
          <FormAturan
            key={`${aturan.id}:${aturan.versi}`}
            aturan={aturan}
            onDone={onClose}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}
