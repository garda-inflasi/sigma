"use client";

import { motion, type HTMLMotionProps } from "motion/react";
import type { ReactNode } from "react";
import { ease } from "@/lib/motion/presets";

/** Masuk halaman: fade + geser naik 10px, 300ms, easing Garda. Reduced-motion ditangani MotionConfig di providers. */
export function PageEnter({ children, ...props }: { children: ReactNode } & HTMLMotionProps<"div">) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3, ease: ease.garda }}
      {...props}
    >
      {children}
    </motion.div>
  );
}

const wadah = {
  hidden: {},
  show: { transition: { staggerChildren: 0.04, delayChildren: 0.05 } },
};
const butir = {
  hidden: { opacity: 0, y: 10 },
  show: { opacity: 1, y: 0, transition: { duration: 0.3, ease: ease.garda } },
};

/** Wadah stagger ringan (40ms) untuk deretan kartu KPI atau panel. */
export function Stagger({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <motion.div className={className} variants={wadah} initial="hidden" animate="show">
      {children}
    </motion.div>
  );
}

export function StaggerItem({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <motion.div className={className} variants={butir}>
      {children}
    </motion.div>
  );
}
