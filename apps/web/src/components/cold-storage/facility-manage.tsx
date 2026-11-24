"use client";

import { Loader2 } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { FormBff } from "@/components/form/form-bff";
import { FormAlert, FormField } from "@/components/form/form-field";
import { SpringButton } from "@/components/garda/spring-button";
import { StatusBadge } from "@/components/garda/status-badge";
import { Button } from "@/components/ui/button";
import { DateTimeField } from "@/components/ui/date-field";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { SheetFooter } from "@/components/ui/sheet";
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
import { skemaDariValidasi } from "@/lib/form/skema";
import { useFormBff } from "@/lib/form/use-form-bff";
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

/** Lima pembacaan terbaru beserta sumbernya, termasuk yang dicatat manual. */
export function PembacaanTerbaru({ telemetri }: { telemetri: TitikSuhu[] }) {
  const terbaru = telemetri.slice(-5).reverse();
  if (terbaru.length === 0) return null;
  return (
    <section
      aria-label="Pembacaan terbaru"
      className="space-y-3 border-t border-border px-5 py-4"
    >
      <h3 className="text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
        Pembacaan terbaru
      </h3>
      <ul className="divide-y divide-border rounded-lg border border-border text-xs">
        {terbaru.map((t) => (
          <li
            key={t.waktu}
            className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 px-3 py-2"
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
            <span className="flex flex-wrap items-center gap-2">
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

export type ModeKelola = "ubah" | "catat" | null;

/**
 * Tombol pengelolaan fasilitas (izin `coldstorage.kelola`) untuk footer panel detail. Tidak dirender sama sekali
 * bagi peran tanpa izin; Go tetap yang berwenang.
 */
export function AksiKelolaFasilitas({
  fasilitas: f,
  mode,
  onMode,
}: {
  fasilitas: FasilitasColdStorage;
  mode: ModeKelola;
  onMode: (m: ModeKelola) => void;
}) {
  const bisaKelola = useCan("coldstorage.kelola");
  if (!bisaKelola) return null;
  // Fasilitas rencana belum menerima telemetri (ditolak backend).
  const bisaCatat = f.status !== "RENCANA";
  const alih = (m: "ubah" | "catat") => onMode(mode === m ? null : m);

  return (
    <SheetFooter role="group" aria-label="Pengelolaan fasilitas">
      {bisaCatat && (
        <Button
          variant="outline"
          className="aria-pressed:bg-secondary"
          aria-pressed={mode === "catat"}
          onClick={() => alih("catat")}
        >
          Catat suhu manual
        </Button>
      )}
      <Button
        variant="outline"
        className="aria-pressed:bg-secondary"
        aria-pressed={mode === "ubah"}
        onClick={() => alih("ubah")}
      >
        Ubah fasilitas
      </Button>
    </SheetFooter>
  );
}

/** Formulir yang dibuka tombol di footer; tampil di dalam isi panel agar footer tetap ringkas. */
export function FormKelolaFasilitas({
  fasilitas: f,
  mode,
  onTutup,
}: {
  fasilitas: FasilitasColdStorage;
  mode: ModeKelola;
  onTutup: () => void;
}) {
  const bisaKelola = useCan("coldstorage.kelola");
  const wadah = useRef<HTMLDivElement>(null);
  // Tombol ada di footer, sedangkan isi panel bisa sudah tergulir jauh dari formulir.
  useEffect(() => {
    if (mode) wadah.current?.scrollIntoView({ block: "nearest" });
  }, [mode]);
  if (!bisaKelola || !mode) return null;

  return (
    <div ref={wadah} className="border-t border-border px-5 py-4">
      {mode === "ubah" && (
        <FormUbahFasilitas
          key={`${f.id}-${f.kapasitasTon}-${f.batasMinC}-${f.batasMaksC}-${f.status}`}
          fasilitas={f}
          onSelesai={onTutup}
        />
      )}
      {mode === "catat" && f.status !== "RENCANA" && (
        <FormCatatSuhu fasilitas={f} onTutup={onTutup} />
      )}
    </div>
  );
}

interface MasukanUbah {
  /** Hanya kolom yang berubah. */
  payload: UbahFasilitasPayload;
  nilai: { kapasitasTon: number; suhuMinC: number; suhuMaksC: number };
}
const skemaUbah = skemaDariValidasi<MasukanUbah>((m) =>
  Object.keys(m.payload).length === 0
    ? "Belum ada perubahan."
    : galatUbahFasilitas(m.nilai),
);

interface MasukanCatat {
  suhuKosong: boolean;
  suhuC: number;
  kelembapanPct?: number;
  waktu: Date | null;
}
const galatCatat = (m: MasukanCatat) =>
  m.suhuKosong ? "Suhu wajib diisi." : galatPembacaan(m);
const skemaCatat = skemaDariValidasi<MasukanCatat>(galatCatat);

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

  // Galat isian tampil langsung saat mengetik; galat server (toast dari hook mutasi) juga tampil di form.
  const galatIsian = galatUbahFasilitas({
    kapasitasTon: kapasitasNum,
    suhuMinC: minNum,
    suhuMaksC: maksNum,
  });
  const kiriman = useFormBff<MasukanUbah>({
    skema: skemaUbah,
    kirim: (m) => ubah.mutateAsync(m.payload),
    sukses: () => onSelesai(),
  });
  const galat = galatIsian ?? kiriman.galat?.pesan ?? null;
  const simpan = kiriman.kirimDari(() => ({
    payload,
    nilai: { kapasitasTon: kapasitasNum, suhuMinC: minNum, suhuMaksC: maksNum },
  }));

  return (
    <FormBff
      aria-label="Ubah fasilitas"
      onKirim={simpan}
      sibuk={kiriman.pending}
      className="space-y-3 rounded-lg border border-border p-3"
    >
      <h3 className="font-medium">Ubah fasilitas</h3>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <FormField id="fasilitas-kapasitas" label="Kapasitas (ton)">
          <Input
            id="fasilitas-kapasitas"
            inputMode="numeric"
            value={kapasitas}
            onChange={(e) => setKapasitas(e.target.value)}
          />
        </FormField>
        <FormField id="fasilitas-suhu-min" label="Suhu minimum (°C)">
          <Input
            id="fasilitas-suhu-min"
            inputMode="decimal"
            value={suhuMin}
            onChange={(e) => setSuhuMin(e.target.value)}
          />
        </FormField>
        <FormField id="fasilitas-suhu-maks" label="Suhu maksimum (°C)">
          <Input
            id="fasilitas-suhu-maks"
            inputMode="decimal"
            value={suhuMaks}
            onChange={(e) => setSuhuMaks(e.target.value)}
          />
        </FormField>
        <FormField id="fasilitas-status" label="Status">
          <Select
            value={status}
            onValueChange={(v: string) =>
              setStatus(v as FasilitasColdStorage["status"])
            }
          >
            <SelectTrigger id="fasilitas-status" aria-label="Status">
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
        </FormField>
      </div>
      <p className="text-xs text-muted-foreground">
        Batas suhu menentukan kapan pembacaan ditandai di luar rentang.
        Kapasitas tidak boleh lebih kecil dari stok yang tersimpan.
      </p>
      <FormAlert>{galat}</FormAlert>
      <div className="flex flex-wrap gap-2">
        <SpringButton
          type="submit"
          variant="primary"
          disabled={!berubah || !!galatIsian || kiriman.pending}
        >
          {kiriman.pending && <Loader2 className="animate-spin" />}
          Simpan perubahan
        </SpringButton>
        <Button variant="outline" onClick={onSelesai}>
          Batal
        </Button>
      </div>
    </FormBff>
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
  const galatIsian = galatCatat({
    suhuKosong: suhu.trim() === "",
    suhuC: suhuNum,
    kelembapanPct: lembapNum,
    waktu: waktuDate,
  });

  const kiriman = useFormBff<MasukanCatat, HasilTelemetri>({
    skema: skemaCatat,
    kirim: (m) =>
      catat.mutateAsync({
        fasilitasId: f.id,
        waktu: m.waktu!.toISOString(),
        suhuC: m.suhuC,
        kelembapanPct: m.kelembapanPct,
      }),
    sukses: (res) => {
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
  });
  const galat = galatIsian ?? kiriman.galat?.pesan ?? null;
  const simpan = kiriman.kirimDari(() => ({
    suhuKosong: suhu.trim() === "",
    suhuC: suhuNum,
    kelembapanPct: lembapNum,
    waktu: waktuDate,
  }));

  return (
    <FormBff
      aria-label="Catat suhu manual"
      onKirim={simpan}
      sibuk={kiriman.pending}
      className="space-y-3 rounded-lg border border-border p-3"
    >
      <h3 className="font-medium">Catat suhu manual</h3>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <FormField id="catat-suhu" label="Suhu (°C)">
          <Input
            id="catat-suhu"
            inputMode="decimal"
            value={suhu}
            onChange={(e) => setSuhu(e.target.value)}
            aria-invalid={suhu.trim() !== "" && !!galatIsian}
          />
        </FormField>
        <FormField id="catat-kelembapan" label="Kelembapan (%), opsional">
          <Input
            id="catat-kelembapan"
            inputMode="decimal"
            value={lembap}
            onChange={(e) => setLembap(e.target.value)}
          />
        </FormField>
        <FormField
          id="catat-waktu"
          label="Waktu pembacaan"
          className="sm:col-span-2"
        >
          <DateTimeField id="catat-waktu" value={waktu} onValueChange={setWaktu} />
        </FormField>
      </div>
      {suhu.trim() !== "" && <FormAlert>{galat}</FormAlert>}
      {hasil && <RingkasanHasil hasil={hasil} />}
      <div className="flex flex-wrap gap-2">
        <SpringButton
          type="submit"
          variant="primary"
          disabled={!!galatIsian || kiriman.pending}
        >
          {kiriman.pending && <Loader2 className="animate-spin" />}
          Simpan pembacaan
        </SpringButton>
        <Button variant="outline" onClick={onTutup}>
          Tutup
        </Button>
      </div>
    </FormBff>
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
