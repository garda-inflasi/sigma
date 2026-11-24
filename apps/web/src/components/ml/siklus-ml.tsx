"use client";

import { Clock3, History, RotateCcw } from "lucide-react";
import { useMemo, type ReactNode } from "react";
import { Istilah } from "@/components/garda/istilah";
import { StatusBadge } from "@/components/garda/status-badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
} from "@/components/ui/select";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  siklusFinalTerbaru,
  type SiklusMl,
  type SiklusRingkasMl,
} from "@/lib/api/ml";
import { toneClass } from "@/lib/design/tokens";
import { GLOSARIUM } from "@/lib/glosarium";
import {
  labelTargetMl,
  ringkasModelSiklus,
  statusSiklus,
  waktuWib,
} from "@/lib/ml/label";
import {
  useDaftarSiklusMl,
  useMlAktif,
  useSiklusMlTerpilih,
} from "@/lib/query/ml-hooks";
import { cn } from "@/lib/utils";

/**
 * Pemilih siklus training (`GET /v1/ml/cycles`). Bawaannya siklus final terbaru; pilihan disimpan di URL
 * (`?cycle_id=`) sehingga ikut dikirim ke endpoint halaman. Siklus BERJALAN dan GAGAL tampil sebagai metadata dan
 * tidak bisa dipilih. Tidak tampil sama sekali tanpa izin `prediksi.lihat`, di mode Staging, atau bila penyimpanan
 * hasil training tidak tersedia.
 */
