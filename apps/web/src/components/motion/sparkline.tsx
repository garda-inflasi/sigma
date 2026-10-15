"use client";

import { motion } from "motion/react";
import { useId, useMemo } from "react";
import { drawPath } from "@/lib/motion/presets";

/** Sparkline SVG ringan dengan animasi "menggambar" garis. */
export function Sparkline({ data, color = "currentColor", width = 120, height = 36, fill = true }: { data: number[]; color?: string; width?: number; height?: number; fill?: boolean }) {
  const id = useId();
  const { line, area } = useMemo(() => {
    if (data.length < 2) return { line: "", area: "" };
    const min = Math.min(...data);
    const max = Math.max(...data);
    const span = max - min || 1;
    const pts = data.map((v, i) => [(i / (data.length - 1)) * width, height - 3 - ((v - min) / span) * (height - 6)] as const);
    const d = pts.map(([x, y], i) => `${i ? "L" : "M"}${x.toFixed(1)},${y.toFixed(1)}`).join(" ");
    return { line: d, area: `${d} L${width},${height} L0,${height} Z` };
  }, [data, width, height]);

  return (
    <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} className="overflow-visible" aria-hidden>
      <defs>
        <linearGradient id={id} x1="0" x2="0" y1="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity={0.22} />
          <stop offset="100%" stopColor={color} stopOpacity={0} />
        </linearGradient>
      </defs>
      {fill && <motion.path d={area} fill={`url(#${id})`} initial={{ opacity: 0 }} animate={{ opacity: 1 }} transition={{ delay: 0.6, duration: 0.6 }} />}
      <motion.path d={line} fill="none" stroke={color} strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" variants={drawPath} initial="hidden" animate="show" />
    </svg>
  );
}
