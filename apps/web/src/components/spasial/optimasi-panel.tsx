"use client";

import { ArrowRight, Loader2, Save } from "lucide-react";
import { useState } from "react";
import { PanelTitle } from "@/components/garda/page-header";
import type { MarkerWilayah } from "@/components/map/marker-pin";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { OptimasiParams, RencanaOptimasi } from "@/lib/api/spasial";
import { useCan } from "@/lib/auth/session-context";
import { KOMODITAS } from "@/lib/data/reference";
import { formatAngka, formatRupiah } from "@/lib/format";
import { useOptimasiRute } from "@/lib/query/hooks";

const TANPA = "__tanpa";

/**
 * Optimasi rute distribusi: rencana min-cost dari wilayah surplus ke defisit untuk
 * satu komoditas. Penyesuaian satu wilayah mensimulasikan skenario pasokan (what-if)
 * dan hanya berupa simulasi; rencana tanpa skenario dapat disimpan sebagai usulan rute.
 */
export function OptimasiPanel({ markers }: { markers: MarkerWilayah[] }) {
  const bisaSimpan = useCan("logistik.kelola");
  const optimasi = useOptimasiRute();

  const [komoditas, setKomoditas] = useState("CBM-KRT");
  const [maksJam, setMaksJam] = useState("");
  const [wilayah, setWilayah] = useState(TANPA);
  const [ton, setTon] = useState("");
  const [hasil, setHasil] = useState<RencanaOptimasi>();

  const tonNum = Number(ton.replace(",", "."));
  const skenario =
    wilayah !== TANPA &&
    ton.trim() !== "" &&
    Number.isFinite(tonNum) &&
    tonNum !== 0;
  const maksJamNum = Number(maksJam.replace(",", "."));
  const jamValid =
    maksJam.trim() === "" ||
    (Number.isFinite(maksJamNum) && maksJamNum > 0 && maksJamNum <= 240);

  const params = (simpan: boolean): OptimasiParams => ({
    komoditasKode: komoditas,
    maksJamTempuh: maksJam.trim() ? maksJamNum : undefined,
    penyesuaian: skenario
      ? [{ kabupatenKode: wilayah, ton: tonNum }]
      : undefined,
    simpan,
  });
  const hitung = (simpan: boolean) =>
    optimasi.mutate(params(simpan), { onSuccess: setHasil });

  return (
    <section
      className="glass space-y-4 rounded-xl p-5"
      aria-label="Optimasi rute"
    >
      <PanelTitle
        title="Optimasi rute distribusi"
        description="Rencana biaya terendah dari wilayah surplus ke wilayah defisit menurut neraca terbaru; volume tersalur dimaksimalkan lebih dulu."
      />
      <div className="flex flex-wrap items-end gap-3">
        <Medan label="Komoditas">
          <Select value={komoditas} onValueChange={setKomoditas}>
            <SelectTrigger className="h-10 w-52">
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
        </Medan>
        <Medan label="Batas waktu tempuh (jam)">
          <Input
            inputMode="decimal"
            value={maksJam}
            onChange={(e) => setMaksJam(e.target.value)}
            placeholder="tanpa batas"
            className="h-10 w-40"
            aria-invalid={!jamValid}
          />
        </Medan>
        <Medan label="Skenario: wilayah">
          <Select value={wilayah} onValueChange={setWilayah}>
            <SelectTrigger className="h-10 w-48">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={TANPA}>Tanpa skenario</SelectItem>
              {markers.map((m) => (
                <SelectItem key={m.kode} value={m.kode}>
                  {m.nama}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </Medan>
        <Medan label="Skenario: ubah surplus (ton)">
          <Input
            inputMode="decimal"
            value={ton}
            onChange={(e) => setTon(e.target.value)}
            placeholder="mis. -50 (panen gagal)"
            disabled={wilayah === TANPA}
            className="h-10 w-48"
          />
        </Medan>
        <button
          type="button"
          onClick={() => hitung(false)}
          disabled={!jamValid || optimasi.isPending}
          className="inline-flex h-10 items-center gap-2 rounded-xl bg-navy-900 px-4 text-sm font-medium text-white disabled:opacity-50"
        >
          {optimasi.isPending && <Loader2 className="size-4 animate-spin" />}{" "}
          Hitung rute optimal
        </button>
        {bisaSimpan && (
          <button
            type="button"
            onClick={() => hitung(true)}
            disabled={!jamValid || skenario || optimasi.isPending}
            title={
              skenario
                ? "Skenario hanya simulasi dan tidak dapat disimpan"
                : undefined
            }
            className="inline-flex h-10 items-center gap-2 rounded-xl border border-input bg-white/80 px-4 text-sm font-medium hover:bg-white disabled:opacity-50 dark:bg-white/[0.06]"
          >
            <Save className="size-4" /> Simpan sebagai usulan
          </button>
        )}
      </div>
      {hasil && <Hasil hasil={hasil} />}
    </section>
  );
}

function Hasil({ hasil }: { hasil: RencanaOptimasi }) {
  return (
    <div className="space-y-3" aria-live="polite">
      <p className="text-sm">
        <span className="num font-semibold">
          {formatAngka(hasil.totalVolumeTon, 1)} ton
        </span>{" "}
        tersalur lewat {hasil.rute.length} rute, estimasi biaya{" "}
        <span className="num font-semibold">
          {formatRupiah(hasil.totalBiaya, { compact: true })}
        </span>
        .
        {hasil.optimasiId && (
          <span className="ml-2 text-muted-foreground">
            Disimpan sebagai {hasil.optimasiId}.
          </span>
        )}
      </p>
      {hasil.rute.length > 0 && (
        <ul className="divide-y divide-border rounded-lg border border-border text-sm">
          {hasil.rute.map((r) => (
            <li
              key={r.id}
              className="flex flex-wrap items-center gap-x-3 gap-y-1 px-3 py-2"
            >
              <span className="flex min-w-0 items-center gap-2 font-medium">
                {r.asal}
                <ArrowRight
                  className="size-3.5 shrink-0 text-muted-foreground"
                  aria-hidden
                />
                {r.tujuan}
              </span>
              <span className="num ml-auto text-xs text-muted-foreground">
                {formatAngka(r.volumeTon, 1)} ton · {formatAngka(r.jarakKm, 1)}{" "}
                km · {formatAngka(r.jamTempuh, 1)} jam ·{" "}
                {formatRupiah(r.biaya, { compact: true })}
              </span>
            </li>
          ))}
        </ul>
      )}
      <Sisa judul="Defisit belum terpenuhi" items={hasil.belumTerpenuhi} />
      <Sisa judul="Surplus tak tersalurkan" items={hasil.surplusSisa} />
    </div>
  );
}

function Sisa({
  judul,
  items,
}: {
  judul: string;
  items: RencanaOptimasi["belumTerpenuhi"];
}) {
  if (items.length === 0) return null;
  return (
    <p className="text-xs text-muted-foreground">
      <span className="font-medium text-foreground">{judul}:</span>{" "}
      {items.map((s) => `${s.nama} ${formatAngka(s.ton, 1)} ton`).join(", ")}
    </p>
  );
}

function Medan({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <label className="space-y-1 text-xs text-muted-foreground">
      {label}
      {children}
    </label>
  );
}
