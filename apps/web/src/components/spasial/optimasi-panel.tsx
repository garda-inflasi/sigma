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
import { formatAngka, formatRupiah, formatTanggal } from "@/lib/format";
import { useOptimasiRute, useRiwayatOptimasi } from "@/lib/query/hooks";
import { FormBff } from "@/components/form/form-bff";
import { skemaDariValidasi } from "@/lib/form/skema";
import { useFormBff } from "@/lib/form/use-form-bff";

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
  // Toast hasil/galat dari hook mutasi; galat juga tampil di panel. Enter di isian = "Hitung rute optimal".
  const kiriman = useFormBff<OptimasiParams, RencanaOptimasi>({
    skema: skemaOptimasi,
    kirim: (p) => optimasi.mutateAsync(p),
    sukses: (rencana) => setHasil(rencana),
  });

  return (
    <section
      className="glass space-y-4 rounded-xl p-5"
      aria-label="Optimasi rute"
    >
      <PanelTitle
        title="Optimasi rute distribusi"
        description="Rencana biaya terendah dari wilayah surplus ke wilayah defisit menurut neraca terbaru; volume tersalur dimaksimalkan lebih dulu."
      />
      <FormBff
        onKirim={kiriman.kirimDari(() => params(false))}
        sibuk={kiriman.pending}
        className="flex flex-wrap items-end gap-3"
      >
        <Medan label="Komoditas">
          <Select value={komoditas} onValueChange={setKomoditas}>
            <SelectTrigger aria-label="Komoditas optimasi" className="h-10 w-52">
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
          type="submit"
          disabled={!jamValid || kiriman.pending}
          className="inline-flex h-10 items-center gap-2 rounded-xl bg-navy-900 px-4 text-sm font-medium text-white disabled:opacity-50"
        >
          {kiriman.pending && <Loader2 className="size-4 animate-spin" />}{" "}
          Hitung rute optimal
        </button>
        {bisaSimpan && (
          <button
            type="button"
            onClick={() => void kiriman.kirimNilai(params(true))}
            disabled={!jamValid || skenario || kiriman.pending}
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
      </FormBff>
      {kiriman.galat && (
        <p role="alert" className="text-sm text-defisit-ink dark:text-rose-300">
          {kiriman.galat.pesan}
        </p>
      )}
      {hasil && <Hasil hasil={hasil} />}
      <Riwayat />
    </section>
  );
}

/** Batas waktu tempuh opsional, 0–240 jam (server memeriksa ulang). */
const skemaOptimasi = skemaDariValidasi<OptimasiParams>((p) =>
  p.maksJamTempuh !== undefined && !(p.maksJamTempuh > 0 && p.maksJamTempuh <= 240)
    ? "Batas waktu tempuh harus lebih dari 0 dan paling lama 240 jam."
    : null,
);

/** Rencana yang pernah disimpan sebagai usulan rute (terbaru dulu, paling banyak 20). */
function Riwayat() {
  const { data, isLoading, isError, error } = useRiwayatOptimasi();
  return (
    <div className="space-y-2 border-t border-border pt-4" aria-label="Riwayat optimasi">
      <h3 className="text-sm font-semibold">Riwayat optimasi</h3>
      {isLoading ? (
        <p className="text-sm text-muted-foreground">Memuat riwayat…</p>
      ) : isError ? (
        <p role="alert" className="text-sm text-defisit-ink dark:text-rose-300">
          Gagal memuat riwayat optimasi{error instanceof Error ? `: ${error.message}` : "."}
        </p>
      ) : !data || data.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          Belum ada rencana yang disimpan. Hitung rute lalu pilih “Simpan sebagai usulan”.
        </p>
      ) : (
        <ul className="divide-y divide-border rounded-lg border border-border text-sm">
          {data.map((r) => (
            <li key={r.id} className="flex flex-wrap items-center gap-x-3 gap-y-1 px-3 py-2">
              <span className="font-medium">{r.komoditas}</span>
              <span className="text-xs text-muted-foreground">periode {formatTanggal(r.periode, "MMMM yyyy")}</span>
              <span className="num text-xs text-muted-foreground">
                {formatAngka(r.totalVolumeTon, 1)} ton · {formatRupiah(r.totalBiaya, { compact: true })}
                {r.belumTerpenuhiTon > 0 && ` · defisit tersisa ${formatAngka(r.belumTerpenuhiTon, 1)} ton`}
              </span>
              <span className="ml-auto text-xs text-muted-foreground">
                oleh {r.dibuatOleh ?? "akun terhapus"}, {formatTanggal(r.dibuatPada, "d MMM yyyy, HH.mm")}
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
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
        {hasil.penghematanPersen !== null && (
          <span className="ml-2 text-muted-foreground">
            Hemat{" "}
            <span className="num font-medium text-foreground">
              {formatAngka(hasil.penghematanPersen, 1)}%
            </span>{" "}
            dibanding rencana serakah (pasangan terdekat dulu).
          </span>
        )}
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
