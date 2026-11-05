"use client";

import { Loader2 } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { StatusBadge } from "@/components/garda/status-badge";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  angkaIsian,
  dariDatetimeLocal,
  galatPembacaan,
  galatUbahFasilitas,
  keDatetimeLocal,
  type HasilTelemetri,
  type UbahFasilitasPayload,
} from "@/lib/api/cold-storage";
import { useCan } from "@/lib/auth/session-context";
import type {
  DetailFasilitas,
  FasilitasColdStorage,
  TitikSuhu,
} from "@/lib/data/types";
import { formatAngka, formatTanggal } from "@/lib/format";
import { useCatatTelemetri, useUbahFasilitas } from "@/lib/query/hooks";

const STATUS_LABEL: Record<FasilitasColdStorage["status"], string> = {
  OPERASIONAL: "Operasional",
  PERAWATAN: "Perawatan",
  RENCANA: "Rencana",
};

const SUMBER_LABEL: Record<string, string> = {
  SENSOR: "Sensor",
  MANUAL: "Manual",
  DEMO: "Demo",
};

const TOMBOL_UTAMA =
  "inline-flex h-9 items-center gap-2 rounded-lg bg-navy-900 px-4 text-sm font-medium text-white disabled:opacity-50";
const TOMBOL_SEKUNDER =
  "inline-flex h-9 items-center gap-2 rounded-lg border border-border px-4 text-sm font-medium hover:bg-secondary aria-pressed:bg-secondary";

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

