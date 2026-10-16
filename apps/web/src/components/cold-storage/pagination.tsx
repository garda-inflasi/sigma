"use client";

import { ChevronLeft, ChevronRight } from "lucide-react";
import { cn } from "@/lib/utils";
import { paginationItems } from "./pagination-items";

interface Props {
  page: number;
  totalPages: number;
  onChange: (page: number) => void;
  disabled?: boolean;
  className?: string;
}

const edgeButton =
  "inline-flex h-8 items-center gap-1 rounded-md px-2 text-sm text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground disabled:pointer-events-none disabled:opacity-40";

/** Paginasi bernomor: Sebelumnya · 1 2 3 … · Berikutnya. */
export function Pagination({ page, totalPages, onChange, disabled, className }: Props) {
  if (totalPages <= 1) return null;

  return (
    <nav aria-label="Paginasi stok" className={cn("flex items-center gap-1", className)}>
      <button onClick={() => onChange(page - 1)} disabled={disabled || page <= 1} className={edgeButton} aria-label="Halaman sebelumnya">
        <ChevronLeft className="size-4" />
        <span className="hidden sm:inline">Sebelumnya</span>
      </button>

      {paginationItems(page, totalPages).map((item, i) =>
        item === "…" ? (
          <span key={`gap-${i}`} aria-hidden className="grid size-8 place-items-center text-sm text-muted-foreground">…</span>
        ) : (
          <button
            key={item}
            onClick={() => onChange(item)}
            disabled={disabled}
            aria-label={`Halaman ${item}`}
            aria-current={item === page ? "page" : undefined}
            className={cn(
              "num grid size-8 place-items-center rounded-md text-sm transition-colors disabled:pointer-events-none",
              item === page
                ? "bg-navy-800 font-semibold text-white shadow-ambient-xs dark:bg-data-500"
                : "text-muted-foreground hover:bg-secondary hover:text-foreground",
            )}
          >
            {item}
          </button>
        ),
      )}

      <button onClick={() => onChange(page + 1)} disabled={disabled || page >= totalPages} className={edgeButton} aria-label="Halaman berikutnya">
        <span className="hidden sm:inline">Berikutnya</span>
        <ChevronRight className="size-4" />
      </button>
    </nav>
  );
}
