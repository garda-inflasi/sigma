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
import { EmptyState } from "@/components/garda/empty-state";
import {
  InspectorRingkasan,
  InspectorSection,
  MetaGrid,
  MetaItem,
} from "@/components/garda/inspector";
import { PanelGalat } from "@/components/garda/panel-galat";
import { Shimmer } from "@/components/garda/shimmer";
import { SpringButton } from "@/components/garda/spring-button";
import { StatusBadge } from "@/components/garda/status-badge";
import {
  AksiKelolaFasilitas,
  FormKelolaFasilitas,
  PembacaanTerbaru,
  type ModeKelola,
} from "./facility-manage";
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
  SheetBody,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import type { TransaksiPayload } from "@/lib/api/cold-storage";
import {
  labelUtilisasi,
  nadaUtilisasi,
  urgensiLot,
  utilisasiPersen,
} from "@/lib/api/cold-storage";
import { useCan } from "@/lib/auth/session-context";
import { KOMODITAS } from "@/lib/data/reference";
import type {
  DetailFasilitas,
  FasilitasColdStorage,
  StokColdStorage,
  TransaksiStok,
} from "@/lib/data/types";
import { garda, type Tone } from "@/lib/design/tokens";
import { formatAngka, formatTanggal } from "@/lib/format";
import { useCatatTransaksi, useFasilitasDetail } from "@/lib/query/hooks";
import { z } from "zod";
import { FormBff } from "@/components/form/form-bff";
import { FormAlert } from "@/components/form/form-field";
import type { GalatForm } from "@/lib/form/kiriman";
import { useFormBff } from "@/lib/form/use-form-bff";

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
  const { data, isLoading, isError, error } = useFasilitasDetail(id);

  return (
    <Sheet open={!!id} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="sm:max-w-2xl">
        <SheetHeader>
          <SheetTitle>{data?.fasilitas.nama ?? "Detail fasilitas"}</SheetTitle>
          <SheetDescription>
            {data
              ? data.fasilitas.wilayah
              : "Riwayat suhu, lot tersimpan, dan transaksi stok."}
          </SheetDescription>
        </SheetHeader>
        {data ? (
          <Isi key={data.fasilitas.id} detail={data} semua={fasilitas} />
        ) : (
          <SheetBody className="text-sm">
            {isLoading && (
              <div
                role="status"
                aria-label="Memuat detail"
                className="space-y-3 px-5 py-4"
              >
                <Shimmer className="h-4 w-1/3" />
                <Shimmer className="h-48 w-full" />
                <Shimmer className="h-32 w-full" />
              </div>
            )}
            {isError && (
              <div className="px-5 py-4">
                <PanelGalat
                  judul="Detail fasilitas tidak dapat dimuat."
                  error={error}
                />
              </div>
            )}
          </SheetBody>
        )}
      </SheetContent>
    </Sheet>
  );
}

/** `InspectorSection` yang tetap menjadi region bernama; garis pemisah ada di pembungkus. */
function Seksi({
  label,
  title,
  children,
}: {
  label: string;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section
      aria-label={label}
      className="border-t border-border first:border-t-0"
    >
      <InspectorSection title={title}>{children}</InspectorSection>
    </section>
  );
}

const LABEL_STATUS: Record<
  FasilitasColdStorage["status"],
  { label: string; tone: Tone }
> = {
  OPERASIONAL: { label: "Operasional", tone: "normal" },
  PERAWATAN: { label: "Perawatan", tone: "waspada" },
  RENCANA: { label: "Rencana", tone: "netral" },
};

