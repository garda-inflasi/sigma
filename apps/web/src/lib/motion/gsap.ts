"use client";

import { useGSAP } from "@gsap/react";
import gsap from "gsap";
import { ScrollTrigger } from "gsap/ScrollTrigger";

let terdaftar = false;

/** Daftarkan plugin GSAP sekali di klien secara aman (idempoten, aman terhadap SSR). */
export function pastikanGsap(): boolean {
  if (typeof window === "undefined") return false;
  if (!terdaftar) {
    gsap.registerPlugin(ScrollTrigger, useGSAP);
    terdaftar = true;
  }
  return true;
}

/** Cek preferensi sistem reduced-motion (serta penanda skip animasi pada lingkungan test). */
export function apakahGerakDikurangi(): boolean {
  if ((globalThis as { __GARDA_SKIP_GSAP__?: boolean }).__GARDA_SKIP_GSAP__) {
    return true;
  }
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return false;
  }
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

export const GSAP_SOFT = {
  ease: "power3.out",
  easeExpo: "expo.out",
  duration: 0.45,
  stagger: 0.04,
  distance: 10,
} as const;

export interface SoftRevealOptions {
  y?: number;
  duration?: number;
  delay?: number;
  stagger?: number;
  ease?: string;
}

/**
 * Reveal masuk lembut memakai GSAP. Menggunakan `clearProps` setelah selesai agar tidak meninggalkan transform
 * statis yang bisa bentrok dengan Motion (`hoverLift` / `layoutId`) atau menimpa state hover CSS.
 */
export function softReveal(
  targets: gsap.TweenTarget,
  {
    y = GSAP_SOFT.distance,
    duration = GSAP_SOFT.duration,
    delay = 0,
    stagger = GSAP_SOFT.stagger,
    ease = GSAP_SOFT.ease,
  }: SoftRevealOptions = {},
) {
  if (!pastikanGsap() || apakahGerakDikurangi()) return null;
  return gsap.fromTo(
    targets,
    { opacity: 0, y },
    {
      opacity: 1,
      y: 0,
      duration,
      delay,
      stagger,
      ease,
      clearProps: "transform,opacity",
      overwrite: "auto",
    },
  );
}

/**
 * Reveal berbasis ScrollTrigger untuk daftar/seksi di halaman yang menggulir. Aman untuk elemen yang sudah
 * berada di dalam viewport saat awal muat maupun di lingkungan test DOM.
 */
export function softScrollReveal(
  targets: gsap.TweenTarget,
  trigger: Element | string | null | undefined,
  {
    y = GSAP_SOFT.distance,
    duration = GSAP_SOFT.duration,
    stagger = GSAP_SOFT.stagger,
    ease = GSAP_SOFT.ease,
  }: SoftRevealOptions = {},
) {
  if (!pastikanGsap() || !trigger || apakahGerakDikurangi()) return null;
  return gsap.fromTo(
    targets,
    { opacity: 0, y },
    {
      opacity: 1,
      y: 0,
      duration,
      stagger,
      ease,
      clearProps: "transform,opacity",
      scrollTrigger: {
        trigger,
        start: "top 92%",
        once: true,
      },
    },
  );
}

export { gsap, ScrollTrigger, useGSAP };
