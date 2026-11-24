"use client";

import { FileText } from "lucide-react";
import { motion } from "motion/react";
import Link from "next/link";
import { PanelTitle } from "@/components/garda/page-header";
import {
  STATUS_BRIEF,
  STATUS_EWS,
  StatusBadge,
} from "@/components/garda/status-badge";
import { SpringLink } from "@/components/garda/spring-button";
import { CountUp } from "@/components/motion/count-up";
import { useCan } from "@/lib/auth/session-context";
import type { RingkasanEksekutif } from "@/lib/data/types";
import { SIKLUS_URUTAN } from "@/lib/data/siklus";
import {
  severityMeta,
  siklusMeta,
  toneClass,
  type Severity,
} from "@/lib/design/tokens";
import { formatRelatif } from "@/lib/format";
import { ease } from "@/lib/motion/presets";
import { useBriefs } from "@/lib/query/hooks";
import { cn } from "@/lib/utils";

export function EwsSummary({ data }: { data: RingkasanEksekutif["ews"] }) {
  const urut: Severity[] = ["KRITIS", "SIAGA", "WASPADA", "INFO"];
  return (
    <section className="glass shrink-0 rounded-lg p-3">
      <PanelTitle
        title="Peringatan aktif"
        actions={
          <Link
            href="/ews"
            className="text-sm text-data-700 hover:underline dark:text-data-300"
          >
            Buka
          </Link>
        }
      />
      <div className="mt-2 grid grid-cols-2 gap-1.5">
        {urut.map((s) => {
          const t = toneClass[severityMeta[s].tone];
          return (
            <Link
              key={s}
              href={`/ews?severity=${s}`}
              className={cn(
                "min-w-0 rounded-md px-2 py-1 transition-shadow hover:shadow-ambient-sm",
                t.soft,
              )}
            >
              <CountUp
                value={data[s]}
                className={cn(
                  "block text-base leading-8 font-semibold",
                  t.text,
                )}
              />
              <span className={cn("block truncate text-[14px]", t.text)}>
                {severityMeta[s].label}
              </span>
            </Link>
          );
        })}
      </div>
    </section>
  );
}

export function InterventionSummary({
  data,
  dominan,
}: {
  data: RingkasanEksekutif["intervensi"];
  dominan: RingkasanEksekutif["siklusDominan"];
}) {
  return (
    <section className="glass shrink-0 rounded-lg p-3">
      <PanelTitle title="Status intervensi" />
      <ul className="mt-1.5 flex flex-col gap-0.5">
        {SIKLUS_URUTAN.map((s) => {
          const m = siklusMeta[s];
          const t = toneClass[m.tone];
          return (
            <li key={s}>
              <Link
                href={`/intervensi?siklus=${s}`}
                title={m.deskripsi}
                className={cn(
                  "flex items-center gap-2 rounded-md px-2 py-1 transition-colors hover:bg-secondary",
                  s === dominan && "ring-1 ring-inset",
                  s === dominan && t.ring,
                )}
              >
                <span className={cn("h-4 w-1 shrink-0 rounded-full", t.bg)} />
                <span className="min-w-0 flex-1 truncate text-[13px] font-medium">
                  {m.label}
                </span>
                <span className="shrink-0 text-[11px] text-muted-foreground">
                  <span className="num text-sm font-semibold text-foreground">
                    {data[s].aktif}
                  </span>{" "}
                  aktif, {data[s].selesai} selesai
                </span>
              </Link>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

/** Andil komoditas terhadap inflasi: batang ke kanan mendorong naik, ke kiri menahan. */
export function PenyumbangUtama({
  data,
  className,
}: {
  data: NonNullable<RingkasanEksekutif["inflasi"]>["penyumbang"];
  className?: string;
}) {
  const maksAndil = Math.max(...data.map((p) => Math.abs(p.andil))) || 1;
  return (
    <section
      className={cn("glass flex shrink-0 flex-col rounded-lg p-3", className)}
    >
      <PanelTitle title="Penyumbang utama (poin persen)" />
      {data.length === 0 ? (
        <p className="mt-1.5 text-md text-muted-foreground">
          Andil komoditas untuk bulan ini belum diimpor.
        </p>
      ) : (
        <ul className="mt-2 flex flex-col gap-2">
          {data.map((p, i) => (
            <li
              key={p.nama}
              className="grid grid-cols-[minmax(0,1fr)_120px_40px] items-center gap-3 text-md leading-[18px] sm:grid-cols-[minmax(0,1fr)_144px_40px]"
            >
              <span title={p.nama} className="truncate">
                {p.nama}
              </span>
              <div className="relative h-2">
                <div className="absolute inset-y-0 left-1/2 w-px bg-border" />
                <motion.div
                  className={cn(
                    "absolute inset-y-0 rounded-full",
                    p.andil >= 0
                      ? "left-1/2 bg-defisit"
                      : "right-1/2 bg-data-500",
                  )}
                  initial={{ width: 0 }}
                  animate={{
                    width: `${(Math.abs(p.andil) / maksAndil) * 50}%`,
                  }}
                  transition={{
                    duration: 0.8,
                    ease: ease.garda,
                    delay: 0.4 + i * 0.06,
                  }}
                />
              </div>
              <span className="num text-right text-md font-medium">
                {p.andil > 0 ? "+" : ""}
                {p.andil.toFixed(2).replace(".", ",")}
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

export function LatestBrief() {
  const { data } = useBriefs();
  const bisaSusun = useCan("brief.susun");
  const b = data?.[0];
  if (!b) return null;
  const st = STATUS_BRIEF[b.status];
  return (
    <section className="glass shrink-0 rounded-lg p-3.5">
      <div className="flex flex-col gap-2.5 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 flex-1 items-center gap-2.5">
          <span className="grid size-8 shrink-0 place-items-center rounded-md bg-navy-50 text-navy-700 dark:bg-white/10 dark:text-navy-200">
            <FileText className="size-4" />
          </span>
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <p className="text-xs text-muted-foreground">
                Policy brief {b.periode}
              </p>
              <StatusBadge tone={st.tone}>{st.label}</StatusBadge>
            </div>
            <p
              title={b.judul}
              className="truncate font-display text-[13px] leading-snug font-semibold"
            >
              {b.judul}
            </p>
          </div>
        </div>
        <div className="flex shrink-0 items-center justify-between gap-3 sm:justify-end">
          <p
            title={`${b.jumlahRekomendasi} rekomendasi, diperbarui ${formatRelatif(b.diperbarui)}`}
            className="text-xs text-muted-foreground"
          >
            {b.jumlahRekomendasi} rekomendasi · {formatRelatif(b.diperbarui)}
          </p>
          <SpringLink
            href={`/policy-brief?id=${b.id}`}
            size="sm"
            variant={bisaSusun ? "primary" : "soft"}
            className="shrink-0"
            aria-label={
              bisaSusun
                ? "Lanjutkan penyusunan policy brief"
                : "Telaah draf policy brief"
            }
          >
            {bisaSusun ? "Lanjutkan" : "Telaah draf"}
          </SpringLink>
        </div>
      </div>
    </section>
  );
}

export { STATUS_EWS };