/** Urutan seragam: ringkasan, rincian, riwayat suhu dan lot, lalu aksi di footer. */
function Isi({
  detail,
  semua,
}: {
  detail: DetailFasilitas;
  semua: FasilitasColdStorage[];
}) {
  const { fasilitas: f } = detail;
  const [mode, setMode] = useState<ModeKelola>(null);
  const status = LABEL_STATUS[f.status];
  const persen = utilisasiPersen(f);
  const rencana = f.status === "RENCANA";
  const batasMin = f.batasMinC ?? f.suhuTargetC - 2;
  const batasMaks = f.batasMaksC ?? f.suhuTargetC + 2;

  return (
    <>
      <SheetBody className="text-sm">
        <InspectorRingkasan
          status={
            <>
              <StatusBadge tone={status.tone}>{status.label}</StatusBadge>
              {f.alarmSuhu && (
                <StatusBadge tone="defisit">Suhu di luar batas</StatusBadge>
              )}
              {!rencana && (
                <StatusBadge tone={nadaUtilisasi(persen)}>
                  {labelUtilisasi(persen)}
                </StatusBadge>
              )}
            </>
          }
          nilai={rencana ? `${formatAngka(f.kapasitasTon)} ton` : `${Math.round(persen)}% terisi`}
          keterangan={
            rencana
              ? "Kapasitas rencana; fasilitas belum beroperasi."
              : `${formatAngka(f.terisiTon, 1)} dari ${formatAngka(f.kapasitasTon)} ton kapasitas`
          }
        />
        <InspectorSection title="Rincian">
          <MetaGrid>
            <MetaItem label="Suhu sekarang">
              <span className="num">
                {f.suhuAktualC === null
                  ? "Belum ada data"
                  : `${formatAngka(f.suhuAktualC, 1)}°C`}
              </span>
            </MetaItem>
            <MetaItem label="Target suhu">
              <span className="num">{formatAngka(f.suhuTargetC, 1)}°C</span>
            </MetaItem>
            <MetaItem label="Batas alarm">
              <span className="num">
                {formatAngka(batasMin, 1)} sampai {formatAngka(batasMaks, 1)}°C
              </span>
            </MetaItem>
            <MetaItem label="Pembacaan terakhir">
              <span className="num">
                {f.telemetriPada
                  ? formatTanggal(f.telemetriPada, "d MMM yyyy HH.mm")
                  : "Belum ada"}
              </span>
            </MetaItem>
          </MetaGrid>
        </InspectorSection>
        <FormKelolaFasilitas
          fasilitas={f}
          mode={mode}
          onTutup={() => setMode(null)}
        />
        <GrafikSuhu detail={detail} />
        <PembacaanTerbaru telemetri={detail.telemetri} />
        <DaftarLot stok={detail.stok} />
        {f.status === "OPERASIONAL" && (
          <FormTransaksi detail={detail} fasilitas={semua} />
        )}
        <Riwayat transaksi={detail.transaksi} />
      </SheetBody>
      <AksiKelolaFasilitas fasilitas={f} mode={mode} onMode={setMode} />
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
    <Seksi label="Riwayat suhu" title="Suhu 48 jam terakhir">
      {(f.alarmSuhu || keluar > 0) && (
        <div className="flex flex-wrap items-center gap-2">
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
      )}
      {data.length === 0 ? (
        <EmptyState
          title="Belum ada telemetri untuk fasilitas ini."
          className="py-6"
        />
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
                stroke="var(--foreground)"
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
    </Seksi>
  );
}

function DaftarLot({ stok }: { stok: StokColdStorage[] }) {
  return (
    <Seksi label="Lot tersimpan" title={`Lot tersimpan (${stok.length})`}>
      {stok.length === 0 ? (
        <EmptyState title="Belum ada lot." className="py-6" />
      ) : (
        <ul className="divide-y divide-border rounded-lg border border-border">
          {stok.map((s) => {
            const urgensi = urgensiLot(s.sisaShelfLifeHari);
            return (
              <li
                key={s.id}
                className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 px-3 py-2"
              >
                <div className="min-w-0 flex-1 basis-40">
                  <p className="truncate font-medium">{s.komoditasNama}</p>
                  <p className="num truncate text-xs text-muted-foreground">
                    {s.kode ?? s.id} · {s.pemilik}
                  </p>
                </div>
                <div className="flex flex-wrap items-center justify-end gap-2">
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
    </Seksi>
  );
}

function Riwayat({ transaksi }: { transaksi: TransaksiStok[] }) {
  return (
    <Seksi label="Transaksi terbaru" title="Transaksi terbaru">
      {transaksi.length === 0 ? (
        <EmptyState title="Belum ada transaksi." className="py-6" />
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
                      ({t.fasilitasAsalKode ?? t.fasilitasAsalId} →{" "}
                      {t.fasilitasTujuanKode ?? t.fasilitasTujuanId})
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
    </Seksi>
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
  // Toast hasil/galat datang dari hook mutasi; galat juga tampil di form.
  const kiriman = useFormBff<
    TransaksiPayload,
    unknown,
    GalatForm,
    TransaksiPayload | undefined
  >({
    skema: skemaTransaksi,
    kirim: (p) => catat.mutateAsync(p),
    sukses: () => {
      setVolume("");
      setAlasan("");
      setPemilik("");
    },
  });

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

  return (
    <InspectorSection title="Catat transaksi stok">
      <FormBff
        aria-label="Catat transaksi"
        onKirim={kiriman.kirimDari(() => payload)}
        sibuk={kiriman.pending}
        className="space-y-3"
      >
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <Medan label="Jenis">
            <Select
              value={jenis}
              onValueChange={(v: string) => setJenis(v as Jenis)}
            >
              <SelectTrigger>
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
                  <SelectTrigger>
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
                  maxLength={255}
                />
              </Medan>
            </>
          ) : (
            <Medan label="Lot">
              <Select value={lotId} onValueChange={setLotId}>
                <SelectTrigger>
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
                <SelectTrigger>
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
              aria-invalid={volume !== "" && !volumeValid}
            />
          </Medan>

          {jenis === "KELUAR" && (
            <Medan label="Alasan">
              <Input
                value={alasan}
                onChange={(e) => setAlasan(e.target.value)}
                maxLength={500}
              />
            </Medan>
          )}
        </div>
        <FormAlert>{kiriman.galat?.pesan}</FormAlert>
        <SpringButton
          type="submit"
          variant="primary"
          disabled={!payload || kiriman.pending}
          className="w-full sm:w-auto"
        >
          {kiriman.pending && <Loader2 className="animate-spin" />} Catat
          transaksi
        </SpringButton>
      </FormBff>
    </InspectorSection>
  );
}

/** Isian belum lengkap (payload belum bisa disusun) ditolak sebelum menyentuh server. */
const skemaTransaksi = z.custom<TransaksiPayload>(
  (p) => p !== undefined,
  "Lengkapi isian transaksi terlebih dahulu.",
);

function Medan({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <label className="grid min-w-0 gap-1 text-xs text-muted-foreground">
      {label}
      {children}
    </label>
  );
}