/** Lima pembacaan terbaru beserta sumbernya, termasuk yang dicatat manual. */
export function PembacaanTerbaru({ telemetri }: { telemetri: TitikSuhu[] }) {
  const terbaru = telemetri.slice(-5).reverse();
  if (terbaru.length === 0) return null;
  return (
    <section aria-label="Pembacaan terbaru" className="space-y-2">
      <h3 className="font-medium">Pembacaan terbaru</h3>
      <ul className="divide-y divide-border rounded-lg border border-border text-xs">
        {terbaru.map((t) => (
          <li
            key={t.waktu}
            className="flex items-center justify-between gap-3 px-3 py-2"
          >
            <span className="num">
              {formatAngka(t.suhuC, 1)}°C
              {t.kelembapanPct !== null && (
                <span className="text-muted-foreground">
                  {" "}
                  · {formatAngka(t.kelembapanPct, 0)}%
                </span>
              )}
            </span>
            <span className="flex items-center gap-2">
              {t.diLuarRentang && (
                <StatusBadge tone="defisit">Di luar batas</StatusBadge>
              )}
              {t.sumber && (
                <StatusBadge tone={t.sumber === "MANUAL" ? "data" : "netral"}>
                  {SUMBER_LABEL[t.sumber] ?? t.sumber}
                </StatusBadge>
              )}
              <time className="num text-muted-foreground" dateTime={t.waktu}>
                {formatTanggal(t.waktu, "d MMM HH.mm.ss")}
              </time>
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}

/**
 * Pengelolaan fasilitas (izin `coldstorage.kelola`): ubah kapasitas/batas suhu/status dan catat
 * suhu manual. Tidak dirender sama sekali bagi peran tanpa izin; Go tetap yang berwenang.
 */
export function PengelolaanFasilitas({ detail }: { detail: DetailFasilitas }) {
  const bisaKelola = useCan("coldstorage.kelola");
  const [mode, setMode] = useState<"ubah" | "catat" | null>(null);
  if (!bisaKelola) return null;

  const { fasilitas: f } = detail;
  // Fasilitas rencana belum menerima telemetri (ditolak backend).
  const bisaCatat = f.status !== "RENCANA";
  const alih = (m: "ubah" | "catat") =>
    setMode((now) => (now === m ? null : m));

  return (
    <section aria-label="Pengelolaan fasilitas" className="space-y-3">
      <div className="flex flex-wrap gap-2">
        <button
          type="button"
          className={TOMBOL_SEKUNDER}
          aria-pressed={mode === "ubah"}
          onClick={() => alih("ubah")}
        >
          Ubah fasilitas
        </button>
        {bisaCatat && (
          <button
            type="button"
            className={TOMBOL_SEKUNDER}
            aria-pressed={mode === "catat"}
            onClick={() => alih("catat")}
          >
            Catat suhu manual
          </button>
        )}
      </div>
      {mode === "ubah" && (
        <FormUbahFasilitas
          key={`${f.id}-${f.kapasitasTon}-${f.batasMinC}-${f.batasMaksC}-${f.status}`}
          fasilitas={f}
          onSelesai={() => setMode(null)}
        />
      )}
      {mode === "catat" && bisaCatat && (
        <FormCatatSuhu fasilitas={f} onTutup={() => setMode(null)} />
      )}
    </section>
  );
}

function FormUbahFasilitas({
  fasilitas: f,
  onSelesai,
}: {
  fasilitas: FasilitasColdStorage;
  onSelesai: () => void;
}) {
  const ubah = useUbahFasilitas(f.id);
  // Batas efektif (bawaan target ± 2 °C bila belum diatur) sebagai nilai awal.
  const minAwal = f.batasMinC ?? f.suhuTargetC - 2;
  const maksAwal = f.batasMaksC ?? f.suhuTargetC + 2;

  const [kapasitas, setKapasitas] = useState(String(f.kapasitasTon));
  const [suhuMin, setSuhuMin] = useState(String(minAwal));
  const [suhuMaks, setSuhuMaks] = useState(String(maksAwal));
  const [status, setStatus] = useState<FasilitasColdStorage["status"]>(
    f.status,
  );

  const kapasitasNum = angkaIsian(kapasitas);
  const minNum = angkaIsian(suhuMin);
  const maksNum = angkaIsian(suhuMaks);

  // Hanya kolom yang berubah yang dikirim.
  const payload: UbahFasilitasPayload = {};
  if (kapasitasNum !== f.kapasitasTon) payload.kapasitasTon = kapasitasNum;
  if (minNum !== minAwal) payload.suhuMinC = minNum;
  if (maksNum !== maksAwal) payload.suhuMaksC = maksNum;
  if (status !== f.status) payload.status = status;
  const berubah = Object.keys(payload).length > 0;

  const galat = galatUbahFasilitas({
    kapasitasTon: kapasitasNum,
    suhuMinC: minNum,
    suhuMaksC: maksNum,
  });

  const simpan = (e: React.FormEvent) => {
    e.preventDefault();
    if (!berubah || galat) return;
    ubah.mutate(payload, { onSuccess: onSelesai });
  };

  return (
    <form
      aria-label="Ubah fasilitas"
      onSubmit={simpan}
      className="space-y-3 rounded-lg border border-border p-3"
    >
      <h3 className="font-medium">Ubah fasilitas</h3>
      <div className="flex flex-wrap gap-3">
        <Medan label="Kapasitas (ton)">
          <Input
            inputMode="numeric"
            value={kapasitas}
            onChange={(e) => setKapasitas(e.target.value)}
            className="h-9 w-32"
          />
        </Medan>
        <Medan label="Suhu minimum (°C)">
          <Input
            inputMode="decimal"
            value={suhuMin}
            onChange={(e) => setSuhuMin(e.target.value)}
            className="h-9 w-32"
          />
        </Medan>
        <Medan label="Suhu maksimum (°C)">
          <Input
            inputMode="decimal"
            value={suhuMaks}
            onChange={(e) => setSuhuMaks(e.target.value)}
            className="h-9 w-32"
          />
        </Medan>
        <Medan label="Status">
          <Select
            value={status}
            onValueChange={(v: string) =>
              setStatus(v as FasilitasColdStorage["status"])
            }
          >
            <SelectTrigger className="h-9 w-40" aria-label="Status">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {Object.entries(STATUS_LABEL).map(([nilai, label]) => (
                <SelectItem key={nilai} value={nilai}>
                  {label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </Medan>
      </div>
      <p className="text-xs text-muted-foreground">
        Batas suhu menentukan kapan pembacaan ditandai di luar rentang.
        Kapasitas tidak boleh lebih kecil dari stok yang tersimpan.
      </p>
      {galat && (
        <p role="alert" className="text-xs text-destructive">
          {galat}
        </p>
      )}
      <div className="flex gap-2">
        <button
          type="submit"
          disabled={!berubah || !!galat || ubah.isPending}
          className={TOMBOL_UTAMA}
        >
          {ubah.isPending && <Loader2 className="size-4 animate-spin" />} Simpan
          perubahan
        </button>
        <button type="button" onClick={onSelesai} className={TOMBOL_SEKUNDER}>
          Batal
        </button>
      </div>
    </form>
  );
}

function FormCatatSuhu({
  fasilitas: f,
  onTutup,
}: {
  fasilitas: FasilitasColdStorage;
  onTutup: () => void;
}) {
  const catat = useCatatTelemetri();
  const [suhu, setSuhu] = useState("");
  const [lembap, setLembap] = useState("");
  // Bawaan: sekarang, supaya pencatatan langsung di lapangan tidak perlu mengisi waktu.
  const [waktu, setWaktu] = useState(() => keDatetimeLocal(new Date()));
  const [hasil, setHasil] = useState<HasilTelemetri | null>(null);

  const suhuNum = angkaIsian(suhu);
  const lembapNum = lembap.trim() === "" ? undefined : angkaIsian(lembap);
  const waktuDate = dariDatetimeLocal(waktu);
  const galat =
    suhu.trim() === ""
      ? "Suhu wajib diisi."
      : galatPembacaan({
          suhuC: suhuNum,
          kelembapanPct: lembapNum,
          waktu: waktuDate,
        });

  const simpan = (e: React.FormEvent) => {
    e.preventDefault();
    if (galat || !waktuDate) return;
    catat.mutate(
      {
        fasilitasId: f.id,
        waktu: waktuDate.toISOString(),
        suhuC: suhuNum,
        kelembapanPct: lembapNum,
      },
      {
        onSuccess: (res) => {
          setHasil(res);
          setSuhu("");
          if (res.duplikat > 0)
            toast.warning("Pembacaan sudah pernah dicatat", {
              description:
                "Sudah ada pembacaan pada waktu yang sama; tidak disimpan ulang.",
            });
          else
            toast.success("Pembacaan suhu tercatat", {
              description: "Tercatat di audit trail.",
            });
          if (res.diLuarRentang.length > 0)
            toast.error("Suhu di luar batas alarm", {
              description: `${res.diLuarRentang.length} pembacaan melewati batas fasilitas.`,
            });
        },
      },
    );
  };

  return (
    <form
      aria-label="Catat suhu manual"
      onSubmit={simpan}
      className="space-y-3 rounded-lg border border-border p-3"
    >
      <h3 className="font-medium">Catat suhu manual</h3>
      <div className="flex flex-wrap gap-3">
        <Medan label="Suhu (°C)">
          <Input
            inputMode="decimal"
            value={suhu}
            onChange={(e) => setSuhu(e.target.value)}
            className="h-9 w-32"
            aria-invalid={suhu.trim() !== "" && !!galat}
          />
        </Medan>
        <Medan label="Kelembapan (%), opsional">
          <Input
            inputMode="decimal"
            value={lembap}
            onChange={(e) => setLembap(e.target.value)}
            className="h-9 w-40"
          />
        </Medan>
        <Medan label="Waktu pembacaan">
          <Input
            type="datetime-local"
            step={1}
            value={waktu}
            onChange={(e) => setWaktu(e.target.value)}
            className="h-9 w-56"
          />
        </Medan>
      </div>
      {suhu.trim() !== "" && galat && (
        <p role="alert" className="text-xs text-destructive">
          {galat}
        </p>
      )}
      {hasil && <RingkasanHasil hasil={hasil} />}
      <div className="flex gap-2">
        <button
          type="submit"
          disabled={!!galat || catat.isPending}
          className={TOMBOL_UTAMA}
        >
          {catat.isPending && <Loader2 className="size-4 animate-spin" />}{" "}
          Simpan pembacaan
        </button>
        <button type="button" onClick={onTutup} className={TOMBOL_SEKUNDER}>
          Tutup
        </button>
      </div>
    </form>
  );
}

/** Hasil kiriman terakhir: pembacaan tersimpan, duplikat yang dilewati, dan alarm di luar batas. */
function RingkasanHasil({ hasil }: { hasil: HasilTelemetri }) {
  return (
    <div
      role="status"
      className="space-y-1 rounded-md bg-secondary/60 p-2 text-xs"
    >
      {hasil.diterima > 0 && <p>{hasil.diterima} pembacaan tersimpan.</p>}
      {hasil.duplikat > 0 && (
        <p>
          {hasil.duplikat} pembacaan dilewati karena sudah ada pada waktu yang
          sama.
        </p>
      )}
      {hasil.diLuarRentang.map((a) => (
        <p key={a.waktu} className="text-destructive">
          Suhu {formatAngka(a.suhuC, 1)}°C di luar batas{" "}
          {formatAngka(a.batasMinC, 1)} sampai {formatAngka(a.batasMaksC, 1)}°C
          ({formatTanggal(a.waktu, "d MMM HH.mm.ss")}).
        </p>
      ))}
    </div>
  );
}
