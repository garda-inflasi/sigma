"use client";

import { Loader2 } from "lucide-react";
import { useMemo, useState } from "react";
import {
  CartesianGrid,
  Line,
  LineChart,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
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
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import type { TransaksiPayload } from "@/lib/api/cold-storage";
import { urgensiLot } from "@/lib/api/cold-storage";
import { useCan } from "@/lib/auth/session-context";
import { KOMODITAS } from "@/lib/data/reference";
import type {
  DetailFasilitas,
  FasilitasColdStorage,
  StokColdStorage,
  TransaksiStok,
} from "@/lib/data/types";
import { garda } from "@/lib/design/tokens";
import { formatAngka, formatTanggal } from "@/lib/format";
import { useCatatTransaksi, useFasilitasDetail } from "@/lib/query/hooks";

const JENIS_LABEL: Record<TransaksiStok["jenis"], string> = {
  MASUK: "Masuk",
  KELUAR: "Keluar",
  PINDAH: "Pindah",
};

/**
 * Panel detail satu fasilitas: riwayat suhu terhadap batas alarm, lot tersimpan
 * (dengan peringatan kedaluwarsa), transaksi terbaru, dan formulir transaksi stok.
 */
export function FacilityDetailSheet({
  id,
  fasilitas,
  onClose,
}: {
  id: string | undefined;
  /** Seluruh fasilitas, untuk memilih tujuan pindah. */
  fasilitas: FasilitasColdStorage[];
  onClose: () => void;
}) {
  const { data, isLoading, isError } = useFasilitasDetail(id);

  return (
    <Sheet open={!!id} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="glass-strong w-full gap-0 overflow-y-auto sm:max-w-2xl">
        <SheetHeader>
          <SheetTitle>{data?.fasilitas.nama ?? "Detail fasilitas"}</SheetTitle>
          <SheetDescription>
            {data
              ? `${data.fasilitas.wilayah} · ${formatAngka(data.fasilitas.terisiTon, 1)} / ${formatAngka(data.fasilitas.kapasitasTon)} ton`
              : "Riwayat suhu, lot tersimpan, dan transaksi stok."}
          </SheetDescription>
        </SheetHeader>
        <div className="space-y-6 px-4 pb-8 text-sm">
          {isLoading && (
            <Loader2 className="mx-auto size-5 animate-spin text-muted-foreground" />
          )}
          {isError && (
            <p role="alert" className="text-destructive">
              Detail fasilitas tidak dapat dimuat.
            </p>
          )}
          {data && <Isi detail={data} semua={fasilitas} />}
        </div>
      </SheetContent>
    </Sheet>
  );
}

function Isi({
  detail,
  semua,
}: {
  detail: DetailFasilitas;
  semua: FasilitasColdStorage[];
}) {
  const { fasilitas: f } = detail;
  return (
    <>
      <GrafikSuhu detail={detail} />
      <DaftarLot stok={detail.stok} />
      {f.status === "OPERASIONAL" && (
        <FormTransaksi detail={detail} fasilitas={semua} />
      )}
      <Riwayat transaksi={detail.transaksi} />
    </>
  );
}

function GrafikSuhu({ detail }: { detail: DetailFasilitas }) {
  const { fasilitas: f, telemetri } = detail;
  const data = useMemo(
    () => telemetri.map((t) => ({ ...t, ts: new Date(t.waktu).getTime() })),
    [telemetri],
  );
  const lo = f.batasMinC ?? f.suhuTargetC - 2;
  const hi = f.batasMaksC ?? f.suhuTargetC + 2;
  const keluar = telemetri.filter((t) => t.diLuarRentang).length;

  return (
    <section aria-label="Riwayat suhu" className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        <h3 className="font-medium">Suhu 48 jam terakhir</h3>
        {f.alarmSuhu && (
          <StatusBadge tone="defisit">Sedang di luar batas</StatusBadge>
        )}
        {keluar > 0 && (
          <span className="text-xs text-muted-foreground">
            {keluar} pembacaan di luar batas {formatAngka(lo, 1)} sampai{" "}
            {formatAngka(hi, 1)}°C
          </span>
        )}
      </div>
      {data.length === 0 ? (
        <p className="text-xs text-muted-foreground">
          Belum ada telemetri untuk fasilitas ini.
        </p>
      ) : (
        <div className="h-48">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart
              data={data}
              margin={{ top: 8, right: 8, bottom: 0, left: 0 }}
            >
              <CartesianGrid vertical={false} stroke="var(--border)" />
              <XAxis
                dataKey="ts"
                type="number"
                scale="time"
                domain={["dataMin", "dataMax"]}
                tickFormatter={(v: number) =>
                  formatTanggal(new Date(v), "d MMM HH.mm")
                }
                tick={{ fontSize: 11, fill: "var(--muted-foreground)" }}
                axisLine={false}
                tickLine={false}
                minTickGap={40}
              />
              <YAxis
                domain={[
                  Math.min(lo, ...data.map((d) => d.suhuC)) - 1,
                  Math.max(hi, ...data.map((d) => d.suhuC)) + 1,
                ]}
                tickFormatter={(v: number) => `${Math.round(v)}°`}
                tick={{ fontSize: 11, fill: "var(--muted-foreground)" }}
                axisLine={false}
                tickLine={false}
                width={36}
              />
              <ReferenceLine
                y={lo}
                stroke={garda.navy[300]}
                strokeDasharray="4 3"
              />
              <ReferenceLine
                y={hi}
                stroke={garda.navy[300]}
                strokeDasharray="4 3"
              />
              <Tooltip
                labelFormatter={(v) =>
                  formatTanggal(new Date(Number(v)), "d MMM yyyy HH.mm")
                }
                formatter={(v) => [`${formatAngka(Number(v), 1)}°C`, "Suhu"]}
              />
              <Line
                dataKey="suhuC"
                stroke={garda.navy[600]}
                strokeWidth={2}
                isAnimationActive={false}
                dot={(p: {
                  cx?: number;
                  cy?: number;
                  payload?: { diLuarRentang: boolean };
                  index?: number;
                }) =>
                  p.payload?.diLuarRentang &&
                  p.cx !== undefined &&
                  p.cy !== undefined ? (
                    <circle
                      key={p.index}
                      cx={p.cx}
                      cy={p.cy}
                      r={3.5}
                      fill="var(--color-defisit)"
                    />
                  ) : (
                    <g key={p.index} />
                  )
                }
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}
    </section>
  );
}

function DaftarLot({ stok }: { stok: StokColdStorage[] }) {
  return (
    <section aria-label="Lot tersimpan" className="space-y-2">
      <h3 className="font-medium">Lot tersimpan ({stok.length})</h3>
      {stok.length === 0 ? (
        <p className="text-xs text-muted-foreground">Belum ada lot.</p>
      ) : (
        <ul className="divide-y divide-border rounded-lg border border-border">
          {stok.map((s) => {
            const urgensi = urgensiLot(s.sisaShelfLifeHari);
            return (
              <li
                key={s.id}
                className="flex items-center justify-between gap-3 px-3 py-2"
              >
                <div className="min-w-0">
                  <p className="truncate font-medium">{s.komoditasNama}</p>
                  <p className="num truncate text-xs text-muted-foreground">
                    {s.id} · {s.pemilik}
                  </p>
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  {s.status === "PRIORITAS_JUAL" && (
                    <StatusBadge tone="data">Prioritas jual</StatusBadge>
                  )}
                  {urgensi !== "aman" && (
                    <StatusBadge
                      tone={urgensi === "kritis" ? "defisit" : "waspada"}
                    >
                      {s.sisaShelfLifeHari} hari
                    </StatusBadge>
                  )}
                  <span className="num w-20 text-right">
                    {formatAngka(s.volumeTon, 1)} ton
                  </span>
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

function Riwayat({ transaksi }: { transaksi: TransaksiStok[] }) {
  return (
    <section aria-label="Transaksi terbaru" className="space-y-2">
      <h3 className="font-medium">Transaksi terbaru</h3>
      {transaksi.length === 0 ? (
        <p className="text-xs text-muted-foreground">Belum ada transaksi.</p>
      ) : (
        <ul className="divide-y divide-border rounded-lg border border-border text-xs">
          {transaksi.map((t) => (
            <li
              key={t.id}
              className="flex items-start justify-between gap-3 px-3 py-2"
            >
              <div className="min-w-0">
                <p>
                  <span className="font-medium">{JENIS_LABEL[t.jenis]}</span>{" "}
                  {t.komoditasNama}{" "}
                  <span className="num">{formatAngka(t.volumeTon, 1)} ton</span>
                  {t.jenis === "PINDAH" && (
                    <span className="text-muted-foreground">
                      {" "}
                      ({t.fasilitasAsalId} → {t.fasilitasTujuanId})
                    </span>
                  )}
                </p>
                <p className="truncate text-muted-foreground">
                  {t.olehNama ?? "Sistem"}
                  {t.alasan ? ` · ${t.alasan}` : ""}
                </p>
              </div>
              <time
                className="num shrink-0 text-muted-foreground"
                dateTime={t.waktu}
              >
                {formatTanggal(t.waktu, "d MMM HH.mm")}
              </time>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

type Jenis = TransaksiStok["jenis"];

/** Formulir transaksi stok; kolom menyesuaikan jenis, izin menentukan jenis yang tersedia. */
function FormTransaksi({
  detail,
  fasilitas,
}: {
  detail: DetailFasilitas;
  fasilitas: FasilitasColdStorage[];
}) {
  const bisaKelola = useCan("coldstorage.kelola");
  const bisaPindah = useCan("logistik.kelola") || bisaKelola;
  const catat = useCatatTransaksi();

  const [jenis, setJenis] = useState<Jenis>(bisaKelola ? "MASUK" : "PINDAH");
  const [komoditas, setKomoditas] = useState(KOMODITAS[0]?.kode ?? "");
  const [pemilik, setPemilik] = useState("");
  const [lotId, setLotId] = useState("");
  const [tujuan, setTujuan] = useState("");
  const [volume, setVolume] = useState("");
  const [alasan, setAlasan] = useState("");

  if (!bisaPindah) return null;

  const tujuanPilihan = fasilitas.filter(
    (x) => x.id !== detail.fasilitas.id && x.status === "OPERASIONAL",
  );
  const lot = detail.stok.find((s) => s.id === lotId);
  const volumeNum = Number(volume.replace(",", "."));
  const volumeValid =
    volume.trim() !== "" && Number.isFinite(volumeNum) && volumeNum > 0;

  const payload = ((): TransaksiPayload | undefined => {
    if (jenis === "MASUK") {
      return volumeValid && pemilik.trim() && komoditas
        ? {
            jenis,
            fasilitasId: detail.fasilitas.id,
            komoditasKode: komoditas,
            pemilik: pemilik.trim(),
            volumeTon: volumeNum,
          }
        : undefined;
    }
    if (!lot || !volumeValid) return undefined;
    if (jenis === "KELUAR") {
      return alasan.trim()
        ? { jenis, lotId: lot.id, volumeTon: volumeNum, alasan: alasan.trim() }
        : undefined;
    }
    return tujuan
      ? {
          jenis,
          lotId: lot.id,
          fasilitasTujuanId: tujuan,
          volumeTon: volumeNum,
        }
      : undefined;
  })();

  const kirim = () => {
    if (!payload) return;
    catat.mutate(payload, {
      onSuccess: () => {
        setVolume("");
        setAlasan("");
        setPemilik("");
      },
    });
  };

  return (
    <section
      aria-label="Catat transaksi"
      className="space-y-3 rounded-lg border border-border p-3"
    >
      <h3 className="font-medium">Catat transaksi stok</h3>
      <div className="flex flex-wrap gap-3">
        <Medan label="Jenis">
          <Select
            value={jenis}
            onValueChange={(v: string) => setJenis(v as Jenis)}
          >
            <SelectTrigger className="h-9 w-32">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {bisaKelola && <SelectItem value="MASUK">Masuk</SelectItem>}
              {bisaKelola && <SelectItem value="KELUAR">Keluar</SelectItem>}
              <SelectItem value="PINDAH">Pindah</SelectItem>
            </SelectContent>
          </Select>
        </Medan>

        {jenis === "MASUK" ? (
          <>
            <Medan label="Komoditas">
              <Select value={komoditas} onValueChange={setKomoditas}>
                <SelectTrigger className="h-9 w-48">
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
            <Medan label="Pemilik">
              <Input
                value={pemilik}
                onChange={(e) => setPemilik(e.target.value)}
                className="h-9 w-48"
                maxLength={255}
              />
            </Medan>
          </>
        ) : (
          <Medan label="Lot">
            <Select value={lotId} onValueChange={setLotId}>
              <SelectTrigger className="h-9 w-64">
                <SelectValue placeholder="Pilih lot" />
              </SelectTrigger>
              <SelectContent>
                {detail.stok.map((s) => (
                  <SelectItem key={s.id} value={s.id}>
                    {s.komoditasNama} · {formatAngka(s.volumeTon, 1)} ton
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Medan>
        )}

        {jenis === "PINDAH" && (
          <Medan label="Fasilitas tujuan">
            <Select value={tujuan} onValueChange={setTujuan}>
              <SelectTrigger className="h-9 w-56">
                <SelectValue placeholder="Pilih tujuan" />
              </SelectTrigger>
              <SelectContent>
                {tujuanPilihan.map((x) => (
                  <SelectItem key={x.id} value={x.id}>
                    {x.nama}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Medan>
        )}

        <Medan
          label={
            lot
              ? `Volume (ton, maks ${formatAngka(lot.volumeTon, 1)})`
              : "Volume (ton)"
          }
        >
          <Input
            inputMode="decimal"
            value={volume}
            onChange={(e) => setVolume(e.target.value)}
            className="h-9 w-40"
            aria-invalid={volume !== "" && !volumeValid}
          />
        </Medan>

        {jenis === "KELUAR" && (
          <Medan label="Alasan">
            <Input
              value={alasan}
              onChange={(e) => setAlasan(e.target.value)}
              className="h-9 w-64"
              maxLength={500}
            />
          </Medan>
        )}
      </div>
      <button
        type="button"
        onClick={kirim}
        disabled={!payload || catat.isPending}
        className="inline-flex h-9 items-center gap-2 rounded-lg bg-navy-900 px-4 text-sm font-medium text-white disabled:opacity-50"
      >
        {catat.isPending && <Loader2 className="size-4 animate-spin" />} Catat
        transaksi
      </button>
    </section>
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
