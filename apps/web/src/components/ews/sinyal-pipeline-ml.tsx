"use client";

import { ArrowUpRight } from "lucide-react";
import { PanelPas } from "@/components/garda/halaman-pas";
import { Shimmer } from "@/components/garda/shimmer";
import { STATUS_EWS } from "@/components/garda/status-badge";
import { KosongMl, PeringatanSiklusMl } from "@/components/ml/siklus-ml";
import type { BlokEwsMl, SinyalEwsMl } from "@/lib/api/ml";
import { severityMeta, toneClass } from "@/lib/design/tokens";
import { formatAngka } from "@/lib/format";
import {
  keadaanBlok,
  namaKomoditasMl,
  namaWilayahMl,
  waktuWib,
} from "@/lib/ml/label";
import {
  useLampiranMl,
  useMlAktif,
  useSiklusMlTerpilih,
} from "@/lib/query/ml-hooks";
import { cn } from "@/lib/utils";

const JENIS: Record<string, string> = {
  ANOMALI_HARGA: "Anomali harga",
  PROYEKSI_LONJAKAN: "Proyeksi lonjakan",
  TEKANAN_INFLASI: "Tekanan indeks pangan",
  SUMBER_DATA: "Sumber data",
};

/**
 * Sinyal EWS hasil pipeline ML untuk siklus pilihan, beserta jembatannya ke alert. Sinyal yang menjadi alert dibuka
 * lewat detail alert yang sama dengan tabel (alur akui/eskalasi/selesai dan aturan KRITIS tidak berubah); INFO tidak
 * pernah menjadi alert dan hanya dicatat di sini.
 */
export function PanelSinyalPipelineMl({
  onBukaAlert,
}: {
  onBukaAlert: (id: string) => void;
}) {
  const aktif = useMlAktif();
  const [cycleId] = useSiklusMlTerpilih();
  const lampiran = useLampiranMl("/v1/ews?limit=1&page=1", {
    cycleId,
    enabled: aktif,
  });
  const k = keadaanBlok(lampiran.data, lampiran.data?.ews, lampiran.isLoading);
  if (k.jenis === "sembunyi") return null;
  const blok = k.jenis === "tampil" ? k.blok : null;
  return (
    <PanelPas
      judul="Sinyal pipeline ML"
      deskripsi={
        blok?.cycleTimestamp
          ? `Siklus ${waktuWib(blok.cycleTimestamp, true)} · ${blok.ringkasan.aktif} aktif`
          : "Hasil training siklus pilihan"
      }
      tanpaPadding
      className="lg:min-h-40"
      aria-busy={lampiran.isFetching || undefined}
    >
      {k.jenis === "memuat" && <Shimmer className="m-3.5 h-32" />}
      {k.jenis === "siklus" && (
        <PeringatanSiklusMl pesan={k.pesan} className="m-3" />
      )}
      {k.jenis === "kosong" && <KosongMl pesan={k.pesan} className="m-3" />}
      {blok && <IsiSinyal blok={blok} onBukaAlert={onBukaAlert} />}
    </PanelPas>
  );
}

function IsiSinyal({
  blok,
  onBukaAlert,
}: {
  blok: BlokEwsMl;
  onBukaAlert: (id: string) => void;
}) {
  const r = blok.ringkasan;
  const hitung = [
    { s: "KRITIS" as const, n: r.kritis },
    { s: "SIAGA" as const, n: r.siaga },
    { s: "WASPADA" as const, n: r.waspada },
    { s: "INFO" as const, n: r.info },
  ];
  return (
    <div className={cn(blok.ringkasan.aktif === 0 && "pb-3")}>
      <ul
        aria-label="Jumlah sinyal aktif per tingkat"
        className="flex flex-wrap gap-x-3 gap-y-1 border-b border-border px-3.5 py-2 text-xs"
      >
        {hitung.map(({ s, n }) => (
          <li key={s} className="flex items-center gap-1.5">
            <span
              aria-hidden
              className={cn(
                "size-1.5 rounded-full",
                toneClass[severityMeta[s].tone].dot,
              )}
            />
            <span className="text-muted-foreground">
              {severityMeta[s].label}
            </span>
            <span className="num font-medium">{n}</span>
          </li>
        ))}
      </ul>
      {blok.sinyal.length === 0 ? (
        <p className="px-3.5 pt-3 text-xs text-muted-foreground">
          Tidak ada sinyal aktif pada siklus ini.
        </p>
      ) : (
        <ul className="divide-y divide-border">
          {blok.sinyal.map((s) => (
            <li key={s.sinyalKunci} className="px-3.5 py-2">
              <BarisSinyal s={s} onBukaAlert={onBukaAlert} />
            </li>
          ))}
        </ul>
      )}
      {blok.terpotong && (
        <p className="px-3.5 py-2 text-[11px] text-muted-foreground">
          Daftar dipotong; sinyal paling parah ditampilkan lebih dulu.
        </p>
      )}
    </div>
  );
}

function BarisSinyal({
  s,
  onBukaAlert,
}: {
  s: SinyalEwsMl;
  onBukaAlert: (id: string) => void;
}) {
  const m = severityMeta[s.severity];
  const lokasi = [
    s.komoditasKode ? namaKomoditasMl(s.komoditasKode) : null,
    s.kabupatenKode ? namaWilayahMl(s.kabupatenKode) : "Tingkat sistem",
  ]
    .filter(Boolean)
    .join(" · ");
  return (
    <div className="min-w-0 text-xs">
      <p className="flex items-center gap-1.5">
        <span
          aria-hidden
          className={cn(
            "size-1.5 shrink-0 rounded-full",
            toneClass[m.tone].dot,
          )}
        />
        <span className={cn("font-medium", toneClass[m.tone].text)}>
          {m.label}
        </span>
        <span className="truncate text-muted-foreground">
          {JENIS[s.jenis] ?? s.jenis}
        </span>
        {s.nilai !== null && s.jenis === "ANOMALI_HARGA" && (
          <span className="num ml-auto shrink-0 font-medium">
            {s.nilai > 0 ? "+" : "−"}
            {formatAngka(Math.abs(s.nilai), 1)}%
          </span>
        )}
      </p>
      <p
        title={s.pesan}
        className="mt-0.5 truncate text-[13px] text-foreground"
      >
        {s.judul}
      </p>
      <p className="truncate text-muted-foreground">{lokasi}</p>
      <p className="mt-1">
        {s.alert ? (
          <button
            type="button"
            onClick={() => onBukaAlert(s.alert!.id)}
            className="inline-flex items-center gap-1 font-medium text-data-700 hover:underline dark:text-data-300"
          >
            Menjadi alert {s.alert.kode ?? `#${s.alert.id}`} ·{" "}
            {STATUS_EWS[s.alert.status]?.label ?? s.alert.status}
            <ArrowUpRight className="size-3" />
          </button>
        ) : s.menjadiAlert ? (
          <span className="text-waspada-ink dark:text-amber-300">
            Akan menjadi alert saat jembatan EWS berjalan
          </span>
        ) : (
          <span className="text-muted-foreground">
            Info saja, tidak menjadi alert
          </span>
        )}
      </p>
    </div>
  );
}
