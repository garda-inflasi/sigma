"use client";

import { motion, useReducedMotion } from "motion/react";
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
function Penghubung({ aliran, tone = "data", indeks }: { aliran?: boolean; tone?: Tone; indeks: number }) {
  const reduce = useReducedMotion();
  return (
    <div className="relative flex shrink-0 items-center justify-center py-1 lg:w-12 lg:py-0" aria-hidden>
      <motion.span
        className="block h-6 w-0.5 rounded-full bg-gradient-to-b from-navy-300 to-data-400 lg:h-0.5 lg:w-full lg:bg-gradient-to-r dark:from-white/20"
        initial={{ clipPath: "inset(0 100% 100% 0)" }}
        whileInView={{ clipPath: "inset(0 0% 0% 0)" }}
        viewport={{ once: true }}
        transition={{ duration: 0.6, ease: ease.inOut, delay: 0.15 + indeks * 0.12 }}
      />
      {aliran && !reduce && (
        <motion.span
          className={cn("absolute top-1/2 left-0 hidden size-2 -translate-y-1/2 rounded-full shadow-glow-data lg:block", toneClass[tone].bg)}
          animate={{ left: ["0%", "100%"], opacity: [0, 1, 1, 0] }}
          transition={{ duration: 1.8, repeat: Infinity, ease: "easeInOut", delay: indeks * 0.35 }}
        />
      )}
    </div>
  );
}

function Node({ n }: { n: NodeAlur }) {
  const t = toneClass[n.tone ?? "data"];
  const isi = (
    <>
      <div className="flex items-center gap-2.5">
        <span className={cn("grid size-9 shrink-0 place-items-center rounded-xl", t.soft, t.text)}><n.icon className="size-[18px]" /></span>
        <p className="font-display text-[15px] leading-tight font-semibold">{n.judul}</p>
      </div>
      {n.deskripsi && <p className="mt-2 text-xs leading-relaxed text-muted-foreground">{n.deskripsi}</p>}
      {n.butir && (
        <ul className="mt-2.5 flex flex-wrap gap-1">
          {n.butir.map((b) => <li key={b} className="rounded-md bg-white/70 px-1.5 py-0.5 text-[11px] text-foreground/80 dark:bg-white/5">{b}</li>)}
        </ul>
      )}
      {(n.peran || n.metrik) && (
        <div className="mt-3 flex items-center justify-between gap-2 border-t border-border/60 pt-2.5 text-[11px]">
          {n.peran && <span className="text-muted-foreground">{n.peran}</span>}
          {n.metrik && <span className={cn("num font-semibold", t.text)}>{n.metrik}</span>}
        </div>
      )}
    </>
  );
  return (
    <motion.div variants={fadeUp} className="min-w-0 flex-1 lg:max-w-[260px]">
      {n.href ? (
        <Link href={n.href} className="glass block h-full rounded-xl p-4 transition-shadow hover:shadow-lift">{isi}</Link>
      ) : (
        <div className="glass h-full rounded-xl p-4">{isi}</div>
      )}
    </motion.div>
  );
}

/** Diagram alur horizontal (desktop) / vertikal (mobile) dengan reveal bertahap. */
export function DiagramAlur({ nodes, aliran, catatan }: { nodes: NodeAlur[]; aliran?: boolean; catatan?: ReactNode }) {
  return (
    <div>
      <motion.div variants={stagger(0.12)} initial="hidden" whileInView="show" viewport={{ once: true, margin: "-60px" }} className="flex flex-col items-stretch lg:flex-row lg:items-stretch">
        {nodes.map((n, i) => (
          <Fragment key={n.judul}>
            {i > 0 && <Penghubung aliran={aliran} tone={n.tone} indeks={i} />}
            <Node n={n} />
          </Fragment>
        ))}
      </motion.div>
      {catatan && <p className="mt-3 text-xs text-muted-foreground">{catatan}</p>}
    </div>
  );
}
