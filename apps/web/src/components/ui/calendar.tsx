"use client";

import { ChevronDown, ChevronLeft, ChevronRight } from "lucide-react";
import type { ComponentProps } from "react";
import { DayPicker } from "react-day-picker";
import { id } from "react-day-picker/locale";
import { cn } from "@/lib/utils";

const FOKUS =
  "outline-none focus-visible:ring-[3px] focus-visible:ring-ring/40";

const TOMBOL_NAV = cn(
  "pointer-events-auto grid size-8 place-items-center rounded-md text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground disabled:pointer-events-none disabled:opacity-30",
  FOKUS,
);

function Panah({
  orientation,
  className,
}: {
  orientation?: "up" | "down" | "left" | "right";
  className?: string;
}) {
  const Ikon =
    orientation === "left"
      ? ChevronLeft
      : orientation === "right"
        ? ChevronRight
        : ChevronDown;
  return <Ikon aria-hidden className={cn("size-4", className)} />;
}

/** Kalender berbahasa Indonesia (minggu mulai Senin) dengan pemilih bulan dan tahun. */
export function Calendar({
  className,
  classNames,
  components,
  ...props
}: ComponentProps<typeof DayPicker>) {
  return (
    <DayPicker
      locale={id}
      weekStartsOn={1}
      showOutsideDays
      fixedWeeks
      captionLayout="dropdown"
      className={cn("w-[17.25rem] max-w-full select-none", className)}
      classNames={{
        months: "relative",
        month: "flex flex-col gap-1.5",
        nav: "pointer-events-none absolute inset-x-0 top-0 z-10 flex h-8 items-center justify-between",
        button_previous: TOMBOL_NAV,
        button_next: TOMBOL_NAV,
        month_caption: "flex h-8 items-center justify-center px-9",
        dropdowns: "flex items-center gap-1",
        dropdown_root:
          "relative inline-flex h-8 items-center rounded-md px-2 transition-colors hover:bg-secondary has-focus-visible:ring-[3px] has-focus-visible:ring-ring/40",
        dropdown:
          "absolute inset-0 z-[1] w-full cursor-pointer bg-popover opacity-0 dark:scheme-dark",
        caption_label:
          "flex items-center gap-1 text-[13px] font-semibold whitespace-nowrap text-foreground [&>svg]:size-3.5 [&>svg]:text-muted-foreground",
        month_grid: "w-full border-collapse",
        weekday:
          "h-7 w-9 p-0 text-center text-[11px] font-medium text-muted-foreground",
        day: "p-px text-center",
        day_button: cn(
          "num mx-auto grid size-[2.375rem] place-items-center rounded-md text-[13px] text-foreground transition-colors hover:bg-secondary",
          FOKUS,
        ),
        today:
          "[&>button]:font-semibold [&>button]:text-data-700 [&>button]:ring-1 [&>button]:ring-data-500/70 [&>button]:ring-inset dark:[&>button]:text-data-300",
        selected:
          "[&>button]:bg-primary [&>button]:font-semibold [&>button]:text-primary-foreground [&>button]:hover:bg-primary/90 dark:[&>button]:text-primary-foreground",
        outside: "[&>button]:text-muted-foreground/55",
        disabled:
          "[&>button]:cursor-not-allowed [&>button]:text-muted-foreground/35 [&>button]:hover:bg-transparent",
        hidden: "invisible",
        ...classNames,
      }}
      components={{ Chevron: Panah, ...components }}
      {...props}
    />
  );
}
