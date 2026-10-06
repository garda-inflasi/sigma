"use client";

import { animate, useInView, useMotionValue, useMotionValueEvent, useReducedMotion } from "motion/react";
import { useEffect, useRef } from "react";
import { countUp } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";

interface CountUpProps {
  value: number;
  format?: (n: number) => string;
  duration?: number;
  className?: string;
  /** Mulai saat masuk viewport (default) atau langsung */
  onView?: boolean;
}

/**
 * Angka KPI yang menghitung naik. Saat `value` berubah (refetch real-time),
 * animasi berlanjut dari nilai sebelumnya — perubahan terasa, bukan meloncat.
 */
export function CountUp({ value, format = (n) => Math.round(n).toLocaleString("id-ID"), duration = countUp.duration, className, onView = true }: CountUpProps) {
  const ref = useRef<HTMLSpanElement>(null);
  const inView = useInView(ref, { once: true, margin: "-40px" });
  const reduce = useReducedMotion();
  const mv = useMotionValue(0);

  useEffect(() => {
    if (onView && !inView) return;
    if (reduce) {
      mv.set(value);
      if (ref.current) ref.current.textContent = format(value);
      return;
    }
    const c = animate(mv, value, { duration, ease: countUp.ease });
    return () => c.stop();
    // format sengaja tidak masuk deps: fungsi inline akan memicu ulang animasi
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value, inView, reduce, duration, onView, mv]);

  useMotionValueEvent(mv, "change", (v) => {
    if (ref.current) ref.current.textContent = format(v);
  });

  return (
    <span ref={ref} className={cn("num", className)} aria-label={format(value)}>
      {format(0)}
    </span>
  );
}
