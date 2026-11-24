"use client";

import Lenis from "lenis";
import { usePathname } from "next/navigation";
import { useEffect, useRef } from "react";
import { apakahGerakDikurangi, gsap, pastikanGsap, ScrollTrigger } from "@/lib/motion/gsap";

/**
 * Pengelola smooth scrolling global memakai Lenis yang disinkronkan ke ticker GSAP & ScrollTrigger.
 * - `allowNestedScroll: true` + `prevent` menjaga panel scroll internal, modal, sheet, select, dan peta Leaflet tetap responsif.
 * - Menghormati `prefers-reduced-motion` secara dinamis.
 * - Reset posisi scroll dokumen secara halus saat rute berganti.
 */
export function SmoothScrollProvider() {
  const pathname = usePathname();
  const lenisRef = useRef<Lenis | null>(null);

  useEffect(() => {
    if (!pastikanGsap() || typeof window.matchMedia !== "function") return;

    const media = window.matchMedia("(prefers-reduced-motion: reduce)");
    let aktif = false;
    let onTick: ((time: number) => void) | null = null;
    let onScroll: (() => void) | null = null;

    function pasang() {
      if (aktif || media.matches || apakahGerakDikurangi()) return;
      try {
        const lenis = new Lenis({
          lerp: 0.1,
          smoothWheel: true,
          wheelMultiplier: 0.95,
          touchMultiplier: 1.0,
          anchors: true,
          allowNestedScroll: true,
          stopInertiaOnNavigate: true,
          respectReducedMotion: true,
          prevent: (node) => Boolean(node.closest?.("[data-lenis-prevent], .leaflet-container")),
        });
        lenisRef.current = lenis;
        aktif = true;

        onScroll = () => ScrollTrigger.update();
        lenis.on("scroll", onScroll);

        onTick = (time: number) => {
          lenis.raf(time * 1000);
        };
        gsap.ticker.add(onTick);
        gsap.ticker.lagSmoothing(0);
      } catch {
        // Fallback diam ke scroll native bila lingkungan DOM minimal (mis. test) tidak mendukung Lenis penuh.
      }
    }

    function lepas() {
      if (!aktif) return;
      aktif = false;
      if (onTick) {
        gsap.ticker.remove(onTick);
        onTick = null;
      }
      if (lenisRef.current) {
        if (onScroll) {
          lenisRef.current.off("scroll", onScroll);
          onScroll = null;
        }
        lenisRef.current.destroy();
        lenisRef.current = null;
      }
    }

    pasang();
    const onChange = () => {
      if (media.matches) lepas();
      else pasang();
    };
    media.addEventListener?.("change", onChange);

    return () => {
      media.removeEventListener?.("change", onChange);
      lepas();
    };
  }, []);

  useEffect(() => {
    if (lenisRef.current) {
      lenisRef.current.scrollTo(0, { immediate: true });
      ScrollTrigger.refresh();
    }
  }, [pathname]);

  return null;
}
