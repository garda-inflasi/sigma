"use client";

import { cva, type VariantProps } from "class-variance-authority";
import { motion, type HTMLMotionProps } from "motion/react";
import Link from "next/link";
import { forwardRef } from "react";
import { buttonSpring } from "@/lib/motion/presets";
import { cn } from "@/lib/utils";

export const buttonVariants = cva(
  "inline-flex select-none items-center justify-center gap-2 whitespace-nowrap font-medium transition-[background-color,box-shadow,color] duration-200 focus-visible:outline-2 focus-visible:outline-offset-2 disabled:pointer-events-none disabled:opacity-50 [&_svg]:size-4 [&_svg]:shrink-0",
  {
    variants: {
      variant: {
        primary: "bg-gradient-to-b from-navy-700 to-navy-800 text-white shadow-ambient-sm hover:shadow-ambient dark:from-navy-200 dark:to-navy-300 dark:text-navy-950",
        data: "bg-data-600 text-white shadow-ambient-sm hover:bg-data-700 hover:shadow-glow-data",
        soft: "glass-subtle text-foreground hover:bg-white/70 dark:hover:bg-white/8",
        outline: "border border-border bg-transparent text-foreground hover:bg-secondary",
        ghost: "text-muted-foreground hover:bg-secondary hover:text-foreground",
        danger: "bg-defisit text-white shadow-ambient-sm hover:bg-kritis hover:shadow-glow-defisit",
        siklus: "bg-siklus text-white shadow-ambient-sm hover:brightness-110",
      },
      size: {
        sm: "h-8 rounded-lg px-3 text-xs",
        md: "h-10 rounded-xl px-4 text-sm",
        lg: "h-12 rounded-xl px-5 text-[15px]",
        xl: "h-14 rounded-xl px-6 text-base",
        icon: "size-9 rounded-xl",
      },
    },
    defaultVariants: { variant: "primary", size: "md" },
  },
);

export type SpringButtonProps = HTMLMotionProps<"button"> & VariantProps<typeof buttonVariants>;

/** Tombol dengan pegas saat hover/tekan. Gunakan untuk semua aksi utama. */
export const SpringButton = forwardRef<HTMLButtonElement, SpringButtonProps>(function SpringButton(
  { className, variant, size, type = "button", ...props },
  ref,
) {
  return <motion.button ref={ref} type={type} className={cn(buttonVariants({ variant, size }), className)} {...buttonSpring} {...props} />;
});

const MotionNextLink = motion.create(Link);

export function SpringLink({ className, variant, size, ...props }: React.ComponentProps<typeof MotionNextLink> & VariantProps<typeof buttonVariants>) {
  return <MotionNextLink className={cn(buttonVariants({ variant, size }), className)} {...buttonSpring} {...props} />;
}
