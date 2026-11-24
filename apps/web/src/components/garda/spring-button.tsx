"use client";

import { cva, type VariantProps } from "class-variance-authority";
import { motion, type HTMLMotionProps } from "motion/react";
import Link from "next/link";
import { forwardRef } from "react";
import { kelasDasarTombol, ukuranTombol } from "@/components/ui/button";
import { buttonSpring } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";

export const buttonVariants = cva(
  `${kelasDasarTombol} select-none transition-[background-color,box-shadow,color] duration-200 [&_svg]:size-4`,
  {
    variants: {
      variant: {
        primary:
          "bg-navy-800 text-white shadow-ambient-xs hover:bg-navy-700 dark:bg-white dark:text-navy-900 dark:hover:bg-navy-100",
        data: "bg-data-600 text-white shadow-ambient-xs hover:bg-data-700",
        soft: "border border-border bg-card text-foreground shadow-ambient-xs hover:bg-secondary",
        outline:
          "border border-border bg-card text-foreground shadow-ambient-xs hover:bg-secondary",
        ghost: "text-muted-foreground hover:bg-secondary hover:text-foreground",
        danger:
          "bg-kritis text-white shadow-ambient-xs hover:bg-kritis/90 focus-visible:ring-destructive/25",
        siklus: "bg-siklus text-white shadow-ambient-xs hover:brightness-110",
      },
      size: {
        sm: ukuranTombol.sm,
        md: ukuranTombol.md,
        lg: ukuranTombol.lg,
        xl: "h-12 px-5 text-[15px]",
        icon: "size-9",
      },
    },
    defaultVariants: { variant: "primary", size: "md" },
  },
);

export type SpringButtonProps = HTMLMotionProps<"button"> &
  VariantProps<typeof buttonVariants>;

/** Tombol dengan pegas saat hover/tekan. Gunakan untuk semua aksi utama. */
export const SpringButton = forwardRef<HTMLButtonElement, SpringButtonProps>(
  function SpringButton(
    { className, variant, size, type = "button", ...props },
    ref,
  ) {
    return (
      <motion.button
        ref={ref}
        type={type}
        className={cn(buttonVariants({ variant, size }), className)}
        {...buttonSpring}
        {...props}
      />
    );
  },
);

const MotionNextLink = motion.create(Link);

export function SpringLink({
  className,
  variant,
  size,
  ...props
}: React.ComponentProps<typeof MotionNextLink> &
  VariantProps<typeof buttonVariants>) {
  return (
    <MotionNextLink
      className={cn(buttonVariants({ variant, size }), className)}
      {...buttonSpring}
      {...props}
    />
  );
}
