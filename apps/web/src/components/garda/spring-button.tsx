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
        primary: "bg-navy-800 text-white shadow-ambient-xs hover:bg-navy-700 dark:bg-white dark:text-navy-900 dark:hover:bg-navy-100",
        data: "bg-data-600 text-white shadow-ambient-xs hover:bg-data-700 focus-visible:shadow-glow-data",
        soft: "border border-border bg-card text-foreground shadow-ambient-xs hover:bg-secondary",
        outline: "border border-border bg-card text-foreground shadow-ambient-xs hover:bg-secondary",
        ghost: "text-muted-foreground hover:bg-secondary hover:text-foreground",
        danger: "bg-defisit-ink text-white shadow-ambient-xs hover:bg-kritis focus-visible:shadow-glow-defisit",
        siklus: "bg-siklus text-white shadow-ambient-xs hover:brightness-110",
      },
      size: {
        sm: "h-8 rounded-md px-3 text-xs",
        md: "h-9 rounded-md px-3.5 text-[13px]",
        lg: "h-10 rounded-md px-4 text-sm",
        xl: "h-12 rounded-md px-5 text-[15px]",
        icon: "size-9 rounded-md",
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
