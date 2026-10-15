"use client";

import { FileText } from "lucide-react";
import Link from "next/link";
import { PanelTitle } from "@/components/garda/page-header";
import { STATUS_EWS, StatusBadge } from "@/components/garda/status-badge";
import { SpringLink } from "@/components/garda/spring-button";
import { CountUp } from "@/components/motion/count-up";
import { useCan } from "@/lib/auth/session-context";
import type { RingkasanEksekutif } from "@/lib/data/types";
import { SIKLUS_URUTAN } from "@/lib/data/siklus";
import { severityMeta, siklusMeta, toneClass, type Severity } from "@/lib/design/tokens";
import { formatRelatif } from "@/lib/format";
import { useBriefs } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";

export function EwsSummary({ data }: { data: RingkasanEksekutif["ews"] }) {
  const urut: Severity[] = ["KRITIS", "SIAGA", "WASPADA", "INFO"];
  return (
    <section className="glass rounded-xl p-5">
      <PanelTitle title="Peringatan aktif" actions={<Link href="/ews" className="text-sm text-data-700 hover:underline dark:text-data-300">Buka</Link>} />
      <div className="mt-4 grid grid-cols-4 gap-2">
        {urut.map((s) => {
          const t = toneClass[severityMeta[s].tone];
          return (
            <Link key={s} href={`/ews?severity=${s}`} className={cn("rounded-xl p-3 transition-shadow hover:shadow-ambient-sm", t.soft)}>
              <CountUp value={data[s]} className={cn("block text-2xl font-semibold", t.text)} />
              <span className={cn("text-xs", t.text)}>{severityMeta[s].label}</span>
            </Link>
          );
        })}
      </div>
    </section>
  );
}

export function InterventionSummary({ data, dominan }: { data: RingkasanEksekutif["intervensi"]; dominan: RingkasanEksekutif["siklusDominan"] }) {
  return (
    <section className="glass rounded-xl p-5">
      <PanelTitle title="Status intervensi" description="Per siklus ekonomi" />
      <ul className="mt-4 space-y-2">
        {SIKLUS_URUTAN.map((s) => {
          const m = siklusMeta[s];
          const t = toneClass[m.tone];
          return (
            <li key={s}>
              <Link href={`/intervensi?siklus=${s}`} className={cn("flex items-center gap-3 rounded-xl p-3 transition-colors hover:bg-white/60 dark:hover:bg-white/5", s === dominan && "ring-1 ring-inset", s === dominan && t.ring)}>
                <span className={cn("h-9 w-1.5 rounded-full", t.bg)} />
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-medium">{m.label}</p>
                  <p className="truncate text-xs text-muted-foreground">{m.deskripsi}</p>
                </div>
                <div className="text-right">
                  <p className="num text-lg leading-none font-semibold">{data[s].aktif}</p>
                  <p className="text-[11px] text-muted-foreground">aktif, {data[s].selesai} selesai</p>
                </div>
              </Link>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

export function LatestBrief() {
  const { data } = useBriefs();
  const bisaSusun = useCan("brief.susun");
  const b = data?.[0];
  if (!b) return null;
  const st = { DRAF: { tone: "netral", label: "Draf" }, TELAAH: { tone: "waspada", label: "Ditelaah" }, DISETUJUI: { tone: "normal", label: "Disetujui" }, DIDISTRIBUSIKAN: { tone: "data", label: "Didistribusikan" } } as const;
  return (
    <section className="glass rounded-xl p-5">
      <div className="flex items-center justify-between">
        <span className="grid size-10 place-items-center rounded-xl bg-navy-50 text-navy-700 dark:bg-white/10 dark:text-navy-200"><FileText className="size-5" /></span>
        <StatusBadge tone={st[b.status].tone}>{st[b.status].label}</StatusBadge>
      </div>
      <p className="mt-4 text-xs text-muted-foreground">Policy brief {b.periode}</p>
      <p className="mt-1 font-display leading-snug font-semibold">{b.judul}</p>
      <p className="mt-2 text-xs text-muted-foreground">{b.jumlahRekomendasi} rekomendasi, diperbarui {formatRelatif(b.diperbarui)}</p>
      <SpringLink href={`/policy-brief?id=${b.id}`} size="sm" variant={bisaSusun ? "primary" : "soft"} className="mt-4 w-full">
        {bisaSusun ? "Lanjutkan penyusunan" : "Telaah draf"}
      </SpringLink>
    </section>
  );
}

export { STATUS_EWS };
