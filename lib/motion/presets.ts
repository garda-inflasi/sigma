/**
 * Preset gerak Garda (motion/react — penerus Framer Motion, API identik).
 * Prinsip: gerak menjawab aksi pengguna; gerak otomatis hanya untuk menarik
 * perhatian (EWS, data baru). Semua preset menghormati prefers-reduced-motion
 * melalui <MotionConfig reducedMotion="user"> di providers.
 */
import type { Transition, Variants } from "motion/react";

export const ease = {
  garda: [0.22, 1, 0.36, 1] as [number, number, number, number],
  inOut: [0.65, 0, 0.35, 1] as [number, number, number, number],
  out: [0.16, 1, 0.3, 1] as [number, number, number, number],
};

export const spring = {
  /** Kartu, panel, hover lift */
  soft: { type: "spring", stiffness: 260, damping: 28, mass: 0.9 },
  /** Tombol, toggle, chip */
  snappy: { type: "spring", stiffness: 480, damping: 30 },
  /** Pop-in, marker peta, badge baru */
  bouncy: { type: "spring", stiffness: 520, damping: 17, mass: 0.8 },
  /** Indikator layoutId (sidebar, tab, bottom nav) */
  layout: { type: "spring", stiffness: 400, damping: 34 },
} satisfies Record<string, Transition>;

/* ── Entrance ─────────────────────────────────────────────── */
export const fadeUp: Variants = {
  hidden: { opacity: 0, y: 14, filter: "blur(4px)" },
  show: { opacity: 1, y: 0, filter: "blur(0px)", transition: { duration: 0.55, ease: ease.garda } },
  exit: { opacity: 0, y: -6, filter: "blur(2px)", transition: { duration: 0.18 } },
};

export const fadeIn: Variants = {
  hidden: { opacity: 0 },
  show: { opacity: 1, transition: { duration: 0.35, ease: ease.out } },
  exit: { opacity: 0, transition: { duration: 0.15 } },
};

/** Kontainer stagger — pakai bersama fadeUp/popIn pada anak. */
export const stagger = (step = 0.06, delayChildren = 0.04): Variants => ({
  hidden: {},
  show: { transition: { staggerChildren: step, delayChildren } },
  exit: { transition: { staggerChildren: 0.03, staggerDirection: -1 } },
});

export const popIn: Variants = {
  hidden: { opacity: 0, scale: 0.86 },
  show: { opacity: 1, scale: 1, transition: spring.bouncy },
  exit: { opacity: 0, scale: 0.92, transition: { duration: 0.14 } },
};

/** Item feed real-time (EWS): masuk dari kanan, keluar menyusut. */
export const feedItem: Variants = {
  hidden: { opacity: 0, x: 28, height: 0, marginBottom: 0 },
  show: { opacity: 1, x: 0, height: "auto", marginBottom: 8, transition: { ...spring.soft, opacity: { duration: 0.25 } } },
  exit: { opacity: 0, x: -20, height: 0, marginBottom: 0, transition: { duration: 0.22, ease: ease.inOut } },
};

/** Varian feed tanpa jarak antar-item (untuk daftar berbingkai). */
export const feedItemFlat: Variants = {
  hidden: { opacity: 0, x: 24, height: 0 },
  show: { opacity: 1, x: 0, height: "auto", transition: { ...spring.soft, opacity: { duration: 0.25 } } },
  exit: { opacity: 0, x: -20, height: 0, transition: { duration: 0.22, ease: ease.inOut } },
};

/** Bilah aksi massal: naik dari bawah dengan pegas. */
export const bulkBar: Variants = {
  hidden: { opacity: 0, y: 32, scale: 0.96 },
  show: { opacity: 1, y: 0, scale: 1, transition: spring.snappy },
  exit: { opacity: 0, y: 24, scale: 0.97, transition: { duration: 0.16 } },
};

/* ── Interaksi ────────────────────────────────────────────── */
/** Hover lift untuk kartu yang dapat diklik. Bayangan ditangani kelas `hover:shadow-lift`. */
export const hoverLift = {
  whileHover: { y: -4, transition: spring.soft },
  whileTap: { y: -1, scale: 0.995, transition: spring.snappy },
} as const;

/** Pegas tombol. */
export const buttonSpring = {
  whileHover: { scale: 1.02 },
  whileTap: { scale: 0.955 },
  transition: spring.snappy,
} as const;

/* ── Perhatian ────────────────────────────────────────────── */
/** Cincin denyut EWS. `delay` dipakai untuk cincin bertingkat. */
export const pulseRing = (delay = 0, duration = 1.9) => ({
  initial: { scale: 1, opacity: 0.55 },
  animate: { scale: [1, 2.6], opacity: [0.5, 0] },
  transition: { duration, ease: "easeOut" as const, repeat: Infinity, delay },
});

/* ── Data viz ─────────────────────────────────────────────── */
export const drawPath: Variants = {
  hidden: { pathLength: 0, opacity: 0 },
  show: {
    pathLength: 1,
    opacity: 1,
    transition: { pathLength: { duration: 1.15, ease: ease.inOut }, opacity: { duration: 0.2 } },
  },
};

/** Marker peta: jatuh-memantul dengan stagger berbasis indeks (pakai `custom={i}`). */
export const markerBounce: Variants = {
  hidden: { opacity: 0, y: -26, scale: 0.55 },
  show: (i: number) => ({ opacity: 1, y: 0, scale: 1, transition: { ...spring.bouncy, delay: 0.2 + i * 0.045 } }),
};

/** Sel heatmap: gelombang diagonal (custom = baris + kolom). */
export const heatCell: Variants = {
  hidden: { opacity: 0, scale: 0.6 },
  show: (d: number) => ({ opacity: 1, scale: 1, transition: { ...spring.soft, delay: d * 0.025 } }),
};

/** Durasi default countUp. */
export const countUp = { duration: 1.25, ease: ease.garda } as const;
