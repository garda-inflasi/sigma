"use client";

import { motion, type HTMLMotionProps } from "motion/react";
import { forwardRef } from "react";
import { hoverLift } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";

type Level = "subtle" | "base" | "strong";

export interface GlassCardProps extends HTMLMotionProps<"div"> {
  level?: Level;
  /** Aktifkan hover lift — hanya untuk kartu yang dapat diklik */
  interactive?: boolean;
  radius?: "2xl" | "3xl" | "4xl";
}

const levelClass: Record<Level, string> = { subtle: "glass-subtle", base: "glass", strong: "glass-strong" };
const radiusClass = { "2xl": "rounded-xl", "3xl": "rounded-xl", "4xl": "rounded-xl" };

export const GlassCard = forwardRef<HTMLDivElement, GlassCardProps>(function GlassCard(
  { level = "base", interactive = false, radius = "2xl", className, ...props },
  ref,
) {
  return (
    <motion.div
      ref={ref}
      className={cn(
        levelClass[level],
        radiusClass[radius],
        "relative",
        interactive && "cursor-pointer transition-shadow duration-300 hover:shadow-lift",
        className,
      )}
      {...(interactive ? hoverLift : {})}
      {...props}
    />
  );
});
