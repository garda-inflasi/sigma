"use client";

import { useRef, type ComponentProps, type ReactNode } from "react";
import { GSAP_SOFT, softReveal, useGSAP } from "@/lib/motion/gsap";
import { cn } from "@/lib/utils";

/** Masuk halaman memakai GSAP (`power3.out`, geser mikro 8px, 360ms). Hormati reduced-motion otomatis. */
export function PageEnter({
  children,
  className,
  ...props
}: { children: ReactNode } & ComponentProps<"div">) {
  const ref = useRef<HTMLDivElement>(null);

  useGSAP(
    () => {
      if (!ref.current) return;
      softReveal(ref.current, { y: 8, duration: 0.36, stagger: 0 });
    },
    { scope: ref },
  );

  return (
    <div ref={ref} className={className} {...props}>
      {children}
    </div>
  );
}

/** Wadah stagger lembut GSAP (40ms) untuk deretan kartu KPI atau panel tanpa bentrok dengan Motion hover/tap. */
export function Stagger({ children, className }: { children: ReactNode; className?: string }) {
  const ref = useRef<HTMLDivElement>(null);

  useGSAP(
    () => {
      const items = ref.current?.querySelectorAll("[data-stagger-item]");
      if (!items?.length) return;
      softReveal(items, {
        y: GSAP_SOFT.distance,
        duration: GSAP_SOFT.duration,
        stagger: GSAP_SOFT.stagger,
      });
    },
    { scope: ref },
  );

  return (
    <div ref={ref} className={className}>
      {children}
    </div>
  );
}

export function StaggerItem({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div data-stagger-item className={cn("min-w-0", className)}>
      {children}
    </div>
  );
}
