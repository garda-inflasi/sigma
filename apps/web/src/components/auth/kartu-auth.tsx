"use client";

import { ShieldCheck } from "lucide-react";
import { useRef, type ReactNode } from "react";
import { BrandMark } from "@/components/layout/brand-mark";
import { softReveal, useGSAP } from "@/lib/motion/gsap";

/** Kerangka halaman autentikasi: satu kartu, satu tujuan, dan catatan keamanan yang tidak menyaingi form. */
export function KartuAuth({
  judul,
  deskripsi,
  catatan,
  children,
}: {
  judul: string;
  deskripsi?: ReactNode;
  catatan?: ReactNode;
  children: ReactNode;
}) {
  const cardRef = useRef<HTMLDivElement>(null);

  useGSAP(
    () => {
      if (!cardRef.current) return;
      softReveal(cardRef.current, { y: 10, duration: 0.44, stagger: 0 });
    },
    { scope: cardRef },
  );

  return (
    <main className="grid min-h-dvh place-items-center px-4 py-4">
      <div
        ref={cardRef}
        className="glass-strong w-full max-w-[420px] rounded-xl p-6 sm:p-7"
      >
        <BrandMark className="size-10" />
        <h1 className="mt-4 text-display-md font-bold text-navy-900 dark:text-white">
          {judul}
        </h1>
        {deskripsi && (
          <p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">
            {deskripsi}
          </p>
        )}

        {children}

        {catatan && (
          <div className="mt-5 flex gap-2 border-t border-border pt-4 text-xs leading-relaxed text-muted-foreground">
            <ShieldCheck className="mt-px size-4 shrink-0" aria-hidden />
            <p>{catatan}</p>
          </div>
        )}
      </div>
    </main>
  );
}
