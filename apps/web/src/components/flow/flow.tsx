"use client";

import { motion } from "motion/react";
import type { LucideIcon } from "lucide-react";
import Link from "next/link";
import { Fragment, type ReactNode } from "react";
import { toneClass, type Tone } from "@/lib/design/tokens";
import { ease, fadeUp, stagger } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";

export interface NodeAlur {
  judul: string;
  deskripsi?: string;
  icon: LucideIcon;
  tone?: Tone;
  butir?: string[];
  peran?: string;
  metrik?: string;
  href?: string;
}

/** Penghubung antar-node: garis tergambar + paket data mengalir (hanya bila `aliran`). */
function Penghubung({
  aliran,
  tone = "data",
  indeks,
  mengisi,
}: {
  aliran?: boolean;
  tone?: Tone;
  indeks: number;
  mengisi?: boolean;
}) {
  const tampil = { clipPath: "inset(0 0% 0% 0)" };
  return (
    <div
      className={cn(
        "relative flex shrink-0 items-center justify-center py-1 xl:py-0",
        mengisi ? "xl:w-5 2xl:w-9" : "xl:w-12",
      )}
      aria-hidden
    >
      <motion.span
        className={cn(
          "block w-0.5 rounded-full bg-gradient-to-b from-navy-300 to-data-400 xl:h-0.5 xl:w-full xl:bg-gradient-to-r dark:from-white/20",
          mengisi ? "h-5" : "h-6",
        )}
        initial={{ clipPath: "inset(0 100% 100% 0)" }}
        {...(mengisi
          ? { animate: tampil }
          : { whileInView: tampil, viewport: { once: true } })}
        transition={{
          duration: 0.6,
          ease: ease.inOut,
          delay: 0.15 + indeks * 0.12,
        }}
      />
      {aliran && (
        <motion.span
          className={cn(
            "absolute top-1/2 left-0 hidden size-2 -translate-y-1/2 rounded-full shadow-glow-data xl:block motion-reduce:hidden!",
            toneClass[tone].bg,
          )}
          animate={{ left: ["0%", "100%"], opacity: [0, 1, 1, 0] }}
          transition={{
            duration: 1.8,
            repeat: Infinity,
            ease: "easeInOut",
            delay: indeks * 0.35,
          }}
        />
      )}
    </div>
  );
}

function Node({ n, mengisi }: { n: NodeAlur; mengisi?: boolean }) {
  const t = toneClass[n.tone ?? "data"];
  const isi = (
    <>
      <div
        className={cn(
          "flex items-center gap-2.5",
          mengisi && "xl:flex-col xl:items-start xl:gap-2",
        )}
      >
        <span
          className={cn(
            "grid shrink-0 place-items-center",
            mengisi ? "size-8 rounded-lg xl:size-10" : "size-9 rounded-xl",
            t.soft,
            t.text,
          )}
        >
          <n.icon className={mengisi ? "size-4 xl:size-5" : "size-[18px]"} />
        </span>
        <p
          className={cn(
            "font-display leading-tight font-semibold",
            mengisi ? "text-sm xl:text-[15px]" : "text-[15px]",
          )}
        >
          {n.judul}
        </p>
      </div>
      {n.deskripsi && (
        <p
          className={cn(
            "mt-2 text-xs leading-relaxed text-muted-foreground",
            mengisi && "xl:text-[13px]",
          )}
        >
          {n.deskripsi}
        </p>
      )}
      {n.butir && (
        <ul
          className={cn(
            "flex flex-wrap gap-1",
            mengisi ? "mt-2 xl:mt-3" : "mt-2.5",
          )}
        >
          {n.butir.map((b) => (
            <li
              key={b}
              className={cn(
                "rounded-md bg-secondary px-1.5 py-0.5 text-[11px] text-foreground/80",
                mengisi && "xl:text-xs",
              )}
            >
              {b}
            </li>
          ))}
        </ul>
      )}
      {(n.peran || n.metrik) && (
        <div
          className={cn(
            "flex items-center justify-between gap-2 text-[11px]",
            mengisi
              ? "mt-auto flex-wrap gap-y-0.5 pt-3 xl:text-xs"
              : "mt-3 border-t border-border pt-2.5",
          )}
        >
          {n.peran && <span className="text-muted-foreground">{n.peran}</span>}
          {n.metrik && (
            <span className={cn("num font-semibold", t.text)}>{n.metrik}</span>
          )}
        </div>
      )}
    </>
  );
  const kelas = mengisi
    ? "glass flex h-full flex-col rounded-lg p-3 xl:p-4"
    : "glass h-full rounded-xl p-4";
  return (
    <motion.div
      variants={fadeUp}
      className={cn(
        "min-w-0 flex-1",
        mengisi ? "xl:max-w-[300px]" : "xl:max-w-[260px]",
      )}
    >
      {n.href ? (
        <Link
          href={n.href}
          className={cn(
            kelas,
            !mengisi && "block",
            "transition-shadow hover:shadow-lift",
          )}
        >
          {isi}
        </Link>
      ) : (
        <div className={kelas}>{isi}</div>
      )}
    </motion.div>
  );
}

/**
 * Diagram alur horizontal (desktop) / vertikal (mobile) dengan reveal bertahap. `mengisi` untuk diagram yang menjadi
 * isi utama satu panel: kartu lebih besar dan langsung tampil karena panelnya menggulir sendiri.
 */
export function DiagramAlur({
  nodes,
  aliran,
  catatan,
  mengisi,
  className,
}: {
  nodes: NodeAlur[];
  aliran?: boolean;
  catatan?: ReactNode;
  mengisi?: boolean;
  className?: string;
}) {
  return (
    <div className={className}>
      <motion.div
        variants={stagger(mengisi ? 0.08 : 0.12)}
        initial="hidden"
        {...(mengisi
          ? { animate: "show" }
          : {
              whileInView: "show",
              viewport: { once: true, margin: "-60px" },
            })}
        className={cn(
          "flex flex-col items-stretch xl:flex-row xl:items-stretch",
          mengisi && "xl:justify-center",
        )}
      >
        {nodes.map((n, i) => (
          <Fragment key={n.judul}>
            {i > 0 && (
              <Penghubung
                aliran={aliran}
                tone={n.tone}
                indeks={i}
                mengisi={mengisi}
              />
            )}
            <Node n={n} mengisi={mengisi} />
          </Fragment>
        ))}
      </motion.div>
      {catatan && (
        <p
          className={cn(
            "mt-3 text-xs text-muted-foreground",
            mengisi && "shrink-0 text-center",
          )}
        >
          {catatan}
        </p>
      )}
    </div>
  );
}