export function PemilihSiklusMl({ className }: { className?: string }) {
  const aktif = useMlAktif();
  const { data } = useDaftarSiklusMl();
  const [cycleId, setCycleId] = useSiklusMlTerpilih();
  const terbaru = siklusFinalTerbaru(data);

  const items = useMemo(() => {
    const daftar = data?.items ?? [];
    // Siklus dari URL yang lebih lama dari 24 siklus terakhir tetap bisa ditampilkan sebagai pilihan aktif.
    if (cycleId && !daftar.some((s) => s.cycleId === cycleId)) {
      return [...daftar, siklusLuarDaftar(cycleId)];
    }
    return daftar;
  }, [data, cycleId]);

  if (!aktif || !data?.tersedia || !terbaru) return null;
  const nilai = cycleId ?? terbaru.cycleId;
  const dipilih = items.find((s) => s.cycleId === nilai);

  return (
    <Select
      value={nilai}
      onValueChange={(id) =>
        setCycleId(id === terbaru.cycleId ? undefined : id)
      }
    >
      <SelectTrigger
        aria-label="Pilih siklus training"
        title={dipilih ? `Siklus ${dipilih.cycleId}` : undefined}
        className={cn(
          "h-9 w-full min-w-0 gap-1.5 text-[13px] sm:w-auto sm:min-w-56 md:text-[13px] lg:min-w-0 min-[88rem]:min-w-56",
          className,
        )}
      >
        <Clock3 className="size-3.5 shrink-0 text-muted-foreground" />
        <span className="min-w-0 truncate">
          <span className="text-muted-foreground lg:max-[88rem]:hidden">Siklus </span>
          <span className="num">
            {dipilih ? waktuWib(dipilih.cycleTimestamp, true) : nilai}
          </span>
          {nilai === terbaru.cycleId && (
            <span className="text-muted-foreground lg:max-[88rem]:hidden"> · terbaru</span>
          )}
        </span>
      </SelectTrigger>
      <SelectContent className="max-h-80">
        {items.map((s) => (
          <SelectItem
            key={s.cycleId}
            value={s.cycleId}
            disabled={!s.final}
            textValue={`${waktuWib(s.cycleTimestamp)} ${statusSiklus(s.status).label}`}
            className="py-1.5"
          >
            <OpsiSiklus siklus={s} terbaru={s.cycleId === terbaru.cycleId} />
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

function siklusLuarDaftar(cycleId: string): SiklusMl {
  const m = /^C(\d{4})(\d{2})(\d{2})T(\d{2})Z$/.exec(cycleId);
  return {
    cycleId,
    cycleTimestamp: m ? `${m[1]}-${m[2]}-${m[3]}T${m[4]}:00:00Z` : "",
    dataCutoff: null,
    status: "BERHASIL",
    final: true,
    durationMs: null,
    pesan: "",
    versiKode: "",
    jumlah: {},
    targetInflasi: null,
    labelTarget: null,
    klasifikasi: null,
    regresi: null,
  };
}

function OpsiSiklus({
  siklus: s,
  terbaru,
}: {
  siklus: SiklusMl;
  terbaru: boolean;
}) {
  const st = statusSiklus(s.status);
  return (
    <span className="flex min-w-0 flex-col gap-0.5">
      <span className="flex items-center gap-1.5">
        <span className="num text-[13px]">{waktuWib(s.cycleTimestamp)}</span>
        <StatusBadge tone={st.tone} className="h-[18px] px-1.5 text-[10px]">
          {st.label}
        </StatusBadge>
        {terbaru && (
          <span className="text-[11px] text-muted-foreground">terbaru</span>
        )}
      </span>
      <span className="truncate text-[11px] text-muted-foreground">
        {ringkasModelSiklus(s)}
      </span>
    </span>
  );
}

/**
 * Siklus yang sedang dipakai halaman, ringkas. Halaman tanpa pemilih menampilkan ini saja. Tanpa `siklus`, badge
 * memakai siklus final terbaru dari daftar (butuh `prediksi.lihat`). Tidak tampil bila hasil training tidak tersedia.
 */
export function BadgeSiklusMl({
  siklus,
  className,
}: {
  siklus?: SiklusRingkasMl | null;
  className?: string;
}) {
  const aktif = useMlAktif();
  const { data } = useDaftarSiklusMl(!siklus);
  const [cycleId] = useSiklusMlTerpilih();
  if (!aktif) return null;
  const cadangan = siklusFinalTerbaru(data);
  const pakai:
    | Pick<
        SiklusRingkasMl,
        "cycleId" | "cycleTimestamp" | "status" | "targetInflasi"
      >
    | undefined =
    siklus ??
    (cadangan && (!cycleId || cycleId === cadangan.cycleId)
      ? cadangan
      : undefined);
  if (!pakai) return null;
  const st = statusSiklus(pakai.status);
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          tabIndex={0}
          data-testid="badge-siklus-ml"
          className={cn(
            "inline-flex h-7 max-w-full shrink-0 items-center gap-1.5 rounded-md border border-border bg-card px-2 text-xs whitespace-nowrap text-muted-foreground",
            className,
          )}
        >
          <History className="size-3.5 shrink-0" aria-hidden />
          <span className="truncate">
            Siklus training{" "}
            <span className="num text-foreground">
              {waktuWib(pakai.cycleTimestamp, true)}
            </span>
          </span>
          <span
            aria-hidden
            className={cn(
              "size-1.5 shrink-0 rounded-full",
              toneClass[st.tone].dot,
            )}
          />
          <span className="sr-only">, status {st.label}</span>
        </span>
      </TooltipTrigger>
      <TooltipContent className="max-w-72 text-xs leading-relaxed">
        <p className="font-medium">
          {pakai.cycleId} · {st.label}
        </p>
        <p>{waktuWib(pakai.cycleTimestamp)}</p>
        <p>Target: {labelTargetMl(pakai.targetInflasi)}</p>
        <p className="mt-1 text-muted-foreground">{GLOSARIUM.siklusTraining}</p>
      </TooltipContent>
    </Tooltip>
  );
}

/** Pemberitahuan bila `cycle_id` di URL tidak dapat dipakai, dengan pilihan kembali ke siklus terbaru. */
export function PeringatanSiklusMl({
  pesan,
  className,
}: {
  pesan: string;
  className?: string;
}) {
  const [cycleId, setCycleId] = useSiklusMlTerpilih();
  return (
    <div
      role="status"
      className={cn(
        "flex flex-wrap items-center gap-x-3 gap-y-1 rounded-md border border-dashed border-waspada/50 bg-waspada/10 px-3 py-1.5 text-xs text-waspada-ink dark:text-amber-300",
        className,
      )}
    >
      <span className="min-w-0 flex-1">
        Siklus {cycleId ?? "pilihan"} tidak dapat dipakai: {pesan}
      </span>
      {cycleId && (
        <button
          type="button"
          onClick={() => setCycleId(undefined)}
          className="inline-flex items-center gap-1 font-medium underline-offset-2 hover:underline"
        >
          <RotateCcw className="size-3" /> Kembali ke siklus terbaru
        </button>
      )}
    </div>
  );
}

/** Nama target sinyal inflasi yang jujur, bertooltip glosarium. */
export function LabelTargetMl({
  target,
  className,
}: {
  target: string | null | undefined;
  className?: string;
}) {
  return (
    <Istilah
      kunci={target === "RESMI_BPS" ? "inflasi" : "indeksPangan"}
      className={className}
    >
      {labelTargetMl(target)}
    </Istilah>
  );
}

/** Kotak "Model belum dapat diandalkan" beserta alasannya (tanpa kelas, skor, atau siklus). */
export function CatatanBelumAndal({
  judul,
  alasan,
  children,
  className,
}: {
  judul: string;
  alasan: string;
  children?: ReactNode;
  className?: string;
}) {
  return (
    <div
      role="note"
      className={cn(
        "rounded-md border border-dashed border-border bg-secondary/60 px-3 py-2 text-xs",
        className,
      )}
    >
      <p className="font-medium text-foreground">
        <Istilah kunci="kelayakanModel">{judul}</Istilah>
      </p>
      <p className="mt-0.5 leading-snug text-muted-foreground">{alasan}</p>
      {children}
    </div>
  );
}

/** Keadaan kosong ringkas untuk blok yang siklusnya tidak punya data. Tidak pernah berisi angka. */
export function KosongMl({
  pesan,
  className,
}: {
  pesan: string;
  className?: string;
}) {
  return (
    <p
      role="status"
      className={cn(
        "rounded-md bg-secondary/60 px-3 py-2 text-xs text-muted-foreground",
        className,
      )}
    >
      {pesan}
    </p>
  );
}
